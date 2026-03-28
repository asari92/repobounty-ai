package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/yourusername/repobounty-ai/internal/domain/models"
)

type SolanaClient interface {
	FinalizeCampaign(ctx context.Context, campaignID uuid.UUID, allocations []models.Allocation) error
}

type MockSolanaService struct{}

func (s *MockSolanaService) FinalizeCampaign(ctx context.Context, campaignID uuid.UUID, allocations []models.Allocation) error {
	fmt.Printf("[Solana stub] finalize_campaign campaignID=%s allocations=%d\n", campaignID, len(allocations))
	for _, a := range allocations {
		fmt.Printf("  -> %s: %.4f SOL (%s)\n", a.Contributor, a.Amount, a.Reason)
	}
	return nil
}
