use anchor_lang::prelude::*;

use crate::errors::RepoBountyError;
use crate::state::Config;

#[derive(Accounts)]
pub struct UpdateConfig<'info> {
    #[account(mut)]
    pub admin_wallet: Signer<'info>,
    #[account(
        mut,
        seeds = [b"config"],
        bump = config.bump,
        constraint = config.admin_wallet == admin_wallet.key() @ RepoBountyError::Unauthorized,
    )]
    pub config: Account<'info, Config>,
}

pub fn handler(
    ctx: Context<UpdateConfig>,
    finalize_authority: Option<Pubkey>,
    claim_authority: Option<Pubkey>,
    treasury_wallet: Option<Pubkey>,
) -> Result<()> {
    let config = &mut ctx.accounts.config;

    if let Some(v) = finalize_authority {
        config.finalize_authority = v;
    }
    if let Some(v) = claim_authority {
        config.claim_authority = v;
    }
    if let Some(v) = treasury_wallet {
        config.treasury_wallet = v;
    }

    Ok(())
}
