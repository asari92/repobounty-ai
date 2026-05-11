# Architecture

This document reflects the current Enshor repository state: a React/Vite frontend, a Go backend API, and one Anchor/Solana program.

## System Overview

Enshor is an escrow-backed GitHub reward campaign MVP.

The core flow is:

1. A sponsor chooses a public GitHub repository, reward amount, deadline, and sponsor wallet.
2. The backend validates the repository and builds a Solana create-with-deposit transaction.
3. The sponsor wallet signs the transaction in the frontend.
4. The Solana program creates the campaign account and escrow PDA, locks the reward pool, and transfers the service fee to treasury.
5. After the deadline, the backend calculates contributor allocations from GitHub data using OpenRouter or deterministic fallback.
6. The backend finalizes allocations on-chain by creating ClaimRecord accounts keyed by `github_user_id`.
7. A contributor authenticates with GitHub, proves wallet control, signs a claim transaction, and receives SOL from escrow.
8. After `deadline_at + 365 days`, the sponsor can refund unclaimed escrow balance.

## Component Diagram

```text
+-------------------+        +--------------------------+
| React/Vite SPA    | -----> | Go API                   |
| Wallet Adapter    |        | Chi, OAuth, SQLite       |
+-------------------+        +--------------------------+
        |                              |
        |                              +----> GitHub API
        |                              |
        |                              +----> OpenRouter or deterministic fallback
        |                              |
        |                              +----> Solana RPC
        |
        +-----------------------------------> Solana wallet signing

+--------------------------------------------------------+
| Anchor/Solana Program                                  |
| Config, Campaign, ClaimRecord, escrow PDA, instructions |
+--------------------------------------------------------+
```

## Frontend

Technology:

- React 18.
- Vite.
- TypeScript.
- Tailwind CSS.
- `@solana/wallet-adapter` with Phantom configured by default.

Routes:

| Route | Purpose |
|---|---|
| `/` | Home/explore entry point. |
| `/create` | Campaign creation and sponsor wallet signing. |
| `/campaign/:id` | Campaign details, finalize actions, claim flow, refund flow. |
| `/profile` | User profile, claims, and sponsored campaigns. |
| `/about` | Project overview page. |
| `/auth/callback` | GitHub OAuth callback handler. |

Frontend responsibilities:

- Connect sponsor and contributor wallets.
- Submit campaign create requests to the backend.
- Sign and send Solana transactions returned by the backend.
- Run GitHub login through the backend OAuth flow.
- Display campaign state from merged backend/on-chain data.
- Manage claim UX phases: wallet prompt, transaction build, wallet signature, chain confirmation, backend confirmation, refetch, and error states.

Runtime notes:

- API calls use the relative `/api` path.
- Wallet RPC defaults to devnet via `VITE_SOLANA_NETWORK` or `VITE_SOLANA_RPC_URL`.
- Auth token is stored in `localStorage`.

## Backend API

Technology:

- Go.
- Chi router.
- Zap logging.
- SQLite or in-memory store.
- GitHub OAuth and JWT.
- GitHub repository/contributor API client.
- OpenRouter allocation with deterministic fallback.
- Solana RPC client and transaction builders.

Main modules:

| Module | Responsibility |
|---|---|
| `cmd/api` | Service composition, config load, server startup, graceful shutdown, worker startup. |
| `internal/config` | Environment parsing and defaults. |
| `internal/http` | Routes, handlers, middleware, wallet challenges, finalize snapshots, auto-finalize worker. |
| `internal/auth` | JWT and GitHub OAuth. |
| `internal/github` | Repository validation, user/repo search, contributor data, PR/commit fallback paths. |
| `internal/ai` | LLM allocation, deterministic fallback, allocation normalization helpers. |
| `internal/solana` | PDA derivation, account decoding, transaction construction, claim/refund verification. |
| `internal/store` | SQLite and in-memory persistence. |

Backend responsibilities:

