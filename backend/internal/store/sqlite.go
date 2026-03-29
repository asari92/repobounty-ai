package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLite(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(4)
	db.Exec("PRAGMA journal_mode=WAL")

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS campaigns (
		campaign_id TEXT PRIMARY KEY,
		campaign_pda TEXT NOT NULL DEFAULT '',
		vault_address TEXT NOT NULL DEFAULT '',
		repo TEXT NOT NULL,
		pool_amount INTEGER NOT NULL,
		total_claimed INTEGER NOT NULL DEFAULT 0,
		deadline TEXT NOT NULL,
		state TEXT NOT NULL DEFAULT 'created',
		authority TEXT NOT NULL DEFAULT '',
		sponsor TEXT NOT NULL DEFAULT '',
		allocations TEXT NOT NULL DEFAULT '[]',
		created_at TEXT NOT NULL,
		finalized_at TEXT,
		tx_signature TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS users (
		github_username TEXT PRIMARY KEY,
		github_id INTEGER UNIQUE,
		email TEXT NOT NULL DEFAULT '',
		avatar_url TEXT NOT NULL DEFAULT '',
		wallet_address TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	);
	`
	_, err := db.Exec(schema)
	return err
}

func (s *SQLiteStore) Create(c *models.Campaign) error {
	allocs, err := json.Marshal(c.Allocations)
	if err != nil {
		return fmt.Errorf("marshal allocations: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO campaigns (campaign_id, campaign_pda, vault_address, repo, pool_amount, total_claimed,
			deadline, state, authority, sponsor, allocations, created_at, finalized_at, tx_signature)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.CampaignID, c.CampaignPDA, c.VaultAddress, c.Repo, c.PoolAmount, c.TotalClaimed,
		c.Deadline.Format(time.RFC3339Nano), string(c.State), c.Authority, c.Sponsor,
		string(allocs), c.CreatedAt.Format(time.RFC3339Nano), nilifyTime(c.FinalizedAt), c.TxSignature,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return errors.New("campaign already exists")
		}
		return fmt.Errorf("insert campaign: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Get(id string) (*models.Campaign, error) {
	c, err := s.scanCampaign(s.db.QueryRow(`
		SELECT campaign_id, campaign_pda, vault_address, repo, pool_amount, total_claimed,
			deadline, state, authority, sponsor, allocations, created_at, finalized_at, tx_signature
		FROM campaigns WHERE campaign_id = ?`, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return c, nil
}

func (s *SQLiteStore) Update(c *models.Campaign) error {
	allocs, err := json.Marshal(c.Allocations)
	if err != nil {
		return fmt.Errorf("marshal allocations: %w", err)
	}

	res, err := s.db.Exec(`
		UPDATE campaigns SET campaign_pda=?, vault_address=?, repo=?, pool_amount=?, total_claimed=?,
			deadline=?, state=?, authority=?, sponsor=?, allocations=?, created_at=?, finalized_at=?, tx_signature=?
		WHERE campaign_id=?`,
		c.CampaignPDA, c.VaultAddress, c.Repo, c.PoolAmount, c.TotalClaimed,
		c.Deadline.Format(time.RFC3339Nano), string(c.State), c.Authority, c.Sponsor,
		string(allocs), c.CreatedAt.Format(time.RFC3339Nano), nilifyTime(c.FinalizedAt),
		c.TxSignature, c.CampaignID,
	)
	if err != nil {
		return fmt.Errorf("update campaign: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) List() []*models.Campaign {
	rows, err := s.db.Query(`
		SELECT campaign_id, campaign_pda, vault_address, repo, pool_amount, total_claimed,
			deadline, state, authority, sponsor, allocations, created_at, finalized_at, tx_signature
		FROM campaigns ORDER BY created_at DESC`)
	if err != nil {
		log.Printf("sqlite: list campaigns query failed: %v", err)
		return nil
	}
	defer rows.Close()

	var result []*models.Campaign
	for rows.Next() {
		c, err := s.scanCampaign(rows)
		if err != nil {
			log.Printf("sqlite: scan campaign failed: %v", err)
			continue
		}
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

func (s *SQLiteStore) GetUser(username string) (*User, error) {
	var u User
	var createdAt string
	err := s.db.QueryRow(`
		SELECT github_username, github_id, email, avatar_url, wallet_address, created_at
		FROM users WHERE github_username = ?`, username,
	).Scan(&u.GitHubUsername, &u.GitHubID, &u.Email, &u.AvatarURL, &u.WalletAddress, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	u.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return &u, nil
}

func (s *SQLiteStore) CreateUser(u *User) error {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	_, err := s.db.Exec(`
		INSERT INTO users (github_username, github_id, email, avatar_url, wallet_address, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		u.GitHubUsername, u.GitHubID, u.Email, u.AvatarURL, u.WalletAddress,
		u.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return errors.New("user already exists")
		}
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateUser(u *User) error {
	res, err := s.db.Exec(`
		UPDATE users SET github_id=?, email=?, avatar_url=?, wallet_address=?, created_at=?
		WHERE github_username=?`,
		u.GitHubID, u.Email, u.AvatarURL, u.WalletAddress,
		u.CreatedAt.Format(time.RFC3339Nano), u.GitHubUsername,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) GetWalletForGitHub(githubUsername string) (string, error) {
	var wallet string
	err := s.db.QueryRow(
		"SELECT wallet_address FROM users WHERE github_username = ?", githubUsername,
	).Scan(&wallet)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get wallet: %w", err)
	}
	return wallet, nil
}

func (s *SQLiteStore) scanCampaign(scanner interface{ Scan(...interface{}) error }) (*models.Campaign, error) {
	var (
		c              models.Campaign
		allocJSON      string
		deadlineStr    string
		createdAtStr   string
		finalizedAtStr sql.NullString
		txSigStr       sql.NullString
	)

	err := scanner.Scan(
		&c.CampaignID, &c.CampaignPDA, &c.VaultAddress, &c.Repo,
		&c.PoolAmount, &c.TotalClaimed, &deadlineStr, &c.State,
		&c.Authority, &c.Sponsor, &allocJSON, &createdAtStr,
		&finalizedAtStr, &txSigStr,
	)
	if err != nil {
		return nil, fmt.Errorf("scan campaign: %w", err)
	}

	var parseErr error
	c.Deadline, parseErr = time.Parse(time.RFC3339Nano, deadlineStr)
	if parseErr != nil {
		c.Deadline, _ = time.Parse(time.RFC3339, deadlineStr)
	}
	c.CreatedAt, parseErr = time.Parse(time.RFC3339Nano, createdAtStr)
	if parseErr != nil {
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	}

	if finalizedAtStr.Valid && finalizedAtStr.String != "" {
		t, err := time.Parse(time.RFC3339Nano, finalizedAtStr.String)
		if err != nil {
			t, _ = time.Parse(time.RFC3339, finalizedAtStr.String)
		}
		c.FinalizedAt = &t
	}

	if txSigStr.Valid {
		c.TxSignature = txSigStr.String
	}

	if allocJSON != "" && allocJSON != "[]" {
		if err := json.Unmarshal([]byte(allocJSON), &c.Allocations); err != nil {
			log.Printf("sqlite: unmarshal allocations for %s failed: %v", c.CampaignID, err)
		}
	}

	return &c, nil
}

func nilifyTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}
