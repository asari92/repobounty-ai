# RepoBounty AI

**AI-powered reward allocation for open-source contributors, with on-chain escrow and claims on Solana.**

> National Solana Hackathon (Decentrathon) | Case 2: "AI + Blockchain: Autonomous Smart Contracts"

---

## The Idea

Sponsors fund public GitHub repositories with a reward pool and a deadline. After the deadline, the system fetches contributor data, runs AI-based impact scoring on code diffs, and finalizes the allocation on-chain. Contributors log in with GitHub, bind their wallets, and claim rewards from the escrow vault.

**AI is not advisory — its decision directly determines on-chain reward distribution.**

```
GitHub data → AI allocation → Solana finalization → GitHub-based claims → SOL transfer
```

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
                    |  GitHub   |     | OpenRouter  |
                    |  API      |     |  (LLM AI)   |
                    +-----------+     +-------------+
```

**Data flow:**

1. Sponsor signs in to the app, connects Phantom, and submits a campaign (repo, pool, deadline).
2. The backend authority calls `create_campaign` on-chain, records the sponsor wallet, and returns the campaign PDA plus the derived vault PDA.
3. The frontend requests a funding transaction, and the sponsor signs it in Phantom to transfer SOL into the vault and move the campaign to `Funded`.
4. After the deadline, the backend fetches GitHub contributors and PR diffs when available.
5. The backend runs AI code-impact scoring, or a deterministic fallback, to produce basis-point allocations.
6. The backend finalize endpoint or auto-finalize worker calls `finalize_campaign`, storing the allocations on-chain.
7. Contributors log in with GitHub, can link a wallet on the Profile page, and open the finalized campaign.
8. A contributor requests a claim, and the backend verifies GitHub identity before submitting `claim` to transfer SOL to the contributor wallet.
9. When all allocations are claimed, the campaign transitions to `Completed`.

---

## Tech Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| **Frontend** | React + TypeScript + Vite | React 18.3, Vite 5.4, TS 5.5 |
| **Styling** | Tailwind CSS | 3.4 |
| **Wallet** | @solana/wallet-adapter (Phantom) | wallet-adapter-react 0.15, web3.js 1.95 |
| **Routing** | React Router | v6.26 |
| **Backend** | Go + Chi router | Go 1.25, Chi 5.1 |
| **Auth** | GitHub OAuth + JWT (HS256) | golang-jwt 5.3 |
| **AI Engine** | OpenRouter LLM (code diff analysis) | Nemotron free tier by default |
| **AI Fallback** | Deterministic weighted scoring | Built-in, no API key needed |
| **Storage** | SQLite (persistent) / in-memory | modernc.org/sqlite 1.48 |
| **Blockchain** | Solana (devnet) + Anchor | Anchor 0.32.1, Rust 1.92 |
| **Solana Client** | gagliardetto/solana-go | 1.11 |
| **Logging** | Zap (structured) | 1.27 |

---

## Quick Start (one command)

```bash
./start.sh
```

The script will:
- Create `.env` from template if missing and show what to fill in
- Check required and optional variables, print instructions for each
- Start backend + frontend via Docker Compose

After start:
- **Frontend:** http://localhost:5173
- **Backend API:** http://localhost:8080
- **Health check:** http://localhost:8080/api/health

> Without any API keys the app runs in **mock/demo mode**: mock GitHub data, deterministic AI allocation, mock Solana transactions.

---

## Local Development (without Docker)

### Prerequisites

| Tool | Version | Check |
|------|---------|-------|
| Go | 1.25+ | `go version` |
| Node.js | 20+ | `node --version` |
| npm | 10+ | `npm --version` |
| Rust | 1.92+ | `rustc --version` |
| Solana CLI | 1.18+ | `solana --version` |
| Anchor CLI | 0.32.1 | `anchor --version` |

> Solana CLI and Anchor are needed only if you want to build/deploy the smart contract. Backend and frontend run without them.

### Step 1. Clone and configure

```bash
git clone <repo-url> repobounty-ai
cd repobounty-ai
```

### Step 2. Backend

```bash
cd backend
cp .env.example .env
```

Edit `backend/.env`:

```env
# Required
JWT_SECRET=<random string, min 32 chars>
# Generate: openssl rand -base64 32

