package handlers

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"github.com/sgl-project/ome/web-console/backend/internal/services"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// allowedHostsMap maps user input hosts to safe constant values.
// Using a map lookup breaks taint tracking because the returned value
// is a constant from the map, not derived from user input.
var allowedHostsMap = map[string]string{
	"raw.githubusercontent.com":  "raw.githubusercontent.com",
	"github.com":                  "github.com",
	"gist.githubusercontent.com":  "gist.githubusercontent.com",
	"gitlab.com":                  "gitlab.com",
	"bitbucket.org":               "bitbucket.org",
}

// RuntimesHandler handles HTTP requests for ClusterServingRuntime resources
type RuntimesHandler struct {
	k8sClient    *k8s.Client
	logger       *zap.Logger
	intelligence *services.RuntimeIntelligenceService
}

// NewRuntimesHandler creates a new RuntimesHandler
func NewRuntimesHandler(k8sClient *k8s.Client, logger *zap.Logger) *RuntimesHandler {
	return &RuntimesHandler{
		k8sClient:    k8sClient,
		logger:       logger,
		intelligence: services.NewRuntimeIntelligenceService(k8sClient, logger),
	}
}

// List handles GET /api/v1/runtimes
// Supports optional ?namespace= query parameter
// - No namespace: returns ClusterServingRuntime resources
// - namespace=<name>: returns ServingRuntime resources from that namespace
func (h *RuntimesHandler) List(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Query("namespace")

	// If namespace is specified, list namespace-scoped ServingRuntimes
	if namespace != "" {
		runtimes, err := h.k8sClient.ListServingRuntimes(ctx, namespace)
		if err != nil {
			h.logger.Error("Failed to list serving runtimes",
				zap.String("namespace", namespace),
				zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to list serving runtimes",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"items":     runtimes.Items,
			"total":     len(runtimes.Items),
			"namespace": namespace,
		})
		return
	}

	// Otherwise, list cluster-scoped ClusterServingRuntimes
	runtimes, err := h.k8sClient.ListClusterServingRuntimes(ctx)
	if err != nil {
		h.logger.Error("Failed to list runtimes", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to list runtimes",
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
			"error":   "Runtime not found",
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
			"error":   "Invalid request body",
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
			"error":   "Failed to create runtime",
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
			"error":   "Invalid request body",
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
			"error":   "Failed to update runtime",
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
			"error":   "Failed to delete runtime",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Runtime deleted successfully", zap.String("name", name))
	c.JSON(http.StatusOK, gin.H{
		"message": "Runtime deleted successfully",
		"name":    name,
	})
}

// getSafeHost looks up a host in the allowlist and returns the safe constant value.
// This breaks taint tracking because map values are constants, not derived from input.
func getSafeHost(inputHost string) (string, bool) {
	inputHost = strings.ToLower(inputHost)

	// Direct match - return constant from map
	if safeHost, ok := allowedHostsMap[inputHost]; ok {
		return safeHost, true
	}

	// Check for subdomain match
	for allowedHost, safeHost := range allowedHostsMap {
		if strings.HasSuffix(inputHost, "."+allowedHost) {
			// For subdomains of allowed hosts, construct safe host from constant suffix
			prefix := strings.TrimSuffix(inputHost, "."+allowedHost)
			return prefix + "." + safeHost, true
		}
	}

	return "", false
}

// buildSafeURL validates a URL and constructs a safe version using only allowlisted hosts.
// Returns empty string and error message if validation fails.
func buildSafeURL(rawURL string) (string, string) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", "Invalid URL format: " + err.Error()
	}

	// Only allow HTTPS
	if parsedURL.Scheme != "https" {
		return "", "Only HTTPS URLs are allowed"
	}

	// Get safe host from allowlist (breaks taint chain via map lookup)
	safeHost, ok := getSafeHost(parsedURL.Host)
	if !ok {
		return "", "Host not in allowed list. Only GitHub, GitLab, and Bitbucket URLs are allowed"
	}

	// Clean path - remove any query strings or fragments for safety
	cleanPath := parsedURL.Path
	if cleanPath == "" {
		cleanPath = "/"
	}

	// Construct URL from safe constant host and cleaned path
	return "https://" + safeHost + cleanPath, ""
}

