package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/repobounty/repobounty-ai/internal/models"
)

const (
	openrouterBaseURL = "https://openrouter.ai/api/v1"
)

type Allocator struct {
	client *openai.Client
	model  string
	apiKey string
}

func NewAllocator(apiKey, model string) *Allocator {
	a := &Allocator{model: model, apiKey: apiKey}
	if apiKey != "" {
		config := openai.DefaultConfig(apiKey)
		config.BaseURL = openrouterBaseURL
		a.client = openai.NewClientWithConfig(config)
	}
	return a
}

func (a *Allocator) Model() string {
	if a.apiKey == "" {
		return "deterministic-fallback"
	}
	return a.model
}

type CodeEvaluation struct {
	Contributor     string `json:"contributor"`
	ImpactScore     int    `json:"impact_score"`
	ComplexityScore int    `json:"complexity_score"`
	ScopeScore      int    `json:"scope_score"`
	QualityScore    int    `json:"quality_score"`
	CommunityScore  int    `json:"community_score"`
	TotalScore      int    `json:"total_score"`
	Reasoning       string `json:"reasoning"`
}

func (a *Allocator) EvaluateCodeImpact(ctx context.Context, repo string, contributorPRs map[string][]string, poolAmount uint64) ([]models.Allocation, error) {
	if len(contributorPRs) == 0 {
		return nil, fmt.Errorf("no PRs to evaluate")
	}

	if a.client != nil {
		allocs, err := a.evaluateByDiffWithAI(ctx, repo, contributorPRs, poolAmount)
		if err != nil {
			log.Printf("ai: LLM evaluation failed (%v), using deterministic fallback", err)
			return deterministicEvaluate(contributorPRs, poolAmount), nil
		}
		return allocs, nil
	}

	log.Printf("ai: no API key configured, using deterministic fallback")
	return deterministicEvaluate(contributorPRs, poolAmount), nil
}

type PRSummary struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

func (a *Allocator) evaluateByDiffWithAI(ctx context.Context, repo string, contributorPRs map[string][]string, poolAmount uint64) ([]models.Allocation, error) {
	contributors := make([]string, 0, len(contributorPRs))
	prData := make(map[string][]PRSummary)

	for contributor, diffs := range contributorPRs {
		contributors = append(contributors, contributor)
		prData[contributor] = parseDiffSummaries(diffs)
	}

	limitPRs := 5
	for contributor, summaries := range prData {
		if len(summaries) > limitPRs {
			sort.Slice(summaries, func(i, j int) bool {
				return len(summaries[i].Description) > len(summaries[j].Description)
			})
			prData[contributor] = summaries[:limitPRs]
		}
	}

	prDataJSON, _ := json.Marshal(prData)

	systemPrompt := `You are a code impact evaluator for the RepoBounty AI platform.
Analyze actual code diffs and allocate reward percentages (in basis points, where 10000 = 100%).
Evaluate based on five dimensions (0-10 scale for each):
1. Impact: How much the code affects the system (critical bugs vs minor refactors)
2. Complexity: Technical difficulty of the implementation
3. Scope: Breadth of changes (files modified, lines changed)
4. Quality: Code quality, test coverage, documentation
5. Community: Review participation, collaboration

The total score should reflect the overall contribution value.
You MUST return ONLY a valid JSON array with no extra text.`

	userPrompt := fmt.Sprintf(`Repository: %s
Total reward pool: %d lamports

Code diffs by contributor (top %d PRs per contributor shown):
%s

Evaluate contributions and allocate rewards as basis points (must sum to exactly 10000).
Return ONLY this JSON format:
[{"contributor": "username", "percentage": 5000, "reasoning": "detailed evaluation based on code quality and impact"}]`, repo, poolAmount, limitPRs, string(prDataJSON))

	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.4,
	})
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}

	content := resp.Choices[0].Message.Content

	var aiAllocs []aiAllocation
	if err := json.Unmarshal([]byte(content), &aiAllocs); err != nil {
		return nil, fmt.Errorf("parse AI response: %w (raw: %s)", err, string(content))
	}

	var totalBps int
	seen := make(map[string]bool)
	for _, a := range aiAllocs {
		if a.Percentage <= 0 || a.Percentage > 10000 {
			return nil, fmt.Errorf("invalid allocation percentage %d for %s, must be > 0 and <= 10000", a.Percentage, a.Contributor)
		}
		if seen[a.Contributor] {
			return nil, fmt.Errorf("duplicate contributor %s in allocations", a.Contributor)
		}
		seen[a.Contributor] = true
		totalBps += a.Percentage
	}
	if totalBps != 10000 {
		return nil, fmt.Errorf("AI allocation sums to %d bps, expected 10000", totalBps)
	}

	if len(aiAllocs) > 10 {
		return nil, fmt.Errorf("max 10 allocations allowed")
	}

	allocs := make([]models.Allocation, len(aiAllocs))
	for i, a := range aiAllocs {
		allocs[i] = models.Allocation{
			Contributor: a.Contributor,
			Percentage:  uint16(a.Percentage),
			Amount:      poolAmount * uint64(a.Percentage) / 10000,
			Reasoning:   a.Reasoning,
		}
	}
	return allocs, nil
}

