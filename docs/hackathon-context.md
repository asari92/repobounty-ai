# Hackathon Context

Decentrathon — National Solana Hackathon, Case 2: "AI + Blockchain: Autonomous Smart Contracts".

## Why Case 2

Case 2 fits this project because we can combine:

- public GitHub contribution data
- AI-driven reward allocation
- real Solana state transitions

without needing a heavy marketplace or tokenized RWA frontend.

## Core Chain

The core chain that must work end-to-end is:

**GitHub data -> AI allocation -> Solana state change**

## Current MVP Reality

What is already implemented:

- public GitHub repo campaigns
- deadline-based campaign creation
- wallet-connected frontend
- AI allocation preview and finalization
- on-chain campaign creation and finalization
- on-chain campaign listing in the UI
- `All Campaigns` / `My Campaigns` filtering

What is still simplified in the current MVP:

- backend-triggered Solana transactions
- no escrow yet
- no GitHub login
- no wallet binding for contributors
- no real claim flow yet

## Agreed Next Architecture

The team decision for the next contract revision is:

- sponsor wallet becomes the on-chain campaign owner
- campaign creation should be sponsor-signed
- campaign funds should move into escrow
- backend should use a dedicated trusted finalizer key after deadline
- finalization should store GitHub-based contributor entitlements first
- contributors should later log in with GitHub, bind wallets, and claim or receive released rewards

This keeps sponsor ownership honest while still supporting contributors whose wallets are unknown at finalization time.

## What Can Be Mocked During Demo

- GitHub data, if API access is flaky
- AI logic, via deterministic fallback
- some future claim UX screens that are not implemented yet

## What Cannot Be Mocked

- real Solana program
- real Solana transaction
- real on-chain state change
- AI influencing the resulting on-chain reward state

## Product Flows

### Flow 1: Create Campaign

Current implementation:

1. Sponsor connects wallet
2. Enters repo, reward pool, deadline
3. Frontend sends data to backend
4. Backend validates and creates campaign on Solana
5. Campaign appears in UI

Planned next revision:

1. Sponsor connects wallet
2. Sponsor signs campaign creation
3. Campaign is created on-chain with sponsor wallet as `authority`
4. Reward funds move into escrow

### Flow 2: Finalize Campaign

Current implementation:

1. Deadline reached
2. Backend fetches GitHub contributor activity
3. Backend normalizes metrics and sends them to AI
4. AI returns allocation JSON
5. Backend validates result
6. Backend sends `finalize_campaign`
7. Program stores allocations and marks finalized
8. Frontend shows final allocations

Planned next revision:

1. Deadline reached
2. Backend fetches GitHub contributor activity
3. AI computes allocation
4. Trusted backend finalizer key writes GitHub-based entitlements on-chain
5. Campaign moves to finalized state

### Flow 3: Claim / Release

Planned next revision:

1. Contributor logs in with GitHub
2. Contributor binds Solana wallet
3. Backend verifies entitlement for that GitHub identity
4. Backend authorizes claim or release
5. Escrowed reward is paid to the bound wallet

## Backend / API Reality

Current implemented endpoints:

- `GET /api/campaigns`
- `POST /api/campaigns`
- `GET /api/campaigns/{id}`
- `POST /api/campaigns/{id}/finalize-preview`
- `POST /api/campaigns/{id}/finalize`

Planned next endpoints:

- GitHub OAuth login endpoints
- wallet binding endpoints
- claim listing endpoints
- claim or release endpoints

## Definition of Done

### Current MVP

1. Sponsor can create a campaign for a public repo
2. Campaign is visible in UI
3. Backend fetches contributor activity
4. AI returns allocation decision
5. Decision is written to Solana
6. UI shows finalized campaign with allocations

### Next revision

1. Sponsor is the on-chain campaign authority
2. Funds are escrowed
3. Backend finalizer is separated from sponsor ownership
4. Contributor entitlement is tied to GitHub identity first
5. Contributor can later bind wallet and claim reward

## Key Advice

The strongest version of this product is still the simplest one that honestly shows:

- real sponsor intent
- real AI allocation
- real on-chain reward state
