# RepoBounty AI

**AI-powered reward allocation for open-source contributors, finalized on Solana.**

> National Solana Hackathon (Decentrathon) | Case 2: "AI + Blockchain: Autonomous Smart Contracts"

---

## The Idea

Sponsors fund public GitHub repositories with a reward pool and a deadline. After the deadline, the system fetches contributor activity, runs AI-based reward allocation, and sends the final allocation on-chain to a Solana smart contract.

**The core chain that must work:**

```
GitHub data --> AI allocation --> Solana transaction --> on-chain state change
```

AI is not advisory — its decision directly creates on-chain state.

---

## Architecture

```
+------------------+      +------------------+      +------------------+
|                  |      |                  |      |                  |
|    Frontend      +----->+   Go Backend     +----->+  Solana Program  |
|  React + Wallet  |      |   (Orchestrator) |      |  (Anchor/Rust)   |
|                  |      |                  |      |                  |
+------------------+      +--------+---------+      +------------------+
                                   |
                          +--------+---------+
                          |                  |
                    +-----+-----+     +------+------+
                    |  GitHub   |     |   OpenAI    |
                    |  API      |     |   (GPT-4o)  |
                    +-----------+     +-------------+
```

**Data flow:**

1. Sponsor connects wallet, creates campaign via frontend
2. Backend validates and calls `create_campaign` on Solana
3. After deadline: backend fetches GitHub contributor stats
4. Backend sends metrics to AI, receives allocation with reasoning
5. Backend calls `finalize_campaign` on Solana with allocation data
6. Smart contract validates and stores allocations on-chain
7. Frontend displays final results with Solana Explorer links

---

## Tech Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Frontend** | React 18 + TypeScript + Vite | Campaign UI, wallet connection |
| **Styling** | Tailwind CSS | Solana-themed dark UI |
| **Wallet** | @solana/wallet-adapter | Phantom, Solflare support |
| **Backend** | Go 1.22 + Chi router | REST API, orchestration |
| **AI Engine** | OpenAI GPT-4o-mini | Contributor reward allocation |
| **AI Fallback** | Deterministic weighted scoring | Works without API key |
| **Blockchain** | Solana (devnet) + Anchor 0.30 | On-chain campaign state |
| **Smart Contract** | Rust + Anchor framework | PDA-based campaign accounts |

---

## Quick Start

### Docker Quick Start

The simplest way to run the full MVP locally is with Docker Compose:

```bash
docker compose up --build
```

Services:

- Frontend: `http://localhost:5173`
- Backend API: `http://localhost:8080`
- Backend health check: `http://localhost:8080/api/health`

Notes:

- Docker Compose reads configuration from the repository root `.env`
- `solana-program` is built as part of the stack, but it is not a long-running API service
- the Solana program image contains the compiled program artifacts only

### Manual Local Start

### Prerequisites

- **Go** >= 1.22
- **Node.js** >= 18
- **Rust** + **Anchor CLI** >= 0.30 (for smart contract)
- **Solana CLI** (for devnet interaction)

### 1. Backend

```bash
cd backend
cp .env.example .env        # Edit with your keys
go mod tidy
go run ./cmd/api
```

The server starts on `http://localhost:8080`. Works without any API keys (uses mock GitHub data and deterministic AI fallback).

### 2. Frontend

```bash
cd frontend
npm install
npm run dev
```

Opens on `http://localhost:3000`. Proxies `/api` requests to backend.

### 3. Solana Program

```bash
cd program
yarn install
anchor build
anchor test                  # Runs on localnet
anchor deploy --provider.cluster devnet
```

After deploying, update `PROGRAM_ID` in `backend/.env`.

