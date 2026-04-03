# Mirror & Data Extraction Service Integration

## Контекст
Интегрировать в RepoBounty AI функциональность зеркалирования GitHub репозиториев 
и извлечения метаданных для анализа контрибьюторов на этапе финализации кампании.

**Текущее состояние**: 
- Папки `internal/mirror/` и `internal/extractor/` существуют, но пусты
- GitHub клиент в `internal/github/client.go` уже выполняет запросы к API
- `internal/models/` содержит `Campaign`, `Contributor`, `Allocation`
- Хранилище: SQLite и in-memory в `internal/store/`

## Требования

### 1. Архитектура
- **Хранилище**: На диске в `backend/data/mirrors/` с git bare repository для каждого репо
- **БД структура**: Новая таблица `repository_mirrors` с полями:
  - `campaign_id` (FK), `github_repo_id`, `mirror_path`, `last_synced_at`, `sync_status` (pending/syncing/done/failed)
  - JSONB поля: `commit_shas` (list), `author_mappings` (github_username → github_user_id)
- **Только main/master**: Синхронизировать только основную ветку
- **Периодическая синхронизация**: Фоновый worker (раз в день) через `internal/http/worker.go`

### 2. Модули
- `internal/mirror/cloner.go` — Clone/update git bare repo (git2go или exec.Command)
- `internal/mirror/metadata.go` — Extract commits, authors, stats from mirror
- `internal/extractor/analyzer.go` — Parse commit diffs, map authors → contributors
- **Schema migrations**: SQLite схема для `repository_mirrors`

## Конкретные структуры

### Models в `internal/models/mirror.go`

```go
package models

import "time"

type RepositoryMirror struct {
	ID              int64  `json:"id"`
	CampaignID      string `json:"campaign_id"`
	GitHubRepoID    int    `json:"github_repo_id"`
	OwnerLogin      string `json:"owner_login"`     // owner/repo
	RepoName        string `json:"repo_name"`
	MirrorPath      string `json:"mirror_path"`     // /backend/data/mirrors/owner/repo.git
	LastSyncedAt    time.Time `json:"last_synced_at"`
	SyncStatus      string `json:"sync_status"`     // pending, syncing, done, failed
	LastErrorMsg    string `json:"last_error_msg,omitempty"`
	CommitCount     int    `json:"commit_count"`
	DefaultBranch   string `json:"default_branch"`  // main или master
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type MirrorMetadata struct {
	RepoID          int64                     `json:"repo_id"`
	CommitSHAs      []string                  `json:"commit_shas"`           // list of commit SHA1
	AuthorMappings  map[string]int            `json:"author_mappings"`       // github_username -> github_user_id
	ContributorStats map[string]*CommitStats `json:"contributor_stats"`     // username -> stats
}

type CommitStats struct {
	Username      string `json:"username"`
	CommitCount   int    `json:"commit_count"`
	LinesAdded    int    `json:"lines_added"`
	LinesDeleted  int    `json:"lines_deleted"`
	FilesTouched  int    `json:"files_touched"`
	FirstCommitAt time.Time `json:"first_commit_at"`
	LastCommitAt  time.Time `json:"last_commit_at"`
}

type CommitInfo struct {
	SHA       string    `json:"sha"`
	Author    string    `json:"author"`
	Email     string    `json:"email"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	// Не include merge commits
	IsMergeCommit bool `json:"is_merge_commit"`
	// Из diff-stat
	FilesChanged int `json:"files_changed"`
	Insertions   int `json:"insertions"`
	Deletions    int `json:"deletions"`
}
```

### SQL Schema для `repository_mirrors`

```sql
CREATE TABLE IF NOT EXISTS repository_mirrors (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  campaign_id TEXT NOT NULL UNIQUE,
  github_repo_id INTEGER NOT NULL,
  owner_login TEXT NOT NULL,
  repo_name TEXT NOT NULL,
  mirror_path TEXT NOT NULL UNIQUE,
  last_synced_at DATETIME,
  sync_status TEXT NOT NULL DEFAULT 'pending', -- pending, syncing, done, failed
  last_error_msg TEXT,
  commit_count INTEGER DEFAULT 0,
  default_branch TEXT DEFAULT 'main',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY(campaign_id) REFERENCES campaigns(campaign_id)
);

