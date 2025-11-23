package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/internal/handlers"
	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"github.com/sgl-project/ome/web-console/backend/internal/middleware"
	"go.uber.org/zap"
)

// Server wraps the HTTP server and dependencies
type Server struct {
	k8sClient *k8s.Client
	logger    *zap.Logger
}

// NewServer creates a new API server instance
func NewServer(k8sClient *k8s.Client, logger *zap.Logger) *Server {
	return &Server{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

// SetupRoutes configures all API routes
func (s *Server) SetupRoutes() *gin.Engine {
	// Set Gin mode based on environment
	ginMode := gin.ReleaseMode
	if gin.Mode() == gin.DebugMode {
		ginMode = gin.DebugMode
	}
	gin.SetMode(ginMode)

	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(middleware.Logger(s.logger))

	// CORS configuration
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:3000", "http://localhost:3001"}
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	router.Use(cors.New(config))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "ome-web-console-api",
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// ClusterBaseModel endpoints (cluster-scoped)
		modelsHandler := handlers.NewModelsHandler(s.k8sClient, s.logger)
		models := v1.Group("/models")
		{
			models.GET("", modelsHandler.List)
			models.GET("/:name", modelsHandler.Get)
			models.POST("", modelsHandler.Create)
			models.PUT("/:name", modelsHandler.Update)
			models.DELETE("/:name", modelsHandler.Delete)
			models.GET("/:name/status", modelsHandler.GetStatus)
		}

		// Namespaces endpoints
		namespacesHandler := handlers.NewNamespacesHandler(s.k8sClient, s.logger)
		v1.GET("/namespaces", namespacesHandler.List)

		// BaseModel endpoints (namespace-scoped)
		namespacesGroup := v1.Group("/namespaces")
		{
			baseModels := namespacesGroup.Group("/:namespace/models")
			{
				baseModels.GET("", modelsHandler.ListBaseModels)
				baseModels.GET("/:name", modelsHandler.GetBaseModel)
				baseModels.POST("", modelsHandler.CreateBaseModel)
				baseModels.PUT("/:name", modelsHandler.UpdateBaseModel)
				baseModels.DELETE("/:name", modelsHandler.DeleteBaseModel)
			}
		}

		// Runtimes endpoints
		runtimesHandler := handlers.NewRuntimesHandler(s.k8sClient, s.logger)
		runtimes := v1.Group("/runtimes")
		{
			runtimes.GET("", runtimesHandler.List)
			runtimes.GET("/fetch-yaml", runtimesHandler.FetchYAML)
			runtimes.GET("/:name", runtimesHandler.Get)
			runtimes.POST("", runtimesHandler.Create)
			runtimes.PUT("/:name", runtimesHandler.Update)
			runtimes.DELETE("/:name", runtimesHandler.Delete)
		}

		// Inference Services endpoints
		servicesHandler := handlers.NewServicesHandler(s.k8sClient, s.logger)
		services := v1.Group("/services")
		{
			services.GET("", servicesHandler.List)
			services.GET("/:name", servicesHandler.Get)
			services.POST("", servicesHandler.Create)
			services.PUT("/:name", servicesHandler.Update)
			services.DELETE("/:name", servicesHandler.Delete)
			services.GET("/:name/status", servicesHandler.GetStatus)
		}

		// Accelerators endpoints
		acceleratorsHandler := handlers.NewAcceleratorsHandler(s.k8sClient, s.logger)
		accelerators := v1.Group("/accelerators")
		{
			accelerators.GET("", acceleratorsHandler.List)
			accelerators.GET("/:name", acceleratorsHandler.Get)
		}

		// Validation endpoints
		validationHandler := handlers.NewValidationHandler(s.k8sClient, s.logger)
		validation := v1.Group("/validate")
		{
			validation.POST("/yaml", validationHandler.ValidateYAML)
			validation.POST("/model", validationHandler.ValidateModel)
			validation.POST("/runtime", validationHandler.ValidateRuntime)
		}

		// HuggingFace integration endpoints
		hfHandler := handlers.NewHuggingFaceHandler(s.logger)
		hf := v1.Group("/huggingface")
		{
			hf.GET("/models/search", hfHandler.SearchModels)
			hf.GET("/models/:modelId/info", hfHandler.GetModelInfo)
			hf.GET("/models/:modelId/:modelName/info", hfHandler.GetModelInfo)
			hf.GET("/models/:modelId/config", hfHandler.GetModelConfig)
			hf.GET("/models/:modelId/:modelName/config", hfHandler.GetModelConfig)
		}

		// Server-Sent Events endpoint for real-time updates
		eventsHandler := handlers.NewEventsHandler(s.k8sClient, s.logger)
		v1.GET("/events", eventsHandler.Stream)
	}

	s.logger.Info("API routes configured successfully")
	return router
}
