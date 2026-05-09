package http

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/store"
)

type mockGitHubServiceNoPRs struct {
	windowData *github.ContributionWindowData
	err        error
}

func (m *mockGitHubServiceNoPRs) RepositoryExists(ctx context.Context, repo string) (bool, error) {
	return true, nil
}

func (m *mockGitHubServiceNoPRs) RepositoryID(ctx context.Context, repo string) (uint64, error) {
	return 123456, nil
}

func (m *mockGitHubServiceNoPRs) FetchContributionWindowData(
	ctx context.Context,
	repo string,
	windowStart time.Time,
	windowEnd time.Time,
) (*github.ContributionWindowData, error) {
	return m.windowData, m.err
}

func (m *mockGitHubServiceNoPRs) SearchUsers(ctx context.Context, query string) ([]github.UserSearchResult, error) {
	return nil, nil
}

func (m *mockGitHubServiceNoPRs) SearchRepositories(ctx context.Context, owner, query string) ([]github.RepoSearchResult, error) {
	return nil, nil
}

func setupTestHandlers(t *testing.T) *Handlers {
	t.Helper()

	mockGH := &mockGitHubServiceNoPRs{
		windowData: &github.ContributionWindowData{
			ContributorPRDiffs: make(map[string][]string),
			Contributors: []models.Contributor{
				{
					GithubUserID: 123,
					Username:     "alice",
					Commits:      10,
					PullRequests: 5,
					Reviews:      3,
					LinesAdded:   100,
					LinesDeleted: 50,
				},
				{
					GithubUserID: 456,
					Username:     "bob",
					Commits:      5,
					PullRequests: 2,
					Reviews:      1,
					LinesAdded:   50,
					LinesDeleted: 25,
				},
			},
			WindowStart:       time.Now().Add(-24 * time.Hour),
			WindowEnd:         time.Now(),
			ContributorSource: "github_api",
		},
	}

	s := store.New()

	return NewHandlers(
		s,
		mockGH,
		&stubSolanaNotConfiguredService{},
		ai.NewAllocator("", "test"),
		nil,
		nil,
		nil,
	)
}

func TestCalculateAllocations_NoPRs_UseMetricBased(t *testing.T) {
	h := setupTestHandlers(t)

	campaign := &models.Campaign{
		CampaignID: "test-no-prs",
		Repo:       "test-owner/test-repo-no-prs",
		PoolAmount: 100000000,
		State:      models.StateFunded,
		CreatedAt:  time.Now().Add(-24 * time.Hour),
		Deadline:   time.Now(),
	}
	err := h.store.Create(campaign)
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	result, err := h.calculateAllocations(context.Background(), campaign, allocationOptions{})

	if err != nil {
		t.Fatalf("calculateAllocations failed: %v", err)
	}

	if result.allocationMode != models.AllocationModeMetrics {
		t.Errorf("expected AllocationModeMetrics, got %v", result.allocationMode)
	}

	if len(result.allocations) == 0 {
		t.Error("expected some allocations, got none")
	}
}

func TestCalculateAllocations_GitHubError_ReturnsError(t *testing.T) {
	s := store.New()
	h := NewHandlers(
		s,
		&mockGitHubServiceNoPRs{err: fmt.Errorf("github API failed: rate limited")},
		&stubSolanaNotConfiguredService{},
		ai.NewAllocator("", "test"),
		nil, nil, nil,
	)

	campaign := &models.Campaign{
		CampaignID: "test-gh-err",
		Repo:       "owner/repo",
		PoolAmount: 100000000,
		State:      models.StateFunded,
		CreatedAt:  time.Now().Add(-24 * time.Hour),
		Deadline:   time.Now(),
	}
	err := h.store.Create(campaign)
	if err != nil {
		t.Fatalf("create campaign: %v", err)
	}

	_, err = h.calculateAllocations(context.Background(), campaign, allocationOptions{})
	if err == nil {
		t.Fatal("expected error when GitHub API fails, got nil")
	}
	if !strings.Contains(err.Error(), "github API failed") {
		t.Errorf("expected GitHub error message, got: %v", err)
	}

	updated, _ := h.store.Get(campaign.CampaignID)
	if updated.FinalizationStatus != models.FinalizationStatusNeedsReview {
		t.Errorf("expected finalization_status=%s, got %s", models.FinalizationStatusNeedsReview, updated.FinalizationStatus)
	}
}

