use anchor_lang::prelude::*;

declare_id!("8oSXz4bbvUYVnNruhPEF3JR7jMsSApf7EpAyDpXxDLSJ");

/// Maximum repo identifier length (e.g. "owner/repo-name").
const MAX_REPO_LEN: usize = 64;
/// Maximum GitHub username length.
const MAX_CONTRIBUTOR_LEN: usize = 39;
/// Maximum contributors per campaign.
const MAX_ALLOCATIONS: usize = 10;
/// 100 % expressed in basis points.
const BPS_100: u16 = 10_000;

// ---------------------------------------------------------------------------
// Program
// ---------------------------------------------------------------------------

#[program]
pub mod repobounty {
    use super::*;

    /// Sponsor creates a new campaign for a public GitHub repository.
    pub fn create_campaign(
        ctx: Context<CreateCampaign>,
        campaign_id: String,
        repo: String,
        pool_amount: u64,
        deadline: i64,
        sponsor: Pubkey,
    ) -> Result<()> {
        require!(campaign_id.len() <= 32, RepoBountyError::CampaignIdTooLong);
        require!(repo.len() <= MAX_REPO_LEN, RepoBountyError::RepoNameTooLong);
        require!(pool_amount > 0, RepoBountyError::InvalidPoolAmount);

        let clock = Clock::get()?;
        require!(
            deadline > clock.unix_timestamp,
            RepoBountyError::DeadlineInPast
        );

        let campaign = &mut ctx.accounts.campaign;
        campaign.authority = ctx.accounts.authority.key();
        campaign.sponsor = sponsor;
        campaign.campaign_id = campaign_id;
        campaign.repo = repo;
        campaign.pool_amount = pool_amount;
        campaign.total_claimed = 0;
        campaign.deadline = deadline;
        campaign.state = CampaignState::Created;
        campaign.allocations = vec![];
        campaign.bump = ctx.bumps.campaign;
        campaign.vault_bump = ctx.bumps.vault;
        campaign.created_at = clock.unix_timestamp;
        campaign.finalized_at = None;

        msg!(
            "Campaign created: {} | pool={} | sponsor={}",
            campaign.repo,
            pool_amount,
            campaign.sponsor,
        );
        Ok(())
    }

    /// Sponsor funds the campaign by transferring SOL to vault.
    /// Must be called in the same transaction as SystemProgram.transfer.
    pub fn fund_campaign(ctx: Context<FundCampaign>) -> Result<()> {
        let campaign = &mut ctx.accounts.campaign;
        let vault = &ctx.accounts.vault;

        require!(
            campaign.state == CampaignState::Created,
            RepoBountyError::AlreadyFunded,
        );

        let vault_balance = vault.lamports();
        require!(
            vault_balance >= campaign.pool_amount,
            RepoBountyError::InsufficientFunds,
        );

        campaign.state = CampaignState::Funded;

        msg!(
            "Campaign funded: {} | vault_balance={} | state=Funded",
            campaign.repo,
            vault_balance,
        );
        Ok(())
    }

    /// Backend finalizes the campaign with AI-generated allocations.
    ///
    /// Validates that percentages sum to 100 % (10 000 bps), that contributor
    /// names are unique, and that the campaign has not already been finalized.
    pub fn finalize_campaign(
        ctx: Context<FinalizeCampaign>,
        allocations: Vec<AllocationInput>,
    ) -> Result<()> {
        require!(!allocations.is_empty(), RepoBountyError::EmptyAllocations);
        require!(
            allocations.len() <= MAX_ALLOCATIONS,
            RepoBountyError::TooManyAllocations
        );

        let clock = Clock::get()?;
        require!(
            clock.unix_timestamp >= ctx.accounts.campaign.deadline,
            RepoBountyError::DeadlineNotReached
        );

        // --- percentage validation -------------------------------------------
        let total_bps: u64 = allocations.iter().map(|a| a.percentage as u64).sum();
        require!(
            total_bps == BPS_100 as u64,
            RepoBountyError::InvalidAllocationTotal
        );

        // --- uniqueness check ------------------------------------------------
        let mut seen = Vec::with_capacity(allocations.len());
        for a in &allocations {
            require!(
                a.contributor.len() <= MAX_CONTRIBUTOR_LEN,
                RepoBountyError::ContributorNameTooLong
            );
            require!(
                !seen.contains(&a.contributor),
                RepoBountyError::DuplicateContributor
            );
            seen.push(a.contributor.clone());
        }

        // --- store -----------------------------------------------------------
        let campaign = &mut ctx.accounts.campaign;
        campaign.allocations = allocations
            .iter()
            .map(|a| Allocation {
                contributor: a.contributor.clone(),
                percentage: a.percentage,
                amount: campaign
                    .pool_amount
                    .checked_mul(a.percentage as u64)
                    .unwrap()
                    / BPS_100 as u64,
                claimed: false,
                claimant: None,
            })
            .collect();

        campaign.state = CampaignState::Finalized;
        campaign.finalized_at = Some(Clock::get()?.unix_timestamp);

        msg!(
            "Campaign finalized: {} | {} allocations",
            campaign.repo,
            campaign.allocations.len()
        );
        Ok(())
    }

