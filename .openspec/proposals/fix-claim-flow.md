# Proposal: Fix Claim Flow — Post-Transaction UI State Race Condition

**Status:** Final (v4.2 — implementation-ready, ambiguity-free)
**Author:** agent
**Date:** 2026-05-11
**Scope:** Frontend + Backend
**Severity:** High — users see false errors after successful on-chain claims

---

## Non-Goals

This proposal explicitly does **not** include:

- Backend data model or schema redesign
- Per-allocation on-chain claim-record scan on page load (would require N RPC calls per load)
- `localStorage` or any persistent storage for claim UI state
- Automatic TX resubmission after a TX has already been submitted
- Changes to campaign creation, finalization, refund, or auth flows
- New npm dependencies

---

## Problem

After a user successfully claims a reward (wallet confirms, balance increases, TX lands on-chain), the UI shows an error and the Claim button remains visible. The user must manually refresh and sometimes retry to see the correct CLAIMED state.

---

## Current Behavior (Reproduced)

| Step | What happens | Correct? |
|---|---|---|
| 1. User clicks Claim | Wallet opens, user signs message | Yes |
| 2. Frontend calls `claimAllocation` | Backend builds partial TX | Yes |
| 3. Frontend calls `signTransaction` | Wallet opens, user approves TX | Yes |
| 4. Frontend calls `sendRawTransaction` + `confirmTransaction('confirmed')` | TX confirmed on-chain, balance increases | Yes |
| 5. Frontend calls `claimConfirm` | Backend calls `GetAccountInfo` **without commitment param** → defaults to `finalized` → claim record not yet updated → returns 409 `"claim is not confirmed on-chain yet"` (no structured error code) | **No** |
| 6. Frontend catch block shows red error | User sees misleading error | **No** |
| 7. `claimedContributors` not updated | Claim button remains visible | **No** |
| 8. User refreshes page | `getCampaign` → `loadCampaign` merges on-chain campaign state (`confirmed` commitment → `completed`) with DB allocations (still `claimed: false` because `ClaimConfirm` never succeeded) | **No** |
| 9. User clicks Claim again | Backend `Claim` handler checks on-chain claim record (now finalized), sees `claimed: true`, updates DB, returns 409 `CLAIM_ALREADY_CLAIMED` | **No** |
| 10. Frontend catches "already claimed" | Finally updates `claimedContributors`, shows CLAIMED | **No** (takes 2 attempts) |

---

## Root Cause Analysis

### RC-1: Commitment Level Mismatch in `GetClaimStatus`

**Location:** `backend/internal/solana/client.go:395`

```go
account, err := c.rpcClient.GetAccountInfo(ctx, claimRecordPDA)
```

No commitment parameter → defaults to `finalized`. Frontend confirms at `confirmed` (1-2 slots). Backend queries at `finalized` (~32 slots). Race condition: claim record not yet visible at `finalized`.

### RC-2: No Frontend Retry/Polling

**Location:** `CampaignDetails.tsx:179-205`

`claimConfirm` failure treated as terminal. No retry, no pending state.

### RC-3: Misleading Error Classification

"claim is not confirmed on-chain yet" shown as red error banner. It is a **transient state**.

### RC-4: Page Refresh Shows Stale Allocation State

**Location:** `handlers.go:1568-1597`, `handlers.go:1599-1697`

On-chain campaign account has `Allocations: nil` (`client.go:1073`). `mergeCampaignWithChainData` uses `stored.Allocations` from DB (line 1675-1677). DB not updated because `ClaimConfirm` failed.

### RC-5: `claimedContributors` Not Set on Transient Failure

**Location:** `CampaignDetails.tsx:179-205`

Only the `"already claimed"` path sets the guard. Transient failure path does not.

### RC-6: Missing Structured Error Code for `ClaimConfirm` Rejection

**Location:** `handlers.go:1130`

`ClaimConfirm` returns `"claim is not confirmed on-chain yet"` via `writeError` (no `code` field). Other claim errors use `writeCodedError` with structured codes. Frontend falls back to fragile message substring matching.

---

## Backend Commitment Audit

| Method | File:Line | Commitment | Change? |
|---|---|---|---|
| `GetProgramAccountsWithOpts` | `client.go:114-117` | `CommitmentConfirmed` | No |
| `GetBalance` | `client.go:237` | `CommitmentConfirmed` | No |
| `GetMinimumBalanceForRentExemption` | `client.go:255-258` | `CommitmentConfirmed` | No |
| `GetLatestBlockhash` (claim) | `client.go:320` | `CommitmentFinalized` | No |
| `GetAccountInfo` (claim record) | `client.go:395` | **None (→ `finalized`)** | **Yes → `CommitmentConfirmed`** |
| `GetLatestBlockhash` (finalize) | `client.go:494` | `CommitmentFinalized` | No |
| `GetLatestBlockhash` (refund) | `client.go:632` | `CommitmentFinalized` | No |
| `GetLatestBlockhash` (create) | `client.go:813` | `CommitmentFinalized` | No |
| `SendTransaction` | `client.go:837` | Default | No |

Only one call changes.

---

## Source-of-Truth Definitions

### Three distinct concepts, never conflated:

