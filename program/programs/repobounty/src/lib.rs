use anchor_lang::prelude::*;

pub mod constants;
pub mod errors;
pub mod instructions;
pub mod state;

use instructions::*;

declare_id!("5VdatUgJ6AsZ7RbC8TBz6AxUdBNtQ6MckLsKbxgZQdS6");

#[program]
pub mod repobounty {
    use super::*;

    pub fn initialize_config(
        ctx: Context<InitializeConfig>,
        finalize_authority: Pubkey,
        claim_authority: Pubkey,
    ) -> Result<()> {
        instructions::initialize_config::handler(ctx, finalize_authority, claim_authority)
    }

    pub fn update_config(
        ctx: Context<UpdateConfig>,
        new_admin: Option<Pubkey>,
        new_finalize_authority: Option<Pubkey>,
        new_claim_authority: Option<Pubkey>,
        paused: Option<bool>,
    ) -> Result<()> {
        instructions::update_config::handler(
            ctx,
            new_admin,
            new_finalize_authority,
            new_claim_authority,
            paused,
        )
    }

    pub fn create_campaign_with_deposit(
        ctx: Context<CreateCampaignWithDeposit>,
        campaign_id: u64,
        github_repo_id: u64,
        repo_owner: String,
        repo_name: String,
        deadline_at: i64,
        total_amount: u64,
    ) -> Result<()> {
        instructions::create_campaign::handler(
            ctx,
            campaign_id,
            github_repo_id,
            repo_owner,
            repo_name,
            deadline_at,
            total_amount,
        )
    }

    pub fn finalize_campaign<'info>(
        ctx: Context<'_, '_, 'info, 'info, FinalizeCampaign<'info>>,
        allocations: Vec<AllocationInput>,
        is_final_batch: bool,
    ) -> Result<()> {
        instructions::finalize_campaign::handler(ctx, allocations, is_final_batch)
    }

    pub fn claim_backend_paid(ctx: Context<ClaimBackendPaid>) -> Result<()> {
        instructions::claim_backend_paid::handler(ctx)
    }

    pub fn claim_user_paid(ctx: Context<ClaimUserPaid>) -> Result<()> {
        instructions::claim_user_paid::handler(ctx)
    }

    pub fn refund_unclaimed(ctx: Context<RefundUnclaimed>) -> Result<()> {
        instructions::refund_unclaimed::handler(ctx)
    }
}
