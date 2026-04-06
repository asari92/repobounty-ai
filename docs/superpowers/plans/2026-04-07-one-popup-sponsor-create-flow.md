# One-Popup Sponsor Create Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove sponsor wallet-message challenge from campaign creation, make create flow a one-button/one-Phantom-popup path, and remove the tracked `backend/api` binary from git.

**Architecture:** Keep `POST /api/campaigns/` as the backend build step that returns an unsigned `create_campaign_with_deposit` transaction, keep `POST /api/campaigns/{id}/create-confirm` as the persistence step after on-chain confirmation, and remove sponsor `create-challenge` completely. Frontend moves to a single CTA path that builds, signs, sends, confirms, and redirects without any intermediate step UI.

**Tech Stack:** Go backend, Chi router, React + TypeScript + wallet-adapter frontend, SQLite enrichment store, git cleanup via `.gitignore`

---

## File Structure And Responsibilities

- `backend/internal/http/handlers.go`
  Sponsor create handler logic. This is where wallet-challenge verification is removed from create flow.
- `backend/internal/http/router.go`
  Public API routes. This is where sponsor `create-challenge` route is removed.
- `backend/internal/http/wallet_challenges.go`
  Claim challenge flow remains here. Sponsor-specific challenge code should be removed, while shared validation helpers can stay.
- `backend/internal/models/models.go`
  Create request/response types. This is where sponsor challenge fields are removed from `CreateCampaignRequest`.
- `backend/internal/models/wallet_challenges.go`
  Wallet challenge models. This file should no longer expose sponsor create challenge request types or action constants if they become unused.
- `backend/internal/http/create_campaign_integration_test.go`
  Existing sponsor create tests. This is where sponsor challenge-based tests are replaced by tx-only create tests and route removal checks.
- `frontend/src/api/client.ts`
  Frontend API client. This is where sponsor `createCampaignChallenge()` is removed.
- `frontend/src/types/index.ts`
  Shared frontend request/response types. This is where `CreateCampaignRequest` is simplified.
- `frontend/src/pages/CreateCampaign.tsx`
  Create page UI and wallet flow. This is where the two-step UI and `signMessage()` path are removed.
- `.gitignore`
  Repository ignore rules. Add `backend/api` here after untracking it.

### Task 1: Remove Sponsor Challenge From Backend Create Flow

**Files:**
- Modify: `backend/internal/http/handlers.go`
- Modify: `backend/internal/http/router.go`
- Modify: `backend/internal/http/wallet_challenges.go`
- Modify: `backend/internal/models/models.go`
- Modify: `backend/internal/models/wallet_challenges.go`
- Modify: `backend/internal/http/create_campaign_integration_test.go`

- [ ] **Step 1: Write the failing backend tests**

Add focused tests to `backend/internal/http/create_campaign_integration_test.go`:

```go
func TestCreateCampaignDoesNotRequireSponsorChallengeFields(t *testing.T) {
	handlers := newTestHandlersWithSolana(t)

	req := models.CreateCampaignRequest{
		Repo:          "octocat/Hello-World",
		PoolAmount:    500_000_000,
		Deadline:      time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
		SponsorWallet: testWalletAddress(t),
	}

	rec := performJSONRequestRecorder(t, handlers, http.MethodPost, "/api/campaigns/", req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCreateCampaignChallengeRouteIsGone(t *testing.T) {
	router := NewRouter(newTestHandlersWithSolana(t), "test")
	rec := performRequest(t, router, http.MethodPost, "/api/campaigns/create-challenge", nil, "")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
```

- [ ] **Step 2: Run the focused backend tests to verify they fail**

Run:

```bash
cd backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./internal/http -run 'TestCreateCampaign(DoesNotRequireSponsorChallengeFields|ChallengeRouteIsGone)'
```

Expected: FAIL because `CreateCampaign` still requires `challenge_id` and `signature`, and router still exposes `/campaigns/create-challenge`.

- [ ] **Step 3: Implement the minimal backend create-flow simplification**

Apply these changes:

```go
// backend/internal/models/models.go
type CreateCampaignRequest struct {
	Repo          string `json:"repo"`
	PoolAmount    uint64 `json:"pool_amount"`
	RewardAmount  uint64 `json:"reward_amount"`
	Deadline      string `json:"deadline"`
	SponsorWallet string `json:"sponsor_wallet"`
}
```

```go
// backend/internal/http/router.go
r.Route("/campaigns", func(r chi.Router) {
	r.Post("/", h.CreateCampaign)
	r.Post("/{id}/create-confirm", h.CreateCampaignConfirm)
	r.Get("/", h.ListCampaigns)
	r.Get("/{id}", h.GetCampaign)
	// claim routes stay
})
```

```go
// backend/internal/http/handlers.go
func (h *Handlers) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	// decode req
	// validate repo / amount / deadline / sponsor_wallet
	// DO NOT call loadAndVerifyWalletChallenge(...)
	// DO NOT unmarshal createCampaignChallengePayload
	// DO NOT call markWalletChallengeUsed(...)
	// keep:
	// - GitHub repo existence and repo id lookup
	// - sponsor balance and cost estimate check
	// - campaignID generation
	// - BuildFundTransaction(...)
	// - JSON response with unsigned_tx
}
```

Remove or shrink sponsor-only challenge code in `backend/internal/http/wallet_challenges.go` and `backend/internal/models/wallet_challenges.go`, but keep claim challenge behavior intact. Shared helper `normalizeCreateChallengeRequest(...)` may stay if reused by `CreateCampaign`.

