use anchor_lang::prelude::*;
use anchor_spl::{
    associated_token::AssociatedToken,
    token::{self, Mint, Token, TokenAccount, Transfer},
};

use crate::constants::*;
use crate::errors::RepoBountyError;
use crate::state::{Campaign, CampaignStatus, ClaimRecord, Config};

#[derive(Accounts)]
pub struct ClaimBackendPaid<'info> {
    #[account(
        seeds = [SEED_CONFIG],
        bump = config.bump,
        constraint = !config.paused @ RepoBountyError::ProgramPaused,
        has_one = claim_authority @ RepoBountyError::Unauthorized,
    )]
    pub config: Box<Account<'info, Config>>,

    #[account(
        mut,
        constraint = campaign.status == CampaignStatus::Finalized @ RepoBountyError::CampaignNotFinalized,
    )]
    pub campaign: Box<Account<'info, Campaign>>,

    #[account(
        mut,
        seeds = [SEED_CLAIM, campaign.key().as_ref(), &claim_record.github_user_id.to_le_bytes()],
        bump = claim_record.bump,
        constraint = claim_record.campaign == campaign.key() @ RepoBountyError::InvalidClaimRecord,
        constraint = !claim_record.claimed @ RepoBountyError::ClaimAlreadyClaimed,
    )]
    pub claim_record: Box<Account<'info, ClaimRecord>>,

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
    pub escrow_token_account: Box<Account<'info, TokenAccount>>,

    #[account(
        init_if_needed,
        payer = claim_authority,
        associated_token::mint = token_mint,
        associated_token::authority = recipient,
    )]
    pub recipient_token_account: Box<Account<'info, TokenAccount>>,

    /// CHECK: Wallet that receives the claimed tokens.
    pub recipient: UncheckedAccount<'info>,

    #[account(constraint = token_mint.key() == campaign.token_mint)]
    pub token_mint: Box<Account<'info, Mint>>,

    #[account(mut)]
    pub claim_authority: Signer<'info>,

    pub token_program: Program<'info, Token>,
    pub associated_token_program: Program<'info, AssociatedToken>,
    pub system_program: Program<'info, System>,
}

pub fn handler(ctx: Context<ClaimBackendPaid>) -> Result<()> {
    let clock = Clock::get()?;

    // Extract values before mutable borrows
    let campaign_key = ctx.accounts.campaign.key();
    let claim_deadline_at = ctx.accounts.campaign.claim_deadline_at;
    let escrow_authority_bump = ctx.accounts.campaign.escrow_authority_bump;
    let claim_amount = ctx.accounts.claim_record.amount;
    let recipient_key = ctx.accounts.recipient.key();

    // Validate claim window and amount
    require!(
        clock.unix_timestamp <= claim_deadline_at,
        RepoBountyError::ClaimWindowExpired
    );
    require!(claim_amount > 0, RepoBountyError::InvalidAmount);
    require!(
        ctx.accounts.escrow_token_account.amount >= claim_amount,
        RepoBountyError::EscrowInsufficientFunds
    );

    // Transfer tokens from escrow to recipient
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
                to: ctx.accounts.recipient_token_account.to_account_info(),
                authority: ctx.accounts.escrow_authority.to_account_info(),
            },
            &[seeds],
        ),
        claim_amount,
    )?;

    // Update claim record
    let claim_record = &mut ctx.accounts.claim_record;
    claim_record.claimed = true;
    claim_record.claimed_to_wallet = Some(recipient_key);
    claim_record.claimed_at = Some(clock.unix_timestamp);

    // Update campaign counters
    let campaign = &mut ctx.accounts.campaign;
    campaign.claimed_amount = campaign
        .claimed_amount
        .checked_add(claim_amount)
        .ok_or(RepoBountyError::ArithmeticOverflow)?;
    campaign.claimed_count = campaign
        .claimed_count
        .checked_add(1)
        .ok_or(RepoBountyError::ArithmeticOverflow)?;

    if campaign.claimed_count == campaign.allocations_count {
        campaign.status = CampaignStatus::Closed;
    }

    Ok(())
}