| Concept | Variable | Role | Source | Persistence |
|---|---|---|---|---|
| **Authoritative claimed state** | `campaign.allocations[].claimed` | Business truth — whether allocation is claimed | Backend DB via `getCampaign` API response | Survives page refresh, tab close, browser restart |
| **Ephemeral claim UI state** | `claimPhase` (discriminated union) | Tracks current step in the active claim flow for one allocation | Local React state only | Lost on unmount, page refresh, navigation |
| **Anti-double-submit guard** | `claimedContributors` (map) | Prevents Claim button re-rendering after TX submission but before backend confirms | Local React state only | Lost on unmount, page refresh, navigation |

### Lifecycle rules for `claimedContributors`:

- Page-instance only — never persisted to `localStorage`, `sessionStorage`, or any external store
- Reset on campaign ID change (if user navigates between campaigns)
- Reset on page refresh (intentional — backend payload is the truth)
- Not a source of business truth — only a UI guard
- **Cleared per-contributor only when:**
  - (a) Backend payload confirms `a.claimed === true` for that contributor (guard is superseded by authoritative data), OR
  - (b) The flow ended in a pre-submit recoverable state (`wallet_rejected`, `validation_error`) — no TX was submitted, the guard is unnecessary
- **NOT cleared when `claimPhase` returns to `idle` after `refetching` or `sync_failed`.** The guard must persist so the button does not reappear after a TX was submitted.
- Remains set during `sync_failed` and refetch-failure fallback so the button stays hidden until the next page refresh reconciles from backend.

### Active vs. non-active claim states

`claimPhase` states fall into two categories:

**Active flow states** — a claim operation is in progress:

| State | Meaning |
|---|---|
| `wallet_prompt` | Waiting for user to sign message |
| `tx_building` | Backend building partial TX |
| `tx_confirming` | TX sent, awaiting on-chain confirmation |
| `confirming_backend` | Calling `claimConfirm` (may be retrying) |
| `refetching` | `claimConfirm` succeeded, fetching authoritative backend data |

**Non-active terminal/recoverable states** — no operation in progress, local UI only:

| State | Meaning |
|---|---|
| `idle` | No claim activity |
| `wallet_rejected` | User declined signing — recoverable |
| `validation_error` | Backend validation failed — recoverable |
| `sync_failed` | Backend sync retries exhausted — terminal, refresh required |

### Single active claim constraint:

When `claimPhase.status` is an **active flow state**, Claim buttons for **all** allocations are hidden. The user cannot start a second claim while one is in progress.

When `claimPhase.status` is a **non-active state** (`idle`, `wallet_rejected`, `validation_error`, `sync_failed`), Claim buttons for **other** allocations (not the one with terminal local state) are shown normally. The button for the `sync_failed` contributor remains hidden via the `claimedContributors` guard. The buttons for `wallet_rejected` and `validation_error` contributors are visible and enabled (recoverable).

---

## Backend Error Code Audit and Gaps

### Existing structured codes (backend already returns `{ error: string, code: string }`):

| Code | HTTP status | When | Handler |
|---|---|---|---|
| `CLAIM_ALREADY_CLAIMED` | 409 | Allocation already claimed (checked before TX build and after on-chain check) | `Claim`, `Claim` |
| `CAMPAIGN_NOT_FINALIZED` | 409 | Campaign not in finalized/completed state | `Claim` |
| `CLAIM_NOT_YOUR_OWN` | 403 | Authenticated user is not the allocation contributor | `Claim` |
| `CONTRIBUTOR_NOT_FOUND` | 404 | Contributor not in campaign allocations | `Claim` |
| `BAD_REQUEST` | 400 | Generic validation failure | `Claim` |

### Missing codes (currently use `writeError` without `code` field):

| Location | Message | Needs code |
|---|---|---|
| `handlers.go:1130` | `"claim is not confirmed on-chain yet"` | **Yes → `CLAIM_NOT_CONFIRMED_ON_CHAIN`** |
| `handlers.go:1134` | `"claim was finalized to a different wallet"` | **Yes → `CLAIM_WALLET_MISMATCH`** |
| `handlers.go:1126` | `"failed to confirm on-chain claim"` | **Yes → `CLAIM_CHAIN_LOOKUP_FAILED`** |

### Frontend error detection strategy:

1. **Primary:** Check `err.code` (structured field from `{ error, code }` JSON response)
2. **Fallback:** Message substring matching (for cases where code is missing or for non-API errors like wallet rejections)

All new code must use `err.code` first. Substring matching is only for backward compatibility and non-API errors.

### Complete error code reference for frontend:

| Code | Meaning | Frontend action |
|---|---|---|
| `CLAIM_ALREADY_CLAIMED` | Allocation claimed on-chain | Treat as success → refetch → CLAIMED |
| `CLAIM_NOT_CONFIRMED_ON_CHAIN` | TX not yet visible at queried commitment | Retry with backoff |
| `CLAIM_WALLET_MISMATCH` | Claimed to different wallet | Terminal error (inline) |
| `CLAIM_CHAIN_LOOKUP_FAILED` | RPC error reading chain | Retry up to 2 times, then `sync_failed` |
| `CAMPAIGN_NOT_FINALIZED` | Campaign not finalized | `validation_error`, button re-enabled |
| `CLAIM_NOT_YOUR_OWN` | Not your allocation | `validation_error`, button re-enabled |
| `CONTRIBUTOR_NOT_FOUND` | Contributor missing | `validation_error`, button re-enabled |
| `BAD_REQUEST` | Generic validation | `validation_error`, button re-enabled |

