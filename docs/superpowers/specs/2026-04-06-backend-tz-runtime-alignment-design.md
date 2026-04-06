# Backend Runtime TЗ Alignment Design

**Problem:** The deployed V2 Solana contract and the current MVP TЗ define a runtime flow where campaigns are created atomically on-chain, finalized automatically by the service after the deadline, claimed by GitHub-authenticated contributors with wallet signatures, and refunded by the sponsor only after `claim_deadline_at`. The Go backend is closer to this model than before, but parts of the runtime behavior are still split between old manual flows, partial V1 assumptions, and service DB behavior that should no longer be authoritative.

**Decision:** Align the backend runtime around the deployed V2 contract and the current MVP TЗ while keeping the existing HTTP surface as stable as practical. Solana remains the source of truth for campaign existence and on-chain state; SQLite remains an enrichment and service-state store. Auto-finalization becomes the primary product path with a default 1-minute worker interval, while manual finalize endpoints remain available only as debug/fallback tools.

## Scope

This design covers only runtime MVP backend behavior:

- campaign reads: `GET /api/campaigns/`, `GET /api/campaigns/{id}`
- sponsor create flow: `create-challenge`, `create`, `create-confirm`
- automatic finalization after deadline
- contributor claim flow
- sponsor refund flow

This design explicitly does **not** cover:

- admin APIs for `init/update config/pause/unpause`
- recovery tooling for partially finalized campaigns
- data cleanup or migration beyond what is needed for runtime correctness
- broad frontend redesign

## Source Of Truth Model

When Solana is configured:

- campaign existence comes from Solana only
- campaign status/state comes from Solana only
- reward amounts, claimed totals, deadlines, and claim state come from Solana only
- SQLite data is used only to enrich on-chain campaigns with service-managed fields

Service DB data remains useful for:

- `owner_github_username`
- allocation reasoning
- finalize snapshots
- wallet-proof challenges
- service logs, comments, and other non-authoritative metadata

If a DB row exists for a `campaign_id` that does not exist on-chain, public campaign APIs must treat that campaign as non-existent.

## Runtime Flows

### Sponsor Create Flow

The sponsor flow is wallet-only. GitHub authentication is not required.

Flow:

1. Sponsor requests `create-challenge`.
2. Backend verifies repo metadata and wallet-proof inputs, then issues a wallet challenge.
3. Sponsor signs the wallet challenge.
4. Backend verifies the signed wallet proof and builds an unsigned `create_campaign_with_deposit` transaction.
5. Sponsor signs and submits the Solana transaction in the wallet.
6. Backend confirms on-chain creation in `create-confirm`.
7. Only after successful on-chain confirmation does the backend persist/update DB enrichment for the campaign.

Implications:

- there is no valid runtime state where a campaign exists in the product but not on-chain
- `fund-tx` is no longer part of the normal sponsor flow and should not remain a working alternate funding path
- backend validation must fail early for cases the contract would reject, including minimum campaign amount and insufficient sponsor balance

### Campaign Reads

`GET /api/campaigns/`:

- lists on-chain campaigns
- enriches matching campaigns from the DB
- does not append DB-only orphan campaigns

`GET /api/campaigns/{id}`:

- loads from Solana first
- returns `404` if the campaign does not exist on-chain
- enriches the on-chain result from the DB if a matching record exists

### Auto-Finalize Flow

Auto-finalization is the primary product path for MVP.

Behavior:

- a background worker polls on a configurable interval
- default interval is `1 minute`
- the worker finds campaigns whose on-chain state is still active/funded and whose deadline has passed
- the worker computes allocations using MVP whole-repository analysis
- the worker persists a finalize snapshot
- the worker sends the on-chain finalize transaction
- after on-chain success, the worker updates service-side enrichment

Manual endpoints:

- `POST /api/campaigns/{id}/finalize-preview`
- `POST /api/campaigns/{id}/finalize`

remain in the backend as debug/fallback tools. They are not the primary runtime path and should not drive product expectations.

### Claim Flow

Claim remains GitHub-authenticated and wallet-signed.

Flow:

1. Contributor authenticates with GitHub.
2. Contributor requests available claims from the backend.
3. Contributor connects a wallet.
4. Backend issues a claim challenge tied to the authenticated GitHub identity and the selected wallet.
5. Contributor signs the wallet challenge.
6. Backend verifies the wallet proof and builds a partial claim transaction.
7. Contributor signs and submits the claim transaction with the wallet.
8. Backend confirms the on-chain claim result and updates service-side enrichment.

Rules:

- the contributor must be authenticated with GitHub
- the contributor may only claim for their own GitHub identity
- the backend must build claim transactions against the V2 contract interface
- backend claim state shown in the product must follow on-chain truth, not local assumptions

### Refund Flow

Refund is sponsor-initiated and sponsor-signed.

Flow:

1. Sponsor requests refund for a campaign.
2. Backend verifies that the campaign exists on-chain and that `claim_deadline_at` has passed.
3. Backend builds an unsigned refund transaction for the sponsor.
4. Sponsor signs and submits the refund transaction with the wallet.
5. Backend confirms the on-chain refund result and updates service-side enrichment if needed.

Rules:

- refund is unavailable before `claim_deadline_at`
- refund must be derived from on-chain campaign state, not DB-only state
- refund must not be executed by the service wallet on behalf of the sponsor

## API and Service Changes

### Keep Stable

Keep the following endpoints, updating their behavior as needed:

- `GET /api/campaigns/`
- `GET /api/campaigns/{id}`
- `POST /api/campaigns/create-challenge`
- `POST /api/campaigns/`
- `POST /api/campaigns/{id}/create-confirm`
- `POST /api/campaigns/{id}/finalize-preview`
- `POST /api/campaigns/{id}/finalize`
- `POST /api/campaigns/{id}/claim-challenge`
- `POST /api/campaigns/{id}/claim`
- `POST /api/campaigns/{id}/claim-confirm`

### Change or Retire

- `POST /api/campaigns/{id}/fund-tx` should no longer behave as a supported product flow because campaign creation is atomic on-chain. It can be retired or downgraded to an explicit unsupported/deprecated path.
- add a runtime refund endpoint backed by sponsor-signed refund transactions
- auto-finalize interval should be configurable, with a default of `1 minute`

## Data Ownership

### Solana-Owned Fields

These fields must be treated as Solana-owned:

- campaign existence
- sponsor
- repo ID association
- campaign state/status
- pool amount / reward amount
- finalized allocations
- claimed totals and claim records
- deadline and claim deadline

### Service-Owned Fields

These fields remain service-owned:

- `owner_github_username`
- reasoning text and preview explanations
- snapshot provenance
- wallet-proof challenge state
- worker retry bookkeeping
- service-side explorer/comment metadata

## Testing Requirements

The runtime-alignment pass must add or update tests that prove:

- public campaign reads do not expose DB-only orphan campaigns when Solana is configured
- create flow remains wallet-only and only persists after confirmed on-chain creation
- minimum amount and sponsor balance validation fail before wallet submission
- auto-finalize worker uses a 1-minute default interval and finalizes eligible campaigns from on-chain truth
- claim flow uses the V2 transaction builder and requires GitHub auth plus wallet proof
- refund flow rejects early refunds and builds sponsor-signed transactions only after `claim_deadline_at`
- `go test ./...` in `backend` passes after the changes

## Risks and Non-Goals

- Claim and refund are the highest-risk runtime areas because they combine auth, wallet signing, and strict on-chain interfaces.
- Manual finalize endpoints are intentionally retained in this pass for operational safety and are explicitly out of scope for removal here.
- This design does not attempt to clean up historical orphan rows in SQLite. Runtime correctness matters more than data migration in this pass.

## Acceptance Criteria

This runtime-alignment pass is complete when:

- sponsor create flow matches the atomic on-chain campaign model
- public campaign APIs treat Solana as source of truth
- auto-finalize is the default product path and runs every minute by default
- contributor claim flow matches the deployed V2 contract and MVP TЗ trust model
- sponsor refund flow exists and follows on-chain timing rules
- backend runtime no longer depends on deprecated pre-V2 campaign funding assumptions
