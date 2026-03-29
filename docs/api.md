# API Reference

Base URL: `http://localhost:8080/api`

All responses are JSON. Errors return `{"error": "message", "details": "optional"}`.

This file documents the currently implemented HTTP API. The agreed next contract revision adds sponsor-owned escrow and GitHub-based claim/release, but those claim endpoints are not implemented yet.

---

## Health

### `GET /health`

```json
{"status": "ok"}
```

---

## Campaigns

### `GET /campaigns`

List all campaigns, sorted by creation date (newest first).

**Response** `200 OK`:
```json
[
  {
    "campaign_id": "a1b2c3d4",
    "repo": "anthropics/claude-code",
    "pool_amount": 1000000000,
    "deadline": "2025-04-01T00:00:00Z",
    "state": "created",
    "authority": "7xKX...",
    "allocations": [],
    "created_at": "2025-03-28T12:00:00Z",
    "tx_signature": "5K7n..."
  }
]
```

---

### `POST /campaigns`

Create a new campaign. Calls `create_campaign` on Solana.

**Request body:**
```json
{
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "deadline": "2025-04-01T00:00:00Z",
  "wallet_address": "7xKX..."
}
```

| Field | Type | Validation |
|-------|------|-----------|
| `repo` | string | Must match `owner/repo` format |
| `pool_amount` | uint64 | Must be > 0 (in lamports) |
| `deadline` | string | RFC3339 format, must be in the future |
| `wallet_address` | string | Sponsor's Solana public key |

**Response** `201 Created`:
```json
{
  "campaign_id": "a1b2c3d4",
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "deadline": "2025-04-01T00:00:00Z",
  "state": "created",
  "tx_signature": "5K7n..."
}
```

**Errors:**
- `400` — Invalid repo format, missing fields, deadline in the past
- `500` — Solana transaction failed

---

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

---

### `POST /campaigns/{id}/finalize-preview`

Fetch GitHub contributor data, run AI allocation, return preview **without** writing to Solana.

**Response** `200 OK`:
```json
{
  "campaign_id": "a1b2c3d4",
  "repo": "owner/repo",
  "contributors": [
    {
      "username": "alice",
      "commits": 47,
      "pull_requests": 12,
      "reviews": 8,
      "lines_added": 3200,
      "lines_deleted": 980
    },
    {
      "username": "bob",
      "commits": 31,
      "pull_requests": 8,
      "reviews": 15,
      "lines_added": 2100,
      "lines_deleted": 650
    }
  ],
  "allocations": [
    {
      "contributor": "alice",
      "percentage": 5000,
      "amount": 500000000,
      "reasoning": "Highest commit count and active PR contributions"
    },
    {
      "contributor": "bob",
      "percentage": 5000,
      "amount": 500000000,
      "reasoning": "Strong review activity and consistent contributions"
    }
  ],
  "ai_model": "gpt-4o-mini"
}
```

**Notes:**
- Percentages are in basis points (10000 = 100%)
- Amounts are in lamports
- `ai_model` is `"deterministic-fallback"` when no OpenAI key is configured

**Errors:**
- `404` — Campaign not found
- `409` — Campaign already finalized
- `502` — GitHub API failure

---

### `POST /campaigns/{id}/finalize`

Execute full finalization: GitHub fetch + AI allocation + Solana `finalize_campaign` transaction.

This is **irreversible**. The campaign state changes to `finalized` on-chain.

In the agreed target architecture, this call is expected to be authorized by a dedicated backend finalizer key after the deadline, while campaign ownership remains with the sponsor wallet.

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

**Errors:**
- `404` — Campaign not found
- `409` — Campaign already finalized
- `500` — AI or Solana transaction failure
- `502` — GitHub API failure

---

## Data Types

### Allocation percentages

All percentages are expressed in **basis points** (bps):
- `10000` = 100%
- `5000` = 50%
- `100` = 1%

Allocations for a campaign always sum to exactly `10000`.

### Amounts

All monetary amounts are in **lamports** (1 SOL = 1,000,000,000 lamports).

### Timestamps

All timestamps are in **RFC3339** format (e.g. `2025-04-01T00:00:00Z`).

### Campaign states

| State | Description |
|-------|-------------|
| `created` | Campaign exists on-chain, waiting for deadline |
| `finalized` | AI allocation stored on-chain, irreversible |

---

## Planned API Extensions

The next escrow-and-claim revision is expected to add endpoints similar to:

- GitHub OAuth endpoints for contributor login
- wallet binding endpoints for authenticated GitHub users
- claim listing endpoints for pending rewards
- claim or release endpoints that trigger payout from escrow

These endpoints are architectural intent only and do not exist in the current backend.
