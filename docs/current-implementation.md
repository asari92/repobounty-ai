# Current Implementation

Baseline: `ТЗ_Enshor_v4_1_has_more_note.md` only. This document describes what the repository currently implements, not what the README or product copy implies.

## High-Level Shape

Enshor is implemented as a React/Vite frontend, a Go API backend, and one Anchor/Solana program.

The implemented MVP flow is:

1. Sponsor creates a campaign for a public GitHub repository.
2. Backend validates the repo, generates a numeric campaign id, and builds a Solana create-with-deposit transaction.
3. Sponsor signs the transaction in the frontend.
4. Backend confirms the on-chain campaign and stores off-chain metadata in SQLite or memory.
5. After deadline, backend calculates allocations from GitHub data and AI or deterministic fallback.
6. Backend finalizes allocations on-chain by creating claim records.
7. A GitHub-authenticated contributor asks the backend for a partially signed claim transaction.
8. The contributor signs and submits the claim transaction from their selected wallet.
9. Backend confirms the claim against on-chain claim-record state and updates local display state.
10. Sponsor can build and confirm a refund transaction after the claim deadline.

The flow is runtime-dependent. If `SERVICE_PRIVATE_KEY` is empty or `PROGRAM_ID` is empty/placeholder, the backend boots in Solana mock mode, but on-chain create/finalize/claim/refund endpoints return unavailable instead of fake transactions. Evidence: `backend/internal/solana/client.go:43`, `backend/internal/http/handlers.go:187`, `backend/internal/http/handlers.go:583`, `backend/internal/http/handlers.go:930`, `backend/internal/http/handlers.go:1185`.

## Solana Program

The repository contains one Anchor program under `program/programs/repobounty`. The entrypoints include `initialize_config`, `update_config`, `create_campaign_with_deposit`, `finalize_campaign_batch`, `claim`, `refund_unclaimed`, `close_unfinalizable_campaign`, and `set_paused`. Evidence: `program/programs/repobounty/src/lib.rs`.

Implemented on-chain entities:

| Entity | Implementation evidence | Notes |
|---|---|---|
| Config | `program/programs/repobounty/src/state/config.rs`, `initialize_config.rs`, `update_config.rs`, `set_paused.rs` | Stores admin wallet, finalize authority, claim authority, treasury wallet, pause flag, and version. |
| Campaign | `program/programs/repobounty/src/state/campaign.rs` | Stores `github_repo_id`, sponsor, deadlines, reward totals, allocation/claim counters, status, and version. |
| ClaimRecord | `program/programs/repobounty/src/state/claim_record.rs` | Stores campaign, `github_user_id`, amount, claimed flag, claimed wallet, claimed timestamp. |
| Escrow PDA | `create_campaign.rs`, `claim.rs`, `refund_unclaimed.rs` | Holds SOL reward pool for the campaign. |

Implemented on-chain business rules:

| Rule | Status | Evidence |
|---|---|---|
| One program for MVP | Implemented | Single Anchor program under `program/programs/repobounty`. |
| Campaign creation locks reward pool immediately | Implemented | `create_campaign.rs:93` transfers reward amount to escrow in the same instruction. |
| Service fee is separate from reward pool | Implemented | `create_campaign.rs:79` computes fee and `create_campaign.rs:105` transfers it to treasury. |
| Minimum campaign amount is `0.5 SOL` | Implemented | `constants.rs:17`, `create_campaign.rs:56`. |
| Deadline range is `now + 5 minutes` to `now + 365 days` | Implemented on-chain | `constants.rs:22`, `constants.rs:25`, `create_campaign.rs:64`. |
| Claim deadline is `deadline_at + 365 days` | Implemented | `constants.rs:29`, `create_campaign.rs:123`. |
| Campaign statuses are Active/Finalized/Closed | Implemented on-chain as `u8` | `constants.rs:41`, `campaign.rs:48`. |
| Claim only after finalized | Implemented | `claim.rs:24`. |
| Claim window enforced | Implemented | `claim.rs:80`. |
| Double claim prevented | Implemented | `claim.rs:30`, `claim.rs:111`. |
| Claim recipient must be user signer | Implemented | `claim.rs:49`. |
| Sponsor refund only after claim deadline | Implemented | `refund_unclaimed.rs:40`. |
| Close unfinalizable only before partial finalization | Implemented on-chain | `close_unfinalizable.rs:23`. |
| Program pause blocks create/finalize/claim/refund/close | Implemented | Constraints in instruction account structs. |

