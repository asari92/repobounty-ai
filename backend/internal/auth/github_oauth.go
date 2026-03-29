package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/repobounty/repobounty-ai/internal/config"
)

type GitHubOAuth struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
}

func NewGitHubOAuth(cfg *config.Config) *GitHubOAuth {
	return &GitHubOAuth{
		clientID:     cfg.GitHubClientID,
		clientSecret: cfg.GitHubClientSecret,
		redirectURL:  cfg.FrontendURL + "/auth/callback",
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (g *GitHubOAuth) GetAuthURL(state string) string {
	u, _ := url.Parse("https://github.com/login/oauth/authorize")
	q := u.Query()
	q.Set("client_id", g.clientID)
	q.Set("redirect_uri", g.redirectURL)
	q.Set("scope", "read:user,user:email")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String()
}

type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type GitHubUser struct {
	Login     string `json:"login"`
	ID        int    `json:"id"`
	AvatarURL string `json:"avatar_url"`
	Email     string `json:"email"`
}

func (g *GitHubOAuth) ExchangeCode(ctx context.Context, code string) (*GitHubUser, string, error) {
	tokenURL := "https://github.com/login/oauth/access_token"
	data := url.Values{}
	data.Set("client_id", g.clientID)
	data.Set("client_secret", g.clientSecret)
	data.Set("code", code)
	data.Set("redirect_uri", g.redirectURL)

	req, _ := http.NewRequestWithContext(ctx, "POST", tokenURL, nil)
	req.URL.RawQuery = data.Encode()
	req.Header.Set("Accept", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("github token exchange failed: %d", resp.StatusCode)
	}

	var tokenResp GitHubTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, "", fmt.Errorf("decode token response: %w", err)
	}

	userReq, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, "", fmt.Errorf("build user request: %w", err)
	}
	userReq.Header.Set("Authorization", "Bearer "+tokenResp.AccessToken)
	userReq.Header.Set("Accept", "application/vnd.github+json")

	userResp, err := g.httpClient.Do(userReq)
	if err != nil {
		return nil, "", fmt.Errorf("fetch user: %w", err)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("github user fetch failed: %d", userResp.StatusCode)
	}

	var user GitHubUser
	if err := json.NewDecoder(userResp.Body).Decode(&user); err != nil {
		return nil, "", fmt.Errorf("decode user response: %w", err)
	}

	return &user, tokenResp.AccessToken, nil
}
