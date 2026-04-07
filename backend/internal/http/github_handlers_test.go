package http

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/store"
)

type mockGitHubSearchService struct {
	users           []github.UserSearchResult
	repositories    []github.RepoSearchResult
	usersErr        error
	repositoriesErr error
}

func (m *mockGitHubSearchService) RepositoryExists(ctx context.Context, repo string) (bool, error) {
	return true, nil
}

func (m *mockGitHubSearchService) RepositoryID(ctx context.Context, repo string) (uint64, error) {
	return 123456, nil
}

func (m *mockGitHubSearchService) FetchContributionWindowData(
	ctx context.Context,
	repo string,
	windowStart time.Time,
	windowEnd time.Time,
) (*github.ContributionWindowData, error) {
	return nil, nil
}

func (m *mockGitHubSearchService) SearchUsers(ctx context.Context, query string) ([]github.UserSearchResult, error) {
	if m.usersErr != nil {
		return nil, m.usersErr
	}
	return m.users, nil
}

func (m *mockGitHubSearchService) SearchRepositories(ctx context.Context, owner, query string) ([]github.RepoSearchResult, error) {
	if m.repositoriesErr != nil {
		return nil, m.repositoriesErr
	}
	return m.repositories, nil
}

func TestGitHubSearchUsers(t *testing.T) {
	InitLogger("development")

	mockService := &mockGitHubSearchService{
		users: []github.UserSearchResult{
			{Login: "alice", AvatarURL: "https://github.com/alice.png"},
			{Login: "bob", AvatarURL: "https://github.com/bob.png"},
		},
	}

	handlers := NewHandlers(
		store.New(),
		mockService,
		&stubSolanaNotConfiguredService{},
		nil,
		nil,
		nil,
		nil,
	)

	router := NewRouter(handlers, "test")

	rec := performRequest(t, router, http.MethodGet, "/api/github/search?q=alice", nil, "test-token")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var results []github.UserSearchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected non-empty results array")
	}

	if results[0].Login != "alice" {
		t.Errorf("expected first result login to be 'alice', got '%s'", results[0].Login)
	}
}

func TestGitHubSearchRepositories(t *testing.T) {
	InitLogger("development")

	mockService := &mockGitHubSearchService{
		repositories: []github.RepoSearchResult{
			{Name: "hello-world", Owner: "octocat"},
			{Name: "test-repo", Owner: "octocat"},
		},
	}

	handlers := NewHandlers(
		store.New(),
		mockService,
		&stubSolanaNotConfiguredService{},
		nil,
		nil,
		nil,
		nil,
	)

	router := NewRouter(handlers, "test")

	rec := performRequest(t, router, http.MethodGet, "/api/github/search?q=octocat/repo", nil, "test-token")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var results []github.RepoSearchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected non-empty results array")
	}

	if results[0].Name != "hello-world" {
		t.Errorf("expected first result name to be 'hello-world', got '%s'", results[0].Name)
	}
}