# GitHub OAuth (for user login)
GITHUB_CLIENT_ID=<from https://github.com/settings/developers → New OAuth App>
GITHUB_CLIENT_SECRET=<same app>
# Set callback URL to: http://localhost:3000/auth/callback

# GitHub API (for contributor data)
GITHUB_TOKEN=<from https://github.com/settings/tokens, scopes: repo, read:user>

# Solana
SOLANA_RPC_URL=https://api.devnet.solana.com
SOLANA_PRIVATE_KEY=<base58 or JSON array, see "Solana keypair" section below>
PROGRAM_ID=GRfG4X51Uy6Jwunh93dXdFDMk5nN2ZVRAxBFr5sbegKy

# AI (optional — without it, deterministic fallback is used)
OPENROUTER_API_KEY=<from https://openrouter.ai/keys>
MODEL=nvidia/nemotron-3-super-120b-a12b:free

# URLs
FRONTEND_URL=http://localhost:3000
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
```

Start the backend:

```bash
go mod tidy
go run ./cmd/api
# → Listening on :8080
```

### Step 3. Frontend

```bash
cd frontend
npm install
npm run dev
# → http://localhost:3000
# Vite proxies /api → http://localhost:8080
```

### Step 4. Verify

```bash
curl http://localhost:8080/api/health
# → {"status":"ok"}
```

Open http://localhost:3000 in a browser with Phantom wallet extension.

---

## Solana Smart Contract

### Program ID

```
GRfG4X51Uy6Jwunh93dXdFDMk5nN2ZVRAxBFr5sbegKy
```

The current frontend/backend primarily use the deadline-campaign flow:
`create_campaign`, `fund_campaign`, `finalize_campaign`, and `claim`.
The on-chain program also includes additional maintenance and roadmap
instructions that are listed below.

### Generating a keypair for backend authority

```bash
# Generate a new keypair
solana-keygen new -o authority.json

# View public key
solana-keygen pubkey authority.json

# Get devnet SOL
solana config set --url https://api.devnet.solana.com
solana airdrop 5 --keypair authority.json
```

Put the private key into `SOLANA_PRIVATE_KEY` in `.env`. Two formats accepted:

```env
# JSON array (contents of the .json file)
SOLANA_PRIVATE_KEY=[174,23,45,...]

