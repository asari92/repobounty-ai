package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                string
	GitHubToken         string
	GitHubClientID      string
	GitHubClientSecret  string
	FrontendURL         string
	JWTSecret           string
	OpenRouterAPIKey    string
	Model               string
	SolanaRPCURL        string
	SolanaPrivateKey    string
	ProgramID           string
	Env                 string
	AllowedOrigins      string
	GitHubAppID         int64
	GitHubAppPrivateKey string
	DatabasePath        string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:                envOrDefault("PORT", "8080"),
		GitHubToken:         os.Getenv("GITHUB_TOKEN"),
		GitHubClientID:      os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:  os.Getenv("GITHUB_CLIENT_SECRET"),
		FrontendURL:         envOrDefault("FRONTEND_URL", "http://localhost:3000"),
		JWTSecret:           os.Getenv("JWT_SECRET"),
		OpenRouterAPIKey:    os.Getenv("OPENROUTER_API_KEY"),
		Model:               envOrDefault("MODEL", "nvidia/nemotron-3-super-120b-a12b:free"),
		SolanaRPCURL:        envOrDefault("SOLANA_RPC_URL", "https://api.devnet.solana.com"),
		SolanaPrivateKey:    os.Getenv("SOLANA_PRIVATE_KEY"),
		ProgramID:           os.Getenv("PROGRAM_ID"),
		Env:                 envOrDefault("ENV", "development"),
		AllowedOrigins:      os.Getenv("ALLOWED_ORIGINS"),
		GitHubAppID:         envOrDefaultInt64("GITHUB_APP_ID", 0),
		GitHubAppPrivateKey: os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		DatabasePath:        envOrDefault("DATABASE_PATH", "repobounty.db"),
	}
	if cfg.Env == "production" && cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required in production")
	}
	if cfg.Env == "production" && cfg.JWTSecret != "" && len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrDefaultInt64(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}
