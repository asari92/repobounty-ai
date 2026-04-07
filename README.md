# RepoBounty AI

AI-powered reward allocation for open-source contributors, with SOL escrow and claims on Solana.

## What is RepoBounty AI

RepoBounty AI helps sponsors fund open-source repositories and distribute rewards to contributors based on their actual GitHub impact. The system uses GitHub data, AI allocation logic, and Solana on-chain state to make reward distribution transparent and verifiable.

## Features

- Sponsor-funded campaigns for open-source repositories
- GitHub OAuth login for campaign owners and contributors
- AI-assisted reward allocation based on GitHub activity
- SOL escrow and on-chain campaign state on Solana
- Funding, finalization, and claim flows
- Deterministic fallback when external services are unavailable
- Preview and finalize flows aligned with the same allocation logic

## Why Solana

We chose Solana because this project needs fast, low-cost, and verifiable on-chain reward distribution. RepoBounty AI creates many small blockchain actions — campaign creation, funding, finalization, and multiple claims — so transaction fees and confirmation time matter a lot. On Solana, these operations stay cheap and fast, which makes the UX practical even when the backend or contributors trigger several on-chain actions.

Solana also fits our architecture well. Program Derived Addresses (PDAs) let us model campaigns, escrow vaults, and claim records in a clean deterministic way, without deploying a new contract per campaign. That makes the on-chain state easier to organize and reason about. In addition, Solana wallet support is smooth: Phantom and `@solana/wallet-adapter` make sponsor funding and contributor claims straightforward.

For RepoBounty AI, Solana is not just a blockchain choice — it is a strong technical fit for escrow, transparent rewards, and high-frequency reward flows.

## Why this matters

Open-source contributors often do valuable work without a clear, fair, or transparent reward mechanism. Manual distribution is subjective, slow, and hard to verify. RepoBounty AI addresses that problem by turning GitHub activity into a reward flow that is automated, auditable, and backed by real blockchain state.

This matters because it makes sponsor funding more trustworthy, contributor rewards more transparent, and the overall process easier to scale across repositories and communities.

## The Problem We Solve

RepoBounty AI helps sponsors and open-source projects automatically distribute rewards to contributors. Instead of manual and often subjective selection, the system analyzes contributor activity, creates allocations, and records the result on-chain.

This solves a common open-source problem: it is hard to reward contributors fairly, consistently, and transparently. RepoBounty AI turns GitHub activity into a reward flow that is automated, auditable, and backed by real blockchain state.

## What Ships Today

RepoBounty AI is a deadline-based MVP:

1. A sponsor connects a Solana wallet (NO GitHub login required).
2. The sponsor creates a campaign with repo, amount, deadline, and wallet.
3. Backend validates repository, checks wallet balance, returns unsigned transaction.
4. Sponsor signs and broadcasts the funding transaction.
5. Sponsor confirms campaign creation with backend.
6. After the deadline, backend fetches GitHub contributor data and PR diffs.
7. Owner (GitHub auth) previews and finalizes allocations on-chain.
8. Contributors log in with GitHub, initiate claim, sign challenge, and claim rewards.

The backend is the trust anchor for GitHub auth, repo validation, allocation logic, and claim authorization. The on-chain program enforces campaign state, deadline, escrow, percentages, and claim bookkeeping.

## Current Permission Model

- `POST /api/campaigns/` — NO auth (wallet only)
- `POST /api/campaigns/{id}/create-confirm` — NO auth (wallet only)
- `POST /api/campaigns/{id}/refund` — NO auth (wallet only)
- `POST /api/campaigns/{id}/refund-confirm` — NO auth (wallet only)
- `POST /api/campaigns/{id}/fund-tx` — NO auth (wallet only)
- `POST /api/campaigns/{id}/finalize-preview` — GitHub auth required (owner only)
- `POST /api/campaigns/{id}/finalize` — GitHub auth required (owner only)
- `POST /api/campaigns/{id}/claim-challenge` — GitHub auth required
- `POST /api/campaigns/{id}/claim-permit` — GitHub auth required
- `POST /api/campaigns/{id}/claim-confirm` — GitHub auth required
- `GET /api/auth/my-campaigns` — GitHub auth required

Saved profile wallets are not authoritative in this MVP. They are informational only unless a stricter wallet-linking flow is implemented end to end.

## Finalize Preview Behavior

Finalize preview is intentionally aligned with real finalization:

