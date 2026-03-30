package store

import (
	"testing"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"
)

func newTestCampaign(id string) *models.Campaign {
	return &models.Campaign{
		CampaignID:  id,
		CampaignPDA: "pda_" + id,
		Repo:        "owner/repo",
		PoolAmount:  1_000_000_000,
		Deadline:    time.Now().Add(24 * time.Hour),
		State:       models.StateCreated,
		Allocations: []models.Allocation{},
		CreatedAt:   time.Now(),
	}
}

func TestStore_CreateAndGet(t *testing.T) {
	s := New()
	c := newTestCampaign("test-1")

	if err := s.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get("test-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.CampaignID != "test-1" {
		t.Errorf("CampaignID = %q, want %q", got.CampaignID, "test-1")
	}
	if got.Repo != "owner/repo" {
		t.Errorf("Repo = %q, want %q", got.Repo, "owner/repo")
	}
}

func TestStore_CreateDuplicate(t *testing.T) {
	s := New()
	c := newTestCampaign("dup-1")

	if err := s.Create(c); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	if err := s.Create(c); err == nil {
		t.Error("expected error on duplicate create, got nil")
	}
}

func TestStore_GetNotFound(t *testing.T) {
	s := New()
	_, err := s.Get("nonexistent")
	if err != ErrNotFound {
		t.Errorf("Get = %v, want ErrNotFound", err)
	}
}

func TestStore_Update(t *testing.T) {
	s := New()
	c := newTestCampaign("upd-1")
	s.Create(c)

	c.State = models.StateFunded
	if err := s.Update(c); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, _ := s.Get("upd-1")
	if got.State != models.StateFunded {
		t.Errorf("State = %q, want %q", got.State, models.StateFunded)
	}
}

func TestStore_UpdateNotFound(t *testing.T) {
	s := New()
	c := newTestCampaign("no-exist")
	if err := s.Update(c); err != ErrNotFound {
		t.Errorf("Update = %v, want ErrNotFound", err)
	}
}

func TestStore_List(t *testing.T) {
	s := New()

	s.Create(newTestCampaign("list-1"))
	time.Sleep(time.Millisecond) // ensure different timestamps
	s.Create(newTestCampaign("list-2"))

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("List len = %d, want 2", len(list))
	}
	// Should be sorted by CreatedAt descending
	if list[0].CampaignID != "list-2" {
		t.Errorf("first item = %q, want list-2 (newest first)", list[0].CampaignID)
	}
}

func TestStore_DeepCopy(t *testing.T) {
	s := New()
	c := newTestCampaign("copy-1")
	c.Allocations = []models.Allocation{{Contributor: "alice", Percentage: 10000}}
	s.Create(c)

	got, _ := s.Get("copy-1")
	got.Allocations[0].Contributor = "modified"

	got2, _ := s.Get("copy-1")
	if got2.Allocations[0].Contributor != "alice" {
		t.Error("deep copy failed: modifying returned value changed stored value")
	}
}

func TestStore_Users(t *testing.T) {
	s := New()
	u := &User{
		GitHubUsername: "testuser",
		GitHubID:       123,
		WalletAddress:  "wallet123",
		CreatedAt:      time.Now(),
	}

	if err := s.CreateUser(u); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUser("testuser")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.GitHubID != 123 {
		t.Errorf("GitHubID = %d, want 123", got.GitHubID)
	}

	got.WalletAddress = "new-wallet"
	if err := s.UpdateUser(got); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	got2, _ := s.GetUser("testuser")
	if got2.WalletAddress != "new-wallet" {
		t.Errorf("WalletAddress = %q, want %q", got2.WalletAddress, "new-wallet")
	}
}

func TestStore_GetWalletForGitHub(t *testing.T) {
	s := New()
	s.CreateUser(&User{
		GitHubUsername: "alice",
		WalletAddress:  "alice_wallet",
	})

	wallet, err := s.GetWalletForGitHub("alice")
	if err != nil {
		t.Fatalf("GetWalletForGitHub: %v", err)
	}
	if wallet != "alice_wallet" {
		t.Errorf("wallet = %q, want %q", wallet, "alice_wallet")
	}

	_, err = s.GetWalletForGitHub("nobody")
	if err != ErrNotFound {
		t.Errorf("GetWalletForGitHub = %v, want ErrNotFound", err)
	}
}

func TestStore_UserDuplicate(t *testing.T) {
	s := New()
	u := &User{GitHubUsername: "dup-user"}
	s.CreateUser(u)
	if err := s.CreateUser(u); err == nil {
		t.Error("expected error on duplicate user create")
	}
}

func TestStore_UpdateUserNotFound(t *testing.T) {
	s := New()
	if err := s.UpdateUser(&User{GitHubUsername: "ghost"}); err != ErrNotFound {
		t.Errorf("UpdateUser = %v, want ErrNotFound", err)
	}
}
