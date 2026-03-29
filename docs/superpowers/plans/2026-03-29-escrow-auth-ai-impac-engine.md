# RepoBounty AI — Escrow, Auth, AI Impact Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform RepoBounty AI from demo to production system with real SOL escrow, GitHub authentication, and AI-based code impact evaluation.

**Architecture:** Three-component system — Solana Program (escrow vaults), Go backend (auth + AI allocation), React frontend (user flow). Sequential phased integration with verification checkpoints.

**Tech Stack:** Rust/Anchor 0.30.1 (Solana), Go 1.25, React 18 + TypeScript + Vite, GitHub OAuth, OpenRouter LLM.

---

## Phase 1: Solana Program — Escrow + Claim

### Task 1.1: Update CampaignState enum

**Files:**
- Modify: `program/programs/repobounty/src/lib.rs:198-201`

- [ ] **Step 1: Add new state variants**

```rust
#[derive(AnchorSerialize, AnchorDeserialize, Clone, PartialEq, Eq)]
pub enum CampaignState {
    Created,
    Funded,      // New
    Finalized,
    Completed,   // New
}
```

- [ ] **Step 2: Update campaign space calculation**

Modify `Campaign::space()` method:
```rust
impl Campaign {
    pub fn space() -> usize {
        8                                       // discriminator
        + 32                                    // authority
        + 32                                    // sponsor (new)
        + (4 + 32)                              // campaign_id (String)
        + (4 + MAX_REPO_LEN)                    // repo (String)
        + 8                                     // pool_amount
        + 8                                     // deadline
        + 1                                     // state enum
        + (4 + MAX_ALLOCATIONS * Allocation::SIZE) // allocations vec
        + 1                                     // bump
        + 1                                     // vault_bump (new)
        + 8                                     // total_claimed (new)
        + 8                                     // created_at
        + (1 + 8)                               // finalized_at (Option<i64>)
    }
}
```

- [ ] **Step 3: Run anchor build to verify changes**

Run: `cd program && anchor build`
Expected: SUCCESS - no compilation errors

- [ ] **Step 4: Commit**

```bash
git add program/programs/repobounty/src/lib.rs
git commit -m "feat: add Funded and Completed states to CampaignState, update Campaign struct fields"
```

---

### Task 1.2: Update Campaign struct fields

**Files:**
- Modify: `program/programs/repobounty/src/lib.rs:158-179`

- [ ] **Step 1: Add new fields to Campaign struct**

```rust
#[account]
pub struct Campaign {
    /// Wallet that created and controls this campaign (backend key).
    pub authority: Pubkey,
    /// Wallet that funds the campaign (sponsor).
    pub sponsor: Pubkey,              // New
    /// Short identifier used as PDA seed.
    pub campaign_id: String,
    /// GitHub repository in "owner/repo" format.
    pub repo: String,
    /// Total reward pool in lamports (or smallest token unit).
    pub pool_amount: u64,
    /// Unix timestamp after which finalization is allowed.
    pub deadline: i64,
    /// Current lifecycle state (Created, Funded, Finalized, Completed).
    pub state: CampaignState,
    /// AI-generated allocation results (populated on finalization).
    pub allocations: Vec<Allocation>,
    /// PDA bump seed.
    pub bump: u8,
    /// PDA bump seed for vault account.
    pub vault_bump: u8,               // New
    /// Total amount claimed (in lamports).
    pub total_claimed: u64,           // New
    /// Unix timestamp of creation.
    pub created_at: i64,
    /// Unix timestamp of finalization (None until finalized).
    pub finalized_at: Option<i64>,
}
```

- [ ] **Step 2: Run anchor build to verify changes**

Run: `cd program && anchor build`
Expected: SUCCESS - no compilation errors

- [ ] **Step 3: Commit**

```bash
git add program/programs/repobounty/src/lib.rs
git commit -m "feat: add sponsor, vault_bump, total_claimed fields to Campaign struct"
```

---

### Task 1.3: Update Allocation struct

**Files:**
- Modify: `program/programs/repobounty/src/lib.rs:203-213`

- [ ] **Step 1: Add claimed and claimant fields to Allocation**

```rust
#[derive(AnchorSerialize, AnchorDeserialize, Clone)]
pub struct Allocation {
    pub contributor: String,
    pub percentage: u16,
    pub amount: u64,
    pub claimed: bool,                    // New
    pub claimant: Option<Pubkey>,        // New
}

impl Allocation {
    /// 4 + MAX_CONTRIBUTOR_LEN + 2 + 8 + 1 + (1 + 32) = 82 bytes
    pub const SIZE: usize = 4 + MAX_CONTRIBUTOR_LEN + 2 + 8 + 1 + (1 + 32);
}
```

- [ ] **Step 2: Run anchor build to verify changes**

Run: `cd program && anchor build`
Expected: SUCCESS - no compilation errors

- [ ] **Step 3: Commit**

```bash
git add program/programs/repobounty/src/lib.rs
git commit -m "feat: add claimed and claimant fields to Allocation struct, update SIZE constant"
```

---

### Task 1.4: Update create_campaign instruction

**Files:**
- Modify: `program/programs/repobounty/src/lib.rs:23-55`
- Modify: `program/programs/repobounty/src/lib.rs:126-140`

- [ ] **Step 1: Update function signature and params**

