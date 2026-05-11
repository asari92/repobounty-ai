# Solana Program Documentation

This document describes the current Enshor Anchor program as implemented under `program/programs/repobounty` and aligned with the repository MVP technical specification.

## Overview

The Enshor Solana program manages the on-chain part of GitHub reward campaigns:

- global program configuration;
- campaign accounts;
- SOL reward escrow;
- service fee transfer;
- finalized claim rights by `github_user_id`;
- claim execution and double-claim prevention;
- delayed sponsor refunds;
- administrative pause and authority updates.

The program intentionally does not verify GitHub data or AI allocation quality on-chain. GitHub OAuth, repository validation, contributor analysis, AI or deterministic allocation, and claim authorization remain off-chain backend responsibilities.

Program id in the current source and Anchor config:

```text
5VdatUgJ6AsZ7RbC8TBz6AxUdBNtQ6MckLsKbxgZQdS6
```

The backend still uses the runtime `PROGRAM_ID` environment variable, so deployments must keep the backend configuration in sync with the deployed program id.

## Constants

| Constant | Value | Meaning |
|---|---:|---|
| `VERSION` | `1` | Account version marker for future migrations. |
| `MIN_CAMPAIGN_AMOUNT` | `500_000_000` lamports | Minimum campaign reward pool, `0.5 SOL`. |
| `MIN_ALLOCATION_AMOUNT` | `50_000_000` lamports | Minimum allocation per claim record, `0.05 SOL`. |
| `MIN_DEADLINE_SECONDS` | `300` | Minimum campaign duration for the hackathon MVP, 5 minutes. |
| `MAX_DEADLINE_SECONDS` | `31_536_000` | Maximum campaign duration, 365 days. |
| `CLAIM_WINDOW_SECONDS` | `31_536_000` | Claim window after `deadline_at`, 365 days. |
| `SERVICE_FEE_NUMERATOR` | `5` | Service fee numerator. |
| `SERVICE_FEE_DENOMINATOR` | `1000` | Service fee denominator, 0.5%. |
| `MIN_SERVICE_FEE` | `50_000_000` lamports | Minimum service fee, `0.05 SOL`. |
| `STATUS_ACTIVE` | `0` | Campaign is active. |
| `STATUS_FINALIZED` | `1` | Claim records are finalized and claim is available. |
| `STATUS_CLOSED` | `2` | Campaign is closed. |
| `PAYER_MODE_USER_PAID` | `0` | Claim transaction fee is paid by the user. |
| `PAYER_MODE_BACKEND_PAID` | `1` | Claim transaction fee is paid by the backend/service wallet. |

Service fee formula:

```text
service_fee = max(reward_amount * 5 / 1000, 50_000_000)
```

## PDA Seeds

| Account | Seeds |
|---|---|
| `Config` | `["config"]` |
| `Campaign` | `["campaign", sponsor_pubkey, campaign_id.to_le_bytes()]` |
| Escrow PDA | `["escrow", campaign_pubkey]` |
| `ClaimRecord` | `["claim", campaign_pubkey, github_user_id.to_le_bytes()]` |

## Accounts

### Config

Global configuration account. There is one config PDA per program.

Fields:

| Field | Type | Description |
|---|---|---|
| `version` | `u8` | Version marker. |
| `admin_wallet` | `Pubkey` | Wallet authorized to update config and pause/unpause. |
| `finalize_authority` | `Pubkey` | Backend/service wallet authorized to finalize campaigns. |
| `claim_authority` | `Pubkey` | Backend/service wallet required to co-sign claims. |
| `treasury_wallet` | `Pubkey` | Receives service fees. |
| `paused` | `bool` | Blocks state-changing instructions when true. |
| `bump` | `u8` | PDA bump. |
| `_reserved` | `[u8; 64]` | Reserved for future migrations. |

Space: `203` bytes including Anchor discriminator.

### Campaign

On-chain campaign state. The program stores only numeric `github_repo_id` on-chain, not repo owner, repo name, or repo URL.

Fields:

