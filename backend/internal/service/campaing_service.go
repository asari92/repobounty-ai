package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/repobounty-ai/internal/domain/models"
)

type CampaingRepository interface {
	Save(ctx context.Context, campaign *models.Campaign) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Campaign, error)
}

type githubProvider interface {
	GetContributors(ctx context.Context, repoURL string) ([]models.Contributor, error)
}

type aiAllocator interface {
	ComputeAllocations(id uuid.UUID, contributors []models.Contributor, rewardPool float64) []models.Allocation
}

type CampaignService struct {
	repo   CampaingRepository
	github githubProvider
	ai     aiAllocator
	solana SolanaClient
}

func NewCampaignService(repo CampaingRepository, github githubProvider, ai aiAllocator, solana SolanaClient) *CampaignService {
	return &CampaignService{
		repo:   repo,
		github: github,
		ai:     ai,
		solana: solana,
	}
}

func (s *CampaignService) GetCampaign(ctx context.Context, id uuid.UUID) (*models.Campaign, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *CampaignService) CreateCampaign(ctx context.Context, repoURL string, rewardPool float64, deadline time.Time) (*models.Campaign, error) {
	campaign := &models.Campaign{
		ID:         uuid.New(),
		RepoURL:    repoURL,
		RewardPool: rewardPool,
		Deadline:   deadline,
		Status:     models.CampaignStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.repo.Save(ctx, campaign); err != nil {
		return nil, err
	}
	return campaign, nil
}

func (s *CampaignService) FinalizeCampaign(ctx context.Context, id uuid.UUID) error {
	campaign, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	contributors, err := s.github.GetContributors(ctx, campaign.RepoURL)
	if err != nil {
		return err
	}

	allocations := s.ai.ComputeAllocations(campaign.ID, contributors, campaign.RewardPool)
	for i := range allocations {
		allocations[i].CampaignID = campaign.ID
	}

	if err := s.solana.FinalizeCampaign(ctx, campaign.ID, allocations); err != nil {
		return err
	}

	campaign.Allocations = allocations
	campaign.Status = models.CampaignStatusFinalized
	campaign.UpdatedAt = time.Now()
	return s.repo.Save(ctx, campaign)
}
