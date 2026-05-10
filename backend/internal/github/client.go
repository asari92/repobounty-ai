package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

type RepositoryMetadata struct {
	ID            uint64
	Owner         string
	Name          string
	HTMLURL       string
	DefaultBranch string
}

func NewClient(token string) *Client {
	return &Client{token: token, httpClient: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) RepositoryExists(ctx context.Context, repo string) (bool, error) {
	_, found, err := c.fetchRepositoryMetadata(ctx, repo)
	return found, err
}

func (c *Client) RepositoryID(ctx context.Context, repo string) (uint64, error) {
	metadata, found, err := c.fetchRepositoryMetadata(ctx, repo)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, fmt.Errorf("repository was not found or is not public")
	}
	return metadata.ID, nil
}

func (c *Client) GetDefaultBranch(ctx context.Context, repo string) (string, error) {
	meta, found, err := c.fetchRepositoryMetadata(ctx, repo)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("repository %s not found", repo)
	}
	if meta.DefaultBranch != "" {
		return meta.DefaultBranch, nil
	}

	branches, err := c.listBranches(ctx, repo)
	if err != nil {
		return "", fmt.Errorf("cannot determine default branch for %s: %w", repo, err)
	}
	if len(branches) == 1 {
		return branches[0], nil
	}
	return "", fmt.Errorf("cannot determine default branch for %s: %d branches found and no default_branch from GitHub API", repo, len(branches))
}

func (c *Client) listBranches(ctx context.Context, repo string) ([]string, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format: %s", repo)
	}
	u := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches?per_page=100", parts[0], parts[1])
	body, err := c.doGet(ctx, u)
	if err != nil {
		return nil, err
	}

	var result []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse branches: %w", err)
	}

	names := make([]string, 0, len(result))
	for _, b := range result {
		if b.Name != "" {
			names = append(names, b.Name)
		}
	}
	return names, nil
}

