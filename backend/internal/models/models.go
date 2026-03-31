package models

import "time"

type CampaignState string

const (
	StateCreated   CampaignState = "created"
	StateFunded    CampaignState = "funded"
	StateFinalized CampaignState = "finalized"
	StateCompleted CampaignState = "completed"
)

type AllocationMode string

const (
	AllocationModeCodeImpact AllocationMode = "code_impact"
	AllocationModeMetrics    AllocationMode = "metrics"
)

type Campaign struct {
	CampaignID          string        `json:"campaign_id"`
	CampaignPDA         string        `json:"campaign_pda"`
	VaultAddress        string        `json:"vault_address"`
	Repo                string        `json:"repo"`
	PoolAmount          uint64        `json:"pool_amount"`
	TotalClaimed        uint64        `json:"total_claimed"`
	Deadline            time.Time     `json:"deadline"`
	State               CampaignState `json:"state"`
	Authority           string        `json:"authority"`
	Sponsor             string        `json:"sponsor"`
	OwnerGitHubUsername string        `json:"owner_github_username,omitempty"`
	Allocations         []Allocation  `json:"allocations"`
	CreatedAt           time.Time     `json:"created_at"`
	FinalizedAt         *time.Time    `json:"finalized_at,omitempty"`
	TxSignature         string        `json:"tx_signature,omitempty"`
}

type Allocation struct {
	Contributor    string `json:"contributor"`
	Percentage     uint16 `json:"percentage"`
	Amount         uint64 `json:"amount"`
	Reasoning      string `json:"reasoning,omitempty"`
	Claimed        bool   `json:"claimed"`
	ClaimantWallet string `json:"claimant_wallet,omitempty"`
}

type User struct {
	GitHubUsername string    `json:"github_username"`
	GitHubID       int       `json:"github_id"`
	AvatarURL      string    `json:"avatar_url"`
	WalletAddress  string    `json:"wallet_address"`
	CreatedAt      time.Time `json:"created_at"`
}

type Contributor struct {
	Username     string `json:"username"`
	Commits      int    `json:"commits"`
	PullRequests int    `json:"pull_requests"`
	Reviews      int    `json:"reviews"`
	LinesAdded   int    `json:"lines_added"`
	LinesDeleted int    `json:"lines_deleted"`
}

type CreateCampaignRequest struct {
	Repo          string `json:"repo"`
	PoolAmount    uint64 `json:"pool_amount"`
	Deadline      string `json:"deadline"`
	SponsorWallet string `json:"sponsor_wallet"`
	ChallengeID   string `json:"challenge_id"`
	Signature     string `json:"signature"`
}

type CreateCampaignResponse struct {
	CampaignID   string        `json:"campaign_id"`
	CampaignPDA  string        `json:"campaign_pda"`
	VaultAddress string        `json:"vault_address"`
	Repo         string        `json:"repo"`
	PoolAmount   uint64        `json:"pool_amount"`
	Deadline     string        `json:"deadline"`
	State        CampaignState `json:"state"`
	TxSignature  string        `json:"tx_signature"`
}

type FinalizePreviewResponse struct {
	CampaignID     string          `json:"campaign_id"`
	Repo           string          `json:"repo"`
	Contributors   []Contributor   `json:"contributors"`
	Allocations    []Allocation    `json:"allocations"`
	AIModel        string          `json:"ai_model"`
	AllocationMode AllocationMode  `json:"allocation_mode"`
	Snapshot       SnapshotSummary `json:"snapshot"`
}

type FinalizeResponse struct {
	CampaignID        string           `json:"campaign_id"`
	State             CampaignState    `json:"state"`
	Allocations       []Allocation     `json:"allocations"`
	TxSignature       string           `json:"tx_signature"`
	SolanaExplorerURL string           `json:"solana_explorer_url"`
	AllocationMode    AllocationMode   `json:"allocation_mode,omitempty"`
	Snapshot          *SnapshotSummary `json:"snapshot,omitempty"`
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
	ContributorGithub string `json:"contributor_github"`
	WalletAddress     string `json:"wallet_address"`
	ChallengeID       string `json:"challenge_id"`
	Signature         string `json:"signature"`
}
