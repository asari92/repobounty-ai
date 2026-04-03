package extractor

import (
	"strings"

	"github.com/repobounty/repobounty-ai/internal/models"
)

// Analyzer maps raw commit data to contributor models used by the allocation engine.
type Analyzer struct{}

// NewAnalyzer creates an Analyzer.
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// ContributorsFromStats converts mirror CommitStats into the Contributor slice
// expected by the AI allocator.
func (a *Analyzer) ContributorsFromStats(stats map[string]*models.CommitStats) []models.Contributor {
	result := make([]models.Contributor, 0, len(stats))
	for _, s := range stats {
		result = append(result, models.Contributor{
			Username:     s.Username,
			Commits:      s.CommitCount,
			LinesAdded:   s.LinesAdded,
			LinesDeleted: s.LinesDeleted,
		})
	}
	return result
}

// IsMergeCommit returns true when the commit subject looks like a merge commit.
func IsMergeCommit(subject string) bool {
	lower := strings.ToLower(subject)
	return strings.HasPrefix(lower, "merge pull request") ||
		strings.HasPrefix(lower, "merge branch") ||
		strings.HasPrefix(lower, "merge remote-tracking")
}