func (c *Client) fetchRepositoryMetadata(ctx context.Context, repo string) (*RepositoryMetadata, bool, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, false, fmt.Errorf("invalid repo format, expected owner/repo: %s", repo)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", parts[0], parts[1])
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var payload struct {
			ID            uint64 `json:"id"`
			Name          string `json:"name"`
			HTMLURL       string `json:"html_url"`
			DefaultBranch string `json:"default_branch"`
			Owner         struct {
				Login string `json:"login"`
			} `json:"owner"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
			return nil, false, fmt.Errorf("parse github repository metadata: %w", err)
		}
		return &RepositoryMetadata{
			ID:            payload.ID,
			Owner:         payload.Owner.Login,
			Name:          payload.Name,
			HTMLURL:       payload.HTMLURL,
			DefaultBranch: payload.DefaultBranch,
		}, true, nil
	case http.StatusNotFound:
		return nil, false, nil
	default:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, false, fmt.Errorf("github repo lookup %s returned %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func (c *Client) FetchContributors(ctx context.Context, repo string) ([]models.Contributor, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format, expected owner/repo: %s", repo)
	}
	owner, name := parts[0], parts[1]

	contributors, err := c.fetchContributorStats(ctx, owner, name)
	if err != nil {
		return nil, fmt.Errorf("github API failed: %w", err)
	}
	if len(contributors) == 0 {
		return nil, fmt.Errorf("no contributors found for %s/%s", owner, name)
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
		page := 1
		perPage := 100
		var allGhContribs []ghContributor

		for {
			url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contributors?per_page=%d&page=%d", owner, repo, perPage, page)
			body, err := c.doGet(ctx, url)
			if err != nil {
				return err
			}

			var ghContribs []ghContributor
			if err := json.Unmarshal(body, &ghContribs); err != nil {
				return fmt.Errorf("parse contributors: %w", err)
			}

			if len(ghContribs) == 0 {
				break
			}

			allGhContribs = append(allGhContribs, ghContribs...)

			if len(ghContribs) < perPage {
				break
			}
			page++
		}

		result = make([]models.Contributor, 0, len(allGhContribs))
		for _, gc := range allGhContribs {
			if gc.Login == "" {
				continue
			}
			result = append(result, models.Contributor{
				GithubUserID: gc.ID,
				Username:     gc.Login,
				Commits:      gc.Contributions,
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
	ID            uint64 `json:"id"`
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

	return io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
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
		return nil, fmt.Errorf("github API failed: %w", err)
	}
	if len(contributors) == 0 {
		return nil, fmt.Errorf("no contributors found for %s/%s", owner, name)
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
		return nil, fmt.Errorf("github PR API failed: %w", err)
	}
	if len(prs) == 0 {
		return prs, nil
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
	Commits      int       `json:"commits"`
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
		return "", fmt.Errorf("github PR diff API failed: %w", err)
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
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
		return nil, fmt.Errorf("github reviews API failed: %w", err)
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

// buildPRDiffMap fetches diffs for PRs and builds contributor->diffs map
func (c *Client) buildPRDiffMap(prs []PullRequest, ctx context.Context, repo string) map[string][]string {
	type diffJob struct {
		contributor string
		prNumber    int
	}

	var jobs []diffJob
	for _, pr := range prs {
		if pr.User == "" {
			continue
		}
		jobs = append(jobs, diffJob{contributor: pr.User, prNumber: pr.Number})
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
	return result
}

func (c *Client) FetchContributorsPRDiffs(ctx context.Context, repo string, mergedSinceUnix int64) (map[string][]string, error) {
	prs, err := c.FetchPRsWithDiffs(ctx, repo, mergedSinceUnix)
	if err != nil {
		return nil, fmt.Errorf("github PR API failed: %w", err)
	}
	if len(prs) == 0 {
		return map[string][]string{}, nil
	}

	return c.buildPRDiffMap(prs, ctx, repo), nil
}

func (c *Client) FetchBranchCommits(ctx context.Context, repo string, branch string) ([]models.Contributor, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format, expected owner/repo: %s", repo)
	}
	owner, name := parts[0], parts[1]

	params := url.Values{}
	params.Set("sha", branch)
	params.Set("per_page", "100")
	u := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?%s", owner, name, params.Encode())

	body, err := c.doGet(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("github commits API failed for branch %s: %w", branch, err)
	}

	var ghCommits []struct {
		Author struct {
			ID    uint64 `json:"id"`
			Login string `json:"login"`
		} `json:"author"`
	}
	if err := json.Unmarshal(body, &ghCommits); err != nil {
		return nil, fmt.Errorf("parse commits: %w", err)
	}

	type authorKey struct {
		id    uint64
		login string
	}
	counts := make(map[authorKey]int)
	for _, cm := range ghCommits {
		if cm.Author.ID == 0 || cm.Author.Login == "" {
			continue
		}
		counts[authorKey{id: cm.Author.ID, login: cm.Author.Login}]++
	}

	if len(counts) == 0 {
		return nil, fmt.Errorf("no valid commits with real GitHub author identity on branch %s", branch)
	}

	result := make([]models.Contributor, 0, len(counts))
	for k, count := range counts {
		result = append(result, models.Contributor{
			GithubUserID: k.id,
			Username:     k.login,
			Commits:      count,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Commits > result[j].Commits
	})

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

type UserSearchResult struct {
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

func (c *Client) SearchUsers(ctx context.Context, query string) ([]UserSearchResult, error) {
	if len(query) < 3 {
		return nil, nil
	}

	url := "https://api.github.com/search/users?q=" + url.QueryEscape(query) + "&per_page=10"
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
		return nil, fmt.Errorf("search users request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search users returned %d", resp.StatusCode)
	}

	var payload struct {
		Items []struct {
			Login     string `json:"login"`
			AvatarURL string `json:"avatar_url"`
		} `json:"items"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("parse search users: %w", err)
	}

	results := make([]UserSearchResult, len(payload.Items))
	for i, item := range payload.Items {
		results[i] = UserSearchResult{
			Login:     item.Login,
			AvatarURL: item.AvatarURL,
		}
	}

	return results, nil
}

type RepoSearchResult struct {
	Name  string `json:"name"`
	Owner string `json:"owner"`
}

func (c *Client) SearchRepositories(ctx context.Context, owner, query string) ([]RepoSearchResult, error) {
	if len(owner) < 2 {
		return nil, nil
	}

	if len(query) == 0 {
		return c.ListUserRepos(ctx, owner)
	}

	url := "https://api.github.com/search/repositories?q=" + url.QueryEscape(owner+"/"+query) + "&per_page=10"
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
		return nil, fmt.Errorf("search repositories request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search repositories returned %d", resp.StatusCode)
	}

	var payload struct {
		Items []struct {
			Name  string `json:"name"`
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"items"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("parse search repositories: %w", err)
	}

	results := make([]RepoSearchResult, len(payload.Items))
	for i, item := range payload.Items {
		results[i] = RepoSearchResult{
			Name:  item.Name,
			Owner: item.Owner.Login,
		}
	}

	return results, nil
}

func (c *Client) ListUserRepos(ctx context.Context, owner string) ([]RepoSearchResult, error) {
	url := "https://api.github.com/users/" + url.QueryEscape(owner) + "/repos?type=public&sort=updated&per_page=30"
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
		return nil, fmt.Errorf("list user repos request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list user repos returned %d", resp.StatusCode)
	}

	var payload []struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("parse user repos: %w", err)
	}

	results := make([]RepoSearchResult, len(payload))
	for i, item := range payload {
		results[i] = RepoSearchResult{
			Name:  item.Name,
			Owner: item.Owner.Login,
		}
	}

	return results, nil
}
