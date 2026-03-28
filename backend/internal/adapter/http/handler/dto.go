package handler

import "time"

type createCampaignRequest struct {
	RepoURL    string    `json:"repo_url" binding:"required"`
	RewardPool float64   `json:"reward_pool" binding:"required"`
	Deadline   time.Time `json:"deadline" binding:"required"`
}