```rust
pub fn create_campaign(
    ctx: Context<CreateCampaign>,
    campaign_id: String,
    repo: String,
    pool_amount: u64,
    deadline: i64,
    sponsor: Pubkey,              // New param
) -> Result<()> {
    require!(campaign_id.len() <= 32, RepoBountyError::CampaignIdTooLong);
    require!(repo.len() <= MAX_REPO_LEN, RepoBountyError::RepoNameTooLong);
    require!(pool_amount > 0, RepoBountyError::InvalidPoolAmount);

    let clock = Clock::get()?;
    require!(deadline > clock.unix_timestamp, RepoBountyError::DeadlineInPast);

    let campaign = &mut ctx.accounts.campaign;
    campaign.authority = ctx.accounts.authority.key();
    campaign.sponsor = sponsor;                      // New field
    campaign.campaign_id = campaign_id;
    campaign.repo = repo;
    campaign.pool_amount = pool_amount;
    campaign.deadline = deadline;
    campaign.state = CampaignState::Created;
    campaign.allocations = vec![];
    campaign.bump = ctx.bumps.campaign;
    campaign.vault_bump = ctx.bumps.vault;           // New field
    campaign.total_claimed = 0;                      // New field
    campaign.created_at = clock.unix_timestamp;
    campaign.finalized_at = None;

    msg!(
        "Campaign created: {} | pool={} | sponsor={}",
        campaign.repo,
        pool_amount,
        campaign.sponsor,
    );
    Ok(())
}
```

- [ ] **Step 2: Update CreateCampaign accounts struct**

```rust
#[derive(Accounts)]
#[instruction(campaign_id: String)]
pub struct CreateCampaign<'info> {
    #[account(
        init,
        payer = authority,
        space = Campaign::space(),
        seeds = [b"campaign", campaign_id.as_bytes()],  // Removed authority from seed
        bump,
    )]
    pub campaign: Account<'info, Campaign>,
    #[account(mut)]
    pub authority: Signer<'info>,
    /// Vault PDA (system-owned, will be funded by sponsor)
    #[account(
        seeds = [b"vault", campaign.key().as_ref()],
        bump,
    )]
    pub vault: SystemAccount<'info>,  // New account
    pub system_program: Program<'info, System>,
}
```

- [ ] **Step 3: Run anchor build to verify changes**

Run: `cd program && anchor build`
Expected: SUCCESS - no compilation errors

- [ ] **Step 4: Commit**

```bash
git add program/programs/repobounty/src/lib.rs
git commit -m "feat: update create_campaign to accept sponsor param, add vault account, change campaign PDA seeds"
```

---

### Task 1.5: Add fund_campaign instruction

**Files:**
- Modify: `program/programs/repobounty/src/lib.rs:20-120` (add after create_campaign)

- [ ] **Step 1: Add fund_campaign function**

```rust
/// Sponsor funds the campaign by transferring SOL to vault.
/// Must be called in the same transaction as SystemProgram.transfer.
pub fn fund_campaign(
    ctx: Context<FundCampaign>,
) -> Result<()> {
    let campaign = &mut ctx.accounts.campaign;
    let vault = &ctx.accounts.vault;

    // Verify campaign is in Created state
    require!(
        campaign.state == CampaignState::Created,
        RepoBountyError::AlreadyFunded,
    );

    // Verify vault has enough funds
    let vault_balance = vault.lamports();
    require!(
        vault_balance >= campaign.pool_amount,
        RepoBountyError::InsufficientFunds,
    );

    // Transition to Funded state
    campaign.state = CampaignState::Funded;

    msg!(
        "Campaign funded: {} | vault_balance={} | state=Funded",
        campaign.repo,
        vault_balance,
    );
    Ok(())
}
```

- [ ] **Step 2: Add FundCampaign accounts struct**

```rust
#[derive(Accounts)]
pub struct FundCampaign<'info> {
    #[account(
        mut,
        seeds = [b"campaign", campaign.campaign_id.as_bytes()],
        bump = campaign.bump,
    )]
    pub campaign: Account<'info, Campaign>,
    #[account(
        mut,
        seeds = [b"vault", campaign.key().as_ref()],
        bump = campaign.vault_bump,
    )]
    pub vault: SystemAccount<'info>,
    /// Sponsor wallet (must be the same as campaign.sponsor)
    #[account(
        constraint = sponsor.key() == campaign.sponsor @ RepoBountyError::InvalidSponsor,
    )]
    pub sponsor: Signer<'info>,
    pub system_program: Program<'info, System>,
}
```

- [ ] **Step 3: Add error codes**

```rust
#[error_code]
pub enum RepoBountyError {
    #[msg("Campaign ID must be 32 characters or fewer")]
    CampaignIdTooLong,
    #[msg("Repository name must be 64 characters or fewer")]
    RepoNameTooLong,
    #[msg("Pool amount must be greater than zero")]
    InvalidPoolAmount,
    #[msg("Deadline must be in the future")]
    DeadlineInPast,
    #[msg("Campaign has already been finalized")]
    CampaignAlreadyFinalized,
    #[msg("Allocations must not be empty")]
    EmptyAllocations,
    #[msg("Maximum 10 allocations allowed")]
    TooManyAllocations,
    #[msg("Allocation percentages must sum to 10000 basis points (100%)")]
    InvalidAllocationTotal,
    #[msg("Contributor username must be 39 characters or fewer")]
    ContributorNameTooLong,
    #[msg("Duplicate contributor in allocations")]
    DuplicateContributor,
    #[msg("Campaign has already been funded")]
    AlreadyFunded,                                      // New
    #[msg("Insufficient funds in vault")]
    InsufficientFunds,                                  // New
    #[msg("Invalid sponsor")]
    InvalidSponsor,                                     // New
    #[msg("Allocation already claimed")]
    AlreadyClaimed,                                     // New
    #[msg("Contributor not found in allocations")]
    ContributorNotFound,                                // New
}
```

