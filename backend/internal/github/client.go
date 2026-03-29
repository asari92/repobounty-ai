package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"
	"github.com/repobounty/repobounty-ai/internal/utils"
)

type Client struct {
	token      string
	httpClient *http.Client
}

func NewClient(token string) *Client {
	return &Client{token: token, httpClient: &http.Client{}}
}

func (c *Client) FetchContributors(ctx context.Context, repo string) ([]models.Contributor, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format, expected owner/repo: %s", repo)
	}
	owner, name := parts[0], parts[1]

	contributors, err := c.fetchContributorStats(ctx, owner, name)
	if err != nil {
		log.Printf("github: API failed (%v), using mock data", err)
		return mockContributors(), nil
	}
	if len(contributors) == 0 {
		log.Printf("github: no contributors found, using mock data")
		return mockContributors(), nil
	}

	prCounts, err := c.fetchPRCounts(ctx, owner, name)
	if err != nil {
		log.Printf("github: PR fetch failed (%v), continuing without PR data", err)
	}

	for i := range contributors {
		if count, ok := prCounts[contributors[i].Username]; ok {
			contributors[i].PullRequests = count
		}
	}

	return contributors, nil
}

func (c *Client) fetchContributorStats(ctx context.Context, owner, repo string) ([]models.Contributor, error) {
	var result []models.Contributor

	err := utils.Retry(ctx, 3, 1*time.Second, func(ctx context.Context) error {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contributors?per_page=10", owner, repo)
		body, err := c.doGet(ctx, url)
		if err != nil {
			return err
		}

		var ghContribs []ghContributor
		if err := json.Unmarshal(body, &ghContribs); err != nil {
			return fmt.Errorf("parse contributors: %w", err)
		}

		result = make([]models.Contributor, 0, len(ghContribs))
		for _, gc := range ghContribs {
			if gc.Login == "" {
				continue
			}
			result = append(result, models.Contributor{
				Username: gc.Login,
				Commits:  gc.Contributions,
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

type ghContributor struct {
	Login         string `json:"login"`
	AvatarURL     string `json:"avatar_url"`
	Contributions int    `json:"contributions"`
}

type ghPullRequest struct {
	User struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (c *Client) fetchPRCounts(ctx context.Context, owner, repo string) (map[string]int, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=all&per_page=100", owner, repo)
	body, err := c.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var prs []ghPullRequest
	if err := json.Unmarshal(body, &prs); err != nil {
		return nil, fmt.Errorf("parse PRs: %w", err)
	}

	counts := make(map[string]int)
	for _, pr := range prs {
		if pr.User.Login != "" {
			counts[pr.User.Login]++
		}
	}
	return counts, nil
}

func (c *Client) doGet(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API %s returned %d", url, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func mockContributors() []models.Contributor {
	return []models.Contributor{
		{Username: "alice-dev", Commits: 47, PullRequests: 12, Reviews: 8, LinesAdded: 3200, LinesDeleted: 980},
		{Username: "bob-builder", Commits: 31, PullRequests: 8, Reviews: 15, LinesAdded: 2100, LinesDeleted: 650},
		{Username: "charlie-fix", Commits: 19, PullRequests: 5, Reviews: 3, LinesAdded: 890, LinesDeleted: 420},
	}
}

type ContributorDetailed struct {
	Username      string `json:"username"`
	AvatarURL     string `json:"avatar_url"`
	Contributions int    `json:"contributions"`
}

type PullRequest struct {
	ID           int    `json:"id"`
	Number       int    `json:"number"`
	Title        string `json:"title"`
	User         string `json:"user"`
	State        string `json:"state"`
	CreatedAt    string `json:"created_at"`
	MergedAt     string `json:"merged_at"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	ChangedFiles int    `json:"changed_files"`
}

func (c *Client) FetchContributorsDetailed(ctx context.Context, repo string) ([]ContributorDetailed, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format, expected owner/repo: %s", repo)
	}
	owner, name := parts[0], parts[1]

	contributors, err := c.fetchContributorStatsDetailed(ctx, owner, name)
	if err != nil {
		log.Printf("github: API failed (%v), using mock data", err)
		return mockContributorsDetailed(), nil
	}
	if len(contributors) == 0 {
		log.Printf("github: no contributors found, using mock data")
		return mockContributorsDetailed(), nil
	}

	return contributors, nil
}

func (c *Client) fetchContributorStatsDetailed(ctx context.Context, owner, repo string) ([]ContributorDetailed, error) {
	var result []ContributorDetailed

	err := utils.Retry(ctx, 3, 1*time.Second, func(ctx context.Context) error {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contributors?per_page=30", owner, repo)
		body, err := c.doGet(ctx, url)
		if err != nil {
			return err
		}

		var ghContribs []ghContributor
		if err := json.Unmarshal(body, &ghContribs); err != nil {
			return fmt.Errorf("parse contributors: %w", err)
		}

		result = make([]ContributorDetailed, 0, len(ghContribs))
		for _, gc := range ghContribs {
			if gc.Login == "" {
				continue
			}
			result = append(result, ContributorDetailed{
				Username:      gc.Login,
				AvatarURL:     gc.AvatarURL,
				Contributions: gc.Contributions,
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) FetchPRsWithDiffs(ctx context.Context, repo string, mergedSince int64) ([]PullRequest, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format, expected owner/repo: %s", repo)
	}
	owner, name := parts[0], parts[1]

	prs, err := c.fetchPRsWithStats(ctx, owner, name, mergedSince)
	if err != nil {
		log.Printf("github: PR fetch failed (%v), using mock data", err)
		return mockPRs(repo), nil
	}
	if len(prs) == 0 {
		log.Printf("github: no PRs found, using mock data")
		return mockPRs(repo), nil
	}

	return prs, nil
}

func (c *Client) fetchPRsWithStats(ctx context.Context, owner, repo string, mergedSince int64) ([]PullRequest, error) {
	var allPRs []PullRequest
	page := 1
	perPage := 100

	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=closed&per_page=%d&page=%d", owner, repo, perPage, page)
		body, err := c.doGet(ctx, url)
		if err != nil {
			return nil, err
		}

		var ghPRs []ghPullRequestDetailed
		if err := json.Unmarshal(body, &ghPRs); err != nil {
			return nil, fmt.Errorf("parse PRs: %w", err)
		}

		if len(ghPRs) == 0 {
			break
		}

		for _, ghPR := range ghPRs {
			if !ghPR.Merged || (mergedSince > 0 && ghPR.MergedAt.Unix() < mergedSince) {
				continue
			}

			allPRs = append(allPRs, PullRequest{
				ID:           ghPR.ID,
				Number:       ghPR.Number,
				Title:        ghPR.Title,
				User:         ghPR.User.Login,
				State:        "merged",
				CreatedAt:    ghPR.CreatedAt,
				MergedAt:     ghPR.MergedAt.Format(time.RFC3339),
				Additions:    ghPR.Additions,
				Deletions:    ghPR.Deletions,
				ChangedFiles: ghPR.ChangedFiles,
			})
		}

		if len(ghPRs) < perPage {
			break
		}
		page++
	}

	return allPRs, nil
}

type ghPullRequestDetailed struct {
	ID     int    `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	State        string    `json:"state"`
	CreatedAt    string    `json:"created_at"`
	MergedAt     time.Time `json:"merged_at"`
	Merged       bool      `json:"merged"`
	Additions    int       `json:"additions"`
	Deletions    int       `json:"deletions"`
	ChangedFiles int       `json:"changed_files"`
}

func (c *Client) FetchPRDiff(ctx context.Context, repo string, prNumber int) (string, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repo format, expected owner/repo: %s", repo)
	}
	owner, name := parts[0], parts[1]

	diff, err := c.fetchPRDiff(ctx, owner, name, prNumber)
	if err != nil {
		log.Printf("github: PR diff fetch failed (%v), using mock data", err)
		return mockPRDiff(prNumber), nil
	}

	return diff, nil
}

func (c *Client) fetchPRDiff(ctx context.Context, owner, repo string, prNumber int) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, prNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3.diff")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API %s returned %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func (c *Client) FetchReviews(ctx context.Context, repo string, prNumber int) ([]Review, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format, expected owner/repo: %s", repo)
	}
	owner, name := parts[0], parts[1]

	reviews, err := c.fetchReviews(ctx, owner, name, prNumber)
	if err != nil {
		log.Printf("github: Reviews fetch failed (%v), using mock data", err)
		return mockReviews(prNumber), nil
	}

	return reviews, nil
}

func (c *Client) fetchReviews(ctx context.Context, owner, repo string, prNumber int) ([]Review, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d/reviews", owner, repo, prNumber)
	body, err := c.doGet(ctx, url)
	if err != nil {
		return nil, err
	}

	var ghReviews []ghReview
	if err := json.Unmarshal(body, &ghReviews); err != nil {
		return nil, fmt.Errorf("parse reviews: %w", err)
	}

	reviews := make([]Review, 0, len(ghReviews))
	for _, ghR := range ghReviews {
		reviews = append(reviews, Review{
			ID:          ghR.ID,
			User:        ghR.User.Login,
			State:       ghR.State,
			Body:        ghR.Body,
			SubmittedAt: ghR.SubmittedAt.Format(time.RFC3339),
		})
	}

	return reviews, nil
}

const (
	maxDiffLinesPerFile    = 50
	maxParallelDiffFetches = 5
)

func truncateDiff(diff string, maxLinesPerFile int) string {
	lines := strings.Split(diff, "\n")
	var result []string
	linesInFile := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git a/") {
			linesInFile = 0
		} else if !strings.HasPrefix(line, "index ") && !strings.HasPrefix(line, "+++ ") && !strings.HasPrefix(line, "--- ") {
			linesInFile++
		}
		if linesInFile <= maxLinesPerFile {
			result = append(result, line)
		} else {
			result = append(result, "...")
		}
	}
	return strings.Join(result, "\n")
}

func (c *Client) FetchContributorsPRDiffs(ctx context.Context, repo string, mergedSinceUnix int64) (map[string][]string, error) {
	prs, err := c.FetchPRsWithDiffs(ctx, repo, mergedSinceUnix)
	if err != nil {
		return nil, err
	}

	contributorPRs := make(map[string][]PullRequest)
	for _, pr := range prs {
		contributorPRs[pr.User] = append(contributorPRs[pr.User], pr)
	}

	type diffJob struct {
		contributor string
		prNumber    int
	}

	var jobs []diffJob
	for contributor, prList := range contributorPRs {
		for _, pr := range prList {
			jobs = append(jobs, diffJob{contributor: contributor, prNumber: pr.Number})
		}
	}

	sort.Slice(jobs, func(i, j int) bool {
		return (jobs[i].prNumber - jobs[j].prNumber) < 0
	})

	result := make(map[string][]string)
	var mu sync.Mutex
	sem := make(chan struct{}, maxParallelDiffFetches)
	var wg sync.WaitGroup

	for _, job := range jobs {
		wg.Add(1)
		go func(j diffJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			diff, err := c.FetchPRDiff(ctx, repo, j.prNumber)
			if err != nil {
				log.Printf("github: failed to fetch diff for PR #%d: %v", j.prNumber, err)
				return
			}

			truncated := truncateDiff(diff, maxDiffLinesPerFile)
			if truncated == "" {
				return
			}

			mu.Lock()
			result[j.contributor] = append(result[j.contributor], truncated)
			mu.Unlock()
		}(job)
	}

	wg.Wait()

	return result, nil
}

type Review struct {
	ID          int    `json:"id"`
	User        string `json:"user"`
	State       string `json:"state"`
	Body        string `json:"body"`
	SubmittedAt string `json:"submitted_at"`
}

type ghReview struct {
	ID   int `json:"id"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	State       string    `json:"state"`
	Body        string    `json:"body"`
	SubmittedAt time.Time `json:"submitted_at"`
}

func mockContributorsDetailed() []ContributorDetailed {
	return []ContributorDetailed{
		{Username: "alice-dev", AvatarURL: "https://github.com/alice-dev.png", Contributions: 47},
		{Username: "bob-builder", AvatarURL: "https://github.com/bob-builder.png", Contributions: 31},
		{Username: "charlie-fix", AvatarURL: "https://github.com/charlie-fix.png", Contributions: 19},
	}
}

func mockPRs(repo string) []PullRequest {
	return []PullRequest{
		{
			ID:           12345,
			Number:       42,
			Title:        "Add authentication flow",
			User:         "alice-dev",
			State:        "merged",
			CreatedAt:    "2024-03-15T10:30:00Z",
			MergedAt:     "2024-03-16T14:20:00Z",
			Additions:    450,
			Deletions:    120,
			ChangedFiles: 8,
		},
		{
			ID:           12346,
			Number:       43,
			Title:        "Fix memory leak in scheduler",
			User:         "bob-builder",
			State:        "merged",
			CreatedAt:    "2024-03-17T09:15:00Z",
			MergedAt:     "2024-03-18T11:45:00Z",
			Additions:    85,
			Deletions:    230,
			ChangedFiles: 3,
		},
		{
			ID:           12347,
			Number:       44,
			Title:        "Implement rate limiting",
			User:         "alice-dev",
			State:        "merged",
			CreatedAt:    "2024-03-19T16:00:00Z",
			MergedAt:     "2024-03-20T09:30:00Z",
			Additions:    320,
			Deletions:    45,
			ChangedFiles: 5,
		},
	}
}

func mockPRDiff(prNumber int) string {
	return fmt.Sprintf(`diff --git a/src/auth/auth.go b/src/auth/auth.go
index 1234567..abcdefg 100644
--- a/src/auth/auth.go
+++ b/src/auth/auth.go
@@ -10,6 +10,12 @@ import (
 	"time"
 )
 
+// JWTManager handles JWT token generation and validation
+type JWTManager struct {
+	secret string
+}
+
 func NewJWTManager(secret string) *JWTManager {
 	return &JWTManager{secret: secret}
 }
@@ -45,7 +51,15 @@ func (m *JWTManager) Validate(token string) (*Claims, error) {
 		return nil, err
 	}
 	
-	// TODO: validate claims
+	if claims.ExpiresAt < time.Now().Unix() {
+		return nil, errors.New("token expired")
+	}
+	
+	if claims.Issuer != "repobounty" {
+		return nil, errors.New("invalid issuer")
+	}
+	
 	return claims, nil
 }`)
}

func mockReviews(prNumber int) []Review {
	return []Review{
		{
			ID:          789,
			User:        "bob-builder",
			State:       "APPROVED",
			Body:        "Looks good! The token validation is more robust now.",
			SubmittedAt: "2024-03-16T15:00:00Z",
		},
		{
			ID:          790,
			User:        "charlie-fix",
			State:       "COMMENTED",
			Body:        "Consider adding rate limiting for the token refresh endpoint.",
			SubmittedAt: "2024-03-16T15:30:00Z",
		},
	}
}
