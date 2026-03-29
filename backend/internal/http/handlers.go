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
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/auth"
	"github.com/repobounty/repobounty-ai/internal/config"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/githubapp"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/solana"
	"github.com/repobounty/repobounty-ai/internal/store"
)

var repoPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)
var base58Pattern = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)

func isValidSolanaAddress(addr string) bool {
	return base58Pattern.MatchString(addr)
}

type Handlers struct {
	store       store.CampaignStore
	github      *github.Client
	solana      *solana.Client
	ai          *ai.Allocator
	jwt         *auth.JWTManager
	githubOAuth *auth.GitHubOAuth
	config      *config.Config

	oauthStates   map[string]time.Time // state -> expiry
	oauthStatesMu sync.Mutex

	claimLocks   map[string]*sync.Mutex // campaign_id -> lock
	claimLocksMu sync.Mutex
}

func NewHandlers(
	s store.CampaignStore,
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
		oauthStates: make(map[string]time.Time),
		claimLocks:  make(map[string]*sync.Mutex),
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
	if req.SponsorWallet != "" && !isValidSolanaAddress(req.SponsorWallet) {
		writeError(w, http.StatusBadRequest, "invalid sponsor wallet address")
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

	campaignID := uuid.New().String()[:12]

	txSig, campaignPDA, vaultPDA, err := h.solana.CreateCampaign(
		r.Context(),
		campaignID,
		req.Repo,
		req.PoolAmount,
		deadline.Unix(),
		req.SponsorWallet,
	)
	if err != nil {
		log.Printf("solana create_campaign failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create campaign on-chain")
		return
	}

	now := time.Now()
	campaign := &models.Campaign{
		CampaignID:   campaignID,
		CampaignPDA:  campaignPDA,
		VaultAddress: vaultPDA,
		Repo:         req.Repo,
		PoolAmount:   req.PoolAmount,
		Deadline:     deadline,
		State:        models.StateCreated,
		Authority:    "",
		Sponsor:      req.SponsorWallet,
		Allocations:  []models.Allocation{},
		CreatedAt:    now,
		TxSignature:  txSig,
	}

	if err := h.store.Create(campaign); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store campaign")
		return
	}

	writeJSON(w, http.StatusCreated, models.CreateCampaignResponse{
		CampaignID:   campaign.CampaignID,
		CampaignPDA:  campaign.CampaignPDA,
		VaultAddress: campaign.VaultAddress,
		Repo:         campaign.Repo,
		PoolAmount:   campaign.PoolAmount,
		Deadline:     campaign.Deadline.Format(time.RFC3339),
		State:        campaign.State,
		TxSignature:  txSig,
	})
}

type fundTxRequest struct {
	SponsorWallet string `json:"sponsor_wallet"`
}

func (h *Handlers) FundTx(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req fundTxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SponsorWallet == "" {
		writeError(w, http.StatusBadRequest, "sponsor_wallet is required")
		return
	}

	campaign, err := h.store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if campaign.Sponsor != req.SponsorWallet {
		writeError(w, http.StatusForbidden, "only the campaign sponsor can fund")
		return
	}
	if campaign.State != models.StateCreated {
		writeError(w, http.StatusBadRequest, "campaign is not in created state")
		return
	}

	fundTx, err := h.solana.BuildFundTransaction(r.Context(), id, campaign.PoolAmount, req.SponsorWallet)
	if err != nil {
		log.Printf("solana build_fund_tx failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to build fund transaction")
		return
	}

	writeJSON(w, http.StatusOK, fundTx)
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

	if time.Now().Before(campaign.Deadline) {
		writeError(w, http.StatusConflict, "campaign deadline has not been reached yet")
		return
	}

	contributors, err := h.github.FetchContributors(r.Context(), campaign.Repo)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("github fetch failed: %v", err))
		return
	}

	contributorPRDiffs, err := h.github.FetchContributorsPRDiffs(r.Context(), campaign.Repo, campaign.CreatedAt.Unix())
	if err != nil {
		log.Printf("github: PR diff fetch failed (%v), falling back to metric-based allocation", err)
	}

	var allocations []models.Allocation
	if len(contributorPRDiffs) > 0 {
		allocations, err = h.ai.EvaluateCodeImpact(r.Context(), campaign.Repo, contributorPRDiffs, campaign.PoolAmount)
		if err != nil {
			log.Printf("ai: code impact evaluation failed (%v), falling back to metric-based allocation", err)
			allocations = nil
		}
	}

	if allocations == nil {
		allocations, err = h.ai.Allocate(r.Context(), campaign.Repo, contributors, campaign.PoolAmount)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("AI allocation failed: %v", err))
			return
		}
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
				log.Printf("CRITICAL: store create failed after on-chain finalization (campaign=%s, tx=%s): %v", campaign.CampaignID, txSig, createErr)
			}
		} else {
			log.Printf("CRITICAL: store update failed after on-chain finalization (campaign=%s, tx=%s): %v", campaign.CampaignID, txSig, err)
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

	go func() {
		appClient := githubapp.NewClient(h.config.GitHubAppID, h.config.GitHubAppPrivateKey)
		appAllocations := make([]githubapp.Allocation, len(allocations))
		for i, a := range allocations {
			appAllocations[i] = githubapp.Allocation{
				Contributor: a.Contributor,
				Percentage:  a.Percentage,
				Amount:      a.Amount,
				Claimed:     a.Claimed,
			}
		}
		githubapp.PostAllocationComments(
			r.Context(),
			appClient,
			campaign.Repo,
			campaign.CampaignID,
			appAllocations,
			h.config.FrontendURL,
		)
	}()
}