- Validate public GitHub repository existence.
- Generate campaign ids.
- Build create-with-deposit transactions.
- Confirm on-chain campaign creation before storing campaign metadata.
- Maintain users, campaigns, wallet challenges, and finalize snapshots.
- Authenticate users with GitHub OAuth.
- Calculate allocations after deadline.
- Finalize campaigns on-chain using the service wallet.
- Build user-paid claim transactions partially signed by the service wallet.
- Confirm claims by reading on-chain ClaimRecord state.
- Build and verify sponsor refund transactions.
- Run an auto-finalize worker with retry and `needs_manual_review` status.

Runtime-dependent behavior:

- If `SERVICE_PRIVATE_KEY` or `PROGRAM_ID` is missing, Solana transaction endpoints are disabled.
- If `OPENROUTER_API_KEY` is missing or model calls fail, deterministic fallback allocation is used.
- If `DATABASE_PATH` is empty, storage is in-memory; otherwise SQLite is used.
- In production, `JWT_SECRET` is required and must be at least 32 characters.

## Solana Program

The repository contains one Anchor program under `program/programs/repobounty`.

Core accounts:

| Account | Purpose |
|---|---|
| `Config` | Stores version, admin wallet, finalize authority, claim authority, treasury wallet, pause flag, and bump. |
| `Campaign` | Stores campaign id, sponsor, GitHub repo id, deadlines, reward totals, allocated/claimed amounts, counts, status, and bump. |
| `ClaimRecord` | Stores campaign pubkey, GitHub user id, claim amount, claimed flag, claimed wallet, claimed timestamp, and bump. |
| Escrow PDA | System-owned PDA that holds reward SOL for a campaign. |

Instructions:

| Instruction | Purpose |
|---|---|
| `initialize_config` | Creates global config and sets admin/service/treasury authorities. |
| `update_config` | Allows admin to update finalize authority, claim authority, and treasury wallet. |
| `set_paused` | Allows admin to pause or unpause the program. |
| `create_campaign_with_deposit` | Creates campaign, locks reward pool in escrow, transfers service fee to treasury. |
| `finalize_campaign_batch` | Creates ClaimRecord accounts and marks the campaign finalized on the final batch. |
| `claim` | Transfers a claim amount from escrow to the user signer wallet and marks the claim record as claimed. |
| `refund_unclaimed` | After claim deadline, refunds remaining escrow to sponsor and closes the campaign. |
| `close_unfinalizable_campaign` | Closes an active campaign after deadline when no finalize batch has succeeded. |

On-chain invariants:

- Minimum campaign amount is `0.5 SOL`.
- Minimum allocation amount is `0.05 SOL`.
- Campaign deadline must be between `now + 5 minutes` and `now + 365 days`.
- Claim deadline is `deadline_at + 365 days`.
- Service fee is `max(0.5% of reward_pool, 0.05 SOL)`.
- Claim requires campaign status `Finalized`.
- Claim requires `recipient_wallet == user signer`.
- Double claim is prevented by `claim_record.claimed == false`.
- Refund requires `now > claim_deadline_at` and positive escrow balance.
- Close-unfinalizable requires campaign status `Active`, deadline reached, and `allocations_count == 0`.

## Data Flow

### Campaign Creation

1. Frontend submits repo, reward amount, deadline, and sponsor wallet to the backend.
2. Backend validates repo format, GitHub public repository existence, sponsor balance, reward minimum, and deadline.
3. Backend resolves `github_repo_id`.
4. Backend generates a numeric campaign id.
5. Backend builds an unsigned create-with-deposit transaction.
6. Frontend asks the sponsor wallet to sign and send the transaction.
7. Frontend calls create-confirm with the transaction signature.
8. Backend reads the on-chain campaign and verifies sponsor, amount, deadline, and repository id.
9. Backend stores campaign metadata in SQLite or memory.

### Finalization

1. Deadline passes.
2. Manual finalize preview, sponsor wallet finalize, or auto-finalize triggers allocation calculation.
3. Backend fetches full repository-history contributor data for MVP/demo behavior.
4. Backend uses OpenRouter when configured, otherwise deterministic fallback.
5. Backend normalizes and validates allocations.
6. Backend sends finalization to Solana with service wallet as finalize authority.
7. Solana program creates ClaimRecord accounts and sets campaign status to Finalized when total allocated amount equals total reward amount.
8. Backend stores finalization result and allocation metadata.