---

## Target UX Flow

### Normal Path

```
User clicks "Claim X SOL"
  → "Signing message..."          (button hidden, claimPhase: wallet_prompt)
  → "Building transaction..."     (button hidden, claimPhase: tx_building)
  → "Confirming on-chain..."      (button hidden, claimPhase: tx_confirming)
  → claimConfirm succeeds
  → set claimedContributors guard (button hidden)
  → "Claim confirmed. Refreshing..." (transient, claimPhase: refetching)
  → getCampaign refetch resolves
  → setCampaign(refreshed)
  → claimPhase → idle
  → CLAIMED badge + wallet from refreshed backend payload
```

### Recovery Fallback (Browser Crash / Tab Close Before Sync)

This is **not** a normal UX path. It is a safety net only. It should be extremely rare.

```
Page refresh after crash
  → Backend DB not updated → allocation.claimed: false
  → Claim button visible (unavoidable — backend doesn't know yet)
  → User clicks Claim
  → Backend Claim handler checks on-chain → already claimed → updates DB → 409 CLAIM_ALREADY_CLAIMED
  → Frontend: no error shown → refetch → CLAIMED from backend payload
```

---

## Technical Design

### Part 1: Backend — Use `confirmed` Commitment for `GetClaimStatus`

**File:** `backend/internal/solana/client.go:395`

```go
account, err := c.rpcClient.GetAccountInfoWithOpts(ctx, claimRecordPDA, &rpc.GetAccountInfoOpts{
    Commitment: rpc.CommitmentConfirmed,
})
```

### Part 2: Backend — Add Structured Error Codes to `ClaimConfirm`

**File:** `backend/internal/http/handlers.go`

Change line 1126 (chain lookup failure):
```go
writeCodedError(w, http.StatusBadGateway, "failed to confirm on-chain claim", "CLAIM_CHAIN_LOOKUP_FAILED")
```

Change line 1130 (not confirmed yet):
```go
writeCodedError(w, http.StatusConflict, "claim is not confirmed on-chain yet", "CLAIM_NOT_CONFIRMED_ON_CHAIN")
```

Change line 1134 (wallet mismatch):
```go
writeCodedError(w, http.StatusConflict, "claim was finalized to a different wallet", "CLAIM_WALLET_MISMATCH")
```

### Part 3: Frontend — Claim State Machine

**File:** `frontend/src/pages/CampaignDetails.tsx`

#### State definitions

```typescript
type ClaimPhase =
  | { status: 'idle' }
  | { status: 'wallet_prompt'; contributor: string }
  | { status: 'tx_building'; contributor: string }
  | { status: 'tx_confirming'; contributor: string }
  | { status: 'confirming_backend'; contributor: string }
  | { status: 'refetching'; contributor: string }
  | { status: 'wallet_rejected'; contributor: string }
  | { status: 'sync_failed'; contributor: string; message: string }
  | { status: 'validation_error'; contributor: string; message: string };
```

Note: there is no `claimed` state in `claimPhase`. The CLAIMED rendering is driven entirely by `campaign.allocations[].claimed === true` from the backend payload. The `refetching` state is a transient non-error success-sync state.

#### Transitions

| From | To | Trigger |
|---|---|---|
| `idle` | `wallet_prompt` | User clicks Claim |
| `wallet_prompt` | `tx_building` | `signMessage` resolves |
| `wallet_prompt` | `wallet_rejected` | `signMessage` rejects |
| `tx_building` | `tx_confirming` | `sendRawTransaction` resolves |
| `tx_building` | `wallet_rejected` | `signTransaction` rejects |
| `tx_building` | `validation_error` | `claimAllocation` returns non-retryable error |
| `tx_building` | `validation_error` | `claimChallenge` network failure with non-retryable status |
| `tx_confirming` | `confirming_backend` | `confirmTransaction` resolves |
| `tx_confirming` | `sync_failed` | `confirmTransaction` timeout |
| `confirming_backend` | `refetching` | `claimConfirm` returns 200 or CLAIM_ALREADY_CLAIMED |
| `confirming_backend` | `confirming_backend` | `claimConfirm` returns CLAIM_NOT_CONFIRMED_ON_CHAIN → retry |
| `confirming_backend` | `sync_failed` | All retries exhausted |
| `refetching` | `idle` | `getCampaign` refetch resolves — campaign state is now authoritative |
| `refetching` | `idle` | Refetch fails after 2 retries — local fallback (see Part 5) |
| `wallet_rejected` | `wallet_prompt` | User clicks Claim again |
| `validation_error` | `wallet_prompt` | User clicks Claim again |

### Part 4: Exact Post-Success Sequence

After `claimConfirm` returns 200 or CLAIM_ALREADY_CLAIMED:

```
1. setClaimedContributors(prev => ({ ...prev, [contributor]: true }))
   // Button hidden by guard. No TX can be re-submitted.

2. setClaimPhase({ status: 'refetching', contributor })
   // Transient non-error success-sync state.
   // Render: "Claim confirmed. Refreshing latest data..." (not CLAIMED badge yet)

3. try {
     const refreshed = await api.getCampaign(id);
     setCampaign(refreshed);
     // campaign.allocations[].claimed is now true from backend.
     // Final CLAIMED rendering comes from this payload.
   } catch {
     // Refetch failed — see Part 5
   }

4. setClaimPhase({ status: 'idle' })
   // claimPhase returns to idle. The allocation card now reads
   // a.claimed === true from the refreshed campaign state.
   // CLAIMED badge + wallet address rendered from backend payload.
```

**The CLAIMED badge is never rendered from `claimPhase`.** It is rendered from `a.claimed === true` in `campaign.allocations`. The `refetching` state shows a transient "Claim confirmed. Refreshing..." message, then `idle` + backend `claimed: true` takes over.

### Part 5: Refetch Failure After Successful `claimConfirm`

If the mandatory `getCampaign` refetch fails:

1. Retry refetch up to 2 times with 500ms backoff
2. If refetch succeeds: normal flow — `setCampaign(refreshed)`, `claimPhase → idle`
3. If all refetch retries fail:
   - `setClaimPhase({ status: 'idle' })`
   - The `claimedContributors` guard remains set for this contributor — button stays hidden even though `claimPhase` is `idle`
   - Render a **degraded success-sync state** on the allocation card:
     - Show a neutral "Claim confirmed." text (NOT the full CLAIMED badge, since we don't have the backend's `claimant_wallet` value)
     - This is **not** a final authoritative CLAIMED state. It is a degraded mode indicating the claim succeeded but the UI lacks authoritative backend data.
     - Do NOT fabricate fields — no guessed wallet address, no invented timestamps
     - The backend's `claimConfirm` 200 response already confirmed the claim succeeded
   - The final authoritative CLAIMED render happens only when the backend payload later contains `a.claimed === true` — either from a subsequent refetch, background poll, or the next page refresh

### Part 5b: `CLAIM_WALLET_MISMATCH` Handling

When `claimConfirm` returns `CLAIM_WALLET_MISMATCH`:

1. Do **not** treat as a terminal error immediately
2. Perform one immediate authoritative `getCampaign` refetch
3. If the refetched backend payload shows `a.claimed === true` for this allocation: render CLAIMED badge from backend payload (button hidden). The mismatch likely means the user's connected wallet differs from the one used for the claim, but the allocation is claimed nonetheless.
4. If the refetched payload still shows `a.claimed === false` (edge case): show terminal inline error "This reward was claimed to a different wallet." Button remains hidden. Page refresh required.

### Part 6: Rendering Precedence Rules

When rendering an allocation card, evaluate conditions in this exact order. First match wins:

| Priority | Condition | Render |
|---|---|---|
| **1** | `a.claimed === true` (from `campaign.allocations` — backend payload) | CLAIMED badge + `a.claimant_wallet` from backend. Button not rendered. |
| **2** | `claimPhase.contributor === a.contributor` AND `claimPhase.status` is one of: `wallet_prompt`, `tx_building`, `tx_confirming`, `confirming_backend`, `refetching` | Pending inline status message. Button not rendered. |
| **3** | `claimPhase.contributor === a.contributor` AND `claimPhase.status` is `wallet_rejected` or `validation_error` | Error inline message. Button rendered and enabled. |
| **4** | `claimPhase.contributor === a.contributor` AND `claimPhase.status` is `sync_failed` | Yellow sync warning. Button not rendered. |
| **5** | `claimedContributors[a.contributor] === true` (local guard, no active claimPhase) | Degraded success-sync: "Claim confirmed." text without wallet address. Button not rendered. Not final authoritative CLAIMED. |
| **6** | Default: `a.claimed === false`, no guard, `claimPhase` does not target this contributor | Normal allocation display. Button rendered and enabled (if user owns it). |

Priority 1 always wins over all others. If `a.claimed === true` from backend, the UI always shows CLAIMED regardless of any local state.

### Part 7: Error Taxonomy by Stage

Each error that can occur during the claim flow, with exact behavior:

#### Stage: Challenge Request

| Error | Code | Message shown | Button | Retryable | Page refresh? |
|---|---|---|---|---|---|
| Network failure (fetch error, timeout) | *(no code — network error)* | "Could not reach the server. Please try again." | Visible, enabled | Yes — user retries | No |
| 401 Unauthorized | *(401 status)* | "Please log in to claim." | Visible, enabled | Yes — after login | No |
| 503 Solana not configured | *(503 status)* | "Claims are unavailable until backend is connected to Solana." | Visible, enabled | No | No |

#### Stage: Claim Allocation (TX Build)

| Error | Code | Message shown | Button | Retryable | Page refresh? |
|---|---|---|---|---|---|
| `CAMPAIGN_NOT_FINALIZED` | `CAMPAIGN_NOT_FINALIZED` | "Campaign is not finalized." | Visible, enabled | No | No |
| `CLAIM_NOT_YOUR_OWN` | `CLAIM_NOT_YOUR_OWN` | "You can only claim your own allocation." | Visible, enabled | No | No |
| `CONTRIBUTOR_NOT_FOUND` | `CONTRIBUTOR_NOT_FOUND` | "Contributor not found in allocations." | Visible, enabled | No | No |
| `CLAIM_ALREADY_CLAIMED` | `CLAIM_ALREADY_CLAIMED` | *(no message — treat as success)* | Hidden → refetch → CLAIMED | N/A (success) | No |
| `BAD_REQUEST` | `BAD_REQUEST` | Backend error message | Visible, enabled | No | No |
| Network failure | *(no code)* | "Could not reach the server. Please try again." | Visible, enabled | Yes | No |