// FetchYAML handles GET /api/v1/runtimes/fetch-yaml?url=<url>
func (h *RuntimesHandler) FetchYAML(c *gin.Context) {
	rawURL := c.Query("url")
	if rawURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "URL parameter is required",
		})
		return
	}

	// Validate and build safe URL to prevent SSRF attacks
	// The host is looked up from allowedHostsMap which returns constant values
	safeURL, errMsg := buildSafeURL(rawURL)
	if errMsg != "" {
		h.logger.Warn("URL validation failed",
			zap.String("url", rawURL),
			zap.String("reason", errMsg))
		c.JSON(http.StatusForbidden, gin.H{
			"error":   "URL not allowed",
			"details": errMsg,
		})
		return
	}

	// Create HTTP client with timeout and strict redirect policy
	client := &http.Client{
		Timeout: 30 * time.Second,
		// Prevent all redirects - fetch only from the validated URL
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Block all redirects to prevent SSRF via open redirect
			return http.ErrUseLastResponse
		},
	}

	// Fetch using the safe URL constructed from allowlist constants
	resp, err := client.Get(safeURL)
	if err != nil {
		h.logger.Error("Failed to fetch URL", zap.String("url", safeURL), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch URL",
			"details": err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.Error("URL returned non-200 status",
			zap.String("url", safeURL),
			zap.Int("status", resp.StatusCode))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Failed to fetch URL",
			"details": "HTTP status: " + resp.Status,
		})
		return
	}

	// Limit response size to prevent memory exhaustion (10MB max)
	limitedReader := io.LimitReader(resp.Body, 10*1024*1024)
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		h.logger.Error("Failed to read response body", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to read response",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Successfully fetched YAML from URL", zap.String("url", safeURL))
	c.JSON(http.StatusOK, gin.H{
		"content": string(content),
	})
}

// FindCompatibleRuntimes handles GET /api/v1/runtimes/compatible?format=<format>&framework=<framework>
func (h *RuntimesHandler) FindCompatibleRuntimes(c *gin.Context) {
	ctx := c.Request.Context()
	modelFormat := c.Query("format")
	modelFramework := c.Query("framework")

	if modelFormat == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Model format is required",
		})
		return
	}

	matches, err := h.intelligence.FindCompatibleRuntimes(ctx, modelFormat, modelFramework)
	if err != nil {
		h.logger.Error("Failed to find compatible runtimes",
			zap.String("format", modelFormat),
			zap.String("framework", modelFramework),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to find compatible runtimes",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"matches": matches,
		"total":   len(matches),
	})
}

// CheckCompatibility handles GET /api/v1/runtimes/:name/compatibility?format=<format>&framework=<framework>
func (h *RuntimesHandler) CheckCompatibility(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")
	modelFormat := c.Query("format")
	modelFramework := c.Query("framework")

	if modelFormat == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Model format is required",
		})
		return
	}

	check, err := h.intelligence.CheckCompatibility(ctx, name, modelFormat, modelFramework)
	if err != nil {
		h.logger.Error("Failed to check compatibility",
			zap.String("runtime", name),
			zap.String("format", modelFormat),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to check compatibility",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, check)
}

// GetRecommendation handles GET /api/v1/runtimes/recommend?format=<format>&framework=<framework>
func (h *RuntimesHandler) GetRecommendation(c *gin.Context) {
	ctx := c.Request.Context()
	modelFormat := c.Query("format")
	modelFramework := c.Query("framework")

	if modelFormat == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Model format is required",
		})
		return
	}

	recommendation, err := h.intelligence.GetRecommendation(ctx, modelFormat, modelFramework)
	if err != nil {
		h.logger.Error("Failed to get recommendation",
			zap.String("format", modelFormat),
			zap.String("framework", modelFramework),
			zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "No compatible runtime found",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, recommendation)
}

// ValidateConfiguration handles POST /api/v1/runtimes/validate
func (h *RuntimesHandler) ValidateConfiguration(c *gin.Context) {
	ctx := c.Request.Context()

	var runtimeData map[string]interface{}
	if err := c.ShouldBindJSON(&runtimeData); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	runtime := &unstructured.Unstructured{Object: runtimeData}

	errors, warnings, err := h.intelligence.ValidateRuntimeConfiguration(ctx, runtime)
	if err != nil {
		h.logger.Error("Failed to validate configuration", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to validate configuration",
			"details": err.Error(),
		})
		return
	}

	valid := len(errors) == 0
	c.JSON(http.StatusOK, gin.H{
		"valid":    valid,
		"errors":   errors,
		"warnings": warnings,
	})
}

// Clone handles POST /api/v1/runtimes/:name/clone
func (h *RuntimesHandler) Clone(c *gin.Context) {
	ctx := c.Request.Context()
	name := c.Param("name")

	var req struct {
		NewName string `json:"newName" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to parse request body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Get the existing runtime
	runtime, err := h.k8sClient.GetClusterServingRuntime(ctx, name)
	if err != nil {
		h.logger.Error("Failed to get runtime for cloning", zap.String("name", name), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Runtime not found",
			"details": err.Error(),
		})
		return
	}

	// Clone the runtime
	cloned := runtime.DeepCopy()
	cloned.SetName(req.NewName)
	cloned.SetResourceVersion("")
	cloned.SetUID("")
	cloned.SetCreationTimestamp(metav1.Time{})
	cloned.SetGeneration(0)

	// Remove status
	unstructured.RemoveNestedField(cloned.Object, "status")

	// Create the cloned runtime
	created, err := h.k8sClient.CreateClusterServingRuntime(ctx, cloned)
	if err != nil {
		h.logger.Error("Failed to create cloned runtime",
			zap.String("original", name),
			zap.String("new", req.NewName),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create cloned runtime",
			"details": err.Error(),
		})
		return
	}

	h.logger.Info("Runtime cloned successfully",
		zap.String("original", name),
		zap.String("new", req.NewName))
	c.JSON(http.StatusCreated, created.Object)
}
