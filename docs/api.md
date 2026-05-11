# API Reference

Base URL: `http://localhost:8080/api` for local backend access.

The frontend calls the API through the relative `/api` path. In Docker, Nginx proxies frontend `/api` requests to the backend service.

All request and response bodies are JSON unless stated otherwise. Errors use this shape:

```json
{
  "error": "message",
  "details": "optional details",
  "code": "OPTIONAL_MACHINE_CODE"
}
```

Some endpoints require `Authorization: Bearer <jwt>`. JWTs are issued by the GitHub OAuth callback and stored by the frontend in `localStorage`.

## Runtime Behavior

Solana-dependent endpoints require both `SERVICE_PRIVATE_KEY` and a non-placeholder `PROGRAM_ID`. If either value is missing, the backend still starts, but create, finalize, claim, and refund transaction endpoints return `503 Service Unavailable` instead of returning fake transactions.

AI allocation uses OpenRouter when `OPENROUTER_API_KEY` is configured. Without it, or when model calls fail, the backend uses deterministic fallback allocation.

The MVP allocation data path analyzes the full available repository history, not only the campaign window.

## Health

### `GET /health`

Returns backend runtime readiness flags.

Response `200 OK`:

```json
{
  "status": "ok",
  "solana": true,
  "github": true,
  "ai_model": "nvidia/nemotron-3-super-120b-a12b:free",
  "store": true
}
```

## Authentication

### `GET /auth/github/url`

Creates a short-lived OAuth state and returns the GitHub authorization URL.

Response `200 OK`:

```json
{
  "auth_url": "https://github.com/login/oauth/authorize?...",
  "state": "..."
}
```

### `POST /auth/github/callback`

Exchanges a GitHub OAuth code for an Enshor JWT session.

Request body:

```json
{
  "code": "github-oauth-code",
  "state": "oauth-state-from-auth-url"
}
```

Response `200 OK`:

```json
{
  "token": "eyJhbGciOi...",
  "user": {
    "github_username": "alice",
    "github_id": 123456,
    "avatar_url": "https://avatars.githubusercontent.com/u/123456?v=4",
    "wallet_address": "",
    "created_at": "2026-05-11T12:00:00Z"
  }
}
```

Errors:

- `400` when `code` or `state` is missing, invalid, expired, or already used.
- `500` when GitHub OAuth exchange or session creation fails.

### `GET /auth/me`

Returns the authenticated GitHub user.

Requires `Authorization: Bearer <jwt>`.

Response `200 OK`:

```json
{
  "github_username": "alice",
  "github_id": 123456,
  "email": "",
  "avatar_url": "https://avatars.githubusercontent.com/u/123456?v=4",
  "wallet_address": "7xKX...",
  "created_at": "2026-05-11T12:00:00Z"
}
```

### `POST /auth/wallet/link`

Links a wallet to the authenticated user after wallet-proof verification.

Requires `Authorization: Bearer <jwt>`.

Current backend request body:

```json
{
  "wallet_address": "7xKX...",
  "challenge_id": "...",
  "signature": "..."
}
```

Response `200 OK`: updated user object.

Note: the backend expects a wallet-proof challenge for link operations. The campaign claim and finalize flows expose challenge endpoints; a standalone link-challenge route is not currently exposed in the router.

### `GET /auth/claims`

Lists finalized allocations claimable by the authenticated GitHub user.

Requires `Authorization: Bearer <jwt>`.

Response `200 OK`:

```json
[
  {
    "campaign_id": "123456789",
    "repo": "owner/repo",
    "contributor": "alice",
    "percentage": 5000,
    "amount": 500000000,
    "amount_sol": "0.5000",
    "claimed": false,
    "state": "finalized"
  }
]
```

### `GET /auth/my-campaigns?wallet=...`

Lists campaigns sponsored by the authenticated user wallet or by the optional `wallet` query parameter.

Authentication is optional. If no authenticated user wallet and no query wallet are available, the endpoint returns an empty array.

Response `200 OK`:

```json
[
  {
    "campaign_id": "123456789",
    "campaign_pda": "...",
    "repo": "owner/repo",
    "pool_amount": 1000000000,
    "state": "finalized",
    "status": "finalized",
    "sponsor": "7xKX...",
    "authority": "7xKX...",
    "owner_github_username": "alice",
    "allocations": [],
    "created_at": "2026-05-11T12:00:00Z",
    "deadline": "2026-05-11T12:10:00Z",
    "can_refund": false
  }
]
```

## GitHub

### `GET /github/search?q=...`

Searches GitHub users or repositories.

Query behavior:

- `q=alice` searches users.
- `q=owner/repo` searches repositories under `owner` matching `repo`.

Authentication is optional.

Response `200 OK`: array of user or repository search results.

Errors:

- `400` when `q` is missing or has an invalid slash format.
- `500` when GitHub search fails.

## Campaigns

### `GET /campaigns/`

Lists campaigns sorted newest first.

Authentication is optional.

Response `200 OK`: array of campaign objects.

When Solana is configured, the backend merges on-chain campaign state with stored metadata. When Solana is not configured, it returns stored campaigns only.

### `GET /campaigns/{id}`

Returns a campaign by id.

Response `200 OK`:

```json
{
  "campaign_id": "123456789",
  "campaign_pda": "...",
  "escrow_pda": "...",
  "vault_address": "...",
  "github_repo_id": 1234567890,
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "total_reward_amount": 1000000000,
  "deadline": "2026-05-11T12:10:00Z",
  "deadline_at": "2026-05-11T12:10:00Z",
  "claim_deadline_at": "2027-05-11T12:10:00Z",
  "state": "finalized",
  "status": "finalized",
  "sponsor": "7xKX...",
  "authority": "7xKX...",
  "owner_github_username": "alice",
  "allocations": [
    {
      "github_user_id": 123456,
      "github_username": "alice",
      "contributor": "alice",
      "percentage": 10000,
      "amount": 1000000000,
      "reasoning": "Deterministic fallback allocation",
      "claimed": false
    }
  ],
  "created_at": "2026-05-11T12:00:00Z",
  "finalized_at": "2026-05-11T12:12:00Z",
  "tx_signature": "4Rm9...",
  "finalization_status": "finalized"
}
```

Errors:

- `404` when the campaign is not found.
- `500` for internal load errors.

### `POST /campaigns/`

Validates campaign input and returns an unsigned Solana `create_campaign_with_deposit` transaction for the sponsor wallet to sign.

Solana must be configured.

Request body:

```json
{
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "deadline": "2026-05-11T12:10:00Z",
  "sponsor_wallet": "7xKX..."
}
```

Response `200 OK`:

```json
{
  "campaign_id": "123456789",
  "campaign_pda": "...",
  "escrow_pda": "...",
  "vault_address": "...",
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "deadline": "2026-05-11T12:10:00Z",
  "unsigned_tx": "base58-serialized-transaction"
}
```

Validation performed by the backend includes repository existence, minimum campaign amount, deadline lead time, sponsor wallet format, sponsor wallet balance, and create transaction cost estimation.

Errors:

- `400` for invalid input, unavailable public repository, or insufficient wallet balance.
- `502` when GitHub or Solana balance/cost lookup fails.
- `503` when Solana is not configured.
- `500` when campaign id generation or transaction building fails.

### `POST /campaigns/{id}/create-confirm`

Confirms that the sponsor-submitted create transaction is visible on-chain, validates it against the requested repo, pool amount, deadline, sponsor wallet, and GitHub repo id, then stores campaign metadata.

Request body:

```json
{
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "deadline": "2026-05-11T12:10:00Z",
  "sponsor_wallet": "7xKX...",
  "tx_signature": "5K7n..."
}
```

Response `201 Created`: campaign object summary.

Errors:

- `400` for invalid input or on-chain/request mismatch.
- `409` when the on-chain campaign is not confirmed yet.
- `502` when GitHub metadata lookup fails.
- `503` when Solana is not configured.

### `POST /campaigns/{id}/fund-tx`

Deprecated compatibility endpoint.

