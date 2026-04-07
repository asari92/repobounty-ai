# Backend vs Smart Contract: Current Live Behavior

## Summary

Enshor is still a backend-authority MVP, but the live implementation now closes the biggest trust gaps that existed in the first hackathon version:

- Campaign creation requires a backend-issued, single-use, expiring wallet challenge signed by the sponsor wallet.
- Claims require a backend-issued, single-use, expiring wallet challenge signed by the destination wallet.
- Finalize preview now persists a frozen snapshot, and manual finalize uses that stored snapshot instead of recomputing live data.
- Auto-finalize reuses an approved snapshot when one exists. Otherwise it creates one deterministic snapshot once, persists it, and finalizes from that.
- Backend allocations are normalized before on-chain finalize so they stay within the contract's limits.
- Campaign contribution data is now bounded to the campaign window using merged PRs with `merged_at` inside `[created_at, deadline]`.
- `/api/auth/claims` now reflects merged chain + store state instead of raw store state only.

The backend is still the trust anchor for GitHub identity, snapshot approval, and claim authorization. The contract still remains the source of truth for escrow, final allocations, claimed flags, and total claimed.

## Create Campaign

### What happens now

1. The frontend asks the backend for a create challenge at `POST /api/campaigns/create-challenge`.
2. The backend creates a signed-message challenge bound to:
   - GitHub username
   - repo
   - pool amount
   - deadline
   - sponsor wallet
   - challenge ID
   - expiry timestamp
3. The connected sponsor wallet signs that exact message in the browser.
4. The frontend submits the normal create request plus `challenge_id` and `signature`.
5. The backend verifies:
   - the challenge exists
   - the challenge is unused
   - the challenge has not expired
   - the signed wallet matches `sponsor_wallet`
   - the stored challenge payload exactly matches the create request
6. The backend marks the challenge used before the on-chain create call.

### Trust impact

The backend no longer accepts an arbitrary `sponsor_wallet` string without proof that the creator controls that wallet.

### Persistence behavior

The campaign row is written to the store before the on-chain create call so the GitHub owner record exists even if the later store sync step fails. If the chain call fails, the pre-created store row is rolled back.

## Funding

Funding is unchanged in architecture:

- the backend builds the funding transaction
- the sponsor wallet signs it in the browser
- the contract enforces that the signer is the stored sponsor

The create-time signed challenge now complements this by proving the same wallet was controlled when the campaign was created.

## Preview and Finalize

### What changed

Preview is no longer advisory-only.

`POST /api/campaigns/{id}/finalize-preview` now:

- fetches campaign-window contribution data
- calculates allocations
- normalizes allocations to contract-safe output
- persists a versioned finalize snapshot
- marks that snapshot approved by the campaign owner
- returns the snapshot metadata to the frontend

`POST /api/campaigns/{id}/finalize` now:

- requires the owner-approved stored snapshot
- refuses to recompute live data at finalize time
- uses the saved snapshot allocations for the on-chain finalize call

### Snapshot metadata stored

Each snapshot stores:

- version
- allocation mode
- contributor set
- normalized allocations
- campaign contribution window start/end
- contributor source label
- contributor approximation notes
- approval metadata
- an input hash used to detect stale snapshots

### Auto-finalize behavior

Auto-finalize now follows this order:

1. Use the latest approved snapshot if one exists and still matches the funded campaign inputs.
2. Otherwise, reuse the latest stored deterministic snapshot if one already exists for retries.
3. Otherwise, create one deterministic snapshot, persist it, and finalize from that frozen snapshot.

This prevents preview/final drift and avoids recomputing live GitHub data on every worker retry.

## Allocation Safety

Before any finalize call reaches Solana, the backend now enforces:

- at most 10 contributors
- no duplicate contributors
- percentages sum to exactly `10000`
- every allocation rounds to a non-zero lamport amount
- AI output contributors must exist in the fetched campaign contributor dataset
- deterministic ordering of the final allocation list

