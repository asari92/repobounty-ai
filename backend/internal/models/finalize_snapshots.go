package models

import "time"

type FinalizeSnapshot struct {
	CampaignID               string         `json:"campaign_id"`
	Version                  int            `json:"version"`
	InputHash                string         `json:"input_hash"`
	AllocationMode           AllocationMode `json:"allocation_mode"`
	Contributors             []Contributor  `json:"contributors"`
	Allocations              []Allocation   `json:"allocations"`
	WindowStart              time.Time      `json:"window_start"`
	WindowEnd                time.Time      `json:"window_end"`
	ContributorSource        string         `json:"contributor_source"`
	ContributorNotes         string         `json:"contributor_notes,omitempty"`
	CreatedAt                time.Time      `json:"created_at"`
	ApprovedByGitHubUsername string         `json:"approved_by_github_username,omitempty"`
	ApprovedAt               *time.Time     `json:"approved_at,omitempty"`
}

type SnapshotSummary struct {
	Version                  int            `json:"version"`
	AllocationMode           AllocationMode `json:"allocation_mode"`
	WindowStart              time.Time      `json:"window_start"`
	WindowEnd                time.Time      `json:"window_end"`
	ContributorSource        string         `json:"contributor_source"`
	ContributorNotes         string         `json:"contributor_notes,omitempty"`
	CreatedAt                time.Time      `json:"created_at"`
	ApprovedByGitHubUsername string         `json:"approved_by_github_username,omitempty"`
	ApprovedAt               *time.Time     `json:"approved_at,omitempty"`
}

func (s *FinalizeSnapshot) Summary() SnapshotSummary {
	return SnapshotSummary{
		Version:                  s.Version,
		AllocationMode:           s.AllocationMode,
		WindowStart:              s.WindowStart,
		WindowEnd:                s.WindowEnd,
		ContributorSource:        s.ContributorSource,
		ContributorNotes:         s.ContributorNotes,
		CreatedAt:                s.CreatedAt,
		ApprovedByGitHubUsername: s.ApprovedByGitHubUsername,
		ApprovedAt:               s.ApprovedAt,
	}
}