Response `410 Gone`:

```json
{
  "error": "campaign funding is deprecated; campaigns are created atomically on-chain"
}
```

### `POST /campaigns/{id}/finalize-preview`

Calculates allocations and stores a finalize snapshot.

Requires `Authorization: Bearer <jwt>`. The authenticated GitHub user must be the stored campaign owner.

Response `200 OK`:

```json
{
  "campaign_id": "123456789",
  "repo": "owner/repo",
  "contributors": [
    {
      "github_user_id": 123456,
      "username": "alice",
      "commits": 47,
      "pull_requests": 12,
      "reviews": 0,
      "lines_added": 3200,
      "lines_deleted": 980
    }
  ],
  "allocations": [
    {
      "github_user_id": 123456,
      "github_username": "alice",
      "contributor": "alice",
      "percentage": 10000,
      "amount": 1000000000,
      "reasoning": "Deterministic fallback allocation",
      "claimed": false
    }
  ],
  "ai_model": "deterministic-fallback",
  "allocation_mode": "metrics",
  "snapshot": {
    "version": 1,
    "allocation_mode": "metrics",
    "window_start": "2026-05-11T12:00:00Z",
    "window_end": "2026-05-11T12:10:00Z",
    "contributor_source": "repository_history_mvp",
    "contributor_notes": "MVP analyzes the full available repository history. A future production flow will restrict analysis to activity inside the campaign window.",
    "created_at": "2026-05-11T12:11:00Z",
    "approved_by_github_username": "alice",
    "approved_at": "2026-05-11T12:11:00Z"
  }
}
```

Errors:

- `401` when authentication is missing or invalid.
- `403` when the authenticated user is not the stored campaign owner.
- `404` when the campaign is not found.
- `409` when the campaign is already finalized or the deadline has not been reached.

### `POST /campaigns/{id}/finalize`

Finalizes allocations on-chain using the approved current snapshot.

Requires `Authorization: Bearer <jwt>`. The authenticated GitHub user must be the stored campaign owner. Solana must be configured.

Response `200 OK`:

```json
{
  "campaign_id": "123456789",
  "state": "finalized",
  "allocations": [],
  "tx_signature": "4Rm9...",
  "solana_explorer_url": "https://explorer.solana.com/tx/4Rm9...?cluster=devnet",
  "allocation_mode": "metrics",
  "snapshot": {
    "version": 1,
    "allocation_mode": "metrics",
    "window_start": "2026-05-11T12:00:00Z",
    "window_end": "2026-05-11T12:10:00Z",
    "contributor_source": "repository_history_mvp",
    "created_at": "2026-05-11T12:11:00Z"
  }
}
```

Notes:

- The Solana program supports batched finalization with `has_more`.
- The current backend finalization sender sends one finalization instruction with `has_more = false`; durable multi-batch orchestration is not wired end-to-end.

Errors:

- `401`, `403`, `404`, or `409` for auth, ownership, missing campaign, stale/missing preview, already finalized, or deadline-state errors.
- `503` when Solana is not configured.
- `500` when on-chain finalization fails.

### `POST /campaigns/{id}/finalize-challenge`

Creates a wallet-proof challenge for sponsor-wallet finalization.

Authentication is not required, but the supplied wallet must match the campaign sponsor.

Request body:

```json
{
  "wallet_address": "7xKX..."
}
```

Response `201 Created`:

```json
{
  "challenge_id": "...",
  "action": "finalize",
  "wallet_address": "7xKX...",
  "message": "...",
  "expires_at": "2026-05-11T12:15:00Z"
}
```

### `POST /campaigns/{id}/finalize-wallet`

Submits a sponsor wallet proof and finalizes the campaign. If an approved snapshot exists, it is used; otherwise the backend calculates and stores one before finalizing.

Solana must be configured.

Request body:

```json
{
  "wallet_address": "7xKX...",
  "challenge_id": "...",
  "signature": "base58-signature"
}
```

Response `200 OK`: same shape as `POST /campaigns/{id}/finalize`.

### `POST /campaigns/{id}/claim-challenge`

