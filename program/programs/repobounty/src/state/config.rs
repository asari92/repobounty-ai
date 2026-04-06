use anchor_lang::prelude::*;

use crate::constants::SEED_CONFIG;

/// Global program configuration. One per program, created by admin.
///
/// PDA seeds: `["config"]`
#[account]
pub struct Config {
    /// Protocol version.
    pub version: u8,

    /// Admin wallet who can update authorities.
    pub admin_wallet: Pubkey,

    /// Backend wallet authorized to call `finalize_campaign_batch`.
    pub finalize_authority: Pubkey,

    /// Backend wallet authorized to co-sign `claim`.
    pub claim_authority: Pubkey,

    /// Treasury wallet that receives service fees.
    pub treasury_wallet: Pubkey,

    /// When true, state-changing instructions are blocked until the admin unpauses the program.
    pub paused: bool,

    /// PDA bump seed.
    pub bump: u8,

    /// Reserved for future use.
    pub _reserved: [u8; 64],
}

impl Config {
    pub const SPACE: usize = 8  // discriminator
        + 1                     // version
        + 32                    // admin_wallet
        + 32                    // finalize_authority
        + 32                    // claim_authority
        + 32                    // treasury_wallet
        + 1                     // paused
        + 1                     // bump
        + 64; // _reserved

    pub fn seeds() -> &'static [u8] {
        SEED_CONFIG
    }
}
