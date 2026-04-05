package http

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	gosolana "github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/config"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/solana"
	"github.com/repobounty/repobounty-ai/internal/store"
)

func TestCreateCampaignWalletProofFlow(t *testing.T) {
	InitLogger("development")

	dbPath := filepath.Join(t.TempDir(), "repobounty-wallet-proof.db")
	sqliteStore, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer sqliteStore.Close()

	solanaStub := &stubSolanaService{}
	handlers := NewHandlers(
		sqliteStore,
		stubGitHubService{},
		solanaStub,
		ai.NewAllocator("", "test-model"),
		nil,
		nil,
		&config.Config{
			Env:          "test",
			DatabasePath: dbPath,
		},
	)

	router := NewRouter(handlers, "test")

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	walletAddress := gosolana.PublicKeyFromBytes(publicKey).String()
	deadline := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)

	challengeReq := models.CreateCampaignChallengeRequest{
		Repo:          "octocat/Hello-World",
		PoolAmount:    1_000_000_000,
		Deadline:      deadline,
		SponsorWallet: walletAddress,
	}
	challengeRes := performJSONRequest[models.WalletChallengeResponse](
		t,
		router,
		http.MethodPost,
		"/api/campaigns/create-challenge",
		"",
		challengeReq,
		http.StatusCreated,
	)
	if challengeRes.ChallengeID == "" {
		t.Fatal("challenge_id was empty")
	}

	signature := base58.Encode(ed25519.Sign(privateKey, []byte(challengeRes.Message)))
	createReq := models.CreateCampaignRequest{
		Repo:          challengeReq.Repo,
		PoolAmount:    challengeReq.PoolAmount,
		Deadline:      deadline,
		SponsorWallet: walletAddress,
		ChallengeID:   challengeRes.ChallengeID,
		Signature:     signature,
	}
	createRes := performJSONRequest[models.CreateCampaignResponse](
		t,
		router,
		http.MethodPost,
		"/api/campaigns/",
		"",
		createReq,
		http.StatusOK,
	)
	if createRes.CampaignID == "" {
		t.Fatal("campaign_id was empty")
	}
	if _, err := strconv.ParseUint(createRes.CampaignID, 10, 64); err != nil {
		t.Fatalf("campaign_id must be numeric for V2 Solana PDAs, got %q: %v", createRes.CampaignID, err)
	}
	if createRes.UnsignedTx == "" {
		t.Fatal("unsigned_tx was empty")
	}
	if _, err := sqliteStore.Get(createRes.CampaignID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected no stored campaign before on-chain confirmation, got err=%v", err)
	}

	challenge, err := sqliteStore.GetWalletChallenge(challengeRes.ChallengeID)
	if err != nil {
		t.Fatalf("GetWalletChallenge: %v", err)
	}
	if challenge.UsedAt == nil {
		t.Fatal("challenge was not marked used")
	}

	solanaStub.onChainCampaign = &models.Campaign{
		CampaignID:        createRes.CampaignID,
		CampaignPDA:       "campaign-pda",
		EscrowPDA:         "escrow-pda",
		VaultAddress:      "escrow-pda",
		GithubRepoID:      123456,
		PoolAmount:        challengeReq.PoolAmount,
		TotalRewardAmount: challengeReq.PoolAmount,
		Deadline:          deadlineTime(deadline),
		DeadlineAt:        deadlineTime(deadline),
		ClaimDeadlineAt:   deadlineTime(deadline).Add(365 * 24 * time.Hour),
		State:             models.StateFunded,
		Status:            models.StateActive,
		Sponsor:           walletAddress,
		CreatedAt:         time.Now().UTC(),
	}

	confirmReq := struct {
		Repo          string `json:"repo"`
		PoolAmount    uint64 `json:"pool_amount"`
		Deadline      string `json:"deadline"`
		SponsorWallet string `json:"sponsor_wallet"`
		TxSignature   string `json:"tx_signature"`
	}{
		Repo:          challengeReq.Repo,
		PoolAmount:    challengeReq.PoolAmount,
		Deadline:      deadline,
		SponsorWallet: walletAddress,
		TxSignature:   "mock-signature",
	}

	confirmRes := performJSONRequest[models.CreateCampaignResponse](
		t,
		router,
		http.MethodPost,
		"/api/campaigns/"+createRes.CampaignID+"/create-confirm",
		"",
		confirmReq,
		http.StatusCreated,
	)
	if confirmRes.CampaignID != createRes.CampaignID {
		t.Fatalf("confirm campaign_id = %q, want %q", confirmRes.CampaignID, createRes.CampaignID)
	}

	storedCampaign, err := sqliteStore.Get(createRes.CampaignID)
	if err != nil {
		t.Fatalf("Get stored campaign after confirm: %v", err)
	}
	if storedCampaign.Repo != challengeReq.Repo {
		t.Fatalf("stored repo = %q, want %q", storedCampaign.Repo, challengeReq.Repo)
	}
	if storedCampaign.OwnerGitHubUsername != "" {
		t.Fatalf("expected empty owner github username for wallet-only create, got %q", storedCampaign.OwnerGitHubUsername)
	}
}

