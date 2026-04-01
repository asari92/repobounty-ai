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
	"testing"
	"time"

	gosolana "github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/auth"
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

	user := &store.User{
		GitHubUsername: "alice",
		GitHubID:       101,
		AvatarURL:      "https://example.com/alice.png",
		CreatedAt:      time.Now().UTC(),
	}
	if err := sqliteStore.CreateUser(user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	jwtManager := auth.NewJWTManager("test-secret-with-at-least-32-bytes")
	token, err := jwtManager.GenerateToken(user.GitHubUsername)
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	handlers := NewHandlers(
		sqliteStore,
		stubGitHubService{},
		stubSolanaService{},
		ai.NewAllocator("", "test-model"),
		jwtManager,
		nil,
		&config.Config{
			Env:          "test",
			DatabasePath: dbPath,
		},
	)

	server := httptest.NewServer(NewRouter(handlers, "test"))
	defer server.Close()

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
		server.Client(),
		http.MethodPost,
		server.URL+"/api/campaigns/create-challenge",
		token,
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
		server.Client(),
		http.MethodPost,
		server.URL+"/api/campaigns/",
		token,
		createReq,
		http.StatusCreated,
	)
	if createRes.CampaignID == "" {
		t.Fatal("campaign_id was empty")
	}

	challenge, err := sqliteStore.GetWalletChallenge(challengeRes.ChallengeID)
	if err != nil {
		t.Fatalf("GetWalletChallenge: %v", err)
	}
	if challenge.UsedAt == nil {
		t.Fatal("challenge was not marked used")
	}
}

type stubGitHubService struct{}

func (stubGitHubService) RepositoryExists(ctx context.Context, repo string) (bool, error) {
	return true, nil
}

func (stubGitHubService) FetchContributionWindowData(
	ctx context.Context,
	repo string,
	windowStart time.Time,
	windowEnd time.Time,
) (*github.ContributionWindowData, error) {
	return &github.ContributionWindowData{}, nil
}

type stubSolanaService struct{}

func (stubSolanaService) IsConfigured() bool {
	return true
}

func (stubSolanaService) AuthorityAddress() string {
	return "mock-authority"
}

func (stubSolanaService) ListCampaigns(ctx context.Context) ([]*models.Campaign, error) {
	return nil, nil
}

func (stubSolanaService) GetCampaign(ctx context.Context, campaignID string) (*models.Campaign, error) {
	return nil, errors.New("campaign not found")
}

func (stubSolanaService) GetBalance(ctx context.Context, wallet string) (uint64, error) {
	return 10_000_000_000, nil
}

func (stubSolanaService) CreateCampaign(
	ctx context.Context,
	campaignID string,
	repo string,
	poolAmount uint64,
	deadline int64,
	sponsorPubkey string,
) (string, string, string, error) {
	return "tx-test-signature", "campaign-pda", "vault-pda", nil
}

func (stubSolanaService) BuildFundTransaction(
	ctx context.Context,
	campaignID string,
	poolAmount uint64,
	sponsorPubkey string,
) (*solana.FundTransaction, error) {
	return nil, errors.New("not implemented")
}

func (stubSolanaService) FinalizeCampaign(
	ctx context.Context,
	campaignID string,
	allocations []solana.AllocationInput,
) (string, error) {
	return "", errors.New("not implemented")
}

func (stubSolanaService) ClaimAllocation(
	ctx context.Context,
	campaignID string,
	contributorGitHub string,
	contributorWallet string,
) (string, error) {
	return "", errors.New("not implemented")
}

func performJSONRequest[T any](
	t *testing.T,
	client *http.Client,
	method string,
	url string,
	token string,
	body any,
	wantStatus int,
) T {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal request: %v", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do request: %v", err)
	}
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
