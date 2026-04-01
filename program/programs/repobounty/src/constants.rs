/// PDA seed for the global Config account.
pub const SEED_CONFIG: &[u8] = b"config";

/// PDA seed prefix for Campaign accounts.
pub const SEED_CAMPAIGN: &[u8] = b"campaign";

/// PDA seed prefix for the escrow authority PDA.
pub const SEED_ESCROW_AUTHORITY: &[u8] = b"escrow_authority";

/// PDA seed prefix for ClaimRecord accounts.
pub const SEED_CLAIM: &[u8] = b"claim";

/// Minimum deadline: 24 hours from campaign creation.
pub const MIN_DEADLINE_SECONDS: i64 = 24 * 60 * 60;

/// Maximum deadline: 365 days from campaign creation.
pub const MAX_DEADLINE_SECONDS: i64 = 365 * 24 * 60 * 60;

/// Claim window duration: 365 days after finalization deadline.
pub const CLAIM_WINDOW_SECONDS: i64 = 365 * 24 * 60 * 60;

/// Maximum length for GitHub repository owner (org or user name).
pub const MAX_REPO_OWNER_LEN: usize = 39;

/// Maximum length for GitHub repository name.
pub const MAX_REPO_NAME_LEN: usize = 100;

/// Maximum length for GitHub username (stored in ClaimRecord).
pub const MAX_GITHUB_USERNAME_LEN: usize = 39;

/// Maximum allocations per finalize_campaign transaction (Solana tx size limit).
pub const MAX_ALLOCATIONS_PER_BATCH: usize = 5;
