use anchor_lang::prelude::*;
use anchor_lang::system_program::{transfer, Transfer};

use crate::constants::*;
use crate::errors::RepoBountyError;
use crate::events::{CampaignClosed, RefundProcessed};
use crate::state::Campaign;

#[derive(Accounts)]
pub struct RefundUnclaimed<'info> {
    #[account(mut)]
    pub sponsor: Signer<'info>,

    #[account(
        mut,
        constraint = campaign.sponsor == sponsor.key() @ RepoBountyError::InvalidSponsor,
    )]
    pub campaign: Account<'info, Campaign>,

    #[account(
        mut,
        seeds = [b"escrow", campaign.key().as_ref()],
        bump,
    )]
    pub escrow: UncheckedAccount<'info>,

    pub system_program: Program<'info, System>,
}

pub fn handler(ctx: Context<RefundUnclaimed>) -> Result<()> {
    let clock = Clock::get()?;
    let claim_deadline_at = ctx.accounts.campaign.claim_deadline_at;

    require!(
        clock.unix_timestamp > claim_deadline_at,
        RepoBountyError::ClaimDeadlineNotReached
    );

    let escrow_balance = ctx.accounts.escrow.lamports();
    require!(escrow_balance > 0, RepoBountyError::EscrowEmpty);

    let campaign_key = ctx.accounts.campaign.key();
    let escrow_bump = ctx.bumps.escrow;

    let signer_seeds: &[&[&[u8]]] = &[&[b"escrow", campaign_key.as_ref(), &[escrow_bump]]];

    transfer(
        CpiContext::new_with_signer(
            ctx.accounts.system_program.to_account_info(),
            Transfer {
                from: ctx.accounts.escrow.to_account_info(),
                to: ctx.accounts.sponsor.to_account_info(),
            },
            signer_seeds,
        ),
        escrow_balance,
    )?;

    ctx.accounts.campaign.status = STATUS_CLOSED;

    emit!(RefundProcessed {
        campaign_pubkey: ctx.accounts.campaign.key(),
        sponsor: ctx.accounts.sponsor.key(),
        refunded_amount: escrow_balance,
    });

    emit!(CampaignClosed {
        campaign_pubkey: ctx.accounts.campaign.key(),
        reason: "refund".to_string(),
    });

    Ok(())
}
