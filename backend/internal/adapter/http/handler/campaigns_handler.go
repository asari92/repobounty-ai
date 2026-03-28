package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/yourusername/repobounty-ai/internal/domain/types"
	"github.com/yourusername/repobounty-ai/internal/service"
)

type CampaignsHandler struct {
	service *service.CampaignService
}

func NewCampaignsHandler(service *service.CampaignService) *CampaignsHandler {
	return &CampaignsHandler{
		service: service,
	}
}

func (h *CampaignsHandler) CreateCampaign(c *gin.Context) {
	var req createCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 100*time.Second)
	defer cancel()

	campaign, err := h.service.CreateCampaign(ctx, req.RepoURL, req.RewardPool, req.Deadline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Campaign created successfully",
		"id":      campaign.ID,
	})
}

func (h *CampaignsHandler) GetCampaign(c *gin.Context) {
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 100*time.Second)
	defer cancel()

	campaign, err := h.service.GetCampaign(ctx, id)
	if err != nil {
		if errors.Is(err, types.ErrCampaignNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, campaign)
}

func (h *CampaignsHandler) FinalizeCampaign(c *gin.Context) {
	idStr := c.Param("id")

	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid campaign id"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 100*time.Second)
	defer cancel()

	if err := h.service.FinalizeCampaign(ctx, id); err != nil {
		if errors.Is(err, types.ErrCampaignNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		if errors.Is(err, types.ErrCapaignAlreadyFinalized) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Campaign finalized successfully"})
}
