package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/repobounty/repobounty-ai/internal/auth"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/store"
	"github.com/repobounty/repobounty-ai/internal/walletproof"
)

type claimChallengePayload struct {
	GitHubUsername    string `json:"github_username"`
	CampaignID        string `json:"campaign_id"`
	ContributorGitHub string `json:"contributor_github"`
	WalletAddress     string `json:"wallet_address"`
}

func (h *Handlers) minCampaignAmount() uint64 {
	if h != nil && h.config != nil && h.config.MinCampaignAmount > 0 {
		return h.config.MinCampaignAmount
	}
	return 500_000_000
}

func (h *Handlers) minCampaignLeadTime() time.Duration {
	if h != nil && h.config != nil && h.config.MinDeadlineSeconds > 0 {
		return time.Duration(h.config.MinDeadlineSeconds) * time.Second
	}
	return 5 * time.Minute
}

func (h *Handlers) ClaimChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	if h.solana == nil || !h.solana.IsConfigured() {
		writeError(w, http.StatusServiceUnavailable, "claims are unavailable until Solana is configured")
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

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req models.ClaimChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateClaimInputs(user.GitHubUsername, campaign, req.ContributorGithub, req.WalletAddress); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(walletproof.ChallengeTTL)
	challengeID := generateState()
	payload := claimChallengePayload{
		GitHubUsername:    user.GitHubUsername,
		CampaignID:        id,
		ContributorGitHub: req.ContributorGithub,
		WalletAddress:     req.WalletAddress,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("claim challenge: marshal payload failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create wallet challenge")
		return
	}

	message := walletproof.BuildClaimMessage(walletproof.ClaimMessageInput{
		ChallengeID:       challengeID,
		GitHubUsername:    user.GitHubUsername,
		CampaignID:        id,
		ContributorGitHub: req.ContributorGithub,
		WalletAddress:     req.WalletAddress,
		IssuedAt:          issuedAt,
		ExpiresAt:         expiresAt,
	})

	challenge := &models.WalletChallenge{
		ChallengeID:   challengeID,
		Action:        models.WalletChallengeActionClaim,
		WalletAddress: req.WalletAddress,
		Message:       message,
		PayloadJSON:   string(payloadJSON),
		CreatedAt:     issuedAt,
		ExpiresAt:     expiresAt,
	}

	if err := h.store.CreateWalletChallenge(challenge); err != nil {
		h.logWalletChallengeEvent(
			"create_failed",
			challenge.ChallengeID,
			challenge.Action,
			challenge.WalletAddress,
			"lookup_result", "store_error",
		)
		log.Printf("claim challenge: store create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create wallet challenge")
		return
	}
	h.logWalletChallengeEvent(
		"created",
		challenge.ChallengeID,
		challenge.Action,
		challenge.WalletAddress,
		"lookup_result", "stored",
	)

	writeJSON(w, http.StatusCreated, models.WalletChallengeResponse{
		ChallengeID:   challenge.ChallengeID,
		Action:        challenge.Action,
		WalletAddress: challenge.WalletAddress,
		Message:       challenge.Message,
		ExpiresAt:     challenge.ExpiresAt,
	})
}

func normalizeCreateCampaignRequest(repo string, poolAmount uint64, deadlineValue string, sponsorWallet string, minCampaignAmount uint64, minLeadTime time.Duration) (time.Time, error) {
	return normalizeCreateCampaignRequestWithLeadTime(repo, poolAmount, deadlineValue, sponsorWallet, minCampaignAmount, minLeadTime)
}

func normalizeCreateCampaignConfirmRequest(repo string, poolAmount uint64, deadlineValue string, sponsorWallet string, minCampaignAmount uint64) (time.Time, error) {
	return normalizeCreateCampaignRequestWithLeadTime(repo, poolAmount, deadlineValue, sponsorWallet, minCampaignAmount, 0)
}

func normalizeCreateCampaignRequestWithLeadTime(repo string, poolAmount uint64, deadlineValue string, sponsorWallet string, minCampaignAmount uint64, minLeadTime time.Duration) (time.Time, error) {
	if !repoPattern.MatchString(repo) {
		return time.Time{}, errors.New("repo must be in owner/repo format")
	}
	if poolAmount == 0 {
		return time.Time{}, errors.New("pool_amount must be greater than 0")
	}
	if poolAmount < minCampaignAmount {
		return time.Time{}, errors.New("pool_amount must be at least 0.5 SOL")
	}
	if sponsorWallet == "" {
		return time.Time{}, errors.New("sponsor_wallet is required")
	}
	if !isValidSolanaAddress(sponsorWallet) {
		return time.Time{}, errors.New("invalid sponsor wallet address")
	}

	deadline, err := time.Parse(time.RFC3339, deadlineValue)
	if err != nil {
		return time.Time{}, errors.New("deadline must be RFC3339 format")
	}
	if minLeadTime > 0 && deadline.Before(time.Now().UTC().Add(minLeadTime)) {
		return time.Time{}, fmt.Errorf("deadline must be at least %d seconds in the future", int(minLeadTime.Seconds()))
	}
	return deadline.UTC(), nil
}

