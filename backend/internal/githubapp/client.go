package githubapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Client struct {
	appID      int64
	privateKey []byte
	httpClient *http.Client
}

func NewClient(appID int64, privateKey string) *Client {
	if appID == 0 || privateKey == "" {
		return nil
	}

	return &Client{
		appID:      appID,
		privateKey: []byte(privateKey),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) IsConfigured() bool {
	return c != nil
}

func (c *Client) generateJWT() (string, error) {
	now := time.Now()

	key, err := jwt.ParseRSAPrivateKeyFromPEM(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Unix(),
		"exp": now.Add(10 * time.Minute).Unix(),
		"iss": c.appID,
	})

	return token.SignedString(key)
}

type installationResponse struct {
	ID        int64 `json:"id"`
	AccountID int64 `json:"account_id"`
}

type installationsResponse struct {
	TotalCount    int                    `json:"total_count"`
	Installations []installationResponse `json:"installations"`
}

func (c *Client) GetInstallationToken(ctx context.Context, installationID int64) (string, error) {
	jwtToken, err := c.generateJWT()
	if err != nil {
		return "", fmt.Errorf("generate JWT: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request installation token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("get installation token: %d %s", resp.StatusCode, string(body))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	return result.Token, nil
}

func (c *Client) GetAppInstallation(ctx context.Context, repo string) (*installationResponse, error) {
	jwtToken, err := c.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("generate JWT: %w", err)
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format: %s", repo)
	}
	owner := parts[0]

	url := fmt.Sprintf("https://api.github.com/app/installations?per_page=100")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request installations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list installations: %d", resp.StatusCode)
	}

	var result installationsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode installations: %w", err)
	}

	for _, inst := range result.Installations {
		if inst.AccountID == 0 {
			continue
		}
		accountLogin, err := c.getAccountLogin(ctx, jwtToken, inst.AccountID)
		if err != nil {
			continue
		}
		if accountLogin == owner {
			return &inst, nil
		}
	}

	return nil, nil
}

func (c *Client) getAccountLogin(ctx context.Context, jwtToken string, accountID int64) (string, error) {
	url := fmt.Sprintf("https://api.github.com/user/%d", accountID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get account: %d", resp.StatusCode)
	}

	var result struct {
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Login, nil
}

type pullRequestResponse struct {
	Number int `json:"number"`
	User   struct {
		Login string `json:"login"`
	} `json:"user"`
	Merged bool `json:"merged"`
}

func (c *Client) FindContributorPR(ctx context.Context, installToken, repo, contributor string) (int, error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid repo format: %s", repo)
	}
	owner, name := parts[0], parts[1]

	page := 1
	for {
		url := fmt.Sprintf(
			"https://api.github.com/repos/%s/%s/pulls?state=closed&per_page=100&page=%d",
			owner, name, page,
		)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("Authorization", "Bearer "+installToken)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return 0, err
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return 0, err
		}

		if resp.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("list PRs: %d %s", resp.StatusCode, string(body))
		}

		var prs []pullRequestResponse
		if err := json.Unmarshal(body, &prs); err != nil {
			return 0, fmt.Errorf("decode PRs: %w", err)
		}

		if len(prs) == 0 {
			break
		}

		for _, pr := range prs {
			if pr.Merged && pr.User.Login == contributor {
				return pr.Number, nil
			}
		}

		if len(prs) < 100 {
			break
		}
		page++
	}

	return 0, fmt.Errorf("no merged PR found for %s", contributor)
}

type CommentBody struct {
	Contributor string
	AmountSOL   string
	Percentage  uint16
	ClaimURL    string
	CampaignID  string
	Repo        string
}

func (c *Client) PostAllocationComment(
	ctx context.Context,
	installToken string,
	repo string,
	prNumber int,
	body *CommentBody,
) error {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid repo format: %s", repo)
	}
	owner, name := parts[0], parts[1]

	comment := fmt.Sprintf(
		"🎉 **@%s**, you earned **%s SOL** (%s%%) for your contributions to `%s`!\n\n"+
			"RepoBounty AI analyzed your code impact and allocated this reward as part of campaign `%s`.\n\n"+
			"→ Claim your reward: %s",
		body.Contributor,
		body.AmountSOL,
		strconv.FormatFloat(float64(body.Percentage)/100, 'f', 1, 64),
		body.Repo,
		body.CampaignID,
		body.ClaimURL,
	)

	commentPayload := struct {
		Body string `json:"body"`
	}{Body: comment}

	payload, err := json.Marshal(commentPayload)
	if err != nil {
		return fmt.Errorf("marshal comment: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, name, prNumber)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+installToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("post comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("post comment: %d %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func PostAllocationComments(
	ctx context.Context,
	client *Client,
	repo string,
	campaignID string,
	allocations []Allocation,
	frontendURL string,
) {
	if !client.IsConfigured() {
		return
	}

	installation, err := client.GetAppInstallation(ctx, repo)
	if err != nil {
		log.Printf("githubapp: failed to find installation for %s: %v", repo, err)
		return
	}
	if installation == nil {
		log.Printf("githubapp: app not installed on %s, skipping PR comments", repo)
		return
	}

	installToken, err := client.GetInstallationToken(ctx, installation.ID)
	if err != nil {
		log.Printf("githubapp: failed to get installation token: %v", err)
		return
	}

	for _, alloc := range allocations {
		if alloc.Claimed {
			continue
		}

		prNumber, err := client.FindContributorPR(ctx, installToken, repo, alloc.Contributor)
		if err != nil {
			log.Printf("githubapp: no merged PR found for %s in %s: %v", alloc.Contributor, repo, err)
			continue
		}

		amountSOL := fmt.Sprintf("%.4f", float64(alloc.Amount)/1e9)

		err = client.PostAllocationComment(ctx, installToken, repo, prNumber, &CommentBody{
			Contributor: alloc.Contributor,
			AmountSOL:   amountSOL,
			Percentage:  alloc.Percentage,
			ClaimURL:    fmt.Sprintf("%s/campaign/%s", frontendURL, campaignID),
			CampaignID:  campaignID,
			Repo:        repo,
		})
		if err != nil {
			log.Printf("githubapp: failed to post comment on PR #%d for %s: %v", prNumber, alloc.Contributor, err)
			continue
		}

		log.Printf("githubapp: posted allocation comment on PR #%d for @%s", prNumber, alloc.Contributor)
		time.Sleep(500 * time.Millisecond)
	}
}

type Allocation struct {
	Contributor string
	Percentage  uint16
	Amount      uint64
	Claimed     bool
}