Important on-chain behavior:

- `refund_unclaimed` is implemented as a sponsor-signed transaction and requires `clock.unix_timestamp > campaign.claim_deadline_at`. It transfers the remaining escrow balance to the sponsor and closes the campaign. Evidence: `program/programs/repobounty/src/instructions/refund_unclaimed.rs`.
- `claim` accepts both payer-mode constants, but the on-chain handler only validates/emits `payer_mode`; fee-payer selection is off-chain transaction construction. Evidence: `program/programs/repobounty/src/instructions/claim.rs:58`, `claim.rs:126`.
- `finalize_campaign_batch` supports the spec’s `has_more` concept. It leaves status Active when `has_more == true` and sets Finalized only when `has_more == false` and the allocated amount equals the total reward amount. Evidence: `program/programs/repobounty/src/instructions/finalize_campaign.rs:154`.

## Backend

The backend is a Go/Chi API with structured logging, CORS, rate limiting, GitHub OAuth, GitHub data collection, OpenRouter/deterministic allocation, SQLite or in-memory storage, Solana transaction builders, and an auto-finalization worker. Evidence: `backend/cmd/api/main.go`, `backend/internal/http/router.go`.

Implemented API groups:

| Area | Evidence | Notes |
|---|---|---|
| Health | `backend/internal/http/router.go:44` | `/api/health`. |
| GitHub OAuth | `backend/internal/http/router.go:46` | GitHub URL, callback, current user, wallet link, claims, my campaigns. |
| GitHub search | `backend/internal/http/router.go:55` | Optional auth. |
| Campaigns | `backend/internal/http/router.go:60` | List, get, create, create-confirm, finalize, claim, refund. |

Implemented campaign creation behavior:

- `POST /api/campaigns/` requires Solana configured, validates request body, validates repo existence, checks sponsor wallet balance, resolves GitHub repository id, generates a numeric campaign id, and builds a create-with-deposit transaction. Evidence: `backend/internal/http/handlers.go:187`.
- `POST /api/campaigns/{id}/create-confirm` verifies the on-chain campaign exists and matches sponsor, pool amount, deadline, and GitHub repo id before storing metadata. Evidence: `backend/internal/http/handlers.go:302`.
- The older `fund-tx` endpoint is still routed but returns `410 Gone`; campaigns are now created atomically on-chain. Evidence: `backend/internal/http/handlers.go:418`.

Implemented finalization behavior:

- Manual finalize preview requires GitHub auth and stored campaign owner. It calculates allocations and persists a finalize snapshot. Evidence: `backend/internal/http/handlers.go:436`.
- Wallet-proof finalization allows the sponsor wallet to sign a challenge and finalize without GitHub auth ownership. Evidence: `backend/internal/http/handlers.go:493`, `backend/internal/http/handlers.go:583`.
- GitHub-auth finalization requires an approved current snapshot and then sends finalization on-chain. Evidence: `backend/internal/http/handlers.go:840`.
- Auto-finalization runs in the background, scans stored and on-chain campaigns, retries failures, and marks campaigns `needs_manual_review` after three retries. Evidence: `backend/internal/http/worker.go:17`, `backend/internal/http/worker.go:26`, `backend/internal/http/worker.go:106`.
- The backend currently calls `FinalizeCampaign` with one instruction and `has_more = false`; helper code for planning batches exists, but the production finalization sender does not currently iterate batch plans. Evidence: `backend/internal/solana/client.go:682`, `backend/internal/solana/client.go:766`.

