use anchor_lang::prelude::*;

use crate::constants::VERSION;
use crate::state::Config;

#[derive(Accounts)]
pub struct InitializeConfig<'info> {
    #[account(mut)]
    pub admin_wallet: Signer<'info>,
    #[account(
        init,
        payer = admin_wallet,
        space = Config::SPACE,
        seeds = [b"config"],
        bump
    )]
    pub config: Account<'info, Config>,
    pub system_program: Program<'info, System>,
}

pub fn handler(
    ctx: Context<InitializeConfig>,
    finalize_authority: Pubkey,
    claim_authority: Pubkey,
    treasury_wallet: Pubkey,
) -> Result<()> {
    let config = &mut ctx.accounts.config;

    config.version = VERSION;
    config.admin_wallet = ctx.accounts.admin_wallet.key();
    config.finalize_authority = finalize_authority;
    config.claim_authority = claim_authority;
    config.treasury_wallet = treasury_wallet;
    config.paused = false;
    config.bump = ctx.bumps.config;
    config._reserved = [0; 64];

    Ok(())
}
