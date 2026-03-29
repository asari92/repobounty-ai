package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/auth"
	"github.com/repobounty/repobounty-ai/internal/config"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/solana"
	"github.com/repobounty/repobounty-ai/internal/store"
)

var repoPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)

type Handlers struct {
	store       *store.Store
	github      *github.Client
	solana      *solana.Client
	ai          *ai.Allocator
	jwt         *auth.JWTManager
	githubOAuth *auth.GitHubOAuth
	config      *config.Config
}

func NewHandlers(
	s *store.Store,
	gh *github.Client,
	sol *solana.Client,
	alloc *ai.Allocator,
	jwt *auth.JWTManager,
	githubOAuth *auth.GitHubOAuth,
	config *config.Config,
) *Handlers {
	return &Handlers{
		store:       s,
		github:      gh,
		solana:      sol,
		ai:          alloc,
		jwt:         jwt,
		githubOAuth: githubOAuth,
		config:      config,
	}
}

func (h *Handlers) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns, err := h.listCampaigns(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("failed to load campaigns: %v", err))
		return
	}
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
		Authority:   "",
		Sponsor:     req.SponsorWallet,
		Allocations: []models.Allocation{},
		CreatedAt:   now,
		TxSignature: txSig,
	}

	if err := h.store.Create(campaign); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store campaign")
		return
	}

	writeJSON(w, http.StatusCreated, models.CreateCampaignResponse{
		CampaignID:   campaign.CampaignID,
		CampaignPDA:  "",
		VaultAddress: "",
		Repo:         campaign.Repo,
		PoolAmount:   campaign.PoolAmount,
		Deadline:     campaign.Deadline.Format(time.RFC3339),
		State:        campaign.State,
		TxSignature:  txSig,
	})
}

func (h *Handlers) GetCampaign(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	campaign, err := h.loadCampaign(r.Context(), id)
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
	campaign, err := h.loadCampaign(r.Context(), id)
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
	campaign, err := h.loadCampaign(r.Context(), id)
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
		if errors.Is(err, store.ErrNotFound) {
			if createErr := h.store.Create(campaign); createErr != nil {
				log.Printf("store create failed after finalization: %v", createErr)
			}
		} else {
			log.Printf("store update failed after finalization: %v", err)
		}
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
	json.NewEncoder(w).Encode(data)
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

func (h *Handlers) listCampaigns(ctx context.Context) ([]*models.Campaign, error) {
	if h.solana != nil && h.solana.IsConfigured() {
		campaigns, err := h.solana.ListCampaigns(ctx)
		if err != nil {
			return nil, err
		}
		if campaigns != nil {
			return campaigns, nil
		}
	}
	return h.store.List(), nil
}

func (h *Handlers) loadCampaign(ctx context.Context, id string) (*models.Campaign, error) {
	campaign, err := h.store.Get(id)
	if err == nil {
		return campaign, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	if h.solana != nil && h.solana.IsConfigured() {
		campaign, err := h.solana.GetCampaign(ctx, id)
		if err == nil {
			return campaign, nil
		}
	}

	return nil, store.ErrNotFound
}

func (h *Handlers) GetGitHubAuthURL(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	authURL := h.githubOAuth.GetAuthURL(state)
	json.NewEncoder(w).Encode(map[string]string{"url": authURL, "state": state})
}

func (h *Handlers) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	_ = r.URL.Query().Get("state")

	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code parameter")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	user, _, err := h.githubOAuth.ExchangeCode(ctx, code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to exchange code: "+err.Error())
		return
	}

	existingUser, _ := h.store.GetUser(user.Login)
	if existingUser == nil {
		newUser := &store.User{
			GitHubUsername: user.Login,
			WalletAddress:  "",
			GitHubID:       user.ID,
			Email:          user.Email,
			AvatarURL:      user.AvatarURL,
		}
		if err := h.store.CreateUser(newUser); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create user: "+err.Error())
			return
		}
		existingUser = newUser
	}

	token, err := h.jwt.GenerateToken(existingUser.GitHubUsername)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token: "+err.Error())
		return
	}

	userModel := &models.User{
		GitHubUsername: existingUser.GitHubUsername,
		GitHubID:       existingUser.GitHubID,
		AvatarURL:      existingUser.AvatarURL,
		WalletAddress:  existingUser.WalletAddress,
		CreatedAt:      time.Now(),
	}
	response := models.GitHubAuthResponse{
		Token: token,
		User:  *userModel,
	}
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	json.NewEncoder(w).Encode(user)
}

func (h *Handlers) LinkWallet(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.LinkWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user.WalletAddress = req.WalletAddress
	if err := h.store.UpdateUser(user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user: "+err.Error())
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