Implemented allocation behavior:

- The MVP intentionally analyzes full available repository history, not the campaign window. Evidence: `backend/internal/github/campaign_window.go:25`.
- If OpenRouter is configured, allocation can use LLM scoring. If unavailable or failing, backend falls back to deterministic allocation. Evidence: `backend/internal/ai/allocator.go:65`, `backend/internal/ai/allocator.go:277`.
- Backend validates final allocations before sending them on-chain: non-empty, `github_user_id != 0`, contributor id in repository identity set, each amount at least `0.05 SOL`, and exact sum equal to campaign pool. Evidence: `backend/internal/http/handlers.go:689`.
- There is a standalone allocation calculator that can trim by dynamic max recipients, minimum allocation, and top-1 remainder. Evidence: `backend/internal/ai/allocation_calculator.go:21`.

Implemented claim behavior:

- Claim requires GitHub auth and a wallet challenge. The backend validates that the GitHub user claims only their own allocation. Evidence: `backend/internal/http/handlers.go:930`.
- Backend checks on-chain claim state before building a claim transaction and again during claim confirmation. Evidence: `backend/internal/http/handlers.go:1017`, `backend/internal/http/handlers.go:1119`.
- Backend builds a partially signed claim transaction with user as fee payer and service private key as claim authority signer. Evidence: `backend/internal/solana/client.go:274`, `backend/internal/solana/client.go:325`, `backend/internal/solana/client.go:338`.
- Claim confirmation only marks the allocation claimed after reading confirmed on-chain `ClaimRecord` state. Evidence: `backend/internal/solana/client.go:365`, `backend/internal/http/handlers.go:1119`.

Implemented refund behavior:

- Backend builds sponsor-paid refund transactions only after on-chain claim deadline and only for the campaign sponsor. Evidence: `backend/internal/http/handlers.go:1185`.
- Backend verifies refund transactions before updating local display state. Evidence: `backend/internal/http/handlers.go:1247`, `backend/internal/solana/client.go:520`.

Storage behavior:

- SQLite schema stores campaigns, users, wallet challenges, and finalize snapshots. Evidence: `backend/internal/store/sqlite.go:47`.
- If `DATABASE_PATH` is empty, backend falls back to in-memory storage. Evidence: `backend/cmd/api/main.go:43`.
- Defaults set `DATABASE_PATH` to `repobounty.db`, so normal local startup uses SQLite unless config is changed. Evidence: `backend/internal/config/config.go:44`.

## Frontend

The frontend is React 18, Vite, TypeScript, Tailwind, and Solana wallet adapter. Evidence: `frontend/package.json`.

Implemented routes:

| Route | Evidence | Notes |
|---|---|---|
| `/` | `frontend/src/App.tsx:34` | Home/explore. |
| `/create` | `frontend/src/App.tsx:35` | Campaign creation. |
| `/campaign/:id` | `frontend/src/App.tsx:36` | Campaign details, finalize, claim, refund. |
| `/profile` | `frontend/src/App.tsx:37` | User profile/claims. |
| `/about` | `frontend/src/App.tsx:38` | Project explanation page. |
| `/auth/callback` | `frontend/src/App.tsx:39` | GitHub OAuth callback. |

Frontend wallet behavior:

- Frontend uses Phantom via wallet adapter and defaults to devnet unless `VITE_SOLANA_NETWORK` or `VITE_SOLANA_RPC_URL` are set. Evidence: `frontend/src/main.tsx:11`.
- Campaign creation asks connected wallet to sign/send the unsigned create transaction returned by the backend and then calls create-confirm. Evidence: `frontend/src/pages/CreateCampaign.tsx:253`, `frontend/src/pages/CreateCampaign.tsx:98`.
- Claim UI has an explicit claim phase state machine for wallet prompt, transaction building, transaction confirmation, backend confirmation, refetching, claimed, and error states. Evidence: `frontend/src/pages/CampaignDetails.tsx:36`.

