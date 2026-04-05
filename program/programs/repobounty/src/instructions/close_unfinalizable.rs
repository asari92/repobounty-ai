use anchor_lang::prelude::*;

use crate::constants::*;
use crate::errors::RepoBountyError;
use crate::events::CampaignClosed;
use crate::state::{Campaign, Config};

#[derive(Accounts)]
pub struct CloseUnfinalizableCampaign<'info> {
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
        constraint = campaign.allocations_count == 0 @ RepoBountyError::PartialFinalizationExists,
    )]
    pub campaign: Account<'info, Campaign>,
}

pub fn handler(ctx: Context<CloseUnfinalizableCampaign>) -> Result<()> {
    let clock = Clock::get()?;
    let deadline_at = ctx.accounts.campaign.deadline_at;

    require!(
        clock.unix_timestamp >= deadline_at,
        RepoBountyError::DeadlineNotReached
    );

    ctx.accounts.campaign.status = STATUS_CLOSED;

    emit!(CampaignClosed {
        campaign_pubkey: ctx.accounts.campaign.key(),
        reason: "unfinalizable".to_string(),
    });

    Ok(())
}
