# Setup Guide

This guide describes the current Enshor development, Docker, and Solana setup. It is intentionally written for the current repository state, including runtime-dependent behavior and known MVP limitations.

## Prerequisites

Local development:

| Component | Version or Notes |
|---|---|
| Go | `1.25+` for the backend. |
| Node.js | `20+` for the frontend. |
| npm | Used by the frontend and Anchor TypeScript test tooling. |
| Docker and Docker Compose | Recommended for app startup and Solana program tooling. |
| Solana wallet | Phantom or another wallet compatible with `@solana/wallet-adapter`. |

Solana program work:

| Component | Version or Notes |
|---|---|
| Rust | Docker image uses Rust `1.85.0`. |
| Anchor CLI | Docker image installs Anchor `0.32.1`. |
| Solana/Agave tooling | Docker image installs configured runtime/build versions. |

You can avoid installing Anchor and Solana CLI locally by using the Docker Compose `deploy` profile.

## Repository Layout

```text
backend/    Go API, GitHub OAuth, allocation, SQLite storage, Solana builders
frontend/   React 18 + Vite + TypeScript + Tailwind SPA
program/    Anchor/Solana program and program tooling scripts
docs/       Project documentation
```

## Environment Files

The backend loads `.env` from the current working directory and also from the repository root when running from `backend/`.

Create a root `.env` from the repository example:

```bash
cp .env.example .env
```

Minimum local `.env` shape:

```env
JWT_SECRET=change-me-to-a-random-string-at-least-32-chars

GITHUB_CLIENT_ID=
GITHUB_CLIENT_SECRET=
GITHUB_TOKEN=

SOLANA_RPC_URL=https://api.devnet.solana.com
SERVICE_PRIVATE_KEY=
PROGRAM_ID=
SOLANA_DEPLOY_WALLET=

OPENROUTER_API_KEY=
MODEL=nvidia/nemotron-3-super-120b-a12b:free

FRONTEND_URL=http://localhost:5173
ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000
PORT=8080
DATABASE_PATH=repobounty.db
```

Important runtime behavior:

- `JWT_SECRET` is required in production and must be at least 32 characters there.
- If `SERVICE_PRIVATE_KEY` or `PROGRAM_ID` is missing, the backend starts but disables on-chain create, finalize, claim, and refund transaction flows.
- If `OPENROUTER_API_KEY` is missing or model calls fail, deterministic allocation fallback is used.
- `DATABASE_PATH=repobounty.db` stores SQLite data under `backend/` when the backend is run from that directory.
- `DATABASE_PATH=` enables in-memory storage and should only be used for throwaway testing.

Backend config also supports these operational knobs:

```env
ENV=development
TREASURY_WALLET=
MIN_CAMPAIGN_AMOUNT=500000000
MIN_ALLOCATION_AMOUNT=50000000
MAX_ALLOCATIONS=200
MIN_DEADLINE_SECONDS=300
FINALIZE_BATCH_SIZE=5
AUTO_FINALIZE_INTERVAL_SECONDS=60
GITHUB_APP_ID=0
GITHUB_APP_PRIVATE_KEY=
```

The frontend currently uses relative `/api` requests. For wallet RPC selection, it reads:

```env
VITE_SOLANA_NETWORK=devnet
VITE_SOLANA_RPC_URL=
```

If `VITE_SOLANA_RPC_URL` is empty, the frontend uses `clusterApiUrl(VITE_SOLANA_NETWORK || "devnet")`.

## Local Development

Install and run the backend:

```bash
cd backend
go mod download
go run ./cmd/api
```

The backend listens on `http://localhost:8080` by default.

Install and run the frontend:

```bash
cd frontend
npm install
npm run dev
```

Vite serves the frontend at `http://localhost:5173` by default.

Check backend health:

```bash
curl http://localhost:8080/api/health
```

Expected shape:

```json
{
  "status": "ok",
  "solana": false,
  "github": true,
  "ai_model": "deterministic-fallback",
  "store": true
}
```

`solana` is `false` until `SERVICE_PRIVATE_KEY` and `PROGRAM_ID` are configured.

## Docker Startup

Run the app stack:

```bash
docker compose up --build
```

Default URLs:

- Frontend: `http://localhost:5173`
- Backend inside Compose network: `http://backend:8080`
- Backend direct host port is not published by the current compose file; frontend Nginx proxies `/api` to the backend.

Compose services:

| Service | Purpose |
|---|---|
| `backend` | Go API, SQLite volume at `/data/repobounty.db`, healthcheck on `/api/health`. |
| `frontend` | Built React SPA served by Nginx on host port `5173`. |
| `solana-check` | Deploy-profile program check workflow. |
| `solana-deployer` | Deploy-profile devnet deploy workflow. |

## GitHub Setup

GitHub OAuth is required for login, owner-gated preview/finalize, and contributor claim authorization.

Create a GitHub OAuth App:

1. Go to `https://github.com/settings/developers`.
2. Create a new OAuth App.
3. Set Homepage URL to `http://localhost:5173` for local development.
4. Set Authorization callback URL to `http://localhost:5173/auth/callback`.
5. Copy Client ID to `GITHUB_CLIENT_ID`.
6. Generate a client secret and copy it to `GITHUB_CLIENT_SECRET`.

GitHub API access:

- `GITHUB_TOKEN` is used for repository and contributor API calls.
- Public GitHub access may work for some calls, but rate limits are lower.
- The OAuth scope used by the backend is `read:user,user:email`.

## OpenRouter Setup

OpenRouter is optional for the demo flow.

1. Create an API key at `https://openrouter.ai`.
2. Set `OPENROUTER_API_KEY`.
3. Optionally override `MODEL`.

If OpenRouter is not configured, the backend uses deterministic fallback allocation.

## Solana Keys and Roles

The MVP has two operational key roles:

| Role | Config | Purpose |
|---|---|---|
| Admin wallet | `SOLANA_DEPLOY_WALLET` | Deploys the program and initializes/updates config. |
| Service wallet | `SERVICE_PRIVATE_KEY` | Backend finalize authority, claim authority, and treasury wallet in the MVP deploy flow. |

The program supports separate `admin_wallet`, `finalize_authority`, `claim_authority`, and `treasury_wallet`. The MVP deploy script may configure the service wallet for finalize, claim, and treasury roles.

`SERVICE_PRIVATE_KEY` can be a base58 private key or a 64-byte JSON keypair array.

Example:

```env
SERVICE_PRIVATE_KEY=base58-private-key
```

or:

```env
SERVICE_PRIVATE_KEY=[174,23,45,...]
```

For devnet, fund the admin and service wallets with devnet SOL.

## Solana Program Check and Deploy

Run the local/containerized program check:

```bash
docker compose --profile deploy run --rm solana-check
```

The check profile uses the program Docker image and project scripts. It is intended to validate the program workflow without deploying to devnet.

Deploy to devnet:

```bash
docker compose --profile deploy run --rm solana-deployer
```

After deployment, copy the generated program id into `.env`:

```env
PROGRAM_ID=<deployed-program-id>
```

The current Anchor program id in source is declared in `program/programs/repobounty/src/lib.rs`; the runtime backend uses the `PROGRAM_ID` environment variable.

## Devnet Frontend Wallet Setup

Use Phantom or another Solana wallet:

1. Enable developer/testnet mode.
2. Select Solana Devnet.
3. Fund the sponsor wallet with enough devnet SOL for reward pool, service fee, rent, and network fees.
4. Fund contributor wallets if using the current user-paid claim path.

Current claim fee behavior:

- The on-chain claim instruction accepts both user-paid and backend-paid payer-mode values.
- The shipped backend/frontend claim path currently builds user-paid claim transactions.
- Contributors need enough SOL for the claim transaction fee in the current UI flow.

## End-to-End Demo Flow

Use a small public repository and a short MVP deadline.

1. Start backend and frontend locally or through Docker.
2. Confirm `/api/health` reports `solana: true` for real on-chain flows.
3. Open `http://localhost:5173`.
4. Connect the sponsor wallet.
5. Create a campaign with at least `0.5 SOL` reward amount.
6. Sign the create-with-deposit transaction in the wallet.
7. Let the frontend call create-confirm after transaction submission.
8. Wait until the campaign deadline has passed.
9. Use manual finalize or sponsor wallet finalize if auto-finalization has not completed yet.
10. Log in through GitHub as an allocated contributor.
11. Connect a wallet and run the claim flow.
12. Let the backend confirm claim state from the on-chain ClaimRecord.

For hackathon demos, avoid very large recipient counts. The Solana program supports batch finalization semantics, but durable backend multi-batch orchestration is not complete.

## Verification Commands

Backend:

```bash
cd backend
go test ./...
```

Frontend:

```bash
cd frontend
npm run build
```

Program through Docker:

```bash
docker compose --profile deploy run --rm solana-check
```

Compose syntax check:

```bash
docker compose config
```

## Troubleshooting

### Backend starts, but Solana actions are unavailable

Check `/api/health`. If `solana` is `false`, set both:

```env
SERVICE_PRIVATE_KEY=<service-wallet-private-key>
PROGRAM_ID=<deployed-program-id>
```

The backend intentionally disables real on-chain transaction endpoints when Solana is not configured.

### `JWT_SECRET` error in production

Production requires `JWT_SECRET` and it must be at least 32 characters.

### GitHub OAuth redirect mismatch

The GitHub OAuth App callback URL must match:

```text
<FRONTEND_URL>/auth/callback
```

For local Vite development, use:

```text
http://localhost:5173/auth/callback
```

### CORS errors

In development, the backend allows `http://localhost:3000` and `http://localhost:5173`.

In production, set `ALLOWED_ORIGINS` explicitly:

```env
ALLOWED_ORIGINS=https://your-domain.example
```

### Sponsor wallet has insufficient funds

Campaign creation requires reward pool, service fee, account rent, and network fee. The service fee is:

```text
max(0.5% of reward_pool, 0.05 SOL)
```

Fund the wallet on devnet and retry.

### Claim fails for an otherwise valid contributor

Check these conditions:

- Campaign must be finalized.
- The authenticated GitHub username must match the allocation contributor.
- The claim record must not already be claimed.
- The claim transaction must be confirmed on-chain before `claim-confirm` succeeds.
- The contributor wallet needs transaction-fee SOL in the current user-paid claim flow.

### Finalization enters manual review

The auto-finalize worker retries failed campaign-level finalization attempts and then marks campaigns as `needs_manual_review`. This does not automatically call the on-chain `close_unfinalizable_campaign` instruction in the current backend.
