package http

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/store"
)

var errSnapshotNotFound = errors.New("finalize snapshot not found")
var errSnapshotStale = errors.New("finalize snapshot is stale")
var errSnapshotNotApproved = errors.New("finalize snapshot has not been approved")

const snapshotInputVersion = "repository_history_mvp_v1"

type allocationOptions struct {
	forceDeterministic bool
}

func campaignContributionWindow(campaign *models.Campaign) (time.Time, time.Time) {
	return campaign.CreatedAt.UTC(), campaign.Deadline.UTC()
}

func buildSnapshotInputHash(campaign *models.Campaign) string {
	start, end := campaignContributionWindow(campaign)
	hash := sha256.Sum256([]byte(
		snapshotInputVersion + "|" +
			campaign.CampaignID + "|" +
			campaign.Repo + "|" +
			campaign.Sponsor + "|" +
			campaign.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" +
			start.Format(time.RFC3339Nano) + "|" +
			end.Format(time.RFC3339Nano) + "|" +
			string(campaign.State),
	))
	return hex.EncodeToString(hash[:])
}

func (h *Handlers) createFinalizeSnapshot(
	campaign *models.Campaign,
	result *allocationResult,
	approvedBy string,
) (*models.FinalizeSnapshot, error) {
	now := time.Now().UTC()
	snapshot := &models.FinalizeSnapshot{
		CampaignID:        campaign.CampaignID,
		InputHash:         buildSnapshotInputHash(campaign),
		AllocationMode:    result.allocationMode,
		Contributors:      result.contributors,
		Allocations:       result.allocations,
		WindowStart:       result.windowStart.UTC(),
		WindowEnd:         result.windowEnd.UTC(),
		ContributorSource: result.contributorSource,
		ContributorNotes:  result.contributorNotes,
		CreatedAt:         now,
	}
	if approvedBy != "" {
		snapshot.ApprovedByGitHubUsername = approvedBy
		snapshot.ApprovedAt = &now
	}

	if err := h.store.SaveFinalizeSnapshot(snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (h *Handlers) loadFinalizeSnapshot(
	campaign *models.Campaign,
	requireApproved bool,
) (*models.FinalizeSnapshot, error) {
	snapshot, err := h.store.GetLatestFinalizeSnapshot(campaign.CampaignID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errSnapshotNotFound
		}
		return nil, err
	}
	if snapshot.InputHash != buildSnapshotInputHash(campaign) {
		return nil, errSnapshotStale
	}
	if requireApproved && snapshot.ApprovedAt == nil {
		return nil, errSnapshotNotApproved
	}
	return snapshot, nil
}

func snapshotToAllocationResult(snapshot *models.FinalizeSnapshot) *allocationResult {
	return &allocationResult{
		contributors:      snapshot.Contributors,
		allocations:       snapshot.Allocations,
		allocationMode:    snapshot.AllocationMode,
		windowStart:       snapshot.WindowStart,
		windowEnd:         snapshot.WindowEnd,
		contributorSource: snapshot.ContributorSource,
		contributorNotes:  snapshot.ContributorNotes,
	}
}
