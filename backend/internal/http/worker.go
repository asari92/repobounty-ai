package http

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/solana"
	"github.com/repobounty/repobounty-ai/internal/store"
	"go.uber.org/zap"
)

const maxAutoFinalizeRetries = 3

func StartAutoFinalizeWorker(
	ctx context.Context,
	campaignStore store.CampaignStore,
	ghClient *github.Client,
	allocator *ai.Allocator,
	solClient *solana.Client,
	logger *zap.Logger,
	interval time.Duration,
) {
	if interval == 0 {
		interval = 5 * time.Minute
	}

	retries := make(map[string]int) // campaign_id -> failure count

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("auto-finalize worker panic", zap.Any("recover", r))
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		logger.Info("auto-finalize worker started", zap.Duration("interval", interval))

		for {
			select {
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Error("auto-finalize tick panic", zap.Any("recover", r))
						}
					}()
					autoFinalize(ctx, campaignStore, ghClient, allocator, solClient, logger, retries)
				}()
			case <-ctx.Done():
				logger.Info("auto-finalize worker stopping")
				return
			}
		}
	}()
}

func autoFinalize(
	ctx context.Context,
	campaignStore store.CampaignStore,
	ghClient *github.Client,
	allocator *ai.Allocator,
	solClient *solana.Client,
	logger *zap.Logger,
	retries map[string]int,
) {
	campaigns := campaignStore.List()
	helper := &Handlers{
		store:  campaignStore,
		github: ghClient,
		ai:     allocator,
	}
	if solClient != nil && solClient.IsConfigured() {
		onChainCampaigns, err := solClient.ListCampaigns(ctx)
		if err != nil {
			logger.Warn("auto-finalize: failed to list on-chain campaigns, using store snapshot", zap.Error(err))
		} else if onChainCampaigns != nil {
			campaigns = mergeAutoFinalizeCampaigns(campaigns, onChainCampaigns)
		}
	}
	now := time.Now()

	for _, c := range campaigns {
		if c.State != models.StateFunded {
			continue
		}
		if now.Before(c.Deadline) {
			continue
		}
		if retries[c.CampaignID] >= maxAutoFinalizeRetries {
			delete(retries, c.CampaignID)
			continue
		}

		logger.Info("auto-finalizing campaign",
			zap.String("campaign_id", c.CampaignID),
			zap.String("repo", c.Repo),
		)

		snapshot, err := helper.loadFinalizeSnapshot(c, true)
		if err != nil {
			if !errors.Is(err, errSnapshotNotFound) &&
				!errors.Is(err, errSnapshotNotApproved) &&
				!errors.Is(err, errSnapshotStale) {
				retries[c.CampaignID]++
				logger.Error("auto-finalize: snapshot load failed",
					zap.String("campaign_id", c.CampaignID),
					zap.Int("attempt", retries[c.CampaignID]),
					zap.Error(err),
				)
				continue
			}

			snapshot, err = helper.loadFinalizeSnapshot(c, false)
			if err != nil {
				if !errors.Is(err, errSnapshotNotFound) && !errors.Is(err, errSnapshotStale) {
					retries[c.CampaignID]++
					logger.Error("auto-finalize: current snapshot load failed",
						zap.String("campaign_id", c.CampaignID),
						zap.Int("attempt", retries[c.CampaignID]),
						zap.Error(err),
					)
					continue
				}

				result, calcErr := helper.calculateAllocations(ctx, c, allocationOptions{forceDeterministic: true})
				if calcErr != nil {
					retries[c.CampaignID]++
					logger.Error("auto-finalize: deterministic allocation failed",
						zap.String("campaign_id", c.CampaignID),
						zap.Int("attempt", retries[c.CampaignID]),
						zap.Error(calcErr),
					)
					continue
				}

				snapshot, err = helper.createFinalizeSnapshot(c, result, "")
				if err != nil {
					retries[c.CampaignID]++
					logger.Error("auto-finalize: snapshot persistence failed",
						zap.String("campaign_id", c.CampaignID),
						zap.Int("attempt", retries[c.CampaignID]),
						zap.Error(err),
					)
					continue
				}
			}
		}
		result := snapshotToAllocationResult(snapshot)

		solanaInputs := make([]solana.AllocationInput, len(result.allocations))
		for i, a := range result.allocations {
			solanaInputs[i] = solana.AllocationInput{
				Contributor: a.Contributor,
				Percentage:  a.Percentage,
			}
		}

		txSig, err := solClient.FinalizeCampaign(ctx, c.CampaignID, solanaInputs)
		if err != nil {
			retries[c.CampaignID]++
			logger.Error("auto-finalize: solana finalize failed",
				zap.String("campaign_id", c.CampaignID),
				zap.Int("attempt", retries[c.CampaignID]),
				zap.Error(err),
			)
			continue
		}

		finalizedAt := time.Now()
		c.State = models.StateFinalized
		c.Allocations = result.allocations
		c.FinalizedAt = &finalizedAt
		c.TxSignature = txSig

		if err := campaignStore.Update(c); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				if createErr := campaignStore.Create(c); createErr != nil {
					logger.Error("auto-finalize: store create failed after on-chain finalization",
						zap.String("campaign_id", c.CampaignID),
						zap.Error(createErr),
					)
					continue
				}
			} else {
				logger.Error("auto-finalize: store update failed",
					zap.String("campaign_id", c.CampaignID),
					zap.Error(err),
				)
				continue
			}
		}

		delete(retries, c.CampaignID) // clear retry counter on success

		explorerURL := fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", txSig)
		logger.Info("auto-finalize: campaign finalized",
			zap.String("campaign_id", c.CampaignID),
			zap.String("tx", explorerURL),
			zap.Int("allocations", len(result.allocations)),
		)
	}
}

