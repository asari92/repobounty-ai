use anchor_lang::prelude::*;
use anchor_spl::token::{self, Token, TokenAccount, Transfer};

use crate::constants::SEED_ESCROW_AUTHORITY;
use crate::errors::RepoBountyError;
use crate::state::{Campaign, CampaignStatus};

#[derive(Accounts)]
pub struct RefundUnclaimed<'info> {
    #[account(
        mut,
        has_one = sponsor @ RepoBountyError::InvalidSponsor,
        constraint = campaign.status == CampaignStatus::Finalized
            || campaign.status == CampaignStatus::Closed
            @ RepoBountyError::CampaignNotFinalized,
    )]
    pub campaign: Account<'info, Campaign>,

    /// CHECK: PDA escrow authority for token transfers.
    #[account(
        seeds = [SEED_ESCROW_AUTHORITY, campaign.key().as_ref()],
        bump = campaign.escrow_authority_bump,
    )]
    pub escrow_authority: UncheckedAccount<'info>,

    #[account(
        mut,
        constraint = escrow_token_account.key() == campaign.escrow_token_account,
    )]
    pub escrow_token_account: Account<'info, TokenAccount>,

    #[account(
        mut,
        constraint = sponsor_refund_account.mint == campaign.token_mint,
        constraint = sponsor_refund_account.owner == sponsor.key(),
    )]
    pub sponsor_refund_account: Account<'info, TokenAccount>,

    pub sponsor: Signer<'info>,

    pub token_program: Program<'info, Token>,
}

pub fn handler(ctx: Context<RefundUnclaimed>) -> Result<()> {
    let clock = Clock::get()?;

    // Extract values before mutable borrow
    let campaign_key = ctx.accounts.campaign.key();
    let claim_deadline_at = ctx.accounts.campaign.claim_deadline_at;
    let escrow_authority_bump = ctx.accounts.campaign.escrow_authority_bump;

    // Claim window must have expired (no paused check — refund always available)
    require!(
        clock.unix_timestamp > claim_deadline_at,
        RepoBountyError::ClaimWindowNotExpired
    );

    let escrow_balance = ctx.accounts.escrow_token_account.amount;
    require!(escrow_balance > 0, RepoBountyError::EscrowEmpty);

    // Transfer remaining escrow balance to sponsor
    let seeds = &[
        SEED_ESCROW_AUTHORITY,
        campaign_key.as_ref(),
        &[escrow_authority_bump],
    ];

    token::transfer(
        CpiContext::new_with_signer(
            ctx.accounts.token_program.to_account_info(),
            Transfer {
                from: ctx.accounts.escrow_token_account.to_account_info(),
                to: ctx.accounts.sponsor_refund_account.to_account_info(),
                authority: ctx.accounts.escrow_authority.to_account_info(),
            },
            &[seeds],
        ),
        escrow_balance,
    )?;

    let campaign = &mut ctx.accounts.campaign;
    campaign.status = CampaignStatus::Closed;

    Ok(())
}
