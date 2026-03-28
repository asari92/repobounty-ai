package models

import (
	"time"

	"github.com/google/uuid"
)

type Allocation struct {
	ID         uuid.UUID `json:"id" validate:"required"`
	CampaignID uuid.UUID `json:"campaign_id" validate:"required"`

	Contributor string  `json:"contributor" validate:"required"`
	Amount      float64 `json:"amount" validate:"required,gt=0"`
	Reason      string  `json:"reason"`

	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