func (h *Handlers) loadAndVerifyWalletChallenge(
	action models.WalletChallengeAction,
	challengeID string,
	walletAddress string,
	signature string,
) (*models.WalletChallenge, error) {
	if challengeID == "" || signature == "" {
		h.logWalletChallengeEvent(
			"lookup_rejected",
			challengeID,
			action,
			walletAddress,
			"lookup_result", "missing_wallet_proof",
		)
		return nil, errors.New("wallet proof is required")
	}

	challenge, err := h.store.GetWalletChallenge(challengeID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.logWalletChallengeEvent(
				"lookup_failed",
				challengeID,
				action,
				walletAddress,
				"lookup_result", "not_found",
			)
			return nil, errors.New("wallet proof challenge was not found")
		}
		h.logWalletChallengeEvent(
			"lookup_failed",
			challengeID,
			action,
			walletAddress,
			"lookup_result", "store_error",
		)
		return nil, err
	}
	h.logWalletChallengeEvent(
		"lookup_succeeded",
		challengeID,
		action,
		walletAddress,
		"lookup_result", "found",
	)
	if challenge.Action != action {
		return nil, errors.New("wallet proof challenge action did not match")
	}
	if challenge.WalletAddress != walletAddress {
		return nil, errors.New("wallet proof challenge did not match the provided wallet")
	}
	if challenge.UsedAt != nil {
		return nil, errors.New("wallet proof challenge has already been used")
	}
	if time.Now().UTC().After(challenge.ExpiresAt) {
		return nil, errors.New("wallet proof challenge has expired")
	}
	if err := walletproof.VerifySignature(walletAddress, challenge.Message, signature); err != nil {
		return nil, errors.New("wallet proof verification failed")
	}

	return challenge, nil
}

func (h *Handlers) markWalletChallengeUsed(challengeID string) error {
	challenge, lookupErr := h.store.GetWalletChallenge(challengeID)
	if lookupErr != nil {
		if errors.Is(lookupErr, store.ErrNotFound) {
			h.logWalletChallengeEvent(
				"mark_used_failed",
				challengeID,
				"",
				"",
				"mark_used_result", "not_found",
			)
			return errors.New("wallet proof challenge was not found")
		}
		h.logWalletChallengeEvent(
			"mark_used_failed",
			challengeID,
			"",
			"",
			"mark_used_result", "lookup_error",
		)
		return lookupErr
	}

	err := h.store.MarkWalletChallengeUsed(challengeID, time.Now().UTC())
	if err == nil {
		h.logWalletChallengeEvent(
			"mark_used_succeeded",
			challengeID,
			challenge.Action,
			challenge.WalletAddress,
			"mark_used_result", "used",
		)
		return nil
	}
	if errors.Is(err, store.ErrAlreadyUsed) {
		h.logWalletChallengeEvent(
			"mark_used_failed",
			challengeID,
			challenge.Action,
			challenge.WalletAddress,
			"mark_used_result", "already_used",
		)
		return errors.New("wallet proof challenge has already been used")
	}
	if errors.Is(err, store.ErrNotFound) {
		h.logWalletChallengeEvent(
			"mark_used_failed",
			challengeID,
			challenge.Action,
			challenge.WalletAddress,
			"mark_used_result", "not_found",
		)
		return errors.New("wallet proof challenge was not found")
	}
	h.logWalletChallengeEvent(
		"mark_used_failed",
		challengeID,
		challenge.Action,
		challenge.WalletAddress,
		"mark_used_result", "store_error",
	)
	return err
}

func validateClaimInputs(
	githubUsername string,
	campaign *models.Campaign,
	contributorGithub string,
	walletAddress string,
) error {
	if campaign.State != models.StateFinalized && campaign.State != models.StateCompleted {
		return errors.New("campaign is not finalized")
	}
	if contributorGithub == "" || walletAddress == "" {
		return errors.New("contributor_github and wallet_address are required")
	}
	if !isValidSolanaAddress(walletAddress) {
		return errors.New("invalid wallet address format")
	}
	if githubUsername != contributorGithub {
		return errors.New("can only claim your own allocation")
	}

	allocation := findAllocation(campaign, contributorGithub)
	if allocation == nil {
		return errors.New("contributor not found in allocations")
	}
	if allocation.Claimed {
		return errors.New("allocation already claimed")
	}
	return nil
}

func validateClaimConfirmationInputs(
	githubUsername string,
	campaign *models.Campaign,
	contributorGithub string,
	walletAddress string,
) error {
	// ClaimConfirm uses on-chain claim status as the source of truth.
	// This helper only validates the authenticated user, the request payload,
	// and that the contributor exists on the campaign.
	if campaign.State != models.StateFinalized && campaign.State != models.StateCompleted {
		return errors.New("campaign is not finalized")
	}
	if contributorGithub == "" || walletAddress == "" {
		return errors.New("contributor_github and wallet_address are required")
	}
	if !isValidSolanaAddress(walletAddress) {
		return errors.New("invalid wallet address format")
	}
	if githubUsername != contributorGithub {
		return errors.New("can only claim your own allocation")
	}
	if findAllocation(campaign, contributorGithub) == nil {
		return errors.New("contributor not found in allocations")
	}
	return nil
}

func findAllocation(campaign *models.Campaign, contributor string) *models.Allocation {
	for i := range campaign.Allocations {
		if campaign.Allocations[i].Contributor == contributor {
			return &campaign.Allocations[i]
		}
	}
	return nil
}
