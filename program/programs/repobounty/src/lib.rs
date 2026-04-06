use anchor_lang::prelude::*;

pub mod constants;
pub mod errors;
pub mod events;
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
        treasury_wallet: Pubkey,
    ) -> Result<()> {
        instructions::initialize_config::handler(ctx, finalize_authority, claim_authority, treasury_wallet)
    }

    pub fn update_config(
        ctx: Context<UpdateConfig>,
        finalize_authority: Option<Pubkey>,
        claim_authority: Option<Pubkey>,
        treasury_wallet: Option<Pubkey>,
    ) -> Result<()> {
        instructions::update_config::handler(
            ctx,
            finalize_authority,
            claim_authority,
            treasury_wallet,
        )
    }

    pub fn create_campaign_with_deposit(
        ctx: Context<CreateCampaignWithDeposit>,
        campaign_id: u64,
        github_repo_id: u64,
        deadline_at: i64,
        reward_amount: u64,
    ) -> Result<()> {
        instructions::create_campaign::handler(
            ctx,
            campaign_id,
            github_repo_id,
            deadline_at,
            reward_amount,
        )
    }

    pub fn finalize_campaign_batch<'info>(
        ctx: Context<'_, '_, 'info, 'info, FinalizeCampaignBatch<'info>>,
        allocations: Vec<AllocationEntry>,
        has_more: bool,
    ) -> Result<()> {
        instructions::finalize_campaign::handler(ctx, allocations, has_more)
    }

    pub fn claim(ctx: Context<Claim>, github_user_id: u64, payer_mode: u8) -> Result<()> {
        instructions::claim::handler(ctx, github_user_id, payer_mode)
    }

    pub fn close_unfinalizable_campaign(ctx: Context<CloseUnfinalizableCampaign>) -> Result<()> {
        instructions::close_unfinalizable::handler(ctx)
    }

    pub fn refund_unclaimed(ctx: Context<RefundUnclaimed>) -> Result<()> {
        instructions::refund_unclaimed::handler(ctx)
    }

    pub fn set_paused(ctx: Context<SetPaused>, paused: bool) -> Result<()> {
        instructions::set_paused::handler(ctx, paused)
    }
}
