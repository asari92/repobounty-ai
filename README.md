# Enshor

<p align="center">
  <img src="./frontend/public/brand/enshor-logo-wide.png" alt="Enshor" width="100%" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Solana-Frontier-9945FF" alt="Solana Frontier" />
  <img src="https://img.shields.io/badge/Colosseum-Hackathon-0F172A" alt="Colosseum Hackathon" />
  <img src="https://img.shields.io/badge/Solana-Devnet-14F195" alt="Solana Devnet" />
  <img src="https://img.shields.io/badge/Status-MVP-blue" alt="MVP" />
</p>

> Escrow-backed GitHub reward campaigns on Solana.  
> Sponsors lock SOL upfront, contributor reward rights are finalized on-chain, and contributors can claim later with the wallet they choose for that campaign.

<p align="center">
  <a href="https://enshor.duckdns.org/">Live App</a>
  ·
  <a href="https://youtu.be/oIxKo_K1Q0E">Video Walkthrough</a>
</p>

---

## Submission

Enshor is being prepared as a submission for the **Solana Frontier Hackathon by Colosseum**.

| Name | Role |
|------|------|
| Enshor Team | Product, backend, Solana MVP |

---

## Problem and Solution

### 1. Manual reward distribution is subjective
- **Problem:** Sponsors and maintainers often decide rewards manually, which makes the process inconsistent, opaque, and hard to verify.
- **Enshor:** Uses GitHub repository activity with backend allocation logic, including AI-assisted scoring with deterministic fallback, to generate a structured reward distribution flow.

### 2. Funding is often not locked upfront
- **Problem:** A sponsor can promise rewards without actually securing funds, which weakens trust for contributors.
- **Enshor:** Creates campaign-linked SOL escrow on Solana so campaign funding is committed on-chain from the start.

### 3. Contributors need transparent payout rights
- **Problem:** Even if contributors are selected fairly, they still need a reliable way to verify and claim rewards.
- **Enshor:** Finalizes allocations on-chain and allows contributors to claim later through a GitHub-authenticated flow plus wallet signing.

### 4. Open-source reward programs do not scale well
- **Problem:** Running reward campaigns across repositories becomes operationally heavy when review, accounting, and payout are all manual.
- **Enshor:** Combines GitHub identity, backend orchestration, allocation logic, and Solana claim bookkeeping into one deadline-based workflow.

---

## Why Solana

- **Speed:** campaign funding, finalization, and claims need fast confirmation for a usable contributor experience.
- **Low cost:** Enshor may require multiple on-chain actions per campaign, so transaction cost matters.
- **Deterministic state:** PDAs are a clean fit for campaigns, escrow vaults, and claim records.
- **Wallet UX:** sponsor funding and contributor claims work naturally with standard Solana wallet flows.

For Enshor, Solana is not an add-on. It is the execution layer that makes escrow-backed and verifiable contributor rewards practical.

---

## Summary of Features

- Sponsor-funded reward campaigns for public GitHub repositories
- SOL escrow and campaign state on Solana
- GitHub OAuth for authenticated user flows
- Allocation pipeline based on GitHub activity
- AI-assisted scoring with deterministic fallback
- Finalize-preview and finalize flows aligned with the same allocation logic
- Contributor claim flow after campaign finalization
- Backend + on-chain separation of responsibilities for MVP delivery

---

## What Ships in the MVP

Enshor currently supports this deadline-based flow:

1. Sponsor connects a Solana wallet.
2. Sponsor creates a campaign for a public GitHub repository.
3. Backend validates the repository and builds a create-with-deposit transaction.
4. Sponsor signs and sends the transaction.
5. After the deadline, allocations are calculated and finalized on-chain.
6. Contributors authenticate with GitHub, connect a wallet, and claim rewards.

The backend acts as the trust layer for GitHub identity, allocation orchestration, and claim transaction preparation. The Solana program enforces campaign state, escrow, deadlines, allocation bookkeeping, and claim tracking.

---

## Why Enshor Is Interesting

A key property of the MVP is that contributor reward rights are finalized by **GitHub user identity**, while the actual recipient wallet can be chosen later at claim time.

That means a contributor does **not** need to have a wallet when the campaign is created or finalized. The payout right exists first, and the wallet gets attached when the contributor is ready to claim.

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
Backend (Go API, GitHub OAuth/JWT, GitHub fetches, allocation logic, SQLite)
        |
        v
Solana Program (Anchor)
```

### On-chain responsibilities
- campaign account and status
- escrow PDA for locked reward funds
- claim records keyed by campaign and `github_user_id`
- service fee transfer
- deadline enforcement
- claim invariants
- refund invariants
- admin config and pause controls

### Off-chain responsibilities
- GitHub OAuth and session handling
- public repository validation
- repository data collection
- allocation logic
- transaction construction and orchestration
- UI state and off-chain campaign metadata

---

## Current Demo Path

Best live demo path:

1. Create a campaign for a small public GitHub repository.
2. Lock SOL on devnet at campaign creation.
3. Finalize allocations after the deadline.
4. Log in as an allocated GitHub user.
5. Connect a wallet and claim.
6. Show the claim state update after backend confirmation of the on-chain claim record.

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

Typical runtime values include:

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
MODEL=

FRONTEND_URL=http://localhost:5173
ALLOWED_ORIGINS=http://localhost:5173,http://localhost:3000
PORT=8080
DATABASE_PATH=/data/repobounty.db
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
docs/            supporting documentation
```

---

## Demo Resources

- Live App
- Video Walkthrough
- Devnet deployment
- Supporting docs in `docs/`

---

## One-Line Summary

**Enshor is an escrow-backed GitHub reward campaign MVP: sponsors lock SOL upfront, backend logic produces contributor allocations, and a Solana program enforces claim rights, double-claim protection, and delayed refunds.**
