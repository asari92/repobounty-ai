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
	"github.com/go-chi/chi/v5"
	"github.com/mr-tron/base58"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/auth"
	"github.com/repobounty/repobounty-ai/internal/config"
	"github.com/repobounty/repobounty-ai/internal/github"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/solana"
	"github.com/repobounty/repobounty-ai/internal/store"
)

func TestCreateCampaignChallengeAcceptsConfiguredMVPDeadline(t *testing.T) {
	cfg := &config.Config{MinCampaignAmount: 500_000_000, MinDeadlineSeconds: 900}
	handlers := NewHandlers(store.New(), stubGitHubService{}, &stubSolanaService{}, ai.NewAllocator("", "test"), nil, nil, cfg)

	req := models.CreateCampaignChallengeRequest{
		Repo:          "octocat/Hello-World",
		PoolAmount:    500_000_000,
		Deadline:      time.Now().UTC().Add(20 * time.Minute).Format(time.RFC3339),
		SponsorWallet: testWalletAddress(t),
	}

	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/api/campaigns/create-challenge", mustJSONBody(t, req))
	handlers.CreateCampaignChallenge(rec, httpReq)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestCreateCampaignChallengeRejectsDeadlineBelowConfiguredMinimum(t *testing.T) {
	cfg := &config.Config{MinCampaignAmount: 500_000_000, MinDeadlineSeconds: 900}
	handlers := NewHandlers(store.New(), stubGitHubService{}, &stubSolanaService{}, ai.NewAllocator("", "test"), nil, nil, cfg)

	req := models.CreateCampaignChallengeRequest{
		Repo:          "octocat/Hello-World",
		PoolAmount:    500_000_000,
		Deadline:      time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
		SponsorWallet: testWalletAddress(t),
	}

	rec := httptest.NewRecorder()
	httpReq := httptest.NewRequest(http.MethodPost, "/api/campaigns/create-challenge", mustJSONBody(t, req))
	handlers.CreateCampaignChallenge(rec, httpReq)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestFundTxReturnsGoneForAtomicCreateFlow(t *testing.T) {
	router := NewRouter(newTestHandlersWithSolana(t), "test")
	rec := performRequest(t, router, http.MethodPost, "/api/campaigns/123/fund-tx", nil, "")

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusGone)
	}
}

func TestListCampaignsHidesStoreOnlyCampaignsWhenSolanaConfigured(t *testing.T) {
	memStore := store.New()
	orphan := &models.Campaign{
		CampaignID: "local-only",
		Repo:       "acme/local-only",
		CreatedAt:  time.Unix(100, 0).UTC(),
	}
	if err := memStore.Create(orphan); err != nil {
		t.Fatalf("store.Create orphan: %v", err)
	}

	onChain := &models.Campaign{
		CampaignID: "chain-1",
		Repo:       "acme/chain",
		CreatedAt:  time.Unix(200, 0).UTC(),
	}

	handlers := NewHandlers(
		memStore,
		nil,
		&stubSolanaService{campaigns: []*models.Campaign{onChain}},
		nil,
		nil,
		nil,
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/campaigns/", nil)
	rec := httptest.NewRecorder()
	handlers.ListCampaigns(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var campaigns []models.Campaign
	if err := json.NewDecoder(rec.Body).Decode(&campaigns); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(campaigns) != 1 {
		t.Fatalf("len(campaigns) = %d, want 1", len(campaigns))
	}
	if campaigns[0].CampaignID != "chain-1" {
		t.Fatalf("campaign_id = %q, want %q", campaigns[0].CampaignID, "chain-1")
	}
}

func mustJSONBody(t *testing.T, v any) *bytes.Reader {
	t.Helper()

	payload, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}
	return bytes.NewReader(payload)
}

func performRequest(t *testing.T, handler http.Handler, method, path string, body []byte, token string) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, path, reader)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func performAuthedJSONRequest(
	t *testing.T,
	handlers *Handlers,
	user *store.User,
	method string,
	path string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()

	if handlers == nil || handlers.jwt == nil {
		t.Fatal("handlers.jwt must be configured for authenticated test requests")
	}

	token, err := handlers.jwt.GenerateToken(user.GitHubUsername)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}

	req, err := http.NewRequest(method, path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	NewRouter(handlers, "test").ServeHTTP(recorder, req)
	return recorder
}

func newTestHandlersWithSolana(t *testing.T) *Handlers {
	t.Helper()
	InitLogger("development")

	return NewHandlers(
		store.New(),
		stubGitHubService{},
		&stubSolanaService{},
		ai.NewAllocator("", "test-model"),
		nil,
		nil,
		&config.Config{
			Env:                "test",
			MinCampaignAmount:  500_000_000,
			MinDeadlineSeconds: 300,
		},
	)
}

