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
        campaign.sponsor = ctx.accounts.authority.key();
        campaign.campaign_id = campaign_id;
        campaign.repo = repo;
        campaign.pool_amount = pool_amount;
        campaign.total_claimed = 0;
        campaign.deadline = deadline;
        campaign.state = CampaignState::Created;
        campaign.allocations = vec![];
        campaign.bump = ctx.bumps.campaign;
        campaign.vault_bump = 0;
        campaign.created_at = clock.unix_timestamp;

        msg!(
            "Campaign created: {} | pool={} | deadline={}",
            campaign.repo,
            pool_amount,
            deadline
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
        seeds = [b"campaign", authority.key().as_ref(), campaign_id.as_bytes()],
        bump,
    )]
    pub campaign: Account<'info, Campaign>,
    #[account(mut)]
    pub authority: Signer<'info>,
    pub system_program: Program<'info, System>,
}

#[derive(Accounts)]
pub struct FinalizeCampaign<'info> {
    #[account(
        mut,
        has_one = authority,
        constraint = campaign.state == CampaignState::Created @ RepoBountyError::CampaignAlreadyFinalized,
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
    /// 4 + MAX_CONTRIBUTOR_LEN + 2 + 8 + 1 + (1 + 32) = 82 bytes
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
}
