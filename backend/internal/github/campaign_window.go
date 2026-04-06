package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/repobounty/repobounty-ai/internal/models"
)

type ContributionWindowData struct {
	Contributors       []models.Contributor
	ContributorPRDiffs map[string][]string
	WindowStart        time.Time
	WindowEnd          time.Time
	ContributorSource  string
	ContributorNotes   string
}

// FetchContributionWindowData returns contributor data for allocation.
// In MVP/demo mode we analyze the full repository history.
// In production we fall back to campaign-window-only analysis.
func (c *Client) FetchContributionWindowData(
	ctx context.Context,
	repo string,
	windowStart time.Time,
	windowEnd time.Time,
) (*ContributionWindowData, error) {
	if !c.isProduction {
		contributors, err := c.FetchContributors(ctx, repo)
		if err != nil {
			return nil, err
		}

		contributorPRDiffs, err := c.FetchContributorsPRDiffs(ctx, repo, 0)
		if err != nil {
			return nil, err
		}

		return &ContributionWindowData{
			Contributors:       contributors,
			ContributorPRDiffs: contributorPRDiffs,
			WindowStart:        windowStart.UTC(),
			WindowEnd:          windowEnd.UTC(),
			ContributorSource:  "repository_history_mvp",
			ContributorNotes:   "MVP/demo mode analyzes the full available repository history. A future production flow will restrict analysis to activity inside the campaign window.",
		}, nil
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format, expected owner/repo: %s", repo)
	}
	owner, name := parts[0], parts[1]

	prs, err := c.fetchMergedPRDetailsInWindow(ctx, owner, name, windowStart, windowEnd)
	if err != nil {
		if c.isProduction {
			return nil, fmt.Errorf("github API failed and mock data is disabled in production: %w", err)
		}
		log.Printf("github: contribution window fetch failed (%v), using mock data", err)
		return mockContributionWindowData(repo, windowStart, windowEnd), nil
	}
	if len(prs) == 0 {
		if c.isProduction {
			return nil, fmt.Errorf("no merged PRs found inside the campaign window")
		}
		log.Printf("github: no merged PRs found in campaign window, using mock data")
		return mockContributionWindowData(repo, windowStart, windowEnd), nil
	}

	contributorMap := make(map[string]*models.Contributor)
	for _, pr := range prs {
		if pr.User.Login == "" {
			continue
		}
		contributor, ok := contributorMap[pr.User.Login]
		if !ok {
			contributor = &models.Contributor{Username: pr.User.Login}
			contributorMap[pr.User.Login] = contributor
		}
		contributor.PullRequests++
		contributor.Commits += pr.Commits
		contributor.LinesAdded += pr.Additions
		contributor.LinesDeleted += pr.Deletions
	}

	contributors := make([]models.Contributor, 0, len(contributorMap))
	for _, contributor := range contributorMap {
		contributors = append(contributors, *contributor)
	}
	sort.Slice(contributors, func(i, j int) bool {
		if contributors[i].PullRequests == contributors[j].PullRequests {
			return contributors[i].Username < contributors[j].Username
		}
		return contributors[i].PullRequests > contributors[j].PullRequests
	})

	contributorPRDiffs := c.fetchPRDiffsForWindow(ctx, repo, prs)

	return &ContributionWindowData{
		Contributors:       contributors,
		ContributorPRDiffs: contributorPRDiffs,
		WindowStart:        windowStart.UTC(),
		WindowEnd:          windowEnd.UTC(),
		ContributorSource:  "merged_pr_window",
		ContributorNotes:   "Contributor metrics are approximated from merged PRs inside the campaign window. Direct pushes and review-only activity outside merged PRs are not captured by the current GitHub API flow.",
	}, nil
}

