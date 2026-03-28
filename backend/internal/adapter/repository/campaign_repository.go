package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/yourusername/repobounty-ai/internal/domain/models"
	"github.com/yourusername/repobounty-ai/internal/domain/types"
)

type CampaignRepositoryImpl struct {
	Campiagns map[uuid.UUID]*models.Campaign
}

func (r *CampaignRepositoryImpl) Save(ctx context.Context, campaign *models.Campaign) error {
	r.Campiagns[campaign.ID] = campaign
	return nil
}

func (r *CampaignRepositoryImpl) GetByID(ctx context.Context, id uuid.UUID) (*models.Campaign, error) {
	campaign, exists := r.Campiagns[id]
	if !exists {
		return nil, types.ErrCampaignNotFound
	}
	return campaign, nil
}
