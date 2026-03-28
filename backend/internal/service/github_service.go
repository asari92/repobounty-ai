package service

import (
	"context"

	"github.com/yourusername/repobounty-ai/internal/domain/models"
)

type GitHubService struct{}

func (s *GitHubService) GetContributors(ctx context.Context, repoURL string) ([]models.Contributor, error) {
	return []models.Contributor{
		{Username: "alice", Commits: 42, PRs: 8},
		{Username: "bob", Commits: 20, PRs: 3},
		{Username: "carol", Commits: 15, PRs: 5},
	}, nil
}
