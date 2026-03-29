# RepoBounty AI Implementation Plan — Escrow, Auth, AI Impact Engine

**Goal:** Transform RepoBounty AI from demo to production system with real SOL escrow, GitHub authentication, and AI-based code impact evaluation.

**Architecture:** Three-component system — Solana Program (escrow vaults), Go backend (auth + AI allocation), React frontend (user flow). Sequential phased integration with verification checkpoints.

**Tech Stack:** Rust/Anchor 0.30.1 (Solana), Go 1.25, React 18 + TypeScript + Vite, GitHub OAuth, OpenRouter LLM.

---

## Phase 1: Solana Program — Escrow + Claim

### State Machine

```
Created → Funded → Finalized → Completed
```

### Account Architecture

**Campaign PDA** — seeds: `["campaign", campaign_id]` (authority removed from seed)

```rust
pub struct Campaign {
    pub authority: Pubkey,           // Backend key (can finalize)
    pub sponsor: Pubkey,             // Sponsor wallet (who funds)
    pub campaign_id: String,
    pub repo: String,
    pub pool_amount: u64,
    pub deadline: i64,
    pub state: CampaignState,        // New: Funded, Completed
    pub allocations: Vec<Allocation>,
    pub bump: u8,
    pub vault_bump: u8,              // New: for vault PDA derivation
    pub total_claimed: u64,          // New: track claimed amounts
    pub created_at: i64,
    pub finalized_at: Option<i64>,
}
```

**Vault PDA** — seeds: `["vault", campaign_pda]`, system-owned, stores SOL

**Allocation struct** — new fields:

```rust
pub struct Allocation {
    pub contributor: String,
    pub percentage: u16,
    pub amount: u64,
    pub claimed: bool,               // New
    pub claimant: Option<Pubkey>,   // New
}
```

### New Instructions

**`fund_campaign`** — validates vault has required funds, state → Funded

```rust
pub fn fund_campaign(ctx: Context<FundCampaign>) -> Result<()> {
    let vault_balance = ctx.accounts.vault.get_lamports();
    require!(vault_balance >= ctx.accounts.campaign.pool_amount,
             RepoBountyError::InsufficientFunds);
    ctx.accounts.campaign.state = CampaignState::Funded;
    Ok(())
}
```

**`claim`** — contributor claims their allocation

```rust
pub fn claim(
    ctx: Context<Claim>,
    contributor_github: String,
) -> Result<()> {
    // Find allocation by github username
    // Verify !claimed
    // invoke_signed: vault → contributor_wallet transfer
    // claimed = true, claimant = Some(wallet)
    // total_claimed += amount
    // If all claimed → state = Completed
}
```

**Updated `finalize_campaign`** — constraint changes to `state == Funded`

### Tests

File: `program/tests/repobounty.ts`

- Full lifecycle: create → fund → finalize → claim
- Double-claim rejection
- Partial claims with state transition to Completed
- Insufficient funds rejection

---

## Phase 2: Backend Auth — GitHub OAuth + JWT

### Package Structure

```
backend/internal/auth/
  ├── github_oauth.go    // OAuth flow, token exchange
  ├── jwt.go             // JWT generation/validation
  └── middleware.go      // Chi auth middleware
```

### Auth Flow

1. `GET /api/auth/github/url` → Returns GitHub OAuth URL
2. `GET /api/auth/github?code=XXX` → Exchanges code → Returns `{token: "jwt...", user: {...}}`
3. `GET /api/auth/me` → Protected endpoint, validates JWT → Returns current user
4. `POST /api/profile/link-wallet` → Links wallet to user (protected)

### JWT Implementation

```go
type Claims struct {
    Sub         string `json:"sub"`          // github_username
    GitHubID    int    `json:"github_id"`
    github_jwt.StandardClaims
}

func GenerateJWT(username string, githubID int) (string, error)
func ValidateJWT(token string) (*Claims, error)
```

### Middleware

