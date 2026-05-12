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

> Transparent funding for open-source contributors.  
> Sponsors lock SOL upfront, reward rights are finalized on-chain, and contributors claim their share through a verifiable, deadline-driven flow.

<p align="center">
  <a href="https://enshor.duckdns.org/">Live App</a>
  ·
  <a href="https://www.youtube.com/watch?v=RNdNvIYouGk">Video Walkthrough</a>
</p>

---

## Submission

Enshor is a submission to the **Solana Frontier Hackathon by Colosseum**.

| Name | Role |
|------|------|
| Enshor Team | Product, backend, Solana MVP |

---

## The Problem

Open-source produces enormous value, but the way contributors get paid still lags behind the work itself. Funding is informal, payout decisions are hard to defend, and the whole process leaks trust at every step:

- rewards get promised but rarely locked,
- contribution-based payouts are hard to justify and easy to dispute,
- contributors may not have a wallet — or even know they will be paid — when the work happens,
- maintainers and sponsors end up running payout logistics by hand.

The result is a funding layer that depends on goodwill and spreadsheets rather than verifiable commitments.

## The Approach

Enshor turns reward distribution into a structured, on-chain flow:

1. a sponsor opens a campaign for a public repository,
2. the reward pool is locked on-chain in SOL at creation,
3. after the deadline, contributor reward rights are finalized on-chain from contribution data,
4. contributors authenticate and claim their share with any wallet they choose,
5. any unclaimed funds can be returned to the sponsor after the claim window closes.

The funding commitment, finalized reward rights, and payout results are all recorded on-chain.

---

## Why Open Source First

Open-source repositories are the cleanest starting point for contribution-based funding. The work is public, the history is inspectable, and "who contributed what" is a question that can be answered with data instead of opinion.

Open source also has a sustainability problem: the projects that the rest of the industry depends on are largely maintained by people whose compensation does not match the value of the work. Funding exists — through sponsorships, grants, and corporate budgets — but it rarely reaches the contributors who actually move a project forward, and when it does, the path is manual and opaque.

Enshor is designed to help open-source teams move funding out of private chats and ad-hoc transfers into something legible:

- funding is committed up front, not promised,
- contributor reward rights become fixed on-chain after the deadline,
- payout happens through a verifiable claim,
- sponsor refunds are delayed until the claim window ends, so the commitment is real.

The same primitives generalize to bounties, grants, and contribution-weighted payouts beyond OSS, but open source is where transparency matters most and where it is easiest to deliver.

---

## What Enshor Changes in Open-Source Contribution

Enshor is built around a simple bet: making the funding side of open-source contribution as legible and durable as the code itself will let more people sustain the work. To do that, contribution has to be a first-class object — visible, attributable, and rewardable — without forcing the people doing the work to also run the payout logistics.

### For contributors

- Work is recognized through a structured allocation grounded in repository data, not a private decision.
- No wallet is required at the time of contribution. The right to a reward exists first; the wallet attaches when the contributor is ready to claim.
- Claims are self-serve: a contributor authenticates with GitHub, sees what is owed, and claims when convenient. No DMs, no "send me your address" threads.
- Every claim leaves an on-chain record, building a verifiable trail of paid contribution history that lives outside any single platform.

### For maintainers

- Reward decisions move from spreadsheets and side channels into a transparent process anchored on actual contribution data.
- Allocation logic is consistent across campaigns, with AI-assisted scoring and a deterministic fallback for reproducibility.
- The operational tax of running a payout — collecting wallets, chasing signatures, splitting transfers, handling no-shows — is replaced by a single deadline-driven flow.
- Recognition becomes routine rather than ceremonial: campaigns can recur, and contributors who do not respond to one campaign are not lost from future ones.

### For sponsors

- Funding is committed up front and locked on-chain, not promised in a Discord channel.
- The reward pool, service fee, and refund window are all enforced by the program, not by trust in the recipient.
- Unclaimed balances can be returned to the sponsor after the claim deadline, so commitments do not become open-ended liabilities.
- Sponsorship leaves a public, structured record — a useful signal for companies that want to demonstrate real support for the projects they depend on.

### For the ecosystem

- Contribution and payout share the same ledger as the original commitment, so the question "did anyone actually get paid for this work?" stops being a matter of taking someone's word.
- Over time, finalized campaigns and claim history could become a useful public signal of paid contribution history — portable, verifiable, and independent of any single platform's metrics.

---

## Why Solana

Solana is the execution layer that makes this MVP practical:

- campaign funds are locked on-chain at creation,
- reward pool and service fee are separated cleanly,
- finalized claim rights are stored per contributor,
- claims confirm fast and at low cost,
- double claims are prevented by the program,
- refund timing is enforced on-chain rather than by a backend cron.

A defining property of Enshor: reward rights are finalized by `github_user_id`, while the payout wallet is chosen later at claim time. A contributor does not need a wallet — or even an account — at the moment funding is committed. The right exists first; the wallet attaches when the contributor is ready.

---

## Current MVP Flow

The repository implements this end-to-end path:

1. Sponsor creates a campaign for a public repository.
2. Backend validates the repository and builds a Solana create-with-deposit transaction.
3. Sponsor signs the transaction in the frontend.
4. Backend confirms the on-chain campaign and stores off-chain metadata.
5. After the deadline, the backend computes allocations from contribution data using AI-assisted scoring with deterministic fallback.
6. Backend finalizes allocations on-chain by creating claim records.
7. A contributor authenticates, sees available claims in their profile, connects a wallet, and requests a claim transaction.
8. The contributor signs and submits the claim transaction.
9. Backend confirms the claim against on-chain claim-record state.
10. After the claim deadline, the sponsor can refund any unclaimed balance.

---

## Core Properties

- sponsor-funded reward campaigns for public repositories
- SOL escrow at campaign creation
- separated reward pool and service fee
- contributor reward rights fixed on-chain after finalization
- claim records keyed by contributor identity, not by wallet
- GitHub-authenticated claim flow
- on-chain double-claim protection
- delayed sponsor refund window
- AI-assisted allocation with deterministic fallback
- full create → finalize → claim path on devnet

---

## Architecture

```text
Frontend (React + Wallet Adapter)
        |
        v
Backend (Go API, OAuth/JWT, contribution fetches, allocation logic, SQLite)
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
- repository validation
- contributor identity and authentication
- contribution data collection
- allocation logic
- transaction construction and orchestration
- UI state and off-chain campaign metadata

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

## Demo Path

Best live demo path:

1. Create a campaign for a small public repository.
2. Lock SOL on devnet at campaign creation.
3. Finalize allocations after the deadline.
4. Log in as an allocated contributor.
5. Show available claims in profile.
6. Connect a wallet and claim.
7. Show the claim state update after backend confirmation of the on-chain claim record.

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

Typical runtime values:

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

### Local build, test, and deploy check

```bash
docker compose --profile deploy run --rm solana-check
```

### Devnet deploy

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

## One-Line Summary

**Enshor brings transparent funding to open-source contributors: sponsors lock SOL upfront, reward rights are finalized on-chain, and payouts happen through a verifiable claim flow.**