func (h *Handlers) Claim(w http.ResponseWriter, r *http.Request) {
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

	if campaign.State != models.StateFinalized && campaign.State != models.StateCompleted {
		writeError(w, http.StatusConflict, "campaign is not finalized")
		return
	}

	// Per-campaign lock to prevent race conditions on concurrent claims
	h.claimLocksMu.Lock()
	mu, exists := h.claimLocks[id]
	if !exists {
		mu = &sync.Mutex{}
		h.claimLocks[id] = mu
	}
	h.claimLocksMu.Unlock()
	mu.Lock()
	defer mu.Unlock()

	// Re-load campaign under lock to get fresh state
	campaign, err = h.loadCampaign(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		ContributorGithub string `json:"contributor_github"`
		WalletAddress     string `json:"wallet_address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ContributorGithub == "" || req.WalletAddress == "" {
		writeError(w, http.StatusBadRequest, "contributor_github and wallet_address are required")
		return
	}
	if !isValidSolanaAddress(req.WalletAddress) {
		writeError(w, http.StatusBadRequest, "invalid wallet address format")
		return
	}

	if user.GitHubUsername != req.ContributorGithub {
		writeError(w, http.StatusForbidden, "can only claim your own allocation")
		return
	}

	var matchedAlloc *models.Allocation
	for i := range campaign.Allocations {
		if campaign.Allocations[i].Contributor == req.ContributorGithub {
			matchedAlloc = &campaign.Allocations[i]
			break
		}
	}
	if matchedAlloc == nil {
		writeError(w, http.StatusNotFound, "contributor not found in allocations")
		return
	}
	if matchedAlloc.Claimed {
		writeError(w, http.StatusConflict, "allocation already claimed")
		return
	}

	txSig, err := h.solana.ClaimAllocation(r.Context(), campaign.CampaignID, req.ContributorGithub, req.WalletAddress)
	if err != nil {
		log.Printf("solana claim failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to claim on-chain")
		return
	}

	matchedAlloc.Claimed = true
	matchedAlloc.ClaimantWallet = req.WalletAddress
	campaign.TotalClaimed += matchedAlloc.Amount

	allClaimed := true
	for _, a := range campaign.Allocations {
		if !a.Claimed {
			allClaimed = false
			break
		}
	}
	if allClaimed {
		campaign.State = models.StateCompleted
	}

	if err := h.store.Update(campaign); err != nil {
		log.Printf("claim: store update failed for campaign %s: %v", campaign.CampaignID, err)
	}

	explorerURL := fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", txSig)
	writeJSON(w, http.StatusOK, models.FinalizeResponse{
		CampaignID:        campaign.CampaignID,
		State:             campaign.State,
		Allocations:       campaign.Allocations,
		TxSignature:       txSig,
		SolanaExplorerURL: explorerURL,
	})
}

func (h *Handlers) ClaimPermit(w http.ResponseWriter, r *http.Request) {
	h.Claim(w, r)
}

func (h *Handlers) GetClaims(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	type claimItem struct {
		CampaignID     string `json:"campaign_id"`
		Repo           string `json:"repo"`
		Contributor    string `json:"contributor"`
		Percentage     uint16 `json:"percentage"`
		Amount         uint64 `json:"amount"`
		AmountSOL      string `json:"amount_sol"`
		Claimed        bool   `json:"claimed"`
		ClaimantWallet string `json:"claimant_wallet,omitempty"`
		State          string `json:"state"`
	}

	var items []claimItem
	for _, campaign := range h.store.List() {
		if campaign.State != models.StateFinalized && campaign.State != models.StateCompleted {
			continue
		}
		for _, alloc := range campaign.Allocations {
			if alloc.Contributor == user.GitHubUsername && !alloc.Claimed {
				items = append(items, claimItem{
					CampaignID:  campaign.CampaignID,
					Repo:        campaign.Repo,
					Contributor: alloc.Contributor,
					Percentage:  alloc.Percentage,
					Amount:      alloc.Amount,
					AmountSOL:   fmt.Sprintf("%.4f", float64(alloc.Amount)/1e9),
					Claimed:     alloc.Claimed,
					State:       string(campaign.State),
				})
			}
		}
	}

	if items == nil {
		items = []claimItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(items)
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

	h.oauthStatesMu.Lock()
	// Clean expired states
	now := time.Now()
	for k, exp := range h.oauthStates {
		if now.After(exp) {
			delete(h.oauthStates, k)
		}
	}
	h.oauthStates[state] = now.Add(10 * time.Minute)
	h.oauthStatesMu.Unlock()

	authURL := h.githubOAuth.GetAuthURL(state)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"auth_url": authURL, "state": state})
}

func (h *Handlers) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	var req models.GitHubAuthRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		code := r.URL.Query().Get("code")
		if code != "" {
			req.Code = code
			req.State = r.URL.Query().Get("state")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	code := req.Code

	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code parameter")
		return
	}

	// Validate OAuth state to prevent CSRF
	if req.State == "" {
		writeError(w, http.StatusBadRequest, "missing state parameter")
		return
	}
	h.oauthStatesMu.Lock()
	expiry, exists := h.oauthStates[req.State]
	if exists {
		delete(h.oauthStates, req.State) // one-time use
	}
	h.oauthStatesMu.Unlock()
	if !exists || time.Now().After(expiry) {
		writeError(w, http.StatusBadRequest, "invalid or expired state parameter")
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

func generateState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
