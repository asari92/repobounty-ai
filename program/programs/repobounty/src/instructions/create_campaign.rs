use anchor_lang::prelude::*;
use anchor_lang::system_program::{transfer, Transfer};

use crate::constants::*;
use crate::errors::RepoBountyError;
use crate::events::CampaignCreated;
use crate::state::{Campaign, Config};

#[derive(Accounts)]
#[instruction(campaign_id: u64)]
pub struct CreateCampaignWithDeposit<'info> {
    #[account(mut)]
    pub sponsor: Signer<'info>,

    #[account(
        seeds = [b"config"],
        bump = config.bump,
        constraint = !config.paused @ RepoBountyError::ProgramPaused,
    )]
    pub config: Account<'info, Config>,

    #[account(
        init,
        payer = sponsor,
        space = Campaign::SPACE,
        seeds = [b"campaign", sponsor.key().as_ref(), &campaign_id.to_le_bytes()],
        bump,
    )]
    pub campaign: Account<'info, Campaign>,

    /// CHECK: Escrow PDA — system-owned, simply holds SOL
    #[account(
        mut,
        seeds = [b"escrow", campaign.key().as_ref()],
        bump,
    )]
    pub escrow: UncheckedAccount<'info>,

    /// CHECK: treasury receives service fee
    #[account(
        mut,
        constraint = treasury_wallet.key() == config.treasury_wallet @ RepoBountyError::Unauthorized,
    )]
    pub treasury_wallet: UncheckedAccount<'info>,

    pub system_program: Program<'info, System>,
}

pub fn handler(
    ctx: Context<CreateCampaignWithDeposit>,
    campaign_id: u64,
    github_repo_id: u64,
    deadline_at: i64,
    reward_amount: u64,
) -> Result<()> {
    require!(
        reward_amount >= MIN_CAMPAIGN_AMOUNT,
        RepoBountyError::InvalidCampaignAmount
    );

    let clock = Clock::get()?;
    let now = clock.unix_timestamp;

    require!(
        deadline_at
            >= now
                .checked_add(MIN_DEADLINE_SECONDS)
                .ok_or(RepoBountyError::ArithmeticOverflow)?,
        RepoBountyError::InvalidDeadline
    );
    require!(
        deadline_at
            <= now
                .checked_add(MAX_DEADLINE_SECONDS)
                .ok_or(RepoBountyError::ArithmeticOverflow)?,
        RepoBountyError::InvalidDeadline
    );

    let service_fee = std::cmp::max(
        reward_amount
            .checked_mul(SERVICE_FEE_NUMERATOR)
            .ok_or(RepoBountyError::ArithmeticOverflow)?
            .checked_div(SERVICE_FEE_DENOMINATOR)
            .ok_or(RepoBountyError::ArithmeticOverflow)?,
        MIN_SERVICE_FEE,
    );

    let campaign_key = ctx.accounts.campaign.key();
    let escrow_bump = ctx.bumps.escrow;

    let signer_seeds: &[&[&[u8]]] = &[&[b"escrow", campaign_key.as_ref(), &[escrow_bump]]];

    transfer(
        CpiContext::new_with_signer(
            ctx.accounts.system_program.to_account_info(),
            Transfer {
                from: ctx.accounts.sponsor.to_account_info(),
                to: ctx.accounts.escrow.to_account_info(),
            },
            signer_seeds,
        ),
        reward_amount,
    )?;

    transfer(
        CpiContext::new(
            ctx.accounts.system_program.to_account_info(),
            Transfer {
                from: ctx.accounts.sponsor.to_account_info(),
                to: ctx.accounts.treasury_wallet.to_account_info(),
            },
        ),
        service_fee,
    )?;

    let campaign = &mut ctx.accounts.campaign;
    campaign.version = VERSION;
    campaign.campaign_id = campaign_id;
    campaign.sponsor = ctx.accounts.sponsor.key();
    campaign.github_repo_id = github_repo_id;
    campaign.created_at = now;
    campaign.deadline_at = deadline_at;
    campaign.claim_deadline_at = deadline_at
        .checked_add(CLAIM_WINDOW_SECONDS)
        .ok_or(RepoBountyError::ArithmeticOverflow)?;
    campaign.total_reward_amount = reward_amount;
    campaign.allocated_amount = 0;
    campaign.claimed_amount = 0;
    campaign.allocations_count = 0;
    campaign.claimed_count = 0;
    campaign.status = STATUS_ACTIVE;
    campaign.bump = ctx.bumps.campaign;
    campaign._reserved = [0; 64];

    emit!(CampaignCreated {
        campaign_pubkey: campaign.key(),
        campaign_id,
        sponsor: ctx.accounts.sponsor.key(),
        github_repo_id,
        deadline_at,
        total_reward_amount: reward_amount,
        service_fee,
    });

    Ok(())
}