CREATE TABLE IF NOT EXISTS mirror_commit_stats (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  mirror_id INTEGER NOT NULL,
  username TEXT NOT NULL,
  github_user_id INTEGER,
  commit_count INTEGER DEFAULT 0,
  lines_added INTEGER DEFAULT 0,
  lines_deleted INTEGER DEFAULT 0,
  files_touched INTEGER DEFAULT 0,
  first_commit_at DATETIME,
  last_commit_at DATETIME,
  FOREIGN KEY(mirror_id) REFERENCES repository_mirrors(id)
);

CREATE INDEX idx_mirror_campaign ON repository_mirrors(campaign_id);
CREATE INDEX idx_mirror_sync_status ON repository_mirrors(sync_status);
```

### 3. API Endpoints с примерами

#### `GET /api/mirror/{repo}` (e.g. `/api/mirror/facebook/react`)
**Response:**
```json
{
  "id": 42,
  "campaign_id": "camp_abc123",
  "github_repo_id": 12345,
  "owner_login": "facebook",
  "repo_name": "react",
  "mirror_path": "/app/backend/data/mirrors/facebook/react.git",
  "last_synced_at": "2026-04-03T15:30:00Z",
  "sync_status": "done",
  "commit_count": 15420,
  "default_branch": "main",
  "created_at": "2026-04-01T10:00:00Z"
}
```

#### `GET /api/mirror/{repo}/metadata`
**Response:**
```json
{
  "repo_id": 42,
  "commit_count": 15420,
  "author_mappings": {
    "dan_abramov": 2815346,
    "gaearon": 810438,
    "sophiebits": 1234567
  },
  "contributor_stats": {
    "dan_abramov": {
      "username": "dan_abramov",
      "commit_count": 1204,
      "lines_added": 45320,
      "lines_deleted": 23100,
      "files_touched": 891,
      "first_commit_at": "2013-05-10T15:22:00Z",
      "last_commit_at": "2026-03-28T10:15:00Z"
    },
    "gaearon": {
      "username": "gaearon",
      "commit_count": 892,
      "lines_added": 32100,
      "lines_deleted": 15200,
      "files_touched": 645,
      "first_commit_at": "2015-03-15T08:45:00Z",
      "last_commit_at": "2026-03-25T14:30:00Z"
    }
  }
}
```

#### `GET /api/mirror/{repo}/commits?limit=50&skip=0&author=dan_abramov`
**Response:**
```json
{
  "total": 15420,
  "limit": 50,
  "skip": 0,
  "commits": [
    {
      "sha": "e9c55e0b8c5c5e5e5e5e5e5e5e5e5e5e",
      "author": "dan_abramov",
      "email": "dan.abramov@fb.com",
      "message": "Add hooks API",
      "timestamp": "2018-10-30T12:45:00Z",
      "is_merge_commit": false,
      "files_changed": 12,
      "insertions": 3450,
      "deletions": 1200
    }
  ]
}
```

#### `GET /api/mirror/{repo}/contributors`
**Response:**
```json
{
  "total": 1847,
  "contributors": [
    {
      "username": "dan_abramov",
      "github_user_id": 2815346,
      "commit_count": 1204,
      "lines_added": 45320,
      "lines_deleted": 23100,
      "files_touched": 891
    },
    {
      "username": "gaearon",
      "github_user_id": 810438,
      "commit_count": 892,
      "lines_added": 32100,
      "lines_deleted": 15200,
      "files_touched": 645
    }
  ]
}
```

#### `POST /api/mirror/{repo}/sync`
**Request:**
```json
{}
```
**Response:** (202 Accepted)
```json
{
  "sync_status": "syncing",
  "message": "Mirror sync started in background"
}
```

## Примеры реализации

### `internal/mirror/cloner.go` (концепция)
```go
package mirror

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const CloneTimeout = 5 * time.Minute

