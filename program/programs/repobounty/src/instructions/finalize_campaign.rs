use anchor_lang::prelude::*;
use anchor_lang::system_program;

use crate::constants::*;
use crate::errors::RepoBountyError;
use crate::events::{CampaignFinalized, FinalizeBatchAppended};
use crate::state::{Campaign, ClaimRecord, Config};

#[derive(AnchorSerialize, AnchorDeserialize, Clone)]
pub struct AllocationEntry {
    pub github_user_id: u64,
    pub amount: u64,
}

#[derive(Accounts)]
pub struct FinalizeCampaignBatch<'info> {
    #[account(mut)]
    pub finalize_authority: Signer<'info>,

    #[account(
        seeds = [b"config"],
        bump = config.bump,
        constraint = !config.paused @ RepoBountyError::ProgramPaused,
        constraint = config.finalize_authority == finalize_authority.key() @ RepoBountyError::Unauthorized,
    )]
    pub config: Account<'info, Config>,

    #[account(
        mut,
        constraint = campaign.status == STATUS_ACTIVE @ RepoBountyError::CampaignNotActive,
    )]
    pub campaign: Account<'info, Campaign>,

    pub system_program: Program<'info, System>,
}

pub fn handler<'info>(
    ctx: Context<'_, '_, 'info, 'info, FinalizeCampaignBatch<'info>>,
    allocations: Vec<AllocationEntry>,
    has_more: bool,
) -> Result<()> {
    let campaign_key = ctx.accounts.campaign.key();
    let deadline_at = ctx.accounts.campaign.deadline_at;
    let total_amount = ctx.accounts.campaign.total_reward_amount;
    let current_allocated = ctx.accounts.campaign.allocated_amount;
    let current_count = ctx.accounts.campaign.allocations_count;
    let program_id = ctx.program_id;

    let clock = Clock::get()?;
    require!(
        clock.unix_timestamp >= deadline_at,
        RepoBountyError::DeadlineNotReached
    );

    require!(!allocations.is_empty(), RepoBountyError::EmptyAllocations);

    let mut seen_ids = Vec::with_capacity(allocations.len());
    for alloc in &allocations {
        require!(
            alloc.github_user_id > 0,
            RepoBountyError::InvalidGithubUserId
        );
        require!(
            alloc.amount >= MIN_ALLOCATION_AMOUNT,
            RepoBountyError::AllocationTooSmall
        );
        require!(
            !seen_ids.contains(&alloc.github_user_id),
            RepoBountyError::DuplicateAllocation
        );
        seen_ids.push(alloc.github_user_id);
    }

    let rent = Rent::get()?;
    let space = ClaimRecord::SPACE;
    let lamports = rent.minimum_balance(space);
    let mut batch_total: u64 = 0;

    for (i, alloc) in allocations.iter().enumerate() {
        let claim_account_info = &ctx.remaining_accounts[i];

        let user_id_bytes = alloc.github_user_id.to_le_bytes();
        let seeds: &[&[u8]] = &[b"claim", campaign_key.as_ref(), &user_id_bytes];
        let (expected_key, bump) = Pubkey::find_program_address(seeds, program_id);
        require_keys_eq!(
            claim_account_info.key(),
            expected_key,
            RepoBountyError::ClaimNotFound
        );

        require!(
            claim_account_info.lamports() == 0,
            RepoBountyError::ClaimRecordAlreadyExists
        );

        let signer_seeds: &[&[u8]] = &[b"claim", campaign_key.as_ref(), &user_id_bytes, &[bump]];

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

        let claim = ClaimRecord {
            campaign: campaign_key,
            github_user_id: alloc.github_user_id,
            amount: alloc.amount,
            claimed: false,
            claimed_to_wallet: None,
            claimed_at: None,
            bump,
            _reserved: [0; 32],
        };

        let mut data = claim_account_info.try_borrow_mut_data()?;
        claim.try_serialize(&mut &mut data[..])?;

        batch_total = batch_total
            .checked_add(alloc.amount)
            .ok_or(RepoBountyError::ArithmeticOverflow)?;
    }

    let new_allocated = current_allocated
        .checked_add(batch_total)
        .ok_or(RepoBountyError::ArithmeticOverflow)?;
    let new_count = current_count
        .checked_add(allocations.len() as u32)
        .ok_or(RepoBountyError::ArithmeticOverflow)?;

    require!(
        new_allocated <= total_amount,
        RepoBountyError::AllocationOverflow
    );

    emit!(FinalizeBatchAppended {
        campaign_pubkey: campaign_key,
        batch_count: allocations.len() as u32,
        batch_total_amount: batch_total,
        has_more,
    });

    let campaign = &mut ctx.accounts.campaign;
    campaign.allocated_amount = new_allocated;
    campaign.allocations_count = new_count;

    if !has_more {
        require!(
            campaign.allocated_amount == campaign.total_reward_amount,
            RepoBountyError::AllocationTotalMismatch
        );
        campaign.status = STATUS_FINALIZED;

        emit!(CampaignFinalized {
            campaign_pubkey: campaign_key,
            allocations_count: campaign.allocations_count,
            allocated_amount: campaign.allocated_amount,
        });
    }

    Ok(())
}