- It is only available for funded campaigns after the deadline.
- It uses the same backend allocation path as real finalization.
- When PR diffs are available, preview and finalize both use PR-diff impact scoring.
- When PR diffs are unavailable, both fall back to contributor-metric allocation.

## Mock And Fallback Behavior

Two different fallback modes exist:

- GitHub / AI fallback:
  Without `GITHUB_TOKEN` or `OPENROUTER_API_KEY`, the backend can still use public GitHub access, mock contributor data, and deterministic allocation.
- Solana fallback:
  If the backend is missing a real Solana authority key or program ID, the API still boots, but on-chain create, fund, finalize, and claim actions are disabled instead of returning fake transactions.

Check `GET /api/health` to see whether Solana is currently configured.

## Architecture

```text
Frontend (React + Wallet)
        |
        v
Backend (Go API, GitHub OAuth/JWT, GitHub fetches, AI allocation, SQLite)
        |
        v
Solana Program (Anchor)
```

## Tech Stack

| Layer | Technology |
|-------|------------|
| Frontend | React 18, Vite, TypeScript, Tailwind |
| Wallet | `@solana/wallet-adapter` |
| Backend | Go 1.25, Chi, Zap |
| Auth | GitHub OAuth + JWT |
| Storage | SQLite / in-memory |
| AI | OpenRouter or deterministic fallback |
| Blockchain | Solana + Anchor 0.32.1 |

## How To Run

### Prerequisites

Before running RepoBounty AI locally, make sure you have:

- Go 1.25+ installed
- Node.js 20+ installed
- Docker and Docker Compose installed
- A Solana wallet such as Phantom
- `anchor` and `solana-cli` if you want to run the program locally

### Environment Variables

Create `.env` files for the backend and frontend if needed.

#### Root .env example
```bash
# === Required ===
JWT_SECRET=change-me-to-a-random-string-at-least-32-chars

# === GitHub OAuth (for user login) ===
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret

# === GitHub API (for fetching contributor data) ===
GITHUB_TOKEN=your_github_token

# === GitHub App (optional, for PR notifications) ===
GITHUB_APP_ID=your_github_app_id
GITHUB_APP_PRIVATE_KEY=-----BEGIN RSA PRIVATE KEY-----

# === Solana ===
SOLANA_RPC_URL=https://api.devnet.solana.com
SERVICE_PRIVATE_KEY=          # service_wallet: finalize/claim/backend-paid txs
PROGRAM_ID=your_program_id
SOLANA_DEPLOY_WALLET=         # admin_wallet dir containing id.json

# === AI (optional, works without it via deterministic fallback) ===
OPENROUTER_API_KEY=your_openrouter_api_key
MODEL=nvidia/nemotron-3-super-120b-a12b:free

# === URLs ===
FRONTEND_URL=http://localhost:5173
ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000

# === Backend ===
PORT=8080
DATABASE_PATH=/data/repobounty.db
```

#### Frontend example
```bash
VITE_API_BASE_URL=http://localhost:8080
VITE_SOLANA_CLUSTER=devnet
```

### Docker Workflows

#### 1. Contract already deployed

If you already have a live program on Solana, put its public key into `PROGRAM_ID` in the root `.env`, then start the app:

```bash
./start.sh
```

or:

```bash
docker compose up --build
```

#### 2. Build, test, and deploy the contract from Docker

If you do not want to install Anchor or Solana locally, use the dedicated deploy profile:

```bash
docker compose --profile deploy run --rm solana-check
```

This safe check container will:
- run `anchor build`
- start `solana-test-validator`
- run `anchor deploy --provider.cluster localnet`
- run the TypeScript integration tests
- never deploy to `devnet`

#### 3. Real devnet deploy

```bash
docker compose --profile deploy run --rm solana-deployer
```

This flow will:
- run the same local `build + deploy + test` validation first
- deploy the program to `devnet`
- initialize or update on-chain config so:
  `admin_wallet = SOLANA_DEPLOY_WALLET`,
  `finalize_authority = SERVICE_PRIVATE_KEY`,
  `claim_authority = SERVICE_PRIVATE_KEY`,
  `treasury_wallet = SERVICE_PRIVATE_KEY`
- write the resulting program id to `program/.artifacts/program-id`

Before running it, set the two Solana roles in the root `.env`:

```env
SOLANA_DEPLOY_WALLET=/home/your-user/.config/solana
SERVICE_PRIVATE_KEY=<base58-or-json-keypair>
```

