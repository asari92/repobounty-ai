package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestClient_SearchRepositories_ReturnsOwnerRepositoriesForEmptyPrefix(t *testing.T) {
	client := NewClientWithEnv("", false)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := responseRecorder{header: make(http.Header)}

		switch {
		case r.URL.Path == "/users/octocat/repos":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{
				{"name": "hello-world", "owner": map[string]any{"login": "octocat"}},
				{"name": "docs", "owner": map[string]any{"login": "octocat"}},
			})
		default:
			recorder.WriteHeader(http.StatusNotFound)
		}

		return recorder.Response(), nil
	})}

	results, err := client.SearchRepositories(context.Background(), "octocat", "")
	if err != nil {
		t.Fatalf("SearchRepositories returned error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Owner != "octocat" || results[0].Name != "hello-world" {
		t.Fatalf("first result = %#v", results[0])
	}
}

func TestClient_SearchRepositories_FiltersByPrefix(t *testing.T) {
	client := NewClientWithEnv("", false)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := responseRecorder{header: make(http.Header)}

		switch {
		case r.URL.Path == "/users/octocat/repos":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{
				{"name": "hello-world", "owner": map[string]any{"login": "over"}},
				{"name": "docs", "owner": map[string]any{"login": "over"}},
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
EOF
