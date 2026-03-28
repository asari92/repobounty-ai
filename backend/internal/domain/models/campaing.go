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
	ID uuid.UUID `json:"id" validate:"required"`

	RepoURL    string    `json:"repo_url" validate:"required,url"`
	RewardPool float64   `json:"reward_pool" validate:"required,gt=0"`
	Deadline   time.Time `json:"deadline" validate:"required,gt=now"`

	Status      string       `json:"status"`
	Allocations []Allocation `json:"allocations,omitempty"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