For the Docker build we use `anchor build --no-idl` inside the container to avoid an Anchor IDL-generation issue in this environment. This does not change how the backend or frontend are started, and it does not mean the project can only run via Docker.

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PORT` | No | `8080` | Backend API port |
| `GITHUB_TOKEN` | No | — | GitHub personal access token (higher rate limits) |
| `OPENAI_API_KEY` | No | — | OpenAI API key (falls back to deterministic allocation) |
| `SOLANA_RPC_URL` | No | `https://api.devnet.solana.com` | Solana RPC endpoint |
| `SOLANA_PRIVATE_KEY` | No | — | Base58 private key for signing transactions |
| `PROGRAM_ID` | No | — | Deployed Anchor program address |

**Without any keys configured**, the backend still works using:
- Mock GitHub contributor data (3 demo contributors)
- Deterministic allocation (weighted by commits/PRs/reviews)
- Mock Solana transactions (returns simulated signatures)

---

## API Endpoints

### `GET /api/health`
Health check. Returns `{"status": "ok"}`.

### `GET /api/campaigns`
List all campaigns, sorted by creation date (newest first).

### `POST /api/campaigns`
Create a new campaign.

```json
{
  "repo": "owner/repo",
  "pool_amount": 1000000000,
  "deadline": "2025-04-01T00:00:00Z",
  "wallet_address": "YourSolanaPublicKey..."
}
```

Returns campaign details with `tx_signature` from the on-chain `create_campaign` call.

### `GET /api/campaigns/{id}`
Get campaign details including allocations (if finalized).

### `POST /api/campaigns/{id}/finalize-preview`
Fetch GitHub data + run AI allocation **without** writing to Solana. Returns:

```json
{
  "campaign_id": "abc12345",
  "repo": "owner/repo",
  "contributors": [
    {"username": "alice", "commits": 47, "pull_requests": 12, "reviews": 8}
  ],
  "allocations": [
    {"contributor": "alice", "percentage": 5000, "amount": 500000000, "reasoning": "..."}
  ],
  "ai_model": "gpt-4o-mini"
}
```

Percentages are in basis points (10000 = 100%).

### `POST /api/campaigns/{id}/finalize`
Execute finalization: GitHub fetch + AI allocation + Solana `finalize_campaign` transaction. Irreversible.

```json
{
  "campaign_id": "abc12345",
  "state": "finalized",
  "allocations": [...],
  "tx_signature": "5K7n...",
  "solana_explorer_url": "https://explorer.solana.com/tx/5K7n...?cluster=devnet"
}
```

---

## Solana Smart Contract

The Anchor program manages campaign lifecycle with two instructions:

### `create_campaign`
Creates a PDA account seeded by `["campaign", authority, campaign_id]`.

| Field | Type | Description |
|-------|------|-------------|
| `campaign_id` | String (max 32) | Unique identifier from backend |
| `repo` | String (max 64) | GitHub repo in `owner/repo` format |
| `pool_amount` | u64 | Reward pool in lamports |
| `deadline` | i64 | Unix timestamp |

### `finalize_campaign`
Stores AI allocation results on-chain. Validates:
- Campaign exists and is in `Created` state
- Allocations sum to exactly 10000 basis points (100%)
- No duplicate contributors
- Max 10 allocations per campaign

| Field | Type | Description |
|-------|------|-------------|
| `allocations` | Vec\<AllocationInput\> | `{contributor: String, percentage: u16}` |

### Account Structure

```rust
Campaign {
    authority:    Pubkey,       // sponsor wallet
    campaign_id:  String,       // PDA seed
    repo:         String,       // "owner/repo"
    pool_amount:  u64,          // lamports
    deadline:     i64,          // unix timestamp
    state:        CampaignState,// Created | Finalized
    allocations:  Vec<Allocation>,
    bump:         u8,
    created_at:   i64,
    finalized_at: Option<i64>,
}
```

---

## AI Allocation Engine

### With OpenAI (production)

The backend sends contributor metrics to GPT-4o-mini with a structured prompt:

```
Repository: owner/repo
Contributors: [{username, commits, pull_requests, reviews, lines_added, lines_deleted}]
```

The model returns a JSON allocation with reasoning:

```json
[
  {"contributor": "alice", "percentage": 5000, "reasoning": "Highest commit count and PR activity"},
  {"contributor": "bob", "percentage": 3500, "reasoning": "Strong review participation"},
  {"contributor": "charlie", "percentage": 1500, "reasoning": "Focused bug fixes"}
]
```

Validation ensures percentages sum to exactly 10000 bps.

### Deterministic Fallback (demo / no API key)

When no OpenAI key is configured, the engine uses weighted scoring:

```
weight = commits * 3 + pull_requests * 5 + reviews * 2
percentage = weight / total_weight * 10000
```

This always produces valid allocations and works offline.

---

## Project Structure

```
repobounty-ai/
|-- backend/                        # Go API server
|   |-- cmd/api/main.go             # Entry point
|   |-- internal/
|   |   |-- config/config.go        # Environment configuration
|   |   |-- models/models.go        # Data structures
|   |   |-- store/memory.go         # In-memory campaign store
|   |   |-- github/client.go        # GitHub API client
|   |   |-- ai/allocator.go         # AI allocation engine
|   |   |-- solana/client.go        # Solana transaction builder
|   |   |-- http/router.go          # Chi router + middleware
|   |   |-- http/handlers.go        # Request handlers
|   |-- go.mod
|   |-- .env.example
|
|-- frontend/                       # React web interface
|   |-- src/
|   |   |-- main.tsx                # App bootstrap + wallet providers
|   |   |-- App.tsx                 # Routes
|   |   |-- components/
|   |   |   |-- Layout.tsx          # Header, nav, footer
|   |   |   |-- WalletButton.tsx    # Solana wallet connect
|   |   |   |-- CampaignCard.tsx    # Campaign list item
|   |   |-- pages/
|   |   |   |-- Home.tsx            # Campaign dashboard
|   |   |   |-- CreateCampaign.tsx  # Campaign creation form
|   |   |   |-- CampaignDetails.tsx # Details + finalization UI
|   |   |-- api/client.ts           # API client
|   |   |-- types/index.ts          # TypeScript interfaces
|   |-- package.json
|   |-- vite.config.ts
|   |-- tailwind.config.js
|
|-- program/                        # Solana/Anchor smart contract
|   |-- programs/repobounty/
|   |   |-- src/lib.rs              # Program logic
|   |   |-- Cargo.toml
|   |-- tests/repobounty.ts         # Anchor integration tests
|   |-- Anchor.toml
|   |-- Cargo.toml
|
|-- docs/                           # Documentation
|   |-- architecture.md             # System design
|   |-- hackathon-context.md        # Hackathon requirements
|   |-- demo-script.md              # Demo flow
|   |-- roadmap.md                  # MVP + future
|   |-- api.md                      # API reference
|   |-- smart-contract.md           # On-chain program docs
```

---

## Demo Flow

1. Open frontend at `localhost:3000`
2. Connect Phantom wallet (devnet)
3. Create a campaign: enter `owner/repo`, pool amount (SOL), deadline
4. Campaign appears in dashboard with "Active" status
5. After deadline passes, click "Preview Allocations"
6. System fetches GitHub data, AI generates allocation with reasoning
7. Review allocation breakdown and click "Finalize on Solana"
8. Transaction is sent — campaign moves to "Finalized"
9. View allocations on-chain via Solana Explorer link

**Backup mode (no external services):** The system works end-to-end with mock GitHub data, deterministic AI, and mock Solana transactions.

---

## MVP Scope

**In scope:**
- Public GitHub repositories
- Deadline-based campaigns
- AI-powered allocation with reasoning
- On-chain finalization via Solana
- Campaign dashboard with real-time status

**Out of scope (future):**
- Goal-based campaigns
- GitHub App integration
- Wallet binding by GitHub identity
- Claim flow (actual token distribution)
- Anti-fraud / anti-gaming detection
- Notifications
- Multi-sponsor campaigns
