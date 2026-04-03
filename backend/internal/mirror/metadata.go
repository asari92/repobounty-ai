package mirror

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"
)

// MetadataExtractor extracts commit and author metadata from a bare git repository.
type MetadataExtractor struct{}

// NewMetadataExtractor creates a MetadataExtractor.
func NewMetadataExtractor() *MetadataExtractor {
	return &MetadataExtractor{}
}

// ExtractCommits reads all non-merge commits from the bare repo at bareRepoPath
// on the given defaultBranch and returns them as CommitInfo slices.
func (me *MetadataExtractor) ExtractCommits(bareRepoPath, defaultBranch string) ([]models.CommitInfo, error) {
	// Use a delimiter unlikely to appear in commit messages.
	const sep = "\x1e"
	const recSep = "\x1f"

	// First pass: list commits with their metadata.
	// --no-merges skips merge commits.
	args := []string{
		fmt.Sprintf("--git-dir=%s", bareRepoPath),
		"log",
		fmt.Sprintf("origin/%s", defaultBranch),
		"--no-merges",
		fmt.Sprintf("--pretty=format:%%H%s%%an%s%%ae%s%%aI%s%%s", sep, sep, sep, sep),
	}
	logCmd := exec.Command("git", args...)
	logOut, err := logCmd.Output()
	if err != nil {
		// Try without "origin/" prefix for freshly cloned bare repos where HEAD is set.
		args[2] = defaultBranch
		logCmd = exec.Command("git", args...)
		logOut, err = logCmd.Output()
		if err != nil {
			return nil, fmt.Errorf("%w: git log: %s", ErrMetadataExtractFailed, err)
		}
	}

	type commitMeta struct {
		sha     string
		author  string
		email   string
		ts      time.Time
		subject string
	}

	lines := splitLines(string(logOut))
	metas := make([]commitMeta, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, sep)
		if len(parts) != 5 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, parts[3])
		if err != nil {
			ts, _ = time.Parse("2006-01-02T15:04:05-07:00", parts[3])
		}
		metas = append(metas, commitMeta{
			sha:     parts[0],
			author:  parts[1],
			email:   parts[2],
			ts:      ts,
			subject: parts[4],
		})
	}

	if len(metas) == 0 {
		return nil, nil
	}

	// Second pass: get diff-stat per commit.
	diffArgs := []string{
		fmt.Sprintf("--git-dir=%s", bareRepoPath),
		"log",
		fmt.Sprintf("origin/%s", defaultBranch),
		"--no-merges",
		"--pretty=format:" + recSep + "%H",
		"--shortstat",
	}
	diffCmd := exec.Command("git", diffArgs...)
	diffOut, err := diffCmd.Output()
	if err != nil {
		diffArgs[2] = defaultBranch
		diffCmd = exec.Command("git", diffArgs...)
		diffOut, _ = diffCmd.Output()
	}

	// Parse shortstat blocks: each block is "\x1fSHA\n\n N files changed, A insertions(+), D deletions(-)"
	diffMap := parseShortstat(string(diffOut), recSep)

	// Build index for fast lookup.
	metaIndex := make(map[string]commitMeta, len(metas))
	for _, m := range metas {
		metaIndex[m.sha] = m
	}

	commits := make([]models.CommitInfo, 0, len(metas))
	for _, m := range metas {
		stat := diffMap[m.sha]
		commits = append(commits, models.CommitInfo{
			SHA:           m.sha,
			Author:        m.author,
			Email:         m.email,
			Message:       m.subject,
			Timestamp:     m.ts,
			IsMergeCommit: false,
			FilesChanged:  stat[0],
			Insertions:    stat[1],
			Deletions:     stat[2],
		})
	}

	return commits, nil
}

// ExtractContributorStats aggregates per-author commit statistics.
func (me *MetadataExtractor) ExtractContributorStats(commits []models.CommitInfo) map[string]*models.CommitStats {
	stats := make(map[string]*models.CommitStats)

	for _, commit := range commits {
		if commit.IsMergeCommit {
			continue
		}
		if commit.Insertions == 0 && commit.Deletions == 0 && commit.FilesChanged == 0 {
			continue
		}

		s, exists := stats[commit.Author]
		if !exists {
			s = &models.CommitStats{Username: commit.Author}
			stats[commit.Author] = s
		}

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

// parseShortstat parses the output of `git log --shortstat` using recSep as the record separator.
// Returns a map of SHA -> [filesChanged, insertions, deletions].
func parseShortstat(output, recSep string) map[string][3]int {
	result := make(map[string][3]int)
	// Split on recSep to get per-commit blocks.
	blocks := strings.Split(output, recSep)
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := splitLines(block)
		if len(lines) == 0 {
			continue
		}
		// First line contains the SHA (everything after the recSep).
		sha := strings.TrimSpace(lines[0])
		if len(sha) < 7 {
			continue
		}
		var files, ins, del int
		for _, line := range lines[1:] {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			files, ins, del = parseShortstatLine(line)
		}
		result[sha] = [3]int{files, ins, del}
	}
	return result
}

// parseShortstatLine parses a line like " 3 files changed, 45 insertions(+), 12 deletions(-)".
func parseShortstatLine(line string) (files, insertions, deletions int) {
	parts := strings.Fields(line)
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		if i+1 >= len(parts) {
			continue
		}
		next := parts[i+1]
		switch {
		case strings.HasPrefix(next, "file"):
			files = n
		case strings.HasPrefix(next, "insertion"):
			insertions = n
		case strings.HasPrefix(next, "deletion"):
			deletions = n
		}
	}
	return
}
