# OME Web Console - Quick Start Guide

## Prerequisites

- **Node.js** 20+ and npm/yarn
- **Go** 1.21+
- **Docker** and Docker Compose (for containerized development)
- **Kubernetes cluster** with OME CRDs installed
- **kubectl** configured to access your cluster

---

## Option 1: Automated Setup (Recommended)

Run the initialization script:

```bash
cd /Users/simolin/workspace/ome
./scripts/setup-web-console.sh
```

This script will:
1. Create project structure
2. Initialize frontend (Next.js + TypeScript)
3. Initialize backend (Go module)
4. Install dependencies
5. Generate initial configuration files
6. Set up Docker Compose for local development

---

## Option 2: Manual Setup

### Step 1: Create Project Structure

```bash
cd /Users/simolin/workspace/ome
mkdir -p web-console/{frontend,backend,deployment}
```

### Step 2: Initialize Frontend

```bash
cd web-console/frontend

# Create Next.js app with TypeScript
npx create-next-app@latest . \
  --typescript \
  --tailwind \
  --app \
  --src-dir \
  --import-alias "@/*" \
  --no-git

# Install dependencies
npm install \
  @tanstack/react-query \
  @tanstack/react-query-devtools \
  react-hook-form \
  zod \
  @hookform/resolvers \
  zustand \
  axios \
  @monaco-editor/react \
  recharts \
  date-fns \
  lucide-react \
  class-variance-authority \
  clsx \
  tailwind-merge \
  js-yaml

# Install shadcn/ui
npx shadcn-ui@latest init -y

# Add commonly used shadcn components
npx shadcn-ui@latest add button card input label select table \
  dialog dropdown-menu form tabs toast badge alert \
  separator skeleton switch checkbox radio-group \
  scroll-area tooltip popover command
```

### Step 3: Initialize Backend

```bash
cd ../backend

# Initialize Go module
go mod init github.com/sgl-project/ome/web-console/backend

# Install dependencies
go get -u \
  github.com/gin-gonic/gin \
  k8s.io/client-go@v0.28.4 \
  k8s.io/apimachinery@v0.28.4 \
  sigs.k8s.io/controller-runtime@v0.16.3 \
  github.com/go-playground/validator/v10 \
  go.uber.org/zap \
  github.com/swaggo/gin-swagger \
  github.com/swaggo/files

# Create initial structure
mkdir -p cmd/api internal/{api,handlers,services,k8s,validation,models}
```

### Step 4: Create Configuration Files

#### `frontend/.env.local`
```env
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_WS_URL=ws://localhost:8080
```

#### `backend/.env`
```env
PORT=8080
GIN_MODE=debug
KUBERNETES_IN_CLUSTER=false
KUBECONFIG=/Users/simolin/.kube/config
CORS_ALLOWED_ORIGINS=http://localhost:3000
```

#### `docker-compose.yml` (in web-console/)
```yaml
version: '3.8'

services:
  backend:
    build:
      context: ./backend
      dockerfile: ../deployment/docker/Dockerfile.backend
    ports:
      - "8080:8080"
    environment:
      - KUBERNETES_IN_CLUSTER=false
      - KUBECONFIG=/root/.kube/config
    volumes:
      - ~/.kube:/root/.kube:ro
      - ./backend:/app
    command: go run cmd/api/main.go

  frontend:
    build:
      context: ./frontend
      dockerfile: ../deployment/docker/Dockerfile.frontend
    ports:
      - "3000:3000"
    environment:
      - NEXT_PUBLIC_API_URL=http://backend:8080
    volumes:
      - ./frontend:/app
      - /app/node_modules
      - /app/.next
    command: npm run dev
    depends_on:
      - backend
```

---

## Step 5: Create Initial Files

### Backend Entry Point

**`backend/cmd/api/main.go`**
```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/internal/api"
	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Initialize Kubernetes client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		logger.Fatal("Failed to create Kubernetes client", zap.Error(err))
	}

	// Create API server
	server := api.NewServer(k8sClient, logger)

	// Setup routes
	router := server.SetupRoutes()

	// Create HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	// Start server
	go func() {
		logger.Info("Starting server", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}
```

**`backend/internal/api/server.go`**
```go
package api

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sgl-project/ome/web-console/backend/internal/handlers"
	"github.com/sgl-project/ome/web-console/backend/internal/k8s"
	"go.uber.org/zap"
)

type Server struct {
	k8sClient *k8s.Client
	logger    *zap.Logger
}

func NewServer(k8sClient *k8s.Client, logger *zap.Logger) *Server {
	return &Server{
		k8sClient: k8sClient,
		logger:    logger,
	}
}

func (s *Server) SetupRoutes() *gin.Engine {
	router := gin.Default()

	// CORS middleware
	router.Use(cors.Default())

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Models
		models := v1.Group("/models")
		{
			modelHandler := handlers.NewModelHandler(s.k8sClient, s.logger)
			models.GET("", modelHandler.List)
			models.GET("/:name", modelHandler.Get)
			models.POST("", modelHandler.Create)
			models.PUT("/:name", modelHandler.Update)
			models.DELETE("/:name", modelHandler.Delete)
		}

		// Runtimes
		runtimes := v1.Group("/runtimes")
		{
			runtimeHandler := handlers.NewRuntimeHandler(s.k8sClient, s.logger)
			runtimes.GET("", runtimeHandler.List)
			runtimes.GET("/:name", runtimeHandler.Get)
			runtimes.POST("", runtimeHandler.Create)
			runtimes.PUT("/:name", runtimeHandler.Update)
			runtimes.DELETE("/:name", runtimeHandler.Delete)
		}

		// Services
		services := v1.Group("/services")
		{
			serviceHandler := handlers.NewServiceHandler(s.k8sClient, s.logger)
			services.GET("", serviceHandler.List)
			services.GET("/:name", serviceHandler.Get)
			services.POST("", serviceHandler.Create)
			services.PUT("/:name", serviceHandler.Update)
			services.DELETE("/:name", serviceHandler.Delete)
		}
	}

	return router
}
```

