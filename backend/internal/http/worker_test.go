package http

import (
	"testing"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"
)

func TestNormalizeAutoFinalizeIntervalDefaultsToOneMinute(t *testing.T) {
	if got := normalizeAutoFinalizeInterval(0); got != time.Minute {
		t.Fatalf("normalizeAutoFinalizeInterval(0) = %s, want %s", got, time.Minute)
	}
}

func TestNormalizeAutoFinalizeIntervalDefaultsToOneMinuteForNegativeDuration(t *testing.T) {
	if got := normalizeAutoFinalizeInterval(-time.Second); got != time.Minute {
		t.Fatalf("normalizeAutoFinalizeInterval(-1s) = %s, want %s", got, time.Minute)
	}
}

func TestNormalizeAutoFinalizeIntervalPreservesExplicitValue(t *testing.T) {
	if got := normalizeAutoFinalizeInterval(90 * time.Second); got != 90*time.Second {
		t.Fatalf("normalizeAutoFinalizeInterval(90s) = %s, want %s", got, 90*time.Second)
	}
}

func TestMergeAutoFinalizeCampaignsIncludesOnChainFundedCampaigns(t *testing.T) {
	now := time.Now()
	onChain := []*models.Campaign{
		{
			CampaignID: "chain-1",
			State:      models.StateFunded,
			Deadline:   now.Add(-time.Minute),
			Sponsor:    "SponsorWallet123",
			Repo:       "acme/repo",
		},
	}
	merged := mergeAutoFinalizeCampaigns([]*models.Campaign{}, onChain)
	if len(merged) != 1 {
		t.Fatalf("len(merged) = %d, want 1", len(merged))
	}
	if merged[0].State != models.StateFunded {
		t.Fatalf("merged[0].State = %q, want funded", merged[0].State)
	}
	if merged[0].Deadline.After(now) {
		t.Fatal("merged[0].Deadline should be in the past")
	}
}

func TestMergeAutoFinalizeCampaignsDoesNotCountFinalizedAsFunded(t *testing.T) {
	now := time.Now()
	onChain := []*models.Campaign{
		{CampaignID: "chain-2", State: models.StateFinalized, Deadline: now.Add(-time.Minute)},
	}
	merged := mergeAutoFinalizeCampaigns([]*models.Campaign{}, onChain)
	if len(merged) != 1 {
		t.Fatalf("len(merged) = %d, want 1", len(merged))
	}
	fundedCount := 0
	for _, c := range merged {
		if c.State == models.StateFunded {
			fundedCount++
		}
	}
	if fundedCount != 0 {
		t.Fatalf("funded count = %d, want 0", fundedCount)
	}
}
