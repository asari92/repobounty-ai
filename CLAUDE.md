# CLAUDE.md

This file provides guidance to coding agents working with this repository.

## Project

RepoBounty AI is a hackathon MVP for the National Solana Hackathon, Case 2: "AI + Blockchain: Autonomous Smart Contracts".

The product funds public GitHub repositories with deadline-based campaigns, computes contributor reward allocations with AI, and writes real reward state to Solana.

## Current Implementation

Today the repository contains:

1. **Frontend** (TypeScript + React) — campaign creation UI, campaign list, campaign details, wallet connection
2. **Backend API** (Go) — orchestration layer for campaigns, GitHub data, AI allocation, Solana interaction
3. **AI Allocation Engine** — structured allocation output from contributor metrics, with deterministic fallback
4. **Solana Program** (Rust + Anchor) — on-chain campaign creation and finalization state

Current implemented flow:

`Frontend -> Backend -> GitHub API`

`Frontend -> Backend -> AI Engine`

`Backend -> Solana Program -> on-chain campaign state`

## Agreed Next Revision

The team has agreed on this target architecture:

- sponsor wallet should be the on-chain campaign `authority`
- campaign creation should be signed by the sponsor wallet
- backend should use a dedicated trusted finalizer key after the deadline
- campaign funds should move into escrow
- finalization should store GitHub-based entitlements first, because contributor wallets may be unknown
- contributors should later authenticate with GitHub, bind wallets, and claim or receive released rewards

## Repository Layout

- `backend/` — Go API server (`cmd/api/main.go` entry point)
- `frontend/` — TypeScript web interface
- `program/` — Solana/Anchor smart contract
- `docs/` — architecture, API, roadmap, demo notes

## Build & Run Commands

### Backend

```bash
cd backend
go mod tidy
go run ./cmd/api
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

### Solana Program

```bash
cd program
anchor build
anchor test
```

## Environment Variables

See `backend/.env.example`:

- `PORT` — API server port
- `GITHUB_TOKEN` — GitHub API access
- `OPENROUTER_API_KEY` — AI allocation engine
- `MODEL` — AI model name
- `SOLANA_RPC_URL` — Solana cluster endpoint
- `SOLANA_PRIVATE_KEY` — backend Solana signing key in the current MVP
- `PROGRAM_ID` — deployed Solana program address

## Scope Guidance

### Implemented now

- public repos
- deadline-based campaigns
- campaign creation UI
- AI allocation
- on-chain finalization
- on-chain campaign listing
- `All Campaigns` / `My Campaigns` filtering

### Planned next

- sponsor-owned on-chain authority
- escrow funding
- GitHub login
- wallet binding
- GitHub-based claims / reward release

### Not planned right now

- goal-based campaigns
- anti-fraud
- notifications
- multi-sponsor campaigns