| Field | Type | Description |
|---|---|---|
| `version` | `u8` | Version marker. |
| `campaign_id` | `u64` | Backend-generated campaign id. |
| `sponsor` | `Pubkey` | Wallet that created and funded the campaign. |
| `github_repo_id` | `u64` | Stable numeric GitHub repository id. |
| `created_at` | `i64` | Unix timestamp at creation. |
| `deadline_at` | `i64` | Unix timestamp after which finalization is allowed. |
| `claim_deadline_at` | `i64` | `deadline_at + 365 days`; refund is allowed after this timestamp. |
| `total_reward_amount` | `u64` | Reward pool in lamports. |
| `allocated_amount` | `u64` | Sum of finalized allocations. |
| `claimed_amount` | `u64` | Sum of successful claims. |
| `allocations_count` | `u32` | Number of created claim records. |
| `claimed_count` | `u32` | Number of successful claims. |
| `status` | `u8` | `0 = Active`, `1 = Finalized`, `2 = Closed`. |
| `bump` | `u8` | PDA bump. |
| `_reserved` | `[u8; 64]` | Reserved for future migrations. |

Space: `171` bytes including Anchor discriminator.

### ClaimRecord

Claim right for one GitHub user in one campaign. Contributors do not need to have a wallet at campaign creation or finalization time because claim records are keyed by `github_user_id`.

Fields:

| Field | Type | Description |
|---|---|---|
| `campaign` | `Pubkey` | Campaign this claim belongs to. |
| `github_user_id` | `u64` | Stable numeric GitHub user id. |
| `amount` | `u64` | Claimable amount in lamports. |
| `claimed` | `bool` | Prevents double claim. |
| `claimed_to_wallet` | `Option<Pubkey>` | Recipient wallet after claim. |
| `claimed_at` | `Option<i64>` | Claim timestamp after claim. |
| `bump` | `u8` | PDA bump. |
| `_reserved` | `[u8; 32]` | Reserved for future migrations. |

Space: `132` bytes including Anchor discriminator.

### Escrow PDA

The escrow PDA is a system-owned PDA that holds SOL for the campaign reward pool. It is constrained by seeds `["escrow", campaign.key()]` in create, claim, and refund flows.

## Instructions

### `initialize_config`

Creates the global config PDA.

Arguments:

| Name | Type | Description |
|---|---|---|
| `finalize_authority` | `Pubkey` | Backend/service authority for finalization. |
| `claim_authority` | `Pubkey` | Backend/service authority for claims. |
| `treasury_wallet` | `Pubkey` | Service fee recipient. |

Accounts:

| Account | Signer | Mutable | Description |
|---|---|---|---|
| `admin_wallet` | Yes | Yes | Pays for and becomes admin of config. |
| `config` | No | Yes | Config PDA initialized with seed `["config"]`. |
| `system_program` | No | No | System program. |

Effects:

- Sets `version = 1`.
- Sets admin, finalize authority, claim authority, and treasury wallet.
- Sets `paused = false`.

### `update_config`

Updates one or more config authorities.

Arguments:

| Name | Type | Description |
|---|---|---|
| `finalize_authority` | `Option<Pubkey>` | New finalize authority if provided. |
| `claim_authority` | `Option<Pubkey>` | New claim authority if provided. |
| `treasury_wallet` | `Option<Pubkey>` | New treasury wallet if provided. |

Checks:

- `admin_wallet` must sign.
- `config.admin_wallet == admin_wallet.key()`.

### `set_paused`

Sets the global pause flag.

Arguments:

| Name | Type | Description |
|---|---|---|
| `paused` | `bool` | New pause state. |

Checks:

- `admin_wallet` must sign.
- `config.admin_wallet == admin_wallet.key()`.

When paused, state-changing campaign instructions that read `Config` reject via `ProgramPaused`.

### `create_campaign_with_deposit`

Creates a campaign and locks the reward pool in escrow in the same instruction.

Arguments:

| Name | Type | Description |
|---|---|---|
| `campaign_id` | `u64` | Backend-generated numeric campaign id. |
| `github_repo_id` | `u64` | Numeric GitHub repository id. |
| `deadline_at` | `i64` | Campaign deadline Unix timestamp. |
| `reward_amount` | `u64` | Reward pool in lamports. |

Accounts:

| Account | Signer | Mutable | Description |
|---|---|---|---|
| `sponsor` | Yes | Yes | Pays campaign rent, reward pool, service fee, and transaction fee. |
| `config` | No | No | Config PDA; must not be paused. |
| `campaign` | No | Yes | Campaign PDA initialized with sponsor and campaign id seeds. |
| `escrow` | No | Yes | Escrow PDA for this campaign. |
| `treasury_wallet` | No | Yes | Must equal `config.treasury_wallet`. |
| `system_program` | No | No | System program. |