Creates a wallet-proof challenge for a contributor claim.

Requires `Authorization: Bearer <jwt>`.

Request body:

```json
{
  "contributor_github": "alice",
  "wallet_address": "7xKX..."
}
```

Response `201 Created`:

```json
{
  "challenge_id": "...",
  "action": "claim",
  "wallet_address": "7xKX...",
  "message": "...",
  "expires_at": "2026-05-11T12:15:00Z"
}
```

### `POST /campaigns/{id}/claim`

Verifies the contributor wallet proof and returns a partially signed Solana claim transaction.

Requires `Authorization: Bearer <jwt>`. Solana must be configured.

Current shipped claim path is user-paid: the returned transaction uses the contributor wallet as fee payer and includes the backend service wallet partial signature as `claim_authority`.

Request body:

```json
{
  "contributor_github": "alice",
  "wallet_address": "7xKX...",
  "challenge_id": "...",
  "signature": "base58-signature"
}
```

Response `200 OK`:

```json
{
  "partial_tx": "base58-serialized-transaction"
}
```

Errors can include these machine codes:

- `CLAIM_ALREADY_CLAIMED`
- `CAMPAIGN_NOT_FINALIZED`
- `CLAIM_NOT_YOUR_OWN`
- `CONTRIBUTOR_NOT_FOUND`
- `BAD_REQUEST`

### `POST /campaigns/{id}/claim-confirm`

Confirms that the claim is present on-chain by reading the campaign claim record, then updates the stored campaign allocation display state.

Requires `Authorization: Bearer <jwt>`. Solana must be configured.

Request body:

```json
{
  "contributor_github": "alice",
  "wallet_address": "7xKX...",
  "tx_signature": "4Rm9..."
}
```

Response `200 OK`:

```json
{
  "campaign_id": "123456789",
  "state": "finalized",
  "allocations": [],
  "tx_signature": "4Rm9...",
  "solana_explorer_url": "https://explorer.solana.com/tx/4Rm9...?cluster=devnet"
}
```

Errors can include these machine codes:

- `CLAIM_CHAIN_LOOKUP_FAILED`
- `CLAIM_NOT_CONFIRMED_ON_CHAIN`
- `CLAIM_WALLET_MISMATCH`

### `POST /campaigns/{id}/refund`

Builds a sponsor-paid refund transaction after the campaign claim deadline.

Solana must be configured.

Request body:

```json
{
  "sponsor_wallet": "7xKX..."
}
```

Response `200 OK`:

```json
{
  "partial_tx": "base58-serialized-transaction"
}
```

Errors:

- `400` for missing or invalid sponsor wallet.
- `403` when the wallet is not the campaign sponsor.
- `404` when the campaign is not found on-chain.
- `409` when the claim deadline has not passed or the campaign is already closed according to backend checks.
- `503` when Solana is not configured.

### `POST /campaigns/{id}/refund-confirm`

Verifies a submitted refund transaction and updates stored campaign state.

Solana must be configured.

Request body:

```json
{
  "sponsor_wallet": "7xKX...",
  "tx_signature": "4Rm9..."
}
```

Response `200 OK`: updated campaign object.

## Data Conventions

Amounts are lamports. `1 SOL = 1,000,000,000 lamports`.

Allocation percentages are basis points. `10000` means 100%, `5000` means 50%, and `100` means 1%.

Campaign ids are numeric strings generated by the backend. The current code uses generated numeric ids rather than a DB-backed auto-increment counter.

On-chain campaign status is represented as Active, Finalized, and Closed. The backend still carries compatibility state strings such as `created`, `funded`, `finalized`, and `completed` in some response fields.

## Current Limitations

- Backend-paid claim fee mode exists as an on-chain payer-mode value, but the shipped backend/frontend claim path is user-paid.
- The Solana program supports batch finalization, but durable backend multi-batch orchestration and recovery are not complete.
- The on-chain close-unfinalizable instruction exists, but backend operator automation for it is not exposed as an API flow.
- Service wallet balance monitoring and alerting are not exposed through the API.