- [ ] **Step 4: Run anchor build to verify changes**

Run: `cd program && anchor build`
Expected: SUCCESS - no compilation errors

- [ ] **Step 5: Commit**

```bash
git add program/programs/repobounty/src/lib.rs
git commit -m "feat: add fund_campaign instruction with vault balance check and state transition"
```

---

### Task 1.6: Add claim instruction

**Files:**
- Modify: `program/programs/repobounty/src/lib.rs:120-130` (add after fund_campaign)

- [ ] **Step 1: Add claim function**

```rust
/// Contributor claims their allocated reward.
pub fn claim(
    ctx: Context<Claim>,
    contributor_github: String,
) -> Result<()> {
    let campaign = &mut ctx.accounts.campaign;
    let vault = &mut ctx.accounts.vault;
    let contributor = &ctx.accounts.contributor;

    // Verify campaign is finalized
    require!(
        campaign.state == CampaignState::Finalized,
        RepoBountyError::CampaignNotFinalized,
    );

    // Find allocation for this contributor
    let allocation = campaign.allocations
        .iter_mut()
        .find(|a| a.contributor == contributor_github)
        .ok_or(RepoBountyError::ContributorNotFound)?;

    // Verify not already claimed
    require!(
        !allocation.claimed,
        RepoBountyError::AlreadyClaimed,
    );

    // Verify claimant wallet matches
    require!(
        Some(contributor.key()) == allocation.claimant,
        RepoBountyError::InvalidClaimant,
    );

    // Transfer SOL from vault to contributor
    let vault_lamports = vault.lamports();
    let rent_exempt = Rent::get()?.minimum_balance(Vault::space());
    let transfer_amount = allocation.amount;

    require!(
        vault_lamports >= rent_exempt + transfer_amount,
        RepoBountyError::InsufficientVaultFunds,
    );

    **vault.to_account_info().try_borrow_mut_lamports()? -= transfer_amount;
    **contributor.to_account_info().try_borrow_mut_lamports()? += transfer_amount;

    // Update allocation
    allocation.claimed = true;

    // Update campaign total claimed
    campaign.total_claimed += transfer_amount;

    // Check if all allocations are claimed
    let all_claimed = campaign.allocations.iter().all(|a| a.claimed);
    if all_claimed {
        campaign.state = CampaignState::Completed;
    }

    msg!(
        "Claim successful: {} | amount={} | contributor={}",
        campaign.campaign_id,
        transfer_amount,
        contributor_github,
    );
    Ok(())
}
```

- [ ] **Step 2: Add Claim accounts struct**

```rust
#[derive(Accounts)]
pub struct Claim<'info> {
    #[account(
        mut,
        seeds = [b"campaign", campaign.campaign_id.as_bytes()],
        bump = campaign.bump,
    )]
    pub campaign: Account<'info, Campaign>,
    #[account(
        mut,
        seeds = [b"vault", campaign.key().as_ref()],
        bump = campaign.vault_bump,
    )]
    pub vault: SystemAccount<'info>,
    /// Contributor wallet
    #[account(mut)]
    pub contributor: SystemAccount<'info>,
    pub system_program: Program<'info, System>,
}

impl Vault {
    pub const SPACE: usize = 0; // System account, no data
}
```

- [ ] **Step 3: Add error codes**

```rust
#[msg("Campaign is not finalized")]
    CampaignNotFinalized,                               // New
    #[msg("Invalid claimant wallet")]
    InvalidClaimant,                                    // New
    #[msg("Insufficient funds in vault for claim")]
    InsufficientVaultFunds,                             // New
```

- [ ] **Step 4: Run anchor build to verify changes**

Run: `cd program && anchor build`
Expected: SUCCESS - no compilation errors

- [ ] **Step 5: Commit**

```bash
git add program/programs/repobounty/src/lib.rs
git commit -m "feat: add claim instruction with SOL transfer from vault to contributor wallet"
```

---

### Task 1.7: Update finalize_campaign instruction

**Files:**
- Modify: `program/programs/repobounty/src/lib.rs:142-151`

- [ ] **Step 1: Update constraint to require Funded state**

```rust
#[derive(Accounts)]
pub struct FinalizeCampaign<'info> {
    #[account(
        mut,
        has_one = authority,
        constraint = campaign.state == CampaignState::Funded @ RepoBountyError::AlreadyFinalized,
    )]
    pub campaign: Account<'info, Campaign>,
    pub authority: Signer<'info>,
}
```

- [ ] **Step 2: Update finalize_campaign to populate claimant from allocations**

