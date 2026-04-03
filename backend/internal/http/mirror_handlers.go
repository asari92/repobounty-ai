package http

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/repobounty/repobounty-ai/internal/extractor"
	"github.com/repobounty/repobounty-ai/internal/mirror"
	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/store"
)

// GetMirrorStatus handles GET /api/mirror/{owner}/{repo}
func (h *Handlers) GetMirrorStatus(w http.ResponseWriter, r *http.Request) {
	ownerLogin, repoName, ok := parseMirrorRepo(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repo format")
		return
	}

	m, err := h.store.GetRepositoryMirrorByRepo(ownerLogin, repoName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "mirror not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, m)
}

// GetMirrorMetadata handles GET /api/mirror/{owner}/{repo}/metadata
func (h *Handlers) GetMirrorMetadata(w http.ResponseWriter, r *http.Request) {
	ownerLogin, repoName, ok := parseMirrorRepo(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repo format")
		return
	}

	m, err := h.store.GetRepositoryMirrorByRepo(ownerLogin, repoName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "mirror not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	stats, err := h.store.GetMirrorCommitStats(m.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		log.Printf("mirror: get commit stats for mirror %d failed: %v", m.ID, err)
	}
	if stats == nil {
		stats = make(map[string]*models.CommitStats)
	}

	writeJSON(w, http.StatusOK, models.MirrorMetadata{
		RepoID:           m.ID,
		CommitCount:      m.CommitCount,
		AuthorMappings:   make(map[string]int),
		ContributorStats: stats,
	})
}

// GetMirrorCommits handles GET /api/mirror/{owner}/{repo}/commits?limit=50&skip=0&author=...
func (h *Handlers) GetMirrorCommits(w http.ResponseWriter, r *http.Request) {
	ownerLogin, repoName, ok := parseMirrorRepo(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repo format")
		return
	}

	m, err := h.store.GetRepositoryMirrorByRepo(ownerLogin, repoName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "mirror not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if m.SyncStatus != models.MirrorStatusDone {
		writeError(w, http.StatusConflict, "mirror sync not complete")
		return
	}

	limit := 50
	skip := 0
	author := r.URL.Query().Get("author")
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("skip"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			skip = n
		}
	}

	if !h.config.MirrorEnabled || h.config.MirrorStoragePath == "" {
		writeError(w, http.StatusServiceUnavailable, "mirror service is disabled")
		return
	}

	extObj := mirror.NewMetadataExtractor()
	commits, err := extObj.ExtractCommits(m.MirrorPath, m.DefaultBranch)
	if err != nil {
		log.Printf("mirror: extract commits for %s/%s failed: %v", ownerLogin, repoName, err)
		writeError(w, http.StatusInternalServerError, "failed to read commit history")
		return
	}

	// Filter by author if requested.
	if author != "" {
		filtered := commits[:0]
		for _, c := range commits {
			if strings.EqualFold(c.Author, author) {
				filtered = append(filtered, c)
			}
		}
		commits = filtered
	}

	total := len(commits)
	if skip >= total {
		commits = nil
	} else {
		commits = commits[skip:]
		if len(commits) > limit {
			commits = commits[:limit]
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"total":   total,
		"limit":   limit,
		"skip":    skip,
		"commits": commits,
	})
}

// GetMirrorContributors handles GET /api/mirror/{owner}/{repo}/contributors
func (h *Handlers) GetMirrorContributors(w http.ResponseWriter, r *http.Request) {
	ownerLogin, repoName, ok := parseMirrorRepo(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repo format")
		return
	}

	m, err := h.store.GetRepositoryMirrorByRepo(ownerLogin, repoName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "mirror not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	stats, err := h.store.GetMirrorCommitStats(m.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"total": 0, "contributors": []any{}})
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	a := extractor.NewAnalyzer()
	contributors := a.ContributorsFromStats(stats)

	writeJSON(w, http.StatusOK, map[string]any{
		"total":        len(contributors),
		"contributors": contributors,
	})
}

// SyncMirror handles POST /api/mirror/{owner}/{repo}/sync
func (h *Handlers) SyncMirror(w http.ResponseWriter, r *http.Request) {
	ownerLogin, repoName, ok := parseMirrorRepo(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid repo format")
		return
	}

	if !h.config.MirrorEnabled {
		writeError(w, http.StatusServiceUnavailable, "mirror service is disabled")
		return
	}

	m, err := h.store.GetRepositoryMirrorByRepo(ownerLogin, repoName)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "mirror not found for this repo; create a campaign first")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	go h.runMirrorSync(context.Background(), m.CampaignID)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"sync_status": models.MirrorStatusSyncing,
		"message":     "Mirror sync started in background",
	})
}

