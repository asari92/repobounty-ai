package ai

import (
	"errors"
	"math"
	"sort"

	"github.com/repobounty/repobounty-ai/internal/models"
)

var (
	ErrInvalidTotalAmount    = errors.New("total reward amount must be greater than zero")
	ErrInvalidMinAllocation  = errors.New("min allocation must be greater than zero")
	ErrInvalidMaxAllocations = errors.New("max allocations must be greater than zero")
	ErrAllBelowMinimum       = errors.New("all allocations below minimum threshold")
	ErrAllocationSumMismatch = errors.New("final allocation sum does not match total")
)

type ContributorImpact = models.ContributorImpact

func CalculateFinalAllocations(
	totalRewardAmount uint64,
	contributors []ContributorImpact,
	maxAllocations int,
	minAllocation uint64,
) ([]models.AllocationEntry, error) {
	if totalRewardAmount == 0 {
		return nil, ErrInvalidTotalAmount
	}
	if minAllocation == 0 {
		return nil, ErrInvalidMinAllocation
	}
	if maxAllocations <= 0 {
		return nil, ErrInvalidMaxAllocations
	}

	if len(contributors) == 0 {
		return []models.AllocationEntry{}, nil
	}

	sortedContributors := make([]ContributorImpact, len(contributors))
	copy(sortedContributors, contributors)

	sort.Slice(sortedContributors, func(i, j int) bool {
		return sortedContributors[i].ImpactPercentage > sortedContributors[j].ImpactPercentage
	})

	if len(sortedContributors) > maxAllocations {
		sortedContributors = sortedContributors[:maxAllocations]
		totalImpact := 0.0
		for _, c := range sortedContributors {
			totalImpact += c.ImpactPercentage
		}
		if totalImpact > 0 {
			for i := range sortedContributors {
				sortedContributors[i].ImpactPercentage = sortedContributors[i].ImpactPercentage / totalImpact * 100.0
			}
		}
	}

	var filteredContributors []ContributorImpact
	var filteredAmounts []uint64

	for _, c := range sortedContributors {
		rawAmount := uint64(math.Floor(c.ImpactPercentage / 100.0 * float64(totalRewardAmount)))
		if rawAmount >= minAllocation {
			filteredContributors = append(filteredContributors, c)
			filteredAmounts = append(filteredAmounts, rawAmount)
		}
	}

	if len(filteredContributors) == 0 {
		return []models.AllocationEntry{
			{
				GithubUserID: sortedContributors[0].GithubUserID,
				Amount:       totalRewardAmount,
			},
		}, nil
	}

	var filteredSum uint64
	for _, amount := range filteredAmounts {
		filteredSum += amount
	}

	remainder := totalRewardAmount - filteredSum

	if len(filteredContributors) > 0 {
		filteredAmounts[0] += remainder
	}

	allocations := make([]models.AllocationEntry, len(filteredContributors))
	for i := range filteredContributors {
		allocations[i] = models.AllocationEntry{
			GithubUserID: filteredContributors[i].GithubUserID,
			Amount:       filteredAmounts[i],
		}
	}

	var sum uint64
	for _, a := range allocations {
		sum += a.Amount
	}

	if sum != totalRewardAmount {
		return nil, ErrAllocationSumMismatch
	}

	for _, a := range allocations {
		if a.Amount < minAllocation && len(allocations) > 1 {
			return nil, ErrAllBelowMinimum
		}
	}

	return allocations, nil
}

func CalculateMaxRecipients(
	serviceFee uint64,
	claimRecordRent uint64,
	minAllocation uint64,
	hardCap int,
	totalRewardAmount uint64,
) int {
	if totalRewardAmount == 0 {
		return 1
	}

	var economicLimit int
	if claimRecordRent > 0 {
		economicLimit = int(serviceFee / claimRecordRent)
	} else {
		economicLimit = hardCap
	}

	var rewardLimit int
	if minAllocation > 0 {
		rewardLimit = int(totalRewardAmount / minAllocation)
	} else {
		rewardLimit = hardCap
	}

	maxRecipients := economicLimit
	if rewardLimit < maxRecipients {
		maxRecipients = rewardLimit
	}
	if hardCap < maxRecipients {
		maxRecipients = hardCap
	}

	if maxRecipients < 1 {
		maxRecipients = 1
	}

	return maxRecipients
}
