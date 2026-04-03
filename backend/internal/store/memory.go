package store

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"
)

var ErrNotFound = errors.New("campaign not found")
var ErrAlreadyUsed = errors.New("resource already used")

type CampaignStore interface {
	Create(c *models.Campaign) error
	DeleteCampaign(id string) error
	Get(id string) (*models.Campaign, error)
	Update(c *models.Campaign) error
	List() []*models.Campaign
	GetUser(username string) (*User, error)
	CreateUser(u *User) error
	UpdateUser(u *User) error
	GetWalletForGitHub(githubUsername string) (string, error)
	CreateWalletChallenge(challenge *models.WalletChallenge) error
	GetWalletChallenge(id string) (*models.WalletChallenge, error)
	MarkWalletChallengeUsed(id string, usedAt time.Time) error
	SaveFinalizeSnapshot(snapshot *models.FinalizeSnapshot) error
	GetLatestFinalizeSnapshot(campaignID string) (*models.FinalizeSnapshot, error)

	// Mirror methods
	CreateRepositoryMirror(m *models.RepositoryMirror) error
	GetRepositoryMirrorByCampaign(campaignID string) (*models.RepositoryMirror, error)
	GetRepositoryMirrorByRepo(ownerLogin, repoName string) (*models.RepositoryMirror, error)
	UpdateRepositoryMirror(m *models.RepositoryMirror) error
	ListMirrorsNeedingSync() ([]*models.RepositoryMirror, error)
	SaveMirrorCommitStats(mirrorID int64, stats map[string]*models.CommitStats) error
	GetMirrorCommitStats(mirrorID int64) (map[string]*models.CommitStats, error)
}

type Store struct {
	mu               sync.RWMutex
	campaigns        map[string]*models.Campaign
	users            map[string]*User
	walletChallenges map[string]*models.WalletChallenge
	snapshots        map[string][]*models.FinalizeSnapshot
	mirrors          map[string]*models.RepositoryMirror // campaign_id -> mirror
	mirrorStats      map[int64]map[string]*models.CommitStats
	mirrorIDSeq      int64
}