```go
func AuthMiddleware(next http.Handler) http.Handler {
    // Extract Authorization: Bearer <jwt>
    // Validate JWT
    // Set claims in context: context.WithValue(ctx, "user", claims)
}
```

### Store Extension

File: `backend/internal/store/memory.go`

```go
type User struct {
    GitHubUsername  string    `json:"github_username"`
    GitHubID        int       `json:"github_id"`
    AvatarURL       string    `json:"avatar_url"`
    WalletAddress   string    `json:"wallet_address"`
    CreatedAt       time.Time `json:"created_at"`
}

// Add to Store struct:
users map[string]*User

// Methods:
CreateUser(u *User) error
GetUser(username string) (*User, error)
UpdateUser(u *User) error
GetWalletForGitHub(username string) (string, error)
```

### New Models

File: `backend/internal/models/models.go`

```go
const (
    StateCreated   CampaignState = "created"
    StateFunded    CampaignState = "funded"      // New
    StateFinalized CampaignState = "finalized"
    StateCompleted CampaignState = "completed"   // New
)

type User struct {
    GitHubUsername  string    `json:"github_username"`
    GitHubID        int       `json:"github_id"`
    AvatarURL       string    `json:"avatar_url"`
    WalletAddress   string    `json:"wallet_address"`
    CreatedAt       time.Time `json:"created_at"`
}

type Allocation struct {
    Contributor     string `json:"contributor"`
    Percentage      uint16 `json:"percentage"`
    Amount          uint64 `json:"amount"`
    Reasoning       string `json:"reasoning,omitempty"`
    Claimed         bool   `json:"claimed"`        // New
    ClaimantWallet  string `json:"claimant_wallet,omitempty"`  // New
}
```

### Config

File: `backend/internal/config/config.go`

Add: `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, `JWT_SECRET`, `FRONTEND_URL`

---

## Phase 3: Backend Solana Client — Escrow + Claim

### PDA Derivation Updates

File: `backend/internal/solana/client.go`

```go
// Campaign seeds changed to ["campaign", campaign_id]
func (c *Client) CreateCampaign(ctx context.Context, campaignID, repo string, poolAmount uint64, deadline int64, sponsorPubkey solana.PublicKey) (string, error)

func GetVaultPDA(campaignPDA solana.PublicKey, programID solana.PublicKey) (solana.PublicKey, u8) {
    // seeds: ["vault", campaign_pda]
}

func ClaimAllocation(ctx context.Context, campaignID, contributorGitHub, contributorWallet string) (string, error)
```

### CreateCampaignResponse Updates

```go
type CreateCampaignResponse struct {
    CampaignID        string        `json:"campaign_id"`
    CampaignPDA       string        `json:"campaign_pda"`      // New
    VaultAddress      string        `json:"vault_address"`     // New
    Repo              string        `json:"repo"`
    PoolAmount        uint64        `json:"pool_amount"`
    Deadline          string        `json:"deadline"`
    State             CampaignState `json:"state"`
    TxSignature       string        `json:"tx_signature"`
}
```

### Account Parsing Updates

Update `decodeCampaignAccount`:

```go
// After reading existing fields:
sponsorBytes, _ := dec.readBytes(32)
sponsor := solana.PublicKeyFromBytes(sponsorBytes)
vaultBump, _ := dec.readU8()
totalClaimed, _ := dec.readU64()

// In allocation loop:
claimedTag, _ := dec.readU8()
claimed := claimedTag == 1

claimantTag, _ := dec.readU8()
var claimant string
if claimantTag == 1 {
    claimantBytes, _ := dec.readBytes(32)
    claimant = solana.PublicKeyFromBytes(claimantBytes).String()
}
```

---

## Phase 4: AI Impact Engine — Code Diff Analysis

### Enhanced GitHub Client

File: `backend/internal/github/client.go`

**New methods:**

```go
type PRDetail struct {
    Number       int
    Title        string
    state        string
    MergedAt     time.Time
    Contributor  string
    Files        []FileDetail
    Reviews      []Review
    Comments     int
    ReviewComments int
}