#### Stage: Wallet Message Signing

| Error | Code | Message shown | Button | Retryable | Page refresh? |
|---|---|---|---|---|---|
| User declined | *(wallet error)* | "Transaction was not approved." | Visible, enabled | Yes — immediately | No |
| Wallet not connected | *(wallet error)* | "Connect your wallet to continue." | Visible, enabled | Yes — after connect | No |

#### Stage: Wallet TX Signing

| Error | Code | Message shown | Button | Retryable | Page refresh? |
|---|---|---|---|---|---|
| User declined | *(wallet error)* | "Transaction was not approved." | Visible, enabled | Yes — immediately | No |

#### Stage: On-Chain Confirmation

| Error | Code | Message shown | Button | Retryable | Page refresh? |
|---|---|---|---|---|---|
| Timeout | *(confirmTransaction timeout)* | "Confirmation is taking longer than expected. Your claim may still be processing." | Hidden | No (sync_failed) | Yes — refresh to check |

#### Stage: ClaimConfirm (Backend Sync)

| Error | Code | Message shown | Button | Retryable | Page refresh? |
|---|---|---|---|---|---|
| Not confirmed yet | `CLAIM_NOT_CONFIRMED_ON_CHAIN` | *(none — silent retry)* | Hidden | Yes — 1 initial + 4 retries = 5 attempts total | No |
| `CLAIM_ALREADY_CLAIMED` | `CLAIM_ALREADY_CLAIMED` | *(none — treat as success)* | Hidden → refetch → CLAIMED | N/A (success) | No |
| `CLAIM_WALLET_MISMATCH` | `CLAIM_WALLET_MISMATCH` | *(none — immediate refetch)* | Hidden → refetch → CLAIMED or terminal error | No | Only if refetch shows unclaimed |
| `CLAIM_CHAIN_LOOKUP_FAILED` | `CLAIM_CHAIN_LOOKUP_FAILED` | *(none — retry)* | Hidden | Yes — 1 initial + 2 retries = 3 attempts total | No |
| Network failure | *(no code)* | *(none — retry)* | Hidden | Yes — 1 initial + 2 retries = 3 attempts total | No |
| All retries exhausted | *(last error)* | "Your claim was submitted and is being confirmed. Refresh the page to check your status." | Hidden | No | Yes |

#### Stage: Recovery Fallback (Already Claimed on Click)

| Error | Code | Message shown | Button | Retryable | Page refresh? |
|---|---|---|---|---|---|
| `CLAIM_ALREADY_CLAIMED` from `Claim` handler | `CLAIM_ALREADY_CLAIMED` | *(none)* | Hidden → refetch → CLAIMED | N/A (success) | No |

### Part 8: Frontend Retry/Polling for `claimConfirm`

#### Terminology

- **Attempt** = every `api.claimConfirm(...)` call, including the first one.
- **Retry** = any repeated call after the first failed attempt.

An "initial attempt + N retries" means N+1 total calls.

#### Retry policy

The retry budget is determined by the **last received error category**. Each error type has its own independent budget. When a call fails with error type A after some retries of type A, then the next call fails with error type B, the type B budget starts fresh. In practice, error types do not interleave (the backend returns a consistent code for a given state), but the rule is: budget is per-error-category, not cumulative.

| Error category | Total attempts | Initial + retries | Delays before retries | Total worst-case |
|---|---|---|---|---|
| `CLAIM_NOT_CONFIRMED_ON_CHAIN` | 5 | 1 initial + 4 retries | 1s, 2s, 4s, 8s | ~15s |
| `CLAIM_CHAIN_LOOKUP_FAILED` | 3 | 1 initial + 2 retries | 2s, 4s | ~6s |
| Pure network error (no code) | 3 | 1 initial + 2 retries | 2s, 4s | ~6s |
| `CLAIM_ALREADY_CLAIMED` | 1 | 1 initial + 0 retries | N/A | 0s (success) |
| `CLAIM_WALLET_MISMATCH` | 1 | 1 initial + 0 retries | N/A | 0s (terminal) |
| Any other error with code | 1 | 1 initial + 0 retries | N/A | 0s (terminal) |

#### Pseudocode