- [ ] **Step 4: Re-run the focused backend tests**

Run:

```bash
cd backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./internal/http -run 'TestCreateCampaign(DoesNotRequireSponsorChallengeFields|ChallengeRouteIsGone)'
```

Expected: PASS

- [ ] **Step 5: Run the broader backend package**

Run:

```bash
cd backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./internal/http ./internal/models
```

Expected: PASS

- [ ] **Step 6: Commit the backend sponsor-create simplification**

```bash
git add backend/internal/http/handlers.go backend/internal/http/router.go backend/internal/http/wallet_challenges.go backend/internal/models/models.go backend/internal/models/wallet_challenges.go backend/internal/http/create_campaign_integration_test.go
git commit -m "Remove sponsor wallet challenge from create flow"
```

### Task 2: Convert Frontend Create Page To One Button And One Popup

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/pages/CreateCampaign.tsx`

- [ ] **Step 1: Write the failing frontend type/build checks**

Update the frontend types first so TypeScript exposes the old dependency on challenge fields:

```ts
// frontend/src/types/index.ts
export interface CreateCampaignRequest {
  repo: string;
  pool_amount: number;
  deadline: string;
  sponsor_wallet: string;
}
```

Then run the build before changing implementation.

- [ ] **Step 2: Run frontend build to verify it fails**

Run:

```bash
cd frontend && npm run build
```

Expected: FAIL because `CreateCampaign.tsx` and `api/client.ts` still reference `createCampaignChallenge`, `challenge_id`, `signature`, `signMessage`, `step`, or `handleFund`.

- [ ] **Step 3: Implement the minimal one-popup frontend flow**

Apply these changes:

```ts
// frontend/src/api/client.ts
export const api = {
  createCampaign(data: CreateCampaignRequest): Promise<CreateCampaignResponse> {
    return request('/campaigns/', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },
  // remove createCampaignChallenge(...)
}
```

```tsx
// frontend/src/pages/CreateCampaign.tsx
export default function CreateCampaign() {
  const { publicKey, signTransaction } = useWallet();
  // remove signMessage
  // remove step state
  // remove preparedTx/preparedSponsorWallet step UI state

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setError(null);

    if (!publicKey) {
      setVisible(true);
      return;
    }
    if (!signTransaction) {
      setError('This wallet does not support transaction signing.');
      return;
    }

    const deadlineRFC3339 = toStableRFC3339(deadline);
    const poolLamports = Math.round(parseFloat(poolSol) * 1e9);

    const result = await api.createCampaign({
      repo,
      pool_amount: poolLamports,
      deadline: deadlineRFC3339!,
      sponsor_wallet: publicKey.toBase58(),
    });

    if (!result.unsigned_tx?.trim()) {
      throw new Error('Create transaction was not returned by the backend.');
    }

    await submitPreparedCampaign(
      result.campaign_id,
      result.unsigned_tx,
      publicKey.toBase58(),
      poolLamports,
      deadlineRFC3339!
    );
  }
}
```

Also remove:
- step indicator markup
- second-step explanatory copy
- `handleFund()`
- `preparedTx`, `preparedSponsorWallet`, `createdId`, and `step` state if they are no longer needed

The page should end with one CTA that triggers build → Phantom → send → confirm → redirect.

- [ ] **Step 4: Re-run frontend build**

Run:

```bash
cd frontend && npm run build
```

Expected: PASS

- [ ] **Step 5: Commit the frontend one-popup flow**

```bash
git add frontend/src/api/client.ts frontend/src/types/index.ts frontend/src/pages/CreateCampaign.tsx
git commit -m "Simplify sponsor create flow to one popup"
```

### Task 3: Remove Tracked `backend/api` Binary From Git

**Files:**
- Modify: `.gitignore`
- Remove from git index: `backend/api`

- [ ] **Step 1: Add the ignore rule**

Update `.gitignore`:

```gitignore
# Local build artifacts
backend/api
```

- [ ] **Step 2: Remove the tracked binary from git while keeping local builds possible**

Run:

```bash
git rm --cached backend/api
```

Expected: `backend/api` becomes deleted in git, but only as a tracked artifact removal.

- [ ] **Step 3: Verify binary cleanup**

Run:

```bash
git status --short
```

Expected:
- `.gitignore` modified
- `backend/api` staged as deleted
- no unrelated source files touched by this task

- [ ] **Step 4: Commit the cleanup**

```bash
git add .gitignore
git commit -m "Stop tracking backend build artifact"
```

### Task 4: Full Verification And Smoke

**Files:**
- Verify only: `backend/internal/http/handlers.go`
- Verify only: `frontend/src/pages/CreateCampaign.tsx`
- Verify only: `.gitignore`

- [ ] **Step 1: Run focused backend and frontend verification**

Run:

```bash
cd backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./internal/http
cd ../frontend && npm run build
```

Expected: PASS

- [ ] **Step 2: Run full backend suite**

Run:

```bash
cd backend && GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod-cache go test ./...
```

Expected: PASS

- [ ] **Step 3: Perform manual smoke on running stack**

Validate manually:

```text
1. Open /create.
2. Fill repo / amount / deadline.
3. Click Create Campaign once.
4. Phantom opens exactly once.
5. Approve transaction.
6. Frontend redirects to /campaign/{id}.
7. Campaign appears only after successful create-confirm.
8. No sponsor create-challenge request appears in the browser network log.
```

- [ ] **Step 4: Commit final stabilization if needed**

```bash
git add backend frontend .gitignore
git commit -m "Finalize one-popup sponsor create flow"
```
