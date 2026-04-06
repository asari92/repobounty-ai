use anchor_lang::prelude::*;

#[error_code]
pub enum RepoBountyError {
    #[msg("Program is paused")]
    ProgramPaused,

    #[msg("Invalid deadline")]
    InvalidDeadline,

    #[msg("Campaign amount below minimum (0.5 SOL)")]
    InvalidCampaignAmount,

    #[msg("Campaign already exists")]
    CampaignAlreadyExists,

    #[msg("Campaign is not in Active status")]
    CampaignNotActive,

    #[msg("Campaign is not in Finalized status")]
    CampaignNotFinalized,

    #[msg("Campaign is closed")]
    CampaignClosed,

    #[msg("Deadline not reached yet")]
    DeadlineNotReached,

    #[msg("Claim window has expired")]
    ClaimWindowExpired,

    #[msg("Claim deadline not reached yet (for refund)")]
    ClaimDeadlineNotReached,

    #[msg("Unauthorized")]
    Unauthorized,

    #[msg("Allocations list must not be empty")]
    EmptyAllocations,

    #[msg("Duplicate allocation in batch")]
    DuplicateAllocation,

    #[msg("Allocation amount too small (min 0.05 SOL)")]
    AllocationTooSmall,

    #[msg("Allocated amount would exceed total reward")]
    AllocationOverflow,

    #[msg("Final batch: allocated != total_reward_amount")]
    AllocationTotalMismatch,

    #[msg("ClaimRecord already exists")]
    ClaimRecordAlreadyExists,

    #[msg("Claim record not found")]
    ClaimNotFound,

    #[msg("Reward already claimed")]
    ClaimAlreadyClaimed,

    #[msg("Escrow has insufficient funds")]
    EscrowInsufficientFunds,

    #[msg("Escrow is empty")]
    EscrowEmpty,

    #[msg("Invalid sponsor")]
    InvalidSponsor,

    #[msg("Invalid recipient wallet")]
    InvalidRecipientWallet,

    #[msg("Invalid payer mode")]
    InvalidPayerMode,

    #[msg("Cannot close: partial finalization exists")]
    PartialFinalizationExists,

    #[msg("Invalid github user ID")]
    InvalidGithubUserId,

    #[msg("Arithmetic overflow")]
    ArithmeticOverflow,
}
