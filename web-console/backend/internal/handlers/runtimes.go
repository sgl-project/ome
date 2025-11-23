package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RuntimesHandler handles HTTP requests for ClusterServingRuntime resources
type RuntimesHandler struct {
	k8sClient *k8s.Client
	logger    *zap.Logger
}

// NewRuntimesHandler creates a new RuntimesHandler
func NewRuntimesHandler(k8sClient *k8s.Client, logger *zap.Logger) *RuntimesHandler {
	return &RuntimesHandler{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// List handles GET /api/v1/runtimes
func (h *RuntimesHandler) List(c *gin.Context) {
	ctx := c.Request.Context()

	runtimes, err := h.k8sClient.ListClusterServingRuntimes(ctx)
	if err != nil {
		h.logger.Error("Failed to list runtimes", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list runtimes",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": runtimes.Items,
		"total": len(runtimes.Items),
	})
}

// Get handles GET /api/v1/runtimes/:name
func (h *RuntimesHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	runtime, err := h.k8sClient.GetClusterServingRuntime(ctx, name)
	if err != nil {
		h.logger.Error("Failed to get runtime", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Runtime not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, runtime.Object)
}

// Create handles POST /api/v1/runtimes
func (h *RuntimesHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var runtimeData map[string]interface{}
	if err := c.ShouldBindJSON(&runtimeData); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Create unstructured object
	runtime := &unstructured.Unstructured{Object: runtimeData}

	// Set GVK if not present
	if runtime.GetAPIVersion() == "" {
		runtime.SetAPIVersion("ome.io/v1beta1")
	}
	if runtime.GetKind() == "" {
		runtime.SetKind("ClusterServingRuntime")
	}

	created, err := h.k8sClient.CreateClusterServingRuntime(ctx, runtime)
	if err != nil {
		h.logger.Error("Failed to create runtime", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create runtime",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Runtime created successfully", zap.String("name", created.GetName()))
	c.JSON(http.StatusCreated, created.Object)
}

// Update handles PUT /api/v1/runtimes/:name
func (h *RuntimesHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	var runtimeData map[string]interface{}
	if err := c.ShouldBindJSON(&runtimeData); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Create unstructured object
	runtime := &unstructured.Unstructured{Object: runtimeData}

	// Ensure name matches
	runtime.SetName(name)

	// Set GVK if not present
	if runtime.GetAPIVersion() == "" {
		runtime.SetAPIVersion("ome.io/v1beta1")
	}
	if runtime.GetKind() == "" {
		runtime.SetKind("ClusterServingRuntime")
	}

	updated, err := h.k8sClient.UpdateClusterServingRuntime(ctx, runtime)
	if err != nil {
		h.logger.Error("Failed to update runtime", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update runtime",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Runtime updated successfully", zap.String("name", name))
	c.JSON(http.StatusOK, updated.Object)
}

// Delete handles DELETE /api/v1/runtimes/:name
func (h *RuntimesHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	err := h.k8sClient.DeleteClusterServingRuntime(ctx, name)
	if err != nil {
		h.logger.Error("Failed to delete runtime", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete runtime",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Runtime deleted successfully", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{
		"message": "Runtime deleted successfully",
		"name": name,
	})
}
