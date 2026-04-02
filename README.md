# RepoBounty AI

[![Solana](https://img.shields.io/badge/Solana-devnet-9945FF)](https://solana.com)
[![GitHub](https://img.shields.io/badge/GitHub-OAuth-181717)](https://github.com)

> AI-powered funding and reward allocation for public GitHub repositories on Solana.

---

## Team

| Name | Role | Contact |
|------|------|---------|
||||

---

## Problem and Solution

### 1. Manual Reward Distribution
- **Problem:** Open-source contributors do valuable work, but rewards are often distributed manually, subjectively, and slowly.
- **RepoBounty AI:** Automatically analyzes GitHub activity and allocates rewards based on contributor impact.

### 2. Lack of Transparency
- **Problem:** Sponsors and contributors have limited visibility into how rewards are calculated.
- **RepoBounty AI:** Uses deterministic allocation logic and on-chain state to make the reward flow auditable and verifiable.

### 3. Fragmented Funding Flow
- **Problem:** Funding, campaign management, and claims are usually disconnected from the actual contribution process.
- **RepoBounty AI:** Combines GitHub auth, campaign funding, on-chain escrow, and contributor claims into one flow.

### 4. High Friction for Small Rewards
- **Problem:** Small contributor rewards are hard to manage efficiently when each payout requires manual handling.
- **RepoBounty AI:** Uses Solana for fast, low-cost escrow, finalization, and claims.

---

## Why Solana

- **Speed** — Low-latency transactions make campaign creation, funding, finalization, and claims practical.
- **Cost** — Low fees are ideal for many small reward payouts.
- **Escrow** — On-chain campaign state and SOL escrow keep funds transparent and verifiable.
- **Wallet UX** — Phantom and `@solana/wallet-adapter` make sponsor funding and contributor claims simple.
- **Determinism** — PDAs help model campaigns and claim records cleanly without deploying a new contract per campaign.

---

## Summary of Features

- Sponsor-funded campaigns for open-source repositories
- GitHub OAuth login for campaign owners and contributors
- AI-assisted reward allocation based on GitHub activity
- SOL escrow and on-chain campaign state on Solana
- Funding, finalization, and claim flows
- Deterministic fallback when external services are unavailable
- Preview and finalize flows aligned with the same allocation logic

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| On-chain program | Rust · Anchor Framework |
| Backend API | Go |
| Frontend | TypeScript · React |
| Wallet Integration | `@solana/wallet-adapter` |
| AI / Allocation | OpenRouter / deterministic fallback |
| Storage | SQLite |
| Auth | GitHub OAuth |

---

## Architecture

```text
┌───────────────┐     ┌────────────────────┐     ┌──────────────────────┐
│ GitHub User   │────▶│   Backend (Go)    │────▶│  Solana Program      │
│ Sponsor/Claim │     │ GitHub OAuth/JWT   │     │  (Escrow + State)    │
└───────────���───┘     │ GitHub Fetches     │     └──────────────────────┘
                      │ AI Allocation      │
                      │ SQLite Storage     │
                      └─────────┬──────────┘
                                │
                                ▼
                      ┌────────────────────┐
                      │ Frontend (React)   │
                      │ Wallet Connection  │
                      └────────────────────┘
```

See the project docs for more details on campaign lifecycle, claims, and allocation logic.

---

## What Ships Today

RepoBounty AI is a deadline-based MVP:

1. A sponsor logs in with GitHub and connects a Solana wallet.
2. The backend authority creates a campaign on-chain and stores the GitHub creator off-chain.
3. The sponsor signs a funding transaction from the connected wallet.
4. After the deadline, the backend fetches GitHub contributor data and PR diffs when available.
5. The backend computes allocations off-chain, then finalizes them on-chain.
6. Contributors log in with GitHub and claim to the wallet they currently have connected.

The backend is the trust anchor for GitHub auth, repo validation, allocation logic, and claim authorization. The on-chain program enforces campaign state, deadline, escrow, percentages, and claim bookkeeping.

---

## Current Permission Model

- `POST /api/campaigns/` requires GitHub auth.
- The authenticated GitHub user who creates a campaign becomes the stored campaign owner.
- Manual `fund-tx`, `finalize-preview`, and `finalize` actions are limited to that stored owner.
- Auto-finalize still runs in the backend worker after the deadline.
- Claims require GitHub auth, but they send SOL to the wallet currently connected in the UI.

Saved profile wallets are not authoritative in this MVP. They are informational only unless a stricter wallet-linking flow is implemented end to end.

---

## Finalize Preview Behavior

Finalize preview is intentionally aligned with real finalization:

- It is only available for funded campaigns after the deadline.
- It uses the same backend allocation path as real finalization.
- When PR diffs are available, preview and finalize both use PR-diff impact scoring.
- When PR diffs are unavailable, both fall back to contributor-metric allocation.

---

## Mock and Fallback Behavior

Two different fallback modes exist:

- **GitHub / AI fallback:** Without `GITHUB_TOKEN` or `OPENROUTER_API_KEY`, the backend can still use public GitHub access, mock contributor data, and deterministic allocation.
- **Solana fallback:** If the backend is missing a real Solana authority key or program ID, the API still boots, but on-chain create, fund, finalize, and claim actions are disabled instead of returning fake transactions.

Check `GET /api/health` to see whether Solana is currently configured.

---

## Quick Start

**Prerequisites:** Go 1.22+, Node.js 20+, Docker, Docker Compose, Solana CLI, Anchor CLI

```bash
# Clone the repository
git clone https://github.com/asari92/repobounty-ai
cd repobounty-ai

# Install dependencies
# Frontend
cd frontend && npm install

# Backend
cd ../backend && go mod tidy

# Copy environment variables
cp .env.example .env

# Build Solana program
cd ../program && anchor build

# Run tests
anchor test

# Start backend
cd ../backend && go run ./cmd/api

# Start frontend
cd ../frontend && npm run dev
```

---

## How To Run

### Run with Docker

```bash
docker compose up --build
```

Then open:

- Frontend: http://localhost:5173
- Backend API: http://localhost:8080

### Run Backend Locally

```bash
cd backend
go mod tidy
go run ./cmd/api
```

### Run Frontend Locally

```bash
cd frontend
npm install
npm run dev
```

### Run Solana Program Locally

```bash
cd program
yarn install
anchor build
anchor test
```

### Health Check

```bash
GET /api/health
```

This will show whether GitHub, AI, and Solana components are configured correctly.

---

## Roadmap

- [x] GitHub OAuth campaign creation
- [x] On-chain SOL escrow and campaign state
- [x] Funding and claim flows
- [x] AI-assisted allocation
- [x] Deterministic fallback behavior
- [ ] More advanced contribution scoring
- [ ] Multi-campaign sponsor dashboards
- [ ] Better wallet-linking and claimant identity checks
- [ ] Expanded analytics and reward history

---

## Resources

- [Live Demo](#)
- [Docs](docs/)
- [GitHub Repository](https://github.com/asari92/repobounty-ai)

---

## License

MIT — see [LICENSE](LICENSE)
