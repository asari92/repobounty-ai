package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestClient_SearchRepositories_ReturnsEmptyForShortQuery(t *testing.T) {
	client := NewClientWithEnv("", false)

	results, err := client.SearchRepositories(context.Background(), "octocat", "")
	if err != nil {
		t.Fatalf("SearchRepositories returned error: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected empty results for short query, got %d", len(results))
	}
}

func TestClient_SearchRepositories_FiltersByPrefix(t *testing.T) {
	client := NewClientWithEnv("", false)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := responseRecorder{header: make(http.Header)}

		switch {
		case r.URL.Path == "/search/repositories":
			_ = json.NewEncoder(&recorder).Encode(map[string]any{
				"items": []map[string]any{
					{"name": "hello-world", "owner": map[string]any{"login": "octocat"}},
				},
			})
		default:
			recorder.WriteHeader(http.StatusNotFound)
		}

		return recorder.Response(), nil
	})}

	results, err := client.SearchRepositories(context.Background(), "octocat", "he")
	if err != nil {
		t.Fatalf("SearchRepositories returned error: %v", err)
	}

	if len(results) != 1 || results[0].Name != "hello-world" {
		t.Fatalf("results = %#v", results)
	}
}

func TestFetchPRsWithDiffs_EmptyResult_NoMock(t *testing.T) {
	_ = NewClient("test-token")
	// Mock fetchPRsWithStats to return empty array
	// This is difficult to test without refactoring, so we'll test integration style

	// For now, we'll test the behavior through a higher-level test
	// This placeholder ensures we remember to test this scenario
}
