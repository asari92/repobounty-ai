# One-Popup Sponsor Create Flow Design

**Problem:** The current sponsor campaign creation flow still uses a wallet-proof message challenge plus a separate transaction signature. That makes Phantom open twice and leaves a two-step UX on the frontend. For MVP, this is unnecessary friction because campaign creation is already anchored by a sponsor-signed on-chain `create_campaign_with_deposit` transaction.

**Goal:** Make sponsor campaign creation a one-button, one-popup flow. Remove sponsor `create-challenge` entirely, keep contributor `claim-challenge` intact, and remove the tracked `backend/api` binary from the repository so build artifacts stop polluting git state.

## Scope

**In scope**
- sponsor campaign create flow in backend and frontend
- removal of sponsor `create-challenge` endpoint usage
- removal of sponsor wallet-proof requirements from create request handling
- keep `create-confirm` as the backend persistence step after confirmed on-chain creation
- git cleanup for tracked `backend/api`

**Out of scope**
- contributor claim flow
- contributor wallet challenge flow
- finalize worker behavior
- refund flow
- admin/config flows

## Desired UX

Sponsor opens the create page, fills repo / amount / deadline, clicks one button, approves one Phantom popup, and lands on the created campaign page.

There is no intermediate “prepared transaction” step and no separate wallet message-signing step.

## Backend Design

### Remove sponsor challenge flow

Delete sponsor-facing `POST /api/campaigns/create-challenge`.

Rationale:
- it is not needed for MVP sponsor create once transaction signing is the proof of wallet control
- it is not part of the desired one-popup UX
- on-chain transaction confirmation is a stronger proof than an off-chain signed message for this flow

This removal applies only to sponsor create. Contributor claim challenge remains unchanged.

### Create campaign request

`POST /api/campaigns/` becomes the only sponsor pre-submit API call.

Request body:
- `repo`
- `pool_amount`
- `deadline`
- `sponsor_wallet`

Removed fields:
- `challenge_id`
- `signature`

Backend responsibilities:
- validate repo / amount / deadline
- validate sponsor wallet format
- resolve GitHub repo metadata
- check sponsor balance against full create cost
- build unsigned `create_campaign_with_deposit` transaction
- return:
  - `campaign_id`
  - `campaign_pda`
  - `escrow_pda` / `vault_address`
  - `unsigned_tx`

The backend does **not** create any local campaign row at this step.

### Create confirm

`POST /api/campaigns/{id}/create-confirm` remains.

It is called only after the frontend submits the signed transaction and gets a real `tx_signature`.

Backend responsibilities:
- reload on-chain campaign by `campaign_id`
- verify sponsor / pool / deadline / repo metadata
- persist off-chain enrichment into SQLite

The campaign exists for product purposes only after `create-confirm` succeeds.

## Frontend Design

### One-screen flow

The create page stays on one screen and one main CTA.

Button behavior:
1. call `POST /api/campaigns/`
2. decode `unsigned_tx`
3. open Phantom once for transaction signing
4. send raw transaction
5. poll / retry `create-confirm`
6. redirect to `/campaign/{id}`

There is no step indicator and no fallback second phase in the normal UX.

### Error handling

Frontend should surface:
- invalid repo / invalid amount / invalid deadline
- insufficient sponsor balance
- wallet missing / wallet does not support transaction signing
- wallet rejected transaction
- create transaction sent but not yet visible on-chain
- create confirmation timed out

The message should clearly distinguish:
- validation failure before Phantom
- rejection inside Phantom
- delayed on-chain confirmation after send

## API Compatibility

This is a breaking change for sponsor create clients:
- old `create-challenge` endpoint is removed
- old `CreateCampaignRequest` shape is simplified

This is acceptable for MVP because frontend and backend are versioned together in this repository.

## Git Cleanup

`backend/api` is a build artifact and must not remain tracked.

Required cleanup:
- remove tracked `backend/api` from git
- add `backend/api` to `.gitignore`

Rationale:
- prevents noisy diffs after local builds
- keeps repository source-only
- avoids accidental commits of large binaries

## Testing

### Backend
- integration test that `POST /api/campaigns/` no longer requires sponsor challenge fields
- route-level test that sponsor `create-challenge` is gone
- existing create-confirm tests continue to pass

### Frontend
- build must pass
- manual smoke:
  - fill form
  - click create once
  - one Phantom popup
  - successful redirect to campaign page

## Risks

Primary risk:
- removing sponsor challenge changes API shape, so frontend and backend must land together

Acceptable MVP tradeoff:
- sponsor wallet control is proven by the successful signed on-chain transaction instead of a separate off-chain signed message

## Success Criteria

- sponsor create no longer calls `create-challenge`
- sponsor create no longer signs a message
- Phantom opens only once during sponsor create
- campaign appears only after successful `create-confirm`
- `backend/api` is no longer tracked and no longer shows up after normal builds