```typescript
async function retryClaimConfirm(
  id: string,
  contributor: string,
  walletAddress: string,
  txSignature: string
): Promise<void> {
  const CONFIRM_TOTAL = 5;   // 1 initial + 4 retries
  const INFRA_TOTAL = 3;     // 1 initial + 2 retries
  const confirmDelays = [1000, 2000, 4000, 8000]; // before each retry (4 entries)
  const infraDelays = [2000, 4000];                // before each retry (2 entries)

  let confirmAttempts = 0; // counts toward CONFIRM_TOTAL
  let infraAttempts = 0;   // counts toward INFRA_TOTAL

  // Attempt 1 (initial): no delay before first call
  // Attempt 2+: delay based on retry number within the active category

  while (true) {
    // Determine delay: no delay before attempt 1 of any category
    let delay = 0;
    if (confirmAttempts > 0) {
      delay = confirmDelays[Math.min(confirmAttempts - 1, confirmDelays.length - 1)];
    } else if (infraAttempts > 0) {
      delay = infraDelays[Math.min(infraAttempts - 1, infraDelays.length - 1)];
    }
    if (delay > 0) await new Promise((r) => setTimeout(r, delay));

    try {
      await api.claimConfirm(id, contributor, walletAddress, txSignature);
      return; // success
    } catch (e: unknown) {
      const code = getErrorCode(e);

      if (code === 'CLAIM_ALREADY_CLAIMED') return; // success

      if (code === 'CLAIM_NOT_CONFIRMED_ON_CHAIN') {
        confirmAttempts++;
        if (confirmAttempts < CONFIRM_TOTAL) continue;
        throw e; // exhausted: 5th attempt failed
      }

      if (code === 'CLAIM_CHAIN_LOOKUP_FAILED' || (!code && isNetworkError(e))) {
        infraAttempts++;
        if (infraAttempts < INFRA_TOTAL) continue;
        throw e; // exhausted: 3rd attempt failed
      }

      // Any other error: throw immediately (1 attempt only)
      throw e;
    }
  }
}

function getErrorCode(e: unknown): string | undefined {
  if (!(e instanceof Error)) return undefined;
  return (e as Error & { code?: string }).code;
}
```

Error detection: `getErrorCode()` checks the structured `code` field first. Message substring matching is fallback only for errors without codes (wallet errors, network errors).

### Part 9: Page Load / Refresh — Source of Truth

On page load:

1. `GET /campaigns/:id` → `loadCampaign` → merges on-chain state + DB allocations
2. Allocations come from DB (on-chain campaign has `Allocations: nil`)
3. `campaign.allocations[].claimed` from backend is the single source of truth
4. `claimedContributors` starts empty (page-instance only, never persisted)
5. `claimPhase` starts at `{ status: 'idle' }`

If `a.claimed === true` from backend: CLAIMED badge, no button. This is always correct.

If `a.claimed === false` but the allocation is actually claimed on-chain (crash recovery): button appears, user clicks, recovery fallback fires (Part 7).

---

## Acceptance Criteria

### Final CLAIMED rendering

| # | Criterion |
|---|---|
| AC-1 | The CLAIMED badge and claimed wallet address in the final UI are rendered from `campaign.allocations[].claimed === true` and `campaign.allocations[].claimant_wallet` in the refreshed backend `getCampaign` payload. |
| AC-2 | `claimPhase` is ephemeral UI state only. It is never used as the source of truth for the final CLAIMED rendering. |
| AC-3 | If `claimConfirm` succeeds but `getCampaign` refetch fails after 2 retries, a degraded success-sync state is shown: "Claim confirmed." text without fabricating missing fields (no guessed wallet address). This is not a final authoritative CLAIMED state. The final authoritative CLAIMED render happens only when the backend payload later contains `a.claimed === true` — from a subsequent refetch or the next page refresh. |

### Button visibility

| # | Criterion |
|---|---|
| AC-4 | Claim button is **visible and enabled** only when ALL of: user owns the allocation, `a.claimed === false` from backend, `claimedContributors[contributor]` is not set, and `claimPhase` is `idle`, `wallet_rejected`, or `validation_error`. |
| AC-5 | Claim button is **not rendered** (hidden) during: `wallet_prompt`, `tx_building`, `tx_confirming`, `confirming_backend`, `refetching`, `sync_failed`. Also hidden when `a.claimed === true` or `claimedContributors[contributor] === true`. |
| AC-6 | There is no "visible but disabled" button state. Button is either rendered and clickable, or not rendered. |
| AC-7 | Only one claim flow is active at a time. When `claimPhase.status` is an active flow state (`wallet_prompt`, `tx_building`, `tx_confirming`, `confirming_backend`, `refetching`), Claim buttons for all allocations are hidden. When `claimPhase.status` is a non-active state (`idle`, `wallet_rejected`, `validation_error`, `sync_failed`), buttons for other allocations are shown normally. |

### Post-success sequence

| # | Criterion |
|---|---|
| AC-8 | After `confirmTransaction` resolves, UI shows non-final "Finalizing claim..." (button hidden, `claimPhase: confirming_backend`). |
| AC-9 | After `claimConfirm` returns 200 or CLAIM_ALREADY_CLAIMED, the exact sequence is: (1) set `claimedContributors` guard, (2) set `claimPhase` to `refetching`, (3) call `getCampaign` with up to 2 retries on failure, (4) on resolve: `setCampaign(refreshed)`, `claimPhase → idle` with CLAIMED from backend payload. If refetch fails after all retries: `claimPhase → idle` in degraded success-sync mode (see AC-3). |
| AC-10 | During `refetching`, the UI shows "Claim confirmed. Refreshing latest data..." — a transient non-error success-sync state. The CLAIMED badge is NOT shown until the refetch resolves and `a.claimed === true` from the backend payload. |

### Error handling

