package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/config"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/store"
)

func TestRefundBuildRejectsBeforeClaimDeadline(t *testing.T) {
	handlers, sponsorWallet := newRefundHandlersWithOnChainCampaign(t, time.Now().UTC().Add(24*time.Hour))
	rec := performJSONRequestRecorder(t, handlers, http.MethodPost, "/api/campaigns/42/refund", models.RefundBuildRequest{
		SponsorWallet: sponsorWallet,
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestRefundBuildReturnsUnsignedTxAfterClaimDeadline(t *testing.T) {
	handlers, sponsorWallet := newRefundHandlersWithOnChainCampaign(t, time.Now().UTC().Add(-24*time.Hour))
	rec := performJSONRequestRecorder(t, handlers, http.MethodPost, "/api/campaigns/42/refund", models.RefundBuildRequest{
		SponsorWallet: sponsorWallet,
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRefundBuildRejectsAlreadyClosedCampaign(t *testing.T) {
	handlers, sponsorWallet := newRefundHandlersWithClosedOnChainCampaign(t)
	rec := performJSONRequestRecorder(t, handlers, http.MethodPost, "/api/campaigns/42/refund", models.RefundBuildRequest{
		SponsorWallet: sponsorWallet,
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestRefundConfirmRequiresTxSignature(t *testing.T) {
	handlers, sponsorWallet := newRefundConfirmHandlersWithOnChainCampaign(t, "sig-ok")
	rec := performJSONRequestRecorder(t, handlers, http.MethodPost, "/api/campaigns/42/refund-confirm", models.RefundConfirmRequest{
		SponsorWallet: sponsorWallet,
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRefundConfirmRejectsWrongRefundTx(t *testing.T) {
	handlers, sponsorWallet := newRefundConfirmHandlersWithOnChainCampaign(t, "sig-ok")
	rec := performJSONRequestRecorder(t, handlers, http.MethodPost, "/api/campaigns/42/refund-confirm", models.RefundConfirmRequest{
		SponsorWallet: sponsorWallet,
		TxSignature:   "sig-bad",
	})

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestRefundConfirmSucceedsWithMatchingRefundTx(t *testing.T) {
	handlers, sponsorWallet := newRefundConfirmHandlersWithOnChainCampaign(t, "sig-ok")
	rec := performJSONRequestRecorder(t, handlers, http.MethodPost, "/api/campaigns/42/refund-confirm", models.RefundConfirmRequest{
		SponsorWallet: sponsorWallet,
		TxSignature:   "sig-ok",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	stored, err := handlers.store.Get("42")
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if stored.TxSignature != "sig-ok" {
		t.Fatalf("tx_signature = %q, want %q", stored.TxSignature, "sig-ok")
	}
	if stored.State != models.StateCompleted {
		t.Fatalf("state = %q, want %q", stored.State, models.StateCompleted)
	}
	if stored.ClosedAt == nil {
		t.Fatal("closed_at was not recorded")
	}
}

func TestRefundConfirmUsesTransactionProofEvenIfCampaignReadIsStale(t *testing.T) {
	handlers, sponsorWallet := newRefundConfirmHandlersWithStaleOnChainCampaign(t, "sig-ok")
	rec := performJSONRequestRecorder(t, handlers, http.MethodPost, "/api/campaigns/42/refund-confirm", models.RefundConfirmRequest{
		SponsorWallet: sponsorWallet,
		TxSignature:   "sig-ok",
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	stored, err := handlers.store.Get("42")
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if stored.State != models.StateCompleted {
		t.Fatalf("state = %q, want %q", stored.State, models.StateCompleted)
	}
	if stored.Status != models.StateClosed {
		t.Fatalf("status = %q, want %q", stored.Status, models.StateClosed)
	}
	if stored.CloseReason != "refund" {
		t.Fatalf("close_reason = %q, want %q", stored.CloseReason, "refund")
	}
}

func newRefundHandlersWithOnChainCampaign(t *testing.T, claimDeadlineAt time.Time) (*Handlers, string) {
	t.Helper()
	InitLogger("development")

	sponsorWallet := testWalletAddress(t)
	memStore := store.New()
	solanaStub := &refundSolanaStub{
		stubSolanaService: stubSolanaService{
			onChainCampaign: &models.Campaign{
				CampaignID:      "42",
				Repo:            "octocat/Hello-World",
				Sponsor:         sponsorWallet,
				State:           models.StateFinalized,
				Status:          models.StateFinalized,
				CreatedAt:       time.Unix(200, 0).UTC(),
				ClaimDeadlineAt: claimDeadlineAt,
			},
		},
	}

	return NewHandlers(
		memStore,
		stubGitHubService{},
		solanaStub,
		ai.NewAllocator("", "test-model"),
		nil,
		nil,
		&config.Config{Env: "test", MinCampaignAmount: 500_000_000},
	), sponsorWallet
}

func newRefundConfirmHandlersWithOnChainCampaign(t *testing.T, expectedTxSignature string) (*Handlers, string) {
	t.Helper()
	InitLogger("development")

	sponsorWallet := testWalletAddress(t)
	memStore := store.New()
	solanaStub := &refundSolanaStub{
		stubSolanaService: stubSolanaService{
			onChainCampaign: &models.Campaign{
				CampaignID:      "42",
				Repo:            "octocat/Hello-World",
				Sponsor:         sponsorWallet,
				State:           models.StateCompleted,
				Status:          models.StateClosed,
				CreatedAt:       time.Unix(200, 0).UTC(),
				ClaimDeadlineAt: time.Now().UTC().Add(-24 * time.Hour),
			},
		},
		expectedRefundTxSignature: expectedTxSignature,
	}

	return NewHandlers(
		memStore,
		stubGitHubService{},
		solanaStub,
		ai.NewAllocator("", "test-model"),
		nil,
		nil,
		&config.Config{Env: "test", MinCampaignAmount: 500_000_000},
	), sponsorWallet
}

func newRefundHandlersWithClosedOnChainCampaign(t *testing.T) (*Handlers, string) {
	t.Helper()
	InitLogger("development")

	sponsorWallet := testWalletAddress(t)
	memStore := store.New()
	solanaStub := &refundSolanaStub{
		stubSolanaService: stubSolanaService{
			onChainCampaign: &models.Campaign{
				CampaignID:      "42",
				Repo:            "octocat/Hello-World",
				Sponsor:         sponsorWallet,
				State:           models.StateCompleted,
				Status:          models.StateClosed,
				CreatedAt:       time.Unix(200, 0).UTC(),
				ClaimDeadlineAt: time.Now().UTC().Add(-24 * time.Hour),
			},
		},
	}

	return NewHandlers(
		memStore,
		stubGitHubService{},
		solanaStub,
		ai.NewAllocator("", "test-model"),
		nil,
		nil,
		&config.Config{Env: "test", MinCampaignAmount: 500_000_000},
	), sponsorWallet
}

func newRefundConfirmHandlersWithStaleOnChainCampaign(t *testing.T, expectedTxSignature string) (*Handlers, string) {
	t.Helper()
	InitLogger("development")

	sponsorWallet := testWalletAddress(t)
	memStore := store.New()
	solanaStub := &refundSolanaStub{
		stubSolanaService: stubSolanaService{
			onChainCampaign: &models.Campaign{
				CampaignID:      "42",
				Repo:            "octocat/Hello-World",
				Sponsor:         sponsorWallet,
				State:           models.StateFinalized,
				Status:          models.StateFinalized,
				CreatedAt:       time.Unix(200, 0).UTC(),
				ClaimDeadlineAt: time.Now().UTC().Add(-24 * time.Hour),
			},
		},
		expectedRefundTxSignature: expectedTxSignature,
	}

	return NewHandlers(
		memStore,
		stubGitHubService{},
		solanaStub,
		ai.NewAllocator("", "test-model"),
		nil,
		nil,
		&config.Config{Env: "test", MinCampaignAmount: 500_000_000},
	), sponsorWallet
}

func performJSONRequestRecorder(
	t *testing.T,
	handlers *Handlers,
	method string,
	path string,
	body any,
) *httptest.ResponseRecorder {
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

	recorder := httptest.NewRecorder()
	NewRouter(handlers, "test").ServeHTTP(recorder, req)
	return recorder
}

type refundSolanaStub struct {
	stubSolanaService
	refundTx                  string
	expectedRefundTxSignature string
}

func (s *refundSolanaStub) BuildRefundTransaction(ctx context.Context, campaignID string, sponsor string) (string, error) {
	if s.refundTx != "" {
		return s.refundTx, nil
	}
	if s.onChainCampaign == nil {
		return "", errors.New("not implemented")
	}
	if sponsor != s.onChainCampaign.Sponsor {
		return "", errors.New("unexpected sponsor")
	}
	return "refund-partial-tx", nil
}

func (s *refundSolanaStub) VerifyRefundTransaction(
	ctx context.Context,
	campaignID string,
	sponsor string,
	txSignature string,
) error {
	if s.expectedRefundTxSignature == "" {
		return errors.New("not implemented")
	}
	if txSignature != s.expectedRefundTxSignature {
		return errors.New("refund transaction does not match expected signature")
	}
	if s.onChainCampaign != nil {
		if s.onChainCampaign.CampaignID != campaignID {
			return errors.New("refund transaction campaign mismatch")
		}
		if s.onChainCampaign.Sponsor != sponsor {
			return errors.New("refund transaction sponsor mismatch")
		}
	}
	return nil
}

func (*stubSolanaService) BuildRefundTransaction(
	ctx context.Context,
	campaignID string,
	sponsor string,
) (string, error) {
	return "", errors.New("not implemented")
}