func mergeAutoFinalizeCampaigns(storedCampaigns, onChainCampaigns []*models.Campaign) []*models.Campaign {
	storedByID := make(map[string]*models.Campaign, len(storedCampaigns))
	for _, campaign := range storedCampaigns {
		storedByID[campaign.CampaignID] = campaign
	}

	merged := make([]*models.Campaign, 0, len(onChainCampaigns)+len(storedCampaigns))
	for _, onChainCampaign := range onChainCampaigns {
		if storedCampaign, ok := storedByID[onChainCampaign.CampaignID]; ok {
			merged = append(merged, mergeCampaignWithChainData(storedCampaign, onChainCampaign))
			delete(storedByID, onChainCampaign.CampaignID)
			continue
		}
		merged = append(merged, onChainCampaign)
	}

	for _, storedCampaign := range storedByID {
		merged = append(merged, storedCampaign)
	}

	return merged
}

// StartMirrorSyncWorker starts a background goroutine that periodically re-syncs
// mirrors for all campaigns that need an update.
func StartMirrorSyncWorker(
	ctx context.Context,
	handlers *Handlers,
	logger *zap.Logger,
	interval time.Duration,
) {
	if handlers == nil || !handlers.config.MirrorEnabled {
		return
	}
	if interval == 0 {
		interval = 24 * time.Hour
	}

	maxConcurrent := handlers.config.MirrorMaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("mirror sync worker panic", zap.Any("recover", r))
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		logger.Info("mirror sync worker started", zap.Duration("interval", interval))

		for {
			select {
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							logger.Error("mirror sync tick panic", zap.Any("recover", r))
						}
					}()
					runMirrorSyncTick(ctx, handlers, logger, maxConcurrent)
				}()
			case <-ctx.Done():
				logger.Info("mirror sync worker stopping")
				return
			}
		}
	}()
}

func runMirrorSyncTick(
	ctx context.Context,
	handlers *Handlers,
	logger *zap.Logger,
	maxConcurrent int,
) {
	mirrors, err := handlers.store.ListMirrorsNeedingSync()
	if err != nil {
		logger.Error("mirror sync tick: list mirrors failed", zap.Error(err))
		return
	}
	if len(mirrors) == 0 {
		return
	}

	semaphore := make(chan struct{}, maxConcurrent)
	for _, m := range mirrors {
		campaignID := m.CampaignID
		semaphore <- struct{}{}
		go func() {
			defer func() { <-semaphore }()
			defer func() {
				if r := recover(); r != nil {
					logger.Error("mirror sync goroutine panic",
						zap.String("campaign_id", campaignID),
						zap.Any("recover", r),
					)
				}
			}()
			handlers.runMirrorSync(ctx, campaignID)
		}()
	}
}
