package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ServicesHandler handles HTTP requests for InferenceService resources
type ServicesHandler struct {
	k8sClient *k8s.Client
	logger    *zap.Logger
}

// NewServicesHandler creates a new ServicesHandler
func NewServicesHandler(k8sClient *k8s.Client, logger *zap.Logger) *ServicesHandler {
	return &ServicesHandler{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// List handles GET /api/v1/services
func (h *ServicesHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Query("namespace") // Optional query parameter

	services, err := h.k8sClient.ListInferenceServices(ctx, namespace)
	if err != nil {
		h.logger.Error("Failed to list services", zap.String("namespace", namespace), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list services",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": services.Items,
		"total": len(services.Items),
	})
}

// Get handles GET /api/v1/services/:name
func (h *ServicesHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")
	namespace := c.Query("namespace")

	if namespace == "" {
		namespace = "default"
	}

	service, err := h.k8sClient.GetInferenceService(ctx, namespace, name)
	if err != nil {
		h.logger.Error("Failed to get service",
			zap.String("name", name),
			zap.String("namespace", namespace),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Service not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, service.Object)
}

// Create handles POST /api/v1/services
func (h *ServicesHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var serviceData map[string]interface{}
	if err := c.ShouldBindJSON(&serviceData); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Create unstructured object
	service := &unstructured.Unstructured{Object: serviceData}

	// Set GVK if not present
	if service.GetAPIVersion() == "" {
		service.SetAPIVersion("ome.io/v1beta1")
	}
	if service.GetKind() == "" {
		service.SetKind("InferenceService")
	}

	// Get namespace from metadata or use default
	namespace := service.GetNamespace()
	if namespace == "" {
		namespace = "default"
		service.SetNamespace(namespace)
	}

	created, err := h.k8sClient.CreateInferenceService(ctx, namespace, service)
	if err != nil {
		h.logger.Error("Failed to create service", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create service",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Service created successfully",
		zap.String("name", created.GetName()),
		zap.String("namespace", namespace))
	c.JSON(http.StatusCreated, created.Object)
}

// Update handles PUT /api/v1/services/:name
func (h *ServicesHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	var serviceData map[string]interface{}
	if err := c.ShouldBindJSON(&serviceData); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Create unstructured object
	service := &unstructured.Unstructured{Object: serviceData}

	// Ensure name matches
	service.SetName(name)

	// Set GVK if not present
	if service.GetAPIVersion() == "" {
		service.SetAPIVersion("ome.io/v1beta1")
	}
	if service.GetKind() == "" {
		service.SetKind("InferenceService")
	}

	// Get namespace from metadata or use default
	namespace := service.GetNamespace()
	if namespace == "" {
		namespace = "default"
		service.SetNamespace(namespace)
	}

	updated, err := h.k8sClient.UpdateInferenceService(ctx, namespace, service)
	if err != nil {
		h.logger.Error("Failed to update service",
			zap.String("name", name),
			zap.String("namespace", namespace),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update service",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Service updated successfully",
		zap.String("name", name),
		zap.String("namespace", namespace))
	c.JSON(http.StatusOK, updated.Object)
}

// Delete handles DELETE /api/v1/services/:name
func (h *ServicesHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")
	namespace := c.Query("namespace")

	if namespace == "" {
		namespace = "default"
	}

	err := h.k8sClient.DeleteInferenceService(ctx, namespace, name)
	if err != nil {
		h.logger.Error("Failed to delete service",
			zap.String("name", name),
			zap.String("namespace", namespace),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete service",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Service deleted successfully",
		zap.String("name", name),
		zap.String("namespace", namespace))
	c.JSON(http.StatusOK, gin.H{
		"message":   "Service deleted successfully",
		"name":      name,
		"namespace": namespace,
	})
}

// GetStatus handles GET /api/v1/services/:name/status
func (h *ServicesHandler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")
	namespace := c.Query("namespace")

	if namespace == "" {
		namespace = "default"
	}

	service, err := h.k8sClient.GetInferenceService(ctx, namespace, name)
	if err != nil {
		h.logger.Error("Failed to get service status",
			zap.String("name", name),
			zap.String("namespace", namespace),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Service not found",
			"details": err.Error(),
		})
		return
	}

	// Extract status from the service
	status, found, err := unstructured.NestedMap(service.Object, "status")
	if err != nil || !found {
		c.JSON(http.StatusOK, gin.H{
			"status": map[string]interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": status,
	})
}
