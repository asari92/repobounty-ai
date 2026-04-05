use anchor_lang::prelude::*;
use anchor_lang::system_program::{transfer, Transfer};

use crate::constants::*;
use crate::errors::RepoBountyError;
use crate::events::ClaimProcessed;
use crate::state::{Campaign, ClaimRecord, Config};

#[derive(Accounts)]
#[instruction(github_user_id: u64)]
pub struct Claim<'info> {
    #[account(mut)]
    pub user: Signer<'info>,

    pub claim_authority: Signer<'info>,

    #[account(
        seeds = [b"config"],
        bump = config.bump,
        constraint = !config.paused @ RepoBountyError::ProgramPaused,
        constraint = config.claim_authority == claim_authority.key() @ RepoBountyError::Unauthorized,
    )]
    pub config: Account<'info, Config>,

    #[account(
        mut,
        constraint = campaign.status == STATUS_FINALIZED @ RepoBountyError::CampaignNotFinalized,
    )]
    pub campaign: Account<'info, Campaign>,

    #[account(
        mut,
        seeds = [b"claim", campaign.key().as_ref(), &github_user_id.to_le_bytes()],
        bump = claim_record.bump,
        constraint = claim_record.campaign == campaign.key() @ RepoBountyError::ClaimNotFound,
        constraint = claim_record.github_user_id == github_user_id @ RepoBountyError::ClaimNotFound,
        constraint = !claim_record.claimed @ RepoBountyError::ClaimAlreadyClaimed,
        constraint = claim_record.amount > 0 @ RepoBountyError::ClaimNotFound,
    )]
    pub claim_record: Account<'info, ClaimRecord>,

    #[account(
        mut,
        seeds = [b"escrow", campaign.key().as_ref()],
        bump,
    )]
    pub escrow: UncheckedAccount<'info>,

    pub system_program: Program<'info, System>,
}

pub fn handler(ctx: Context<Claim>, github_user_id: u64) -> Result<()> {
    let clock = Clock::get()?;
    let claim_deadline_at = ctx.accounts.campaign.claim_deadline_at;

    require!(
        clock.unix_timestamp <= claim_deadline_at,
        RepoBountyError::ClaimWindowExpired
    );

    let amount = ctx.accounts.claim_record.amount;

    let campaign_key = ctx.accounts.campaign.key();
    let escrow_bump = ctx.bumps.escrow;

    let signer_seeds: &[&[&[u8]]] = &[&[b"escrow", campaign_key.as_ref(), &[escrow_bump]]];

    transfer(
        CpiContext::new_with_signer(
            ctx.accounts.system_program.to_account_info(),
            Transfer {
                from: ctx.accounts.escrow.to_account_info(),
                to: ctx.accounts.user.to_account_info(),
            },
            signer_seeds,
        ),
        amount,
    )?;

    let claim_record = &mut ctx.accounts.claim_record;
    claim_record.claimed = true;
    claim_record.claimed_to_wallet = Some(ctx.accounts.user.key());
    claim_record.claimed_at = Some(clock.unix_timestamp);

    let campaign = &mut ctx.accounts.campaign;
    campaign.claimed_amount += amount;
    campaign.claimed_count += 1;

    emit!(ClaimProcessed {
        campaign_pubkey: campaign.key(),
        github_user_id,
        recipient_wallet: ctx.accounts.user.key(),
        amount,
    });

    if campaign.claimed_amount == campaign.total_reward_amount
        || campaign.claimed_count == campaign.allocations_count
        || ctx.accounts.escrow.lamports() == 0
    {
        campaign.status = STATUS_CLOSED;
    }

    Ok(())
}
