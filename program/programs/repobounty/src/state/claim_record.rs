use anchor_lang::prelude::*;

use crate::constants::MAX_GITHUB_USERNAME_LEN;

/// Record of a contributor's allocation and claim status within a campaign.
///
/// PDA seeds: `["claim", campaign, github_user_id.to_le_bytes()]`
#[account]
pub struct ClaimRecord {
    /// The campaign this allocation belongs to.
    pub campaign: Pubkey,

    /// GitHub numeric user ID (stable across username changes).
    pub github_user_id: u64,

    /// GitHub username at time of finalization (for display).
    pub github_username: String,

    /// Allocated reward amount in smallest token units.
    pub amount: u64,

    /// Whether this allocation has been claimed.
    pub claimed: bool,

    /// Wallet that received the claimed tokens (None until claimed).
    pub claimed_to_wallet: Option<Pubkey>,

    /// Unix timestamp of the claim (None until claimed).
    pub claimed_at: Option<i64>,

    /// PDA bump seed.
    pub bump: u8,
}

impl ClaimRecord {
    pub const SPACE: usize = 8  // discriminator
        + 32                    // campaign
        + 8                     // github_user_id
        + (4 + MAX_GITHUB_USERNAME_LEN) // github_username (String)
        + 8                     // amount
        + 1                     // claimed
        + (1 + 32)              // claimed_to_wallet (Option<Pubkey>)
        + (1 + 8)               // claimed_at (Option<i64>)
        + 1;                    // bump
}
