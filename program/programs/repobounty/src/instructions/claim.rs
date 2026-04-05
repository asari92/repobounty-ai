use anchor_lang::prelude::*;
use anchor_lang::system_program::{transfer, Transfer};

use crate::constants::*;
use crate::errors::RepoBountyError;
use crate::events::{CampaignClosed, ClaimProcessed};
use crate::state::{Campaign, ClaimRecord, Config};

#[derive(Accounts)]
#[instruction(github_user_id: u64, payer_mode: u8)]
pub struct Claim<'info> {
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
    /// CHECK: Escrow PDA only holds SOL for this campaign and is constrained by PDA seeds.
    pub escrow: UncheckedAccount<'info>,

    #[account(
        mut,
        constraint = recipient_wallet.key() == user.key() @ RepoBountyError::InvalidRecipientWallet,
    )]
    pub recipient_wallet: SystemAccount<'info>,

    pub system_program: Program<'info, System>,
}

fn is_valid_payer_mode(payer_mode: u8) -> bool {
    payer_mode == PAYER_MODE_USER_PAID || payer_mode == PAYER_MODE_BACKEND_PAID
}

fn should_close_campaign_after_claim(
    claimed_amount: u64,
    total_reward_amount: u64,
    claimed_count: u32,
    allocations_count: u32,
    escrow_balance: u64,
) -> bool {
    claimed_amount == total_reward_amount
        || claimed_count == allocations_count
        || escrow_balance == 0
}

pub fn handler(ctx: Context<Claim>, github_user_id: u64, payer_mode: u8) -> Result<()> {
    require!(
        is_valid_payer_mode(payer_mode),
        RepoBountyError::InvalidPayerMode
    );

    let clock = Clock::get()?;
    let claim_deadline_at = ctx.accounts.campaign.claim_deadline_at;

    require!(
        clock.unix_timestamp <= claim_deadline_at,
        RepoBountyError::ClaimWindowExpired
    );

    let amount = ctx.accounts.claim_record.amount;
    require!(
        ctx.accounts.escrow.lamports() >= amount,
        RepoBountyError::EscrowInsufficientFunds
    );

    let campaign_key = ctx.accounts.campaign.key();
    let escrow_bump = ctx.bumps.escrow;

    let signer_seeds: &[&[&[u8]]] = &[&[b"escrow", campaign_key.as_ref(), &[escrow_bump]]];

    transfer(
        CpiContext::new_with_signer(
            ctx.accounts.system_program.to_account_info(),
            Transfer {
                from: ctx.accounts.escrow.to_account_info(),
                to: ctx.accounts.recipient_wallet.to_account_info(),
            },
            signer_seeds,
        ),
        amount,
    )?;

    let claim_record = &mut ctx.accounts.claim_record;
    claim_record.claimed = true;
    claim_record.claimed_to_wallet = Some(ctx.accounts.recipient_wallet.key());
    claim_record.claimed_at = Some(clock.unix_timestamp);

    let campaign = &mut ctx.accounts.campaign;
    campaign.claimed_amount = campaign
        .claimed_amount
        .checked_add(amount)
        .ok_or(RepoBountyError::ArithmeticOverflow)?;
    campaign.claimed_count = campaign
        .claimed_count
        .checked_add(1)
        .ok_or(RepoBountyError::ArithmeticOverflow)?;

    emit!(ClaimProcessed {
        campaign_pubkey: campaign.key(),
        github_user_id,
        recipient_wallet: ctx.accounts.recipient_wallet.key(),
        amount,
        payer_mode,
    });

    if should_close_campaign_after_claim(
        campaign.claimed_amount,
        campaign.total_reward_amount,
        campaign.claimed_count,
        campaign.allocations_count,
        ctx.accounts.escrow.lamports(),
    ) {
        campaign.status = STATUS_CLOSED;

        emit!(CampaignClosed {
            campaign_pubkey: campaign.key(),
            reason: "claim_completed".to_string(),
        });
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn payer_mode_accepts_user_paid() {
        assert!(is_valid_payer_mode(0));
    }

    #[test]
    fn payer_mode_accepts_backend_paid() {
        assert!(is_valid_payer_mode(1));
    }

    #[test]
    fn payer_mode_rejects_unknown_values() {
        assert!(!is_valid_payer_mode(2));
        assert!(!is_valid_payer_mode(u8::MAX));
    }

    #[test]
    fn final_claim_close_when_total_reward_claimed() {
        assert!(should_close_campaign_after_claim(100, 100, 1, 3, 40));
    }

    #[test]
    fn final_claim_close_when_all_claim_records_claimed() {
        assert!(should_close_campaign_after_claim(50, 100, 3, 3, 50));
    }

    #[test]
    fn final_claim_close_when_escrow_balance_is_zero() {
        assert!(should_close_campaign_after_claim(50, 100, 1, 3, 0));
    }

    #[test]
    fn claim_keeps_campaign_finalized_when_no_close_condition_matches() {
        assert!(!should_close_campaign_after_claim(50, 100, 1, 3, 50));
    }
}
