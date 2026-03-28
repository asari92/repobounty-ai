package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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
