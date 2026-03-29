# RepoBounty AI

**AI-powered reward allocation for open-source contributors, with sponsor-owned campaigns and Solana settlement.**

> National Solana Hackathon (Decentrathon) | Case 2: "AI + Blockchain: Autonomous Smart Contracts"

---

## The Idea

Sponsors fund public GitHub repositories with a reward pool and a deadline. The agreed next architecture is sponsor-owned on-chain campaigns, backend-triggered finalization, and GitHub-based contributor claims after wallet binding.

**The core chain that must work:**

```
GitHub data --> AI allocation --> Solana finalization --> GitHub-based entitlements --> claim/release
```

AI is not advisory — its decision directly influences on-chain reward state.

## Architecture Decision

The agreed target model is:

- sponsor wallet is the on-chain `authority`
- campaign creation should be signed by the sponsor wallet
- campaign funds are intended to move into escrow in the next contract revision
- backend uses a dedicated trusted finalizer key after the deadline
- finalization stores GitHub-based entitlements first, because contributor wallets may be unknown
- contributors later authenticate with GitHub, bind wallets, and claim or receive released rewards

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

1. Sponsor connects wallet and creates campaign via frontend
2. Backend validates campaign input and orchestrates Solana interaction
3. After deadline: backend fetches GitHub contributor stats
4. Backend sends metrics to AI, receives allocation with reasoning
5. Trusted backend finalizer key calls `finalize_campaign` on Solana
6. Smart contract validates and stores GitHub-based allocations or entitlements on-chain
7. Contributors later bind wallets and claim or receive released rewards
8. Frontend displays final results with Solana Explorer links

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
| `SOLANA_PRIVATE_KEY` | No | — | Backend service/finalizer key for Solana transactions in the current MVP |
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

Returns campaign details with `tx_signature` from the on-chain `create_campaign` call. `deadline` should be sent as RFC3339 with time, for example `2025-04-01T12:30:00Z`.

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

In the agreed target architecture, this call is authorized by a dedicated backend finalizer key after the deadline, while campaign ownership remains with the sponsor wallet.

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

The current Anchor program manages basic campaign lifecycle. The agreed next revision is a sponsor-owned escrow-and-claim model.

### `create_campaign`
Creates a PDA account seeded by `["campaign", authority, campaign_id]`, where `authority` is the sponsor wallet.

| Field | Type | Description |
|-------|------|-------------|
| `campaign_id` | String (max 32) | Unique identifier from backend |
| `repo` | String (max 64) | GitHub repo in `owner/repo` format |
| `pool_amount` | u64 | Reward pool in lamports |
| `deadline` | i64 | Unix timestamp |

### `finalize_campaign`
In the target architecture, stores AI allocation results or GitHub-based entitlements on-chain. Validates:
- Campaign exists and is in `Created` state
- Deadline has passed
- Signer matches the configured trusted finalizer authority
- Allocations sum to exactly 10000 basis points (100%)
- No duplicate contributors
- Max 10 allocations per campaign

| Field | Type | Description |
|-------|------|-------------|
| `allocations` | Vec\<AllocationInput\> | `{contributor: String, percentage: u16}` in the current MVP, evolving toward GitHub-based entitlements |

### `claim_reward` (planned)

The next contract revision should add a claim or release instruction:

- finalization records entitlement by GitHub identity
- contributor authenticates with GitHub off-chain
- contributor binds a wallet
- backend authorizes the claim or release
- escrowed reward is transferred to the bound wallet

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

The next contract revision should extend this model with:

- escrow / vault linkage
- trusted finalizer authority
- claim status per contributor entitlement
- wallet release metadata

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
10. In the target claim flow, contributors later log in with GitHub, bind wallets, and claim rewards

**Backup mode (no external services):** The system works end-to-end with mock GitHub data, deterministic AI, and mock Solana transactions.

---

## MVP Scope

**In scope:**
- Public GitHub repositories
- Deadline-based campaigns
- AI-powered allocation with reasoning
- On-chain finalization via Solana
- On-chain campaign listing
- `All Campaigns` / `My Campaigns` filtering
- Campaign dashboard with real-time status

**Out of scope (future):**
- Sponsor-signed escrow deposits
- GitHub login + wallet binding
- Claim flow (actual token distribution)
- Goal-based campaigns
- GitHub App integration
- Anti-fraud / anti-gaming detection
- Notifications
- Multi-sponsor campaigns