type Cloner struct {
	storageDir string // /app/backend/data/mirrors
}

// CloneOrUpdate клонирует или обновляет bare repository
func (c *Cloner) CloneOrUpdate(ctx context.Context, ownerLogin, repoName, defaultBranch string) (string, error) {
	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", ownerLogin, repoName)
	bareRepoPath := filepath.Join(c.storageDir, ownerLogin, fmt.Sprintf("%s.git", repoName))
	
	// Создать директорию если не существует
	if err := os.MkdirAll(filepath.Dir(bareRepoPath), 0755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}
	
	// Проверить existe ли bare repo
	if _, err := os.Stat(bareRepoPath); os.IsNotExist(err) {
		// Clone as bare
		ctx, cancel := context.WithTimeout(ctx, CloneTimeout)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, "git", "clone", "--bare", "--single-branch", "-b", defaultBranch, repoURL, bareRepoPath)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git clone bare: %w", err)
		}
	} else {
		// Update existing
		ctx, cancel := context.WithTimeout(ctx, CloneTimeout)
		defer cancel()
		
		cmd := exec.CommandContext(ctx, "git", "-C", bareRepoPath, "fetch", "origin", defaultBranch)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git fetch: %w", err)
		}
	}
	
	return bareRepoPath, nil
}
```

### `internal/mirror/metadata.go` (концепция)
```go
package mirror

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"
)

type MetadataExtractor struct{}

// ExtractCommits лезит в git repo и extracts commits
func (me *MetadataExtractor) ExtractCommits(bareRepoPath, defaultBranch string) ([]models.CommitInfo, error) {
	// git --git-dir=bareRepoPath log origin/defaultBranch --pretty=format:%H|%an|%ae|%ai|%s|%b --numstat
	var commits []models.CommitInfo
	
	// Пример git log с numstat для получения statistics
	cmd := exec.Command("git", 
		fmt.Sprintf("--git-dir=%s", bareRepoPath),
		"log", 
		fmt.Sprintf("origin/%s", defaultBranch),
		"--pretty=format:%H|%an|%ae|%ai|%s|%b",
		"--numstat",
		"--reverse",
	)
	
	output, err := cmd.Output()
	// Парсить output и заполнить commits slice
	// Skip merge commits (where %b содержит "Merge pull request" или файл /dev/null)
	
	return commits, err
}

// ExtractContributorStats агрегирует статыstics по контурибьюторам
func (me *MetadataExtractor) ExtractContributorStats(commits []models.CommitInfo) map[string]*models.CommitStats {
	stats := make(map[string]*models.CommitStats)
	
	for _, commit := range commits {
		if commit.IsMergeCommit {
			continue // Skip merge commits
		}
		
		if _, exists := stats[commit.Author]; !exists {
			stats[commit.Author] = &models.CommitStats{
				Username: commit.Author,
			}
		}
		
		s := stats[commit.Author]
		s.CommitCount++
		s.LinesAdded += commit.Insertions
		s.LinesDeleted += commit.Deletions
		s.FilesTouched += commit.FilesChanged
		
		if s.FirstCommitAt.IsZero() || commit.Timestamp.Before(s.FirstCommitAt) {
			s.FirstCommitAt = commit.Timestamp
		}
		if commit.Timestamp.After(s.LastCommitAt) {
			s.LastCommitAt = commit.Timestamp
		}
	}
	
	return stats
}
```

### Интеграция в `internal/http/handlers.go` (концепция)
```go
// GetMirrorStatus возвращает статус зеркала
func (h *Handlers) GetMirrorStatus(w http.ResponseWriter, r *http.Request) {
	repo := chi.URLParam(r, "repo") // "facebook/react"
	
	mirror, err := h.store.GetRepositoryMirror(r.Context(), repo)
	if err != nil {
		writeError(w, http.StatusNotFound, "Mirror not found")
		return
	}
	
	writeJSON(w, http.StatusOK, mirror)
}

