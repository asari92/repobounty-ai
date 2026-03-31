package walletproof

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
)

func TestVerifySignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	message := BuildCreateCampaignMessage(CreateCampaignMessageInput{
		ChallengeID:    "challenge-1",
		GitHubUsername: "alice",
		SponsorWallet:  solana.PublicKeyFromBytes(publicKey).String(),
		Repo:           "owner/repo",
		PoolAmount:     1_000_000_000,
		Deadline:       time.Unix(1_700_000_000, 0).UTC(),
		IssuedAt:       time.Unix(1_699_000_000, 0).UTC(),
		ExpiresAt:      time.Unix(1_699_000_600, 0).UTC(),
	})

	signature := ed25519.Sign(privateKey, []byte(message))
	signatureBase58 := base58.Encode(signature)

	if err := VerifySignature(solana.PublicKeyFromBytes(publicKey).String(), message, signatureBase58); err != nil {
		t.Fatalf("VerifySignature: %v", err)
	}
}

func TestVerifySignatureRejectsTamperedMessage(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	message := BuildClaimMessage(ClaimMessageInput{
		ChallengeID:       "challenge-2",
		GitHubUsername:    "alice",
		CampaignID:        "camp-1",
		ContributorGitHub: "alice",
		WalletAddress:     solana.PublicKeyFromBytes(publicKey).String(),
		IssuedAt:          time.Unix(1_699_000_000, 0).UTC(),
		ExpiresAt:         time.Unix(1_699_000_600, 0).UTC(),
	})

	signature := ed25519.Sign(privateKey, []byte(message))
	signatureBase58 := base58.Encode(signature)

	if err := VerifySignature(
		solana.PublicKeyFromBytes(publicKey).String(),
		message+"\nextra",
		signatureBase58,
	); err == nil {
		t.Fatal("VerifySignature succeeded for tampered message")
	}
}