Frontend auth behavior:

- GitHub OAuth/JWT state is held in `localStorage`. Evidence: `frontend/src/hooks/useAuth.tsx:33`, `frontend/src/hooks/useAuth.tsx:78`.

## Deployment and Runtime Configuration

Docker support exists for backend, frontend, and Solana program tooling.

| Component | Evidence | Notes |
|---|---|---|
| Backend image | `backend/Dockerfile` | Multi-stage Go build, Alpine runtime, non-root user, exposes 8080. |
| Frontend image | `frontend/Dockerfile` | Node build, Nginx runtime, non-root nginx user, exposes 8080. |
| Program tool image | `program/Dockerfile` | Rust, Anchor, Solana/Agave tooling. |
| Compose | `docker-compose.yml` | Backend, frontend, deploy/check profiles, backend healthcheck, frontend healthcheck. |

Runtime environment surface:

- Backend config supports `PORT`, `GITHUB_TOKEN`, `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `FRONTEND_URL`, `JWT_SECRET`, `OPENROUTER_API_KEY`, `MODEL`, `SOLANA_RPC_URL`, `SERVICE_PRIVATE_KEY`, `PROGRAM_ID`, `ENV`, `ALLOWED_ORIGINS`, `GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY`, `DATABASE_PATH`, `TREASURY_WALLET`, `MIN_CAMPAIGN_AMOUNT`, `MIN_ALLOCATION_AMOUNT`, `MAX_ALLOCATIONS`, `MIN_DEADLINE_SECONDS`, `FINALIZE_BATCH_SIZE`, and `AUTO_FINALIZE_INTERVAL_SECONDS`. Evidence: `backend/internal/config/config.go:13`.
- `.env.example` documents only a subset of that surface and omits several operational knobs. Evidence: `.env.example`.

## README Mismatches

The README is broadly aligned with the direction of the implementation, but several statements are stronger or less precise than the code supports.

| README claim | Implementation reality |
|---|---|
| “Allocations are finalized automatically” | Auto-finalize exists, but it can fall back to stored state, retries only three times, and marks `needs_manual_review`; it does not implement full close-unfinalizable workflow. |
| “Contributor claim flow after campaign finalization” | Implemented for user-paid fee mode in backend/frontend; backend-paid fee mode exists on-chain as a payer-mode constant but is not wired as a selectable frontend/backend path. |
| “Backend validates input and prepares funding flow” | Current flow is create-with-deposit; `fund-tx` is deprecated and returns Gone. |
| “GitHub / AI fallback can still operate with public GitHub access, mock contributor data, or deterministic allocation” | Deterministic allocation exists. In runtime code inspected here, Solana actions are disabled when unconfigured; any mock contributor behavior appears to be test-oriented or historical rather than a guaranteed production fallback. |
| `DATABASE_PATH=/data/enshor.db` in README example | Compose uses `/data/repobounty.db`; config default is `repobounty.db`; `.env.example` does not document `DATABASE_PATH`. |

## Current Evidence-Based Summary

The on-chain program implements most of the core Solana MVP contract model from the spec: one program, Config/Campaign/ClaimRecord, escrowed SOL at creation, separate service fee, deadline-gated finalization, claim records by `github_user_id`, no double claim, claim-window expiry, refund after `deadline_at + 365 days`, and admin/service authority separation.

The backend and frontend implement an end-to-end demo-capable flow, but not every off-chain requirement in the spec is complete. The main incomplete areas are backend-paid claim fee selection, durable batch finalization progress/recovery, automatic or manual close-unfinalizable orchestration, dynamic recipient limit based on actual ClaimRecord rent/service-fee economics in the active finalization path, and production monitoring/alerting around service wallet balance and manual review.
