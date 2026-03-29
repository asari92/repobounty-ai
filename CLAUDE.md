# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

RepoBounty AI — hackathon MVP for the National Solana Hackathon (Case 2: "AI + Blockchain: Autonomous Smart Contracts"). Sponsors fund public GitHub repos with a deadline; after the deadline, the system fetches contributor data, runs AI-based reward allocation, and finalizes on-chain via a Solana smart contract.

Key constraint: AI decisions must produce real on-chain state changes (not advisory-only).

## Build & Run Commands

### Full Stack (Docker)
```bash
docker compose up --build
# Frontend: http://localhost:5173  Backend: http://localhost:8080
```

### Backend (Go 1.25)
```bash
cd backend
go mod tidy
go run ./cmd/api          # Starts on :8080
go build -o main ./cmd/api
```

### Frontend (React 18 + Vite)
```bash
cd frontend
npm install
npm run dev               # Dev server on :3000, proxies /api to :8080
npm run build
npm run lint              # ESLint
npm run lint:fix
npm run format            # Prettier
npm run format:check
```

### Solana Program (Anchor 0.30.1)
```bash
cd program
yarn install
anchor build              # Use --no-idl for Docker builds
anchor test               # Runs on localnet
anchor deploy --provider.cluster devnet
```

## Architecture

```
Frontend (React + Solana wallet-adapter)
    → Backend (Go + Chi router)
        → GitHub API (contributor metrics + code diffs)
        → AI Engine (OpenRouter, multi-dimensional impact scoring)
    → Solana Program (Anchor)
        → On-chain campaign state + escrow vault
```

### Backend (`backend/`)
- **Entry point:** `cmd/api/main.go` — loads config, injects services, starts server + auto-finalize worker
- **Services in `internal/`:**
  - `config/` — env loading via godotenv with defaults
  - `models/` — shared types: Campaign, Allocation, Contributor, User
  - `store/sqlite.go` — SQLite persistent storage (fallback: in-memory)
  - `auth/` — GitHub OAuth, JWT (HS256), auth middleware
  - `github/client.go` — fetches contributors, PR diffs, reviews; adaptive selection; 5-concurrent limit
  - `ai/allocator.go` — OpenRouter LLM with code diff analysis or deterministic fallback
  - `solana/client.go` — builds/sends Solana transactions; mock mode when no private key
  - `http/` — Chi router, handlers, middleware (CORS, rate limiting, structured zap logging)
  - `http/worker.go` — auto-finalize background worker (scans campaigns past deadline)
- **Handler pattern:** `Handlers` struct holds all services; methods are endpoint handlers
- **Allocations use basis points:** percentages sum to 10000 (100%)
- **Dual authority model:** `authority` = backend key (finalizes), `sponsor` = user wallet (funds)

### Frontend (`frontend/`)
- React 18 + TypeScript + React Router v6 + Tailwind CSS (Solana theme colors)
- Wallet: `@solana/wallet-adapter` with Phantom on devnet
- GitHub OAuth login + JWT auth context (`hooks/useAuth.tsx`)
- API client in `src/api/client.ts` — fetch wrapper with JWT injection
- Pages: Home (campaign list, all/my filter by sponsor), CreateCampaign (2-step: create → fund), CampaignDetails (preview + finalize + claim), Profile (wallet linking + claims)

### Solana Program (`program/`)
- Program ID: `8oSXz4bbvUYVnNruhPEF3JR7jMsSApf7EpAyDpXxDLSJ`
- Campaign PDA seeds: `["campaign", campaign_id]`
- Vault PDA seeds: `["vault", campaign_pda]` — holds escrowed SOL
- State machine: Created → Funded → Finalized → Completed
- Instructions: `create_campaign`, `fund_campaign`, `finalize_campaign` (deadline enforced), `claim`
- Constraints: percentages sum to 10000 bps, max 10 allocations, deadline check on finalize

## API Endpoints

```
GET  /api/health                         — Health check
GET  /api/auth/github/url                — GitHub OAuth URL
POST /api/auth/github/callback           — OAuth code exchange → JWT
GET  /api/auth/me                        — Current user (protected)
POST /api/wallet/link                    — Link Solana wallet (protected)
GET  /api/claims                         — List claimable allocations (protected)
GET  /api/campaigns                      — List campaigns
POST /api/campaigns                      — Create campaign
GET  /api/campaigns/{id}                 — Get campaign
POST /api/campaigns/{id}/finalize-preview — AI preview (no on-chain)
POST /api/campaigns/{id}/finalize        — AI allocate + Solana finalize
POST /api/campaigns/{id}/claim           — Claim allocation (protected)
POST /api/campaigns/{id}/claim-permit    — Claim permit (protected)
POST /api/campaigns/{id}/fund-tx         — Get funding transaction
```

## Environment Variables

See `backend/.env.example`. The backend works without external keys using mock data and deterministic allocation:
- `PORT` (default 8080), `GITHUB_TOKEN`, `OPENROUTER_API_KEY`, `MODEL` (default: nvidia/nemotron free tier)
- `SOLANA_RPC_URL` (default devnet), `SOLANA_PRIVATE_KEY`, `PROGRAM_ID`
- `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `JWT_SECRET`, `FRONTEND_URL`
- `DATABASE_PATH` (default: in-memory, set path for SQLite persistence)

## Scope

**Implemented:** public repos, deadline-based campaigns, escrow funding (vault PDA), AI allocation with code diff analysis, on-chain finalization with deadline enforcement, GitHub OAuth + wallet linking, contributor claim flow, auto-finalize worker, SQLite persistence.

**Not in scope:** goal-based campaigns, GitHub App integration (PR comments), anti-fraud, notifications, multi-sponsor campaigns.
