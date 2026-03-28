# Hackathon Context

Decentrathon — National Solana Hackathon, Case 2: "AI + Blockchain: Autonomous Smart Contracts".

## Why Case 2

Case 1 (RWA Tokenization) requires complex SPL-token logic, heavy frontend (marketplace/AMM), and mock physical asset verification — too much for a tight timeline.

Case 2 lets us leverage existing LLM APIs for the complex logic, keep the smart contract simple (state-updater accepting AI wallet signatures), and build a minimal dashboard instead of a trading UI. Scores high on Innovation criteria (15 points).

## Core Chain (Must Work End-to-End)

**GitHub data -> AI allocation -> Solana transaction -> on-chain state change**

## What Can Be Mocked

- **GitHub data:** hardcoded contributors or static JSON instead of real API
- **AI (partially):** simple formula or LLM without complex logic, but must look like "AI makes the decision"
- **Funding:** fixed values, mock deposit
- **Claim:** show allocation + simulated claim button

## What Cannot Be Mocked

- Real Solana program
- Real transaction
- Real on-chain state change
- The chain: AI -> decision -> blockchain state change

## Why Solana (Positioning)

Don't say "we could use any chain." Say:

> "Solana is the high-performance settlement layer for open-source reward distribution — frequent, low-cost transactions for potentially many contributors across many campaigns."

## Killer Feature

Don't overcomplicate. The killer feature is already there:

> "AI-automated reward allocation for open-source contributions — not manual, not subjective, but automatic based on AI analysis."

Future vision (not MVP): unclaimed rewards by GitHub identity.

---

## Team Roles

| Person | Ownership |
|--------|-----------|
| Mukhan | Rust + Solana program + frontend Solana integration (wallet connect, tx signing) |
| You | Go backend, GitHub integration, AI integration, product logic |
| Kirill | Go backend, API, models, orchestration, integration support |

## Scope Freeze

### In MVP

- Public GitHub repos only
- Deadline-based campaigns only
- Campaign creation UI
- Contributor stats collection (commits, PRs, usernames)
- AI allocation in structured JSON
- Finalization transaction on Solana
- Result display in frontend

### Out of MVP

Goal-based campaigns, GitHub App, notifications, wallet binding, contributor registration, claim flow (if no time), anti-fraud, multi-sponsor.

## Product Flows

### Flow 1: Create Campaign

1. Sponsor connects wallet
2. Enters: repo URL, reward pool, deadline
3. Frontend sends to backend for validation
4. `create_campaign` transaction to Solana
5. Campaign created on-chain, shown in UI

### Flow 2: Finalize Campaign

1. Deadline reached (or demo trigger)
2. Backend fetches GitHub contributor activity
3. Backend normalizes metrics, sends to AI
4. AI returns allocation JSON
5. Backend validates result
6. `finalize_campaign` transaction to Solana
7. Program stores allocations, marks finalized
8. Frontend shows final allocations

## Integration Contract

### Backend -> Frontend (finalize preview response)

```json
{
  "campaign_id": "camp_001",
  "repo": "owner/repo",
  "contributors": [
    { "username": "alice", "commits": 10, "prs": 2 },
    { "username": "bob", "commits": 5, "prs": 1 }
  ],
  "allocations": [
    { "username": "alice", "percentage": 60, "amount": 6.0, "reason": "Higher overall contribution" },
    { "username": "bob", "percentage": 40, "amount": 4.0, "reason": "Lower but still meaningful contribution" }
  ]
}
```

## Solana Program Spec

### Instructions

- `create_campaign` — repo ID, sponsor pubkey, pool amount, deadline
- `finalize_campaign` — campaign PDA, contributor allocations

### On-chain Entities

**Campaign:** sponsor pubkey, repo string/hash, pool amount, deadline, status, created_at, finalized_at

**Allocation:** contributor identifier (GitHub username/hash), amount/percentage, optional reason

### Not in MVP

claim_reward, add_funds, cancel_campaign, token minting, SPL transfers

## Go Backend Spec

### API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/campaigns/preview` | Validate and return normalized campaign draft |
| GET | `/campaigns/:id` | Campaign metadata and state |
| POST | `/campaigns/:id/finalize-preview` | Fetch contributors + AI allocation (no on-chain write) |
| POST | `/campaigns/:id/finalize` | Full finalization pipeline |

### AI Allocation Validation

- Sum of percentages = 100
- All users unique
- No negative values
- Empty result forbidden

### Package Structure

`internal/http`, `internal/campaigns`, `internal/github`, `internal/ai`, `internal/solana`, `internal/models`

### Not in MVP

Database persistence (in-memory OK), cron scheduler, retries/job queue, user accounts, notifications.

## Definition of Done

1. Sponsor can create a campaign for a public repo
2. Campaign is visible in UI
3. Backend fetches contributor activity
4. AI returns allocation decision
5. Decision is written to Solana
6. UI shows finalized campaign with allocations

## Key Advice

> The winning project is not the most complex — it's the most understandable and working one.