type FileDetail struct {
    Filename    string
    Patch       string      // Full diff
    Additions   int
    Deletions   int
}

type Review struct {
    Username    string
    State       string      // APPROVED, CHANGES_REQUESTED, COMMENTED
    SubmittedAt time.Time
}

func (c *Client) FetchContributorsDetailed(ctx context.Context, repo string) ([]DetailedContributor, error)
func (c *Client) FetchPRsWithDiffs(ctx context.Context, owner, repo string) (map[string][]PRDetail, error)
func (c *Client) FetchPRDiff(ctx context.Context, owner, repo string, prNumber int) ([]FileDetail, error)
func (c *Client) FetchReviews(ctx context.Context, owner, repo string, prNumber int) ([]Review, error)
```

**Adaptive Selection Logic:**

```go
// PR significance score for sorting
func getPRSignificance(pr PRDetail) float64 {
    return float64(pr.Comments)*3 + float64(pr.ReviewComments)*2 +
           float64(len(pr.Files)) + float64(pr.Additions+pr.Deletions)/100
}

// Take top-5 PRs per contributor by significance
func selectTopPRs(prs []PRDetail) []PRDetail {
    sorted := make([]PRDetail, len(prs))
    copy(sorted, prs)
    sort.Slice(sorted, func(i, j int) bool {
        return getPRSignificance(sorted[i]) > getPRSignificance(sorted[j])
    })
    if len(sorted) > 5 {
        return sorted[:5]
    }
    return sorted
}

// Take top-3 files per PR by additions+deletions
func selectTopFiles(files []FileDetail) []FileDetail {
    sorted := make([]FileDetail, len(files))
    copy(sorted, files)
    sort.Slice(sorted, func(i, j int) bool {
        return (sorted[i].Additions+sorted[i].Deletions) > (sorted[j].Additions+sorted[j].Deletions)
    })
    if len(sorted) > 3 {
        return sorted[:3]
    }
    return sorted
}

// Truncate diff to 50 lines per file
func truncatePatch(patch string) string {
    lines := strings.Split(patch, "\n")
    if len(lines) > 50 {
        return strings.Join(lines[:50], "\n") + "\n... (truncated)"
    }
    return patch
}
```

**Concurrency Control:**

```go
// Limit to 5 parallel requests
const maxConcurrent = 5
var semaphore = make(chan struct{}, maxConcurrent)

// In fetch methods:
semaphore <- struct{}{}
defer func() { <-semaphore }()
```

### Enhanced AI Allocator

File: `backend/internal/ai/allocator.go`

**New prompt structure:**

```go
systemPrompt := `You are a fair open-source contribution evaluator for the RepoBounty AI platform.
Evaluate code contributions across multiple dimensions and allocate reward percentages.

Evaluation dimensions with weights:
1. Impact & Significance (35%) — Does this solve critical problems? Novel algorithms vs CRUD?
2. Code Complexity & Novelty (25%) — Complex logic, unique approaches, non-boilerplate code
3. Scope & Consistency (20%) — Volume of meaningful changes, consistent contribution quality
4. Quality Signals (10%) — Review feedback, test coverage, documentation
5. Community Engagement (10%) — Code reviews, helping other contributors

You will see actual code diffs, not just metrics. Analyze the implementation significance.

Return ONLY a valid JSON array with no extra text.`

userPrompt := fmt.Sprintf(`Repository: %s
Total reward pool: %d lamports

Contributors with code diffs:

%s

Evaluate each contributor based on actual code changes shown above. Return ONLY:
[{
  "contributor": "username",
  "percentage": 5000,
  "scores": {
    "impact": 85,
    "complexity": 90,
    "scope": 60,
    "quality": 70,
    "community": 50
  },
  "reasoning": "Implemented novel caching algorithm that reduced latency 10x by..."
}]`, repo, poolAmount, formatContributorsWithDiffs(detailedContributors))
```

**Response parsing:**

```go
type AIAllocation struct {
    Contributor string                 `json:"contributor"`
    Percentage  int                    `json:"percentage"`
    Scores      map[string]int         `json:"scores"`
    Reasoning   string                 `json:"reasoning"`
}