| # | Criterion |
|---|---|
| AC-11 | Wallet message signing rejection (`signMessage`): button re-enabled immediately, inline "Transaction was not approved." |
| AC-12 | Wallet TX signing rejection (`signTransaction`): button re-enabled immediately, inline "Transaction was not approved." |
| AC-13 | `claimConfirm` returns `CLAIM_NOT_CONFIRMED_ON_CHAIN` (by code, not message): silent retry — 1 initial attempt + 4 retries = 5 total attempts, with delays 1s, 2s, 4s, 8s before retries. No error shown. |
| AC-14 | `claimConfirm` returns `CLAIM_CHAIN_LOOKUP_FAILED` or a pure network error: silent retry — 1 initial attempt + 2 retries = 3 total attempts, with delays 2s, 4s before retries. No error shown. |
| AC-15 | All `claimConfirm` retries exhausted: yellow inline "Your claim was submitted and is being confirmed. Refresh the page to check your status." Button hidden. Page refresh required. |
| AC-16 | `CLAIM_ALREADY_CLAIMED` (by code): no error shown. Refetch → CLAIMED from backend. |
| AC-17 | `CLAIM_WALLET_MISMATCH` (by code): perform one immediate `getCampaign` refetch. If backend payload shows `a.claimed === true`, render CLAIMED from backend. Otherwise show terminal inline "This reward was claimed to a different wallet." Button hidden in both cases. |
| AC-18 | Non-retryable validation errors (by code: `CAMPAIGN_NOT_FINALIZED`, `CLAIM_NOT_YOUR_OWN`, etc.): error inline, button visible and enabled. |
| AC-19 | Claim errors are never shown in the global red error banner. |
| AC-20 | Error code detection uses structured `code` field first. Message substring matching is fallback only. |

### `claimedContributors` guard

| # | Criterion |
|---|---|
| AC-21 | `claimedContributors` is page-instance only. Never persisted to localStorage or any external store. |
| AC-22 | `claimedContributors` is reset when campaign ID changes or on page refresh. |
| AC-23 | `claimedContributors` is not a source of business truth. It only prevents button re-rendering after TX submission. |
| AC-24 | `claimedContributors[contributor]` is cleared only when: (a) backend payload confirms `a.claimed === true` for that contributor, or (b) the flow ended in a pre-submit recoverable state (`wallet_rejected`, `validation_error`). It is NOT cleared when `claimPhase` returns to `idle` after `refetching` or `sync_failed`. |

### Page refresh / recovery

| # | Criterion |
|---|---|
| AC-25 | After page reload where `claimConfirm` previously succeeded: allocation shows `claimed: true` from backend. Claim button never rendered. |
| AC-26 | Browser crash recovery: if reload shows `claimed: false` and user clicks Claim, backend detects on-chain claim, returns CLAIM_ALREADY_CLAIMED (by code). Frontend refetches → shows CLAIMED from backend payload. No error shown. |

### Backend error codes

| # | Criterion |
|---|---|
| AC-27 | `ClaimConfirm` handler returns structured error codes: `CLAIM_NOT_CONFIRMED_ON_CHAIN`, `CLAIM_WALLET_MISMATCH`, `CLAIM_CHAIN_LOOKUP_FAILED`. |
| AC-28 | Backend `GetClaimStatus` uses `confirmed` commitment level. |

### Build verification

| # | Criterion |
|---|---|
| AC-29 | `npm run build` passes. |
| AC-30 | `go test ./...` passes. |
| AC-31 | Existing integration tests continue to pass. |

---

## Implementation Tasks

### Task 1: Backend — Use `confirmed` commitment in `GetClaimStatus`

**File:** `backend/internal/solana/client.go:395`

```go
account, err := c.rpcClient.GetAccountInfoWithOpts(ctx, claimRecordPDA, &rpc.GetAccountInfoOpts{
    Commitment: rpc.CommitmentConfirmed,
})
```

### Task 2: Backend — Add structured error codes to `ClaimConfirm`

**File:** `backend/internal/http/handlers.go`

Replace three `writeError` calls in `ClaimConfirm` with `writeCodedError`:

- Line ~1126: `writeCodedError(w, http.StatusBadGateway, "failed to confirm on-chain claim", "CLAIM_CHAIN_LOOKUP_FAILED")`
- Line ~1130: `writeCodedError(w, http.StatusConflict, "claim is not confirmed on-chain yet", "CLAIM_NOT_CONFIRMED_ON_CHAIN")`
- Line ~1134: `writeCodedError(w, http.StatusConflict, "claim was finalized to a different wallet", "CLAIM_WALLET_MISMATCH")`

### Task 3: Frontend — Define `ClaimPhase` type, remove `claiming` state

**File:** `frontend/src/pages/CampaignDetails.tsx`

Remove `claiming` state. Add `ClaimPhase` discriminated union (Part 3).

### Task 4: Frontend — Rewrite `handleClaim` with state machine

**File:** `frontend/src/pages/CampaignDetails.tsx`

Follow the transition table from Part 3 and the post-success sequence from Part 4.

### Task 5: Frontend — Add `retryClaimConfirm` and `getErrorCode` helpers

**File:** `frontend/src/pages/CampaignDetails.tsx`

As defined in Part 8. Error detection via `code` field, substring fallback.

