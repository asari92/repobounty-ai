# RepoBounty AI

AI-powered reward allocation for open-source contributors, with SOL escrow and
claims on Solana.

## What Ships Today

RepoBounty AI is a deadline-based MVP:

1. A sponsor logs in with GitHub and connects a Solana wallet.
2. The backend authority creates a campaign on-chain and stores the GitHub
   creator off-chain.
3. The sponsor signs a funding transaction from the connected wallet.
4. After the deadline, the backend fetches GitHub contributor data and PR diffs
   when available.
5. The backend computes allocations off-chain, then finalizes them on-chain.
6. Contributors log in with GitHub and claim to the wallet they currently have
   connected.

The backend is the trust anchor for GitHub auth, repo validation, allocation
logic, and claim authorization. The on-chain program enforces campaign state,
deadline, escrow, percentages, and claim bookkeeping.

## Current Permission Model

- `POST /api/campaigns/` requires GitHub auth.
- The authenticated GitHub user who creates a campaign becomes the stored
  campaign owner.
- Manual `fund-tx`, `finalize-preview`, and `finalize` actions are limited to
  that stored owner.
- Auto-finalize still runs in the backend worker after the deadline.
- Claims require GitHub auth, but they send SOL to the wallet currently
  connected in the UI.

Saved profile wallets are not authoritative in this MVP. They are informational
only unless a stricter wallet-linking flow is implemented end to end.

## Finalize Preview Behavior

Finalize preview is intentionally aligned with real finalization:

- It is only available for funded campaigns after the deadline.
- It uses the same backend allocation path as real finalization.
- When PR diffs are available, preview and finalize both use PR-diff impact
  scoring.
- When PR diffs are unavailable, both fall back to contributor-metric
  allocation.

## Mock And Fallback Behavior

Two different fallback modes exist:

- GitHub / AI fallback:
  Without `GITHUB_TOKEN` or `OPENROUTER_API_KEY`, the backend can still use
  public GitHub access, mock contributor data, and deterministic allocation.
- Solana fallback:
  If the backend is missing a real Solana authority key or program ID, the API
  still boots, but on-chain create, fund, finalize, and claim actions are
  disabled instead of returning fake transactions.

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

Current happy path:

```text
GitHub auth -> create -> fund -> wait deadline -> preview/finalize -> claim
```

The repo also contains extra non-MVP program instructions like
`withdraw_remaining`, `close_campaign`, `add_sponsor`, and goal-related state.
The frontend/backend happy path currently focuses on:

- `create_campaign`
- `fund_campaign`
- `finalize_campaign`
- `claim`

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

## Quick Start

```bash
./start.sh
```

After startup:

- Frontend: `http://localhost:5173`
- Backend API: `http://localhost:8080`
- Health check: `http://localhost:8080/api/health`

`./start.sh` starts only `frontend` + `backend`. It does not build or deploy the
Solana program.

## Docker Workflows

### App only

If the program is already deployed and `PROGRAM_ID` is set in the root `.env`:

```bash
./start.sh
```

or:

```bash
docker compose up --build
```

### Safe contract check

```bash
docker compose --profile deploy run --rm solana-check
```

This Docker-only flow will:

- run `anchor build`
- start `solana-test-validator`
- run `anchor deploy --provider.cluster localnet`
- run the TypeScript integration tests
- never deploy to `devnet`

### Real devnet deploy

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

After deploy, copy the value from `program/.artifacts/program-id` into
`PROGRAM_ID` in `.env`.

## Local Development

### Backend

```bash
cd backend
cp .env.example .env
go mod tidy
go run ./cmd/api
```

Important backend environment variables:

```env
JWT_SECRET=<random string, min 32 chars>

GITHUB_CLIENT_ID=<github oauth app client id>
GITHUB_CLIENT_SECRET=<github oauth app client secret>

GITHUB_TOKEN=<optional github token for richer api access>

OPENROUTER_API_KEY=<optional>
MODEL=nvidia/nemotron-3-super-120b-a12b:free

SOLANA_RPC_URL=https://api.devnet.solana.com
SERVICE_PRIVATE_KEY=<service_wallet keypair>
PROGRAM_ID=<set after deploy from program/.artifacts/program-id>

FRONTEND_URL=http://localhost:3000
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
DATABASE_PATH=repobounty.db
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

Vite proxies `/api` to `http://localhost:8080`.

### Program

```bash
cd program
npm ci
anchor build
anchor test
```

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

## API Summary

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/health` | No | Health and Solana readiness |
| GET | `/api/auth/github/url` | No | Start GitHub OAuth |
| POST | `/api/auth/github/callback` | No | Exchange code for JWT |
| GET | `/api/auth/me` | JWT | Current GitHub user |
| POST | `/api/auth/wallet/link` | JWT | Save a profile wallet note |
| GET | `/api/auth/claims` | JWT | List claimable allocations |
| GET | `/api/campaigns/` | No | List campaigns |
| GET | `/api/campaigns/{id}` | No | Campaign details |
| POST | `/api/campaigns/` | JWT | Create a campaign |
| POST | `/api/campaigns/{id}/fund-tx` | JWT | Build sponsor funding transaction |
| POST | `/api/campaigns/{id}/finalize-preview` | JWT | Owner-only preview after deadline |
| POST | `/api/campaigns/{id}/finalize` | JWT | Owner-only manual finalize |
| POST | `/api/campaigns/{id}/claim` | JWT | Claim to the connected wallet |

### Create Campaign Example

```bash
curl -X POST http://localhost:8080/api/campaigns/ \
  -H "Authorization: Bearer <jwt>" \
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

## Demo Notes

The cleanest demo is:

1. Log in with GitHub.
2. Connect Phantom on devnet.
3. Create a campaign.
4. Fund it from the connected wallet.
5. Wait until the deadline passes.
6. Preview allocations.
7. Finalize on Solana.
8. Log in as a contributor and claim to the currently connected wallet.

## Project Structure

```text
backend/   Go API, auth, GitHub, AI, Solana client, SQLite
frontend/  React app, wallet connect, campaign pages
program/   Anchor program and tests
docs/      Extra project documentation
```