func formatContributorsWithDiffs(contributors []DetailedContributor) string {
    // Format each contributor's PRs with truncated diffs
    // Include file selections and patch content
}
```

**Deterministic fallback unchanged** — weight = commits*3 + PRs*5 + reviews*2 + filesChanged*2

---

## Phase 5: Frontend — Auth + Funding + Claims

### Auth Context

File: `frontend/src/contexts/AuthContext.tsx`

```typescript
interface AuthContextType {
  user: User | null;
  token: string | null;
  login: () => Promise<void>;
  logout: () => void;
  refreshUser: () => Promise<void>;
  loading: boolean;
}

// JWT stored in localStorage
// Auto-validate on mount
```

### New Pages

**`frontend/src/pages/AuthCallback.tsx`** — OAuth redirect handler:

```typescript
// Extract code from URL query params
// POST /api/auth/github?code=XXX
// Save token to localStorage
// Save user to context
// Redirect to /profile or /
```

**`frontend/src/pages/Profile.tsx`** — User profile:

- Display GitHub info (avatar, username)
- Wallet linkage form (input + save button)
- Claim history list (amount, date, status)
- Logout button

### Updated CreateCampaign.tsx

**Two-step wizard:**

1. **Create Campaign** — HTTP POST to `/api/campaigns`, receives:
   - `campaign_id`
   - `vault_address`
   - `campaign_pda`

2. **Fund Campaign** — Builds and signs transaction:
   ```
   Instruction 1: SystemProgram.transfer(sponsor_wallet → vault_address, pool_amount)
   Instruction 2: program.fund_campaign(campaign_pda, vault_address, sponsor_wallet)
   ```
   - All signed via Phantom wallet
   - Single atomic transaction

### Updated CampaignDetails.tsx

**New features:**

- Display all campaign states: Created, Funded, Finalized, Completed
- For authenticated users: claim buttons for each unclaimed allocation
- Claim flow: check auth → verify wallet linked → execute claim transaction
- Status indicators for allocations (claimed/unclaimed)

### Updated Components

**`frontend/src/components/Layout.tsx`** — Navigation updates:

- GitHub login button (if not auth'd)
- User avatar + dropdown (if auth'd):
  - Profile link
  - Logout button

**`frontend/src/components/GitHubLoginButton.tsx`** — New component:

```typescript
// GET /api/auth/github/url
// Redirect to GitHub OAuth URL
```

**`frontend/src/components/ClaimCard.tsx`** — New component:

- Display allocation info
- Claim status indicator
- Claim button (with auth check)
- Success/error states

### Updated App.tsx

**New routes:**
- `/auth/callback` — OAuth redirect handler
- `/profile` — User profile page

**Layout:**
```typescript
<AuthProvider>
  <Router>
    <Layout>
      <Routes>
        <Route path="/" element={<Home />} />
        <Route path="/campaigns/:id" element={<CampaignDetails />} />
        <Route path="/create" element={<CreateCampaign />} />
        <Route path="/auth/callback" element={<AuthCallback />} />
        <Route path="/profile" element={<Profile />} />
      </Routes>
    </Layout>
  </Router>
</AuthProvider>
```

### API Client Updates

File: `frontend/src/api/client.ts`

```typescript
// JWT header injection
const getHeaders = () => {
  const token = localStorage.getItem('token');
  return token ? { 'Authorization': `Bearer ${token}` } : {};
};

// New methods:
getGitHubAuthURL(): Promise<{url: string}>
githubCallback(code: string): Promise<{token: string, user: User}>
getMe(): Promise<User>
linkWallet(address: string): Promise<void>
listClaims(): Promise<Claim[]>
claim(allocationId: string): Promise<{tx_signature: string}>
```

### Type Updates

File: `frontend/src/types/index.ts`

```typescript
interface User {
  github_username: string;
  github_id: number;
  avatar_url: string;
  wallet_address: string;
  created_at: string;
}