After deploy, copy the value from `program/.artifacts/program-id` into `PROGRAM_ID` in `.env`.

### Run with Docker

The easiest way to start the full stack is with Docker:

```bash
docker compose up --build
```

Then open:

- Frontend: http://localhost:5173
- Backend API: http://localhost:8080

### Run Backend Locally

If you want to run only the backend:

```bash
cd backend
go mod tidy
go run ./cmd/api
```

The backend will be available at:

```bash
http://localhost:8080
```

### Run Frontend Locally

If you want to run only the frontend:

```bash
cd frontend
npm install
npm run dev
```

The frontend will be available at:

```bash
http://localhost:5173
```

### Build Frontend for Production

```bash
cd frontend
npm run build
```

### Build Backend

```bash
cd backend
go build -o repobounty-api ./cmd/api
```

### Run Solana Program Locally

If you want to work with the on-chain program:

```bash
cd program
yarn install
anchor build
anchor test
```

### Health Check

After starting the backend, you can verify the system state with:

```bash
GET /api/health
```

This will show whether GitHub, AI, and Solana components are configured correctly.

### Notes

- If `GITHUB_TOKEN` or `OPENROUTER_API_KEY` is missing, the backend can still run with public GitHub access and deterministic fallback allocation.
- If Solana authority or program settings are missing, the backend will still boot, but on-chain actions such as create, fund, finalize, and claim will be disabled.

## API Summary

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/health` | No | Health and Solana readiness |
| GET | `/api/auth/github/url` | No | Start GitHub OAuth |
| POST | `/api/auth/github/callback` | No | Exchange code for JWT |
| GET | `/api/auth/me` | JWT | Current GitHub user |
| POST | `/api/auth/wallet/link` | JWT | Save profile wallet note |
| GET | `/api/auth/claims` | JWT | List claimable allocations |
| GET | `/api/auth/my-campaigns` | JWT | List own campaigns |
| GET | `/api/campaigns/` | No | List campaigns |
| GET | `/api/campaigns/{id}` | No | Campaign details |
| POST | `/api/campaigns/` | No | Create a campaign |
| POST | `/api/campaigns/{id}/create-confirm` | No | Confirm campaign creation |
| POST | `/api/campaigns/{id}/fund-tx` | No | Build sponsor funding transaction |
| POST | `/api/campaigns/{id}/finalize-preview` | JWT | Owner-only preview after deadline |
| POST | `/api/campaigns/{id}/finalize` | JWT | Owner-only manual finalize |
| POST | `/api/campaigns/{id}/claim-challenge` | JWT | Generate claim challenge |
| POST | `/api/campaigns/{id}/claim` | JWT | Claim permit |
| POST | `/api/campaigns/{id}/claim-confirm` | JWT | Confirm claim |
| POST | `/api/campaigns/{id}/refund` | No | Build refund transaction |
| POST | `/api/campaigns/{id}/refund-confirm` | No | Confirm refund |

### Create Campaign Example

```bash
curl -X POST http://localhost:8080/api/campaigns/ \
  -H "Content-Type: application/json" \
  -d '{
    "repo": "owner/repo",
    "pool_amount": 1000000000,
    "deadline": "2026-04-01T12:00:00Z",
    "sponsor_wallet": "YourPhantomPublicKey"
  }'
```

### Finalize Preview Example

```bash
curl -X POST http://localhost:8080/api/campaigns/<campaign-id>/finalize-preview \
  -H "Authorization: Bearer <jwt>"
```

The response includes:
- contributor stats
- proposed allocations
- `allocation_mode` showing whether the preview used `code_impact` or `metrics`

## On-Chain Notes

- Campaign PDA seeds:
  `["campaign", sponsor, campaign_id.to_le_bytes()]`
- Escrow PDA seeds:
  `["escrow", campaign_pda]`
- Claim record PDA seeds:
  `["claim", campaign_pda, github_user_id.to_le_bytes()]`
- The deployed `PROGRAM_ID` is written to `program/.artifacts/program-id` by
  `solana-deployer`.

On-chain state machine for the deadline happy path:

```text
Active -> Finalized -> Closed
```

## Project Structure

```text
backend/   Go API, auth, GitHub, AI, Solana client, SQLite
frontend/  React app, wallet connect, campaign pages
program/   Anchor program and tests
docs/      Extra project documentation
```