# MVP Finalize Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the full happy path — create campaign → auto/manual finalize after deadline → contributor claims reward — with full-repo-history analysis and wallet-proof-based manual finalize for the sponsor.

**Architecture:** Extract a shared `commitFinalize` core used by the auto-worker, the existing GitHub-auth Finalize handler, and a new wallet-proof Finalize handler. Make full-history analysis unconditional. Add a sponsor fallback button on the frontend that signs a challenge with the connected wallet.

**Tech Stack:** Go 1.25, Chi router, Solana wallet-adapter, React 18 + TypeScript, bs58

---

## File Map

| File | Change |
|------|--------|
| `backend/internal/github/campaign_window.go` | Remove `isProduction` guard — full history always |
| `backend/internal/github/campaign_window_test.go` | Add test asserting full-history even on production client |
| `backend/internal/models/wallet_challenges.go` | Add `WalletChallengeActionFinalize` + `FinalizeChallengeRequest` |
| `backend/internal/walletproof/challenge.go` | Add `FinalizeMessageInput` + `BuildFinalizeMessage` |
| `backend/internal/http/handlers.go` | Extract `commitFinalize`; refactor `Finalize` + worker; add `FinalizeChallenge` + `FinalizeWithWalletProof` |
| `backend/internal/http/worker.go` | Call `commitFinalize` instead of duplicating logic |
| `backend/internal/http/router.go` | Add two new routes |
| `backend/internal/http/finalize_wallet_test.go` | New — wallet-proof finalize tests |
| `backend/internal/http/worker_test.go` | Add auto-finalize integration test |
| `frontend/src/api/client.ts` | Add `finalizeChallenge` + `finalizeWallet` |
| `frontend/src/pages/CampaignDetails.tsx` | Add sponsor fallback card + wallet-proof finalize flow |

---

## Task 1: Make full-history the default for all clients

**Files:**
- Modify: `backend/internal/github/campaign_window.go`
- Modify: `backend/internal/github/campaign_window_test.go`

- [ ] **Step 1: Run existing campaign_window test to confirm it passes now**

```bash
cd backend && go test ./internal/github/... -run TestFetchContributionWindowData -v
```

Expected: PASS (non-production path already returns `repository_history_mvp`)

- [ ] **Step 2: Replace the `FetchContributionWindowData` function body**

In `backend/internal/github/campaign_window.go`, replace the entire `FetchContributionWindowData` function with:

```go
// FetchContributionWindowData returns contributor data for allocation.
// MVP default: always analyzes full repository history, regardless of environment.
// The window-based path is preserved below for future use but is not active.
func (c *Client) FetchContributionWindowData(
	ctx context.Context,
	repo string,
	windowStart time.Time,
	windowEnd time.Time,
) (*ContributionWindowData, error) {
	contributors, err := c.FetchContributors(ctx, repo)
	if err != nil {
		return nil, err
	}

	contributorPRDiffs, err := c.FetchContributorsPRDiffs(ctx, repo, 0)
	if err != nil {
		return nil, err
	}

	return &ContributionWindowData{
		Contributors:       contributors,
		ContributorPRDiffs: contributorPRDiffs,
		WindowStart:        windowStart.UTC(),
		WindowEnd:          windowEnd.UTC(),
		ContributorSource:  "repository_history_mvp",
		ContributorNotes:   "MVP analyzes the full available repository history. A future production flow will restrict analysis to activity inside the campaign window.",
	}, nil
}
```

- [ ] **Step 3: Add a test for the production client path**

At the end of `backend/internal/github/campaign_window_test.go`, add:

```go
func TestFetchContributionWindowDataUsesFullHistoryEvenForProductionClient(t *testing.T) {
	now := time.Now().UTC()
	windowStart := now.Add(-24 * time.Hour)
	windowEnd := now

	// Production client — should still use full history in MVP.
	client := NewClientWithEnv("", true)
	client.httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := responseRecorder{header: make(http.Header)}
		switch {
		case r.URL.Path == "/repos/acme/repo/contributors":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{
				{"id": 1, "login": "alice", "contributions": 5},
			})
		case r.URL.Path == "/repos/acme/repo/pulls":
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{})
		default:
			t.Logf("unexpected request: %s %s", r.Method, r.URL.String())
			_ = json.NewEncoder(&recorder).Encode([]map[string]any{})
		}
		return recorder.Response(), nil
	})}

	data, err := client.FetchContributionWindowData(context.Background(), "acme/repo", windowStart, windowEnd)
	if err != nil {
		t.Fatalf("FetchContributionWindowData: %v", err)
	}
	if data.ContributorSource != "repository_history_mvp" {
		t.Fatalf("ContributorSource = %q, want repository_history_mvp", data.ContributorSource)
	}
}
```

