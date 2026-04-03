use anchor_lang::prelude::*;
use anchor_spl::{
    associated_token::AssociatedToken,
    token::{self, Mint, Token, TokenAccount, Transfer},
};

use crate::constants::*;
use crate::errors::RepoBountyError;
use crate::state::{Campaign, CampaignStatus, Config};

#[derive(Accounts)]
#[instruction(campaign_id: u64)]
pub struct CreateCampaignWithDeposit<'info> {
    #[account(
        seeds = [SEED_CONFIG],
        bump = config.bump,
        constraint = !config.paused @ RepoBountyError::ProgramPaused,
    )]
    pub config: Account<'info, Config>,

    #[account(
        init,
        payer = sponsor,
        space = Campaign::SPACE,
        seeds = [SEED_CAMPAIGN, sponsor.key().as_ref(), &campaign_id.to_le_bytes()],
        bump,
    )]
    pub campaign: Account<'info, Campaign>,

    /// CHECK: PDA used as authority for the escrow token account.
    #[account(
        seeds = [SEED_ESCROW_AUTHORITY, campaign.key().as_ref()],
        bump,
    )]
    pub escrow_authority: UncheckedAccount<'info>,

    #[account(
        init,
        payer = sponsor,
        associated_token::mint = token_mint,
        associated_token::authority = escrow_authority,
    )]
    pub escrow_token_account: Account<'info, TokenAccount>,

    #[account(mut)]
    pub sponsor: Signer<'info>,

    #[account(
        mut,
        constraint = sponsor_token_account.owner == sponsor.key(),
        constraint = sponsor_token_account.mint == token_mint.key(),
    )]
    pub sponsor_token_account: Account<'info, TokenAccount>,

    pub token_mint: Account<'info, Mint>,

    pub token_program: Program<'info, Token>,
    pub associated_token_program: Program<'info, AssociatedToken>,
    pub system_program: Program<'info, System>,
}

pub fn handler(
    ctx: Context<CreateCampaignWithDeposit>,
    campaign_id: u64,
    github_repo_id: u64,
    repo_owner: String,
    repo_name: String,
    deadline_at: i64,
    total_amount: u64,
) -> Result<()> {
    require!(
        repo_owner.len() <= MAX_REPO_OWNER_LEN,
        RepoBountyError::RepoOwnerTooLong
    );
    require!(
        repo_name.len() <= MAX_REPO_NAME_LEN,
        RepoBountyError::RepoNameTooLong
    );
    require!(total_amount > 0, RepoBountyError::InvalidAmount);

    let clock = Clock::get()?;
    let now = clock.unix_timestamp;

    require!(
        deadline_at >= now
            .checked_add(MIN_DEADLINE_SECONDS)
            .ok_or(RepoBountyError::ArithmeticOverflow)?,
        RepoBountyError::DeadlineTooSoon
    );
    require!(
        deadline_at <= now
            .checked_add(MAX_DEADLINE_SECONDS)
            .ok_or(RepoBountyError::ArithmeticOverflow)?,
        RepoBountyError::DeadlineTooFar
    );

    // Transfer tokens from sponsor to escrow
    token::transfer(
        CpiContext::new(
            ctx.accounts.token_program.to_account_info(),
            Transfer {
                from: ctx.accounts.sponsor_token_account.to_account_info(),
                to: ctx.accounts.escrow_token_account.to_account_info(),
                authority: ctx.accounts.sponsor.to_account_info(),
            },
        ),
        total_amount,
    )?;

    // Initialize campaign state
    let campaign = &mut ctx.accounts.campaign;
    campaign.campaign_id = campaign_id;
    campaign.sponsor = ctx.accounts.sponsor.key();
    campaign.token_mint = ctx.accounts.token_mint.key();
    campaign.escrow_token_account = ctx.accounts.escrow_token_account.key();
    campaign.github_repo_id = github_repo_id;
    campaign.repo_owner = repo_owner;
    campaign.repo_name = repo_name;
    campaign.created_at = now;
    campaign.deadline_at = deadline_at;
    campaign.claim_deadline_at = deadline_at
        .checked_add(CLAIM_WINDOW_SECONDS)
        .ok_or(RepoBountyError::ArithmeticOverflow)?;
    campaign.total_amount = total_amount;
    campaign.allocated_amount = 0;
    campaign.claimed_amount = 0;
    campaign.allocations_count = 0;
    campaign.claimed_count = 0;
    campaign.status = CampaignStatus::Active;
    campaign.bump = ctx.bumps.campaign;
    campaign.escrow_authority_bump = ctx.bumps.escrow_authority;

    Ok(())
}
