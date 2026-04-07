# Enshor

![Solana](https://img.shields.io/badge/Solana-MVP-9945FF) ![Hackathon](https://img.shields.io/badge/Hackathon-National%20Solana%20Hackathon-blue)

> AI-powered reward allocation for open-source contributors, with SOL escrow and transparent claims on Solana.

Demo · Video Walkthrough · [Docs](./docs) · Hackathon Submission

---

## Submission to National Solana Hackathon

| Name | Role |
|------|------|
| Enshor Team | Product, backend, Solana MVP |

---

## Problem and Solution

### 1. Manual reward distribution is subjective
- Problem: Sponsors and maintainers often decide rewards manually, which makes the process inconsistent, opaque, and hard to verify.
- Enshor: Uses GitHub repository activity and AI-assisted allocation logic to generate a structured reward distribution flow.

### 2. Funding is often not locked upfront
- Problem: A sponsor can promise rewards without actually securing funds, which weakens trust for contributors.
- Enshor: Creates campaign-linked SOL escrow on Solana so campaign funding is committed on-chain.

### 3. Contributors need transparent payout rights
- Problem: Even if contributors are selected fairly, they still need a reliable way to verify and claim rewards.
- Enshor: Finalizes allocations on-chain and allows contributors to claim later through GitHub-authenticated flow plus wallet binding.

### 4. Open-source reward programs do not scale well
- Problem: Running reward campaigns across repositories becomes operationally heavy when review, accounting, and payout are all manual.
- Enshor: Combines GitHub data, backend automation, AI allocation, and Solana claim bookkeeping into one deadline-based workflow.

---

## Why Solana

- **Speed**: campaign funding, finalization, and claims need fast confirmation for a usable contributor experience.
- **Low cost**: Enshor can involve multiple on-chain actions per campaign, so low transaction cost matters.
- **Deterministic state**: PDAs are a clean fit for campaigns, escrow vaults, and claim records.
- **Wallet UX**: sponsor funding and contributor claims work naturally with standard Solana wallet flows.

For Enshor, Solana is not an add-on. It is the execution layer that makes escrow-backed and verifiable contributor rewards practical.

---

## Summary of Features

- Sponsor-funded reward campaigns for public GitHub repositories
- SOL escrow and campaign state on Solana
- GitHub OAuth for authenticated user flows
- AI-assisted allocation based on repository activity
- Finalize-preview and finalize flows aligned with the same logic
- Contributor claim flow after campaign finalization
- Deterministic fallback when GitHub or AI dependencies are limited
- Backend + on-chain separation of responsibilities for MVP delivery

---

## What Ships in the MVP

Enshor currently supports this deadline-based flow:

1. Sponsor connects a Solana wallet.
2. Sponsor creates a campaign for a public GitHub repository.
3. Backend validates input and prepares funding flow.
4. Sponsor signs and sends the funding transaction.
5. After the deadline, allocations are finalized automatically. The frontend can also trigger finalization manually as an additional fallback action.
6. Contributors authenticate with GitHub, connect a wallet, and claim rewards.

The backend is the trust anchor for GitHub identity, allocation logic, and claim authorization. The Solana program enforces campaign state, escrow, deadlines, allocation bookkeeping, and claim tracking.

---

## Tech Stack

| Layer | Technology |
|------|------------|
| Frontend | React 18, Vite, TypeScript, Tailwind |
| Wallet | `@solana/wallet-adapter` |
| Backend | Go, Chi, Zap |
| Auth | GitHub OAuth + JWT |
| Storage | SQLite / in-memory |
| AI | OpenRouter or deterministic fallback |
| Blockchain | Solana + Anchor |

---

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

See `docs/architecture.md` for a fuller component breakdown.

---

## Current Permission Model

- `POST /api/campaigns/` - wallet flow, no GitHub auth required
- `POST /api/campaigns/{id}/create-confirm` - wallet flow, no GitHub auth required
- `POST /api/campaigns/{id}/refund` - wallet flow, no GitHub auth required
- `POST /api/campaigns/{id}/refund-confirm` - wallet flow, no GitHub auth required
- `POST /api/campaigns/{id}/fund-tx` - wallet flow, no GitHub auth required
- `POST /api/campaigns/{id}/finalize-preview` - GitHub auth required, owner only
- `POST /api/campaigns/{id}/finalize` - GitHub auth required, owner only
- `POST /api/campaigns/{id}/claim-challenge` - GitHub auth required
- `POST /api/campaigns/{id}/claim-permit` - GitHub auth required
- `POST /api/campaigns/{id}/claim-confirm` - GitHub auth required
- `GET /api/auth/my-campaigns` - GitHub auth required

---

## Mock and Fallback Behavior

Enshor includes two separate fallback modes:

- **GitHub / AI fallback**: when tokens or model access are limited, the backend can still operate with public GitHub access, mock contributor data, or deterministic allocation.
- **Solana fallback**: if required Solana authority configuration is missing, the API can still boot, but on-chain create, fund, finalize, and claim actions are disabled instead of returning fake transactions.

Use `GET /api/health` to inspect current runtime readiness.

---

## Quick Start

### Prerequisites

- Go 1.25+
- Node.js 20+
- Docker and Docker Compose
- Solana wallet such as Phantom
- `anchor` and `solana-cli` for local program work

### Clone the repository

```bash
git clone https://github.com/asari92/enshor.git
cd enshor
```

### Environment

Copy and configure the root environment file:

```bash
cp .env.example .env
```

Example values:

```env
JWT_SECRET=change-me-to-a-random-string-at-least-32-chars

GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret
GITHUB_TOKEN=your_github_token

SOLANA_RPC_URL=https://api.devnet.solana.com
SERVICE_PRIVATE_KEY=
PROGRAM_ID=your_program_id
SOLANA_DEPLOY_WALLET=

OPENROUTER_API_KEY=your_openrouter_api_key
MODEL=nvidia/nemotron-3-super-120b-a12b:free

FRONTEND_URL=http://localhost:5173
ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000
PORT=8080
DATABASE_PATH=/data/enshor.db
```

Frontend example:

```env
VITE_API_BASE_URL=http://localhost:8080
VITE_SOLANA_CLUSTER=devnet
VITE_APP_NAME=Enshor
```

### Run with Docker

```bash
docker compose up --build
```

Then open:

- Frontend: `http://localhost:5173`
- Backend API: `http://localhost:8080`

### Contract already deployed

If you already have a deployed program, set `PROGRAM_ID` in `.env` and run:

```bash
./start.sh
```

### Local build, test, and deploy check

```bash
docker compose --profile deploy run --rm solana-check
```

This validation flow can:
- run `anchor build`
- start `solana-test-validator`
- run `anchor deploy --provider.cluster localnet`
- run TypeScript integration tests
- avoid deploying to devnet

### Real devnet deploy

```bash
docker compose --profile deploy run --rm solana-deployer
```

After deploy, copy the generated program id into `PROGRAM_ID` in `.env`.

---

## Repository Structure

```text
frontend/        React app and wallet UI
backend/         Go API, auth, allocation, storage
program/         Solana program and tests
docs/            architecture, roadmap, API, demo notes
```

---

## Roadmap

- Complete end-to-end claim flow hardening
- Improve wallet binding and authorization guarantees
- Add stronger finalization and recovery flows
- Support richer contributor scoring inputs
- Expand analytics and campaign observability
- Prepare for production-grade dispute and review mechanics

Full roadmap: `docs/roadmap.md`

---

## Resources

- Project Presentation
- Video Demo
- Demo Script
- Architecture Notes