# Base58 string
SOLANA_PRIVATE_KEY=4wBqpZM9k...
```

### Building the program

```bash
cd program
yarn install
anchor build          # Full build with IDL
anchor build --no-idl # Faster, used in Docker
```

Output: `target/deploy/repobounty.so` + `repobounty-keypair.json`

### Running tests

```bash
anchor test
# Starts localnet validator, deploys program, runs ts-mocha tests
```

### Deploying to devnet

```bash
solana config set --url https://api.devnet.solana.com
anchor deploy --provider.cluster devnet
# → Program Id: GRfG4X51Uy6Jwunh93dXdFDMk5nN2ZVRAxBFr5sbegKy
```

### Deploying to mainnet-beta

```bash
solana config set --url https://api.mainnet-beta.solana.com
# Ensure deployer wallet has ~3-5 SOL
anchor deploy --provider.cluster mainnet
```

### Updating Program ID after fresh deploy

If you deploy with a new keypair and get a new Program ID, update in 3 places:

1. `program/programs/repobounty/src/lib.rs` → `declare_id!("NEW_ID")`
2. `program/Anchor.toml` → `[programs.devnet]` and `[programs.localnet]`
3. `backend/.env` → `PROGRAM_ID=NEW_ID`

Then rebuild and redeploy: `anchor build && anchor deploy --provider.cluster devnet`

### On-chain state machine

```
Created → Funded → Finalized → Completed
```

### Instructions

| Instruction | Signer | Precondition | Effect |
|-------------|--------|--------------|--------|
| `create_campaign` | authority (backend) | deadline in future, sponsor pubkey provided | Creates the campaign account; the vault PDA is derived from that campaign |
| `fund_campaign` | sponsor (wallet) | State == Created, vault funded in the same transaction | Verifies vault funding and moves state → Funded |
| `finalize_campaign` | authority (backend) | State == Funded, deadline passed, allocations valid | Stores allocations on-chain and moves state → Finalized |
| `claim` | authority (backend) | State == Finalized, allocation exists, not yet claimed | Transfers SOL from the vault to the contributor recipient account and marks the allocation claimed |
| `withdraw_remaining` | sponsor (wallet) | State == Completed | Withdraws any remaining dust from the vault back to the sponsor |
| `close_campaign` | authority (backend) | State == Completed | Closes the campaign account and returns rent to the authority |
| `add_sponsor` | authority (backend) + sponsor | State == Created or Funded, max 5 sponsors | Adds an additional sponsor entry and increases the pool amount |
| `complete_goal` | authority (backend) | Goal-based campaign, valid incomplete goal index | Marks a goal as completed by a contributor |

### PDA Seeds

```
Campaign PDA: ["campaign", campaign_id]
Vault PDA:    ["vault", campaign_pda]
```

### Constraints

- Allocations sum to exactly 10,000 basis points (100%)
- Max 10 allocations per campaign
- No duplicate contributors
- Deadline enforced on-chain for finalize
- Campaign ID max 32 chars, repo max 64 chars, contributor username max 39 chars
- Contract sizing currently reserves space for up to 5 sponsors and 10 goals per campaign

---

## AI Allocation Engine

### Mode 1: LLM with code diff analysis (production)

When `OPENROUTER_API_KEY` is set, the backend:

1. Fetches merged PRs with full diffs from GitHub
2. Sends code diffs to LLM via OpenRouter
3. LLM scores each contributor on 5 dimensions: Impact, Complexity, Scope, Quality, Community
4. Returns allocation with percentages and reasoning

### Mode 2: Deterministic fallback (demo / offline)

When no API key is configured:

```
score = commits × 3 + pull_requests × 5 + reviews × 2
percentage = score / total_score × 10000
```

Always produces valid allocations. Works fully offline.

---

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/health` | — | Health check |
| GET | `/api/auth/github/url` | — | GitHub OAuth URL |
| POST | `/api/auth/github/callback` | — | OAuth code exchange → JWT |
| GET | `/api/auth/me` | JWT | Current user |
| POST | `/api/auth/wallet/link` | JWT | Link Solana wallet to GitHub account |
| GET | `/api/auth/claims` | JWT | List claimable allocations |
| GET | `/api/campaigns/` | — | List all campaigns |
| POST | `/api/campaigns/` | — | Create campaign |
| GET | `/api/campaigns/{id}` | — | Get campaign details |
| POST | `/api/campaigns/{id}/fund-tx` | — | Get funding transaction for sponsor to sign |
| POST | `/api/campaigns/{id}/finalize-preview` | — | AI preview (no on-chain write) |
| POST | `/api/campaigns/{id}/finalize` | — | AI allocate + Solana finalize |
| POST | `/api/campaigns/{id}/claim` | JWT | Claim allocation |

### Create campaign

```bash
curl -X POST http://localhost:8080/api/campaigns/ \
  -H "Content-Type: application/json" \
  -d '{
    "repo": "owner/repo",
    "pool_amount": 1000000000,
    "deadline": "2026-04-01T12:00:00Z",
    "wallet_address": "YourPhantomPublicKey"
  }'
```

### Finalize preview

```bash
curl -X POST http://localhost:8080/api/campaigns/{id}/finalize-preview
```

Returns allocations with reasoning, without writing to Solana.

### Finalize

```bash
curl -X POST http://localhost:8080/api/campaigns/{id}/finalize
```

Irreversible. Fetches GitHub data → AI allocation → on-chain finalize.

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `PORT` | No | `8080` | Backend port |
| `JWT_SECRET` | **Yes** | — | Token signing key (min 32 chars) |
| `GITHUB_TOKEN` | No | — | GitHub PAT for API (mock data without it) |
| `GITHUB_CLIENT_ID` | No | — | GitHub OAuth app client ID |
| `GITHUB_CLIENT_SECRET` | No | — | GitHub OAuth app secret |
| `FRONTEND_URL` | No | `http://localhost:3000` | For OAuth redirects |
| `ALLOWED_ORIGINS` | No | `localhost:3000,5173` | CORS whitelist |
| `OPENROUTER_API_KEY` | No | — | LLM AI allocation (deterministic fallback without) |
| `MODEL` | No | `nvidia/nemotron-3-super-120b-a12b:free` | OpenRouter model |
| `SOLANA_RPC_URL` | No | `https://api.devnet.solana.com` | Solana RPC endpoint |
| `SOLANA_PRIVATE_KEY` | No | — | Backend authority keypair (mock mode without) |
| `PROGRAM_ID` | No | — | Deployed Anchor program address |
| `DATABASE_PATH` | No | `repobounty.db` | SQLite file path (empty = in-memory) |

