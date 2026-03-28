# RepoBounty AI

RepoBounty AI is a hackathon MVP for Case 2, "AI + Blockchain: Autonomous Smart Contracts", of the National Solana Hackathon by Decentrathon.

The product allows a sponsor to fund a public GitHub repository for a fixed deadline. When the deadline is reached, the system collects contributor activity, runs AI-based reward allocation, and sends the result on-chain to a Solana smart contract. The smart contract stores the final allocation for each contributor.

This project demonstrates the required chain:

**AI → decision → on-chain transaction → smart contract state change**

## Problem

Open-source projects often receive support in an unstructured way:

- contributors are rewarded manually or not rewarded at all
- donations are not transparently tied to actual contribution
- reward distribution is subjective and hard to verify
- there is no clear bridge between contribution analysis and blockchain-based payout logic

## Solution

RepoBounty AI introduces a simple reward campaign for public GitHub repositories:

- a sponsor creates a campaign for a repository
- the sponsor sets a reward pool and a deadline
- after the deadline, the system collects contributor data
- AI determines how the reward pool should be split
- the backend sends the allocation to a Solana program
- the program stores the final contributor allocations on-chain

## Why Solana

Solana is used not as a decorative integration, but as the settlement and state layer of the system:

- campaign state is stored on-chain
- reward allocation is finalized on-chain
- contributor reward amounts are recorded in the smart contract state
- the system is designed for low-cost and frequent allocation transactions

For this use case, Solana is a strong fit because contributor reward systems may require cheap and fast transactions for frequent campaign finalization.

## MVP Scope

This hackathon MVP intentionally focuses on one narrow scenario:

- public GitHub repositories only
- deadline-based campaigns only
- AI-based allocation only after deadline
- on-chain storage of allocations on Solana

The MVP does not yet include:

- goal-based campaigns
- GitHub App integration
- GitHub notifications
- wallet binding by GitHub identity
- anti-fraud scoring
- multi-round campaign logic

These are future extensions, not part of the current MVP.

## User Flow

1. Sponsor enters a public GitHub repository URL.
2. Sponsor creates a reward campaign with:
   - reward pool
   - deadline
3. After the deadline, the backend fetches contributor activity from GitHub.
4. AI calculates the allocation between contributors.
5. Backend sends a finalize transaction to Solana.
6. Smart contract updates the campaign state and stores contributor allocations.
7. Frontend displays the final reward split.

## How AI Works

In the MVP, AI receives a simplified contributor dataset, for example:

- username
- number of commits
- number of pull requests
- optional contribution summary

AI returns a structured allocation decision in JSON format, for example:

- contributor list
- percentage or token amount per contributor
- short explanation of the distribution

The backend validates the AI output and sends the final allocation on-chain.

This means AI is not used for recommendations only. Its decision directly affects smart contract state.

## Architecture

The system contains four main parts:

1. Frontend
- create campaign form
- campaign details page
- allocation results page
- wallet connection and transaction signing UI

2. Backend API
- campaign management
- GitHub public data fetching
- AI request and response validation
- Solana transaction orchestration if needed by the chosen flow

3. AI Allocation Engine
- receives contributor stats
- returns final reward allocation
- can be implemented using an LLM API or a simplified mocked model for MVP

4. Solana Program
- stores campaign state
- stores final allocations
- marks campaign as finalized

## Smart Contract Responsibilities

The Solana program is responsible for:

- creating a campaign
- storing campaign metadata
- accepting final allocation data
- updating campaign state from active to finalized
- storing contributor allocation records

## Suggested Tech Stack

- **Backend:** Go
- **Smart contract / Solana program:** Rust + Anchor
- **Frontend:** TypeScript
- **AI integration:** external LLM API with structured JSON output or deterministic fallback for MVP

## Repository Structure

- `backend/` API for campaigns, GitHub integration, AI allocation, orchestration
- `frontend/` web interface for campaign creation and result viewing
- `program/` Solana smart contract / Anchor program
- `docs/` architecture diagrams, roadmap, and demo notes

## Demo Scenario

The demo shows the full chain:

1. create a campaign for a public repo
2. wait until the deadline or trigger a test deadline
3. fetch contributor data
4. run AI allocation
5. send finalize transaction to Solana
6. show updated on-chain campaign state and contributor allocations

## What Is Mocked in MVP

To keep the hackathon scope realistic, the following parts may be simplified:

- GitHub data can be fetched from public repositories only
- AI can use a simplified API or mocked scoring logic
- reward payout can be represented by on-chain allocation storage instead of full wallet claim logic

## Future Improvements

- goal-based campaigns
- GitHub App integration
- contributor notifications via GitHub comments
- GitHub identity based claims
- unclaimed rewards for contributors without prior registration
- AI anti-spam and anti-gaming detection
- reputation layer for contributors
- multi-sponsor campaigns

## Design Decisions

We intentionally reduced the scope to a deadline-based reward model in order to deliver a complete end-to-end MVP during the hackathon.

The current version prioritizes:

- working smart contract flow
- visible AI decision pipeline
- simple and clear demo
- real on-chain state change

## Local Development

### Backend

```bash
cd backend
go mod tidy
go run ./cmd/api
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

### Solana Program

```bash
cd program
anchor build
anchor test
```

## Team Notes

This repository is structured for hackathon delivery in stages:

- stage 1: code repository, architecture, minimal working element
- stage 2: working prototype, README, demo video
- stage 3: final polishing and presentation
