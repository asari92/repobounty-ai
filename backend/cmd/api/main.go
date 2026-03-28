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

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/repobounty-ai/config"
	"github.com/yourusername/repobounty-ai/internal/adapter/http/handler"
	"github.com/yourusername/repobounty-ai/internal/adapter/http/server"
	"github.com/yourusername/repobounty-ai/internal/adapter/repository"
	"github.com/yourusername/repobounty-ai/internal/domain/models"
	"github.com/yourusername/repobounty-ai/internal/service"
)

func main() {
	cfg := config.MustLoad()

	fmt.Println(cfg)

	repo := &repository.CampaignRepositoryImpl{
		Campiagns: make(map[uuid.UUID]*models.Campaign),
	}

	githubSvc := &service.GitHubService{}
	aiSvc := &service.AIAllocationService{}
	solanaSvc := &service.MockSolanaService{}

	svc := service.NewCampaignService(repo, githubSvc, aiSvc, solanaSvc)
	handler := handler.NewCampaignsHandler(svc)
	r := gin.Default()

	server.SetupRoutes(r, handler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: r,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalln("Failed to start server", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalln("Failed to shutdown server", "error", err)
	}

	fmt.Println("RepoBounty AI API Server started")
}