// SyncMirror запускает асинхронную синхронизацию
func (h *Handlers) SyncMirror(w http.ResponseWriter, r *http.Request) {
	repo := chi.URLParam(r, "repo")
	
	// Запустить асинхронно через worker
	go h.syncMirrorAsync(repo)
	
	writeJSON(w, http.StatusAccepted, map[string]string{
		"sync_status": "syncing",
		"message": "Mirror sync started in background",
	})
}
```

### 4. Интеграция с кампаниями

#### Шаг 1: При создании кампании (`POST /api/campaigns`)
```go
// В handlers.go CreateCampaign()
campaign := &models.Campaign{
	CampaignID: generateID(),
	Repo: req.Repo,
	// ...
}

// Сохранить в БД
if err := h.store.CreateCampaign(ctx, campaign); err != nil {
	writeError(w, 500, "Failed to create campaign")
	return
}

// Запустить асинхронно Clone + Extract Metadata
go func() {
	cloner := mirror.NewCloner(h.config.MirrorStorageDir)
	owner, repo := parseRepo(req.Repo) // "facebook/react" -> ("facebook", "react")
	
	mirrorPath, err := cloner.CloneOrUpdate(ctx, owner, repo, "main")
	if err != nil {
		h.log.Printf("Failed to clone mirror: %v", err)
		return
	}
	
	extractor := mirror.NewMetadataExtractor()
	commits, err := extractor.ExtractCommits(mirrorPath, "main")
	if err != nil {
		h.log.Printf("Failed to extract commits: %v", err)
		return
	}
	
	stats := extractor.ExtractContributorStats(commits)
	h.store.SaveRepositoryMirror(ctx, campaign.CampaignID, mirrorPath, len(commits), stats)
}()
```

#### Шаг 2: На финализации кампании (`POST /api/campaigns/{id}/finalize`)
```go
// В handlers.go FinalizeCampaign()
// Сначала попытаться взять данные из зеркала
mirror, err := h.store.GetRepositoryMirror(ctx, campaign.CampaignID)
if err == nil && mirror.SyncStatus == "done" {
	// Использовать локальные данные
	contributors := mirror.GetContributors() // из stats
	allocations, err := h.ai.AllocateFunds(ctx, campaign, contributors, "mirror")
} else {
	// Fallback на GitHub API
	contributors, err := h.github.GetContributors(ctx, campaign.Repo)
	allocations, err := h.ai.AllocateFunds(ctx, campaign, contributors, "github")
}
```

#### Шаг 3: Фоновый worker в `internal/http/worker.go`
```go
// Добавить в startSyncWorker()
func (h *Handlers) startMirrorSyncWorker() {
	ticker := time.NewTicker(h.config.MirrorSyncInterval) // default 24 hours (1 day)
	defer ticker.Stop()
	
	for range ticker.C {
		mirrors, err := h.store.GetMirrorsNeedingSync(context.Background())
		if err != nil {
			h.log.Printf("Failed to get mirrors: %v", err)
			continue
		}
		
		for _, m := range mirrors {
			go h.syncMirrorAsync(m.CampaignID)
		}
	}
}
```

### 5. Ошибки и edge cases (конкретно)

```go
// internal/mirror/errors.go (Sentinel errors)
package mirror

import "errors"

var (
	ErrMirrorNotFound         = errors.New("mirror not found")
	ErrCloneFailed            = errors.New("git clone failed")
	ErrMetadataExtractFailed  = errors.New("metadata extraction failed")
	ErrSyncTimeout            = errors.New("mirror sync timeout (5 min)")
	ErrMergeCommit            = errors.New("skip merge commit")
	ErrEmptyCommit            = errors.New("skip empty commit")
)

// Логика:
// 1. Игнорировать merge commits:
func isMergeCommit(message string) bool {
	return strings.Contains(message, "Merge pull request") ||
	       strings.Contains(message, "Merge branch")
}

// 2. Пустые коммиты (insertions == 0 && deletions == 0):
func isEmpty(insertions, deletions int) bool {
	return insertions == 0 && deletions == 0
}

