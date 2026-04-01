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
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("sqlite: PRAGMA journal_mode=WAL failed: %v", err)
	}

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
		owner_github_username TEXT NOT NULL DEFAULT '',
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
		github_token TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS wallet_challenges (
		challenge_id TEXT PRIMARY KEY,
		action TEXT NOT NULL,
		wallet_address TEXT NOT NULL,
		message TEXT NOT NULL,
		payload_json TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL,
		used_at TEXT
	);

	CREATE TABLE IF NOT EXISTS finalize_snapshots (
		campaign_id TEXT NOT NULL,
		version INTEGER NOT NULL,
		input_hash TEXT NOT NULL,
		allocation_mode TEXT NOT NULL,
		contributors_json TEXT NOT NULL DEFAULT '[]',
		allocations_json TEXT NOT NULL DEFAULT '[]',
		window_start TEXT NOT NULL,
		window_end TEXT NOT NULL,
		contributor_source TEXT NOT NULL DEFAULT '',
		contributor_notes TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL,
		approved_by_github_username TEXT NOT NULL DEFAULT '',
		approved_at TEXT,
		PRIMARY KEY (campaign_id, version)
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	for _, stmt := range []string{
		`ALTER TABLE campaigns ADD COLUMN owner_github_username TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN github_token TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := db.Exec(stmt); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return err
		}
	}

	return nil
}

func (s *SQLiteStore) Create(c *models.Campaign) error {
	allocs, err := json.Marshal(c.Allocations)
	if err != nil {
		return fmt.Errorf("marshal allocations: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO campaigns (campaign_id, campaign_pda, vault_address, repo, pool_amount, total_claimed,
			deadline, state, authority, sponsor, owner_github_username, allocations, created_at, finalized_at, tx_signature)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.CampaignID, c.CampaignPDA, c.VaultAddress, c.Repo, c.PoolAmount, c.TotalClaimed,
		c.Deadline.Format(time.RFC3339Nano), string(c.State), c.Authority, c.Sponsor, c.OwnerGitHubUsername,
		string(allocs), c.CreatedAt.Format(time.RFC3339Nano), nullableTime(c.FinalizedAt), c.TxSignature,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return errors.New("campaign already exists")
		}
		return fmt.Errorf("insert campaign: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteCampaign(id string) error {
	res, err := s.db.Exec(`DELETE FROM campaigns WHERE campaign_id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete campaign: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) Get(id string) (*models.Campaign, error) {
	c, err := s.scanCampaign(s.db.QueryRow(`
		SELECT campaign_id, campaign_pda, vault_address, repo, pool_amount, total_claimed,
			deadline, state, authority, sponsor, owner_github_username, allocations, created_at, finalized_at, tx_signature
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
			deadline=?, state=?, authority=?, sponsor=?, owner_github_username=?, allocations=?, created_at=?, finalized_at=?, tx_signature=?
		WHERE campaign_id=?`,
		c.CampaignPDA, c.VaultAddress, c.Repo, c.PoolAmount, c.TotalClaimed,
		c.Deadline.Format(time.RFC3339Nano), string(c.State), c.Authority, c.Sponsor, c.OwnerGitHubUsername,
		string(allocs), c.CreatedAt.Format(time.RFC3339Nano), nullableTime(c.FinalizedAt),
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
			deadline, state, authority, sponsor, owner_github_username, allocations, created_at, finalized_at, tx_signature
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
		SELECT github_username, github_id, email, avatar_url, wallet_address, github_token, created_at
		FROM users WHERE github_username = ?`, username,
	).Scan(&u.GitHubUsername, &u.GitHubID, &u.Email, &u.AvatarURL, &u.WalletAddress, &u.GitHubToken, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	u.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		u.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}
	return &u, nil
}

func (s *SQLiteStore) CreateUser(u *User) error {
	if u.CreatedAt.IsZero() {
		u = &User{
			GitHubUsername: u.GitHubUsername,
			WalletAddress:  u.WalletAddress,
			GitHubID:       u.GitHubID,
			Email:          u.Email,
			AvatarURL:      u.AvatarURL,
			GitHubToken:    u.GitHubToken,
			CreatedAt:      time.Now(),
		}
	}
	_, err := s.db.Exec(`
		INSERT INTO users (github_username, github_id, email, avatar_url, wallet_address, github_token, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.GitHubUsername, u.GitHubID, u.Email, u.AvatarURL, u.WalletAddress, u.GitHubToken,
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
		UPDATE users SET github_id=?, email=?, avatar_url=?, wallet_address=?, github_token=?, created_at=?
		WHERE github_username=?`,
		u.GitHubID, u.Email, u.AvatarURL, u.WalletAddress, u.GitHubToken,
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

func (s *SQLiteStore) CreateWalletChallenge(challenge *models.WalletChallenge) error {
	_, err := s.db.Exec(`
		INSERT INTO wallet_challenges (challenge_id, action, wallet_address, message, payload_json, created_at, expires_at, used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		challenge.ChallengeID,
		string(challenge.Action),
		challenge.WalletAddress,
		challenge.Message,
		challenge.PayloadJSON,
		challenge.CreatedAt.Format(time.RFC3339Nano),
		challenge.ExpiresAt.Format(time.RFC3339Nano),
		nullableTime(challenge.UsedAt),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			return errors.New("challenge already exists")
		}
		return fmt.Errorf("insert wallet challenge: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetWalletChallenge(id string) (*models.WalletChallenge, error) {
	var challenge models.WalletChallenge
	var (
		actionStr string
		createdAt string
		expiresAt string
		usedAt    sql.NullString
	)

	err := s.db.QueryRow(`
		SELECT challenge_id, action, wallet_address, message, payload_json, created_at, expires_at, used_at
		FROM wallet_challenges WHERE challenge_id = ?`,
		id,
	).Scan(
		&challenge.ChallengeID,
		&actionStr,
		&challenge.WalletAddress,
		&challenge.Message,
		&challenge.PayloadJSON,
		&createdAt,
		&expiresAt,
		&usedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get wallet challenge: %w", err)
	}

	challenge.Action = models.WalletChallengeAction(actionStr)
	challenge.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		challenge.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}
	challenge.ExpiresAt, err = time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		challenge.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	}
	if usedAt.Valid && usedAt.String != "" {
		t, parseErr := time.Parse(time.RFC3339Nano, usedAt.String)
		if parseErr != nil {
			t, _ = time.Parse(time.RFC3339, usedAt.String)
		}
		challenge.UsedAt = &t
	}

	return &challenge, nil
}

func (s *SQLiteStore) MarkWalletChallengeUsed(id string, usedAt time.Time) error {
	res, err := s.db.Exec(`
		UPDATE wallet_challenges
		SET used_at = ?
		WHERE challenge_id = ? AND used_at IS NULL`,
		usedAt.Format(time.RFC3339Nano),
		id,
	)
	if err != nil {
		return fmt.Errorf("mark wallet challenge used: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		challenge, getErr := s.GetWalletChallenge(id)
		if getErr != nil {
			return getErr
		}
		if challenge.UsedAt != nil {
			return ErrAlreadyUsed
		}
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) SaveFinalizeSnapshot(snapshot *models.FinalizeSnapshot) error {
	contributorsJSON, err := json.Marshal(snapshot.Contributors)
	if err != nil {
		return fmt.Errorf("marshal snapshot contributors: %w", err)
	}
	allocationsJSON, err := json.Marshal(snapshot.Allocations)
	if err != nil {
		return fmt.Errorf("marshal snapshot allocations: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin snapshot transaction: %w", err)
	}
	defer tx.Rollback()

	var version int
	if err := tx.QueryRow(`
		SELECT COALESCE(MAX(version), 0) + 1
		FROM finalize_snapshots
		WHERE campaign_id = ?`,
		snapshot.CampaignID,
	).Scan(&version); err != nil {
		return fmt.Errorf("select next snapshot version: %w", err)
	}
	snapshot.Version = version

	_, err = tx.Exec(`
		INSERT INTO finalize_snapshots (
			campaign_id, version, input_hash, allocation_mode, contributors_json, allocations_json,
			window_start, window_end, contributor_source, contributor_notes, created_at,
			approved_by_github_username, approved_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.CampaignID,
		snapshot.Version,
		snapshot.InputHash,
		string(snapshot.AllocationMode),
		string(contributorsJSON),
		string(allocationsJSON),
		snapshot.WindowStart.Format(time.RFC3339Nano),
		snapshot.WindowEnd.Format(time.RFC3339Nano),
		snapshot.ContributorSource,
		snapshot.ContributorNotes,
		snapshot.CreatedAt.Format(time.RFC3339Nano),
		snapshot.ApprovedByGitHubUsername,
		nullableTime(snapshot.ApprovedAt),
	)
	if err != nil {
		return fmt.Errorf("insert finalize snapshot: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit finalize snapshot: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetLatestFinalizeSnapshot(campaignID string) (*models.FinalizeSnapshot, error) {
	var snapshot models.FinalizeSnapshot
	var (
		allocationMode   string
		contributorsJSON string
		allocationsJSON  string
		windowStart      string
		windowEnd        string
		createdAt        string
		approvedAt       sql.NullString
	)

	err := s.db.QueryRow(`
		SELECT campaign_id, version, input_hash, allocation_mode, contributors_json, allocations_json,
			window_start, window_end, contributor_source, contributor_notes, created_at,
			approved_by_github_username, approved_at
		FROM finalize_snapshots
		WHERE campaign_id = ?
		ORDER BY version DESC
		LIMIT 1`,
		campaignID,
	).Scan(
		&snapshot.CampaignID,
		&snapshot.Version,
		&snapshot.InputHash,
		&allocationMode,
		&contributorsJSON,
		&allocationsJSON,
		&windowStart,
		&windowEnd,
		&snapshot.ContributorSource,
		&snapshot.ContributorNotes,
		&createdAt,
		&snapshot.ApprovedByGitHubUsername,
		&approvedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get finalize snapshot: %w", err)
	}

	snapshot.AllocationMode = models.AllocationMode(allocationMode)
	snapshot.WindowStart, err = time.Parse(time.RFC3339Nano, windowStart)
	if err != nil {
		snapshot.WindowStart, _ = time.Parse(time.RFC3339, windowStart)
	}
	snapshot.WindowEnd, err = time.Parse(time.RFC3339Nano, windowEnd)
	if err != nil {
		snapshot.WindowEnd, _ = time.Parse(time.RFC3339, windowEnd)
	}
	snapshot.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		snapshot.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	}
	if approvedAt.Valid && approvedAt.String != "" {
		t, parseErr := time.Parse(time.RFC3339Nano, approvedAt.String)
		if parseErr != nil {
			t, _ = time.Parse(time.RFC3339, approvedAt.String)
		}
		snapshot.ApprovedAt = &t
	}
	if contributorsJSON != "" && contributorsJSON != "[]" {
		if err := json.Unmarshal([]byte(contributorsJSON), &snapshot.Contributors); err != nil {
			return nil, fmt.Errorf("unmarshal snapshot contributors: %w", err)
		}
	}
	if allocationsJSON != "" && allocationsJSON != "[]" {
		if err := json.Unmarshal([]byte(allocationsJSON), &snapshot.Allocations); err != nil {
			return nil, fmt.Errorf("unmarshal snapshot allocations: %w", err)
		}
	}

	return &snapshot, nil
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
		&c.Authority, &c.Sponsor, &c.OwnerGitHubUsername, &allocJSON, &createdAtStr,
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

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339Nano)
}