interface Allocation {
  contributor: string;
  percentage: number;
  amount: number;
  reasoning?: string;
  claimed: boolean;
  claimant_wallet: string;
}

interface Campaign {
  campaign_id: string;
  campaign_pda: string;        // New
  vault_address: string;       // New
  repo: string;
  pool_amount: number;
  deadline: string;
  state: "created" | "funded" | "finalized" | "completed";
  authority: string;
  sponsor: string;             // New
  allocations: Allocation[];
  created_at: string;
  finalized_at?: string;
  tx_signature?: string;
}

interface Claim {
  allocation: Allocation;
  campaign_id: string;
  claimed: boolean;
  tx_signature?: string;
}
```

---

## Phase 6: GitHub App — Optional PR Notifications

### Setup

- GitHub App with permissions: `pull_requests: write`, `metadata: read`
- No events needed (we write, don't listen)
- Registration: `https://github.com/settings/apps/new`

### Backend Implementation

**New package:** `backend/internal/githubapp/client.go`

```go
type Client struct {
    appID      int64
    privateKey []byte
    httpClient *http.Client
}

// JWT authentication for GitHub App
func (c *Client) GetInstallationToken(installationID int64) (string, error)

// Comment on PR
func (c *Client) PostAllocationComment(
    ctx context.Context,
    repo string,
    prNumber int,
    contributor string,
    amount float64,
    percentage uint16,
    claimURL string,
) error
```

**Integration in Finalize handler:**

File: `backend/internal/http/handlers.go`

```go
// After successful finalize_campaign:
if githubAppClient != nil {
    // Check if app installed on repo
    installation, err := getAppInstallation(ctx, repo)
    if err == nil && installation != nil {
        // For each contributor:
        // 1. Find their latest merged PR
        // 2. Post comment:
        //    "🎉 @username, you earned X.XX SOL (YY.Y%) for your contributions!"
        //    "→ Claim your reward: https://repobounty.ai/claims"
    }
}
```

### Config

Add optional: `GITHUB_APP_ID`, `GITHUB_APP_PRIVATE_KEY` (PEM format)

### Frontend Integration

Banner on CampaignDetails page (for repo owners):

> "Install RepoBounty GitHub App to automatically notify contributors in PRs after finalization"
>
> [Install App] button

---

## Implementation Order (Phased, Verifiable)

1. **Phase 1: Solana Program** → `anchor test` passes ✅
2. **Phase 2: Backend Auth** → curl OAuth flow works ✅
3. **Phase 3: Backend Solana Client** → create + fund tests pass ✅
4. **Phase 4: AI Engine** → finalization produces allocations with scores ✅
5. **Phase 5: Frontend Integration** → full E2E user journey works ✅
6. **Phase 6: GitHub App** (optional) → PR comments appear after finalization ✅

Each phase independently testable before proceeding to the next.

---

## Error Handling & Security

### Solana Program

- All instructions validate state transitions via error codes
- Vault balance checks before `claim` and `fund_campaign`
- Double-claim prevention with `claimed` flag
- Proper PDA derivation with bump seeds

### Backend Auth

- JWT expiration validation
- Signature verification for OAuth callback
- CSRF protection via `state` parameter in OAuth flow
- Rate limiting on auth endpoints

### Backend API

- Graceful degradation when GitHub/AI/Solana unconfigured
- Generic client error messages, detailed logging
- Context propagation for timeout handling
- Configurable retries with exponential backoff

### Frontend

- Error boundaries for component crashes
- Toast notifications for auth failures
- Wallet connection error handling
- Optimistic UI updates with rollback on failure

---

## Testing Strategy

### Phase 1 Tests

