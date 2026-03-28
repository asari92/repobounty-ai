package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/solana"
	"github.com/repobounty/repobounty-ai/internal/store"
)

var repoPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)

type Handlers struct {
	store  *store.Store
	github *github.Client
	ai     *ai.Allocator
	solana *solana.Client
}

func NewHandlers(s *store.Store, gh *github.Client, alloc *ai.Allocator, sol *solana.Client) *Handlers {
	return &Handlers{store: s, github: gh, ai: alloc, solana: sol}
}

func (h *Handlers) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns := h.store.List()
	writeJSON(w, http.StatusOK, campaigns)
}

func (h *Handlers) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	var req models.CreateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !repoPattern.MatchString(req.Repo) {
		writeError(w, http.StatusBadRequest, "repo must be in owner/repo format")
		return
	}
	if req.PoolAmount == 0 {
		writeError(w, http.StatusBadRequest, "pool_amount must be greater than 0")
		return
	}

	deadline, err := time.Parse(time.RFC3339, req.Deadline)
	if err != nil {
		writeError(w, http.StatusBadRequest, "deadline must be RFC3339 format")
		return
	}
	if deadline.Before(time.Now()) {
		writeError(w, http.StatusBadRequest, "deadline must be in the future")
		return
	}

	campaignID := uuid.New().String()[:8]

	txSig, err := h.solana.CreateCampaign(
		r.Context(),
		campaignID,
		req.Repo,
		req.PoolAmount,
		deadline.Unix(),
	)
	if err != nil {
		log.Printf("solana create_campaign failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create campaign on-chain")
		return
	}

	now := time.Now()
	campaign := &models.Campaign{
		CampaignID:  campaignID,
		Repo:        req.Repo,
		PoolAmount:  req.PoolAmount,
		Deadline:    deadline,
		State:       models.StateCreated,
		Authority:   req.WalletAddress,
		Allocations: []models.Allocation{},
		CreatedAt:   now,
		TxSignature: txSig,
	}

	if err := h.store.Create(campaign); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store campaign")
		return
	}

	writeJSON(w, http.StatusCreated, models.CreateCampaignResponse{
		CampaignID:  campaign.CampaignID,
		Repo:        campaign.Repo,
		PoolAmount:  campaign.PoolAmount,
		Deadline:    campaign.Deadline.Format(time.RFC3339),
		State:       campaign.State,
		TxSignature: txSig,
	})
}

func (h *Handlers) GetCampaign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	campaign, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, campaign)
}

func (h *Handlers) FinalizePreview(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	campaign, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if campaign.State == models.StateFinalized {
		writeError(w, http.StatusConflict, "campaign already finalized")
		return
	}

	contributors, err := h.github.FetchContributors(r.Context(), campaign.Repo)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("github fetch failed: %v", err))
		return
	}

	allocations, err := h.ai.Allocate(r.Context(), campaign.Repo, contributors, campaign.PoolAmount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("AI allocation failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, models.FinalizePreviewResponse{
		CampaignID:   campaign.CampaignID,
		Repo:         campaign.Repo,
		Contributors: contributors,
		Allocations:  allocations,
		AIModel:      h.ai.Model(),
	})
}

func (h *Handlers) Finalize(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	campaign, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if campaign.State == models.StateFinalized {
		writeError(w, http.StatusConflict, "campaign already finalized")
		return
	}

	contributors, err := h.github.FetchContributors(r.Context(), campaign.Repo)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("github fetch failed: %v", err))
		return
	}

	allocations, err := h.ai.Allocate(r.Context(), campaign.Repo, contributors, campaign.PoolAmount)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("AI allocation failed: %v", err))
		return
	}

	solanaInputs := make([]solana.AllocationInput, len(allocations))
	for i, a := range allocations {
		solanaInputs[i] = solana.AllocationInput{
			Contributor: a.Contributor,
			Percentage:  a.Percentage,
		}
	}

	txSig, err := h.solana.FinalizeCampaign(r.Context(), campaign.CampaignID, solanaInputs)
	if err != nil {
		log.Printf("solana finalize_campaign failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to finalize on-chain")
		return
	}

	now := time.Now()
	campaign.State = models.StateFinalized
	campaign.Allocations = allocations
	campaign.FinalizedAt = &now
	campaign.TxSignature = txSig

	if err := h.store.Update(campaign); err != nil {
		logger := GetLogger()
		logger.Error("store update failed after on-chain finalization",
			zap.String("campaign_id", campaign.CampaignID),
			zap.String("tx_signature", txSig),
			zap.Error(err),
		)
		writeJSON(w, http.StatusOK, models.FinalizeResponse{
			CampaignID:        campaign.CampaignID,
			State:             models.StateFinalized,
			Allocations:       allocations,
			TxSignature:       txSig,
			SolanaExplorerURL: fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", txSig),
		})
		return
	}

	explorerURL := fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", txSig)

	writeJSON(w, http.StatusOK, models.FinalizeResponse{
		CampaignID:        campaign.CampaignID,
		State:             models.StateFinalized,
		Allocations:       allocations,
		TxSignature:       txSig,
		SolanaExplorerURL: explorerURL,
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("failed to encode JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, models.ErrorResponse{Error: msg})
}

func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"solana":   h.solana.IsConfigured(),
		"github":   h.github != nil,
		"ai_model": h.ai.Model(),
		"store":    h.store != nil,
	})
}
