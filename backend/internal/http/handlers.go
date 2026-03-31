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
	"sort"
	"strings"
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

const minCampaignLeadTime = 24 * time.Hour

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

	claimLocks sync.Map // campaign_id -> *sync.Mutex
}

type allocationResult struct {
	contributors      []models.Contributor
	allocations       []models.Allocation
	allocationMode    models.AllocationMode
	windowStart       time.Time
	windowEnd         time.Time
	contributorSource string
	contributorNotes  string
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
		claimLocks:  sync.Map{},
	}
}

func (h *Handlers) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	campaigns, err := h.listCampaigns(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load campaigns")
		return
	}
	writeJSON(w, http.StatusOK, campaigns)
}

func (h *Handlers) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if h.solana == nil || !h.solana.IsConfigured() {
		writeError(w, http.StatusServiceUnavailable, "campaign creation is disabled until Solana is configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req models.CreateCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	deadline, err := normalizeCreateChallengeRequest(&models.CreateCampaignChallengeRequest{
		Repo:          req.Repo,
		PoolAmount:    req.PoolAmount,
		Deadline:      req.Deadline,
		SponsorWallet: req.SponsorWallet,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Deadline = deadline.Format(time.RFC3339)

	challenge, err := h.loadAndVerifyWalletChallenge(
		models.WalletChallengeActionCreateCampaign,
		req.ChallengeID,
		req.SponsorWallet,
		req.Signature,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var challengePayload createCampaignChallengePayload
	if err := json.Unmarshal([]byte(challenge.PayloadJSON), &challengePayload); err != nil {
		log.Printf("create campaign: unmarshal wallet challenge payload failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to verify wallet proof")
		return
	}
	if challengePayload.GitHubUsername != user.GitHubUsername ||
		challengePayload.Repo != req.Repo ||
		challengePayload.PoolAmount != req.PoolAmount ||
		challengePayload.Deadline != req.Deadline ||
		challengePayload.SponsorWallet != req.SponsorWallet {
		writeError(w, http.StatusBadRequest, "wallet proof did not match this campaign request")
		return
	}

	now := time.Now().UTC()

	repoExists, err := h.github.RepositoryExists(r.Context(), req.Repo)
	if err != nil {
		log.Printf("github repository lookup failed for %s: %v", req.Repo, err)
		writeError(w, http.StatusBadGateway, "failed to verify repository on GitHub")
		return
	}
	if !repoExists {
		writeError(w, http.StatusBadRequest, "repository was not found or is not public")
		return
	}

	if h.solana != nil && h.solana.IsConfigured() {
		balance, err := h.solana.GetBalance(r.Context(), req.SponsorWallet)
		if err != nil {
			log.Printf("solana balance lookup failed for %s: %v", req.SponsorWallet, err)
			writeError(w, http.StatusBadGateway, "failed to verify sponsor wallet balance")
			return
		}
		if balance < req.PoolAmount {
			writeError(w, http.StatusBadRequest, "sponsor wallet does not have enough SOL to fund this campaign")
			return
		}
	}

	if deadline.Before(now) {
		writeError(w, http.StatusBadRequest, "deadline must be in the future")
		return
	}

	campaignID := uuid.New().String()[:12]
	campaign := &models.Campaign{
		CampaignID:          campaignID,
		Repo:                req.Repo,
		PoolAmount:          req.PoolAmount,
		Deadline:            deadline,
		State:               models.StateCreated,
		Authority:           h.solana.AuthorityAddress(),
		Sponsor:             req.SponsorWallet,
		OwnerGitHubUsername: user.GitHubUsername,
		Allocations:         []models.Allocation{},
		CreatedAt:           now,
	}

	if err := h.store.Create(campaign); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store campaign")
		return
	}
	if err := h.markWalletChallengeUsed(req.ChallengeID); err != nil {
		if deleteErr := h.store.DeleteCampaign(campaignID); deleteErr != nil {
			log.Printf("create campaign: rollback failed after challenge use error for %s: %v", campaignID, deleteErr)
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	txSig, campaignPDA, vaultPDA, err := h.solana.CreateCampaign(
		r.Context(),
		campaignID,
		req.Repo,
		req.PoolAmount,
		deadline.Unix(),
		req.SponsorWallet,
	)
	if err != nil {
		if deleteErr := h.store.DeleteCampaign(campaignID); deleteErr != nil {
			log.Printf("create campaign: rollback failed after on-chain create error for %s: %v", campaignID, deleteErr)
		}
		log.Printf("solana create_campaign failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create campaign on-chain")
		return
	}

	campaign.CampaignPDA = campaignPDA
	campaign.VaultAddress = vaultPDA
	campaign.TxSignature = txSig

	if err := h.store.Update(campaign); err != nil {
		log.Printf("WARNING: store update failed after on-chain campaign creation (campaign=%s, tx=%s): %v", campaign.CampaignID, txSig, err)
		writeJSON(w, http.StatusAccepted, models.CreateCampaignResponse{
			CampaignID:   campaign.CampaignID,
			CampaignPDA:  campaign.CampaignPDA,
			VaultAddress: campaign.VaultAddress,
			Repo:         campaign.Repo,
			PoolAmount:   campaign.PoolAmount,
			Deadline:     campaign.Deadline.Format(time.RFC3339),
			State:        campaign.State,
			TxSignature:  txSig,
		})
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
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if h.solana == nil || !h.solana.IsConfigured() {
		writeError(w, http.StatusServiceUnavailable, "campaign funding is unavailable until Solana is configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req fundTxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SponsorWallet == "" {
		writeError(w, http.StatusBadRequest, "sponsor_wallet is required")
		return
	}

	campaign, err := h.loadCampaign(r.Context(), id)
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
	if err := requireCampaignOwner(user, campaign, "fund"); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	fundTx, err := h.solana.BuildFundTransaction(r.Context(), id, campaign.PoolAmount, req.SponsorWallet)
	if err != nil {
		if errors.Is(err, solana.ErrNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "campaign funding is unavailable until Solana is configured")
			return
		}
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
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	campaign, err := h.loadCampaign(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := requireCampaignOwner(user, campaign, "preview allocations for"); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if status, msg := validateFinalizeState(campaign); status != http.StatusOK {
		writeError(w, status, msg)
		return
	}

	result, err := h.calculateAllocations(r.Context(), campaign, allocationOptions{})
	if err != nil {
		log.Printf("finalize preview: allocation failed for %s: %v", campaign.CampaignID, err)
		writeError(w, http.StatusInternalServerError, "failed to build allocation snapshot")
		return
	}

	snapshot, err := h.createFinalizeSnapshot(campaign, result, user.GitHubUsername)
	if err != nil {
		log.Printf("finalize preview: snapshot persistence failed for %s: %v", campaign.CampaignID, err)
		writeError(w, http.StatusInternalServerError, "failed to save allocation snapshot")
		return
	}

	writeJSON(w, http.StatusOK, models.FinalizePreviewResponse{
		CampaignID:     campaign.CampaignID,
		Repo:           campaign.Repo,
		Contributors:   result.contributors,
		Allocations:    result.allocations,
		AIModel:        h.ai.Model(),
		AllocationMode: result.allocationMode,
		Snapshot:       snapshot.Summary(),
	})
}

func (h *Handlers) Finalize(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	campaign, err := h.loadCampaign(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := requireCampaignOwner(user, campaign, "finalize"); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if status, msg := validateFinalizeState(campaign); status != http.StatusOK {
		writeError(w, status, msg)
		return
	}
	if h.solana == nil || !h.solana.IsConfigured() {
		writeError(w, http.StatusServiceUnavailable, "campaign finalization is unavailable until Solana is configured")
		return
	}

	snapshot, err := h.loadFinalizeSnapshot(campaign, true)
	if err != nil {
		switch {
		case errors.Is(err, errSnapshotNotFound), errors.Is(err, errSnapshotNotApproved):
			writeError(w, http.StatusConflict, "preview allocations before finalizing")
		case errors.Is(err, errSnapshotStale):
			writeError(w, http.StatusConflict, "saved preview is stale; run preview again")
		default:
			log.Printf("finalize: snapshot load failed for %s: %v", campaign.CampaignID, err)
			writeError(w, http.StatusInternalServerError, "failed to load allocation snapshot")
		}
		return
	}
	result := snapshotToAllocationResult(snapshot)
	snapshotSummary := snapshot.Summary()

	solanaInputs := make([]solana.AllocationInput, len(result.allocations))
	for i, a := range result.allocations {
		solanaInputs[i] = solana.AllocationInput{
			Contributor: a.Contributor,
			Percentage:  a.Percentage,
		}
	}

	txSig, err := h.solana.FinalizeCampaign(r.Context(), campaign.CampaignID, solanaInputs)
	if err != nil {
		if errors.Is(err, solana.ErrNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "campaign finalization is unavailable until Solana is configured")
			return
		}
		log.Printf("solana finalize_campaign failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to finalize on-chain")
		return
	}

	now := time.Now()
	campaign.State = models.StateFinalized
	campaign.Allocations = result.allocations
	campaign.FinalizedAt = &now
	campaign.TxSignature = txSig

	if err := h.store.Update(campaign); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			if createErr := h.store.Create(campaign); createErr != nil {
				log.Printf("WARNING: store update failed after on-chain finalization (campaign=%s, tx=%s): %v", campaign.CampaignID, txSig, createErr)
				explorerURL := fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", txSig)
				writeJSON(w, http.StatusAccepted, models.FinalizeResponse{
					CampaignID:        campaign.CampaignID,
					State:             models.StateFinalized,
					Allocations:       result.allocations,
					TxSignature:       txSig,
					SolanaExplorerURL: explorerURL,
					AllocationMode:    result.allocationMode,
					Snapshot:          &snapshotSummary,
				})
				return
			}
		} else {
			log.Printf("WARNING: store update failed after on-chain finalization (campaign=%s, tx=%s): %v", campaign.CampaignID, txSig, err)
			explorerURL := fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", txSig)
			writeJSON(w, http.StatusAccepted, models.FinalizeResponse{
				CampaignID:        campaign.CampaignID,
				State:             models.StateFinalized,
				Allocations:       result.allocations,
				TxSignature:       txSig,
				SolanaExplorerURL: explorerURL,
				AllocationMode:    result.allocationMode,
				Snapshot:          &snapshotSummary,
			})
			return
		}
	}

	explorerURL := fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", txSig)

	writeJSON(w, http.StatusOK, models.FinalizeResponse{
		CampaignID:        campaign.CampaignID,
		State:             models.StateFinalized,
		Allocations:       result.allocations,
		TxSignature:       txSig,
		SolanaExplorerURL: explorerURL,
		AllocationMode:    result.allocationMode,
		Snapshot:          &snapshotSummary,
	})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in PostAllocationComments goroutine: %v", r)
			}
		}()
		ctx := context.Background()
		appClient := githubapp.NewClient(h.config.GitHubAppID, h.config.GitHubAppPrivateKey)
		appAllocations := make([]githubapp.Allocation, len(result.allocations))
		for i, a := range result.allocations {
			appAllocations[i] = githubapp.Allocation{
				Contributor: a.Contributor,
				Percentage:  a.Percentage,
				Amount:      a.Amount,
				Claimed:     a.Claimed,
			}
		}
		githubapp.PostAllocationComments(
			ctx,
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
	if h.solana == nil || !h.solana.IsConfigured() {
		writeError(w, http.StatusServiceUnavailable, "claims are unavailable until Solana is configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	val, _ := h.claimLocks.LoadOrStore(id, &sync.Mutex{})
	mu := val.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req models.ClaimAllocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	campaign, err := h.loadCampaign(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "campaign not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := validateClaimInputs(user.GitHubUsername, campaign, req.ContributorGithub, req.WalletAddress); err != nil {
		switch err.Error() {
		case "campaign is not finalized", "allocation already claimed":
			writeError(w, http.StatusConflict, err.Error())
		case "can only claim your own allocation":
			writeError(w, http.StatusForbidden, err.Error())
		case "contributor not found in allocations":
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	challenge, err := h.loadAndVerifyWalletChallenge(
		models.WalletChallengeActionClaim,
		req.ChallengeID,
		req.WalletAddress,
		req.Signature,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var challengePayload claimChallengePayload
	if err := json.Unmarshal([]byte(challenge.PayloadJSON), &challengePayload); err != nil {
		log.Printf("claim: unmarshal wallet challenge payload failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to verify wallet proof")
		return
	}
	if challengePayload.GitHubUsername != user.GitHubUsername ||
		challengePayload.CampaignID != campaign.CampaignID ||
		challengePayload.ContributorGitHub != req.ContributorGithub ||
		challengePayload.WalletAddress != req.WalletAddress {
		writeError(w, http.StatusBadRequest, "wallet proof did not match this claim request")
		return
	}

	matchedAlloc := findAllocation(campaign, req.ContributorGithub)
	if matchedAlloc == nil {
		writeError(w, http.StatusNotFound, "contributor not found in allocations")
		return
	}
	if err := h.markWalletChallengeUsed(req.ChallengeID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	txSig, err := h.solana.ClaimAllocation(r.Context(), campaign.CampaignID, req.ContributorGithub, req.WalletAddress)
	if err != nil {
		if errors.Is(err, solana.ErrNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "claims are unavailable until Solana is configured")
			return
		}
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
		if errors.Is(err, store.ErrNotFound) {
			if createErr := h.store.Create(campaign); createErr == nil {
				err = nil
			} else {
				err = createErr
			}
		}
		if err != nil {
			log.Printf("WARNING: claim store update failed after on-chain claim (campaign=%s, tx=%s): %v", campaign.CampaignID, txSig, err)
			explorerURL := fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", txSig)
			writeJSON(w, http.StatusAccepted, models.FinalizeResponse{
				CampaignID:        campaign.CampaignID,
				State:             campaign.State,
				Allocations:       campaign.Allocations,
				TxSignature:       txSig,
				SolanaExplorerURL: explorerURL,
			})
			return
		}
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
	campaigns, err := h.listCampaigns(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load claims")
		return
	}
	for _, campaign := range campaigns {
		if campaign.State != models.StateFinalized && campaign.State != models.StateCompleted {
			continue
		}
		for _, alloc := range campaign.Allocations {
			if alloc.Contributor == user.GitHubUsername {
				items = append(items, claimItem{
					CampaignID:     campaign.CampaignID,
					Repo:           campaign.Repo,
					Contributor:    alloc.Contributor,
					Percentage:     alloc.Percentage,
					Amount:         alloc.Amount,
					AmountSOL:      fmt.Sprintf("%.4f", float64(alloc.Amount)/1e9),
					Claimed:        alloc.Claimed,
					ClaimantWallet: alloc.ClaimantWallet,
					State:          string(campaign.State),
				})
			}
		}
	}

	if items == nil {
		items = []claimItem{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(items); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("json encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, models.ErrorResponse{Error: msg})
}

type healthResponse struct {
	Status  string `json:"status"`
	Solana  bool   `json:"solana"`
	GitHub  bool   `json:"github"`
	AIModel string `json:"ai_model"`
	Store   bool   `json:"store"`
}

func (h *Handlers) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		Solana:  h.solana != nil && h.solana.IsConfigured(),
		GitHub:  h.github != nil,
		AIModel: h.ai.Model(),
		Store:   h.store != nil,
	})
}

func (h *Handlers) listCampaigns(ctx context.Context) ([]*models.Campaign, error) {
	storedCampaigns := h.store.List()
	if h.solana == nil || !h.solana.IsConfigured() {
		return storedCampaigns, nil
	}

	onChainCampaigns, err := h.solana.ListCampaigns(ctx)
	if err != nil {
		return nil, err
	}
	if onChainCampaigns == nil {
		return storedCampaigns, nil
	}

	storedByID := make(map[string]*models.Campaign, len(storedCampaigns))
	for _, campaign := range storedCampaigns {
		storedByID[campaign.CampaignID] = campaign
	}

	mergedCampaigns := make([]*models.Campaign, 0, len(onChainCampaigns)+len(storedCampaigns))
	for _, onChainCampaign := range onChainCampaigns {
		if storedCampaign, ok := storedByID[onChainCampaign.CampaignID]; ok {
			mergedCampaigns = append(mergedCampaigns, mergeCampaignWithChainData(storedCampaign, onChainCampaign))
			delete(storedByID, onChainCampaign.CampaignID)
			continue
		}
		mergedCampaigns = append(mergedCampaigns, onChainCampaign)
	}

	for _, storedCampaign := range storedByID {
		mergedCampaigns = append(mergedCampaigns, storedCampaign)
	}

	sort.Slice(mergedCampaigns, func(i, j int) bool {
		return mergedCampaigns[i].CreatedAt.After(mergedCampaigns[j].CreatedAt)
	})

	return mergedCampaigns, nil
}

func (h *Handlers) loadCampaign(ctx context.Context, id string) (*models.Campaign, error) {
	storedCampaign, err := h.store.Get(id)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	if h.solana != nil && h.solana.IsConfigured() {
		onChainCampaign, err := h.solana.GetCampaign(ctx, id)
		if err == nil {
			if storedCampaign == nil {
				return onChainCampaign, nil
			}
			mergedCampaign := mergeCampaignWithChainData(storedCampaign, onChainCampaign)
			if err := h.store.Update(mergedCampaign); err != nil && !errors.Is(err, store.ErrNotFound) {
				log.Printf("campaign sync failed for %s: %v", id, err)
			}
			return mergedCampaign, nil
		}
	}

	if storedCampaign != nil {
		return storedCampaign, nil
	}

	return nil, store.ErrNotFound
}

func mergeCampaignWithChainData(stored, onChain *models.Campaign) *models.Campaign {
	if stored == nil {
		return onChain
	}
	if onChain == nil {
		return stored
	}

	merged := *stored
	merged.CampaignPDA = onChain.CampaignPDA
	merged.VaultAddress = onChain.VaultAddress
	merged.Repo = onChain.Repo
	merged.PoolAmount = onChain.PoolAmount
	merged.TotalClaimed = onChain.TotalClaimed
	merged.Deadline = onChain.Deadline
	merged.State = onChain.State
	merged.Authority = onChain.Authority
	merged.Sponsor = onChain.Sponsor
	merged.CreatedAt = onChain.CreatedAt
	merged.FinalizedAt = onChain.FinalizedAt
	if onChain.TxSignature != "" {
		merged.TxSignature = onChain.TxSignature
	}

	if len(onChain.Allocations) == 0 {
		merged.Allocations = stored.Allocations
		return &merged
	}
	if len(stored.Allocations) > 0 {
		reasoningByContributor := make(map[string]string, len(stored.Allocations))
		for _, alloc := range stored.Allocations {
			if alloc.Reasoning != "" {
				reasoningByContributor[alloc.Contributor] = alloc.Reasoning
			}
		}

		merged.Allocations = make([]models.Allocation, len(onChain.Allocations))
		for i, alloc := range onChain.Allocations {
			if reasoning, ok := reasoningByContributor[alloc.Contributor]; ok {
				alloc.Reasoning = reasoning
			}
			merged.Allocations[i] = alloc
		}
	} else {
		merged.Allocations = onChain.Allocations
	}

	return &merged
}

func requireCampaignOwner(user *store.User, campaign *models.Campaign, action string) error {
	if campaign.OwnerGitHubUsername == "" {
		return errors.New("manual campaign management is unavailable for campaigns without a stored creator")
	}
	if !strings.EqualFold(campaign.OwnerGitHubUsername, user.GitHubUsername) {
		return fmt.Errorf("only @%s can %s this campaign", campaign.OwnerGitHubUsername, action)
	}
	return nil
}

func validateFinalizeState(campaign *models.Campaign) (int, string) {
	if campaign.State == models.StateFinalized || campaign.State == models.StateCompleted {
		return http.StatusConflict, "campaign already finalized"
	}
	if campaign.State != models.StateFunded {
		return http.StatusBadRequest, "campaign must be funded before finalization"
	}
	if time.Now().Before(campaign.Deadline) {
		return http.StatusConflict, "campaign deadline has not been reached yet"
	}
	return http.StatusOK, ""
}

func (h *Handlers) calculateAllocations(
	ctx context.Context,
	campaign *models.Campaign,
	options allocationOptions,
) (*allocationResult, error) {
	windowStart, windowEnd := campaignContributionWindow(campaign)
	windowData, err := h.github.FetchContributionWindowData(ctx, campaign.Repo, windowStart, windowEnd)
	if err != nil {
		return nil, fmt.Errorf("fetch contribution window: %w", err)
	}

	var allocations []models.Allocation
	allocationMode := models.AllocationModeMetrics
	if len(windowData.ContributorPRDiffs) > 0 {
		if options.forceDeterministic {
			allocations, err = h.ai.EvaluateCodeImpactDeterministic(windowData.ContributorPRDiffs, campaign.PoolAmount)
		} else {
			allocations, err = h.ai.EvaluateCodeImpact(ctx, campaign.Repo, windowData.ContributorPRDiffs, campaign.PoolAmount)
		}
		if err != nil {
			log.Printf("ai: code impact evaluation failed (%v), falling back to metric-based allocation", err)
			allocations = nil
		} else {
			allocationMode = models.AllocationModeCodeImpact
		}
	}

	if allocations == nil {
		if options.forceDeterministic {
			allocations, err = h.ai.AllocateDeterministic(windowData.Contributors, campaign.PoolAmount)
		} else {
			allocations, err = h.ai.Allocate(ctx, campaign.Repo, windowData.Contributors, campaign.PoolAmount)
		}
		if err != nil {
			return nil, err
		}
		allocationMode = models.AllocationModeMetrics
	}

	allocations, err = ai.NormalizeAllocations(allocations, windowData.Contributors, campaign.PoolAmount)
	if err != nil {
		return nil, err
	}

	return &allocationResult{
		contributors:      windowData.Contributors,
		allocations:       allocations,
		allocationMode:    allocationMode,
		windowStart:       windowData.WindowStart,
		windowEnd:         windowData.WindowEnd,
		contributorSource: windowData.ContributorSource,
		contributorNotes:  windowData.ContributorNotes,
	}, nil
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
	writeJSON(w, http.StatusOK, map[string]string{"auth_url": authURL, "state": state})
}

func (h *Handlers) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
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
		log.Printf("github oauth: exchange code failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to exchange authorization code")
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
			log.Printf("github oauth: create user failed: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create user account")
			return
		}
		existingUser = newUser
	}

	token, err := h.jwt.GenerateToken(existingUser.GitHubUsername)
	if err != nil {
		log.Printf("github oauth: generate token failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate session token")
		return
	}

	userModel := &models.User{
		GitHubUsername: existingUser.GitHubUsername,
		GitHubID:       existingUser.GitHubID,
		AvatarURL:      existingUser.AvatarURL,
		WalletAddress:  existingUser.WalletAddress,
		CreatedAt:      existingUser.CreatedAt,
	}
	response := models.GitHubAuthResponse{
		Token: token,
		User:  *userModel,
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (h *Handlers) LinkWallet(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req models.LinkWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.WalletAddress != "" && !isValidSolanaAddress(req.WalletAddress) {
		writeError(w, http.StatusBadRequest, "invalid wallet address format")
		return
	}

	user.WalletAddress = req.WalletAddress
	if err := h.store.UpdateUser(user); err != nil {
		log.Printf("wallet link: update user failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update user")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func generateState() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