func newClaimConfirmHandlersWithOnChainStatus(t *testing.T, claimed bool) (*Handlers, *models.Campaign, *store.User) {
	t.Helper()
	InitLogger("development")

	memStore := store.New()
	user := &store.User{
		GitHubUsername: "alice",
		WalletAddress:  testWalletAddress(t),
		GitHubID:       42,
		AvatarURL:      "https://example.com/avatar.png",
		CreatedAt:      time.Unix(100, 0).UTC(),
	}
	if err := memStore.CreateUser(user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	campaign := &models.Campaign{
		CampaignID: "123",
		Repo:       "octocat/Hello-World",
		Sponsor:    testWalletAddress(t),
		State:      models.StateFunded,
		Status:     models.StateActive,
		Allocations: []models.Allocation{
			{
				Contributor:  "alice",
				GithubUserID: 99,
				Amount:       1_000_000_000,
				Claimed:      false,
			},
			{
				Contributor:  "bob",
				GithubUserID: 100,
				Amount:       1_000_000_000,
				Claimed:      false,
			},
		},
		CreatedAt: time.Unix(200, 0).UTC(),
	}
	if err := memStore.Create(campaign); err != nil {
		t.Fatalf("Create campaign: %v", err)
	}

	solanaStub := &claimConfirmSolanaStub{
		stubSolanaService: stubSolanaService{
			onChainCampaign: &models.Campaign{
				CampaignID:      campaign.CampaignID,
				Repo:            campaign.Repo,
				Sponsor:         campaign.Sponsor,
				State:           models.StateCompleted,
				Status:          models.StateClosed,
				ClaimedAmount:   2_000_000_000,
				ClaimedCount:    2,
				TotalClaimed:    2_000_000_000,
				Allocations:     nil,
				CreatedAt:       campaign.CreatedAt,
				DeadlineAt:      time.Unix(300, 0).UTC(),
				ClaimDeadlineAt: time.Unix(400, 0).UTC(),
			},
		},
		claimStatus: &solana.ClaimStatus{
			Claimed:         claimed,
			RecipientWallet: user.WalletAddress,
			Amount:          campaign.Allocations[0].Amount,
		},
	}

	handlers := NewHandlers(
		memStore,
		stubGitHubService{},
		solanaStub,
		ai.NewAllocator("", "test-model"),
		auth.NewJWTManager("test-secret"),
		nil,
		&config.Config{
			Env:               "test",
			MinCampaignAmount: 500_000_000,
		},
	)

	return handlers, campaign, user
}

func testWalletAddress(t *testing.T) string {
	t.Helper()

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return gosolana.PublicKeyFromBytes(publicKey).String()
}

type claimConfirmSolanaStub struct {
	stubSolanaService
	claimStatus *solana.ClaimStatus
}

func (s *claimConfirmSolanaStub) GetClaimStatus(
	ctx context.Context,
	campaignID string,
	sponsor string,
	githubUserID uint64,
) (*solana.ClaimStatus, error) {
	if s.claimStatus == nil {
		return nil, errors.New("not implemented")
	}
	status := *s.claimStatus
	return &status, nil
}

func TestGetCampaignReturnsNotFoundForStoreOnlyCampaignWhenSolanaConfigured(t *testing.T) {
	memStore := store.New()
	orphan := &models.Campaign{
		CampaignID: "local-only",
		Repo:       "acme/local-only",
		CreatedAt:  time.Unix(100, 0).UTC(),
	}
	if err := memStore.Create(orphan); err != nil {
		t.Fatalf("store.Create orphan: %v", err)
	}

	handlers := NewHandlers(
		memStore,
		nil,
		&stubSolanaService{},
		nil,
		nil,
		nil,
		nil,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/campaigns/local-only", nil)
	rec := httptest.NewRecorder()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "local-only")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	handlers.GetCampaign(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetCampaignMergesStoreEnrichmentIntoOnChainCampaign(t *testing.T) {
	memStore := store.New()
	stored := &models.Campaign{
		CampaignID:          "chain-1",
		OwnerGitHubUsername: "asari92",
		Allocations: []models.Allocation{
			{Contributor: "alice", Reasoning: "Strong PR impact"},
		},
		CreatedAt: time.Unix(100, 0).UTC(),
	}
	if err := memStore.Create(stored); err != nil {
		t.Fatalf("store.Create stored: %v", err)
	}

	onChain := &models.Campaign{
		CampaignID: "chain-1",
		Repo:       "acme/chain",
		State:      models.StateActive,
		Allocations: []models.Allocation{
			{Contributor: "alice", Amount: 123},
		},
		CreatedAt: time.Unix(200, 0).UTC(),
	}

	handlers := NewHandlers(
		memStore,
		nil,
		&stubSolanaService{
			campaigns:       []*models.Campaign{onChain},
			onChainCampaign: onChain,
		},
		nil,
		nil,
		nil,
		nil,
	)

	got, err := handlers.loadCampaign(context.Background(), "chain-1")
	if err != nil {
		t.Fatalf("loadCampaign: %v", err)
	}
	if got.OwnerGitHubUsername != "asari92" {
		t.Fatalf("owner = %q, want %q", got.OwnerGitHubUsername, "asari92")
	}
	if got.State != models.StateActive {
		t.Fatalf("state = %q, want %q", got.State, models.StateActive)
	}
	if len(got.Allocations) != 1 {
		t.Fatalf("len(allocations) = %d, want 1", len(got.Allocations))
	}
	if got.Allocations[0].Reasoning != "Strong PR impact" {
		t.Fatalf("reasoning = %q, want %q", got.Allocations[0].Reasoning, "Strong PR impact")
	}
}

func TestListCampaignsFallsBackToStoreWhenSolanaNotConfigured(t *testing.T) {
	memStore := store.New()
	campaign := &models.Campaign{
		CampaignID: "local-only",
		Repo:       "acme/local-only",
		CreatedAt:  time.Unix(100, 0).UTC(),
	}
	if err := memStore.Create(campaign); err != nil {
		t.Fatalf("store.Create: %v", err)
	}

	handlers := NewHandlers(memStore, nil, nil, nil, nil, nil, nil)

	got, err := handlers.listCampaigns(context.Background())
	if err != nil {
		t.Fatalf("listCampaigns: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(campaigns) = %d, want 1", len(got))
	}
	if got[0].CampaignID != "local-only" {
		t.Fatalf("campaign_id = %q, want %q", got[0].CampaignID, "local-only")
	}
}

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

func TestCreateCampaignChallengeRejectsPoolBelowMinimum(t *testing.T) {
	InitLogger("development")

	dbPath := filepath.Join(t.TempDir(), "repobounty-wallet-proof-min.db")
	sqliteStore, err := store.NewSQLite(dbPath)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	defer sqliteStore.Close()

	handlers := NewHandlers(
		sqliteStore,
		stubGitHubService{},
		&stubSolanaService{},
		ai.NewAllocator("", "test-model"),
		nil,
		nil,
		&config.Config{
			Env:               "test",
			DatabasePath:      dbPath,
			MinCampaignAmount: 500_000_000,
		},
	)

	router := NewRouter(handlers, "test")

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	walletAddress := gosolana.PublicKeyFromBytes(publicKey).String()
	deadline := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)

	payload := models.CreateCampaignChallengeRequest{
		Repo:          "octocat/Hello-World",
		PoolAmount:    499_999_999,
		Deadline:      deadline,
		SponsorWallet: walletAddress,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/campaigns/create-challenge", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var apiErr models.ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&apiErr); err != nil {
		t.Fatalf("Decode error response: %v", err)
	}
	if apiErr.Error == "" {
		t.Fatal("expected non-empty API error")
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
	campaigns            []*models.Campaign
	balance              uint64
	requiredCreateAmount uint64
}

func (*stubSolanaService) IsConfigured() bool {
	return true
}

func (*stubSolanaService) AuthorityAddress() string {
	return "mock-authority"
}

func (s *stubSolanaService) ListCampaigns(ctx context.Context) ([]*models.Campaign, error) {
	if len(s.campaigns) == 0 {
		return nil, nil
	}
	result := make([]*models.Campaign, 0, len(s.campaigns))
	for _, campaign := range s.campaigns {
		if campaign == nil {
			continue
		}
		cp := *campaign
		result = append(result, &cp)
	}
	return result, nil
}

func (s *stubSolanaService) GetCampaign(ctx context.Context, campaignID string) (*models.Campaign, error) {
	if s.onChainCampaign != nil && s.onChainCampaign.CampaignID == campaignID {
		cp := *s.onChainCampaign
		return &cp, nil
	}
	for _, campaign := range s.campaigns {
		if campaign != nil && campaign.CampaignID == campaignID {
			cp := *campaign
			return &cp, nil
		}
	}
	return nil, solana.ErrCampaignNotFound
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

func (*stubSolanaService) VerifyRefundTransaction(
	ctx context.Context,
	campaignID string,
	sponsor string,
	txSignature string,
) error {
	return errors.New("not implemented")
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
