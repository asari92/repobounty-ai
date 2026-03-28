package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	CampaignStatusPending   = "pending"
	CampaignStatusFinalized = "finalized"
)

type Campaign struct {
	ID          uuid.UUID    `json:"id" validate:"required"`
	RepoURL     string       `json:"repo_url" validate:"required,url"`
	RewardPool  float64      `json:"reward_pool" validate:"required,gt=0"`
	Deadline    time.Time    `json:"deadline" validate:"required,datetime=2006-01-02T15:04:05Z07:00"`
	Status      string       `json:"status"`
	Allocations []Allocation `json:"allocations,omitempty"`
}
