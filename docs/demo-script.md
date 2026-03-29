# Demo Script

## Goal

Show the full autonomous flow:

GitHub data → AI decision → Solana transaction → on-chain state change

When presenting, distinguish clearly between:

- what is implemented now
- what is the agreed next revision

## Demo Steps

1. Open the demo interface.
2. Create a campaign:
   - public GitHub repo URL
   - reward pool
   - deadline
3. Show the created campaign in the UI.
4. Trigger campaign finalization.
5. Show backend fetching contributor data.
6. Show AI allocation response in structured JSON.
7. Send finalize transaction to Solana.
8. Show campaign state changed to finalized.
9. Show contributor allocations in UI.

## If Asked About Ownership / Claim

Use this explanation:

- current MVP writes campaign and allocation state on-chain
- agreed next revision makes sponsor wallet the on-chain campaign owner
- backend becomes a trusted finalizer after deadline
- contributors are identified by GitHub first
- contributors later bind wallets and claim or receive released rewards

## What to Emphasize

- sponsor can fund a public repository
- AI determines reward distribution
- the final decision is sent on-chain
- Solana stores the final allocation state
- the system is intentionally scoped as an MVP
- the next revision adds sponsor-owned authority, escrow, and GitHub-based claims

## Backup Mode

If any external dependency fails during demo:

- use mocked GitHub contributor data
- use deterministic AI fallback response
- still execute the real Solana finalization step
