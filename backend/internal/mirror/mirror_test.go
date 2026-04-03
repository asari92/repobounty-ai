package mirror_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/repobounty/repobounty-ai/internal/mirror"
	"github.com/repobounty/repobounty-ai/internal/models"
)

// setupTestGitRepo initialises a bare-compatible git repository with a few
// commits in a temporary directory.  Returns the working-tree path.
func setupTestGitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Alice",
			"GIT_AUTHOR_EMAIL=alice@example.com",
			"GIT_COMMITTER_NAME=Alice",
			"GIT_COMMITTER_EMAIL=alice@example.com",
			"GIT_AUTHOR_DATE=2024-01-01T12:00:00+00:00",
			"GIT_COMMITTER_DATE=2024-01-01T12:00:00+00:00",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git command %v failed: %s", args, out)
		}
	}

	run("git", "init", "-b", "main")
	run("git", "config", "user.email", "alice@example.com")
	run("git", "config", "user.name", "Alice")

	// First commit
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("git", "add", ".")
	run("git", "commit", "-m", "initial commit")

	// Second commit by Bob
	bobEnv := []string{
		"GIT_AUTHOR_NAME=Bob",
		"GIT_AUTHOR_EMAIL=bob@example.com",
		"GIT_COMMITTER_NAME=Bob",
		"GIT_COMMITTER_EMAIL=bob@example.com",
		"GIT_AUTHOR_DATE=2024-01-02T12:00:00+00:00",
		"GIT_COMMITTER_DATE=2024-01-02T12:00:00+00:00",
	}
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", "bob feature")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), bobEnv...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bob commit failed: %s", out)
	}

	return dir
}

func TestExtractCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	workDir := setupTestGitRepo(t)
	bareDir := t.TempDir()

	// Create a bare clone from the working repo.
	cmd := exec.Command("git", "clone", "--bare", workDir, bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bare clone failed: %s", out)
	}

	extObj := mirror.NewMetadataExtractor()
	commits, err := extObj.ExtractCommits(bareDir, "main")
	if err != nil {
		t.Fatalf("ExtractCommits: %v", err)
	}

	if len(commits) == 0 {
		t.Fatal("expected at least one commit")
	}

	for _, c := range commits {
		if c.SHA == "" {
			t.Error("commit SHA should not be empty")
		}
		if c.Author == "" {
			t.Error("commit author should not be empty")
		}
		if c.Timestamp.IsZero() {
			t.Error("commit timestamp should not be zero")
		}
	}
}

func TestExtractContributorStats(t *testing.T) {
	extObj := mirror.NewMetadataExtractor()

	now := time.Now()
	commits := []models.CommitInfo{
		{SHA: "aaa", Author: "alice", Insertions: 100, Deletions: 50, FilesChanged: 5, IsMergeCommit: false, Timestamp: now.Add(-2 * time.Hour)},
		{SHA: "bbb", Author: "bob", Insertions: 200, Deletions: 100, FilesChanged: 10, IsMergeCommit: false, Timestamp: now.Add(-1 * time.Hour)},
		{SHA: "ccc", Author: "alice", Insertions: 30, Deletions: 10, FilesChanged: 2, IsMergeCommit: false, Timestamp: now},
		// Merge commit — should be skipped.
		{SHA: "ddd", Author: "alice", Insertions: 0, Deletions: 0, FilesChanged: 0, IsMergeCommit: true, Timestamp: now},
	}

	stats := extObj.ExtractContributorStats(commits)

	aliceStat, ok := stats["alice"]
	if !ok {
		t.Fatal("expected stats for alice")
	}
	if aliceStat.CommitCount != 2 {
		t.Errorf("alice commit count: got %d, want 2", aliceStat.CommitCount)
	}
	if aliceStat.LinesAdded != 130 {
		t.Errorf("alice lines added: got %d, want 130", aliceStat.LinesAdded)
	}

	bobStat, ok := stats["bob"]
	if !ok {
		t.Fatal("expected stats for bob")
	}
	if bobStat.CommitCount != 1 {
		t.Errorf("bob commit count: got %d, want 1", bobStat.CommitCount)
	}
	if bobStat.LinesAdded != 200 {
		t.Errorf("bob lines added: got %d, want 200", bobStat.LinesAdded)
	}

	// Empty commits should not appear.
	emptyCommits := []models.CommitInfo{
		{SHA: "eee", Author: "ghost", Insertions: 0, Deletions: 0, FilesChanged: 0, IsMergeCommit: false, Timestamp: now},
	}
	emptyStats := extObj.ExtractContributorStats(emptyCommits)
	if _, found := emptyStats["ghost"]; found {
		t.Error("empty commit should not create a stats entry")
	}
}

func TestClonerSkipsWhenGitUnavailable(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	// Just verify the constructor doesn't panic.
	cloner := mirror.NewCloner(t.TempDir())
	if cloner == nil {
		t.Fatal("NewCloner returned nil")
	}
}

func TestDetectDefaultBranchFallback(t *testing.T) {
	// When called with a non-existent repo URL the function should fall back to "main".
	branch := mirror.DetectDefaultBranch(context.Background(), "nonexistent-owner-xyz", "nonexistent-repo-xyz")
	if branch != "main" && branch != "master" {
		t.Errorf("expected main or master fallback, got %s", branch)
	}
}