func TestCreateCampaignRejectsInsufficientSponsorBalanceForFullCreateCost(t *testing.T) {
	InitLogger("development")

	dbPath := filepath.Join(t.TempDir(), "repobounty-wallet-proof-balance.db")
	sqliteStore, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer sqliteStore.Close()

	solanaStub := &stubSolanaService{
		balance:              1_000_000_000,
		requiredCreateAmount: 1_100_000_000,
	}
	handlers := NewHandlers(
		sqliteStore,
		stubGitHubService{},
		solanaStub,
		ai.NewAllocator("", "test-model"),
		nil,
		nil,
		&config.Config{
			Env:          "test",
			DatabasePath: dbPath,
		},
	)

	router := NewRouter(handlers, "test")

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	walletAddress := gosolana.PublicKeyFromBytes(publicKey).String()
	deadline := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)

	challengeReq := models.CreateCampaignChallengeRequest{
		Repo:          "octocat/Hello-World",
		PoolAmount:    1_000_000_000,
		Deadline:      deadline,
		SponsorWallet: walletAddress,
	}
	challengeRes := performJSONRequest[models.WalletChallengeResponse](
		t,
		router,
		http.MethodPost,
		"/api/campaigns/create-challenge",
		"",
		challengeReq,
		http.StatusCreated,
	)

	signature := base58.Encode(ed25519.Sign(privateKey, []byte(challengeRes.Message)))
	createReq := models.CreateCampaignRequest{
		Repo:          challengeReq.Repo,
		PoolAmount:    challengeReq.PoolAmount,
		Deadline:      deadline,
		SponsorWallet: walletAddress,
		ChallengeID:   challengeRes.ChallengeID,
		Signature:     signature,
	}

	payload, err := json.Marshal(createReq)
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "/api/campaigns/", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	var apiErr models.ErrorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&apiErr); err != nil {
		t.Fatalf("Decode error response: %v", err)
	}
	if apiErr.Error == "" {
		t.Fatal("expected non-empty API error")
	}
}

func deadlineTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed.UTC()
}

type stubGitHubService struct{}

func (stubGitHubService) RepositoryExists(ctx context.Context, repo string) (bool, error) {
	return true, nil
}

func (stubGitHubService) RepositoryID(ctx context.Context, repo string) (uint64, error) {
	return 123456, nil
}

func (stubGitHubService) FetchContributionWindowData(
	ctx context.Context,
	repo string,
	windowStart time.Time,
	windowEnd time.Time,
) (*github.ContributionWindowData, error) {
	return &github.ContributionWindowData{}, nil
}

type stubSolanaService struct {
	onChainCampaign      *models.Campaign
	balance              uint64
	requiredCreateAmount uint64
}

func (*stubSolanaService) IsConfigured() bool {
	return true
}

func (*stubSolanaService) AuthorityAddress() string {
	return "mock-authority"
}

func (*stubSolanaService) ListCampaigns(ctx context.Context) ([]*models.Campaign, error) {
	return nil, nil
}

func (s *stubSolanaService) GetCampaign(ctx context.Context, campaignID string) (*models.Campaign, error) {
	if s.onChainCampaign != nil && s.onChainCampaign.CampaignID == campaignID {
		cp := *s.onChainCampaign
		return &cp, nil
	}
	return nil, errors.New("campaign not found")
}

func (s *stubSolanaService) GetBalance(ctx context.Context, wallet string) (uint64, error) {
	if s.balance != 0 {
		return s.balance, nil
	}
	return 10_000_000_000, nil
}

func (s *stubSolanaService) EstimateCreateCampaignCost(ctx context.Context, rewardAmount uint64) (uint64, error) {
	if s.requiredCreateAmount != 0 {
		return s.requiredCreateAmount, nil
	}
	return rewardAmount, nil
}

func (s *stubSolanaService) BuildFundTransaction(
	ctx context.Context,
	campaignID string,
	poolAmount uint64,
	deadline int64,
	githubRepoID uint64,
	sponsorPubkey string,
) (*solana.FundTransaction, error) {
	return &solana.FundTransaction{
		Transaction:  "unsigned-transaction",
		CampaignPDA:  "campaign-pda",
		VaultAddress: "escrow-pda",
		EscrowPDA:    "escrow-pda",
	}, nil
}

func (*stubSolanaService) FinalizeCampaign(
	ctx context.Context,
	campaignID string,
	sponsor string,
	allocations []solana.AllocationInput,
) (string, error) {
	return "", errors.New("not implemented")
}

func (*stubSolanaService) BuildClaimTransaction(
	ctx context.Context,
	campaignID string,
	sponsor string,
	githubUserID uint64,
	userWallet string,
) (string, error) {
	return "", errors.New("not implemented")
}

func (*stubSolanaService) GetClaimStatus(
	ctx context.Context,
	campaignID string,
	sponsor string,
	githubUserID uint64,
) (*solana.ClaimStatus, error) {
	return nil, errors.New("not implemented")
}

func performJSONRequest[T any](
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	token string,
	body any,
	wantStatus int,
) T {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}

	req, err := http.NewRequest(method, path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	res := recorder.Result()
	defer res.Body.Close()

	if res.StatusCode != wantStatus {
		var apiErr models.ErrorResponse
		_ = json.NewDecoder(res.Body).Decode(&apiErr)
		t.Fatalf("status = %d, want %d (error=%q)", res.StatusCode, wantStatus, apiErr.Error)
	}

	var decoded T
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		t.Fatalf("Decode response: %v", err)
	}
	return decoded
}
