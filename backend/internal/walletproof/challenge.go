package walletproof

import (
	"crypto/ed25519"
	"fmt"
	"strings"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
)

const ChallengeTTL = 10 * time.Minute

type CreateCampaignMessageInput struct {
	ChallengeID    string
	GitHubUsername string
	SponsorWallet  string
	Repo           string
	PoolAmount     uint64
	Deadline       time.Time
	IssuedAt       time.Time
	ExpiresAt      time.Time
}

type ClaimMessageInput struct {
	ChallengeID       string
	GitHubUsername    string
	CampaignID        string
	ContributorGitHub string
	WalletAddress     string
	IssuedAt          time.Time
	ExpiresAt         time.Time
}

func BuildCreateCampaignMessage(input CreateCampaignMessageInput) string {
	lines := []string{
		"RepoBounty AI Wallet Proof",
		"",
		"Action: create_campaign",
		fmt.Sprintf("Challenge ID: %s", input.ChallengeID),
		fmt.Sprintf("GitHub username: @%s", input.GitHubUsername),
		fmt.Sprintf("Sponsor wallet: %s", input.SponsorWallet),
		fmt.Sprintf("Repository: %s", input.Repo),
		fmt.Sprintf("Pool amount (lamports): %d", input.PoolAmount),
		fmt.Sprintf("Deadline (UTC): %s", input.Deadline.UTC().Format(time.RFC3339)),
		fmt.Sprintf("Issued at (UTC): %s", input.IssuedAt.UTC().Format(time.RFC3339)),
		fmt.Sprintf("Expires at (UTC): %s", input.ExpiresAt.UTC().Format(time.RFC3339)),
		"",
		"Only sign this message to prove control of the sponsor wallet for this exact campaign draft.",
	}
	return strings.Join(lines, "\n")
}

func BuildClaimMessage(input ClaimMessageInput) string {
	lines := []string{
		"RepoBounty AI Wallet Proof",
		"",
		"Action: claim",
		fmt.Sprintf("Challenge ID: %s", input.ChallengeID),
		fmt.Sprintf("GitHub username: @%s", input.GitHubUsername),
		fmt.Sprintf("Campaign ID: %s", input.CampaignID),
		fmt.Sprintf("Contributor GitHub: @%s", input.ContributorGitHub),
		fmt.Sprintf("Claim wallet: %s", input.WalletAddress),
		fmt.Sprintf("Issued at (UTC): %s", input.IssuedAt.UTC().Format(time.RFC3339)),
		fmt.Sprintf("Expires at (UTC): %s", input.ExpiresAt.UTC().Format(time.RFC3339)),
		"",
		"Only sign this message to prove control of the wallet that should receive this campaign claim.",
	}
	return strings.Join(lines, "\n")
}

func VerifySignature(walletAddress, message, signatureBase58 string) error {
	publicKey, err := solana.PublicKeyFromBase58(walletAddress)
	if err != nil {
		return fmt.Errorf("parse wallet address: %w", err)
	}

	signature, err := base58.Decode(signatureBase58)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid signature length")
	}

	if !ed25519.Verify(publicKey.Bytes(), []byte(message), signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}