// 3. Author mapping:
//    - Сначала проверить: store.GetAuthorMapping(author) 
//    - Если не найти: запросить через GitHub API /repos/{owner}/{repo}/contributors
//    - Кэшировать в mirror_metadata.author_mappings
func mapAuthorToID(author, owner, repo string, store Store, github GitHubClient) (int, error) {
	// Check cache
	if id, exists := store.GetAuthorMapping(author); exists {
		return id, nil
	}
	
	// Query GitHub API (no auth needed for public repos)
	contributors, err := github.GetContributorsNoAuth(context.Background(), owner, repo)
	if err != nil {
		return 0, fmt.Errorf("failed to get contributors: %w", err)
	}
	
	for _, c := range contributors {
		if c.Login == author {
			store.SaveAuthorMapping(author, c.ID)
			return c.ID, nil
		}
	}
	
	return 0, fmt.Errorf("author %s not found", author)
}

// 4. Конкурентность: max 3 одновременных clones
func (h *Handlers) startMirrorSyncWorker() {
	semaphore := make(chan struct{}, 3) // max 3 concurrent
	ticker := time.NewTicker(h.config.MirrorSyncInterval)
	defer ticker.Stop()
	
	for range ticker.C {
		mirrors, _ := h.store.GetMirrorsNeedingSync(context.Background())
		for _, m := range mirrors {
			go func(mirror *models.RepositoryMirror) {
				semaphore <- struct{}{}
				defer func() { <-semaphore }()
				h.syncMirrorAsync(mirror.CampaignID)
			}(m)
		}
	}
}

// 5. Timeout protection (5 minutes max)
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
```

### 6. Тестирование (конкретно)

#### Unit тесты: `internal/mirror/cloner_test.go`
```go
func TestCloneOrUpdateBare(t *testing.T) {
	// Создать временный bare repo для теста
	tmpDir := t.TempDir()
	
	cloner := NewCloner(tmpDir)
	
	// Test 1: Clone new repo
	path, err := cloner.CloneOrUpdate(context.Background(), "test-owner", "test-repo", "main")
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(path, "config"))
	
	// Test 2: Update existing repo
	path2, err := cloner.CloneOrUpdate(context.Background(), "test-owner", "test-repo", "main")
	require.NoError(t, err)
	require.Equal(t, path, path2)
}

func TestExtractCommits(t *testing.T) {
	// Используйте git init для создания тестовог репо с коммитами
	tmpDir := t.TempDir()
	setupTestGitRepo(tmpDir)
	
	extractor := NewMetadataExtractor()
	commits, err := extractor.ExtractCommits(tmpDir, "master")
	
	require.NoError(t, err)
	require.Greater(t, len(commits), 0)
}
```

#### Integration тесты: `internal/mirror/metadata_test.go`
```go
func TestExtractContributorStats(t *testing.T) {
	commits := []models.CommitInfo{
		{
			Author: "alice",
			Insertions: 100,
			Deletions: 50,
			FilesChanged: 5,
			IsMergeCommit: false,
		},
		{
			Author: "bob",
			Insertions: 200,
			Deletions: 100,
			FilesChanged: 10,
			IsMergeCommit: false,
		},
	}
	
	extractor := NewMetadataExtractor()
	stats := extractor.ExtractContributorStats(commits)
	
	assert.Equal(t, 1204, stats["alice"].CommitCount)
	assert.Equal(t, stats["bob"].LinesAdded, 200)
}
```

### 7. Конвенции проекта
- Следуй Go style guide из `AGENTS.md`
- JSON field tags: `snake_case`, `omitempty` для опциональных полей
- Ошибки: `fmt.Errorf("context: %w", err)`, sentinel errors `var ErrMirrorFailed = errors.New(...)`
- Структуры моделей в `internal/models/`, логика в отдельных пакетах

## Как это используется

1. **Создание кампании** → Clone репо → Запустить фоновую синхи
2. **Финализация** → Параллельный запрос к зеркалу вместо GitHub API
3. **Фоновый worker** → Раз в день обновляет старые зеркала
4. **API** → Аналитика по commit/contributor данным из зеркала

## Файлы для создания/изменения
- `internal/mirror/cloner.go` — git clone/pull logic
- `internal/mirror/metadata.go` — извлечение commits/authors
- `internal/extractor/analyzer.go` — парсинг diff аналитики
- `internal/models/mirror.go` — структуры (RepositoryMirror, MirrorMetadata)
- `internal/store/[sqlite|memory].go` — миграция schema + методы
- `internal/http/handlers.go` — 5 endpoints
- `internal/http/worker.go` — добавить sync worker
- `backend/.env.example` — `MIRROR_STORAGE_PATH`, `MIRROR_SYNC_INTERVAL`

## Переменные окружения

Добавить в `backend/.env.example`:
```env
# Mirror service configuration
MIRROR_STORAGE_PATH=./data/mirrors              # Локальное хранилище зеркал
MIRROR_SYNC_INTERVAL=24h                        # Интервал фонового синхра (раз в день)
MIRROR_CLONE_TIMEOUT=300                        # Timeout в секундах (default 300 = 5 min)
MIRROR_MAX_CONCURRENT=3                         # Max одновременных clones
MIRROR_ENABLED=true                             # Включить/отключить сервис
```

Загрузить в `internal/config/config.go`:
```go
type Config struct {
	// ... existing fields
	MirrorStoragePath   string
	MirrorSyncInterval  time.Duration
	MirrorCloneTimeout  time.Duration
	MirrorMaxConcurrent int
	MirrorEnabled       bool
}