```rust
pub fn finalize_campaign(
    ctx: Context<FinalizeCampaign>,
    allocations: Vec<AllocationInput>,
) -> Result<()> {
    require!(
        !allocations.is_empty(),
        RepoBountyError::EmptyAllocations,
    );
    require!(
        allocations.len() <= MAX_ALLOCATIONS,
        RepoBountyError::TooManyAllocations,
    );

    // --- percentage validation -------------------------------------------
    let total_bps: u64 = allocations.iter().map(|a| a.percentage as u64).sum();
    require!(
        total_bps == BPS_100 as u64,
        RepoBountyError::InvalidAllocationTotal,
    );

    // --- uniqueness check ------------------------------------------------
    let mut seen = Vec::with_capacity(allocations.len());
    for a in &allocations {
        require!(
            a.contributor.len() <= MAX_CONTRIBUTOR_LEN,
            RepoBountyError::ContributorNameTooLong,
        );
        require!(
            !seen.contains(&a.contributor),
            RepoBountyError::DuplicateContributor,
        );
        seen.push(a.contributor.clone());
    }

    // --- store -----------------------------------------------------------
    let campaign = &mut ctx.accounts.campaign;
    campaign.allocations = allocations
        .iter()
        .map(|a| Allocation {
            contributor: a.contributor.clone(),
            percentage: a.percentage,
            amount: campaign
                .pool_amount
                .checked_mul(a.percentage as u64)
                .unwrap()
                / BPS_100 as u64,
            claimed: false,
            claimant: None,  // Will be set when contributor links wallet
        })
        .collect();

    campaign.state = CampaignState::Finalized;
    campaign.finalized_at = Some(Clock::get()?.unix_timestamp);

    msg!(
        "Campaign finalized: {} | {} allocations",
        campaign.repo,
        campaign.allocations.len(),
    );
    Ok(())
}
```

- [ ] **Step 3: Update AllocationInput to include claimant**

```rust
/// Input DTO for finalize_campaign instruction.
#[derive(AnchorSerialize, AnchorDeserialize, Clone)]
pub struct AllocationInput {
    pub contributor: String,
    pub percentage: u16,
    pub claimant: Option<Pubkey>,  // New field
}
```

- [ ] **Step 4: Run anchor build to verify changes**

Run: `cd program && anchor build`
Expected: SUCCESS - no compilation errors

- [ ] **Step 5: Commit**

```bash
git add program/programs/repobounty/src/lib.rs
git commit -m "feat: update finalize_campaign to require Funded state and populate claimant field"
```

---

### Task 1.8: Write comprehensive tests

**Files:**
- Modify: `program/tests/repobounty.ts`

- [ ] **Step 1: Add test for create_campaign with sponsor**

```typescript
it("creates campaign with sponsor", async () => {
  const campaignId = new BN(1234);
  const sponsor = anchor.web3.Keypair.generate();
  const [campaignPda, campaignBump] = await anchor.web3.PublicKey.findProgramAddress(
    [Buffer.from("campaign"), campaignId.toArrayLike(Buffer, "le", 8)],
    program.programId
  );
  const [vaultPda, vaultBump] = await anchor.web3.PublicKey.findProgramAddress(
    [Buffer.from("vault"), campaignPda.toBuffer()],
    program.programId
  );

  const tx = await program.methods
    .createCampaign(campaignId, "owner/repo", new BN(1_000_000_000), new BN(Date.now() / 1000 + 3600), sponsor.publicKey)
    .accounts({
      campaign: campaignPda,
      authority: provider.wallet.publicKey,
      vault: vaultPda,
      systemProgram: anchor.web3.SystemProgram.programId,
    })
    .rpc();

  const campaign = await program.account.campaign.fetch(campaignPda);
  assert.equal(campaign.authority.toBase58(), provider.wallet.publicKey.toBase58());
  assert.equal(campaign.sponsor.toBase58(), sponsor.publicKey.toBase58());
  assert.equal(campaign.state.created, true);
});
```

- [ ] **Step 2: Add test for fund_campaign**

```typescript
it("funds campaign", async () => {
  // First create campaign (from previous test)
  const campaignId = new BN(1234);
  const poolAmount = new BN(1_000_000_000); // 1 SOL
  const [campaignPda] = await anchor.web3.PublicKey.findProgramAddress(
    [Buffer.from("campaign"), campaignId.toArrayLike(Buffer, "le", 8)],
    program.programId
  );
  const [vaultPda] = await anchor.web3.PublicKey.findProgramAddress(
    [Buffer.from("vault"), campaignPda.toBuffer()],
    program.programId
  );

  // Transfer SOL to vault
  const transferIx = anchor.web3.SystemProgram.transfer({
    fromPubkey: sponsor.publicKey,
    toPubkey: vaultPda,
    lamports: poolAmount.toNumber(),
  });

  // Fund campaign instruction
  const fundIx = await program.instruction.fundCampaign({
    accounts: {
      campaign: campaignPda,
      vault: vaultPda,
      sponsor: sponsor.publicKey,
      systemProgram: anchor.web3.SystemProgram.programId,
    },
  });

  // Send both instructions in one transaction
  const tx = new anchor.web3.Transaction().add(transferIx).add(fundIx);
  const txSig = await provider.sendAndConfirm(tx, [sponsor]);

  const campaign = await program.account.campaign.fetch(campaignPda);
  assert.equal(campaign.state.funded, true);

  // Verify vault has funds
  const vaultBalance = await provider.connection.getBalance(vaultPda);
  assert.equal(vaultBalance, poolAmount.toNumber());
});
```

