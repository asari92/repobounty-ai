use anchor_lang::prelude::*;

use crate::constants::SEED_CONFIG;

/// Global program configuration. One per program, created by admin.
///
/// PDA seeds: `["config"]`
#[account]
pub struct Config {
    /// Admin who can update authorities and pause the program.
    pub admin: Pubkey,

    /// Backend wallet authorized to call `finalize_campaign`.
    pub finalize_authority: Pubkey,

    /// Backend wallet authorized to call `claim_backend_paid`
    /// and co-sign `claim_user_paid`.
    pub claim_authority: Pubkey,

    /// When true, most instructions are blocked.
    /// `refund_unclaimed` is always allowed.
    pub paused: bool,

    /// PDA bump seed.
    pub bump: u8,
}

impl Config {
    pub const SPACE: usize = 8  // discriminator
        + 32                    // admin
        + 32                    // finalize_authority
        + 32                    // claim_authority
        + 1                     // paused
        + 1;                    // bump

    pub fn seeds() -> &'static [u8] {
        SEED_CONFIG
    }
}
