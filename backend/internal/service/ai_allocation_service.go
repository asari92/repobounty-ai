package service

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/yourusername/repobounty-ai/internal/domain/models"
)

type AIAllocationService struct{}

func (s *AIAllocationService) ComputeAllocations(contributors []models.Contributor, rewardPool float64) []models.Allocation {
	totalWeight := 0
	for _, c := range contributors {
		totalWeight += c.Commits + c.PRs*2
	}
	if totalWeight == 0 {
		return nil
	}

	allocations := make([]models.Allocation, 0, len(contributors))
	for _, c := range contributors {
		weight := c.Commits + c.PRs*2
		amount := float64(weight) / float64(totalWeight) * rewardPool
		allocations = append(allocations, models.Allocation{
			ID:          uuid.New(),
			Contributor: c.Username,
			Amount:      amount,
			Reason:      fmt.Sprintf("commits: %d, prs: %d", c.Commits, c.PRs),
		})
	}
	return allocations
}
