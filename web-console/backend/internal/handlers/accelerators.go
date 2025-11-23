package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"go.uber.org/zap"
)

// AcceleratorsHandler handles HTTP requests for AcceleratorClass resources
type AcceleratorsHandler struct {
	k8sClient *k8s.Client
	logger    *zap.Logger
}

// NewAcceleratorsHandler creates a new AcceleratorsHandler
func NewAcceleratorsHandler(k8sClient *k8s.Client, logger *zap.Logger) *AcceleratorsHandler {
	return &AcceleratorsHandler{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// List handles GET /api/v1/accelerators
func (h *AcceleratorsHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	accelerators, err := h.k8sClient.ListAcceleratorClasses(ctx)
	if err != nil {
		h.logger.Error("Failed to list accelerators", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list accelerators",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": accelerators.Items,
		"total": len(accelerators.Items),
	})
}

// Get handles GET /api/v1/accelerators/:name
func (h *AcceleratorsHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	accelerator, err := h.k8sClient.GetAcceleratorClass(ctx, name)
	if err != nil {
		h.logger.Error("Failed to get accelerator", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Accelerator not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, accelerator.Object)
}