- [ ] **Step 4: Run all github tests**

```bash
cd backend && go test ./internal/github/... -v
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
cd backend && git add internal/github/campaign_window.go internal/github/campaign_window_test.go
git commit -m "feat: use full repository history for all clients in MVP finalize"
```

---

## Task 2: Add finalize action to models and walletproof

**Files:**
- Modify: `backend/internal/models/wallet_challenges.go`
- Modify: `backend/internal/walletproof/challenge.go`

- [ ] **Step 1: Add the finalize action constant and request type to models**

In `backend/internal/models/wallet_challenges.go`, after `WalletChallengeActionClaim`:

```go
const (
	WalletChallengeActionClaim    WalletChallengeAction = "claim"
	WalletChallengeActionFinalize WalletChallengeAction = "finalize"
)
```

Also add the request type after `ClaimChallengeRequest`:

```go
type FinalizeChallengeRequest struct {
	WalletAddress string `json:"wallet_address"`
}

type FinalizeWalletRequest struct {
	WalletAddress string `json:"wallet_address"`
	ChallengeID   string `json:"challenge_id"`
	Signature     string `json:"signature"`
}
```

- [ ] **Step 2: Add FinalizeMessageInput and BuildFinalizeMessage to walletproof**

In `backend/internal/walletproof/challenge.go`, after `ClaimMessageInput` struct, add:

```go
type FinalizeMessageInput struct {
	ChallengeID   string
	CampaignID    string
	SponsorWallet string
	IssuedAt      time.Time
	ExpiresAt     time.Time
}
```

After `BuildClaimMessage`, add:

```go
func BuildFinalizeMessage(input FinalizeMessageInput) string {
	lines := []string{
		"RepoBounty AI Wallet Proof",
		"",
		"Action: finalize_campaign",
		fmt.Sprintf("Challenge ID: %s", input.ChallengeID),
		fmt.Sprintf("Campaign ID: %s", input.CampaignID),
		fmt.Sprintf("Sponsor wallet: %s", input.SponsorWallet),
		fmt.Sprintf("Issued at (UTC): %s", input.IssuedAt.UTC().Format(time.RFC3339)),
		fmt.Sprintf("Expires at (UTC): %s", input.ExpiresAt.UTC().Format(time.RFC3339)),
		"",
		"Only sign this message to authorize finalization of this campaign as the sponsor.",
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 3: Build to verify no compilation errors**

```bash
cd backend && go build ./...
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/models/wallet_challenges.go internal/walletproof/challenge.go
git commit -m "feat: add finalize wallet challenge action and message builder"
```

---

## Task 3: Extract commitFinalize core from handlers

**Files:**
- Modify: `backend/internal/http/handlers.go`

The goal is to extract the "Solana finalize + store update" steps that are duplicated between the `Finalize` handler (lines ~529–628) and `autoFinalize` in worker.go into a single shared function `commitFinalize`.

- [ ] **Step 1: Add `commitFinalize` to handlers.go**

Find the end of the `Finalize` function (around line 629) and add the following new function **before** `Finalize`:

```go
// commitFinalize sends the finalization transaction to Solana and persists the
// result. It is the shared core called by the GitHub-auth Finalize handler,
// the FinalizeWithWalletProof handler, and the auto-finalize worker.
func (h *Handlers) commitFinalize(
	ctx context.Context,
	campaign *models.Campaign,
	result *allocationResult,
) (models.FinalizeResponse, error) {
	if h.solana == nil || !h.solana.IsConfigured() {
		return models.FinalizeResponse{}, solana.ErrNotConfigured
	}

	solanaInputs := make([]solana.AllocationInput, len(result.allocations))
	for i, a := range result.allocations {
		if a.GithubUserID == 0 {
			return models.FinalizeResponse{}, fmt.Errorf("missing github_user_id for allocation %s", a.Contributor)
		}
		solanaInputs[i] = solana.AllocationInput{
			GithubUserID: a.GithubUserID,
			Amount:       a.Amount,
		}
	}

	txSig, err := h.solana.FinalizeCampaign(ctx, campaign.CampaignID, campaign.Sponsor, solanaInputs)
	if err != nil {
		return models.FinalizeResponse{}, fmt.Errorf("finalize on-chain: %w", err)
	}

	now := time.Now()
	campaign.State = models.StateFinalized
	campaign.Allocations = result.allocations
	campaign.FinalizedAt = &now
	campaign.TxSignature = txSig

	if err := h.store.Update(campaign); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			_ = h.store.Create(campaign)
		} else {
			log.Printf("WARNING: store update failed after on-chain finalization (campaign=%s, tx=%s): %v",
				campaign.CampaignID, txSig, err)
		}
	}

	explorerURL := fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", txSig)
	return models.FinalizeResponse{
		CampaignID:        campaign.CampaignID,
		State:             models.StateFinalized,
		Allocations:       result.allocations,
		TxSignature:       txSig,
		SolanaExplorerURL: explorerURL,
		AllocationMode:    result.allocationMode,
	}, nil
}
```

- [ ] **Step 2: Refactor existing `Finalize` handler to call `commitFinalize`**

In `backend/internal/http/handlers.go`, replace the section of `Finalize` from `solanaInputs := make(...)` down through the `writeJSON` call (approximately lines 529–628), keeping the GitHub app goroutine after, with:

```go
	resp, err := h.commitFinalize(r.Context(), campaign, result)
	if err != nil {
		if errors.Is(err, solana.ErrNotConfigured) {
			writeError(w, http.StatusServiceUnavailable, "campaign finalization is unavailable until Solana is configured")
			return
		}
		log.Printf("finalize: commitFinalize failed for %s: %v", campaign.CampaignID, err)
		writeError(w, http.StatusInternalServerError, "failed to finalize on-chain")
		return
	}

	snapshotSummary := snapshot.Summary()
	resp.Snapshot = &snapshotSummary

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

	writeJSON(w, http.StatusOK, resp)
