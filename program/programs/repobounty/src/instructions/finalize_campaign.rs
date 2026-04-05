use anchor_lang::prelude::*;
use anchor_lang::system_program;

use crate::constants::*;
use crate::errors::RepoBountyError;
use crate::state::{Campaign, CampaignStatus, ClaimRecord, Config};

/// Input DTO for each allocation in a finalize batch.
#[derive(AnchorSerialize, AnchorDeserialize, Clone)]
pub struct AllocationInput {
    pub github_user_id: u64,
    pub github_username: String,
    pub amount: u64,
}

#[derive(Accounts)]
pub struct FinalizeCampaign<'info> {
    #[account(
        seeds = [SEED_CONFIG],
        bump = config.bump,
        constraint = !config.paused @ RepoBountyError::ProgramPaused,
        has_one = finalize_authority @ RepoBountyError::Unauthorized,
    )]
    pub config: Account<'info, Config>,

    #[account(
        mut,
        constraint = campaign.status == CampaignStatus::Active
            || campaign.status == CampaignStatus::Finalizing
            @ RepoBountyError::CampaignNotActiveOrFinalizing,
    )]
    pub campaign: Account<'info, Campaign>,

    #[account(mut)]
    pub finalize_authority: Signer<'info>,

    pub system_program: Program<'info, System>,
}

pub fn handler<'info>(
    ctx: Context<'_, '_, 'info, 'info, FinalizeCampaign<'info>>,
    allocations: Vec<AllocationInput>,
    is_final_batch: bool,
) -> Result<()> {
    // Extract values before mutable borrows
    let campaign_key = ctx.accounts.campaign.key();
    let total_amount = ctx.accounts.campaign.total_amount;
    let current_allocated = ctx.accounts.campaign.allocated_amount;
    let current_count = ctx.accounts.campaign.allocations_count;
    let program_id = ctx.program_id;

    // Validate batch
    require!(!allocations.is_empty(), RepoBountyError::EmptyAllocations);
    require!(
        allocations.len() <= MAX_ALLOCATIONS_PER_BATCH,
        RepoBountyError::TooManyAllocations
    );
    require!(
        ctx.remaining_accounts.len() == allocations.len(),
        RepoBountyError::InvalidClaimRecord
    );

    // Validate each allocation + check for duplicates within batch
    let mut seen_ids = Vec::with_capacity(allocations.len());
    for alloc in &allocations {
        require!(alloc.github_user_id > 0, RepoBountyError::InvalidGithubUserId);
        require!(
            alloc.github_username.len() <= MAX_GITHUB_USERNAME_LEN,
            RepoBountyError::GithubUsernameTooLong
        );
        require!(alloc.amount > 0, RepoBountyError::ZeroAllocationAmount);
        require!(
            !seen_ids.contains(&alloc.github_user_id),
            RepoBountyError::DuplicateAllocation
        );
        seen_ids.push(alloc.github_user_id);
    }

    let clock = Clock::get()?;
    require!(
        clock.unix_timestamp >= ctx.accounts.campaign.deadline_at,
        RepoBountyError::DeadlineNotReached
    );

    // Create ClaimRecord accounts via remaining_accounts
    let rent = Rent::get()?;
    let space = ClaimRecord::SPACE;
    let lamports = rent.minimum_balance(space);
    let mut batch_total: u64 = 0;

    for (i, alloc) in allocations.iter().enumerate() {
        let claim_account_info = &ctx.remaining_accounts[i];

        // Derive and verify PDA
        let user_id_bytes = alloc.github_user_id.to_le_bytes();
        let seeds: &[&[u8]] = &[SEED_CLAIM, campaign_key.as_ref(), &user_id_bytes];
        let (expected_key, bump) = Pubkey::find_program_address(seeds, program_id);
        require_keys_eq!(
            claim_account_info.key(),
            expected_key,
            RepoBountyError::InvalidClaimRecord
        );

        // Account must not already exist
        require!(
            claim_account_info.lamports() == 0,
            RepoBountyError::DuplicateAllocation
        );

        // Create account (PDA signs)
        let signer_seeds: &[&[u8]] = &[
            SEED_CLAIM,
            campaign_key.as_ref(),
            &user_id_bytes,
            &[bump],
        ];

        system_program::create_account(
            CpiContext::new_with_signer(
                ctx.accounts.system_program.to_account_info(),
                system_program::CreateAccount {
                    from: ctx.accounts.finalize_authority.to_account_info(),
                    to: claim_account_info.clone(),
                },
                &[signer_seeds],
            ),
            lamports,
            space as u64,
            program_id,
        )?;

        // Serialize ClaimRecord into the new account
        let claim = ClaimRecord {
            campaign: campaign_key,
            github_user_id: alloc.github_user_id,
            github_username: alloc.github_username.clone(),
            amount: alloc.amount,
            claimed: false,
            claimed_to_wallet: None,
            claimed_at: None,
            bump,
        };

        let mut data = claim_account_info.try_borrow_mut_data()?;
        claim.try_serialize(&mut &mut data[..])?;

        batch_total = batch_total
            .checked_add(alloc.amount)
            .ok_or(RepoBountyError::ArithmeticOverflow)?;
    }

    // Update campaign
    let new_allocated = current_allocated
        .checked_add(batch_total)
        .ok_or(RepoBountyError::ArithmeticOverflow)?;
    let new_count = current_count
        .checked_add(allocations.len() as u32)
        .ok_or(RepoBountyError::ArithmeticOverflow)?;

    if is_final_batch {
        require!(
            new_allocated == total_amount,
            RepoBountyError::AllocationTotalMismatch
        );
    }

    let campaign = &mut ctx.accounts.campaign;
    campaign.allocated_amount = new_allocated;
    campaign.allocations_count = new_count;
    campaign.status = if is_final_batch {
        CampaignStatus::Finalized
    } else {
        CampaignStatus::Finalizing
    };

    Ok(())
}