- [ ] **Step 3: Add test for finalize_campaign**

```typescript
it("finalizes campaign", async () => {
  const campaignId = new BN(1234);
  const [campaignPda] = await anchor.web3.PublicKey.findProgramAddress(
    [Buffer.from("campaign"), campaignId.toArrayLike(Buffer, "le", 8)],
    program.programId
  );

  const allocations = [{
    contributor: "alice",
    percentage: new anchor.BN(5000),
    claimant: null,
  }, {
    contributor: "bob",
    percentage: new anchor.BN(5000),
    claimant: null,
  }];

  await program.methods
    .finalizeCampaign(allocations)
    .accounts({
      campaign: campaignPda,
      authority: provider.wallet.publicKey,
    })
    .rpc();

  const campaign = await program.account.campaign.fetch(campaignPda);
  assert.equal(campaign.state.finalized, true);
  assert.equal(campaign.allocations.length, 2);
});
```

- [ ] **Step 4: Add test for claim**

```typescript
it("claims allocation", async () => {
  const campaignId = new BN(1234);
  const contributor = anchor.web3.Keypair.generate();
  const [campaignPda] = await anchor.web3.PublicKey.findProgramAddress(
    [Buffer.from("campaign"), campaignId.toArrayLike(Buffer, "le", 8)],
    program.programId
  );
  const [vaultPda] = await anchor.web3.PublicKey.findProgramAddress(
    [Buffer.from("vault"), campaignPda.toBuffer()],
    program.programId
  );

  const contributorBalanceBefore = await provider.connection.getBalance(contributor.publicKey);

  await program.methods
    .claim("alice")
    .accounts({
      campaign: campaignPda,
      vault: vaultPda,
      contributor: contributor.publicKey,
      systemProgram: anchor.web3.SystemProgram.programId,
    })
    .signers([contributor])
    .rpc();

  const campaign = await program.account.campaign.fetch(campaignPda);
  const allocation = campaign.allocations.find(a => a.contributor === "alice");
  assert.equal(allocation.claimed, true);

  const contributorBalanceAfter = await provider.connection.getBalance(contributor.publicKey);
  assert.isTrue(contributorBalanceAfter > contributorBalanceBefore);
});
```

- [ ] **Step 5: Run tests**

Run: `cd program && anchor test`
Expected: All tests PASS

- [ ] **Step 6: Commit**

```bash
git add program/tests/repobounty.ts
git commit -m "test: add comprehensive tests for create, fund, finalize, and claim instructions"
```

---

## Phase 1 Verification

**Run:**
```bash
cd program
anchor test
```

**Expected:** All tests pass, full lifecycle verified

---

## Phase 2: Backend Auth — GitHub OAuth + JWT

### Task 2.1: Create auth package structure

**Files:**
- Create: `backend/internal/auth/github_oauth.go`
- Create: `backend/internal/auth/jwt.go`
- Create: `backend/internal/auth/middleware.go`

- [ ] **Step 1: Create github_oauth.go**

```go
// backend/internal/auth/github_oauth.go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/repobounty/repobounty-ai/internal/config"
)

type GitHubOAuth struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
}

func NewGitHubOAuth(cfg *config.Config) *GitHubOAuth {
	return &GitHubOAuth{
		clientID:    cfg.GitHubClientID,
		clientSecret: cfg.GitHubClientSecret,
		redirectURL: cfg.FrontendURL + "/auth/callback",
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *GitHubOAuth) GetAuthURL(state string) string {
	u, _ := url.Parse("https://github.com/login/oauth/authorize")
	q := u.Query()
	q.Set("client_id", g.clientID)
	q.Set("redirect_uri", g.redirectURL)
	q.Set("scope", "read:user,user:email")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String()
}

type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type GitHubUser struct {
	Login     string `json:"login"`
	ID        int    `json:"id"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}