func Load() *Config {
	c := &Config{
		// ...
		MirrorStoragePath:   os.Getenv("MIRROR_STORAGE_PATH"),
		MirrorSyncInterval:  parseDuration(os.Getenv("MIRROR_SYNC_INTERVAL"), 24*time.Hour),
		MirrorCloneTimeout:  parseDuration(os.Getenv("MIRROR_CLONE_TIMEOUT"), 5*time.Minute),
		MirrorMaxConcurrent: parseInt(os.Getenv("MIRROR_MAX_CONCURRENT"), 3),
		MirrorEnabled:       os.Getenv("MIRROR_ENABLED") != "false",
	}
	return c
}
```

## Порядок реализации (пошагово)

1. ✅ **Создать модели** → `internal/models/mirror.go` с RepositoryMirror, MirrorMetadata, CommitInfo структурами
2. ✅ **Создать schema** → migrations в store (sqlite + memory)
3. ✅ **Реализовать cloner** → `internal/mirror/cloner.go` с CloneOrUpdate()
4. ✅ **Реализовать extractor** → `internal/mirror/metadata.go` с ExtractCommits() и ExtractContributorStats()
5. ✅ **Добавить API endpoints** → в `internal/http/handlers.go` 5 endpoints
6. ✅ **Интегрировать с campaign creation** → асинхронный clone при POST /api/campaigns
7. ✅ **Интегрировать с finalize** → использовать mirror вместо GitHub API
8. ✅ **Добавить background worker** → синхра каждый час в `internal/http/worker.go`
9. ✅ **Написать тесты** → unit + integration тесты
10. ✅ **Обновить env** → .env.example + config.go

## Проверочный чек-лист

- [ ] `internal/models/mirror.go` создан с 5+ структурами
- [ ] SQLite schema миграция создана (repository_mirrors таблица)
- [ ] `internal/mirror/cloner.go` реализован с error handling
- [ ] `internal/mirror/metadata.go` parseит commits и statisticу
- [ ] `internal/http/handlers.go` имеет 5 endpoints
- [ ] `POST /api/campaigns` запускает async clone
- [ ] `POST /api/campaigns/{id}/finalize` использует mirror data if available
- [ ] Background worker запускает синхра каждый час
- [ ] Author mapping работает: БД -> GitHub API
- [ ] Merge commits и empty commits пропускаются
- [ ] Timeout protection 5 мин на все clone операции
- [ ] All tests pass: `go test ./internal/mirror/... ./internal/extractor/...`
