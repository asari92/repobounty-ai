package ai

import (
	"testing"

	"github.com/repobounty/repobounty-ai/internal/models"
)

func TestNormalizeAllocationsCanonicalizesAndSorts(t *testing.T) {
	contributors := []models.Contributor{
		{Username: "Alice"},
		{Username: "bob"},
	}
	allocations := []models.Allocation{
		{Contributor: "bob", Percentage: 4000, Reasoning: "b"},
		{Contributor: "alice", Percentage: 6000, Reasoning: "a"},
	}

	normalized, err := NormalizeAllocations(allocations, contributors, 1_000_000_000)
	if err != nil {
		t.Fatalf("NormalizeAllocations: %v", err)
	}
	if len(normalized) != 2 {
		t.Fatalf("len(normalized) = %d, want 2", len(normalized))
	}
	if normalized[0].Contributor != "Alice" {
		t.Fatalf("normalized[0].Contributor = %q, want %q", normalized[0].Contributor, "Alice")
	}
	if normalized[1].Contributor != "bob" {
		t.Fatalf("normalized[1].Contributor = %q, want %q", normalized[1].Contributor, "bob")
	}
	if normalized[0].Amount == 0 || normalized[1].Amount == 0 {
		t.Fatal("expected non-zero allocation amounts")
	}
}

func TestNormalizeAllocationsRejectsUnknownContributor(t *testing.T) {
	_, err := NormalizeAllocations(
		[]models.Allocation{{Contributor: "mallory", Percentage: 10000}},
		[]models.Contributor{{Username: "alice"}},
		1_000_000_000,
	)
	if err == nil {
		t.Fatal("NormalizeAllocations succeeded for unknown contributor")
	}
}

func TestNormalizeAllocationsRejectsZeroLamportAllocation(t *testing.T) {
	_, err := NormalizeAllocations(
		[]models.Allocation{
			{Contributor: "alice", Percentage: 5000},
			{Contributor: "bob", Percentage: 5000},
		},
		[]models.Contributor{
			{Username: "alice"},
			{Username: "bob"},
		},
		1,
	)
	if err == nil {
		t.Fatal("NormalizeAllocations succeeded for zero-lamport allocation")
	}
}

func TestDeterministicAllocateTrimsToContractLimit(t *testing.T) {
	contributors := make([]models.Contributor, 0, 12)
	for i := 0; i < 12; i++ {
		contributors = append(contributors, models.Contributor{
			Username:     string(rune('a' + i)),
			Commits:      12 - i,
			PullRequests: 12 - i,
		})
	}

	allocations, err := deterministicAllocate(contributors, 1_000_000_000)
	if err != nil {
		t.Fatalf("deterministicAllocate: %v", err)
	}
	if len(allocations) != maxContractAllocations {
		t.Fatalf("len(allocations) = %d, want %d", len(allocations), maxContractAllocations)
	}

	var totalBPS uint32
	for _, allocation := range allocations {
		totalBPS += uint32(allocation.Percentage)
		if allocation.Amount == 0 {
			t.Fatalf("allocation %q has zero amount", allocation.Contributor)
		}
	}
	if totalBPS != 10000 {
		t.Fatalf("totalBPS = %d, want 10000", totalBPS)
	}
}