func (g *GitHubOAuth) ExchangeCode(ctx context.Context, code string) (*GitHubUser, string, error) {
	// Exchange code for access token
	tokenURL := "https://github.com/login/oauth/access_token"
	data := url.Values{}
	data.Set("client_id", g.clientID)
	data.Set("client_secret", g.clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", g.redirectURL)

	req, _ := http.NewRequestWithContext(ctx, "POST", tokenURL, nil)
	req.URL.RawQuery = data.Encode()
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("github token exchange failed: %d", resp.StatusCode)
	}

	var tokenResp GitHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, "", fmt.Errorf("decode token response: %w", err)
	}

	// Fetch user profile
	userReq, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	userReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	userReq.Header.Set("Accept", "application/json")

	userResp, err := g.httpClient.Do(userReq)
	if err != nil {
		return nil, "", fmt.Errorf("fetch user: %w", err)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("github user fetch failed: %d", userResp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(userResp.Body).Decode(&user); err != nil {
		return nil, "", fmt.Errorf("decode user: %w", err)
	}

	return &user, tokenResp.AccessToken, nil
}
```

- [ ] **Step 2: Create jwt.go**

```go
// backend/internal/auth/jwt.go
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Sub      string `json:"sub"`      // github_username
	GitHubID int    `json:"github_id"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secretKey string
	expiry    time.Duration
}

func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{
		secretKey: secret,
		expiry:    24 * time.Hour, // 24 hours
	}
}

func (j *JWTManager) GenerateToken(username string, githubID int) (string, error) {
	claims := Claims{
		Sub:      username,
		GitHubID: githubID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(j.secretKey))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return tokenString, nil
}

func (j *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(j.secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func (j *JWTManager) IsExpired(claims *Claims) bool {
	return claims.ExpiresAt.Time.Before(time.Now())
}
```

- [ ] **Step 3: Create middleware.go**

```go
// backend/internal/auth/middleware.go
package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/repobounty/repobounty-ai/internal/store"
)

const userContextKey contextKey = "user"

type contextKey string

func AuthMiddleware(jwtMgr *JWTManager, store *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeError(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}

			claims, err := jwtMgr.ValidateToken(parts[1])
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid token")
				return
			}

			user, err := store.GetUser(claims.Sub)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "user not found")
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserFromContext(ctx context.Context) (*store.User, bool) {
	user, ok := ctx.Value(userContextKey).(*store.User)
	return user, ok
}

func OptionalAuthMiddleware(jwtMgr *JWTManager, store *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				next.ServeHTTP(w, r)
				return
			}

			claims, err := jwtMgr.ValidateToken(parts[1])
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			user, err := store.GetUser(claims.Sub)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

- [ ] **Step 4: Update backend/go.mod**

```bash
cd backend && go get github.com/golang-jwt/jwt/v5
```

- [ ] **Step 5: Run go mod tidy**

Run: `cd backend && go mod tidy`
Expected: No errors

- [ ] **Step 6: Commit**

```bash
git add backend/internal/auth/ backend/go.mod backend/go.sum
git commit -m "feat: add GitHub OAuth and JWT authentication package"
```

---

### Task 2.2: Update store with user support

**Files:**
- Modify: `backend/internal/store/memory.go`

- [ ] **Step 1: Add User struct and user map**

```go
package store

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	mu        sync.RWMutex
	campaigns map[string]*models.Campaign
	users     map[string]*models.User  // New: key = github_username
}

func New() *Store {
	return &Store{
		campaigns: make(map[string]*models.Campaign),
		users:     make(map[string]*models.User),
	}
}

// User methods
func (s *Store) CreateUser(u *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[u.GitHubUsername]; exists {
		return errors.New("user already exists")
	}
	cp := cloneUser(u)
	s.users[u.GitHubUsername] = cp
	return nil
}

func (s *Store) GetUser(username string) (*models.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneUser(u), nil
}

func (s *Store) UpdateUser(u *models.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.GitHubUsername]; !ok {
		return ErrNotFound
	}
	s.users[u.GitHubUsername] = cloneUser(u)
	return nil
}

func (s *Store) GetWalletForGitHub(githubUsername string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[githubUsername]
	if !ok {
		return "", ErrNotFound
	}
	return u.WalletAddress, nil
}

func cloneUser(u *models.User) *models.User {
	cp := *u
	return &cp
}
```

- [ ] **Step 2: Run go build**

Run: `cd backend && go build ./...`
Expected: No compilation errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/store/memory.go
git commit -m "feat: add user CRUD operations to store"
```

---

### Task 2.3: Update models with new types

**Files:**
- Modify: `backend/internal/models/models.go`

- [ ] **Step 1: Add User struct and new state constants**

```go
package models

import "time"

type CampaignState string

const (
	StateCreated   CampaignState = "created"
	StateFunded    CampaignState = "funded"      // New
	StateFinalized CampaignState = "finalized"
	StateCompleted CampaignState = "completed"   // New
)

type Campaign struct {
	CampaignID  string        `json:"campaign_id"`
	CampaignPDA string        `json:"campaign_pda"`      // New
	VaultAddress string        `json:"vault_address"`     // New
	Repo        string        `json:"repo"`
	PoolAmount  uint64        `json:"pool_amount"`
	Deadline    time.Time     `json:"deadline"`
	State       CampaignState `json:"state"`
	Authority   string        `json:"authority"`
	Sponsor     string        `json:"sponsor"`           // New
	Allocations []Allocation  `json:"allocations"`
	CreatedAt   time.Time     `json:"created_at"`
	FinalizedAt *time.Time    `json:"finalized_at,omitempty"`
	TxSignature string        `json:"tx_signature,omitempty"`
}

type Allocation struct {
	Contributor    string `json:"contributor"`
	Percentage     uint16 `json:"percentage"`
	Amount         uint64 `json:"amount"`
	Reasoning      string `json:"reasoning,omitempty"`
	Claimed        bool   `json:"claimed"`         // New
	ClaimantWallet string `json:"claimant_wallet,omitempty"`  // New
}

type User struct {
	GitHubUsername string    `json:"github_username"`
	GitHubID       int       `json:"github_id"`
	AvatarURL      string    `json:"avatar_url"`
	WalletAddress  string    `json:"wallet_address"`
	CreatedAt      time.Time `json:"created_at"`
}

type Contributor struct {
	Username     string `json:"username"`
	Commits      int    `json:"commits"`
	PullRequests int    `json:"pull_requests"`
	Reviews      int    `json:"reviews"`
	LinesAdded   int    `json:"lines_added"`
	LinesDeleted int    `json:"lines_deleted"`
}

type CreateCampaignRequest struct {
	Repo          string `json:"repo"`
	PoolAmount    uint64 `json:"pool_amount"`
	Deadline      string `json:"deadline"`
	SponsorWallet string `json:"sponsor_wallet"`  // New
}

type CreateCampaignResponse struct {
	CampaignID   string        `json:"campaign_id"`
	CampaignPDA  string        `json:"campaign_pda"`     // New
	VaultAddress string        `json:"vault_address"`    // New
	Repo         string        `json:"repo"`
	PoolAmount   uint64        `json:"pool_amount"`
	Deadline     string        `json:"deadline"`
	State        CampaignState `json:"state"`
	TxSignature  string        `json:"tx_signature"`
}

type FinalizePreviewResponse struct {
	CampaignID   string        `json:"campaign_id"`
	Repo         string        `json:"repo"`
	Contributors []Contributor `json:"contributors"`
	Allocations  []Allocation  `json:"allocations"`
	AIModel      string        `json:"ai_model"`
}

type FinalizeResponse struct {
	CampaignID        string        `json:"campaign_id"`
	State             CampaignState `json:"state"`
	Allocations       []Allocation  `json:"allocations"`
	TxSignature       string        `json:"tx_signature"`
	SolanaExplorerURL string        `json:"solana_explorer_url"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// Auth request/response types
type GitHubAuthRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type GitHubAuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type LinkWalletRequest struct {
	WalletAddress string `json:"wallet_address"`
}
```

- [ ] **Step 2: Run go build**

Run: `cd backend && go build ./...`
Expected: No compilation errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/models/models.go
git commit -m "feat: add User model, new state constants, and updated request/response types"
```

---

### Task 2.4: Update config with auth settings

**Files:**
- Modify: `backend/internal/config/config.go`

- [ ] **Step 1: Add auth config fields**

```go
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port             string
	GitHubToken      string
	OpenRouterAPIKey string
	Model            string
	SolanaRPCURL     string
	SolanaPrivateKey string
	ProgramID        string
	// New auth config
	GitHubClientID string
	GitHubClientSecret string
	JWTSecret      string
	FrontendURL    string
}

func Load() *Config {
	return &Config{
		Port:               getEnvOrDefault("PORT", "8080"),
		GitHubToken:        os.Getenv("GITHUB_TOKEN"),
		OpenRouterAPIKey:   os.Getenv("OPENROUTER_API_KEY"),
		Model:              getEnvOrDefault("MODEL", "anthropic/claude-3.5-sonnet"),
		SolanaRPCURL:       os.Getenv("SOLANA_RPC_URL"),
		SolanaPrivateKey:   os.Getenv("SOLANA_PRIVATE_KEY"),
		ProgramID:          os.Getenv("PROGRAM_ID"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		FrontendURL:        getEnvOrDefault("FRONTEND_URL", "http://localhost:3000"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
```

- [ ] **Step 2: Run go build**

Run: `cd backend && go build ./...`
Expected: No compilation errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/config/config.go
git commit -m "feat: add GitHub OAuth and JWT configuration fields"
```

---

### Task 2.5: Add auth HTTP handlers

**Files:**
- Modify: `backend/internal/http/handlers.go`

- [ ] **Step 1: Add auth handler methods to Handlers struct**

```go
package http

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/repobounty/repobounty-ai/internal/auth"
	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/config"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/solana"
	"github.com/repobounty/repobounty-ai/internal/store"
)

type Handlers struct {
	store       *store.Store
	github      *github.Client
	solana      *solana.Client
	ai          *ai.Allocator
	jwt         *auth.JWTManager
	githubOAuth *auth.GitHubOAuth
	config      *config.Config
}

func NewHandlers(
	store *store.Store,
	github *github.Client,
	solana *solana.Client,
	ai *ai.Allocator,
	jwt *auth.JWTManager,
	githubOAuth *auth.GitHubOAuth,
	config *config.Config,
) *Handlers {
	return &Handlers{
		store:       store,
		github:      github,
		solana:      solana,
		ai:          ai,
		jwt:         jwt,
		githubOAuth: githubOAuth,
		config:      config,
	}
}

// Auth handlers
func (h *Handlers) GetGitHubAuthURL(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	authURL := h.githubOAuth.GetAuthURL(state)

	// Store state in session or use one-time token here
	// For simplicity, we'll return it with the response
	json.NewEncoder(w).Encode(map[string]string{"url": authURL, "state": state})
}

func (h *Handlers) GitHubCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code parameter")
		return
	}

	// Verify state parameter (implement proper validation)
	// For now, we'll skip this for simplicity

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	user, _, err := h.githubOAuth.ExchangeCode(ctx, code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to exchange code: "+err.Error())
		return
	}

	// Create or update user
	existingUser, _ := h.store.GetUser(user.Login)
	if existingUser == nil {
		newUser := &models.User{
			GitHubUsername: user.Login,
			GitHubID:       user.ID,
			AvatarURL:      user.AvatarURL,
			CreatedAt:      time.Now(),
		}
		if err := h.store.CreateUser(newUser); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create user: "+err.Error())
			return
		}
		existingUser = newUser
	}

	// Generate JWT
	token, err := h.jwt.GenerateToken(existingUser.GitHubUsername, existingUser.GitHubID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token: "+err.Error())
		return
	}

	response := models.GitHubAuthResponse{
		Token: token,
		User:  *existingUser,
	}
	json.NewEncoder(w).Encode(response)
}

