package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ModelsHandler handles HTTP requests for ClusterBaseModel resources
type ModelsHandler struct {
	k8sClient *k8s.Client
	logger    *zap.Logger
}

// NewModelsHandler creates a new ModelsHandler
func NewModelsHandler(k8sClient *k8s.Client, logger *zap.Logger) *ModelsHandler {
	return &ModelsHandler{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// List handles GET /api/v1/models
// Supports optional ?namespace= query parameter
// - No namespace: returns ClusterBaseModel resources
// - namespace=<name>: returns BaseModel resources from that namespace
func (h *ModelsHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Query("namespace")

	// If namespace is specified, list namespace-scoped BaseModels
	if namespace != "" {
		models, err := h.k8sClient.ListBaseModels(ctx, namespace)
		if err != nil {
			h.logger.Error("Failed to list base models",
				zap.String("namespace", namespace),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to list base models",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"items":     models.Items,
			"total":     len(models.Items),
			"namespace": namespace,
		})
		return
	}

	// Otherwise, list cluster-scoped ClusterBaseModels
	models, err := h.k8sClient.ListClusterBaseModels(ctx)
	if err != nil {
		h.logger.Error("Failed to list models", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list models",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": models.Items,
		"total": len(models.Items),
	})
}

// Get handles GET /api/v1/models/:name
func (h *ModelsHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	model, err := h.k8sClient.GetClusterBaseModel(ctx, name)
	if err != nil {
		h.logger.Error("Failed to get model", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Model not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, model.Object)
}

// Create handles POST /api/v1/models
func (h *ModelsHandler) Create(c *gin.Context) {
	ctx := c.Request.Context()

	var requestBody struct {
		Model            map[string]interface{} `json:"model"`
		HuggingfaceToken string                 `json:"huggingfaceToken,omitempty"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	modelData := requestBody.Model
	if modelData == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Missing 'model' field in request body",
		})
		return
	}

	// Create unstructured object
	model := &unstructured.Unstructured{Object: modelData}

	// Set GVK if not present
	if model.GetAPIVersion() == "" {
		model.SetAPIVersion("ome.io/v1beta1")
	}
	if model.GetKind() == "" {
		model.SetKind("ClusterBaseModel")
	}

	// If HuggingFace token provided, create secret and set storageKey
	if requestBody.HuggingfaceToken != "" {
		secretName := model.GetName() + "-hf-token"
		namespace := "ome" // ClusterBaseModels use ome namespace

		if err := h.k8sClient.CreateHuggingFaceTokenSecret(ctx, secretName, namespace, requestBody.HuggingfaceToken); err != nil {
			h.logger.Error("Failed to create HuggingFace token secret",
				zap.String("secretName", secretName),
				zap.String("namespace", namespace),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to create HuggingFace token secret",
				"details": err.Error(),
			})
			return
		}

		// Set storage.storageKey to reference the secret
		spec, found, err := unstructured.NestedMap(model.Object, "spec")
		if err != nil || !found {
			spec = make(map[string]interface{})
		}

		storage, found, err := unstructured.NestedMap(spec, "storage")
		if err != nil || !found {
			storage = make(map[string]interface{})
		}

		storage["storageKey"] = secretName
		spec["storage"] = storage
		model.Object["spec"] = spec

		h.logger.Info("Created HuggingFace token secret",
			zap.String("secretName", secretName),
			zap.String("namespace", namespace))
	}

	created, err := h.k8sClient.CreateClusterBaseModel(ctx, model)
	if err != nil {
		h.logger.Error("Failed to create model", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create model",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Model created successfully", zap.String("name", created.GetName()))
	c.JSON(http.StatusCreated, created.Object)
}

// Update handles PUT /api/v1/models/:name
func (h *ModelsHandler) Update(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	var modelData map[string]interface{}
	if err := c.ShouldBindJSON(&modelData); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Create unstructured object
	model := &unstructured.Unstructured{Object: modelData}

	// Ensure name matches
	model.SetName(name)

	// Set GVK if not present
	if model.GetAPIVersion() == "" {
		model.SetAPIVersion("ome.io/v1beta1")
	}
	if model.GetKind() == "" {
		model.SetKind("ClusterBaseModel")
	}

	updated, err := h.k8sClient.UpdateClusterBaseModel(ctx, model)
	if err != nil {
		h.logger.Error("Failed to update model", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update model",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Model updated successfully", zap.String("name", name))
	c.JSON(http.StatusOK, updated.Object)
}

// Delete handles DELETE /api/v1/models/:name
func (h *ModelsHandler) Delete(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	err := h.k8sClient.DeleteClusterBaseModel(ctx, name)
	if err != nil {
		h.logger.Error("Failed to delete model", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete model",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Model deleted successfully", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{
		"message": "Model deleted successfully",
		"name":    name,
	})
}

// GetStatus handles GET /api/v1/models/:name/status
func (h *ModelsHandler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	model, err := h.k8sClient.GetClusterBaseModel(ctx, name)
	if err != nil {
		h.logger.Error("Failed to get model status", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Model not found",
			"details": err.Error(),
		})
		return
	}

	// Extract status from the model
	status, found, err := unstructured.NestedMap(model.Object, "status")
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

// ===== BaseModel (namespace-scoped) Handlers =====

// ListBaseModels handles GET /api/v1/namespaces/:namespace/models
func (h *ModelsHandler) ListBaseModels(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	models, err := h.k8sClient.ListBaseModels(ctx, namespace)
	if err != nil {
		h.logger.Error("Failed to list base models", zap.String("namespace", namespace), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list base models",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": models.Items,
		"total": len(models.Items),
	})
}

// GetBaseModel handles GET /api/v1/namespaces/:namespace/models/:name
func (h *ModelsHandler) GetBaseModel(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	name := c.Param("name")

	model, err := h.k8sClient.GetBaseModel(ctx, namespace, name)
	if err != nil {
		h.logger.Error("Failed to get base model",
			zap.String("namespace", namespace),
			zap.String("name", name),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Base model not found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, model.Object)
}

// CreateBaseModel handles POST /api/v1/namespaces/:namespace/models
func (h *ModelsHandler) CreateBaseModel(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")

	var modelData map[string]interface{}
	if err := c.ShouldBindJSON(&modelData); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Create unstructured object
	model := &unstructured.Unstructured{Object: modelData}

	// Set GVK if not present
	if model.GetAPIVersion() == "" {
		model.SetAPIVersion("ome.io/v1beta1")
	}
	if model.GetKind() == "" {
		model.SetKind("BaseModel")
	}

	// Ensure namespace is set
	model.SetNamespace(namespace)

	created, err := h.k8sClient.CreateBaseModel(ctx, namespace, model)
	if err != nil {
		h.logger.Error("Failed to create base model",
			zap.String("namespace", namespace),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create base model",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Base model created successfully",
		zap.String("namespace", namespace),
		zap.String("name", created.GetName()))
	c.JSON(http.StatusCreated, created.Object)
}

// UpdateBaseModel handles PUT /api/v1/namespaces/:namespace/models/:name
func (h *ModelsHandler) UpdateBaseModel(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	name := c.Param("name")

	var modelData map[string]interface{}
	if err := c.ShouldBindJSON(&modelData); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Create unstructured object
	model := &unstructured.Unstructured{Object: modelData}

	// Ensure name and namespace match
	model.SetName(name)
	model.SetNamespace(namespace)

	// Set GVK if not present
	if model.GetAPIVersion() == "" {
		model.SetAPIVersion("ome.io/v1beta1")
	}
	if model.GetKind() == "" {
		model.SetKind("BaseModel")
	}

	updated, err := h.k8sClient.UpdateBaseModel(ctx, namespace, model)
	if err != nil {
		h.logger.Error("Failed to update base model",
			zap.String("namespace", namespace),
			zap.String("name", name),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update base model",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Base model updated successfully",
		zap.String("namespace", namespace),
		zap.String("name", name))
	c.JSON(http.StatusOK, updated.Object)
}

// DeleteBaseModel handles DELETE /api/v1/namespaces/:namespace/models/:name
func (h *ModelsHandler) DeleteBaseModel(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	name := c.Param("name")

	err := h.k8sClient.DeleteBaseModel(ctx, namespace, name)
	if err != nil {
		h.logger.Error("Failed to delete base model",
			zap.String("namespace", namespace),
			zap.String("name", name),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete base model",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Base model deleted successfully",
		zap.String("namespace", namespace),
		zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{
		"message":   "Base model deleted successfully",
		"name":      name,
		"namespace": namespace,
	})
}