func TestValidateAllocationsPreFinalize_RejectsZeroGithubUserID(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 0, Amount: 100000000},
	}
	err := validateAllocationsPreFinalize(allocs, map[uint64]bool{1: true}, 100000000)
	if err == nil {
		t.Fatal("expected error for zero github_user_id")
	}
	if !strings.Contains(err.Error(), "github_user_id=0") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAllocationsPreFinalize_RejectsUnknownGithubUserID(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 999, Amount: 100000000},
	}
	err := validateAllocationsPreFinalize(allocs, map[uint64]bool{123: true, 456: true}, 100000000)
	if err == nil {
		t.Fatal("expected error for unknown github_user_id")
	}
	if !strings.Contains(err.Error(), "not in repository identity set") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAllocationsPreFinalize_RejectsSumMismatch(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 123, Amount: 50000000},
		{GithubUserID: 456, Amount: 50000000},
	}
	err := validateAllocationsPreFinalize(allocs, map[uint64]bool{123: true, 456: true}, 150000000)
	if err == nil {
		t.Fatal("expected error for sum mismatch")
	}
	if !strings.Contains(err.Error(), "sum") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAllocationsPreFinalize_RejectsBelowMinimum(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 123, Amount: 1000},
	}
	err := validateAllocationsPreFinalize(allocs, map[uint64]bool{123: true}, 1000)
	if err == nil {
		t.Fatal("expected error for amount below minimum")
	}
	if !strings.Contains(err.Error(), "below minimum") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAllocationsPreFinalize_RejectsEmptyAllocations(t *testing.T) {
	err := validateAllocationsPreFinalize(nil, map[uint64]bool{123: true}, 100000000)
	if err == nil {
		t.Fatal("expected error for empty allocations")
	}
	if !strings.Contains(err.Error(), "no allocations") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateAllocationsPreFinalize_AcceptsValidData(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 123, Amount: 60000000},
		{GithubUserID: 456, Amount: 50000000},
	}
	err := validateAllocationsPreFinalize(allocs, map[uint64]bool{123: true, 456: true}, 110000000)
	if err != nil {
		t.Fatalf("expected no error for valid data, got: %v", err)
	}
}

func TestRedistributeBelowMinimum_FiltersAndRedistributesToTop1(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 123, Contributor: "alice", Amount: 60000000},
		{GithubUserID: 456, Contributor: "bob", Amount: 50000000},
		{GithubUserID: 789, Contributor: "charlie", Amount: 30000000},
	}
	result := redistributeBelowMinimum(allocs, 140000000)
	if len(result) != 2 {
		t.Fatalf("expected 2 allocations after filtering, got %d", len(result))
	}
	var sum uint64
	for _, a := range result {
		sum += a.Amount
		if a.Amount < models.MinAllocationLamports {
			t.Fatalf("allocation for %s has amount %d below minimum", a.Contributor, a.Amount)
		}
	}
	if sum != 140000000 {
		t.Fatalf("expected sum 140000000, got %d", sum)
	}
	if result[0].Contributor != "alice" {
		t.Fatalf("expected top-1 to be alice, got %s", result[0].Contributor)
	}
	if result[0].Amount != 90000000 {
		t.Fatalf("expected alice to get 90000000 (60000000 + 30000000 + rounding), got %d", result[0].Amount)
	}
}

func TestRedistributeBelowMinimum_MultipleBelowMinimum(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 123, Contributor: "alice", Amount: 80000000},
		{GithubUserID: 456, Contributor: "bob", Amount: 20000000},
		{GithubUserID: 789, Contributor: "charlie", Amount: 10000000},
	}
	result := redistributeBelowMinimum(allocs, 110000000)
	if len(result) != 1 {
		t.Fatalf("expected 1 allocation after filtering, got %d", len(result))
	}
	if result[0].Amount != 110000000 {
		t.Fatalf("expected alice to get entire pool 110000000, got %d", result[0].Amount)
	}
}

func TestRedistributeBelowMinimum_FloorRoundingRedistributed(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 123, Contributor: "alice", Amount: 59999999},
		{GithubUserID: 456, Contributor: "bob", Amount: 50000000},
	}
	result := redistributeBelowMinimum(allocs, 110000000)
	if len(result) != 2 {
		t.Fatalf("expected 2 allocations, got %d", len(result))
	}
	var sum uint64
	for _, a := range result {
		sum += a.Amount
	}
	if sum != 110000000 {
		t.Fatalf("expected sum 110000000, got %d", sum)
	}
	if result[0].Amount != 60000000 {
		t.Fatalf("expected top-1 to get remainder, got %d", result[0].Amount)
	}
}

