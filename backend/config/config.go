package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Port         string `env:"PORT" envDefault:"8080"`
	GitHubToken  string `env:"GITHUB_TOKEN,required"`
	OpenAIAPIKey string `env:"OPENAI_API_KEY,required"`
	SolanaRPCURL string `env:"SOLANA_RPC_URL,required"`
	ProgramID    string `env:"PROGRAM_ID,required"`
}

func MustLoad() *Config {
	_ = godotenv.Load()

	cfg, err := env.ParseAs[Config]()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	return &cfg
}
