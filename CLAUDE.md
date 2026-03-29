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
        → GitHub API (contributor metrics)
        → AI Engine (OpenRouter, configurable model)
    → Solana Program (Anchor)
        → On-chain campaign state
```

### Backend (`backend/`)
- **Entry point:** `cmd/api/main.go` — loads config, injects services, starts server with graceful shutdown
- **Services in `internal/`:**
  - `config/` — env loading via godotenv with defaults
  - `models/` — shared types: Campaign, Allocation, Contributor
  - `store/memory.go` — thread-safe in-memory campaign cache (RWMutex)
  - `github/client.go` — fetches contributors + PRs; falls back to mock data on failure; retries 3x
  - `ai/allocator.go` — OpenRouter LLM allocation or deterministic fallback (weighted: commits×3 + PRs×5 + reviews×2)
  - `solana/client.go` — builds/sends Solana transactions; mock mode when no private key
  - `http/` — Chi router, handlers, middleware (CORS, rate limiting, structured zap logging)
- **Handler pattern:** `Handlers` struct holds all services; methods are endpoint handlers
- **Allocations use basis points:** percentages sum to 10000 (100%)

### Frontend (`frontend/`)
- React 18 + TypeScript + React Router v6 + Tailwind CSS (Solana theme colors)
- Wallet: `@solana/wallet-adapter` with Phantom on devnet
- API client in `src/api/client.ts` — fetch wrapper
- Pages: Home (campaign list with all/my filter), CreateCampaign, CampaignDetails (preview + finalize)
- Vite dev server proxies `/api` to backend at localhost:8080

### Solana Program (`program/`)
- Program ID: `8oSXz4bbvUYVnNruhPEF3JR7jMsSApf7EpAyDpXxDLSJ`
- Campaign PDA seeds: `["campaign", authority, campaign_id]`
- Two instructions: `create_campaign` (init state) and `finalize_campaign` (store allocations)
- Constraints: percentages must sum to 10000 bps, max 10 allocations, no duplicate contributors
- Tests in `tests/repobounty.ts` using ts-mocha + Chai

## API Endpoints

```
GET  /api/health                         — Health check
GET  /api/campaigns                      — List campaigns
POST /api/campaigns                      — Create campaign
GET  /api/campaigns/{id}                 — Get campaign
POST /api/campaigns/{id}/finalize-preview — AI preview (no on-chain)
POST /api/campaigns/{id}/finalize        — AI allocate + Solana finalize
```

## Environment Variables

See `backend/.env.example`. The backend works without external keys using mock data and deterministic allocation:
- `PORT` (default 8080), `GITHUB_TOKEN`, `OPENROUTER_API_KEY`, `MODEL` (default: nvidia/nemotron free tier)
- `SOLANA_RPC_URL` (default devnet), `SOLANA_PRIVATE_KEY`, `PROGRAM_ID`

## MVP Scope Boundaries

**In scope:** public repos, deadline-based campaigns, AI allocation, on-chain finalization.

**Out of scope:** goal-based campaigns, GitHub App integration, wallet binding, anti-fraud, claim flow, notifications.
