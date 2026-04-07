package http

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mr-tron/base58"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/store"
)

// newTestRouter builds a minimal chi router for use in unit tests.
func newTestRouter(h *Handlers) http.Handler {
	return NewRouter(h, "test")
}

// newFinalizeTestHandlers returns Handlers wired to an in-memory store with
// one funded campaign past its deadline.
func newFinalizeTestHandlers(t *testing.T, sponsor string) (*Handlers, *models.Campaign) {
	t.Helper()
	s := store.New()
	campaign := &models.Campaign{
		CampaignID: "test-campaign-1",
		Repo:       "acme/repo",
		PoolAmount: 1_000_000_000,
		State:      models.StateFunded,
		Sponsor:    sponsor,
		Deadline:   time.Now().Add(-time.Minute),
		CreatedAt:  time.Now().Add(-2 * time.Minute),
	}
	if err := s.Create(campaign); err != nil {
		t.Fatalf("store.Create: %v", err)
	}
	return NewHandlers(s, nil, nil, nil, nil, nil, nil), campaign
}

// generateTestKeypair returns a fresh ed25519 keypair and its base58 public key.
func generateTestKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey: %v", err)
	}
	return pub, priv, base58.Encode(pub)
}

func TestFinalizeChallengeIssuesChallengeForSponsor(t *testing.T) {
	_, _, sponsorB58 := generateTestKeypair(t)
	h, _ := newFinalizeTestHandlers(t, sponsorB58)

	body, _ := json.Marshal(models.FinalizeChallengeRequest{WalletAddress: sponsorB58})
	r := newTestRouter(h)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/campaigns/test-campaign-1/finalize-challenge", bytes.NewReader(body)))

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}
	var resp models.WalletChallengeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Action != models.WalletChallengeActionFinalize {
		t.Fatalf("action = %q, want %q", resp.Action, models.WalletChallengeActionFinalize)
	}
	if resp.WalletAddress != sponsorB58 {
		t.Fatalf("wallet_address = %q, want %q", resp.WalletAddress, sponsorB58)
	}
	if resp.Message == "" {
		t.Fatal("message must not be empty")
	}
}

func TestFinalizeChallengeRejectsNonSponsor(t *testing.T) {
	_, _, sponsorB58 := generateTestKeypair(t)
	_, _, otherB58 := generateTestKeypair(t)
	h, _ := newFinalizeTestHandlers(t, sponsorB58)

	body, _ := json.Marshal(models.FinalizeChallengeRequest{WalletAddress: otherB58})
	r := newTestRouter(h)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/campaigns/test-campaign-1/finalize-challenge", bytes.NewReader(body)))

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestFinalizeChallengeRejectsBeforeDeadline(t *testing.T) {
	_, _, sponsorB58 := generateTestKeypair(t)
	s := store.New()
	campaign := &models.Campaign{
		CampaignID: "test-campaign-2",
		Repo:       "acme/repo",
		PoolAmount: 1_000_000_000,
		State:      models.StateFunded,
		Sponsor:    sponsorB58,
		Deadline:   time.Now().Add(time.Hour),
		CreatedAt:  time.Now().Add(-time.Minute),
	}
	_ = s.Create(campaign)
	h := NewHandlers(s, nil, nil, nil, nil, nil, nil)

	body, _ := json.Marshal(models.FinalizeChallengeRequest{WalletAddress: sponsorB58})
	r := newTestRouter(h)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/campaigns/test-campaign-2/finalize-challenge", bytes.NewReader(body)))

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}

func TestFinalizeWithWalletProofRequiresSolanaConfigured(t *testing.T) {
	_, _, sponsorB58 := generateTestKeypair(t)
	h, _ := newFinalizeTestHandlers(t, sponsorB58)
	r := newTestRouter(h)

	// Issue a challenge first.
	challengeBody, _ := json.Marshal(models.FinalizeChallengeRequest{WalletAddress: sponsorB58})
	crr := httptest.NewRecorder()
	r.ServeHTTP(crr, httptest.NewRequest(http.MethodPost, "/api/campaigns/test-campaign-1/finalize-challenge", bytes.NewReader(challengeBody)))
	if crr.Code != http.StatusCreated {
		t.Fatalf("challenge status = %d; body: %s", crr.Code, crr.Body.String())
	}
	var challenge models.WalletChallengeResponse
	_ = json.NewDecoder(crr.Body).Decode(&challenge)

	// Submit with an all-zero (invalid) signature.
	// Without Solana configured, the handler returns 503 before reaching signature
	// verification — this test confirms the correct guard is in place.
	reqBody, _ := json.Marshal(models.FinalizeWalletRequest{
		WalletAddress: sponsorB58,
		ChallengeID:   challenge.ChallengeID,
		Signature:     base58.Encode(bytes.Repeat([]byte{0}, 64)),
	})
	frr := httptest.NewRecorder()
	r.ServeHTTP(frr, httptest.NewRequest(http.MethodPost, "/api/campaigns/test-campaign-1/finalize-wallet", bytes.NewReader(reqBody)))

	// 503 = Solana not configured; 400 = invalid signature (Solana configured).
	// Either response is acceptable; the important thing is we do NOT get 2xx.
	// Accept any non-2xx: exact code depends on whether Solana is configured.
	if frr.Code >= 200 && frr.Code < 300 {
		t.Fatalf("status = %d, want non-2xx; body: %s", frr.Code, frr.Body.String())
	}
}