Checks:

- Program is not paused.
- `reward_amount >= 0.5 SOL`.
- `deadline_at >= now + 5 minutes`.
- `deadline_at <= now + 365 days`.
- `treasury_wallet == config.treasury_wallet`.

Effects:

- Transfers `reward_amount` from sponsor to escrow PDA.
- Transfers `service_fee` from sponsor to treasury wallet.
- Initializes `Campaign` with `status = Active`.
- Sets `claim_deadline_at = deadline_at + 365 days`.
- Emits `CampaignCreated`.

### `finalize_campaign_batch`

Creates one batch of ClaimRecord accounts and finalizes the campaign after the final batch.

Arguments:

| Name | Type | Description |
|---|---|---|
| `allocations` | `Vec<AllocationEntry>` | Batch entries. Each entry has `github_user_id: u64` and `amount: u64`. |
| `has_more` | `bool` | Whether more finalization batches are expected. |

Accounts:

| Account | Signer | Mutable | Description |
|---|---|---|---|
| `finalize_authority` | Yes | Yes | Pays ClaimRecord rent and must match config. |
| `config` | No | No | Config PDA; must not be paused. |
| `campaign` | No | Yes | Active campaign being finalized. |
| `system_program` | No | No | System program. |
| `remaining_accounts` | No | Yes | ClaimRecord PDA accounts to create, one per allocation. |

Checks:

- Program is not paused.
- `config.finalize_authority == finalize_authority.key()`.
- `campaign.status == Active`.
- `now >= campaign.deadline_at`.
- `allocations` is not empty.
- Each `github_user_id > 0`.
- Each `amount >= 0.05 SOL`.
- No duplicate `github_user_id` inside the batch.
- Each remaining account matches the expected ClaimRecord PDA.
- Each ClaimRecord account does not already exist.
- New allocated total does not exceed `campaign.total_reward_amount`.
- On the final batch (`has_more == false`), allocated total must exactly equal reward total.

Effects:

- Creates ClaimRecord accounts owned by the program.
- Writes amount and GitHub user id into each ClaimRecord.
- Increments `allocated_amount` and `allocations_count`.
- Emits `FinalizeBatchAppended`.
- If `has_more == false`, sets `campaign.status = Finalized` and emits `CampaignFinalized`.

Backend integration note:

- The program supports batch semantics.
- The current backend has batch-planning helpers, but the active finalization sender currently submits one finalization instruction with `has_more = false`.

### `claim`

Transfers a finalized claim from escrow to the user signer wallet.

Arguments:

| Name | Type | Description |
|---|---|---|
| `github_user_id` | `u64` | GitHub user id for the ClaimRecord PDA. |
| `payer_mode` | `u8` | `0 = user-paid`, `1 = backend-paid`. Used for validation and event metadata; fee payer is selected off-chain by transaction construction. |

Accounts:

| Account | Signer | Mutable | Description |
|---|---|---|---|
| `user` | Yes | Yes | Wallet receiving the claim. |
| `claim_authority` | Yes | No | Backend/service authority; must match config. |
| `config` | No | No | Config PDA; must not be paused. |
| `campaign` | No | Yes | Finalized campaign. |
| `claim_record` | No | Yes | ClaimRecord PDA for campaign and GitHub user id. |
| `escrow` | No | Yes | Campaign escrow PDA. |
| `recipient_wallet` | No | Yes | Must equal `user.key()`. |
| `system_program` | No | No | System program. |

Checks:

- Program is not paused.
- `payer_mode` is `0` or `1`.
- `config.claim_authority == claim_authority.key()`.
- `campaign.status == Finalized`.
- `now <= campaign.claim_deadline_at`.
- ClaimRecord belongs to this campaign.
- ClaimRecord GitHub user id matches the instruction argument.
- ClaimRecord is not already claimed.
- ClaimRecord amount is positive.
- Escrow has enough lamports.
- `recipient_wallet == user signer`.

Effects:

- Transfers `claim_record.amount` from escrow to recipient wallet.
- Sets `claim_record.claimed = true`.
- Records `claimed_to_wallet` and `claimed_at`.
- Increments campaign `claimed_amount` and `claimed_count`.
- Emits `ClaimProcessed`.
- If all reward amount is claimed, all claim records are claimed, or escrow balance is zero, sets campaign status to Closed and emits `CampaignClosed` with reason `claim_completed`.

