package models

import (
	"github.com/google/uuid"
)

type Allocation struct {
	ID          uuid.UUID `json:"id" validate:"required"`
	CampaignID  uuid.UUID `json:"campaign_id" validate:"required"`
	Contributor string    `json:"contributor" validate:"required"`
	Amount      float64   `json:"amount" validate:"required,gt=0"`
	Reason      string    `json:"reason"`
}
