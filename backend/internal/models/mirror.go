package models

import "time"

type RepositoryMirror struct {
	ID            int64     `json:"id"`
	CampaignID    string    `json:"campaign_id"`
	GitHubRepoID  int       `json:"github_repo_id"`
	OwnerLogin    string    `json:"owner_login"`
	RepoName      string    `json:"repo_name"`
	MirrorPath    string    `json:"mirror_path"`
	LastSyncedAt  time.Time `json:"last_synced_at,omitempty"`
	SyncStatus    string    `json:"sync_status"`
	LastErrorMsg  string    `json:"last_error_msg,omitempty"`
	CommitCount   int       `json:"commit_count"`
	DefaultBranch string    `json:"default_branch"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type MirrorMetadata struct {
	RepoID           int64                  `json:"repo_id"`
	CommitCount      int                    `json:"commit_count"`
	AuthorMappings   map[string]int         `json:"author_mappings"`
	ContributorStats map[string]*CommitStats `json:"contributor_stats"`
}

type CommitStats struct {
	Username      string    `json:"username"`
	CommitCount   int       `json:"commit_count"`
	LinesAdded    int       `json:"lines_added"`
	LinesDeleted  int       `json:"lines_deleted"`
	FilesTouched  int       `json:"files_touched"`
	FirstCommitAt time.Time `json:"first_commit_at,omitempty"`
	LastCommitAt  time.Time `json:"last_commit_at,omitempty"`
}

type CommitInfo struct {
	SHA           string    `json:"sha"`
	Author        string    `json:"author"`
	Email         string    `json:"email"`
	Message       string    `json:"message"`
	Timestamp     time.Time `json:"timestamp"`
	IsMergeCommit bool      `json:"is_merge_commit"`
	FilesChanged  int       `json:"files_changed"`
	Insertions    int       `json:"insertions"`
	Deletions     int       `json:"deletions"`
}

const (
	MirrorStatusPending  = "pending"
	MirrorStatusSyncing  = "syncing"
	MirrorStatusDone     = "done"
	MirrorStatusFailed   = "failed"
)
