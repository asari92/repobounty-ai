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
	if solClient != nil && solClient.IsConfigured() {
		onChainCampaigns, err := solClient.ListCampaigns(ctx)
		if err != nil {
			logger.Warn("auto-finalize: failed to list on-chain campaigns, using store snapshot", zap.Error(err))
		} else if onChainCampaigns != nil {
			campaigns = onChainCampaigns
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

		result, err := (&Handlers{github: ghClient, ai: allocator}).calculateAllocations(ctx, c)
		if err != nil {
			retries[c.CampaignID]++
			logger.Error("auto-finalize: allocation failed",
				zap.String("campaign_id", c.CampaignID),
				zap.Int("attempt", retries[c.CampaignID]),
				zap.Error(err),
			)
			continue
		}

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