Current finalization limitation:

- The Solana program supports batched finalization.
- Backend has batch-planning helper code.
- The active backend finalization sender currently sends a single finalization instruction with `has_more = false`.
- Durable multi-batch progress storage and recovery are not complete.

### Claim

1. Contributor logs in with GitHub.
2. Frontend requests a claim wallet challenge for a specific campaign allocation.
3. Contributor signs the challenge with the selected wallet.
4. Backend verifies GitHub identity, wallet proof, allocation ownership, and current on-chain claim state.
5. Backend builds a partially signed claim transaction.
6. Contributor wallet signs and submits the transaction.
7. Backend claim-confirm reads the on-chain ClaimRecord with confirmed commitment.
8. Backend updates local allocation display state only after the on-chain claim is confirmed.

Current claim limitation:

- The on-chain instruction accepts user-paid and backend-paid payer-mode values.
- The shipped backend/frontend path builds user-paid claim transactions.

### Refund

1. Claim deadline passes.
2. Sponsor requests a refund transaction.
3. Backend loads on-chain campaign data and checks sponsor wallet and claim deadline.
4. Sponsor signs and submits the refund transaction.
5. Backend verifies the refund transaction and updates local campaign state.
6. Solana program transfers remaining escrow to sponsor and closes the campaign.

## Trust Boundaries

On-chain trust boundary:

- Reward pool escrow.
- Service fee transfer.
- Campaign status.
- ClaimRecord amount and claimed state.
- Claim window and refund deadline.
- No double claim.
- Admin/config authority checks.

Off-chain trust boundary:

- GitHub OAuth and GitHub user identity.
- GitHub repository validation.
- Repository contribution data collection.
- AI or deterministic allocation logic.
- Campaign id generation.
- Finalization orchestration.
- Claim transaction construction and service wallet signature.
- Local campaign metadata and UI state.

Important MVP trust limitation:

The contract does not verify GitHub data or AI output. It trusts the configured finalize authority to create ClaimRecord accounts and trusts the configured claim authority to participate in claim transactions. This is intentional for the MVP and should be treated as a production hardening area.

## Persistence Model

SQLite stores:

- Campaign metadata and displayed allocations.
- Users from GitHub OAuth.
- Wallet challenges.
- Finalize snapshots.

On-chain state remains authoritative for:

- Campaign existence and status.
- Escrowed funds.
- Claim records.
- Claimed flags and claimed wallets.
- Refund eligibility and campaign closure.

The backend merges stored campaign metadata with on-chain data when Solana is configured. If on-chain allocations are missing from decoded campaign data, stored allocation metadata is used for display.

## Deployment Architecture

Docker Compose defines:

| Service | Purpose |
|---|---|
| `backend` | Go API, SQLite volume, healthcheck. |
| `frontend` | Nginx-served SPA, proxies `/api`, host port `5173`. |
| `solana-check` | Program check workflow under the `deploy` profile. |
| `solana-deployer` | Devnet deployment workflow under the `deploy` profile. |

Backend container:

- Multi-stage Go build.
- Alpine runtime.
- Non-root user.
- Exposes `8080` inside the Compose network.

Frontend container:

- Node build stage.
- Nginx runtime.
- Non-root nginx user.
- Exposes `8080` in container and maps to host `5173`.

Program tool container:

- Rust, Node, Anchor, and Solana/Agave tooling.
- Used for program check and deploy scripts.

## Known Architecture Gaps

- Backend-paid claim fee mode is not wired end-to-end in the current backend/frontend path.
- Durable multi-batch finalization orchestration is not complete.
- On-chain `close_unfinalizable_campaign` exists, but backend automation/operator API for it is not exposed.
- Campaign id generation is backend-generated numeric id, not a DB-backed auto-increment counter.
- Service wallet balance monitoring and alerting are not implemented.
- The MVP analyzes full repository history instead of only the campaign window.
