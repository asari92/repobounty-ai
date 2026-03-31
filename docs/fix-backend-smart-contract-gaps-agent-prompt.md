## AI Agent Prompt: Fix Backend vs Smart Contract Gaps

You are working in the `RepoBounty AI` repository.

Your job is to harden the current MVP so the live implementation better matches the product trust model described in [backend-vs-smart-contract.md](C:/Users/berek/repobounty-ai/docs/backend-vs-smart-contract.md).

Do not just write a report. Make code changes end to end, run validation where possible, and update docs for any behavior you change.

### Repository context

- Backend: Go API in `backend/`
- Frontend: React + Vite + TypeScript in `frontend/`
- Solana program: Anchor in `program/`
- Live analysis of current gaps: `docs/backend-vs-smart-contract.md`

### Main problems to fix

The current MVP has these important gaps:

1. Campaign creation does not prove the sponsor wallet is controlled by the creator.
2. Claim payout does not prove the destination wallet is controlled by the claimant.
3. Finalize preview is not a frozen snapshot, so reviewed preview can drift from final on-chain allocations.
4. Backend allocation output is not always contract-safe.
5. Contribution collection is not bounded tightly enough to the campaign window.
6. Chain/store reconciliation is inconsistent, especially around claims and auto-finalize.

### Priority order

Implement in this order unless repo constraints force a different sequence:

1. Wallet proof for create and claim
2. Frozen allocation snapshot between preview and finalize
3. Contract-safe allocation normalization before on-chain finalize
4. Tighter campaign contribution time window
5. Better chain/store reconciliation
6. Docs cleanup for changed behavior

### Required outcomes

#### 1. Add sponsor wallet proof during campaign creation

Change the create flow so the backend no longer trusts a bare `sponsor_wallet` string.

Add a signed-message proof flow using the connected wallet in the frontend and backend verification in Go.

Minimum acceptable behavior:

- Frontend requests a backend-issued challenge for campaign creation.
- Challenge must include enough context to prevent replay and cross-use.
- Frontend signs the challenge with the connected sponsor wallet.
- Backend verifies the signature and confirms the signed wallet matches `sponsor_wallet`.
- Challenge must expire and be single-use.

Relevant files:

- `frontend/src/pages/CreateCampaign.tsx`
- `frontend/src/api/client.ts`
- `backend/internal/http/router.go`
- `backend/internal/http/handlers.go`
- `backend/internal/models/models.go`
- any new backend challenge storage/helper files you need

#### 2. Add claim wallet proof during reward claim

Change the claim flow so the backend no longer trusts a bare `wallet_address` string in the request.

Minimum acceptable behavior:

- Frontend requests a backend-issued claim challenge tied to campaign, contributor GitHub username, and wallet address.
- Frontend signs the challenge with the currently connected wallet.
- Backend verifies the signature before sending the on-chain claim transaction.
- Challenge must expire and be single-use.
- Claim should still require GitHub auth and matching allocation ownership.

Relevant files:

- `frontend/src/pages/CampaignDetails.tsx`
- `frontend/src/api/client.ts`
- `backend/internal/http/router.go`
- `backend/internal/http/handlers.go`
- `backend/internal/models/models.go`

#### 3. Freeze preview into a persisted finalize snapshot

Preview must stop being advisory-only.

Minimum acceptable behavior:

- Preview calculation result is persisted as an explicit snapshot.
- Manual finalize uses the saved snapshot instead of recomputing from live GitHub data.
- Snapshot should be invalidated or versioned if campaign inputs or campaign state make it stale.
- Auto-finalize should either:
  - use a stored approved snapshot when one exists, or
  - create and persist a deterministic snapshot once before finalizing.
- Store enough metadata to explain which contributor set, mode, and time window were used.

Try to preserve MVP simplicity. SQLite-backed persistence is fine.

Relevant files:

- `backend/internal/http/handlers.go`
- `backend/internal/http/worker.go`
- `backend/internal/store/sqlite.go`
- `backend/internal/models/models.go`

#### 4. Make backend allocation output contract-safe before finalize

