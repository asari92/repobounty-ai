/// PDA seed for the global Config account.
pub const SEED_CONFIG: &[u8] = b"config";

/// PDA seed prefix for Campaign accounts.
pub const SEED_CAMPAIGN: &[u8] = b"campaign";

/// PDA seed prefix for Escrow accounts.
pub const SEED_ESCROW: &[u8] = b"escrow";

/// PDA seed prefix for ClaimRecord accounts.
pub const SEED_CLAIM: &[u8] = b"claim";

/// Protocol version.
pub const VERSION: u8 = 1;

/// Minimum campaign amount: 0.5 SOL in lamports.
pub const MIN_CAMPAIGN_AMOUNT: u64 = 500_000_000;

/// Minimum allocation amount: 0.05 SOL in lamports.
pub const MIN_ALLOCATION_AMOUNT: u64 = 50_000_000;

/// Minimum deadline from campaign creation for hackathon MVP: 5 minutes.
pub const MIN_DEADLINE_SECONDS: i64 = 5 * 60;

/// Maximum deadline: 365 days from campaign creation.
pub const MAX_DEADLINE_SECONDS: i64 = 365 * 24 * 60 * 60;

/// Claim window duration: 365 days after finalization deadline.
pub const CLAIM_WINDOW_SECONDS: i64 = 365 * 24 * 60 * 60;

/// Service fee percentage: 0.5% (5/1000).
pub const SERVICE_FEE_DENOMINATOR: u64 = 1000;

/// Service fee percentage: 0.5% (5/1000).
pub const SERVICE_FEE_NUMERATOR: u64 = 5;

/// Minimum service fee: 0.05 SOL in lamports.
pub const MIN_SERVICE_FEE: u64 = 50_000_000;

/// Campaign status constants.
pub const STATUS_ACTIVE: u8 = 0;
pub const STATUS_FINALIZED: u8 = 1;
pub const STATUS_CLOSED: u8 = 2;
