package types

import "errors"

var (
	ErrCampaignNotFound        = errors.New("campaign not found")
	ErrCapaignAlreadyFinalized = errors.New("campaign already finalized")
)
