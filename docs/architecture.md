# Architecture

## High-Level Overview

Enshor connects public GitHub contribution data, AI-based reward allocation, and Solana smart contract state.

This document reflects the agreed target architecture for the next contract revision:

- sponsor wallet is the on-chain campaign `authority`
- backend uses a dedicated service/finalizer key after deadline
- contributors are identified by GitHub first, because wallets may be unknown at finalization time
- contributors later authenticate with GitHub, bind wallets, and claim or receive released rewards

Main flow:

**Frontend -> Backend API -> GitHub Public API**  
**Frontend -> Backend API -> AI Allocation Engine**  
**Sponsor Wallet -> Solana Program**  
**Backend Finalizer -> Solana Program**  
**Frontend <- Backend API <- Solana state / entitlement results**

## Components

### 1. Frontend

Responsibilities:

- create reward campaign
- input repository URL, reward pool, deadline
- display campaign state
- display final contributor allocation
- show `All Campaigns` and `My Campaigns`
- support future GitHub login and wallet binding for contributor claims

Main screens:

- Create Campaign
- Campaign Details
- Campaign List
- Future Claim Screen

### 2. Backend API

Responsibilities:

- validate campaign input
- store campaign metadata off-chain if needed
- fetch contributor activity from GitHub public API
- prepare normalized contributor dataset
- call AI allocation engine
- validate AI response format
- trigger trusted finalization after deadline
- maintain GitHub identity to wallet binding
- authorize claim or release after GitHub auth
- return campaign state and results to frontend

Suggested modules:

- campaign service
- github service
- ai allocation service
- solana service
- auth / wallet binding service

### 3. GitHub Data Layer

Responsibilities:

- fetch public repository contribution data
- collect contributor metrics for the MVP
- act as the off-chain identity source for future claims

MVP metrics:

- commits
- pull requests
- contributor usernames

Future metrics:

- issue activity
- review activity
- labels
- milestones
- GitHub App webhooks

### 4. AI Allocation Engine

Responsibilities:

- receive normalized contributor metrics
- compute reward split
- return structured allocation decision

Example input:

- repo
- deadline
- contributors[]
  - username
  - commits
  - prs

Example output:

- allocations[]
  - username
  - percentage
  - reason

MVP implementation options:

- LLM API with strict JSON schema
- mocked AI endpoint with deterministic logic

### 5. Solana Program

Responsibilities:

- create sponsor-owned campaign account
- hold escrowed campaign funds in the target design
- store campaign metadata
- receive final GitHub-based entitlements
- mark campaign finalized
- expose campaign state for UI
- release claimed rewards to bound contributor wallets

Core entities:

- Campaign
- ContributorEntitlement

Core instructions:

- create_campaign
- finalize_campaign
- claim_reward

Optional future instructions:

- add_funds
- cancel_campaign
- refund_campaign
- multi-sponsor support

## Data Flow

### Campaign Creation

1. Sponsor submits repo URL, pool, deadline.
2. Backend validates input and prepares campaign metadata.
3. Sponsor wallet signs the campaign creation transaction.
4. Campaign account is created on-chain with sponsor wallet as `authority`.
5. In the target escrow design, funds are deposited into campaign escrow.
6. Frontend shows created campaign.

### Campaign Finalization

1. Deadline is reached or test-finalize is triggered.
2. Backend fetches GitHub contributor data.
3. Backend sends normalized data to AI engine.
4. AI returns allocation.
5. Backend validates allocation.
6. Backend finalizer key sends finalize transaction to Solana.
7. Smart contract stores GitHub-based entitlements and marks campaign as finalized.
8. Frontend displays final results.

### Claim / Release

1. Contributor logs into the app with GitHub.
2. Contributor binds a Solana wallet.
3. Backend verifies that the GitHub identity has an unclaimed finalized entitlement.
4. Backend authorizes claim or release.
5. Smart contract transfers the reward from escrow to the bound wallet.

## Trust Boundaries

### Off-chain

- GitHub contribution data
- AI allocation logic
- backend orchestration
- GitHub auth
- wallet binding
- claim authorization

### On-chain

- sponsor ownership
- campaign existence
- finalized entitlement state
- escrowed reward pool in the target design
- final state of campaign

This split keeps the MVP realistic while preserving the key requirement:
AI must influence real on-chain reward state.

## Simple Component Diagram

```text
+-------------+        +------------------+        +------------------+
|  Frontend   | -----> |   Backend API    | -----> | GitHub Public API|
+-------------+        +------------------+        +------------------+
        |                       |
        |                       +--------------------> +-------------------+
        |                                            | AI Allocation Engine|
        |                                            +-------------------+
        |
        +---------------------> +------------------+
                                |  Solana Program  |
                                +------------------+
```

## Why This Architecture Fits the Hackathon

This architecture is intentionally narrow:

- small enough to build incrementally
- strong enough to demonstrate real ownership and reward settlement
- extensible enough for escrow, claim, and GitHub identity flows

The main goal is not to build a full production platform, but to prove the end-to-end autonomous reward flow with honest trust boundaries.