    /// Contributor claims their allocated reward.
    pub fn claim(ctx: Context<Claim>, contributor_github: String) -> Result<()> {
        let campaign = &mut ctx.accounts.campaign;
        let vault = &mut ctx.accounts.vault;
        let contributor = &ctx.accounts.contributor;

        require!(
            campaign.state == CampaignState::Finalized,
            RepoBountyError::CampaignNotFinalized,
        );

        let allocation = campaign
            .allocations
            .iter_mut()
            .find(|a| a.contributor == contributor_github)
            .ok_or(RepoBountyError::ContributorNotFound)?;

        require!(!allocation.claimed, RepoBountyError::AlreadyClaimed,);

        require!(
            allocation.claimant.is_none() || Some(contributor.key()) == allocation.claimant,
            RepoBountyError::InvalidClaimant,
        );

        let vault_lamports = vault.lamports();
        let transfer_amount = allocation.amount;

        require!(
            vault_lamports >= transfer_amount,
            RepoBountyError::InsufficientVaultFunds,
        );

        **vault.to_account_info().try_borrow_mut_lamports()? -= transfer_amount;
        **contributor.to_account_info().try_borrow_mut_lamports()? += transfer_amount;

        allocation.claimed = true;
        allocation.claimant = Some(contributor.key());

        campaign.total_claimed += transfer_amount;

        let all_claimed = campaign.allocations.iter().all(|a| a.claimed);
        if all_claimed {
            campaign.state = CampaignState::Completed;
        }

        msg!(
            "Claim successful: {} | amount={} | contributor={}",
            campaign.campaign_id,
            transfer_amount,
            contributor_github,
        );
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

#[derive(Accounts)]
#[instruction(campaign_id: String)]
pub struct CreateCampaign<'info> {
    #[account(
        init,
        payer = authority,
        space = Campaign::space(),
        seeds = [b"campaign", campaign_id.as_bytes()],
        bump,
    )]
    pub campaign: Account<'info, Campaign>,
    #[account(mut)]
    pub authority: Signer<'info>,
    #[account(
        seeds = [b"vault", campaign.key().as_ref()],
        bump,
    )]
    pub vault: SystemAccount<'info>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct FundCampaign<'info> {
    #[account(
        mut,
        seeds = [b"campaign", campaign.campaign_id.as_bytes()],
        bump = campaign.bump,
    )]
    pub campaign: Account<'info, Campaign>,
    #[account(
        mut,
        seeds = [b"vault", campaign.key().as_ref()],
        bump = campaign.vault_bump,
    )]
    pub vault: SystemAccount<'info>,
    #[account(
        constraint = sponsor.key() == campaign.sponsor @ RepoBountyError::InvalidSponsor,
    )]
    pub sponsor: Signer<'info>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct Claim<'info> {
    #[account(
        mut,
        seeds = [b"campaign", campaign.campaign_id.as_bytes()],
        bump = campaign.bump,
    )]
    pub campaign: Account<'info, Campaign>,
    #[account(
        mut,
        seeds = [b"vault", campaign.key().as_ref()],
        bump = campaign.vault_bump,
    )]
    pub vault: SystemAccount<'info>,
    #[account(mut)]
    pub contributor: SystemAccount<'info>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct FinalizeCampaign<'info> {
    #[account(
        mut,
        has_one = authority,
        constraint = campaign.state == CampaignState::Funded @ RepoBountyError::AlreadyFinalized,
    )]
    pub campaign: Account<'info, Campaign>,
    pub authority: Signer<'info>,
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

