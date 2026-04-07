package models

import "time"

// CampaignState type for backward compatibility and V2.
type CampaignState = string

const (
	// V2 statuses (primary)
	StateActive    CampaignState = "active"
	StateFinalized CampaignState = "finalized"
	StateClosed    CampaignState = "closed"

	// V1 compat aliases
	StateCreated   CampaignState = "created"
	StateFunded    CampaignState = "funded"
	StateCompleted CampaignState = "completed"
)

type AllocationMode = string

const (
	AllocationModeCodeImpact AllocationMode = "code_impact"
	AllocationModeMetrics    AllocationMode = "metrics"
)

// Campaign holds both V2 (native SOL) and V1 (compat) fields.
// V1 fields are kept for backward compat with existing handlers/store.
// During the migration, V2 fields will gradually replace V1 fields.
type Campaign struct {
	// --- V2 fields ---
	CampaignPDA      string     `json:"campaign_pda"`
	EscrowPDA        string     `json:"escrow_pda"`
	GithubRepoID     uint64     `json:"github_repo_id"`
	RepoOwner        string     `json:"repo_owner"`
	RepoName         string     `json:"repo_name"`
	RepoURL          string     `json:"repo_url"`
	DeadlineAt       time.Time  `json:"deadline_at"`
	ClaimDeadlineAt  time.Time  `json:"claim_deadline_at"`
	ServiceFee       uint64     `json:"service_fee"`
	AllocatedAmount  uint64     `json:"allocated_amount"`
	ClaimedAmount    uint64     `json:"claimed_amount"`
	AllocationsCount uint32     `json:"allocations_count"`
	ClaimedCount     uint32     `json:"claimed_count"`
	Status           string     `json:"status"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`
	CloseReason      string     `json:"close_reason,omitempty"`

	// --- V1 compat fields (used by store, handlers, solana client) ---
	CampaignID          string        `json:"campaign_id"`
	Repo                string        `json:"repo"`
	PoolAmount          uint64        `json:"pool_amount"`
	TotalRewardAmount   uint64        `json:"total_reward_amount"`
	VaultAddress        string        `json:"vault_address"`
	State               CampaignState `json:"state"`
	Authority           string        `json:"authority"`
	Sponsor             string        `json:"sponsor"`
	Deadline            time.Time     `json:"deadline"`
	TotalClaimed        uint64        `json:"total_claimed"`
	OwnerGitHubUsername string        `json:"owner_github_username"`
	Allocations         []Allocation  `json:"allocations"`
	CreatedAt           time.Time     `json:"created_at"`
	FinalizedAt         *time.Time    `json:"finalized_at,omitempty"`
	TxSignature         string        `json:"tx_signature,omitempty"`
}

// Allocation holds both V2 and V1 fields.
type Allocation struct {
	// V2 fields
	CampaignID     uint64     `json:"campaign_id,omitempty"`
	GithubUserID   uint64     `json:"github_user_id,omitempty"`
	GithubUsername string     `json:"github_username,omitempty"`
	ImpactScore    float64    `json:"impact_score,omitempty"`
	ClaimedWallet  string     `json:"claimed_wallet,omitempty"`
	ClaimedAt      *time.Time `json:"claimed_at,omitempty"`
	ClaimRecordPDA string     `json:"claim_record_pda,omitempty"`

	// V1 compat fields (used by AI allocator, handlers, normalize)
	Contributor    string `json:"contributor"`
	Percentage     uint16 `json:"percentage"`
	Amount         uint64 `json:"amount"`
	Reasoning      string `json:"reasoning,omitempty"`
	Claimed        bool   `json:"claimed"`
	ClaimantWallet string `json:"claimant_wallet,omitempty"`
}

type FinalizeBatch struct {
	ID           uint64            `json:"id"`
	CampaignID   uint64            `json:"campaign_id"`
	BatchIndex   int               `json:"batch_index"`
	TotalBatches int               `json:"total_batches"`
	Allocations  []AllocationEntry `json:"allocations"`
	HasMore      bool              `json:"has_more"`
	Status       string            `json:"status"`
	TxSignature  string            `json:"tx_signature,omitempty"`
	ErrorMessage string            `json:"error_message,omitempty"`
	RetryCount   int               `json:"retry_count"`
	CreatedAt    time.Time         `json:"created_at"`
	SentAt       *time.Time        `json:"sent_at,omitempty"`
	ConfirmedAt  *time.Time        `json:"confirmed_at,omitempty"`
}

type AllocationEntry struct {
	GithubUserID uint64 `json:"github_user_id"`
	Amount       uint64 `json:"amount"`
}

