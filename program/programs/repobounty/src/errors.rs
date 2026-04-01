use anchor_lang::prelude::*;

#[error_code]
pub enum RepoBountyError {
    #[msg("Program is paused")]
    ProgramPaused,

    #[msg("Deadline must be at least 24 hours from now")]
    DeadlineTooSoon,

    #[msg("Deadline must be at most 365 days from now")]
    DeadlineTooFar,

    #[msg("Amount must be greater than zero")]
    InvalidAmount,

    #[msg("Campaign is not in Active state")]
    CampaignNotActive,

    #[msg("Campaign is not in Active or Finalizing state")]
    CampaignNotActiveOrFinalizing,

    #[msg("Campaign is not in Finalized state")]
    CampaignNotFinalized,

    #[msg("Campaign is already closed")]
    CampaignClosed,

    #[msg("Campaign deadline has not been reached yet")]
    DeadlineNotReached,

    #[msg("Claim window has expired")]
    ClaimWindowExpired,

    #[msg("Claim window has not expired yet — refund not available")]
    ClaimWindowNotExpired,

    #[msg("Unauthorized signer")]
    Unauthorized,

    #[msg("Duplicate github_user_id in allocations")]
    DuplicateAllocation,

    #[msg("Allocated total does not match campaign total_amount")]
    AllocationTotalMismatch,

    #[msg("Claim has already been claimed")]
    ClaimAlreadyClaimed,

    #[msg("Escrow has insufficient funds")]
    EscrowInsufficientFunds,

    #[msg("Signer does not match campaign sponsor")]
    InvalidSponsor,

    #[msg("ClaimRecord does not belong to this campaign")]
    InvalidClaimRecord,

    #[msg("Repository owner name exceeds maximum length")]
    RepoOwnerTooLong,

    #[msg("Repository name exceeds maximum length")]
    RepoNameTooLong,

    #[msg("Allocations list must not be empty")]
    EmptyAllocations,

    #[msg("Too many allocations in a single batch")]
    TooManyAllocations,

    #[msg("GitHub user ID must be greater than zero")]
    InvalidGithubUserId,

    #[msg("GitHub username exceeds maximum length")]
    GithubUsernameTooLong,

    #[msg("Allocation amount must be greater than zero")]
    ZeroAllocationAmount,

    #[msg("Arithmetic overflow")]
    ArithmeticOverflow,

    #[msg("Escrow account has no remaining balance")]
    EscrowEmpty,
}
