use anchor_lang::prelude::*;

use crate::constants::SEED_CONFIG;
use crate::state::Config;

#[derive(Accounts)]
pub struct InitializeConfig<'info> {
    #[account(
        init,
        payer = admin,
        space = Config::SPACE,
        seeds = [SEED_CONFIG],
        bump,
    )]
    pub config: Account<'info, Config>,

    #[account(mut)]
    pub admin: Signer<'info>,

    pub system_program: Program<'info, System>,
}

pub fn handler(
    ctx: Context<InitializeConfig>,
    finalize_authority: Pubkey,
    claim_authority: Pubkey,
) -> Result<()> {
    let config = &mut ctx.accounts.config;
    config.admin = ctx.accounts.admin.key();
    config.finalize_authority = finalize_authority;
    config.claim_authority = claim_authority;
    config.paused = false;
    config.bump = ctx.bumps.config;
    Ok(())
}