Backend integration note:

- The on-chain instruction accepts both payer modes.
- The currently shipped backend/frontend claim path builds user-paid transactions.

### `close_unfinalizable_campaign`

Closes a campaign that cannot be finalized due to unavailable or insufficient off-chain contribution data.

Accounts:

| Account | Signer | Mutable | Description |
|---|---|---|---|
| `finalize_authority` | Yes | No | Must match config. |
| `config` | No | No | Config PDA; must not be paused. |
| `campaign` | No | Yes | Active campaign with no successful finalize batches. |

Checks:

- Program is not paused.
- `config.finalize_authority == finalize_authority.key()`.
- `campaign.status == Active`.
- `campaign.allocations_count == 0`.
- `now >= campaign.deadline_at`.

Effects:

- Sets `campaign.status = Closed`.
- Emits `CampaignClosed` with reason `unfinalizable`.

Important spec rule:

This instruction is only appropriate when the backend has determined that valid allocation input data cannot be obtained. It must not be used for temporary model failures, RPC failures, transaction-size issues, or after any finalize batch has succeeded.

Funds remain in escrow until `refund_unclaimed` is allowed after `claim_deadline_at`.

### `refund_unclaimed`

Returns remaining escrow balance to the sponsor after the claim window expires.

Accounts:

| Account | Signer | Mutable | Description |
|---|---|---|---|
| `sponsor` | Yes | Yes | Must be campaign sponsor and receives refund. |
| `config` | No | No | Config PDA; must not be paused. |
| `campaign` | No | Yes | Campaign to close/refund. |
| `escrow` | No | Yes | Campaign escrow PDA. |
| `system_program` | No | No | System program. |

Checks:

- Program is not paused.
- `campaign.sponsor == sponsor.key()`.
- `now > campaign.claim_deadline_at`.
- Escrow balance is positive.

Effects:

- Transfers the full escrow balance to sponsor.
- Sets `campaign.status = Closed`.
- Emits `RefundProcessed`.
- Emits `CampaignClosed` with reason `refund`.

The instruction does not require a specific campaign status. The spec allows refund after claim deadline for Active, Finalized, or Closed campaigns if escrow still has a positive balance.

## Events

| Event | Fields | Emitted by |
|---|---|---|
| `CampaignCreated` | `campaign_pubkey`, `campaign_id`, `sponsor`, `github_repo_id`, `deadline_at`, `total_reward_amount`, `service_fee` | `create_campaign_with_deposit` |
| `FinalizeBatchAppended` | `campaign_pubkey`, `batch_count`, `batch_total_amount`, `has_more` | `finalize_campaign_batch` |
| `CampaignFinalized` | `campaign_pubkey`, `allocations_count`, `allocated_amount` | Final `finalize_campaign_batch` |
| `ClaimProcessed` | `campaign_pubkey`, `github_user_id`, `recipient_wallet`, `amount`, `payer_mode` | `claim` |
| `CampaignClosed` | `campaign_pubkey`, `reason` | `claim`, `close_unfinalizable_campaign`, `refund_unclaimed` |
| `RefundProcessed` | `campaign_pubkey`, `sponsor`, `refunded_amount` | `refund_unclaimed` |

## Error Codes

Anchor assigns numeric error codes at build time. The source enum names and messages are:

| Name | Message |
|---|---|
| `ProgramPaused` | Program is paused |
| `InvalidDeadline` | Invalid deadline |
| `InvalidCampaignAmount` | Campaign amount below minimum (0.5 SOL) |
| `CampaignAlreadyExists` | Campaign already exists |
| `CampaignNotActive` | Campaign is not in Active status |
| `CampaignNotFinalized` | Campaign is not in Finalized status |
| `CampaignClosed` | Campaign is closed |
| `DeadlineNotReached` | Deadline not reached yet |
| `ClaimWindowExpired` | Claim window has expired |
| `ClaimDeadlineNotReached` | Claim deadline not reached yet (for refund) |
| `Unauthorized` | Unauthorized |
| `EmptyAllocations` | Allocations list must not be empty |
| `DuplicateAllocation` | Duplicate allocation in batch |
| `AllocationTooSmall` | Allocation amount too small (min 0.05 SOL) |
| `AllocationOverflow` | Allocated amount would exceed total reward |
| `AllocationTotalMismatch` | Final batch: allocated != total_reward_amount |
| `ClaimRecordAlreadyExists` | ClaimRecord already exists |
| `ClaimNotFound` | Claim record not found |
| `ClaimAlreadyClaimed` | Reward already claimed |
| `EscrowInsufficientFunds` | Escrow has insufficient funds |
| `EscrowEmpty` | Escrow is empty |
| `InvalidSponsor` | Invalid sponsor |
| `InvalidRecipientWallet` | Invalid recipient wallet |
| `InvalidPayerMode` | Invalid payer mode |
| `PartialFinalizationExists` | Cannot close: partial finalization exists |
| `InvalidGithubUserId` | Invalid github user ID |
| `ArithmeticOverflow` | Arithmetic overflow |

