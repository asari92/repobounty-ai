package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	GitHubToken      string
	OpenAIAPIKey     string
	SolanaRPCURL     string
	SolanaPrivateKey string
	ProgramID        string
	Env              string
	AllowedOrigins   string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:             envOrDefault("PORT", "8080"),
		GitHubToken:      os.Getenv("GITHUB_TOKEN"),
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		SolanaRPCURL:     envOrDefault("SOLANA_RPC_URL", "https://api.devnet.solana.com"),
		SolanaPrivateKey: os.Getenv("SOLANA_PRIVATE_KEY"),
		ProgramID:        os.Getenv("PROGRAM_ID"),
		Env:              envOrDefault("ENV", "development"),
		AllowedOrigins:   os.Getenv("ALLOWED_ORIGINS"),
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