---

## Project Structure

```
repobounty-ai/
├── start.sh                          # One-command startup script
├── docker-compose.yml                # Backend + frontend containers
├── .env.example                      # Environment template
│
├── backend/                          # Go API server
│   ├── cmd/api/main.go               # Entry point + auto-finalize worker
│   ├── internal/
│   │   ├── config/config.go          # Env loading with defaults
│   │   ├── models/models.go          # Campaign, Allocation, User types
│   │   ├── store/sqlite.go           # SQLite persistent storage
│   │   ├── auth/                     # GitHub OAuth + JWT + middleware
│   │   ├── github/client.go          # Contributors, PR diffs, reviews
│   │   ├── ai/allocator.go           # LLM + deterministic allocation
│   │   ├── solana/client.go          # Transaction builder, PDA derivation
│   │   └── http/                     # Chi router, handlers, worker
│   ├── go.mod
│   ├── Dockerfile
│   └── .env.example
│
├── frontend/                         # React SPA
│   ├── src/
│   │   ├── main.tsx                  # Wallet providers + router
│   │   ├── App.tsx                   # Routes
│   │   ├── api/client.ts             # API client with JWT injection
│   │   ├── hooks/useAuth.tsx         # Auth context (OAuth + wallet)
│   │   ├── pages/
│   │   │   ├── Home.tsx              # Campaign list (all / my)
│   │   │   ├── CreateCampaign.tsx    # 2-step: create → fund
│   │   │   ├── CampaignDetails.tsx   # Preview, finalize, claim
│   │   │   └── Profile.tsx           # Wallet binding, claimable rewards
│   │   └── components/               # Layout, WalletButton, CampaignCard
│   ├── package.json
│   ├── vite.config.ts                # Dev proxy /api → :8080
│   ├── nginx.conf                    # Production proxy config
│   └── Dockerfile
│
├── program/                          # Solana/Anchor smart contract
│   ├── programs/repobounty/
│   │   ├── src/lib.rs                # Program: create, fund, finalize, claim
│   │   └── Cargo.toml                # anchor-lang 0.32.1
│   ├── tests/repobounty.ts           # Anchor integration tests
│   ├── Anchor.toml                   # Program ID, cluster config
│   └── Dockerfile
│
└── docs/                             # Documentation
    ├── setup-guide.md                # Full setup & deployment guide
    ├── implementation-status.md      # What's done vs planned
    ├── architecture.md               # System design
    ├── api.md                        # API reference
    ├── smart-contract.md             # On-chain program docs
    ├── demo-script.md                # Demo walkthrough
    └── hackathon-context.md          # Hackathon requirements
```

---

## Demo Flow

1. Open http://localhost:3000 (dev) or http://localhost:5173 (Docker)
2. Connect Phantom wallet (devnet)
3. Create a campaign: repo, pool (SOL), deadline
4. Sign the funding transaction in Phantom
5. Wait for deadline (or set a near-future deadline for testing)
6. Click "Preview Allocations" → see AI-generated distribution
7. Click "Finalize on Solana" → on-chain transaction
8. Log in with GitHub on the Profile page
9. Link your Solana wallet
10. Claim your reward — SOL moves from vault to your wallet

---

## MVP Scope

**Implemented:**
- Public GitHub repositories, deadline-based campaigns
- Escrow vault (PDA) funded by sponsor
- AI allocation with code diff analysis (LLM) or deterministic fallback
- On-chain finalization with deadline enforcement
- GitHub OAuth + JWT authentication
- Wallet binding + contributor claim flow
- Auto-finalize background worker
- SQLite persistent storage
- Campaign dashboard with All / My Campaigns filter
- Docker Compose one-command deploy

**Out of scope:**
- Goal-based campaigns
- GitHub App integration (PR comments)
- Anti-fraud / sybil detection
- Notifications (email, Discord)
- Multi-sponsor campaigns
- "Claim all" aggregation
