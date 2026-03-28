# Architecture

## High-Level Overview

RepoBounty AI connects public GitHub contribution data, AI-based reward allocation, and Solana smart contract state.

Main flow:

**Frontend → Backend API → GitHub Public API**  
**Frontend → Backend API → AI Allocation Engine**  
**Backend API → Solana Program**  
**Frontend ← Backend API ← Solana state / allocation results**

## Components

### 1. Frontend

Responsibilities:

- create reward campaign
- input repository URL, reward pool, deadline
- display campaign state
- display final contributor allocation

Main screens:

- Create Campaign
- Campaign Details
- Final Allocation Results

### 2. Backend API

Responsibilities:

- validate campaign input
- store campaign metadata off-chain if needed
- fetch contributor activity from GitHub public API
- prepare normalized contributor dataset
- call AI allocation engine
- validate AI response format
- send finalize transaction to Solana
- return final campaign result to frontend

Suggested modules:

- campaign service
- github service
- ai allocation service
- solana service

### 3. GitHub Data Layer

Responsibilities:

- fetch public repository contribution data
- collect basic contributor metrics for the MVP

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

- create campaign account
- store campaign metadata
- receive final allocations
- mark campaign finalized
- expose campaign state for UI

Core entities:

- Campaign
- ContributorAllocation

Core instructions:

- create_campaign
- finalize_campaign
- fetch campaign state client-side

Optional future instructions:

- claim_reward
- add_funds
- cancel_campaign
- multi-sponsor support

## Data Flow

### Campaign Creation

1. User submits repo URL, pool, deadline.
2. Backend validates input.
3. Backend sends transaction to create campaign on Solana.
4. Campaign account is created on-chain.
5. Frontend shows created campaign.

### Campaign Finalization

1. Deadline is reached or test-finalize is triggered.
2. Backend fetches GitHub contributor data.
3. Backend sends normalized data to AI engine.
4. AI returns allocation.
5. Backend validates allocation.
6. Backend sends finalize transaction to Solana.
7. Smart contract stores allocations and marks campaign as finalized.
8. Frontend displays final results.

## Trust Boundaries

### Off-chain

- GitHub contribution data
- AI allocation logic
- backend orchestration

### On-chain

- campaign existence
- finalized allocation
- final state of campaign

This split keeps the MVP realistic while still preserving the key requirement:
AI must influence real on-chain state.

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
        +<--------------------- +------------------+ <-------------------+
                                |  Solana Program  |
                                +------------------+
```

## Why This Architecture Fits the Hackathon

This architecture is intentionally narrow:

- small enough to build in a few days
- complete enough to demonstrate real autonomy
- extensible enough for future features

The main goal is not to build a full production platform, but to prove the end-to-end autonomous flow.