use anchor_lang::prelude::*;

use crate::constants::SEED_CLAIM;

/// Record of a contributor's allocation and claim status within a campaign.
///
/// PDA seeds: `["claim", campaign, github_user_id.to_le_bytes()]`
#[account]
pub struct ClaimRecord {
    /// The campaign this allocation belongs to.
    pub campaign: Pubkey,

    /// GitHub numeric user ID (stable across username changes).
    pub github_user_id: u64,

    /// Allocated reward amount in lamports.
    pub amount: u64,

    /// Whether this allocation has been claimed.
    pub claimed: bool,

    /// Wallet that received the claimed SOL (None until claimed).
    pub claimed_to_wallet: Option<Pubkey>,

    /// Unix timestamp of the claim (None until claimed).
    pub claimed_at: Option<i64>,

    /// PDA bump seed.
    pub bump: u8,

    /// Reserved for future use.
    pub _reserved: [u8; 32],
}

impl ClaimRecord {
    pub const SPACE: usize = 8  // discriminator
        + 32                    // campaign
        + 8                     // github_user_id
        + 8                     // amount
        + 1                     // claimed
        + (1 + 32)              // claimed_to_wallet (Option<Pubkey>)
        + (1 + 8)               // claimed_at (Option<i64>)
        + 1                     // bump
        + 32; // _reserved

    pub fn seeds() -> &'static [u8] {
        SEED_CLAIM
    }
}
