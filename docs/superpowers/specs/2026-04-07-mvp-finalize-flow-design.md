# RepoBounty AI — MVP Finalize Flow Design

Date: 2026-04-07  
Status: Approved

---

## Goal

Deliver a reliable MVP happy path:  
**Create campaign → auto/manual finalize after deadline → contributor claims reward**

All changes are a targeted delta on the existing codebase. No big refactors.

---

## A. Full-history finalization (default for all)

**File:** `backend/internal/github/campaign_window.go`

Remove the `!c.isProduction` guard. Full-history analysis (`FetchContributors` + `FetchContributorsPRDiffs` with no time window) becomes the unconditional default for all environments.

- `ContributorSource` → `"repository_history_mvp"`
- `ContributorNotes` → honest message that full repo history is used in MVP
- Window-based path stays in the file as dead code (future switch point), but is never reached by default

**Test:** `campaign_window_test.go` — assert that `FetchContributionWindowData` always returns `contributor_source == "repository_history_mvp"` regardless of client configuration.

---

## B. Wallet-proof manual finalize

### Models (`backend/internal/models/wallet_challenges.go`)

Add constant:
```go
WalletChallengeActionFinalize WalletChallengeAction = "finalize"
```

Add request type:
```go
type FinalizeChallengeRequest struct {
    WalletAddress string `json:"wallet_address"`
}
```

### Wallet proof (`backend/internal/walletproof/challenge.go`)

Add `FinalizeMessageInput` struct and `BuildFinalizeMessage` function (same pattern as `BuildClaimMessage`):
- Fields: `ChallengeID`, `CampaignID`, `SponsorWallet`, `IssuedAt`, `ExpiresAt`
- Prefix: `"Action: finalize_campaign"`

### Handlers (`backend/internal/http/handlers.go`)

**Extract shared core:** `performFinalizeCampaign(ctx, campaign) (*models.FinalizeResponse, error)`
- Loads or computes a finalize snapshot (same logic as current `Finalize` and auto-worker)
- Calls `solana.FinalizeCampaign`
- Updates store
- Returns `FinalizeResponse`

Used by:
- `FinalizeWithWalletProof` (new)
- `Finalize` (existing GitHub-auth handler — refactored to call core)
- `autoFinalize` worker — refactored to call core

**`FinalizeChallenge` handler** (no GitHub auth required):
1. Load campaign; verify `state == funded` and `deadline` passed
2. Verify `req.WalletAddress == campaign.Sponsor`
3. Issue one-time expiring `WalletChallenge` with `Action = finalize`
4. Return challenge ID + message to sign

**`FinalizeWithWalletProof` handler** (no GitHub auth required):
1. Load campaign; verify `state == funded` and `deadline` passed
2. Verify wallet challenge (action=finalize, not used, not expired, signature valid)
3. Verify `challenge.WalletAddress == campaign.Sponsor`
4. Mark challenge used
5. Call `performFinalizeCampaign`
6. Return `FinalizeResponse`

### Router (`backend/internal/http/router.go`)

Add two unauthenticated routes:
```
POST /api/campaigns/{id}/finalize-challenge
POST /api/campaigns/{id}/finalize-wallet
```

Keep existing `POST /api/campaigns/{id}/finalize` (GitHub-auth, backward compat).

---

## C. Auto-finalize worker

**File:** `backend/internal/http/worker.go`

Refactor `autoFinalize` to call `performFinalizeCampaign` instead of duplicating finalize logic. Behavior unchanged.

**Test:** `worker_test.go` — assert funded campaigns past deadline are finalized; unfunded or pre-deadline campaigns are skipped.

---

## D. Frontend sponsor fallback button

**File:** `frontend/src/pages/CampaignDetails.tsx`

Show sponsor fallback card when **all** are true:
- `campaign.state === 'funded'`
- `new Date(campaign.deadline) < new Date()`
- `publicKey?.toBase58() === campaign.sponsor`

UX:
- Explanatory text: "The auto-finalize worker usually handles this. If it hasn't run yet, you can finalize now."
- "Finalize now" button — no preview gate required
- Flow: `POST finalize-challenge` → `signMessage(challenge.message)` → `POST finalize-wallet`
- Loading / error / success states; on success reload campaign

**File:** `frontend/src/api/client.ts`

Add:
```ts
finalizeChallenge(campaignId: string, walletAddress: string): Promise<WalletChallengeResponse>
finalizeWallet(campaignId: string, walletAddress: string, challengeId: string, signature: string): Promise<FinalizeResponse>
```

---

## E. Claim flow

No changes. After refactoring finalize core, verify existing claim path compiles and passes tests.

**Test:** `claim_integration_test.go` — assert claim still works after finalization.

---

## F. Test coverage

| Test file | What it covers |
|-----------|---------------|
| `campaign_window_test.go` | Full-history always used |
| `worker_test.go` | Auto-finalize funded-past-deadline campaigns |
| `finalize_wallet_test.go` (new) | Wallet-proof finalize: sponsor accepted, non-sponsor rejected, pre-deadline rejected |
| `claim_integration_test.go` | Claim works after finalization |

---

## Out of scope

- Solana program changes (not needed)
- Separate funding wizard (not reintroduced)
- GitHub-owner-only finalize path (removed as sole path)
- Preview as mandatory gate for manual finalize
