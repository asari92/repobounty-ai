# API Reference

Base URL: `http://localhost:8080/api`

All responses are JSON. Errors return `{"error": "message", "details": "optional"}`.

This file documents the current backend API for Enshor, including wallet-proof and GitHub-authenticated flows.

---

## Health

### `GET /health`

Check service readiness.

**Response** `200 OK`:
```json
{"status": "ok"}
```

---

## Authentication

### `GET /auth/github/url`

Get the GitHub OAuth authorization URL for frontend login.

**Response** `200 OK`:
```json
{
  "url": "https://github.com/login/oauth/authorize?client_id=...&state=..."
}
```

### `POST /auth/github/callback`

Exchange the GitHub OAuth code for a JWT session token.

**Request body:**
```json
{
  "code": "...",
  "state": "..."
}
```

**Response** `200 OK`:
```json
{
  "token": "eyJhb...",
  "user": {
    "github_username": "alice",
    "github_id": 123456,
    "avatar_url": "https://...",
    "wallet_address": "",
    "created_at": "2025-04-01T12:00:00Z"
  }
}
```

### `GET /auth/me`

Return the authenticated GitHub user profile.

Requires `Authorization: Bearer <jwt>`.

**Response** `200 OK`:
```json
{
  "github_username": "alice",
  "github_id": 123456,
  "avatar_url": "https://...",
  "wallet_address": "7xKX...",
  "created_at": "2025-04-01T12:00:00Z"
}
```

### `POST /auth/wallet/link`

Link a Solana wallet to the authenticated GitHub user.

Requires `Authorization: Bearer <jwt>`.

**Request body:**
```json
{
  "wallet_address": "7xKX..."
}
```

**Response** `200 OK` — returns the updated user profile.

### `GET /auth/claims`

List claimable allocations for the authenticated user.

Requires `Authorization: Bearer <jwt>`.

**Response** `200 OK` — array of claim items.

---

## GitHub

### `GET /github/search?query=...`

Search GitHub repositories or users using the backend GitHub integration.

Authentication is optional.

**Response** `200 OK` — search results.

---

## Campaigns

### `GET /campaigns`

List campaigns, sorted by creation date (newest first).

Authentication is optional.

**Response** `200 OK` — array of campaigns.

### `GET /campaigns/{id}`

Get a single campaign by ID.

**Response** `200 OK`:
```json
{
  "campaign_id": "a1b2c3d4",
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "deadline": "2025-04-01T00:00:00Z",
  "state": "finalized",
  "authority": "7xKX...",
  "allocations": [
    {
      "contributor": "alice",
      "percentage": 5000,
      "amount": 500000000,
      "reasoning": "Highest commit activity with 47 commits and 12 PRs"
    }
  ],
  "created_at": "2025-03-28T12:00:00Z",
  "finalized_at": "2025-04-01T12:30:00Z",
  "tx_signature": "4Rm9..."
}
```

**Errors:**
- `404` — Campaign not found

### `POST /campaigns`

Create a new campaign and build the Solana create transaction.

**Request body:**
```json
{
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "deadline": "2025-04-01T00:00:00Z",
  "sponsor_wallet": "7xKX..."
}
```

**Response** `201 Created`:
```json
{
  "campaign_id": "a1b2c3d4",
  "campaign_pda": "...",
  "escrow_pda": "...",
  "vault_address": "...",
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "deadline": "2025-04-01T00:00:00Z",
  "state": "created",
  "tx_signature": "...",
  "unsigned_tx": "BASE64_TX"
}
```

**Errors:**
- `400` — validation failed, invalid wallet address, deadline too soon
- `500` — internal or Solana transaction preparation error

### `POST /campaigns/{id}/create-confirm`

Confirm campaign creation after signing the prepared transaction.

**Request body:**
```json
{
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "deadline": "2025-04-01T00:00:00Z",
  "sponsor_wallet": "7xKX...",
  "tx_signature": "5K7n..."
}
```

**Response** `201 Created` — campaign is stored and created on-chain.

### `POST /campaigns/{id}/fund-tx`

This endpoint is present for backwards compatibility.

**Response** `410 Gone` — campaign funding is deprecated because campaigns are created atomically on-chain.

### `POST /campaigns/{id}/finalize-preview`

Preview allocation results by fetching campaign-window GitHub data and running the AI allocator.

Requires `Authorization: Bearer <jwt>` and campaign ownership.