type User struct {
	GitHubUsername string    `json:"github_username"`
	WalletAddress  string    `json:"wallet_address"`
	GitHubID       int       `json:"github_id"`
	Email          string    `json:"email,omitempty"`
	AvatarURL      string    `json:"avatar_url"`
	GitHubToken    string    `json:"github_token,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func New() *Store {
	return &Store{
		campaigns:        make(map[string]*models.Campaign),
		users:            make(map[string]*User),
		walletChallenges: make(map[string]*models.WalletChallenge),
		snapshots:        make(map[string][]*models.FinalizeSnapshot),
		mirrors:          make(map[string]*models.RepositoryMirror),
		mirrorStats:      make(map[int64]map[string]*models.CommitStats),
	}
}

func (s *Store) Create(c *models.Campaign) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.campaigns[c.CampaignID]; exists {
		return errors.New("campaign already exists")
	}
	cp := copycamp(c)
	s.campaigns[c.CampaignID] = cp
	return nil
}

func (s *Store) Get(id string) (*models.Campaign, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.campaigns[id]
	if !ok {
		return nil, ErrNotFound
	}
	return copycamp(c), nil
}

func (s *Store) DeleteCampaign(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.campaigns[id]; !ok {
		return ErrNotFound
	}
	delete(s.campaigns, id)
	return nil
}

func (s *Store) Update(c *models.Campaign) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.campaigns[c.CampaignID]; !ok {
		return ErrNotFound
	}
	cp := copycamp(c)
	s.campaigns[c.CampaignID] = cp
	return nil
}

func (s *Store) List() []*models.Campaign {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*models.Campaign, 0, len(s.campaigns))
	for _, c := range s.campaigns {
		result = append(result, copycamp(c))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func copycamp(c *models.Campaign) *models.Campaign {
	cp := *c
	if len(c.Allocations) > 0 {
		cp.Allocations = make([]models.Allocation, len(c.Allocations))
		copy(cp.Allocations, c.Allocations)
	}
	if c.FinalizedAt != nil {
		t := *c.FinalizedAt
		cp.FinalizedAt = &t
	}
	return &cp
}

func copyChallenge(challenge *models.WalletChallenge) *models.WalletChallenge {
	cp := *challenge
	if challenge.UsedAt != nil {
		usedAt := *challenge.UsedAt
		cp.UsedAt = &usedAt
	}
	return &cp
}

func copySnapshot(snapshot *models.FinalizeSnapshot) *models.FinalizeSnapshot {
	cp := *snapshot
	if len(snapshot.Contributors) > 0 {
		cp.Contributors = make([]models.Contributor, len(snapshot.Contributors))
		copy(cp.Contributors, snapshot.Contributors)
	}
	if len(snapshot.Allocations) > 0 {
		cp.Allocations = make([]models.Allocation, len(snapshot.Allocations))
		copy(cp.Allocations, snapshot.Allocations)
	}
	if snapshot.ApprovedAt != nil {
		approvedAt := *snapshot.ApprovedAt
		cp.ApprovedAt = &approvedAt
	}
	return &cp
}

func (s *Store) GetUser(username string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[username]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (s *Store) CreateUser(u *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[u.GitHubUsername]; exists {
		return errors.New("user already exists")
	}
	cp := *u
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now()
	}
	s.users[u.GitHubUsername] = &cp
	return nil
}

func (s *Store) UpdateUser(u *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[u.GitHubUsername]; !ok {
		return ErrNotFound
	}
	cp := *u
	s.users[u.GitHubUsername] = &cp
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

func (s *Store) CreateWalletChallenge(challenge *models.WalletChallenge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.walletChallenges[challenge.ChallengeID]; exists {
		return errors.New("challenge already exists")
	}
	s.walletChallenges[challenge.ChallengeID] = copyChallenge(challenge)
	return nil
}

func (s *Store) GetWalletChallenge(id string) (*models.WalletChallenge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	challenge, ok := s.walletChallenges[id]
	if !ok {
		return nil, ErrNotFound
	}
	return copyChallenge(challenge), nil
}

func (s *Store) MarkWalletChallengeUsed(id string, usedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.walletChallenges[id]
	if !ok {
		return ErrNotFound
	}
	if challenge.UsedAt != nil {
		return ErrAlreadyUsed
	}
	challenge.UsedAt = &usedAt
	return nil
}

func (s *Store) SaveFinalizeSnapshot(snapshot *models.FinalizeSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.snapshots[snapshot.CampaignID]
	snapshot.Version = len(current) + 1
	s.snapshots[snapshot.CampaignID] = append(current, copySnapshot(snapshot))
	return nil
}

func (s *Store) GetLatestFinalizeSnapshot(campaignID string) (*models.FinalizeSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshots := s.snapshots[campaignID]
	if len(snapshots) == 0 {
		return nil, ErrNotFound
	}
	return copySnapshot(snapshots[len(snapshots)-1]), nil
}

func (s *Store) CreateRepositoryMirror(m *models.RepositoryMirror) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.mirrors[m.CampaignID]; exists {
		return errors.New("mirror already exists for campaign")
	}
	s.mirrorIDSeq++
	m.ID = s.mirrorIDSeq
	cp := *m
	s.mirrors[m.CampaignID] = &cp
	return nil
}

func (s *Store) GetRepositoryMirrorByCampaign(campaignID string) (*models.RepositoryMirror, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.mirrors[campaignID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *m
	return &cp, nil
}

func (s *Store) GetRepositoryMirrorByRepo(ownerLogin, repoName string) (*models.RepositoryMirror, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.mirrors {
		if m.OwnerLogin == ownerLogin && m.RepoName == repoName {
			cp := *m
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

func (s *Store) UpdateRepositoryMirror(m *models.RepositoryMirror) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.mirrors[m.CampaignID]; !ok {
		return ErrNotFound
	}
	cp := *m
	s.mirrors[m.CampaignID] = &cp
	return nil
}

func (s *Store) ListMirrorsNeedingSync() ([]*models.RepositoryMirror, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*models.RepositoryMirror
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, m := range s.mirrors {
		if m.SyncStatus == models.MirrorStatusPending ||
			(m.SyncStatus == models.MirrorStatusDone && m.LastSyncedAt.Before(cutoff)) ||
			m.SyncStatus == models.MirrorStatusFailed {
			cp := *m
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *Store) SaveMirrorCommitStats(mirrorID int64, stats map[string]*models.CommitStats) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make(map[string]*models.CommitStats, len(stats))
	for k, v := range stats {
		stat := *v
		cp[k] = &stat
	}
	s.mirrorStats[mirrorID] = cp
	return nil
}

func (s *Store) GetMirrorCommitStats(mirrorID int64) (map[string]*models.CommitStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats, ok := s.mirrorStats[mirrorID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := make(map[string]*models.CommitStats, len(stats))
	for k, v := range stats {
		stat := *v
		cp[k] = &stat
	}
	return cp, nil
}