func (c *Client) fetchMergedPRDetailsInWindow(
	ctx context.Context,
	owner string,
	repo string,
	windowStart time.Time,
	windowEnd time.Time,
) ([]ghPullRequestDetailed, error) {
	page := 1
	perPage := 100
	var matched []ghPullRequestDetailed

	for {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=closed&per_page=%d&page=%d", owner, repo, perPage, page)
		body, err := c.doGet(ctx, url)
		if err != nil {
			return nil, err
		}

		var listedPRs []ghPullRequestDetailed
		if err := json.Unmarshal(body, &listedPRs); err != nil {
			return nil, fmt.Errorf("parse PR list: %w", err)
		}
		if len(listedPRs) == 0 {
			break
		}

		for _, listedPR := range listedPRs {
			if !listedPR.Merged {
				continue
			}
			if listedPR.MergedAt.Before(windowStart) || listedPR.MergedAt.After(windowEnd) {
				continue
			}

			detailedPR, err := c.fetchPullRequestDetail(ctx, owner, repo, listedPR.Number)
			if err != nil {
				log.Printf("github: failed to fetch PR #%d details: %v", listedPR.Number, err)
				continue
			}
			if detailedPR.MergedAt.Before(windowStart) || detailedPR.MergedAt.After(windowEnd) {
				continue
			}
			matched = append(matched, detailedPR)
		}

		if len(listedPRs) < perPage {
			break
		}
		page++
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].MergedAt.Equal(matched[j].MergedAt) {
			return matched[i].Number < matched[j].Number
		}
		return matched[i].MergedAt.Before(matched[j].MergedAt)
	})

	return matched, nil
}

func (c *Client) fetchPullRequestDetail(
	ctx context.Context,
	owner string,
	repo string,
	number int,
) (ghPullRequestDetailed, error) {
	var pr ghPullRequestDetailed
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", owner, repo, number)
	body, err := c.doGet(ctx, url)
	if err != nil {
		return pr, err
	}
	if err := json.Unmarshal(body, &pr); err != nil {
		return pr, fmt.Errorf("parse PR detail: %w", err)
	}
	return pr, nil
}

func (c *Client) fetchPRDiffsForWindow(
	ctx context.Context,
	repo string,
	prs []ghPullRequestDetailed,
) map[string][]string {
	type diffJob struct {
		contributor string
		prNumber    int
	}

	jobs := make([]diffJob, 0, len(prs))
	for _, pr := range prs {
		if pr.User.Login == "" {
			continue
		}
		jobs = append(jobs, diffJob{
			contributor: pr.User.Login,
			prNumber:    pr.Number,
		})
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].prNumber < jobs[j].prNumber
	})

	result := make(map[string][]string)
	var mu sync.Mutex
	sem := make(chan struct{}, maxParallelDiffFetches)
	var wg sync.WaitGroup

	for _, job := range jobs {
		wg.Add(1)
		go func(job diffJob) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			diff, err := c.FetchPRDiff(ctx, repo, job.prNumber)
			if err != nil {
				log.Printf("github: failed to fetch diff for PR #%d: %v", job.prNumber, err)
				return
			}

			truncated := truncateDiff(diff, maxDiffLinesPerFile)
			if truncated == "" {
				return
			}

			mu.Lock()
			result[job.contributor] = append(result[job.contributor], truncated)
			mu.Unlock()
		}(job)
	}

	wg.Wait()

	return result
}

func mockContributionWindowData(repo string, windowStart time.Time, windowEnd time.Time) *ContributionWindowData {
	contributors := mockContributors()
	prs := mockPRs(repo)
	contributorPRDiffs := make(map[string][]string)
	for _, pr := range prs {
		contributorPRDiffs[pr.User] = append(contributorPRDiffs[pr.User], mockPRDiff(pr.Number))
	}
	return &ContributionWindowData{
		Contributors:       contributors,
		ContributorPRDiffs: contributorPRDiffs,
		WindowStart:        windowStart.UTC(),
		WindowEnd:          windowEnd.UTC(),
		ContributorSource:  "merged_pr_window",
		ContributorNotes:   "Contributor metrics are approximated from merged PRs inside the campaign window. Direct pushes and review-only activity outside merged PRs are not captured by the current GitHub API flow.",
	}
}