## Lifecycle

```text
create_campaign_with_deposit
  -> Active

finalize_campaign_batch(..., has_more = true)
  -> Active

finalize_campaign_batch(..., has_more = false, allocated == total)
  -> Finalized

claim final record or empty escrow condition
  -> Closed

close_unfinalizable_campaign after deadline and before any finalize batch
  -> Closed

refund_unclaimed after claim_deadline_at with positive escrow
  -> Closed
```

Claim is allowed only while campaign status is Finalized and `now <= claim_deadline_at`.

Refund is allowed after `claim_deadline_at` if sponsor signs and escrow has a positive balance.

## TypeScript PDA Examples

```typescript
import { PublicKey } from '@solana/web3.js';
import BN from 'bn.js';

const [configPda] = PublicKey.findProgramAddressSync(
  [Buffer.from('config')],
  programId
);

const campaignIdBytes = new BN(campaignId).toArrayLike(Buffer, 'le', 8);
const [campaignPda] = PublicKey.findProgramAddressSync(
  [Buffer.from('campaign'), sponsor.toBuffer(), campaignIdBytes],
  programId
);

const [escrowPda] = PublicKey.findProgramAddressSync(
  [Buffer.from('escrow'), campaignPda.toBuffer()],
  programId
);

const githubUserIdBytes = new BN(githubUserId).toArrayLike(Buffer, 'le', 8);
const [claimRecordPda] = PublicKey.findProgramAddressSync(
  [Buffer.from('claim'), campaignPda.toBuffer(), githubUserIdBytes],
  programId
);
```

## Go PDA Examples

```go
configPDA, _, err := solana.FindProgramAddress(
    [][]byte{[]byte("config")},
    programID,
)

var campaignSeed [8]byte
binary.LittleEndian.PutUint64(campaignSeed[:], campaignID)
campaignPDA, _, err := solana.FindProgramAddress(
    [][]byte{[]byte("campaign"), sponsor.Bytes(), campaignSeed[:]},
    programID,
)

escrowPDA, _, err := solana.FindProgramAddress(
    [][]byte{[]byte("escrow"), campaignPDA.Bytes()},
    programID,
)

var githubUserSeed [8]byte
binary.LittleEndian.PutUint64(githubUserSeed[:], githubUserID)
claimRecordPDA, _, err := solana.FindProgramAddress(
    [][]byte{[]byte("claim"), campaignPDA.Bytes(), githubUserSeed[:]},
    programID,
)
```

## Build, Test, and Deploy

Local Anchor workflow:

```bash
cd program
npm install
anchor build
anchor test
```

Docker-based check workflow from the repository root:

```bash
docker compose --profile deploy run --rm solana-check
```

Docker-based devnet deploy workflow:

```bash
docker compose --profile deploy run --rm solana-deployer
```

After deployment, set the backend runtime program id:

```env
PROGRAM_ID=<deployed-program-id>
```

## Current Integration Notes

- The program is close to the MVP smart-contract spec: escrowed SOL at creation, service fee separation, deadline-gated finalization, claim records by GitHub user id, double-claim prevention, claim-window expiry, and delayed refunds are implemented on-chain.
- The backend/frontend currently ship the user-paid claim path. Backend-paid claim is accepted by the program as a payer mode, but fee-payer selection must be implemented off-chain by transaction construction.
- The program supports finalization batches through `has_more`; durable backend multi-batch orchestration is not fully wired in the active backend finalization path.
- The on-chain `close_unfinalizable_campaign` instruction exists, but backend automation/operator API for using it is not exposed as a complete flow.
- The program does not enforce GitHub identity. It relies on the backend and `claim_authority` to only construct/authorize valid claim transactions for the authenticated GitHub user.
