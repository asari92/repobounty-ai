package models

import "time"

type WalletChallengeAction string

const (
	WalletChallengeActionClaim WalletChallengeAction = "claim"
)

type WalletChallenge struct {
	ChallengeID   string                `json:"challenge_id"`
	Action        WalletChallengeAction `json:"action"`
	WalletAddress string                `json:"wallet_address"`
	Message       string                `json:"message"`
	PayloadJSON   string                `json:"payload_json"`
	CreatedAt     time.Time             `json:"created_at"`
	ExpiresAt     time.Time             `json:"expires_at"`
	UsedAt        *time.Time            `json:"used_at,omitempty"`
}

type ClaimChallengeRequest struct {
	ContributorGithub string `json:"contributor_github"`
	WalletAddress     string `json:"wallet_address"`
}

type WalletChallengeResponse struct {
	ChallengeID   string                `json:"challenge_id"`
	Action        WalletChallengeAction `json:"action"`
	WalletAddress string                `json:"wallet_address"`
	Message       string                `json:"message"`
	ExpiresAt     time.Time             `json:"expires_at"`
}
