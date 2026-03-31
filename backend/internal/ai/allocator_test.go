package ai

import (
	"context"
	"testing"

	"github.com/repobounty/repobounty-ai/internal/models"
)

func TestDeterministicAllocate(t *testing.T) {
	contributors := []models.Contributor{
		{Username: "alice", Commits: 10, PullRequests: 5, Reviews: 3},
		{Username: "bob", Commits: 5, PullRequests: 2, Reviews: 1},
	}

	allocs, err := deterministicAllocate(contributors, 1_000_000_000)
	if err != nil {
		t.Fatalf("deterministicAllocate: %v", err)
	}

	if len(allocs) != 2 {
		t.Fatalf("len = %d, want 2", len(allocs))
	}

	var totalBps uint16
	for _, a := range allocs {
		totalBps += a.Percentage
	}
	if totalBps != 10000 {
		t.Errorf("total bps = %d, want 10000", totalBps)
	}

	// alice has more activity, should get more
	if allocs[0].Contributor != "alice" || allocs[0].Percentage <= allocs[1].Percentage {
		t.Errorf("alice should have higher allocation than bob: alice=%d, bob=%d", allocs[0].Percentage, allocs[1].Percentage)
	}

	// amounts should match percentages
	for _, a := range allocs {
		expected := uint64(a.Percentage) * 1_000_000_000 / 10000
		if a.Amount != expected {
			t.Errorf("%s: amount = %d, want %d", a.Contributor, a.Amount, expected)
		}
	}
}

func TestDeterministicAllocate_SingleContributor(t *testing.T) {
	contributors := []models.Contributor{
		{Username: "solo", Commits: 10, PullRequests: 5, Reviews: 3},
	}

	allocs, err := deterministicAllocate(contributors, 1_000_000_000)
	if err != nil {
		t.Fatalf("deterministicAllocate: %v", err)
	}

	if len(allocs) != 1 {
		t.Fatalf("len = %d, want 1", len(allocs))
	}
	if allocs[0].Percentage != 10000 {
		t.Errorf("percentage = %d, want 10000", allocs[0].Percentage)
	}
	if allocs[0].Amount != 1_000_000_000 {
		t.Errorf("amount = %d, want 1_000_000_000", allocs[0].Amount)
	}
}

func TestDeterministicAllocate_MinWeight(t *testing.T) {
	contributors := []models.Contributor{
		{Username: "zero", Commits: 0, PullRequests: 0, Reviews: 0},
		{Username: "active", Commits: 10, PullRequests: 5, Reviews: 3},
	}

	allocs, err := deterministicAllocate(contributors, 1_000_000_000)
	if err != nil {
		t.Fatalf("deterministicAllocate: %v", err)
	}

	var totalBps uint16
	for _, a := range allocs {
		totalBps += a.Percentage
		if a.Percentage == 0 {
			t.Errorf("%s has 0%% allocation", a.Contributor)
		}
	}
	if totalBps != 10000 {
		t.Errorf("total bps = %d, want 10000", totalBps)
	}
}

func TestDeterministicEvaluate(t *testing.T) {
	prs := map[string][]string{
		"alice": {"diff --git a/file.go b/file.go\n+line1\n+line2\n+line3"},
		"bob":   {"diff --git a/other.go b/other.go\n+line1"},
	}

	allocs, err := deterministicEvaluate(prs, 2_000_000_000)
	if err != nil {
		t.Fatalf("deterministicEvaluate: %v", err)
	}

	if len(allocs) != 2 {
		t.Fatalf("len = %d, want 2", len(allocs))
	}

	var totalBps uint16
	for _, a := range allocs {
		totalBps += a.Percentage
	}
	if totalBps != 10000 {
		t.Errorf("total bps = %d, want 10000", totalBps)
	}
}

func TestAllocator_NoContributors(t *testing.T) {
	a := NewAllocator("", "")
	_, err := a.Allocate(context.Background(), "owner/repo", nil, 1_000_000_000)
	if err == nil {
		t.Error("expected error for empty contributors")
	}
}

func TestAllocator_DeterministicFallback(t *testing.T) {
	a := NewAllocator("", "") // no API key -> deterministic

	if a.Model() != "deterministic-fallback" {
		t.Errorf("Model = %q, want %q", a.Model(), "deterministic-fallback")
	}

	contributors := []models.Contributor{
		{Username: "alice", Commits: 10, PullRequests: 5, Reviews: 3},
	}

	allocs, err := a.Allocate(context.Background(), "owner/repo", contributors, 1_000_000_000)
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if len(allocs) != 1 || allocs[0].Percentage != 10000 {
		t.Errorf("unexpected allocation: %+v", allocs)
	}
}

func TestEvaluateCodeImpact_NoPRs(t *testing.T) {
	a := NewAllocator("", "")
	_, err := a.EvaluateCodeImpact(context.Background(), "owner/repo", nil, 1_000_000_000)
	if err == nil {
		t.Error("expected error for empty PRs")
	}
}

func TestEvaluateCodeImpact_DeterministicFallback(t *testing.T) {
	a := NewAllocator("", "")

	prs := map[string][]string{
		"alice": {"diff --git a/file.go b/file.go\n+new code"},
	}

	allocs, err := a.EvaluateCodeImpact(context.Background(), "owner/repo", prs, 1_000_000_000)
	if err != nil {
		t.Fatalf("EvaluateCodeImpact: %v", err)
	}
	if len(allocs) != 1 || allocs[0].Percentage != 10000 {
		t.Errorf("unexpected allocation: %+v", allocs)
	}
}

func TestParseDiffSummaries(t *testing.T) {
	diffs := []string{
		"diff --git a/src/main.go b/src/main.go\n+++ b/src/main.go\n+added line",
	}
	summaries := parseDiffSummaries(diffs)
	if len(summaries) != 1 {
		t.Fatalf("len = %d, want 1", len(summaries))
	}
	if summaries[0].Description == "" {
		t.Error("description should not be empty")
	}
}