Before sending allocations on-chain, enforce the contract’s limits in the backend.

Minimum acceptable behavior:

- Max 10 contributors
- No duplicate contributors
- Percentages sum to exactly `10000`
- No zero-lamport allocations after integer division
- AI output contributors must be a subset of the fetched contributor dataset
- Final allocation list should be stable and deterministic after normalization

If the input cannot be safely normalized, return a clear API error instead of letting the transaction fail on-chain.

Relevant files:

- `backend/internal/ai/allocator.go`
- `backend/internal/http/handlers.go`
- any helper or test files you add

#### 5. Tighten the campaign contribution window

The current backend uses repo-wide contributor stats and PR filtering that only has a lower bound.

Improve this so allocation data is bounded to the campaign window as much as GitHub APIs reasonably allow in this MVP.

Minimum acceptable behavior:

- PR-based data must exclude merges after the campaign deadline.
- If contributor metrics cannot be perfectly time-bounded with the current GitHub endpoints, make the chosen approximation explicit in code comments and docs.
- Prefer deterministic, explainable behavior over pretending the data is more precise than it is.

Relevant files:

- `backend/internal/http/handlers.go`
- `backend/internal/github/client.go`
- docs you update to explain the rule

#### 6. Improve chain/store reconciliation

Reduce stale or destructive store updates.

Minimum acceptable behavior:

- `/api/auth/claims` should reflect merged chain state, not only raw store state.
- Auto-finalize must preserve store-only fields such as `owner_github_username` and reasoning/snapshot metadata.
- Post-chain persistence failures should be handled consistently across create, finalize, and claim.
- Avoid leaving manually managed campaigns orphaned if chain write succeeds but store write fails.

Relevant files:

- `backend/internal/http/handlers.go`
- `backend/internal/http/worker.go`
- `backend/internal/store/sqlite.go`
- `backend/internal/solana/client.go`

### Constraints

- Keep the current MVP architecture. Do not redesign the whole app into a wallet-first protocol.
- Do not remove the backend authority model.
- Prefer minimal schema changes that fit SQLite cleanly.
- Preserve existing happy-path UX where possible.
- Keep API error messages generic for clients and detailed only in logs.
- Respect existing code style from `AGENTS.md`.

### Strongly preferred implementation details

- Use wallet-signed message verification rather than trying to move create/claim transaction signing fully client-side.
- Reuse a generic challenge mechanism for both create and claim if that keeps the code cleaner.
- Add small, focused helpers for:
  - challenge creation and verification
  - allocation normalization
  - snapshot persistence/loading
  - store merge behavior

### Tests and validation

Add or update tests where practical.

At minimum:

- Go tests for challenge verification logic
- Go tests for allocation normalization and edge cases
- Go tests for snapshot persistence behavior
- Frontend build and lint should pass
- Backend tests should pass

Run the most relevant commands you can, such as:

```bash
cd backend
go test ./...

cd ../frontend
npm run build
npm run lint
```

If Solana program changes become necessary, explain why before making them and keep them minimal. Prefer fixing the trust and synchronization gaps in the backend/frontend first.

### Definition of done

You are done when:

- Campaign creation requires sponsor wallet proof.
- Claim requires destination wallet proof.
- Finalize uses a persisted snapshot instead of recomputing live preview data.
- Backend allocation output cannot exceed contract limits.
- Campaign contribution window is tightened and documented.
- Claims endpoint and auto-finalize no longer drift badly from chain/store mismatches.
- Docs reflect the new live behavior.

### Deliverables

When finished, provide:

1. A short summary of what changed
2. The list of files changed
3. Any schema or migration notes
4. Validation commands run and results
5. Any remaining known limitations

Start by reading:

- `docs/backend-vs-smart-contract.md`
- `backend/internal/http/handlers.go`
- `backend/internal/http/worker.go`
- `backend/internal/ai/allocator.go`
- `backend/internal/github/client.go`
- `backend/internal/store/sqlite.go`
- `frontend/src/pages/CreateCampaign.tsx`
- `frontend/src/pages/CampaignDetails.tsx`
- `frontend/src/api/client.ts`