type User struct {
	GitHubUsername string    `json:"github_username"`
	GitHubID       int       `json:"github_id"`
	AvatarURL      string    `json:"avatar_url"`
	WalletAddress  string    `json:"wallet_address"`
	CreatedAt      time.Time `json:"created_at"`
}

type Contributor struct {
	GithubUserID uint64 `json:"github_user_id,omitempty"`
	Username     string `json:"username"`
	Commits      int    `json:"commits"`
	PullRequests int    `json:"pull_requests"`
	Reviews      int    `json:"reviews"`
	LinesAdded   int    `json:"lines_added"`
	LinesDeleted int    `json:"lines_deleted"`
}

type ContributorImpact struct {
	GithubUserID     uint64  `json:"github_user_id"`
	GithubUsername   string  `json:"github_username"`
	ImpactPercentage float64 `json:"impact_percentage"`
}

type CreateCampaignRequest struct {
	Repo          string `json:"repo"`
	PoolAmount    uint64 `json:"pool_amount"`
	Deadline      string `json:"deadline"`
	SponsorWallet string `json:"sponsor_wallet"`
}

type CreateCampaignResponse struct {
	CampaignID   string        `json:"campaign_id"`
	CampaignPDA  string        `json:"campaign_pda"`
	EscrowPDA    string        `json:"escrow_pda,omitempty"`
	VaultAddress string        `json:"vault_address,omitempty"`
	Repo         string        `json:"repo"`
	PoolAmount   uint64        `json:"pool_amount"`
	Deadline     string        `json:"deadline"`
	State        CampaignState `json:"state,omitempty"`
	Status       string        `json:"status,omitempty"`
	ServiceFee   uint64        `json:"service_fee,omitempty"`
	TxSignature  string        `json:"tx_signature,omitempty"`
	UnsignedTx   string        `json:"unsigned_tx,omitempty"`
}

type CreateCampaignConfirmRequest struct {
	Repo          string `json:"repo"`
	PoolAmount    uint64 `json:"pool_amount"`
	Deadline      string `json:"deadline"`
	SponsorWallet string `json:"sponsor_wallet"`
	TxSignature   string `json:"tx_signature"`
}

type FinalizePreviewResponse struct {
	CampaignID     string          `json:"campaign_id"`
	Repo           string          `json:"repo"`
	Contributors   []Contributor   `json:"contributors"`
	Allocations    interface{}     `json:"allocations"`
	AIModel        string          `json:"ai_model"`
	AllocationMode AllocationMode  `json:"allocation_mode,omitempty"`
	Snapshot       SnapshotSummary `json:"snapshot,omitempty"`
}

type FinalizeResponse struct {
	CampaignID        string           `json:"campaign_id"`
	State             CampaignState    `json:"state,omitempty"`
	Status            string           `json:"status,omitempty"`
	Allocations       interface{}      `json:"allocations"`
	TxSignature       string           `json:"tx_signature,omitempty"`
	TxSignatures      []string         `json:"tx_signatures,omitempty"`
	TotalBatches      int              `json:"total_batches,omitempty"`
	SolanaExplorerURL string           `json:"solana_explorer_url,omitempty"`
	AllocationMode    AllocationMode   `json:"allocation_mode,omitempty"`
	Snapshot          *SnapshotSummary `json:"snapshot,omitempty"`
}

type BuildClaimTxResponse struct {
	PartialTx string `json:"partial_tx"`
}

type RefundBuildRequest struct {
	SponsorWallet string `json:"sponsor_wallet"`
}

type RefundBuildResponse struct {
	PartialTx string `json:"partial_tx"`
}

type RefundConfirmRequest struct {
	SponsorWallet string `json:"sponsor_wallet"`
	TxSignature   string `json:"tx_signature,omitempty"`
}

type ClaimConfirmRequest struct {
	ContributorGithub string `json:"contributor_github"`
	WalletAddress     string `json:"wallet_address"`
	TxSignature       string `json:"tx_signature,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type GitHubAuthRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type GitHubAuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type LinkWalletRequest struct {
	WalletAddress string `json:"wallet_address"`
}

type ClaimAllocationRequest struct {
	GithubUsername    string `json:"github_username"`
	ContributorGithub string `json:"contributor_github"`
	WalletAddress     string `json:"wallet_address"`
	ChallengeID       string `json:"challenge_id"`
	Signature         string `json:"signature"`
}

type SnapshotData struct {
	TotalContributors int    `json:"total_contributors"`
	TotalCommits      int    `json:"total_commits"`
	TotalPRs          int    `json:"total_prs"`
	SnapshotTimestamp string `json:"snapshot_timestamp"`
}
