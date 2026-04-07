package github

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestFetchContributionWindowDataUsesRepositoryHistoryInMVP(t *testing.T) {
	now := time.Now().UTC()
	windowStart := now.Add(-24 * time.Hour)
	windowEnd := now

	oldMergedAt := now.Add(-30 * 24 * time.Hour)
	recentMergedAt := now.Add(-2 * time.Hour)

	client := NewClientWithEnv("", false)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := responseRecorder{header: make(http.Header)}

		switch {
		case r.URL.Path == "/repos/acme/repo/contributors":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{
				{"id": 101, "login": "alice", "avatar_url": "https://example.com/alice.png", "contributions": 8},
				{"id": 202, "login": "bob", "avatar_url": "https://example.com/bob.png", "contributions": 5},
			})
		case r.URL.Path == "/repos/acme/repo/pulls" && r.URL.Query().Get("state") == "all":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{
				{"user": map[string]any{"login": "alice"}},
				{"user": map[string]any{"login": "bob"}},
			})
		case r.URL.Path == "/repos/acme/repo/pulls" && r.URL.Query().Get("page") == "1":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{
				{
					"id":            1,
					"number":        1,
					"title":         "old contribution",
					"state":         "closed",
					"created_at":    oldMergedAt.Add(-2 * time.Hour).Format(time.RFC3339),
					"merged_at":     oldMergedAt.Format(time.RFC3339),
					"merged":        true,
					"commits":       3,
					"additions":     40,
					"deletions":     10,
					"changed_files": 2,
					"user":          map[string]any{"login": "bob"},
				},
				{
					"id":            2,
					"number":        2,
					"title":         "recent contribution",
					"state":         "closed",
					"created_at":    recentMergedAt.Add(-2 * time.Hour).Format(time.RFC3339),
					"merged_at":     recentMergedAt.Format(time.RFC3339),
					"merged":        true,
					"commits":       5,
					"additions":     80,
					"deletions":     20,
					"changed_files": 3,
					"user":          map[string]any{"login": "alice"},
				},
			})
		case r.URL.Path == "/repos/acme/repo/pulls" && r.URL.Query().Get("page") == "2":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{})
		case r.URL.Path == "/repos/acme/repo/pulls/1":
			if r.Header.Get("Accept") == "application/vnd.github.v3.diff" {
				_, _ = recorder.Write([]byte("diff --git a/old.txt b/old.txt\n+old contribution\n"))
				break
			}
			_ = json.NewEncoder(&recorder).Encode(map[string]any{
				"id":            1,
				"number":        1,
				"title":         "old contribution",
				"state":         "closed",
				"created_at":    oldMergedAt.Add(-2 * time.Hour).Format(time.RFC3339),
				"merged_at":     oldMergedAt.Format(time.RFC3339),
				"merged":        true,
				"commits":       3,
				"additions":     40,
				"deletions":     10,
				"changed_files": 2,
				"user":          map[string]any{"login": "bob"},
			})
		case r.URL.Path == "/repos/acme/repo/pulls/2":
			if r.Header.Get("Accept") == "application/vnd.github.v3.diff" {
				_, _ = recorder.Write([]byte("diff --git a/recent.txt b/recent.txt\n+recent contribution\n"))
				break
			}
			_ = json.NewEncoder(&recorder).Encode(map[string]any{
				"id":            2,
				"number":        2,
				"title":         "recent contribution",
				"state":         "closed",
				"created_at":    recentMergedAt.Add(-2 * time.Hour).Format(time.RFC3339),
				"merged_at":     recentMergedAt.Format(time.RFC3339),
				"merged":        true,
				"commits":       5,
				"additions":     80,
				"deletions":     20,
				"changed_files": 3,
				"user":          map[string]any{"login": "alice"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}

		return recorder.Response(), nil
	})}

	data, err := client.FetchContributionWindowData(context.Background(), "acme/repo", windowStart, windowEnd)
	if err != nil {
		t.Fatalf("FetchContributionWindowData: %v", err)
	}

	if data.ContributorSource != "repository_history_mvp" {
		t.Fatalf("ContributorSource = %q, want repository_history_mvp", data.ContributorSource)
	}
	if len(data.Contributors) != 2 {
		t.Fatalf("len(Contributors) = %d, want 2", len(data.Contributors))
	}
	if len(data.ContributorPRDiffs["bob"]) == 0 {
		t.Fatal("expected old contributor bob to remain visible in MVP whole-repository mode")
	}
	if len(data.ContributorPRDiffs["alice"]) == 0 {
		t.Fatal("expected recent contributor alice to remain visible in MVP whole-repository mode")
	}
}

func TestFetchContributionWindowDataUsesFullHistoryEvenForProductionClient(t *testing.T) {
	now := time.Now().UTC()
	windowStart := now.Add(-24 * time.Hour)
	windowEnd := now

	// Production client — should still use full history in MVP.
	client := NewClientWithEnv("", true)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := responseRecorder{header: make(http.Header)}
		switch {
		case r.URL.Path == "/repos/acme/repo/contributors":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{
				{"id": 1, "login": "alice", "contributions": 5},
			})
		case r.URL.Path == "/repos/acme/repo/pulls" && r.URL.Query().Get("state") == "all":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{
				{"user": map[string]any{"login": "alice"}},
			})
		case r.URL.Path == "/repos/acme/repo/pulls" && r.URL.Query().Get("page") == "1":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{
				{
					"id":            1,
					"number":        1,
					"title":         "alice feature",
					"state":         "closed",
					"created_at":    now.Add(-2 * time.Hour).Format(time.RFC3339),
					"merged_at":     now.Add(-1 * time.Hour).Format(time.RFC3339),
					"merged":        true,
					"commits":       1,
					"additions":     10,
					"deletions":     2,
					"changed_files": 1,
					"user":          map[string]any{"login": "alice"},
				},
			})
		case r.URL.Path == "/repos/acme/repo/pulls" && r.URL.Query().Get("page") == "2":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{})
		case r.URL.Path == "/repos/acme/repo/pulls/1":
			if r.Header.Get("Accept") == "application/vnd.github.v3.diff" {
				_, _ = recorder.Write([]byte("diff --git a/file.txt b/file.txt\n+alice change\n"))
				break
			}
			_ = json.NewEncoder(&recorder).Encode(map[string]any{
				"id": 1, "number": 1, "title": "alice feature", "state": "closed",
				"merged": true, "user": map[string]any{"login": "alice"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		return recorder.Response(), nil
	})}

	data, err := client.FetchContributionWindowData(context.Background(), "acme/repo", windowStart, windowEnd)
	if err != nil {
		t.Fatalf("FetchContributionWindowData: %v", err)
	}
	if data.ContributorSource != "repository_history_mvp" {
		t.Fatalf("ContributorSource = %q, want repository_history_mvp", data.ContributorSource)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

type responseRecorder struct {
	header http.Header
	body   []byte
	status int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	r.body = append(r.body, p...)
	return len(p), nil
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
}

func (r *responseRecorder) Response() *http.Response {
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Header:     r.header.Clone(),
		Body:       io.NopCloser(bytes.NewReader(r.body)),
	}
}
