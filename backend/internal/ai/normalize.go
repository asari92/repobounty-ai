package ai

import (
	"fmt"
	"sort"
	"strings"

	"github.com/repobounty/repobounty-ai/internal/models"
)

const maxContractAllocations = 10

func NormalizeAllocations(
	allocations []models.Allocation,
	contributors []models.Contributor,
	poolAmount uint64,
) ([]models.Allocation, error) {
	if len(allocations) == 0 {
		return nil, fmt.Errorf("no allocations to normalize")
	}

	allowedContributors := make(map[string]models.Contributor, len(contributors))
	for _, contributor := range contributors {
		key := strings.ToLower(strings.TrimSpace(contributor.Username))
		if key == "" {
			continue
		}
		allowedContributors[key] = contributor
	}
	if len(allowedContributors) == 0 {
		return nil, fmt.Errorf("no eligible contributors available")
	}

	if len(allocations) > maxContractAllocations {
		return nil, fmt.Errorf("allocation set exceeds the contract limit of %d contributors", maxContractAllocations)
	}

	normalized := make([]models.Allocation, 0, len(allocations))
	seen := make(map[string]struct{}, len(allocations))
	var totalBPS uint32

	for _, allocation := range allocations {
		key := strings.ToLower(strings.TrimSpace(allocation.Contributor))
		if key == "" {
			return nil, fmt.Errorf("allocation contributor is required")
		}
		canonicalContributor, ok := allowedContributors[key]
		if !ok {
			return nil, fmt.Errorf("allocation contributor %q is not in the fetched contributor set", allocation.Contributor)
		}
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate contributor %q in allocations", allocation.Contributor)
		}
		if allocation.Percentage == 0 {
			return nil, fmt.Errorf("allocation percentage for %s must be greater than zero", canonicalContributor.Username)
		}

		seen[key] = struct{}{}
		totalBPS += uint32(allocation.Percentage)
		allocation.Contributor = canonicalContributor.Username
		allocation.GithubUsername = canonicalContributor.Username
		allocation.GithubUserID = canonicalContributor.GithubUserID
		allocation.Amount = poolAmount * uint64(allocation.Percentage) / 10000
		if allocation.Amount == 0 {
			return nil, fmt.Errorf("allocation for %s rounds down to zero lamports", canonicalContributor.Username)
		}
		normalized = append(normalized, allocation)
	}

	if totalBPS != 10000 {
		return nil, fmt.Errorf("allocation percentages sum to %d basis points, expected 10000", totalBPS)
	}

	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Percentage == normalized[j].Percentage {
			return normalized[i].Contributor < normalized[j].Contributor
		}
		return normalized[i].Percentage > normalized[j].Percentage
	})

	return normalized, nil
}