#[account]
pub struct Campaign {
    /// Wallet that created and controls this campaign (backend key).
    pub authority: Pubkey,
    /// Wallet that funds the campaign (sponsor).
    pub sponsor: Pubkey,
    /// Short identifier used as PDA seed.
    pub campaign_id: String,
    /// GitHub repository in "owner/repo" format.
    pub repo: String,
    /// Total reward pool in lamports (or smallest token unit).
    pub pool_amount: u64,
    /// Total amount claimed (in lamports).
    pub total_claimed: u64,
    /// Unix timestamp after which finalization is allowed.
    pub deadline: i64,
    /// Current lifecycle state (Created, Funded, Finalized, Completed).
    pub state: CampaignState,
    /// AI-generated allocation results (populated on finalization).
    pub allocations: Vec<Allocation>,
    /// PDA bump seed.
    pub bump: u8,
    /// PDA bump seed for vault account.
    pub vault_bump: u8,
    /// Unix timestamp of creation.
    pub created_at: i64,
    /// Unix timestamp of finalization (None until finalized).
    pub finalized_at: Option<i64>,
}

impl Campaign {
    pub fn space() -> usize {
        8                                       // discriminator
        + 32                                    // authority
        + 32                                    // sponsor (new)
        + (4 + 32)                              // campaign_id (String)
        + (4 + MAX_REPO_LEN)                    // repo (String)
        + 8                                     // pool_amount
        + 8                                     // deadline
        + 1                                     // state enum
        + (4 + MAX_ALLOCATIONS * Allocation::SIZE) // allocations vec
        + 1                                     // bump
        + 1                                     // vault_bump (new)
        + 8                                     // total_claimed (new)
        + 8                                     // created_at
        + (1 + 8) // finalized_at (Option<i64>)
    }
}

#[derive(AnchorSerialize, AnchorDeserialize, Clone, PartialEq, Eq)]
pub enum CampaignState {
    Created,
    Funded,
    Finalized,
    Completed,
}

#[derive(AnchorSerialize, AnchorDeserialize, Clone)]
pub struct Allocation {
    pub contributor: String,
    pub percentage: u16,
    pub amount: u64,
    pub claimed: bool,
    pub claimant: Option<Pubkey>,
}

impl Allocation {
    /// 4 + MAX_CONTRIBUTOR_LEN + 2 + 8 + 1 + (1 + 32) = 87 bytes
    pub const SIZE: usize = 4 + MAX_CONTRIBUTOR_LEN + 2 + 8 + 1 + (1 + 32);
}

/// Input DTO for finalize_campaign instruction.
#[derive(AnchorSerialize, AnchorDeserialize, Clone)]
pub struct AllocationInput {
    pub contributor: String,
    pub percentage: u16,
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

#[error_code]
pub enum RepoBountyError {
    #[msg("Campaign ID must be 32 characters or fewer")]
    CampaignIdTooLong,
    #[msg("Repository name must be 64 characters or fewer")]
    RepoNameTooLong,
    #[msg("Pool amount must be greater than zero")]
    InvalidPoolAmount,
    #[msg("Deadline must be in the future")]
    DeadlineInPast,
    #[msg("Campaign has already been finalized")]
    CampaignAlreadyFinalized,
    #[msg("Allocations must not be empty")]
    EmptyAllocations,
    #[msg("Maximum 10 allocations allowed")]
    TooManyAllocations,
    #[msg("Allocation percentages must sum to 10000 basis points (100%)")]
    InvalidAllocationTotal,
    #[msg("Contributor username must be 39 characters or fewer")]
    ContributorNameTooLong,
    #[msg("Duplicate contributor in allocations")]
    DuplicateContributor,
    #[msg("Campaign has already been funded")]
    AlreadyFunded,
    #[msg("Insufficient funds in vault")]
    InsufficientFunds,
    #[msg("Invalid sponsor")]
    InvalidSponsor,
    #[msg("Allocation already claimed")]
    AlreadyClaimed,
    #[msg("Contributor not found in allocations")]
    ContributorNotFound,
    #[msg("Campaign is not finalized")]
    CampaignNotFinalized,
    #[msg("Invalid claimant wallet")]
    InvalidClaimant,
    #[msg("Insufficient funds in vault for claim")]
    InsufficientVaultFunds,
    #[msg("Campaign deadline has not been reached yet")]
    DeadlineNotReached,
}