```bash
cd program
anchor test

# Coverage:
# - create_campaign with new seeds
# - fund_campaign (atomic 2-instruction transaction)
# - finalize_campaign (requires Funded state)
# - claim instruction
# - double-claim rejection
# - partial claims → Completed state
# - insufficient funds rejection
```

### Phase 2 Tests

```bash
# OAuth flow
curl http://localhost:8080/api/auth/github/url
# Returns OAuth URL

# Simulate GitHub callback with code
curl "http://localhost:8080/api/auth/github?code=test_code" -v
# Returns JWT token + user info

# Verify JWT
curl http://localhost:8080/api/auth/me -H "Authorization: Bearer <token>"
# Returns user data

# Link wallet
curl http://localhost:8080/api/profile/link-wallet \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{"wallet_address": "test_wallet"}'
```

### Phase 3 Tests

```go
// Backend tests:
func TestGetVaultPDA(t *testing.T)
func TestCreateCampaignWithSponsor(t *testing.T)
func TestClaimAllocation(t *testing.T)
func TestDecodeCampaignAccountNewFields(t *testing.T)
```

### Phase 4 Tests

```go
// GitHub client tests (with mock API):
func TestFetchContributorsDetailed(t *testing.T)
func TestAdaptivePRSelection(t *testing.T)
func TestTruncatePatch(t *testing.T)
func TestConcurrencyLimiting(t *testing.T)

// AI allocator tests (with mocked responses):
func TestAIAllocationWithDiffs(t *testing.T)
func TestMultidimensionalScoring(t *testing.T)
func TestDeterministicFallback(t *testing.T)
```

### Phase 5 E2E Tests

**Manual browser checklist:**
1. Sign in with GitHub OAuth
2. Profile page displays correctly
3. Link wallet to profile
4. Create new campaign
5. Fund campaign via Phantom (2-instruction tx)
6. Finalize campaign (AI allocation)
7. View campaign details (shows "completed" state)
8. Claim reward (if user is contributor)
9. Verify SOL transferred to wallet

### Phase 6 Tests (Optional)

- Verify GitHub App authentication
- Test PR comment creation
- Verify comment appears in real PR
- Test graceful skip when app not installed

---

## Configuration Required

### Solana
- `SOLANA_RPC_URL` — Devnet RPC endpoint
- `SOLANA_PRIVATE_KEY` — Backend keypair (authority)
- `PROGRAM_ID` — `8oSXz4bbvUYVnNruhPEF3JR7jMsSApf7EpAyDpXxDLSJ`

### GitHub OAuth
- `GITHUB_CLIENT_ID` — OAuth App client ID
- `GITHUB_CLIENT_SECRET` — OAuth App secret
- `FRONTEND_URL` — `http://localhost:3000` (for redirect)

### JWT
- `JWT_SECRET` — HS256 signing secret

### AI
- `OPENROUTER_API_KEY` — LLM API key
- `MODEL` — Model name (e.g., `anthropic/claude-3.5-sonnet`)

### GitHub App (Optional)
- `GITHUB_APP_ID` — Numeric App ID
- `GITHUB_APP_PRIVATE_KEY` — PEM format private key

---

## Success Criteria

### Phase 1
- Escrow vault system prevents unauthorized withdrawals
- Atomic funding transaction guarantees fund safety
- Claim mechanism prevents double-spending

### Phase 2
- GitHub OAuth flow works end-to-end
- JWT tokens validate correctly
- Wallet linkage persists across sessions

### Phase 3
- Backend can derive vault addresses correctly
- Campaign creation includes sponsor pubkey
- Claim transactions build and sign correctly

### Phase 4
- AI evaluates actual code diffs, not just metrics
- Adaptive selection prioritizes significant PRs
- Multidimensional scores reflect impact
- Fallback produces deterministic allocations

### Phase 5
- Users can authenticate and link wallets
- Sponsors can create and fund campaigns
- Contributors can claim rewards
- All states display correctly in UI

### Phase 6 (Optional)
- PR notifications appear after finalization
- Graceful degradation when app not installed