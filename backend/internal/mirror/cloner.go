package mirror

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const DefaultCloneTimeout = 5 * time.Minute

// Cloner clones and updates bare git repositories on disk.
type Cloner struct {
	storageDir string
}

// NewCloner creates a Cloner that stores bare repos under storageDir.
func NewCloner(storageDir string) *Cloner {
	return &Cloner{storageDir: storageDir}
}

// CloneOrUpdate clones a new bare repo or fetches the latest commits for an
// existing one.  It returns the path to the bare repo on disk.
func (c *Cloner) CloneOrUpdate(ctx context.Context, ownerLogin, repoName, defaultBranch string) (string, error) {
	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", ownerLogin, repoName)
	bareRepoPath := filepath.Join(c.storageDir, ownerLogin, fmt.Sprintf("%s.git", repoName))

	if err := os.MkdirAll(filepath.Dir(bareRepoPath), 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	configPath := filepath.Join(bareRepoPath, "config")
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		if err := c.clone(ctx, repoURL, bareRepoPath, defaultBranch); err != nil {
			return "", err
		}
	} else {
		if err := c.fetch(ctx, bareRepoPath, defaultBranch); err != nil {
			return "", err
		}
	}

	return bareRepoPath, nil
}

func (c *Cloner) clone(ctx context.Context, repoURL, bareRepoPath, defaultBranch string) error {
	cloneCtx, cancel := context.WithTimeout(ctx, DefaultCloneTimeout)
	defer cancel()

	cmd := exec.CommandContext(cloneCtx, "git", "clone", "--bare", "--single-branch",
		"-b", defaultBranch, repoURL, bareRepoPath)
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		if cloneCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("%w: %s", ErrSyncTimeout, bareRepoPath)
		}
		return fmt.Errorf("%w: %s: %s", ErrCloneFailed, err, out)
	}
	return nil
}

func (c *Cloner) fetch(ctx context.Context, bareRepoPath, defaultBranch string) error {
	fetchCtx, cancel := context.WithTimeout(ctx, DefaultCloneTimeout)
	defer cancel()

	cmd := exec.CommandContext(fetchCtx, "git",
		fmt.Sprintf("--git-dir=%s", bareRepoPath),
		"fetch", "origin", defaultBranch)
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		if fetchCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("%w: %s", ErrSyncTimeout, bareRepoPath)
		}
		return fmt.Errorf("%w: %s: %s", ErrFetchFailed, err, out)
	}
	return nil
}

// DetectDefaultBranch returns "main" if that branch exists on the remote,
// otherwise "master".  Falls back to "main" on any error.
func DetectDefaultBranch(ctx context.Context, ownerLogin, repoName string) string {
	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", ownerLogin, repoName)
	cmd := exec.CommandContext(checkCtx, "git", "ls-remote", "--symref", repoURL, "HEAD")
	cmd.Env = gitEnv()
	out, err := cmd.Output()
	if err != nil {
		return "main"
	}
	// Output looks like: "ref: refs/heads/main\tHEAD"
	for _, line := range splitLines(string(out)) {
		if len(line) > 16 && line[:16] == "ref: refs/heads/" {
			parts := splitTab(line)
			if len(parts) > 0 {
				branch := parts[0][16:]
				if branch == "main" || branch == "master" {
					return branch
				}
			}
		}
	}
	return "main"
}

func gitEnv() []string {
	// Pass minimal env to avoid picking up user git config that could change behaviour.
	return []string{
		"HOME=/tmp",
		"GIT_TERMINAL_PROMPT=0",
		// Inherit PATH so git is found.
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitTab(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}
