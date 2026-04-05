use anchor_lang::prelude::*;

use crate::constants::SEED_CAMPAIGN;

/// On-chain campaign state.
///
/// PDA seeds: `["campaign", sponsor, campaign_id.to_le_bytes()]`
#[account]
pub struct Campaign {
    /// Protocol version.
    pub version: u8,

    /// Unique campaign identifier (backend-generated).
    pub campaign_id: u64,

    /// Wallet that created the campaign and deposited funds.
    pub sponsor: Pubkey,

    /// GitHub repository numeric ID (stable across renames).
    pub github_repo_id: u64,

    /// Unix timestamp when the campaign was created.
    pub created_at: i64,

    /// Unix timestamp after which finalization is allowed.
    pub deadline_at: i64,

    /// Unix timestamp after which unclaimed funds can be refunded.
    /// Set to `deadline_at + 365 days` at creation.
    pub claim_deadline_at: i64,

    /// Total reward pool in lamports.
    pub total_reward_amount: u64,

    /// Sum of all allocated amounts across finalize batches.
    /// Must equal `total_reward_amount` after final batch.
    pub allocated_amount: u64,

    /// Sum of all claimed amounts.
    pub claimed_amount: u64,

    /// Number of ClaimRecord accounts created.
    pub allocations_count: u32,

    /// Number of successful claims.
    pub claimed_count: u32,

    /// Current lifecycle status.
    /// 0 = Active, 1 = Finalized, 2 = Closed
    pub status: u8,

    /// PDA bump seed.
    pub bump: u8,

    /// Reserved for future use.
    pub _reserved: [u8; 64],
}

impl Campaign {
    pub const SPACE: usize = 8  // discriminator
        + 1                     // version
        + 8                     // campaign_id
        + 32                    // sponsor
        + 8                     // github_repo_id
        + 8                     // created_at
        + 8                     // deadline_at
        + 8                     // claim_deadline_at
        + 8                     // total_reward_amount
        + 8                     // allocated_amount
        + 8                     // claimed_amount
        + 4                     // allocations_count
        + 4                     // claimed_count
        + 1                     // status
        + 1                     // bump
        + 64; // _reserved

    pub fn seeds() -> &'static [u8] {
        SEED_CAMPAIGN
    }
}
