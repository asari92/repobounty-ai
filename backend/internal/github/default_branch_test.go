package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestGetDefaultBranch_FromRepoMetadata(t *testing.T) {
	client := NewClient("")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := responseRecorder{header: make(http.Header)}
		if r.URL.Path == "/repos/acme/repo" {
			_ = json.NewEncoder(&rec).Encode(map[string]any{
				"id":             123,
				"name":           "repo",
				"default_branch": "feature/multi-service-dashboard",
				"html_url":       "https://github.com/acme/repo",
				"owner":          map[string]any{"login": "acme"},
			})
		} else {
			rec.WriteHeader(http.StatusNotFound)
		}
		return rec.Response(), nil
	})}

	branch, err := client.GetDefaultBranch(context.Background(), "acme/repo")
	if err != nil {
		t.Fatalf("GetDefaultBranch: %v", err)
	}
	if branch != "feature/multi-service-dashboard" {
		t.Fatalf("expected feature/multi-service-dashboard, got %s", branch)
	}
}

func TestGetDefaultBranch_SingleBranchFallback(t *testing.T) {
	client := NewClient("")
	callCount := 0
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := responseRecorder{header: make(http.Header)}
		callCount++

		if r.URL.Path == "/repos/acme/repo" {
			_ = json.NewEncoder(&rec).Encode(map[string]any{
				"id":       123,
				"name":     "repo",
				"owner":    map[string]any{"login": "acme"},
				"html_url": "https://github.com/acme/repo",
			})
		} else if r.URL.Path == "/repos/acme/repo/branches" {
			_ = json.NewEncoder(&rec).Encode([]map[string]any{
				{"name": "develop"},
			})
		} else {
			rec.WriteHeader(http.StatusNotFound)
		}
		return rec.Response(), nil
	})}

	branch, err := client.GetDefaultBranch(context.Background(), "acme/repo")
	if err != nil {
		t.Fatalf("GetDefaultBranch: %v", err)
	}
	if branch != "develop" {
		t.Fatalf("expected develop, got %s", branch)
	}
}

func TestGetDefaultBranch_MultipleBranchesNoDefault_ReturnsError(t *testing.T) {
	client := NewClient("")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := responseRecorder{header: make(http.Header)}

		if r.URL.Path == "/repos/acme/repo" {
			_ = json.NewEncoder(&rec).Encode(map[string]any{
				"id":       123,
				"name":     "repo",
				"owner":    map[string]any{"login": "acme"},
				"html_url": "https://github.com/acme/repo",
			})
		} else if r.URL.Path == "/repos/acme/repo/branches" {
			_ = json.NewEncoder(&rec).Encode([]map[string]any{
				{"name": "develop"},
				{"name": "staging"},
			})
		} else {
			rec.WriteHeader(http.StatusNotFound)
		}
		return rec.Response(), nil
	})}

	_, err := client.GetDefaultBranch(context.Background(), "acme/repo")
	if err == nil {
		t.Fatal("expected error for multiple branches with no default")
	}
}

func TestFetchBranchCommits_WithBranchContainingSlash(t *testing.T) {
	client := NewClient("")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := responseRecorder{header: make(http.Header)}

		if r.URL.Path == "/repos/acme/repo/commits" && r.URL.Query().Get("sha") == "feature/multi-service-dashboard" {
			_ = json.NewEncoder(&rec).Encode([]map[string]any{
				{"author": map[string]any{"id": float64(101), "login": "alice"}},
				{"author": map[string]any{"id": float64(101), "login": "alice"}},
				{"author": map[string]any{"id": float64(202), "login": "bob"}},
				{"author": nil},
			})
		} else {
			rec.WriteHeader(http.StatusNotFound)
		}
		return rec.Response(), nil
	})}

	contributors, err := client.FetchBranchCommits(context.Background(), "acme/repo", "feature/multi-service-dashboard")
	if err != nil {
		t.Fatalf("FetchBranchCommits: %v", err)
	}

	if len(contributors) != 2 {
		t.Fatalf("expected 2 contributors, got %d", len(contributors))
	}

	aliceFound := false
	bobFound := false
	for _, c := range contributors {
		if c.Username == "alice" {
			if c.Commits != 2 {
				t.Fatalf("alice expected 2 commits, got %d", c.Commits)
			}
			if c.GithubUserID != 101 {
				t.Fatalf("alice expected ID 101, got %d", c.GithubUserID)
			}
			aliceFound = true
		}
		if c.Username == "bob" {
			if c.Commits != 1 {
				t.Fatalf("bob expected 1 commit, got %d", c.Commits)
			}
			bobFound = true
		}
	}
	if !aliceFound || !bobFound {
		t.Fatal("missing alice or bob in results")
	}
}

func TestFetchBranchCommits_AllNullAuthors_ReturnsError(t *testing.T) {
	client := NewClient("")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := responseRecorder{header: make(http.Header)}

		if r.URL.Path == "/repos/acme/repo/commits" {
			_ = json.NewEncoder(&rec).Encode([]map[string]any{
				{"author": nil},
				{"author": nil},
			})
		} else {
			rec.WriteHeader(http.StatusNotFound)
		}
		return rec.Response(), nil
	})}

	_, err := client.FetchBranchCommits(context.Background(), "acme/repo", "main")
	if err == nil {
		t.Fatal("expected error when all commits have null author")
	}
}

func TestFetchContributionWindowData_CommitFallback(t *testing.T) {
	now := time.Now().UTC()
	windowStart := now.Add(-24 * time.Hour)
	windowEnd := now

	client := NewClient("")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rec := responseRecorder{header: make(http.Header)}

		switch {
		case r.URL.Path == "/repos/acme/repo/contributors":
			_ = json.NewEncoder(&rec).Encode([]map[string]any{})
		case r.URL.Path == "/repos/acme/repo":
			_ = json.NewEncoder(&rec).Encode(map[string]any{
				"id":             123,
				"name":           "repo",
				"default_branch": "feature/multi-service-dashboard",
				"owner":          map[string]any{"login": "acme"},
			})
		case r.URL.Path == "/repos/acme/repo/commits":
			_ = json.NewEncoder(&rec).Encode([]map[string]any{
				{"author": map[string]any{"id": float64(101), "login": "alice"}},
				{"author": map[string]any{"id": float64(202), "login": "bob"}},
			})
		case r.URL.Path == "/repos/acme/repo/pulls" && r.URL.Query().Get("state") == "all":
			_ = json.NewEncoder(&rec).Encode([]map[string]any{})
		case r.URL.Path == "/repos/acme/repo/pulls":
			_ = json.NewEncoder(&rec).Encode([]map[string]any{})
		default:
			rec.WriteHeader(http.StatusNotFound)
		}
		return rec.Response(), nil
	})}

	data, err := client.FetchContributionWindowData(context.Background(), "acme/repo", windowStart, windowEnd)
	if err != nil {
		t.Fatalf("FetchContributionWindowData: %v", err)
	}

	if len(data.Contributors) != 2 {
		t.Fatalf("expected 2 contributors from commit fallback, got %d", len(data.Contributors))
	}
}
