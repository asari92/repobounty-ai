package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"
)

func TestSQLiteStore_SaveFinalizeSnapshot(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "repobounty-test.db")
	sqliteStore, err := NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer sqliteStore.Close()

	campaign := newTestCampaign("sqlite-snapshot")
	if err := sqliteStore.Create(campaign); err != nil {
		t.Fatalf("Create campaign: %v", err)
	}

	firstApprovedAt := time.Now().UTC()
	firstSnapshot := &models.FinalizeSnapshot{
		CampaignID:        campaign.CampaignID,
		InputHash:         "hash-1",
		AllocationMode:    models.AllocationModeMetrics,
		Contributors:      []models.Contributor{{Username: "alice"}},
		Allocations:       []models.Allocation{{Contributor: "alice", Percentage: 10000, Amount: 1_000_000_000}},
		WindowStart:       campaign.CreatedAt,
		WindowEnd:         campaign.Deadline,
		ContributorSource: "merged_pr_window",
		ContributorNotes:  "note",
		CreatedAt:         time.Now().UTC(),
		ApprovedAt:        &firstApprovedAt,
	}
	if err := sqliteStore.SaveFinalizeSnapshot(firstSnapshot); err != nil {
		t.Fatalf("SaveFinalizeSnapshot(first): %v", err)
	}

	secondSnapshot := &models.FinalizeSnapshot{
		CampaignID:        campaign.CampaignID,
		InputHash:         "hash-2",
		AllocationMode:    models.AllocationModeCodeImpact,
		Contributors:      []models.Contributor{{Username: "alice"}, {Username: "bob"}},
		Allocations:       []models.Allocation{{Contributor: "alice", Percentage: 6000, Amount: 600_000_000}, {Contributor: "bob", Percentage: 4000, Amount: 400_000_000}},
		WindowStart:       campaign.CreatedAt,
		WindowEnd:         campaign.Deadline,
		ContributorSource: "merged_pr_window",
		CreatedAt:         time.Now().UTC(),
	}
	if err := sqliteStore.SaveFinalizeSnapshot(secondSnapshot); err != nil {
		t.Fatalf("SaveFinalizeSnapshot(second): %v", err)
	}

	latest, err := sqliteStore.GetLatestFinalizeSnapshot(campaign.CampaignID)
	if err != nil {
		t.Fatalf("GetLatestFinalizeSnapshot: %v", err)
	}
	if latest.Version != 2 {
		t.Fatalf("latest.Version = %d, want 2", latest.Version)
	}
	if latest.InputHash != "hash-2" {
		t.Fatalf("latest.InputHash = %q, want %q", latest.InputHash, "hash-2")
	}
	if len(latest.Allocations) != 2 {
		t.Fatalf("len(latest.Allocations) = %d, want 2", len(latest.Allocations))
	}
	if latest.ApprovedAt != nil {
		t.Fatal("latest.ApprovedAt should be nil for the second snapshot")
	}
}