### Task 6: Frontend — Update allocation card rendering with precedence rules

**File:** `frontend/src/pages/CampaignDetails.tsx`

Implement the 6-level precedence from Part 6.

### Task 7: Frontend — Remove old `claiming` state and error handling

**File:** `frontend/src/pages/CampaignDetails.tsx`

Remove all `claiming` references. Ensure global `error` is never set by claim flow.

### Task 8: Verify backend tests

- `TestClaimConfirmUsesOnChainClaimStatusAsSourceOfTruth`
- `TestClaimChallengeRequiresAuthenticatedGitHubUser`
- `go test ./...`

### Task 9: Build and test verification

- `npm run build`
- `go test ./...`

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `confirmed` commitment unavailable on RPC | Very low | Medium | All major providers support it |
| Retry storm on `claimConfirm` | Low | Low | Per-campaign mutex; max 4 retries with backoff |
| Browser crash before `claimConfirm` | Low | Low | Recovery fallback: Claim → already claimed → refetch |
| Refetch fails after `claimConfirm` succeeds | Low | Low | Local fallback "Claim confirmed." + next refresh reconciles |
| State machine complexity | Medium | Low | Discriminated union; explicit transition table; precedence rules |
| Existing tests fail | Low | Medium | Stubs ignore commitment; error code is in JSON body |

---

## Test Plan

### Backend Tests

| Test | Verifies |
|---|---|
| `TestClaimConfirmUsesOnChainClaimStatusAsSourceOfTruth` | Backend updates state when on-chain shows claimed |
| `TestClaimChallengeRequiresAuthenticatedGitHubUser` | Auth required |
| New: `GetClaimStatus` uses `CommitmentConfirmed` | Commitment parameter |
| New: `ClaimConfirm` returns structured error codes | `CLAIM_NOT_CONFIRMED_ON_CHAIN`, `CLAIM_WALLET_MISMATCH`, `CLAIM_CHAIN_LOOKUP_FAILED` in response body |

### Frontend Manual Tests

| Test case | Steps | Expected |
|---|---|---|
| Happy path | Claim → sign → approve | "Finalizing..." → "Claim confirmed. Refreshing..." → CLAIMED badge with wallet from backend |
| Slow confirmation | Slow network | Pending messages → CLAIMED |
| "Not confirmed" retry | Backend returns CLAIM_NOT_CONFIRMED_ON_CHAIN on first try | Silent retry → CLAIMED |
| Already claimed (recovery) | Click Claim for claimed allocation | CLAIMED from refetch, no error |
| Page refresh after claim | Full flow, refresh | `claimed: true` from backend, no button |
| Crash recovery | Close tab after TX, reopen, click Claim | CLAIMED from refetch, no error |
| Message rejected | Decline `signMessage` | "Transaction was not approved." button enabled |
| TX rejected | Decline `signTransaction` | "Transaction was not approved." button enabled |
| Retries exhausted | Persistent CLAIM_NOT_CONFIRMED_ON_CHAIN | Yellow warning, button hidden, refresh to recover |
| Validation error | Claim on non-finalized campaign | Error inline, button enabled |
| Refetch fails | `claimConfirm` 200, `getCampaign` fails | "Claim confirmed." local fallback, no wallet address. Refresh shows full CLAIMED |
| Wallet mismatch | `CLAIM_WALLET_MISMATCH`, then refetch shows claimed | CLAIMED badge from backend, no error |
| Wallet mismatch (unclaimed) | `CLAIM_WALLET_MISMATCH`, refetch shows unclaimed | "Claimed to different wallet" inline, button hidden |
| Two allocations, claim one | Start claim for allocation A | Allocation B's button also hidden (single active constraint) |

---

## Files to Modify

### Backend
- `backend/internal/solana/client.go` — `GetClaimStatus`, line 395
- `backend/internal/http/handlers.go` — `ClaimConfirm` error codes, lines ~1126, ~1130, ~1134

### Frontend
- `frontend/src/pages/CampaignDetails.tsx` — `ClaimPhase` type, `handleClaim`, `retryClaimConfirm`, `getErrorCode`, allocation rendering

### No changes
- `frontend/src/api/client.ts` — API surface unchanged
- `frontend/src/types/index.ts` — types unchanged

---

## Implementation Checklist

- [ ] **Task 1:** Backend `GetClaimStatus` — `CommitmentConfirmed`
- [ ] **Task 2:** Backend `ClaimConfirm` — structured error codes
- [ ] **Task 3:** Frontend `ClaimPhase` type, remove `claiming`
- [ ] **Task 4:** Frontend rewrite `handleClaim`
- [ ] **Task 5:** Frontend `retryClaimConfirm` + `getErrorCode`
- [ ] **Task 6:** Frontend allocation card precedence rules
- [ ] **Task 7:** Frontend remove old state
- [ ] **Task 8:** Backend test verification
- [ ] **Task 9:** Full build verification

---

## Runtime Strategy

- **Local tools:** Go and Node.js already installed.
- **Backend:** `go test ./...` locally.
- **Frontend:** `npm run build` locally.
- **No Docker needed.**

---

## Generator Strategy

No generators needed. Bug fix modifying existing files.

---

## Deployment Packaging

No deployment config changes. Code-level fix only.