func parseDiffSummaries(diffs []string) []PRSummary {
	summaries := make([]PRSummary, len(diffs))
	for i, diff := range diffs {
		lines := strings.Split(diff, "\n")
		summary := PRSummary{
			Description: diff,
		}

		for _, line := range lines {
			if strings.HasPrefix(line, "diff --git a/") || strings.HasPrefix(line, "+++ b/") {
				if strings.HasPrefix(line, "diff --git a/") {
					parts := strings.Fields(line)
					if len(parts) >= 4 {
						summary.Title = parts[4]
					}
				}
			}
		}

		if summary.Title == "" {
			summary.Title = fmt.Sprintf("PR #100%d", i+1)
		}
		summary.Number = 10000 + i

		summaries[i] = summary
	}
	return summaries
}

func deterministicEvaluate(contributorPRs map[string][]string, poolAmount uint64) []models.Allocation {
	type weighted struct {
		contributor string
		weight      int
		prCount     int
		linesTotal  int
		filesTotal  int
	}

	entries := make([]weighted, 0, len(contributorPRs))
	totalWeight := 0

	for contributor, diffs := range contributorPRs {
		linesTotal := 0
		filesTotal := 0

		for _, diff := range diffs {
			linesInDiff := strings.Count(diff, "\n")
			linesTotal += linesInDiff

			for _, line := range strings.Split(diff, "\n") {
				if strings.HasPrefix(line, "diff --git a/") || strings.HasPrefix(line, "+++ b/") {
					filesTotal++
				}
			}
		}

		weight := len(diffs)*100 + linesTotal + filesTotal*50
		if weight < 100 {
			weight = 100
		}

		entries = append(entries, weighted{
			contributor: contributor,
			weight:      weight,
			prCount:     len(diffs),
			linesTotal:  linesTotal,
			filesTotal:  filesTotal,
		})
		totalWeight += weight
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].weight > entries[j].weight
	})

	allocs := make([]models.Allocation, len(entries))
	var assignedBps uint16

	for i, e := range entries {
		bps := uint16(uint32(e.weight) * 10000 / uint32(totalWeight))
		if i == len(entries)-1 {
			bps = 10000 - assignedBps
		}
		assignedBps += bps

		reasoning := fmt.Sprintf("Deterministic allocation: %d PRs, %d lines of code, %d files changed",
			e.prCount, e.linesTotal, e.filesTotal)

		allocs[i] = models.Allocation{
			Contributor: e.contributor,
			Percentage:  bps,
			Amount:      poolAmount * uint64(bps) / 10000,
			Reasoning:   reasoning,
		}
	}
	return allocs
}

