package http

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/solana"
	"github.com/repobounty/repobounty-ai/internal/store"
	"go.uber.org/zap"
)

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

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		logger.Info("auto-finalize worker started", zap.Duration("interval", interval))

		for {
			select {
			case <-ticker.C:
				autoFinalize(ctx, campaignStore, ghClient, allocator, solClient, logger)
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
) {
	campaigns := campaignStore.List()
	now := time.Now()

	for _, c := range campaigns {
		if c.State != models.StateFunded {
			continue
		}
		if now.Before(c.Deadline) {
			continue
		}

		logger.Info("auto-finalizing campaign",
			zap.String("campaign_id", c.CampaignID),
			zap.String("repo", c.Repo),
		)

		contributors, err := ghClient.FetchContributors(ctx, c.Repo)
		if err != nil {
			logger.Error("auto-finalize: github fetch failed",
				zap.String("campaign_id", c.CampaignID),
				zap.Error(err),
			)
			continue
		}

		contributorPRDiffs, err := ghClient.FetchContributorsPRDiffs(ctx, c.Repo, c.CreatedAt.Unix())
		if err != nil {
			log.Printf("auto-finalize: PR diff fetch failed (%v), falling back to metric-based allocation", err)
		}

		var allocations []models.Allocation
		if len(contributorPRDiffs) > 0 {
			allocations, err = allocator.EvaluateCodeImpact(ctx, c.Repo, contributorPRDiffs, c.PoolAmount)
			if err != nil {
				log.Printf("auto-finalize: code impact evaluation failed (%v), falling back", err)
				allocations = nil
			}
		}

		if allocations == nil {
			allocations, err = allocator.Allocate(ctx, c.Repo, contributors, c.PoolAmount)
			if err != nil {
				logger.Error("auto-finalize: AI allocation failed",
					zap.String("campaign_id", c.CampaignID),
					zap.Error(err),
				)
				continue
			}
		}

		solanaInputs := make([]solana.AllocationInput, len(allocations))
		for i, a := range allocations {
			solanaInputs[i] = solana.AllocationInput{
				Contributor: a.Contributor,
				Percentage:  a.Percentage,
			}
		}

		txSig, err := solClient.FinalizeCampaign(ctx, c.CampaignID, solanaInputs)
		if err != nil {
			logger.Error("auto-finalize: solana finalize failed",
				zap.String("campaign_id", c.CampaignID),
				zap.Error(err),
			)
			continue
		}

		finalizedAt := time.Now()
		c.State = models.StateFinalized
		c.Allocations = allocations
		c.FinalizedAt = &finalizedAt
		c.TxSignature = txSig

		if err := campaignStore.Update(c); err != nil {
			logger.Error("auto-finalize: store update failed",
				zap.String("campaign_id", c.CampaignID),
				zap.Error(err),
			)
			continue
		}

		explorerURL := fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", txSig)
		logger.Info("auto-finalize: campaign finalized",
			zap.String("campaign_id", c.CampaignID),
			zap.String("tx", explorerURL),
			zap.Int("allocations", len(allocations)),
		)
	}
}