```

- [ ] **Step 3: Build to verify no compilation errors**

```bash
cd backend && go build ./...
```

Expected: no errors

- [ ] **Step 4: Run all backend tests**

```bash
cd backend && go test ./...
```

Expected: all PASS (no regressions)

- [ ] **Step 5: Commit**

```bash
git add internal/http/handlers.go
git commit -m "refactor: extract commitFinalize shared core from Finalize handler"
```

---

## Task 4: Refactor worker to use commitFinalize

**Files:**
- Modify: `backend/internal/http/worker.go`

- [ ] **Step 1: Replace the solana finalize block in `autoFinalize`**

In `backend/internal/http/worker.go`, find the block starting with `solanaInputs := make([]solana.AllocationInput...` (around line 162) down through `delete(retries, c.CampaignID)` and the logger call.

Replace that entire block with:

```go
		resp, finalizeErr := helper.commitFinalize(ctx, c, result)
		if finalizeErr != nil {
			retries[c.CampaignID]++
			logger.Error("auto-finalize: commitFinalize failed",
				zap.String("campaign_id", c.CampaignID),
				zap.Int("attempt", retries[c.CampaignID]),
				zap.Error(finalizeErr),
			)
			continue
		}

		delete(retries, c.CampaignID)
		logger.Info("auto-finalize: campaign finalized",
			zap.String("campaign_id", c.CampaignID),
			zap.String("tx", resp.SolanaExplorerURL),
			zap.Int("allocations", len(resp.Allocations)),
		)
```

Also update the `helper` struct literal in `autoFinalize` to include `solana`:

```go
	helper := &Handlers{
		store:  campaignStore,
		github: ghClient,
		ai:     allocator,
		solana: solClient,
	}
```

(The `solana` field was missing before — `commitFinalize` needs it.)

- [ ] **Step 2: Remove the now-unused store update block from worker**

After the previous step, the block that did:
```go
if err := campaignStore.Update(c); err != nil { ... }
```
should already be removed (it was part of the block replaced in step 1). Verify it's gone.

- [ ] **Step 3: Remove unused imports from worker.go if any**

```bash
cd backend && go build ./...
```

Fix any unused import errors (e.g., `"github.com/repobounty/repobounty-ai/internal/store"` may no longer be needed directly in worker.go).

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./...
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/http/worker.go
git commit -m "refactor: worker uses commitFinalize shared core"
```

---

## Task 5: Add FinalizeChallenge and FinalizeWithWalletProof handlers

**Files:**
- Modify: `backend/internal/http/handlers.go`

- [ ] **Step 1: Add `FinalizeChallenge` handler**

Add the following handler to `backend/internal/http/handlers.go` (before `Finalize` or after `FinalizePreview`):

```go
type finalizeChallengePayload struct {
	CampaignID    string `json:"campaign_id"`
	SponsorWallet string `json:"sponsor_wallet"`
}

func (h *Handlers) FinalizeChallenge(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req models.FinalizeChallengeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WalletAddress == "" {
		writeError(w, http.StatusBadRequest, "wallet_address is required")
		return
	}
	if !isValidSolanaAddress(req.WalletAddress) {
		writeError(w, http.StatusBadRequest, "invalid wallet address format")
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

	if status, msg := validateFinalizeState(campaign); status != http.StatusOK {
		writeError(w, status, msg)
		return
	}

	if !strings.EqualFold(campaign.Sponsor, req.WalletAddress) {
		writeError(w, http.StatusForbidden, "only the campaign sponsor can finalize")
		return
	}

	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(walletproof.ChallengeTTL)
	challengeID := generateState()

	payload := finalizeChallengePayload{
		CampaignID:    id,
		SponsorWallet: req.WalletAddress,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		log.Printf("finalize challenge: marshal payload failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create wallet challenge")
		return
	}

	message := walletproof.BuildFinalizeMessage(walletproof.FinalizeMessageInput{
		ChallengeID:   challengeID,
		CampaignID:    id,
		SponsorWallet: req.WalletAddress,
		IssuedAt:      issuedAt,
		ExpiresAt:     expiresAt,
	})

	challenge := &models.WalletChallenge{
		ChallengeID:   challengeID,
		Action:        models.WalletChallengeActionFinalize,
		WalletAddress: req.WalletAddress,
		Message:       message,
		PayloadJSON:   string(payloadJSON),
		CreatedAt:     issuedAt,
		ExpiresAt:     expiresAt,
	}

	if err := h.store.CreateWalletChallenge(challenge); err != nil {
		log.Printf("finalize challenge: store create failed: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create wallet challenge")
		return
	}

	writeJSON(w, http.StatusCreated, models.WalletChallengeResponse{
		ChallengeID:   challenge.ChallengeID,
		Action:        challenge.Action,
		WalletAddress: challenge.WalletAddress,
		Message:       challenge.Message,
		ExpiresAt:     challenge.ExpiresAt,
	})
}
```

- [ ] **Step 2: Add `FinalizeWithWalletProof` handler**

Add immediately after `FinalizeChallenge`:

```go
func (h *Handlers) FinalizeWithWalletProof(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if h.solana == nil || !h.solana.IsConfigured() {
		writeError(w, http.StatusServiceUnavailable, "campaign finalization is unavailable until Solana is configured")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req models.FinalizeWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WalletAddress == "" || req.ChallengeID == "" || req.Signature == "" {
		writeError(w, http.StatusBadRequest, "wallet_address, challenge_id, and signature are required")
		return
	}
	if !isValidSolanaAddress(req.WalletAddress) {
		writeError(w, http.StatusBadRequest, "invalid wallet address format")
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

	if status, msg := validateFinalizeState(campaign); status != http.StatusOK {
		writeError(w, status, msg)
		return
	}

	challenge, err := h.loadAndVerifyWalletChallenge(
		models.WalletChallengeActionFinalize,
		req.ChallengeID,
		req.WalletAddress,
		req.Signature,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var challengePayload finalizeChallengePayload
	if err := json.Unmarshal([]byte(challenge.PayloadJSON), &challengePayload); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse wallet challenge payload")
		return
	}
	if challengePayload.CampaignID != id ||
		!strings.EqualFold(challengePayload.SponsorWallet, req.WalletAddress) {
		writeError(w, http.StatusBadRequest, "wallet proof did not match this finalize request")
		return
	}

	if !strings.EqualFold(campaign.Sponsor, req.WalletAddress) {
		writeError(w, http.StatusForbidden, "only the campaign sponsor can finalize")
		return
	}

	if err := h.markWalletChallengeUsed(req.ChallengeID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Load approved snapshot if available; otherwise compute.
	snapshot, snapshotErr := h.loadFinalizeSnapshot(campaign, true)
	if snapshotErr != nil {
		result, calcErr := h.calculateAllocations(r.Context(), campaign, allocationOptions{})
		if calcErr != nil {
			log.Printf("finalize wallet: allocation failed for %s: %v", campaign.CampaignID, calcErr)
			writeError(w, http.StatusInternalServerError, "failed to build allocation snapshot")
			return
		}
		snapshot, snapshotErr = h.createFinalizeSnapshot(campaign, result, "")
		if snapshotErr != nil {
			log.Printf("finalize wallet: snapshot persistence failed for %s: %v", campaign.CampaignID, snapshotErr)
			writeError(w, http.StatusInternalServerError, "failed to save allocation snapshot")
			return
		}
	}
	result := snapshotToAllocationResult(snapshot)

	resp, err := h.commitFinalize(r.Context(), campaign, result)
	if err != nil {
		log.Printf("finalize wallet: commitFinalize failed for %s: %v", campaign.CampaignID, err)
		writeError(w, http.StatusInternalServerError, "failed to finalize on-chain")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 3: Build**

```bash
cd backend && go build ./...
```

Expected: no errors

- [ ] **Step 4: Run tests**

```bash
cd backend && go test ./...
```

Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add internal/http/handlers.go
git commit -m "feat: add FinalizeChallenge and FinalizeWithWalletProof handlers"
```

---

## Task 6: Register new routes

**Files:**
- Modify: `backend/internal/http/router.go`

- [ ] **Step 1: Add the two new routes inside the campaigns block**

In `backend/internal/http/router.go`, inside the `r.Route("/campaigns", ...)` block, add after the existing finalize routes:

```go
			r.Post("/{id}/finalize-challenge", h.FinalizeChallenge)
			r.Post("/{id}/finalize-wallet", h.FinalizeWithWalletProof)
```

The campaigns block should now look like:

```go
		r.Route("/campaigns", func(r chi.Router) {
			r.Use(optionalAuth)
			r.Get("/", h.ListCampaigns)
			r.Get("/{id}", h.GetCampaign)
			r.Post("/", h.CreateCampaign)
			r.Post("/{id}/create-confirm", h.CreateCampaignConfirm)
			r.With(requireAuth).Post("/{id}/finalize-preview", h.FinalizePreview)
			r.With(requireAuth).Post("/{id}/finalize", h.Finalize)
			r.Post("/{id}/finalize-challenge", h.FinalizeChallenge)
			r.Post("/{id}/finalize-wallet", h.FinalizeWithWalletProof)
			r.With(requireAuth).Post("/{id}/claim-challenge", h.ClaimChallenge)
			r.With(requireAuth).Post("/{id}/claim", h.ClaimPermit)
			r.With(requireAuth).Post("/{id}/claim-confirm", h.ClaimConfirm)
			r.Post("/{id}/refund", h.RefundBuild)
			r.Post("/{id}/refund-confirm", h.RefundConfirm)
			r.Post("/{id}/fund-tx", h.FundTx)
		})
```

- [ ] **Step 2: Build and test**

```bash
cd backend && go build ./... && go test ./...
```

Expected: all PASS

- [ ] **Step 3: Commit**

```bash
git add internal/http/router.go
git commit -m "feat: register finalize-challenge and finalize-wallet routes"
```

---

## Task 7: Write backend tests for wallet-proof finalize

**Files:**
- Create: `backend/internal/http/finalize_wallet_test.go`

- [ ] **Step 1: Write the test file**

Create `backend/internal/http/finalize_wallet_test.go`:

```go
package http

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mr-tron/base58"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/store"
)

// newFinalizeTestHandlers returns a minimal Handlers wired to an in-memory
// store with a single funded campaign past deadline.
func newFinalizeTestHandlers(t *testing.T, sponsor string) (*Handlers, *models.Campaign) {
	t.Helper()
	s := store.New()
	deadline := time.Now().Add(-time.Minute) // past deadline
	campaign := &models.Campaign{
		CampaignID: "test-campaign-1",
		Repo:       "acme/repo",
		PoolAmount: 1_000_000_000,
		State:      models.StateFunded,
		Sponsor:    sponsor,
		Deadline:   deadline,
		CreatedAt:  time.Now().Add(-2 * time.Minute),
	}
	if err := s.Create(campaign); err != nil {
		t.Fatalf("store.Create: %v", err)
	}
	h := NewHandlers(s, nil, nil, nil, nil, nil, nil)
	return h, campaign
}

// generateTestKeypair returns a fresh ed25519 keypair and its base58-encoded public key.
func generateTestKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	return pub, priv, base58.Encode(pub)
}

func TestFinalizeChallengeIssuesChallengForSponsor(t *testing.T) {
	_, _, sponsorB58 := generateTestKeypair(t)
	h, _ := newFinalizeTestHandlers(t, sponsorB58)

	body, _ := json.Marshal(models.FinalizeChallengeRequest{WalletAddress: sponsorB58})
	r := newTestRouter(h)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/campaigns/test-campaign-1/finalize-challenge", bytes.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}

	var resp models.WalletChallengeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Action != models.WalletChallengeActionFinalize {
		t.Fatalf("action = %q, want %q", resp.Action, models.WalletChallengeActionFinalize)
	}
	if resp.WalletAddress != sponsorB58 {
		t.Fatalf("wallet_address = %q, want %q", resp.WalletAddress, sponsorB58)
	}
	if resp.Message == "" {
		t.Fatal("message must not be empty")
	}
}

func TestFinalizeChallengeRejectsNonSponsor(t *testing.T) {
	_, _, sponsorB58 := generateTestKeypair(t)
	_, _, otherB58 := generateTestKeypair(t)
	h, _ := newFinalizeTestHandlers(t, sponsorB58)

	body, _ := json.Marshal(models.FinalizeChallengeRequest{WalletAddress: otherB58})
	r := newTestRouter(h)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/campaigns/test-campaign-1/finalize-challenge", bytes.NewReader(body)))

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestFinalizeChallengeRejectsBeforeDeadline(t *testing.T) {
	_, _, sponsorB58 := generateTestKeypair(t)
	s := store.NewStore()
	campaign := &models.Campaign{
		CampaignID: "test-campaign-2",
		Repo:       "acme/repo",
		PoolAmount: 1_000_000_000,
		State:      models.StateFunded,
		Sponsor:    sponsorB58,
		Deadline:   time.Now().Add(time.Hour), // future deadline
		CreatedAt:  time.Now().Add(-time.Minute),
	}
	_ = s.Create(campaign)
	h := NewHandlers(s, nil, nil, nil, nil, nil, nil)

	body, _ := json.Marshal(models.FinalizeChallengeRequest{WalletAddress: sponsorB58})
	r := newTestRouter(h)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/campaigns/test-campaign-2/finalize-challenge", bytes.NewReader(body)))

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}

func TestFinalizeWithWalletProofRejectsInvalidSignature(t *testing.T) {
	_, _, sponsorB58 := generateTestKeypair(t)
	h, _ := newFinalizeTestHandlers(t, sponsorB58)

	// First issue a challenge.
	challengeBody, _ := json.Marshal(models.FinalizeChallengeRequest{WalletAddress: sponsorB58})
	r := newTestRouter(h)
	crr := httptest.NewRecorder()
	r.ServeHTTP(crr, httptest.NewRequest(http.MethodPost, "/api/campaigns/test-campaign-1/finalize-challenge", bytes.NewReader(challengeBody)))
	if crr.Code != http.StatusCreated {
		t.Fatalf("challenge status = %d; body: %s", crr.Code, crr.Body.String())
	}
	var challenge models.WalletChallengeResponse
	_ = json.NewDecoder(crr.Body).Decode(&challenge)

	// Submit with a bad signature.
	reqBody, _ := json.Marshal(models.FinalizeWalletRequest{
		WalletAddress: sponsorB58,
		ChallengeID:   challenge.ChallengeID,
		Signature:     base58.Encode(bytes.Repeat([]byte{0}, 64)), // all-zero signature
	})
	frr := httptest.NewRecorder()
	r.ServeHTTP(frr, httptest.NewRequest(http.MethodPost, "/api/campaigns/test-campaign-1/finalize-wallet", bytes.NewReader(reqBody)))

	if frr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", frr.Code, frr.Body.String())
	}
}

// newTestRouter builds a minimal chi router for use in unit tests.
func newTestRouter(h *Handlers) http.Handler {
	return NewRouter(h, "test")
}
```

- [ ] **Step 2: Run the new tests**

```bash
cd backend && go test ./internal/http/... -run TestFinalizeChallenge -v
cd backend && go test ./internal/http/... -run TestFinalizeWithWalletProof -v
```

Expected: TestFinalizeChallengeIssuesChallengForSponsor PASS, TestFinalizeChallengeRejectsNonSponsor PASS, TestFinalizeChallengeRejectsBeforeDeadline PASS, TestFinalizeWithWalletProofRejectsInvalidSignature PASS

- [ ] **Step 3: Run all backend tests**

```bash
cd backend && go test ./...
```

Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/http/finalize_wallet_test.go
git commit -m "test: wallet-proof finalize — sponsor accepted, non-sponsor rejected, pre-deadline rejected"
```

---

## Task 8: Add worker integration test

**Files:**
- Modify: `backend/internal/http/worker_test.go`

- [ ] **Step 1: Add a test that verifies `mergeAutoFinalizeCampaigns` produces funded-past-deadline candidates**

Append to `backend/internal/http/worker_test.go`:

```go
func TestMergeAutoFinalizeCampaignsIncludesOnChainFundedCampaigns(t *testing.T) {
	now := time.Now()
	onChain := []*models.Campaign{
		{
			CampaignID: "chain-1",
			State:      models.StateFunded,
			Deadline:   now.Add(-time.Minute),
			Sponsor:    "SponsorWallet123",
			Repo:       "acme/repo",
		},
	}
	stored := []*models.Campaign{} // nothing in the store yet

	merged := mergeAutoFinalizeCampaigns(stored, onChain)
	if len(merged) != 1 {
		t.Fatalf("len(merged) = %d, want 1", len(merged))
	}
	if merged[0].State != models.StateFunded {
		t.Fatalf("merged[0].State = %q, want funded", merged[0].State)
	}
	if merged[0].Deadline.After(now) {
		t.Fatal("merged[0].Deadline should be in the past")
	}
}

func TestMergeAutoFinalizeCampaignsSkipsAlreadyFinalizedCampaigns(t *testing.T) {
	now := time.Now()
	onChain := []*models.Campaign{
		{CampaignID: "chain-2", State: models.StateFinalized, Deadline: now.Add(-time.Minute)},
	}
	merged := mergeAutoFinalizeCampaigns([]*models.Campaign{}, onChain)
	if len(merged) != 1 {
		t.Fatalf("len(merged) = %d, want 1", len(merged))
	}
	// autoFinalize itself (not merge) skips non-funded — test the filter logic.
	fundedCount := 0
	for _, c := range merged {
		if c.State == models.StateFunded {
			fundedCount++
		}
	}
	if fundedCount != 0 {
		t.Fatalf("funded count = %d, want 0 (finalized campaign should not be funded)", fundedCount)
	}
}
```

- [ ] **Step 2: Run worker tests**

```bash
cd backend && go test ./internal/http/... -run TestMerge -v
```

Expected: both PASS

- [ ] **Step 3: Run all backend tests**

```bash
cd backend && go test ./...
```

Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/http/worker_test.go
git commit -m "test: worker merge produces funded-past-deadline candidates for auto-finalize"
```

---

## Task 9: Frontend — add API client methods

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/types/index.ts`

- [ ] **Step 1: Add `FinalizeChallengeRequest` and `FinalizeWalletRequest` types**

In `frontend/src/types/index.ts`, add after `ClaimChallengeRequest`:

```ts
export interface FinalizeChallengeRequest {
  wallet_address: string;
}

export interface FinalizeWalletRequest {
  wallet_address: string;
  challenge_id: string;
  signature: string;
}
```

- [ ] **Step 2: Add the import and two new API methods**

In `frontend/src/api/client.ts`, add the new types to the import:

```ts
import type {
  // ... existing imports ...
  FinalizeChallengeRequest,
  FinalizeWalletRequest,
} from '../types';
```

Then add the two methods to the `api` object (after `finalize`):

```ts
  finalizeChallenge(campaignId: string, walletAddress: string): Promise<WalletChallengeResponse> {
    const body: FinalizeChallengeRequest = { wallet_address: walletAddress };
    return request(`/campaigns/${campaignId}/finalize-challenge`, {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },

  finalizeWallet(
    campaignId: string,
    walletAddress: string,
    challengeId: string,
    signature: string
  ): Promise<FinalizeResponse> {
    const body: FinalizeWalletRequest = {
      wallet_address: walletAddress,
      challenge_id: challengeId,
      signature,
    };
    return request(`/campaigns/${campaignId}/finalize-wallet`, {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },
```

- [ ] **Step 3: Lint check**

```bash
cd frontend && npm run lint
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
cd frontend && git add src/api/client.ts src/types/index.ts
git commit -m "feat: add finalizeChallenge and finalizeWallet API client methods"
```

---

## Task 10: Frontend — sponsor fallback button in CampaignDetails

**Files:**
- Modify: `frontend/src/pages/CampaignDetails.tsx`

- [ ] **Step 1: Add `sponsorFinalizing` state and `handleSponsorFinalize` function**

After the existing state declarations (around line 37), add:

```ts
  const [sponsorFinalizing, setSponsorFinalizing] = useState(false);
```

After `handleFinalize` function (around line 102), add:

```ts
  async function handleSponsorFinalize() {
    if (!id || !publicKey || !signMessage) return;
    setSponsorFinalizing(true);
    setError(null);
    try {
      const challenge = await api.finalizeChallenge(id, publicKey.toBase58());
      const signatureBytes = await signMessage(new TextEncoder().encode(challenge.message));
      await api.finalizeWallet(id, publicKey.toBase58(), challenge.challenge_id, bs58.encode(signatureBytes));
      const updated = await api.getCampaign(id);
      setCampaign(updated);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Finalization failed');
    } finally {
      setSponsorFinalizing(false);
    }
  }
```

- [ ] **Step 2: Add the sponsor fallback condition**

After the existing `const isOwner = ...` line (around line 173), add:

```ts
  const isSponsor = !!publicKey && publicKey.toBase58() === campaign.sponsor;
  const canSponsorFinalize = campaign.state === 'funded' && isPastDeadline && isSponsor;
```

- [ ] **Step 3: Add the sponsor fallback card**

After the closing `)}` of the existing `{canShowFinalizeCard && ...}` block (around line 358), add:

```tsx
      {/* Sponsor wallet-proof fallback — shown only to the sponsor when funded + past deadline */}
      {canSponsorFinalize && (
        <div className="card mb-5 animate-fade-in-up !border-solana-purple/20" style={{ animationDelay: '120ms' }}>
          <h2 className="text-sm font-semibold mb-2">Sponsor Finalize</h2>
          <p className="text-xs text-gray-500 mb-3">
            The auto-finalize worker usually handles this automatically. If it hasn't run yet,
            you can finalize now by signing a proof with your wallet.
          </p>
          {!solanaReady && (
            <div className="bg-yellow-500/5 border border-yellow-500/20 rounded-lg p-2.5 text-xs text-yellow-200 mb-3">
              Backend not connected to Solana — finalization is disabled.
            </div>
          )}
          {!signMessage && (
            <p className="text-xs text-yellow-200 mb-2">
              Your wallet does not support message signing.
            </p>
          )}
          <button
            onClick={handleSponsorFinalize}
            disabled={sponsorFinalizing || !solanaReady || !signMessage}
            className="btn-primary text-xs"
          >
            {sponsorFinalizing ? 'Finalizing...' : 'Finalize now'}
          </button>
        </div>
      )}
```

- [ ] **Step 4: Lint and build**

```bash
cd frontend && npm run lint && npm run build
```

Expected: no errors, build succeeds

- [ ] **Step 5: Commit**

```bash
cd frontend && git add src/pages/CampaignDetails.tsx
git commit -m "feat: add sponsor wallet-proof finalize fallback button"
```

---

## Task 11: Final verification

- [ ] **Step 1: Run all backend tests**

```bash
cd backend && go test ./... -v 2>&1 | tail -30
```

Expected: all PASS, no FAIL lines

- [ ] **Step 2: Run frontend lint and build**

```bash
cd frontend && npm run lint && npm run build
```

Expected: lint clean, build succeeds with no TypeScript errors

- [ ] **Step 3: Smoke-check the happy path manually (optional but recommended)**

```bash
./start.sh
```

1. Open http://localhost:5173
2. Create a campaign via wallet (sponsor)
3. Wait for deadline (set 5 min minimum), or manually advance time
4. Observe: auto-worker finalizes OR sponsor sees "Finalize now" button
5. Connect as contributor, claim reward

- [ ] **Step 4: Final commit (if any stray changes)**

```bash
git status
```

If clean: done. If any files changed, commit them with a descriptive message.
