use anchor_lang::prelude::*;

use crate::constants::{MAX_REPO_NAME_LEN, MAX_REPO_OWNER_LEN};

/// Campaign lifecycle status.
///
/// Transitions: Active → Finalizing → Finalized → Closed
#[derive(AnchorSerialize, AnchorDeserialize, Clone, Copy, PartialEq, Eq, Debug)]
#[repr(u8)]
pub enum CampaignStatus {
    /// Campaign created, funds in escrow, awaiting deadline.
    Active = 0,
    /// Batched finalization in progress (at least one batch submitted).
    Finalizing = 1,
    /// All allocations recorded, contributors may claim.
    Finalized = 2,
    /// All claims collected or refund executed.
    Closed = 3,
}

/// On-chain campaign state.
///
/// PDA seeds: `["campaign", sponsor, campaign_id.to_le_bytes()]`
#[account]
pub struct Campaign {
    /// Unique campaign identifier (backend-generated).
    pub campaign_id: u64,

    /// Wallet that created the campaign and deposited funds.
    pub sponsor: Pubkey,

    /// SPL token mint for the reward pool.
    pub token_mint: Pubkey,

    /// Token account holding escrowed funds (owned by escrow authority PDA).
    pub escrow_token_account: Pubkey,

    /// GitHub repository numeric ID (stable across renames).
    pub github_repo_id: u64,

    /// GitHub repository owner (org or user), max 39 chars.
    pub repo_owner: String,

    /// GitHub repository name, max 100 chars.
    pub repo_name: String,

    /// Unix timestamp when the campaign was created.
    pub created_at: i64,

    /// Unix timestamp after which finalization is allowed.
    pub deadline_at: i64,

    /// Unix timestamp after which unclaimed funds can be refunded.
    /// Set to `deadline_at + 365 days` at creation.
    pub claim_deadline_at: i64,

    /// Total reward pool in smallest token units (e.g. lamports).
    pub total_amount: u64,

    /// Sum of all allocated amounts across finalize batches.
    /// Must equal `total_amount` after final batch.
    pub allocated_amount: u64,

    /// Sum of all claimed amounts.
    pub claimed_amount: u64,

    /// Number of ClaimRecord accounts created.
    pub allocations_count: u32,

    /// Number of successful claims.
    pub claimed_count: u32,

    /// Current lifecycle status.
    pub status: CampaignStatus,

    /// PDA bump seed.
    pub bump: u8,

    /// Bump seed for the escrow authority PDA (used for signing transfers).
    pub escrow_authority_bump: u8,
}

impl Campaign {
    pub const SPACE: usize = 8  // discriminator
        + 8                     // campaign_id
        + 32                    // sponsor
        + 32                    // token_mint
        + 32                    // escrow_token_account
        + 8                     // github_repo_id
        + (4 + MAX_REPO_OWNER_LEN)  // repo_owner (String)
        + (4 + MAX_REPO_NAME_LEN)   // repo_name (String)
        + 8                     // created_at
        + 8                     // deadline_at
        + 8                     // claim_deadline_at
        + 8                     // total_amount
        + 8                     // allocated_amount
        + 8                     // claimed_amount
        + 4                     // allocations_count
        + 4                     // claimed_count
        + 1                     // status
        + 1                     // bump
        + 1;                    // escrow_authority_bump
}
