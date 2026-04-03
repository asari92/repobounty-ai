use anchor_lang::prelude::*;

use crate::constants::SEED_CONFIG;
use crate::errors::RepoBountyError;
use crate::state::Config;

#[derive(Accounts)]
pub struct UpdateConfig<'info> {
    #[account(
        mut,
        seeds = [SEED_CONFIG],
        bump = config.bump,
        has_one = admin @ RepoBountyError::Unauthorized,
    )]
    pub config: Account<'info, Config>,

    pub admin: Signer<'info>,
}

pub fn handler(
    ctx: Context<UpdateConfig>,
    new_admin: Option<Pubkey>,
    new_finalize_authority: Option<Pubkey>,
    new_claim_authority: Option<Pubkey>,
    paused: Option<bool>,
) -> Result<()> {
    let config = &mut ctx.accounts.config;

    if let Some(v) = new_admin {
        config.admin = v;
    }
    if let Some(v) = new_finalize_authority {
        config.finalize_authority = v;
    }
    if let Some(v) = new_claim_authority {
        config.claim_authority = v;
    }
    if let Some(v) = paused {
        config.paused = v;
    }

    Ok(())
}