**Response** `200 OK`:
```json
{
  "campaign_id": "a1b2c3d4",
  "repo": "owner/repo",
  "contributors": [
    {
      "github_username": "alice",
      "commits": 47,
      "pull_requests": 12,
      "reviews": 8,
      "lines_added": 3200,
      "lines_deleted": 980
    }
  ],
  "allocations": [
    {
      "contributor": "alice",
      "percentage": 5000,
      "amount": 500000000,
      "reasoning": "Highest commit count and active PR contributions"
    }
  ],
  "ai_model": "gpt-4o-mini",
  "allocation_mode": "ai",
  "snapshot": {
    "total_contributors": 2,
    "total_commits": 78,
    "total_prs": 20,
    "snapshot_timestamp": "2025-04-01T12:00:00Z"
  }
}
```

**Errors:**
- `401` — Authentication required
- `403` — Not the campaign owner
- `404` — Campaign not found
- `409` — Campaign already finalized

### `POST /campaigns/{id}/finalize`

Finalize allocations on-chain using the stored approved snapshot.

Requires `Authorization: Bearer <jwt>` and campaign ownership.

**Response** `200 OK`:
```json
{
  "campaign_id": "a1b2c3d4",
  "state": "finalized",
  "allocations": [
    {
      "contributor": "alice",
      "percentage": 5000,
      "amount": 500000000,
      "reasoning": "..."
    }
  ],
  "tx_signature": "4Rm9...",
  "solana_explorer_url": "https://explorer.solana.com/tx/4Rm9...?cluster=devnet"
}
```

### `POST /campaigns/{id}/finalize-challenge`

Request a wallet challenge for sponsor wallet finalization.

**Request body:**
```json
{
  "wallet_address": "7xKX..."
}
```

**Response** `201 Created`:
```json
{
  "challenge_id": "...",
  "action": "finalize",
  "wallet_address": "7xKX...",
  "message": "...",
  "expires_at": "2025-04-01T12:05:00Z"
}
```

### `POST /campaigns/{id}/finalize-wallet`

Submit sponsor wallet proof and perform the finalization.

**Request body:**
```json
{
  "wallet_address": "7xKX...",
  "challenge_id": "...",
  "signature": "..."
}
```

**Response** `200 OK` — finalization result.

### `POST /campaigns/{id}/claim-challenge`

Start a wallet proof challenge for a contributor claim.

Requires `Authorization: Bearer <jwt>`.

**Request body:**
```json
{
  "contributor_github": "alice",
  "wallet_address": "7xKX..."
}
```

**Response** `201 Created`:
```json
{
  "challenge_id": "...",
  "action": "claim",
  "wallet_address": "7xKX...",
  "message": "...",
  "expires_at": "2025-04-01T12:05:00Z"
}
```

### `POST /campaigns/{id}/claim`

Submit the claim wallet proof and build the on-chain claim transaction.

Requires `Authorization: Bearer <jwt>`.

**Request body:**
```json
{
  "github_username": "alice",
  "contributor_github": "alice",
  "wallet_address": "7xKX...",
  "challenge_id": "...",
  "signature": "..."
}
```

**Response** `200 OK`:
```json
{
  "partial_tx": "BASE64_TX"
}
```

### `POST /campaigns/{id}/claim-confirm`

Confirm that the claim transaction was finalized on-chain and update store state.

Requires `Authorization: Bearer <jwt>`.

**Request body:**
```json
{
  "contributor_github": "alice",
  "wallet_address": "7xKX...",
  "tx_signature": "..."
}
```

**Response** `200 OK` — claim confirmation result.

### `POST /campaigns/{id}/refund`

Build a refund transaction after the claim deadline has passed.

**Request body:**
```json
{
  "sponsor_wallet": "7xKX..."
}
```

**Response** `200 OK`:
```json
{
  "partial_tx": "BASE64_TX"
}
```

### `POST /campaigns/{id}/refund-confirm`

Confirm the refund transaction on-chain.

**Request body:**
```json
{
  "sponsor_wallet": "7xKX...",
  "tx_signature": "..."
}
```

---

## Data Types

### Allocation percentages

All percentages are expressed in **basis points** (bps):
- `10000` = 100%
- `5000` = 50%
- `100` = 1%

Allocations for a campaign always sum to exactly `10000`.

### Amounts

Amounts are expressed in **lamports**.

---

## Notes

- Finalization and claim flows now use wallet-proof challenges to verify wallet control.
- Some endpoints require authenticated GitHub users.
- `POST /campaigns/{id}/fund-tx` is deprecated for the current atomic campaign creation flow.
