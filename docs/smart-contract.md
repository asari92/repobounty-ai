# Solana Smart Contract Documentation

## Overview

The Enshor program is an Anchor-based Solana smart contract that manages reward campaign lifecycle.

The agreed target contract model is:

- sponsor wallet owns the campaign on-chain
- campaign funds move into escrow
- backend uses a dedicated trusted finalizer key after deadline
- finalization records GitHub-based entitlements because contributor wallets may be unknown initially
- contributors later authenticate with GitHub, bind wallets, and claim or receive released rewards

**Program ID:** `BontyRePo1111111111111111111111111111111111` (placeholder — update after deployment)

---

## Instructions

### `create_campaign`

Creates a new campaign account as a PDA.

**Seeds:** `["campaign", authority_pubkey, campaign_id_bytes]`

**Arguments:**

| Name | Type | Constraints |
|------|------|------------|
| `campaign_id` | String | Max 32 characters |
| `repo` | String | Max 64 characters, `owner/repo` format |
| `pool_amount` | u64 | Must be > 0 |
| `deadline` | i64 | Must be in the future (checked against `Clock`) |

**Accounts:**

| Account | Signer | Mutable | Description |
|---------|--------|---------|-------------|
| `campaign` | No | Yes | PDA to be initialized |
| `authority` | Yes | Yes | Sponsor's wallet (campaign owner and payer) |
| `system_program` | No | No | System program |

**Effects:**

- Initializes campaign account with `Created` state
- Sets `created_at` to current clock timestamp
- Empty allocations vector
- In the target escrow design, also funds campaign escrow

---

### `finalize_campaign`

Stores AI-generated GitHub-based entitlements and marks campaign as finalized.

**Arguments:**

| Name | Type | Constraints |
|------|------|------------|
| `allocations` | Vec\<AllocationInput\> | 1-10 entries, percentages sum to 10000, unique contributors |

**AllocationInput:**

| Field | Type | Description |
|-------|------|-------------|
| `contributor` | String | GitHub username or normalized GitHub identity, max 39 chars in the current MVP |
| `percentage` | u16 | Basis points (0-10000) |

**Accounts:**

| Account | Signer | Mutable | Description |
|---------|--------|---------|-------------|
| `campaign` | No | Yes | Existing campaign PDA |
| `finalizer` | Yes | No | Trusted backend finalizer authority |

**Validations:**

1. Campaign must be in `Created` state
2. Deadline has passed
3. `finalizer` must match the configured trusted finalizer authority
4. Allocations must not be empty
5. Max 10 allocations
6. All percentages must sum to exactly 10000 (100%)
7. No duplicate contributor identities
8. Each contributor identity must be <= 39 characters in the current MVP

**Effects:**

- Stores entitlement amounts (`pool_amount * percentage / 10000`)
- Sets state to `Finalized`
- Sets `finalized_at` to current clock timestamp

---

### `claim_reward` (planned)

Transfers an allocated reward from escrow to a contributor wallet after GitHub auth and wallet binding.

Expected high-level flow:

1. Contributor authenticates off-chain with GitHub.
2. Contributor binds a Solana wallet.
3. Backend verifies that the GitHub identity is entitled to a reward and not yet claimed.
4. Backend authorizes claim or release.
5. Smart contract transfers the escrowed amount to the bound wallet and marks the entitlement claimed.

---

## Account Layout

### Campaign Account

| Field | Type | Size (bytes) | Description |
|-------|------|-------------|-------------|
| Discriminator | [u8; 8] | 8 | Anchor account discriminator |
| authority | Pubkey | 32 | Sponsor's wallet |
| campaign_id | String | 4 + 32 | PDA seed identifier |
| repo | String | 4 + 64 | GitHub repository |
| pool_amount | u64 | 8 | Reward pool in lamports |
| deadline | i64 | 8 | Campaign deadline (unix) |
| state | enum | 1 | Created(0) or Finalized(1) |
| allocations / entitlements | Vec\<Allocation\> | 4 + 10 * 53 | Up to 10 GitHub-based entitlements |
| bump | u8 | 1 | PDA bump seed |
| created_at | i64 | 8 | Creation timestamp |
| finalized_at | Option\<i64\> | 1 + 8 | Finalization timestamp |

**Total space:** ~699 bytes for the current simple layout.

The target escrow revision will likely extend the account model with:

- escrow vault PDA
- trusted finalizer authority
- claim status per entitlement
- bound wallet or release metadata

### Allocation / Entitlement

| Field | Type | Size (bytes) |
|-------|------|-------------|
| contributor | String | 4 + 39 |
| percentage | u16 | 2 |
| amount | u64 | 8 |

---

## Error Codes

Current codebase errors:

| Code | Name | Message |
|------|------|---------|
| 6000 | CampaignIdTooLong | Campaign ID must be 32 characters or fewer |
| 6001 | RepoNameTooLong | Repository name must be 64 characters or fewer |
| 6002 | InvalidPoolAmount | Pool amount must be greater than zero |
| 6003 | DeadlineInPast | Deadline must be in the future |
| 6004 | CampaignAlreadyFinalized | Campaign has already been finalized |
| 6005 | EmptyAllocations | Allocations must not be empty |
| 6006 | TooManyAllocations | Maximum 10 allocations allowed |
| 6007 | InvalidAllocationTotal | Allocation percentages must sum to 10000 basis points |
| 6008 | ContributorNameTooLong | Contributor username must be 39 characters or fewer |
| 6009 | DuplicateContributor | Duplicate contributor in allocations |

The escrow-and-claim revision will likely add:

- unauthorized finalizer
- deadline not reached
- reward already claimed
- wallet binding / release authorization errors

---

## PDA Derivation

Campaign accounts are Program Derived Addresses:

```text
seeds = ["campaign", authority_pubkey, campaign_id_as_bytes]
```

To derive in TypeScript:

```typescript
const [campaignPda, bump] = PublicKey.findProgramAddressSync(
  [
    Buffer.from("campaign"),
    authority.toBuffer(),
    Buffer.from(campaignId),
  ],
  programId
);
```

To derive in Go:

```go
campaignPDA, bump, _ := solana.FindProgramAddress(
    [][]byte{
        []byte("campaign"),
        authority.Bytes(),
        []byte(campaignID),
    },
    programID,
)
```

---

## Testing

```bash
cd program
yarn install
anchor test
```

The current test suite covers:

1. Campaign creation with valid parameters
2. Campaign finalization with 3 contributors
3. Rejection of double finalization
4. Rejection of allocations not summing to 100%

The next revision should add tests for:

1. sponsor-owned campaign creation
2. escrow funding
3. trusted finalizer authorization
4. claim or release after GitHub-authenticated wallet binding
5. double-claim prevention

---

## Deployment

```bash
# Build
cd program
anchor build

# Get program ID from build
solana address -k target/deploy/repobounty-keypair.json

# Update Anchor.toml and lib.rs with the real program ID

# Deploy to devnet
anchor deploy --provider.cluster devnet

# Verify
solana program show <PROGRAM_ID> --url devnet
```
