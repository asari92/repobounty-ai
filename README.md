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

1. A sponsor logs in with GitHub and connects a Solana wallet.
2. The backend authority creates a campaign on-chain and stores the GitHub creator off-chain.
3. The sponsor signs a funding transaction from the connected wallet.
4. After the deadline, the backend fetches GitHub contributor data and PR diffs when available.
5. The backend computes allocations off-chain, then finalizes them on-chain.
6. Contributors log in with GitHub and claim to the wallet they currently have connected.

The backend is the trust anchor for GitHub auth, repo validation, allocation logic, and claim authorization. The on-chain program enforces campaign state, deadline, escrow, percentages, and claim bookkeeping.

## Current Permission Model

- `POST /api/campaigns/` requires GitHub auth.
- The authenticated GitHub user who creates a campaign becomes the stored campaign owner.
- Manual `fund-tx`, `finalize-preview`, and `finalize` actions are limited to that stored owner.
- Auto-finalize still runs in the backend worker after the deadline.
- Claims require GitHub auth, but they send SOL to the wallet currently connected in the UI.

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

## How To Run

### Prerequisites

Before running RepoBounty AI locally, make sure you have:

- Go 1.22+ installed
- Node.js 20+ installed
- Docker and Docker Compose installed
- A Solana wallet such as Phantom
- `anchor` and `solana-cli` if you want to run the program locally

### Environment Variables

Create `.env` files for the backend and frontend if needed.

#### Backend example
```bash
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret
JWT_SECRET=your_jwt_secret
DATABASE_URL=./data/app.db

# Optional
GITHUB_TOKEN=your_github_token
OPENROUTER_API_KEY=your_openrouter_api_key

# Solana
SOLANA_RPC_URL=https://api.devnet.solana.com
SOLANA_PROGRAM_ID=your_program_id
SOLANA_AUTHORITY_KEYPAIR=./keys/authority.json
```

#### Frontend example
```bash
VITE_API_BASE_URL=http://localhost:8080
VITE_SOLANA_CLUSTER=devnet
```

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
