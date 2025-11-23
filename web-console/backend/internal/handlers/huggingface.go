package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/pkg/huggingface"
	"go.uber.org/zap"
)

// HuggingFaceHandler handles HTTP requests for HuggingFace API integration
type HuggingFaceHandler struct {
	hfClient *huggingface.Client
	logger   *zap.Logger
}

// NewHuggingFaceHandler creates a new HuggingFaceHandler
func NewHuggingFaceHandler(logger *zap.Logger) *HuggingFaceHandler {
	return &HuggingFaceHandler{
		hfClient: huggingface.NewClient(),
		logger:   logger,
	}
}

// SearchModels handles GET /api/v1/huggingface/models/search
func (h *HuggingFaceHandler) SearchModels(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	params := huggingface.SearchModelsParams{
		Query:     c.Query("q"),
		Author:    c.Query("author"),
		Filter:    c.Query("filter"),
		Sort:      c.DefaultQuery("sort", "downloads"),
		Direction: c.DefaultQuery("direction", "desc"),
		Limit:     20,
	}

	// Parse limit
	if limitStr := c.Query("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			params.Limit = limit
		}
	}

	// Parse tags (can have multiple)
	if tags := c.QueryArray("tags"); len(tags) > 0 {
		params.Tags = tags
	}

	results, err := h.hfClient.SearchModels(ctx, params)
	if err != nil {
		h.logger.Error("Failed to search HuggingFace models",
			zap.String("query", params.Query),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to search models",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": results,
		"total": len(results),
	})
}

// GetModelInfo handles GET /api/v1/huggingface/models/:modelId/info
func (h *HuggingFaceHandler) GetModelInfo(c *gin.Context) {
	ctx := c.Request.Context()

	// ModelID is passed as a path parameter, may contain slashes (e.g., "facebook/opt-125m")
	// We need to get the full path after /models/
	modelID := c.Param("modelId")

	// Check if there's a second part (for models like "facebook/opt-125m")
	if modelName := c.Param("modelName"); modelName != "" {
		modelID = modelID + "/" + modelName
	}

	info, err := h.hfClient.GetModelInfo(ctx, modelID)
	if err != nil {
		h.logger.Error("Failed to get HuggingFace model info",
			zap.String("modelId", modelID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Model not found",
			"details": err.Error(),
		})
		return
	}

	// Detect model format from siblings
	if len(info.Siblings) > 0 {
		format := huggingface.DetectModelFormat(info.Siblings)
		size := huggingface.EstimateModelSize(info.Siblings)

		c.JSON(http.StatusOK, gin.H{
			"model":          info,
			"detectedFormat": format,
			"estimatedSize":  size,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"model": info,
	})
}

// GetModelConfig handles GET /api/v1/huggingface/models/:modelId/config
func (h *HuggingFaceHandler) GetModelConfig(c *gin.Context) {
	ctx := c.Request.Context()

	// ModelID is passed as a path parameter, may contain slashes
	modelID := c.Param("modelId")

	// Check if there's a second part (for models like "facebook/opt-125m")
	if modelName := c.Param("modelName"); modelName != "" {
		modelID = modelID + "/" + modelName
	}

	config, err := h.hfClient.GetModelConfig(ctx, modelID)
	if err != nil {
		h.logger.Error("Failed to get HuggingFace model config",
			zap.String("modelId", modelID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Model config not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": config,
	})
}