func (a *Allocator) Allocate(ctx context.Context, repo string, contributors []models.Contributor, poolAmount uint64) ([]models.Allocation, error) {
	if len(contributors) == 0 {
		return nil, fmt.Errorf("no contributors to allocate")
	}

	if a.client != nil {
		allocs, err := a.allocateWithAI(ctx, repo, contributors, poolAmount)
		if err != nil {
			log.Printf("ai: LLM allocation failed (%v), using deterministic fallback", err)
			return deterministicAllocate(contributors, poolAmount), nil
		}
		return allocs, nil
	}

	log.Printf("ai: no API key configured, using deterministic fallback")
	return deterministicAllocate(contributors, poolAmount), nil
}

type aiAllocation struct {
	Contributor string `json:"contributor"`
	Percentage  int    `json:"percentage"`
	Reasoning   string `json:"reasoning"`
}

func (a *Allocator) allocateWithAI(ctx context.Context, repo string, contributors []models.Contributor, poolAmount uint64) ([]models.Allocation, error) {
	contribJSON, _ := json.Marshal(contributors)

	systemPrompt := `You are a fair open-source contribution evaluator for the RepoBounty AI platform.
Analyze contributor metrics and allocate reward percentages (in basis points, where 10000 = 100%).
Consider: commits show consistent work, pull requests show feature contributions, reviews show community involvement, lines of code show scope of work.
You MUST return ONLY a valid JSON array with no extra text.`

	userPrompt := fmt.Sprintf(`Repository: %s
Total reward pool: %d lamports

Contributor metrics:
%s

Allocate rewards as basis points (must sum to exactly 10000).
Return ONLY this JSON format:
[{"contributor": "username", "percentage": 5000, "reasoning": "short reason"}]`, repo, poolAmount, string(contribJSON))

	resp, err := a.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned no choices")
	}

	content := resp.Choices[0].Message.Content

	var aiAllocs []aiAllocation
	if err := json.Unmarshal([]byte(content), &aiAllocs); err != nil {
		return nil, fmt.Errorf("parse AI response: %w (raw: %s)", err, content)
	}

	var totalBps int
	seen := make(map[string]bool)
	for _, a := range aiAllocs {
		if a.Percentage <= 0 || a.Percentage > 10000 {
			return nil, fmt.Errorf("invalid allocation percentage %d for %s, must be > 0 and <= 10000", a.Percentage, a.Contributor)
		}
		if seen[a.Contributor] {
			return nil, fmt.Errorf("duplicate contributor %s in allocations", a.Contributor)
		}
		seen[a.Contributor] = true
		totalBps += a.Percentage
	}
	if totalBps != 10000 {
		return nil, fmt.Errorf("AI allocation sums to %d bps, expected 10000", totalBps)
	}

	if len(aiAllocs) > 10 {
		return nil, fmt.Errorf("max 10 allocations allowed")
	}

	allocs := make([]models.Allocation, len(aiAllocs))
	for i, a := range aiAllocs {
		allocs[i] = models.Allocation{
			Contributor: a.Contributor,
			Percentage:  uint16(a.Percentage),
			Amount:      poolAmount * uint64(a.Percentage) / 10000,
			Reasoning:   a.Reasoning,
		}
	}
	return allocs, nil
}

func deterministicAllocate(contributors []models.Contributor, poolAmount uint64) []models.Allocation {
	type weighted struct {
		index  int
		weight int
	}

	entries := make([]weighted, len(contributors))
	totalWeight := 0
	for i, c := range contributors {
		w := c.Commits*3 + c.PullRequests*5 + c.Reviews*2
		if w < 1 {
			w = 1
		}
		entries[i] = weighted{index: i, weight: w}
		totalWeight += w
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].weight > entries[j].weight
	})

	allocs := make([]models.Allocation, len(contributors))
	var assignedBps uint16

	for i, e := range entries {
		bps := uint16(uint32(e.weight) * 10000 / uint32(totalWeight))
		if i == len(entries)-1 {
			bps = 10000 - assignedBps
		}
		assignedBps += bps

		allocs[i] = models.Allocation{
			Contributor: contributors[e.index].Username,
			Percentage:  bps,
			Amount:      poolAmount * uint64(bps) / 10000,
			Reasoning:   "Deterministic allocation based on contribution metrics",
		}
	}
	return allocs
}
