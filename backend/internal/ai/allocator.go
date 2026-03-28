package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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
	for _, a := range aiAllocs {
		totalBps += a.Percentage
	}
	if totalBps != 10000 {
		return nil, fmt.Errorf("AI allocation sums to %d bps, expected 10000", totalBps)
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

	allocs := make([]models.Allocation, len(contributors))
	var assignedBps uint16

	for i, e := range entries {
		bps := uint16(e.weight * 10000 / totalWeight)
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
