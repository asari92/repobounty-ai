package http

import (
	"context"
	"testing"
	"time"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/store"
)

type mockGitHubServiceNoPRs struct {
	windowData *github.ContributionWindowData
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
	return m.windowData, nil
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
