package server

import (
	"github.com/gin-gonic/gin"
	"github.com/yourusername/repobounty-ai/internal/adapter/http/handler"
)

func SetupRoutes(r *gin.Engine,
	campaignsHandler *handler.CampaignsHandler,
) {
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })

	campaings := r.Group("/")
	{
		campaings.POST("/campaigns", campaignsHandler.CreateCampaign)
		campaings.GET("/campaigns/:id", campaignsHandler.GetCampaign)
		campaings.POST("/campaigns/:id/finalize", campaignsHandler.FinalizeCampaign)
	}
}
