package store

import (
	"errors"
	"sort"
	"sync"

	"github.com/repobounty/repobounty-ai/internal/models"
)

var ErrNotFound = errors.New("campaign not found")

type Store struct {
	mu        sync.RWMutex
	campaigns map[string]*models.Campaign
	users     map[string]*User
}

type User struct {
	GitHubUsername string
	WalletAddress  string
	GitHubID       int
	Email          string
	AvatarURL      string
}

func New() *Store {
	return &Store{
		campaigns: make(map[string]*models.Campaign),
		users:     make(map[string]*User),
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