func TestRedistributeBelowMinimum_AllBelowMinimum_Top1GetsAll(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 123, Contributor: "alice", Amount: 30000000},
		{GithubUserID: 456, Contributor: "bob", Amount: 20000000},
	}
	result := redistributeBelowMinimum(allocs, 50000000)
	if len(result) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(result))
	}
	if result[0].Amount != 50000000 {
		t.Fatalf("expected alice to get entire pool, got %d", result[0].Amount)
	}
	if result[0].Contributor != "alice" {
		t.Fatalf("expected top-1 to be alice, got %s", result[0].Contributor)
	}
}

func TestRedistributeBelowMinimum_NoOpWhenAllAboveMinimum(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 123, Contributor: "alice", Amount: 60000000},
		{GithubUserID: 456, Contributor: "bob", Amount: 50000000},
	}
	result := redistributeBelowMinimum(allocs, 110000000)
	if len(result) != 2 {
		t.Fatalf("expected 2 allocations, got %d", len(result))
	}
	if result[0].Amount != 60000000 || result[1].Amount != 50000000 {
		t.Fatal("expected no change when all above minimum and sum exact")
	}
}

func TestRedistributeBelowMinimum_SumEqualsPoolExactly(t *testing.T) {
	allocs := []models.Allocation{
		{GithubUserID: 123, Contributor: "alice", Amount: 70000000},
		{GithubUserID: 456, Contributor: "bob", Amount: 50000000},
		{GithubUserID: 789, Contributor: "charlie", Amount: 20000000},
	}
	result := redistributeBelowMinimum(allocs, 140000000)
	var sum uint64
	for _, a := range result {
		sum += a.Amount
	}
	if sum != 140000000 {
		t.Fatalf("expected sum 140000000, got %d", sum)
	}
	for _, a := range result {
		if a.Amount < models.MinAllocationLamports {
			t.Fatalf("allocation for %s has amount %d below minimum", a.Contributor, a.Amount)
		}
	}
}

func TestCommitFinalize_SetsNeedsManualReviewOnValidationFailure(t *testing.T) {
	s := store.New()
	h := NewHandlers(
		s,
		&mockGitHubServiceNoPRs{},
		&stubSolanaService{},
		ai.NewAllocator("", "test"),
		nil, nil, nil,
	)

	campaign := &models.Campaign{
		CampaignID: "test-validation-fail",
		Repo:       "owner/repo",
		PoolAmount: 100000000,
		State:      models.StateFunded,
		Sponsor:    "sponsor-wallet",
		CreatedAt:  time.Now().Add(-24 * time.Hour),
		Deadline:   time.Now().Add(-1 * time.Hour),
	}
	if err := h.store.Create(campaign); err != nil {
		t.Fatalf("create: %v", err)
	}

	result := &allocationResult{
		contributors: []models.Contributor{
			{GithubUserID: 123, Username: "alice"},
		},
		allocations: []models.Allocation{
			{GithubUserID: 999, Amount: 1000},
		},
		allocationMode: models.AllocationModeMetrics,
	}

	_, err := h.commitFinalize(context.Background(), campaign, result)
	if err == nil {
		t.Fatal("expected validation error")
	}

	updated, _ := h.store.Get(campaign.CampaignID)
	if updated.FinalizationStatus != models.FinalizationStatusNeedsReview {
		t.Errorf("expected needs_manual_review, got %q", updated.FinalizationStatus)
	}
	if updated.State == models.StateFinalized {
		t.Error("campaign state should not be finalized")
	}
}

func TestWorkerFiltering_SkipsNeedsManualReview(t *testing.T) {
	for _, status := range []string{models.FinalizationStatusNeedsReview, models.FinalizationStatusFinalized} {
		c := &models.Campaign{
			CampaignID:         "test-" + status,
			State:              models.StateFunded,
			Deadline:           time.Now().Add(-1 * time.Hour),
			FinalizationStatus: status,
		}
		if c.FinalizationStatus == models.FinalizationStatusNeedsReview ||
			c.FinalizationStatus == models.FinalizationStatusFinalized {
			continue
		}
		t.Errorf("campaign with status=%s should have been skipped", status)
	}
}

func TestWorkerFiltering_ProcessesPending(t *testing.T) {
	for _, status := range []string{"", models.FinalizationStatusPending, models.FinalizationStatusAnalyzing} {
		c := &models.Campaign{
			CampaignID:         "test-" + status,
			State:              models.StateFunded,
			Deadline:           time.Now().Add(-1 * time.Hour),
			FinalizationStatus: status,
		}
		if c.FinalizationStatus == models.FinalizationStatusNeedsReview ||
			c.FinalizationStatus == models.FinalizationStatusFinalized {
			t.Errorf("campaign with status=%s should not be skipped", status)
		}
	}
}
