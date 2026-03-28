package models

import "time"

type CampaignState string

const (
	StateCreated   CampaignState = "created"
	StateFinalized CampaignState = "finalized"
)

type Campaign struct {
	CampaignID  string        `json:"campaign_id"`
	Repo        string        `json:"repo"`
	PoolAmount  uint64        `json:"pool_amount"`
	Deadline    time.Time     `json:"deadline"`
	State       CampaignState `json:"state"`
	Authority   string        `json:"authority"`
	Allocations []Allocation  `json:"allocations"`
	CreatedAt   time.Time     `json:"created_at"`
	FinalizedAt *time.Time    `json:"finalized_at,omitempty"`
	TxSignature string        `json:"tx_signature,omitempty"`
}

type Allocation struct {
	Contributor string `json:"contributor"`
	Percentage  uint16 `json:"percentage"`
	Amount      uint64 `json:"amount"`
	Reasoning   string `json:"reasoning,omitempty"`
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
	WalletAddress string `json:"wallet_address"`
}

type CreateCampaignResponse struct {
	CampaignID  string        `json:"campaign_id"`
	Repo        string        `json:"repo"`
	PoolAmount  uint64        `json:"pool_amount"`
	Deadline    string        `json:"deadline"`
	State       CampaignState `json:"state"`
	TxSignature string        `json:"tx_signature"`
}

type FinalizePreviewResponse struct {
	CampaignID   string        `json:"campaign_id"`
	Repo         string        `json:"repo"`
	Contributors []Contributor `json:"contributors"`
	Allocations  []Allocation  `json:"allocations"`
	AIModel      string        `json:"ai_model"`
}

type FinalizeResponse struct {
	CampaignID        string        `json:"campaign_id"`
	State             CampaignState `json:"state"`
	Allocations       []Allocation  `json:"allocations"`
	TxSignature       string        `json:"tx_signature"`
	SolanaExplorerURL string        `json:"solana_explorer_url"`
	Warning           string        `json:"warning,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}