**`backend/internal/k8s/client.go`**
```go
package k8s

import (
	"os"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Clientset     *kubernetes.Clientset
	DynamicClient dynamic.Interface
	Config        *rest.Config
}

func NewClient() (*Client, error) {
	var config *rest.Config
	var err error

	// Try in-cluster config first
	if os.Getenv("KUBERNETES_IN_CLUSTER") == "true" {
		config, err = rest.InClusterConfig()
	} else {
		// Use kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("HOME") + "/.kube/config"
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Client{
		Clientset:     clientset,
		DynamicClient: dynamicClient,
		Config:        config,
	}, nil
}
```

### Frontend Setup

**`frontend/src/lib/api/client.ts`**
```typescript
import axios from 'axios';

const API_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export const apiClient = axios.create({
  baseURL: `${API_URL}/api/v1`,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor
apiClient.interceptors.request.use(
  (config) => {
    // Add auth token if available
    const token = localStorage.getItem('auth_token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => Promise.reject(error)
);

// Response interceptor
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    // Handle errors globally
    console.error('API Error:', error);
    return Promise.reject(error);
  }
);
```

**`frontend/src/app/page.tsx`** (Dashboard)
```typescript
export default function DashboardPage() {
  return (
    <div className="container mx-auto py-8">
      <h1 className="text-4xl font-bold mb-8">OME Console</h1>

      <div className="grid grid-cols-1 md:grid-cols-4 gap-6">
        <div className="p-6 border rounded-lg">
          <h3 className="text-lg font-semibold mb-2">Models</h3>
          <p className="text-3xl font-bold">24</p>
        </div>
        <div className="p-6 border rounded-lg">
          <h3 className="text-lg font-semibold mb-2">Runtimes</h3>
          <p className="text-3xl font-bold">18</p>
        </div>
        <div className="p-6 border rounded-lg">
          <h3 className="text-lg font-semibold mb-2">Services</h3>
          <p className="text-3xl font-bold">12</p>
        </div>
        <div className="p-6 border rounded-lg">
          <h3 className="text-lg font-semibold mb-2">Accelerators</h3>
          <p className="text-3xl font-bold">3</p>
        </div>
      </div>
    </div>
  );
}
```

---

## Running the Application

### Development Mode

#### Backend
```bash
cd backend
go run cmd/api/main.go
# Server running on http://localhost:8080
```

#### Frontend
```bash
cd frontend
npm run dev
# Next.js running on http://localhost:3000
```

### Docker Compose
```bash
cd web-console
docker-compose up --build
# Backend: http://localhost:8080
# Frontend: http://localhost:3000
```

---

## Testing the Setup

### 1. Test Backend API
```bash
# Health check
curl http://localhost:8080/health

# List models
curl http://localhost:8080/api/v1/models

# List runtimes
curl http://localhost:8080/api/v1/runtimes

# List services
curl http://localhost:8080/api/v1/services
```

### 2. Test Frontend
Visit http://localhost:3000 in your browser

---

## Next Steps

1. **Implement Model Handlers**: Complete CRUD operations in `backend/internal/handlers/models.go`
2. **Create Model Pages**: Build out `frontend/src/app/(dashboard)/models/` pages
3. **Add Type Definitions**: Define TypeScript types in `frontend/src/lib/types/`
4. **Build Components**: Create reusable components in `frontend/src/components/`

---

## Troubleshooting

### Backend Won't Start
- Check Kubernetes connection: `kubectl cluster-info`
- Verify KUBECONFIG path in `.env`
- Check port 8080 is not in use: `lsof -i :8080`

### Frontend Won't Build
- Clear cache: `rm -rf .next node_modules && npm install`
- Check Node.js version: `node --version` (should be 20+)

### CORS Errors
- Ensure `NEXT_PUBLIC_API_URL` matches backend URL
- Check backend CORS configuration in `api/server.go`

---

## Resources

- [Next.js Documentation](https://nextjs.org/docs)
- [Gin Web Framework](https://gin-gonic.com/docs/)
- [Kubernetes client-go](https://github.com/kubernetes/client-go)
- [shadcn/ui](https://ui.shadcn.com)
- [TanStack Query](https://tanstack.com/query/latest)

---

Ready to build! 🚀
