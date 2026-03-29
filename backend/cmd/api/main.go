package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/repobounty/repobounty-ai/internal/ai"
	"github.com/repobounty/repobounty-ai/internal/auth"
	"github.com/repobounty/repobounty-ai/internal/config"
	"github.com/repobounty/repobounty-ai/internal/github"
	handler "github.com/repobounty/repobounty-ai/internal/http"
	"github.com/repobounty/repobounty-ai/internal/solana"
	"github.com/repobounty/repobounty-ai/internal/store"
	"go.uber.org/zap"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config:", err)
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	handler.InitLogger(env)
	logger := handler.GetLogger()
	defer logger.Sync()

	logger.Info("starting RepoBounty AI API",
		zap.String("port", cfg.Port),
		zap.String("env", env),
	)

	var campaignStore store.CampaignStore
	if cfg.DatabasePath != "" {
		sqliteStore, err := store.NewSQLite(cfg.DatabasePath)
		if err != nil {
			logger.Fatal("failed to open sqlite database", zap.Error(err))
		}
		campaignStore = sqliteStore
		defer sqliteStore.Close()
		logger.Info("using SQLite storage", zap.String("path", cfg.DatabasePath))
	} else {
		campaignStore = store.New()
		logger.Info("using in-memory storage")
	}
	ghClient := github.NewClient(cfg.GitHubToken)
	aiAllocator := ai.NewAllocator(cfg.OpenRouterAPIKey, cfg.Model)

	solClient, err := solana.NewClient(cfg.SolanaRPCURL, cfg.SolanaPrivateKey, cfg.ProgramID)
	if err != nil {
		logger.Fatal("failed to init solana client", zap.Error(err))
	}

	jwtManager := auth.NewJWTManager(cfg.JWTSecret)
	githubOAuth := auth.NewGitHubOAuth(cfg)

	handlers := handler.NewHandlers(
		campaignStore,
		ghClient,
		solClient,
		aiAllocator,
		jwtManager,
		githubOAuth,
		cfg,
	)

	logger.Info("configuration loaded",
		zap.String("solana_rpc", cfg.SolanaRPCURL),
		zap.Bool("solana_configured", solClient.IsConfigured()),
		zap.Bool("github_token_set", cfg.GitHubToken != ""),
		zap.String("ai_model", aiAllocator.Model()),
	)

	router := handler.NewRouter(handlers, env)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handler.StartAutoFinalizeWorker(ctx, campaignStore, ghClient, aiAllocator, solClient, logger, 5*time.Minute)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("server stopped")
}
