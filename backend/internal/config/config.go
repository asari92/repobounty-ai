package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                        string
	GitHubToken                 string
	GitHubClientID              string
	GitHubClientSecret          string
	FrontendURL                 string
	JWTSecret                   string
	OpenRouterAPIKey            string
	Model                       string
	SolanaRPCURL                string
	ServicePrivateKey           string
	ProgramID                   string
	Env                         string
	AllowedOrigins              string
	GitHubAppID                 int64
	GitHubAppPrivateKey         string
	DatabasePath                string
	TreasuryWallet              string
	MinCampaignAmount           uint64
	MinAllocationAmount         uint64
	MaxAllocations              int
	MinDeadlineSeconds          int64
	FinalizeBatchSize           int
	AutoFinalizeIntervalSeconds int
}

func Load() (*Config, error) {
	if err := loadDotEnv(); err != nil {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	databasePath, err := resolveDatabasePath(envOrDefault("DATABASE_PATH", "repobounty.db"))
	if err != nil {
		return nil, fmt.Errorf("resolve DATABASE_PATH: %w", err)
	}

	cfg := &Config{
		Port:                        envOrDefault("PORT", "8080"),
		GitHubToken:                 os.Getenv("GITHUB_TOKEN"),
		GitHubClientID:              os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret:          os.Getenv("GITHUB_CLIENT_SECRET"),
		FrontendURL:                 envOrDefault("FRONTEND_URL", "http://localhost:3000"),
		JWTSecret:                   os.Getenv("JWT_SECRET"),
		OpenRouterAPIKey:            os.Getenv("OPENROUTER_API_KEY"),
		Model:                       envOrDefault("MODEL", "nvidia/nemotron-3-super-120b-a12b:free"),
		SolanaRPCURL:                envOrDefault("SOLANA_RPC_URL", "https://api.devnet.solana.com"),
		ServicePrivateKey:           os.Getenv("SERVICE_PRIVATE_KEY"),
		ProgramID:                   os.Getenv("PROGRAM_ID"),
		Env:                         envOrDefault("ENV", "development"),
		AllowedOrigins:              os.Getenv("ALLOWED_ORIGINS"),
		GitHubAppID:                 envOrDefaultInt64("GITHUB_APP_ID", 0),
		GitHubAppPrivateKey:         os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		DatabasePath:                databasePath,
		TreasuryWallet:              envOrDefault("TREASURY_WALLET", ""),
		MinCampaignAmount:           envOrDefaultUint64("MIN_CAMPAIGN_AMOUNT", 500_000_000),
		MinAllocationAmount:         envOrDefaultUint64("MIN_ALLOCATION_AMOUNT", 50_000_000),
		MaxAllocations:              envOrDefaultInt("MAX_ALLOCATIONS", 200),
		MinDeadlineSeconds:          envOrDefaultInt64("MIN_DEADLINE_SECONDS", 300),
		FinalizeBatchSize:           envOrDefaultInt("FINALIZE_BATCH_SIZE", 5),
		AutoFinalizeIntervalSeconds: envOrDefaultInt("AUTO_FINALIZE_INTERVAL_SECONDS", 60),
	}
	if cfg.Env == "production" && cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required in production")
	}
	if cfg.Env == "production" && cfg.JWTSecret != "" && len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters in production")
	}
	return cfg, nil
}

func loadDotEnv() error {
	for _, path := range dotEnvCandidates() {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := godotenv.Load(path); err != nil {
			return err
		}
	}
	return nil
}

func dotEnvCandidates() []string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	candidates := []string{filepath.Join(cwd, ".env")}
	if backendRoot, ok := findBackendRootFrom(cwd); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(backendRoot), ".env"))
	}
	return uniquePaths(candidates)
}

func resolveDatabasePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return "", nil
	case value == ":memory:", strings.HasPrefix(value, "file:"):
		return value, nil
	case filepath.IsAbs(value):
		return filepath.Clean(value), nil
	}

	if backendRoot, ok := findBackendRoot(); ok {
		return filepath.Join(backendRoot, value), nil
	}

	return filepath.Abs(value)
}

func findBackendRoot() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	return findBackendRootFrom(cwd)
}

func findBackendRootFrom(start string) (string, bool) {
	dir := filepath.Clean(start)
	for {
		if isFile(filepath.Join(dir, "go.mod")) {
			return dir, true
		}

		backendDir := filepath.Join(dir, "backend")
		if isFile(filepath.Join(backendDir, "go.mod")) {
			return backendDir, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
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

func envOrDefaultUint64(key string, fallback uint64) uint64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envOrDefaultInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