// runMirrorSync performs the full clone/fetch + metadata extraction for a campaign's mirror.
func (h *Handlers) runMirrorSync(ctx context.Context, campaignID string) {
	m, err := h.store.GetRepositoryMirrorByCampaign(campaignID)
	if err != nil {
		log.Printf("mirror sync: campaign %s not found: %v", campaignID, err)
		return
	}

	// Mark as syncing.
	m.SyncStatus = models.MirrorStatusSyncing
	m.LastErrorMsg = ""
	if updateErr := h.store.UpdateRepositoryMirror(m); updateErr != nil {
		log.Printf("mirror sync: update status to syncing failed for %s: %v", campaignID, updateErr)
	}

	cloner := mirror.NewCloner(h.config.MirrorStoragePath)
	mirrorPath, err := cloner.CloneOrUpdate(ctx, m.OwnerLogin, m.RepoName, m.DefaultBranch)
	if err != nil {
		m.SyncStatus = models.MirrorStatusFailed
		m.LastErrorMsg = err.Error()
		if updateErr := h.store.UpdateRepositoryMirror(m); updateErr != nil {
			log.Printf("mirror sync: failed to mark mirror as failed for %s: %v", campaignID, updateErr)
		}
		log.Printf("mirror sync: clone/update failed for %s/%s: %v", m.OwnerLogin, m.RepoName, err)
		return
	}

	extObj := mirror.NewMetadataExtractor()
	commits, err := extObj.ExtractCommits(mirrorPath, m.DefaultBranch)
	if err != nil {
		m.SyncStatus = models.MirrorStatusFailed
		m.LastErrorMsg = fmt.Sprintf("extract commits: %v", err)
		if updateErr := h.store.UpdateRepositoryMirror(m); updateErr != nil {
			log.Printf("mirror sync: failed to mark mirror as failed for %s: %v", campaignID, updateErr)
		}
		log.Printf("mirror sync: extract commits failed for %s/%s: %v", m.OwnerLogin, m.RepoName, err)
		return
	}

	stats := extObj.ExtractContributorStats(commits)
	if err := h.store.SaveMirrorCommitStats(m.ID, stats); err != nil {
		log.Printf("mirror sync: save stats failed for %s: %v", campaignID, err)
	}

	now := time.Now().UTC()
	m.MirrorPath = mirrorPath
	m.CommitCount = len(commits)
	m.LastSyncedAt = now
	m.SyncStatus = models.MirrorStatusDone
	m.LastErrorMsg = ""
	if updateErr := h.store.UpdateRepositoryMirror(m); updateErr != nil {
		log.Printf("mirror sync: final update failed for %s: %v", campaignID, updateErr)
	}

	log.Printf("mirror sync: completed for %s/%s — %d commits", m.OwnerLogin, m.RepoName, len(commits))
}

// startMirrorByCampaign creates a RepositoryMirror record (if not exists) and kicks off async sync.
func (h *Handlers) startMirrorByCampaign(campaignID, repo string) {
	if !h.config.MirrorEnabled || h.config.MirrorStoragePath == "" {
		return
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return
	}
	ownerLogin := parts[0]
	repoName := parts[1]

	mirrorPath := fmt.Sprintf("%s/%s/%s.git", h.config.MirrorStoragePath, ownerLogin, repoName)
	defaultBranch := "main"

	m := &models.RepositoryMirror{
		CampaignID:    campaignID,
		OwnerLogin:    ownerLogin,
		RepoName:      repoName,
		MirrorPath:    mirrorPath,
		SyncStatus:    models.MirrorStatusPending,
		DefaultBranch: defaultBranch,
	}

	if err := h.store.CreateRepositoryMirror(m); err != nil {
		// Record already exists — check if it needs a sync triggered
		// (happens on container restart when previous sync was interrupted).
		existing, getErr := h.store.GetRepositoryMirrorByCampaign(campaignID)
		if getErr != nil {
			log.Printf("mirror: get existing mirror for campaign %s: %v", campaignID, getErr)
			return
		}
		// Reset stuck syncing state back to pending so the sync can restart.
		if existing.SyncStatus == models.MirrorStatusSyncing {
			existing.SyncStatus = models.MirrorStatusPending
			existing.LastErrorMsg = "reset after interrupted sync"
			if updateErr := h.store.UpdateRepositoryMirror(existing); updateErr != nil {
				log.Printf("mirror: reset stuck syncing status for %s: %v", campaignID, updateErr)
			}
		}
		// Re-trigger sync if it hasn't completed successfully yet.
		if existing.SyncStatus != models.MirrorStatusDone {
			go h.runMirrorSync(context.Background(), campaignID)
		}
		return
	}

	go h.runMirrorSync(context.Background(), campaignID)
}

// parseMirrorRepo extracts {owner} and {repo} from Chi URL params.
// The route uses two wildcard segments: /api/mirror/{owner}/{repo}/...
func parseMirrorRepo(r *http.Request) (ownerLogin, repoName string, ok bool) {
	ownerLogin = chi.URLParam(r, "owner")
	repoName = chi.URLParam(r, "repo")
	if ownerLogin == "" || repoName == "" {
		return "", "", false
	}
	if !repoPattern.MatchString(ownerLogin + "/" + repoName) {
		return "", "", false
	}
	return ownerLogin, repoName, true
}
