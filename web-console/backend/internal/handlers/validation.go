package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ValidationHandler handles HTTP requests for validation endpoints
type ValidationHandler struct {
	k8sClient *k8s.Client
	logger    *zap.Logger
}

// NewValidationHandler creates a new ValidationHandler
func NewValidationHandler(k8sClient *k8s.Client, logger *zap.Logger) *ValidationHandler {
	return &ValidationHandler{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// ValidateYAML handles POST /api/v1/validate/yaml
func (h *ValidationHandler) ValidateYAML(c *gin.Context) {
	var req struct {
		YAML string `json:"yaml" binding:"required"`
		Kind string `json:"kind"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Try to parse YAML
	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(req.YAML), &data); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Invalid YAML syntax",
			"details": err.Error(),
		})
		return
	}

	// Create unstructured object to validate structure
	obj := &unstructured.Unstructured{Object: data}

	// Basic validation
	if obj.GetAPIVersion() == "" {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Missing apiVersion field",
		})
		return
	}

	if obj.GetKind() == "" {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Missing kind field",
		})
		return
	}

	if obj.GetName() == "" {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Missing metadata.name field",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"message": "YAML is valid",
		"kind": obj.GetKind(),
		"name": obj.GetName(),
	})
}

// ValidateModel handles POST /api/v1/validate/model
func (h *ValidationHandler) ValidateModel(c *gin.Context) {
	var modelData map[string]interface{}
	if err := c.ShouldBindJSON(&modelData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	model := &unstructured.Unstructured{Object: modelData}

	// Validate required fields for ClusterBaseModel
	spec, found, err := unstructured.NestedMap(model.Object, "spec")
	if err != nil || !found {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Missing spec field",
		})
		return
	}

	// Check for storage configuration
	_, found, err = unstructured.NestedMap(spec, "storage")
	if err != nil || !found {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Missing spec.storage field",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"message": "Model configuration is valid",
	})
}

// ValidateRuntime handles POST /api/v1/validate/runtime
func (h *ValidationHandler) ValidateRuntime(c *gin.Context) {
	var runtimeData map[string]interface{}
	if err := c.ShouldBindJSON(&runtimeData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	runtime := &unstructured.Unstructured{Object: runtimeData}

	// Validate required fields for ClusterServingRuntime
	spec, found, err := unstructured.NestedMap(runtime.Object, "spec")
	if err != nil || !found {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Missing spec field",
		})
		return
	}

	// Check for supported model formats
	_, found, err = unstructured.NestedSlice(spec, "supportedModelFormats")
	if err != nil || !found {
		c.JSON(http.StatusOK, gin.H{
			"valid": false,
			"error": "Missing spec.supportedModelFormats field",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"message": "Runtime configuration is valid",
	})
}