If the AI output is malformed or unsafe, the backend falls back to deterministic allocation. If even deterministic output cannot be made contract-safe, the API fails before sending the transaction on-chain.

## Campaign Contribution Window

### Current rule

The backend now builds campaign contribution data from merged pull requests whose `merged_at` timestamps fall within:

- `campaign.created_at`
- `campaign.deadline`

PR diffs used for code-impact scoring are fetched only from that bounded set, so merges after the deadline no longer influence preview or finalize.

### Remaining approximation

GitHub does not provide a clean campaign-window version of the repository contributors summary endpoint. To keep the MVP deterministic and explainable, the backend now approximates campaign-scoped contributors from merged PRs inside the campaign window.

That means the live system still does **not** fully capture:

- direct pushes that never went through PRs
- review-only activity outside the merged PR set

The backend now says this explicitly in code and in snapshot metadata instead of implying stronger precision than the APIs support.

## Claims

### What happens now

1. The contributor must still be authenticated with GitHub.
2. The frontend asks the backend for a claim challenge at `POST /api/campaigns/{id}/claim-challenge`.
3. The backend binds the challenge to:
   - GitHub username
   - campaign ID
   - contributor GitHub username
   - destination wallet
   - challenge ID
   - expiry timestamp
4. The connected destination wallet signs the challenge message.
5. The claim request includes `challenge_id` and `signature`.
6. The backend verifies the challenge, verifies the signature, ensures the request matches the stored challenge payload, then submits the on-chain claim transaction.

### Trust impact

The backend no longer trusts a bare `wallet_address` string for payout destination.

The contract still trusts the backend authority signer for the claim instruction itself, but the backend now requires cryptographic proof that the claimant controls the wallet receiving the payout.

## Chain / Store Reconciliation

### Claims endpoint

`GET /api/auth/claims` now uses merged chain + store campaign state, not store-only snapshots. This reduces stale profile data after finalize or claim events that have already succeeded on-chain.

### Auto-finalize merge behavior

Auto-finalize now merges stored campaigns with on-chain campaigns before updating state, so it preserves store-only metadata such as:

- `owner_github_username`
- reasoning text
- finalize snapshot metadata

### Post-chain persistence handling

Create, finalize, and claim now handle post-chain persistence failures consistently:

- if the chain write succeeds but store persistence fails, the API returns a success-like response instead of pretending the chain write failed
- when possible, the backend recreates missing local rows before falling back to an accepted response
- create now pre-stores owner metadata before the chain call so campaigns are much less likely to become manually orphaned

## Backend vs Contract Responsibilities

### Backend

- GitHub auth and ownership checks
- wallet challenge issuance and verification
- campaign-window GitHub data collection
- AI / deterministic allocation
- snapshot persistence and approval
- auto-finalize orchestration
- claim authorization

### Smart contract

- escrow custody
- campaign state transitions
- final allocation storage
- claimed flags
- payout transfers
- completion bookkeeping

## Remaining MVP Limitations

The live system is meaningfully safer than the original MVP, but these limits still remain:

- The backend authority still creates campaigns, finalizes campaigns, and submits claim transactions.
- Snapshot approval is off-chain and tied to GitHub ownership, not on-chain governance.
- Campaign-window contributor metrics are still an approximation based on merged PRs, not a perfect replay of all repository activity.
- If the chain write succeeds and every persistence retry fails, the backend can still end up temporarily out of sync until the next reconciliation pass.

## Bottom Line

The current split is still backend-trusted, but it is now much closer to the intended product trust model:

- create and claim both require wallet proof
- preview and finalize now share a frozen snapshot
- backend output is normalized to contract-safe allocations
- contribution data stops at the campaign deadline
- reconciliation between chain and store is materially better than before

For this MVP, the remaining trust assumption is no longer "the backend can silently choose any wallet or silently change the reviewed result later." It is now primarily "the backend remains the policy and orchestration layer, while the contract remains the settlement and custody layer."
