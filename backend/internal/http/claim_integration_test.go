package http

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/repobounty/repobounty-ai/internal/models"
)

func TestClaimChallengeRequiresAuthenticatedGitHubUser(t *testing.T) {
	router := NewRouter(newTestHandlersWithSolana(t), "test")
	body, err := json.Marshal(models.ClaimChallengeRequest{
		ContributorGithub: "alice",
		WalletAddress:     testWalletAddress(t),
	})
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{name: "claim-challenge", path: "/api/campaigns/123/claim-challenge"},
		{name: "claim", path: "/api/campaigns/123/claim"},
		{name: "claim-confirm", path: "/api/campaigns/123/claim-confirm"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := performRequest(t, router, http.MethodPost, tc.path, body, "")
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestClaimConfirmUsesOnChainClaimStatusAsSourceOfTruth(t *testing.T) {
	handlers, campaign, user := newClaimConfirmHandlersWithOnChainStatus(t, true)
	rec := performAuthedJSONRequest(t, handlers, user, http.MethodPost, "/api/campaigns/"+campaign.CampaignID+"/claim-confirm", models.ClaimConfirmRequest{
		ContributorGithub: "alice",
		WalletAddress:     user.WalletAddress,
		TxSignature:       "sig-123",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	stored, err := handlers.store.Get(campaign.CampaignID)
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if stored.State != models.StateCompleted {
		t.Fatalf("state = %q, want %q", stored.State, models.StateCompleted)
	}
	if stored.Status != models.StateClosed {
		t.Fatalf("status = %q, want %q", stored.Status, models.StateClosed)
	}
	if stored.TotalClaimed != 2_000_000_000 {
		t.Fatalf("total_claimed = %d, want %d", stored.TotalClaimed, 2_000_000_000)
	}
	if stored.ClaimedAmount != 2_000_000_000 {
		t.Fatalf("claimed_amount = %d, want %d", stored.ClaimedAmount, 2_000_000_000)
	}
	if stored.ClaimedCount != 2 {
		t.Fatalf("claimed_count = %d, want 2", stored.ClaimedCount)
	}
	if !stored.Allocations[0].Claimed {
		t.Fatal("alice allocation was not marked claimed")
	}
	if stored.Allocations[0].ClaimedAt == nil {
		t.Fatal("allocation claim timestamp was not recorded")
	}
	if stored.Allocations[0].ClaimantWallet != user.WalletAddress {
		t.Fatalf("claimant_wallet = %q, want %q", stored.Allocations[0].ClaimantWallet, user.WalletAddress)
	}
	if stored.Allocations[1].Claimed {
		t.Fatal("bob allocation should remain unclaimed in stale local state")
	}
}
