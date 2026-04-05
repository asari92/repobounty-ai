use anchor_lang::prelude::*;

use crate::errors::RepoBountyError;
use crate::state::Config;

#[derive(Accounts)]
pub struct SetPaused<'info> {
    pub admin_wallet: Signer<'info>,
    #[account(
        mut,
        seeds = [b"config"],
        bump = config.bump,
        constraint = config.admin_wallet == admin_wallet.key() @ RepoBountyError::Unauthorized,
    )]
    pub config: Account<'info, Config>,
}

pub fn handler(ctx: Context<SetPaused>, paused: bool) -> Result<()> {
    ctx.accounts.config.paused = paused;
    Ok(())
}