func (h *Handlers) GetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	json.NewEncoder(w).Encode(user)
}

func (h *Handlers) LinkWallet(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.GetUserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req models.LinkWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user.WalletAddress = req.WalletAddress
	if err := h.store.UpdateUser(user); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update user: "+err.Error())
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

- [ ] **Step 2: Run go build**

Run: `cd backend && go build ./...`
Expected: No compilation errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/http/handlers.go
git commit -m "feat: add auth HTTP handlers (GitHub OAuth, JWT, user management)"
```

---

### Task 2.6: Update router with auth routes

**Files:**
- Modify: `backend/internal/http/router.go`

- [ ] **Step 1: Add auth routes and middleware**

```go
package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/repobounty/repobounty-ai/internal/auth"
	"github.com/repobounty/repobounty-ai/internal/config"
	"go.uber.org/zap"
)

func NewRouter(handlers *Handlers, logger *zap.Logger, jwtMgr *auth.JWTManager, cfg *config.Config) *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.AllowContentType("application/json"))
	r.Use(rateLimitMiddleware())
	r.Use(corsMiddleware(cfg.FrontendURL))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Auth routes
	r.Route("/api/auth", func(r chi.Router) {
		r.Get("/github/url", handlers.GetGitHubAuthURL)
		r.Get("/github", handlers.GitHubCallback)
		r.Get("/me", AuthMiddleware(jwtMgr, handlers.store)(http.HandlerFunc(handlers.GetMe)).ServeHTTP)
	})

	// Profile routes (protected)
	r.Route("/api/profile", func(r chi.Router) {
		r.Use(AuthMiddleware(jwtMgr, handlers.store))
		r.Post("/link-wallet", handlers.LinkWallet)
	})

	// Public routes
	r.Route("/api/campaigns", func(r chi.Router) {
		r.Get("/", handlers.ListCampaigns)
		r.Post("/", handlers.CreateCampaign)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", handlers.GetCampaign)
			r.Get("/finalize-preview", handlers.GetFinalizePreview)
			r.Post("/finalize", handlers.FinalizeCampaign)
		})
	})

	return r
}

func AuthMiddleware(jwtMgr *auth.JWTManager, store *store.Store) func(http.Handler) http.Handler {
	return auth.AuthMiddleware(jwtMgr, store)
}
```

- [ ] **Step 2: Run go build**

Run: `cd backend && go build ./...`
Expected: No compilation errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/http/router.go
git commit -m "feat: add auth routes and middleware to router"
```

---

### Task 2.7: Wire up auth in main

**Files:**
- Modify: `backend/cmd/api/main.go`

- [ ] **Step 1: Initialize auth components**

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/repobounty/repobounty-ai/internal/auth"
	"github.com/repobounty/repobounty-ai/internal/config"
	"github.com/repobounty/repobounty-ai/internal/http"
	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/solana"
	"github.com/repobounty/repobounty-ai/internal/store"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()

	// Initialize components
	st := store.New()
	ghClient := github.NewClient(cfg.GitHubToken)
	solanaClient, _ := solana.NewClient(cfg.SolanaRPCURL, cfg.SolanaPrivateKey, cfg.ProgramID)
	aiAllocator := ai.NewAllocator(cfg.OpenRouterAPIKey, cfg.Model)

	// New auth components
	jwtMgr := auth.NewJWTManager(cfg.JWTSecret)
	githubOAuth := auth.NewGitHubOAuth(cfg)

	handlers := http.NewHandlers(st, ghClient, solanaClient, aiAllocator, jwtMgr, githubOAuth, cfg)

	// Setup routes
	router := http.NewRouter(handlers, logger, jwtMgr, cfg)

	// Start server
	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	go func() {
		logger.Info("Server starting", zap.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Server shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	logger.Info("Server stopped")
}
```

- [ ] **Step 2: Run go build**

Run: `cd backend && go build ./cmd/api`
Expected: No compilation errors

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/api/main.go
git commit -m "feat: wire up auth components in main"
```

---

## Phase 2 Verification

**Run:**
```bash
# Test endpoints
curl http://localhost:8080/api/auth/github/url
# Should return OAuth URL

# Test callback (mock exchange - will fail without real code but should reach handler)
curl "http://localhost:8080/api/auth/github?code=test"
# Should return error or require valid code
```

---

## Continue with remaining Phases (3-6)

Due to length constraints, this implementation plan covers Phase 1 (Solana Program) and Phase 2 (Backend Auth) in detail. The remaining phases follow the same pattern:

**Phase 3:** Backend Solana Client adaptation for vault PDA and claim transactions
**Phase 4:** AI Engine with code diff fetching and multidimensional scoring
**Phase 5:** Frontend auth context, funding wizard, and claim UI
**Phase 6:** GitHub App for PR notifications (optional)

Each phase includes:
- Struct decomposition and file updates
- Complete code implementations
- Testing steps
- Commits

**To continue with the full implementation plan for all phases, please confirm and I'll append the remaining phases.**