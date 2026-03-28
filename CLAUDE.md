# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

RepoBounty AI — a hackathon MVP for the National Solana Hackathon (Case 2: "AI + Blockchain: Autonomous Smart Contracts"). Sponsors fund public GitHub repos with a deadline; after the deadline, the system fetches contributor data, runs AI-based reward allocation, and finalizes on-chain via a Solana smart contract.

## Architecture

Four components connected in a pipeline:

1. **Frontend** (TypeScript) — campaign creation UI, results display, wallet connection
2. **Backend API** (Go) — orchestration layer with services: campaign, github, ai allocation, solana
3. **AI Allocation Engine** — receives normalized contributor metrics (username, commits, PRs), returns structured JSON allocation with percentages and reasoning
4. **Solana Program** (Rust + Anchor) — on-chain state: campaign creation, finalization, contributor allocation storage

Data flow: `Frontend → Backend → GitHub API` + `Backend → AI Engine` → `Backend → Solana Program → on-chain state` → `Frontend displays results`

Key constraint: AI decisions must produce real on-chain state changes (not advisory-only).

## Repository Layout

- `backend/` — Go API server (`cmd/api/main.go` entry point)
- `frontend/` — TypeScript web interface (not yet scaffolded)
- `program/` — Solana/Anchor smart contract (not yet scaffolded)
- `docs/` — architecture, roadmap, demo script

## Build & Run Commands

### Backend
```bash
cd backend
go mod tidy
go run ./cmd/api
```

### Frontend (once scaffolded)
```bash
cd frontend
npm install
npm run dev
```

### Solana Program (once scaffolded)
```bash
cd program
anchor build
anchor test
```

## Environment Variables

See `backend/.env.example`:
- `PORT` — API server port (default 8080)
- `GITHUB_TOKEN` — GitHub API access
- `OPENAI_API_KEY` — AI allocation engine
- `SOLANA_RPC_URL` — Solana cluster endpoint
- `SOLANA_PRIVATE_KEY` — signing key for on-chain transactions
- `PROGRAM_ID` — deployed Solana program address

## MVP Scope Boundaries

**In scope:** public repos, deadline-based campaigns, AI allocation, on-chain finalization.

**Out of scope:** goal-based campaigns, GitHub App integration, wallet binding, anti-fraud, claim flow, notifications.
