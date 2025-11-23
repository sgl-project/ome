# OME Web Console - Technical Architecture

## Overview

Modern web interface for managing OME (Open Model Engine) resources including models, runtimes, and inference services.

## Tech Stack

### Frontend
- **Framework**: Next.js 14 (App Router + React Server Components)
- **Language**: TypeScript 5.3+
- **Styling**: Tailwind CSS + shadcn/ui components
- **Forms**: React Hook Form + Zod validation
- **State**: TanStack Query (server) + Zustand (client)
- **Editor**: Monaco Editor (YAML/JSON with IntelliSense)
- **Charts**: Recharts / Chart.js

### Backend
- **Language**: Go 1.21+
- **Framework**: Gin or Fiber (high-performance web framework)
- **K8s Client**: client-go + controller-runtime
- **Validation**: go-playground/validator
- **API Docs**: Swagger/OpenAPI

### Infrastructure
- **Container**: Docker multi-stage builds
- **Orchestration**: Kubernetes (deployed as Deployment)
- **RBAC**: ServiceAccount with appropriate ClusterRole
- **Ingress**: Nginx/Traefik for external access

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         Browser                              │
└────────────────────────┬────────────────────────────────────┘
                         │
                         │ HTTPS
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    Next.js Frontend                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Dashboard    │  │ Models       │  │ Services     │      │
│  │ Page         │  │ Management   │  │ Management   │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│                                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Runtimes     │  │ HuggingFace  │  │ YAML         │      │
│  │ Management   │  │ Import       │  │ Editor       │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└────────────────────────┬────────────────────────────────────┘
                         │
                         │ REST API
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                     Go Backend API                           │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐ │
│  │              API Routes (Gin/Fiber)                    │ │
│  │  /api/v1/models       /api/v1/runtimes                │ │
│  │  /api/v1/services     /api/v1/accelerators            │ │
│  │  /api/v1/import/hf    /api/v1/validate                │ │
│  └────────────────────────────────────────────────────────┘ │
│                          │                                   │
│  ┌────────────────────────────────────────────────────────┐ │
│  │                Business Logic Layer                     │ │
│  │  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐ │ │
│  │  │ Model    │ │ Runtime  │ │ Service  │ │ HF       │ │ │
│  │  │ Service  │ │ Service  │ │ Service  │ │ Client   │ │ │
│  │  └──────────┘ └──────────┘ └──────────┘ └──────────┘ │ │
│  └────────────────────────────────────────────────────────┘ │
│                          │                                   │
│  ┌────────────────────────────────────────────────────────┐ │
│  │            Kubernetes Client (client-go)                │ │
│  │  - Dynamic Client (CRD operations)                      │ │
│  │  - Typed Client (native resources)                     │ │
│  │  - Informers (watch & cache)                           │ │
│  └────────────────────────────────────────────────────────┘ │
└────────────────────────┬────────────────────────────────────┘
                         │
                         │ Kubernetes API
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  Kubernetes API Server                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ ClusterBase  │  │ Cluster      │  │ Inference    │      │
│  │ Model CRD    │  │ ServingRT CRD│  │ Service CRD  │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

## API Endpoints

### Models API
```
GET    /api/v1/models                    # List all models
GET    /api/v1/models/:name              # Get model details
POST   /api/v1/models                    # Create model
PUT    /api/v1/models/:name              # Update model
DELETE /api/v1/models/:name              # Delete model
GET    /api/v1/models/:name/status       # Get model status
GET    /api/v1/models/:name/compatible-runtimes  # Get compatible runtimes
```

### Runtimes API
```
GET    /api/v1/runtimes                  # List all runtimes
GET    /api/v1/runtimes/:name            # Get runtime details
POST   /api/v1/runtimes                  # Create runtime
PUT    /api/v1/runtimes/:name            # Update runtime
DELETE /api/v1/runtimes/:name            # Delete runtime
POST   /api/v1/runtimes/:name/test       # Test runtime configuration
```

### Inference Services API
```
GET    /api/v1/services                  # List all services
GET    /api/v1/services/:name            # Get service details
POST   /api/v1/services                  # Deploy service
PUT    /api/v1/services/:name            # Update service
DELETE /api/v1/services/:name            # Delete service
GET    /api/v1/services/:name/status     # Get service status
POST   /api/v1/services/:name/scale      # Scale service
GET    /api/v1/services/:name/metrics    # Get service metrics
GET    /api/v1/services/:name/logs       # Get service logs
```

### HuggingFace Import API
```
GET    /api/v1/import/hf/search?q=llama          # Search HF models
GET    /api/v1/import/hf/models/:id              # Get HF model details
POST   /api/v1/import/hf/models/:id              # Import HF model
GET    /api/v1/import/hf/models/:id/config       # Get model config.json
```

### Validation & Utilities API
```
POST   /api/v1/validate/yaml             # Validate YAML against CRD schema
POST   /api/v1/validate/model            # Validate model configuration
POST   /api/v1/validate/runtime          # Validate runtime configuration
GET    /api/v1/schemas/:kind             # Get CRD JSON schema
GET    /api/v1/templates                 # List available templates
GET    /api/v1/templates/:name           # Get template YAML
```

### Accelerators API
```
GET    /api/v1/accelerators              # List available accelerators
GET    /api/v1/accelerators/:class       # Get accelerator class details
```

## Data Flow Examples

### Creating a Model from HuggingFace

```
User → Frontend: Click "Import from HuggingFace"
Frontend → Backend: GET /api/v1/import/hf/search?q=llama
Backend → HuggingFace API: Search request
HuggingFace API → Backend: Search results
Backend → Frontend: Formatted results

User → Frontend: Select "meta-llama/Llama-3.3-70B-Instruct"
Frontend → Backend: GET /api/v1/import/hf/models/meta-llama/Llama-3.3-70B-Instruct
Backend → HuggingFace API: Get model info + config.json
Backend → Frontend: Auto-populated model spec

User → Frontend: Click "Import"
Frontend → Backend: POST /api/v1/import/hf/models/meta-llama/Llama-3.3-70B-Instruct
Backend → K8s API: Create ClusterBaseModel CRD
K8s API → Backend: Success response
Backend → Frontend: Created model details
Frontend → User: Success notification
```

### Deploying an Inference Service

```
User → Frontend: Select model + runtime
Frontend → Backend: GET /api/v1/models/:name/compatible-runtimes
Backend → K8s API: List ServingRuntimes, match against model
Backend → Frontend: List of compatible runtimes with recommendations

User → Frontend: Configure scaling, click "Deploy"
Frontend → Backend: POST /api/v1/validate/yaml (pre-validation)
Backend → Frontend: Validation result

Frontend → Backend: POST /api/v1/services
Backend → K8s API: Create InferenceService CRD
K8s API → Backend: Success
Backend → Frontend: Created service
Frontend → User: Redirect to service details page

Frontend → Backend: WebSocket connection for status updates
Backend → K8s API: Watch InferenceService status
K8s API → Backend: Status changes (Pending → Ready)
Backend → Frontend: Real-time status updates
Frontend → User: Live status display
```

## Security Considerations

### Authentication & Authorization
- **RBAC**: ServiceAccount with least-privilege ClusterRole
- **API Auth**: JWT/OIDC integration for user authentication
- **Namespace Isolation**: Support for multi-tenant deployments

### Required Kubernetes Permissions
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ome-console
rules:
  - apiGroups: ["ome.io"]
    resources: ["clusterbasemodels", "clusterservingruntimes", "inferenceservices"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  - apiGroups: ["ome.io"]
    resources: ["clusterbasemodels/status", "inferenceservices/status"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch"]
```

## Deployment

### Docker Image Structure
```dockerfile
# Frontend build stage
FROM node:20-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Backend build stage
FROM golang:1.21-alpine AS backend-builder
WORKDIR /app/backend
COPY backend/go.* ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /ome-console-api ./cmd/api

# Final image
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app

# Copy backend binary
COPY --from=backend-builder /ome-console-api ./

# Copy frontend static files
COPY --from=frontend-builder /app/frontend/.next/static ./frontend/.next/static
COPY --from=frontend-builder /app/frontend/public ./frontend/public

# Backend serves API + frontend static files
EXPOSE 8080
CMD ["/app/ome-console-api"]
```

### Kubernetes Deployment
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ome-console
  namespace: ome-system
spec:
  replicas: 2
  selector:
    matchLabels:
      app: ome-console
  template:
    metadata:
      labels:
        app: ome-console
    spec:
      serviceAccountName: ome-console
      containers:
        - name: console
          image: ome/console:latest
          ports:
            - containerPort: 8080
          env:
            - name: KUBERNETES_SERVICE_HOST
              value: kubernetes.default.svc
          resources:
            requests:
              cpu: 100m
              memory: 256Mi
            limits:
              cpu: 500m
              memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: ome-console
  namespace: ome-system
spec:
  selector:
    app: ome-console
  ports:
    - port: 80
      targetPort: 8080
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ome-console
  namespace: ome-system
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt
spec:
  tls:
    - hosts:
        - console.ome.example.com
      secretName: ome-console-tls
  rules:
    - host: console.ome.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: ome-console
                port:
                  number: 80
```

## Performance Optimizations

1. **Frontend**:
   - Server-side rendering for initial load
   - React Server Components for reduced client bundle
   - Code splitting per route
   - Image optimization with Next.js Image

2. **Backend**:
   - Kubernetes informer cache (reduce API calls)
   - Connection pooling
   - Response caching with Redis (optional)
   - Gzip compression

3. **API**:
   - Pagination for list endpoints
   - Field selectors to reduce payload size
   - WebSocket for real-time updates (reduce polling)

## Development Workflow

### Local Development
```bash
# Backend (requires kubeconfig)
cd backend
go run cmd/api/main.go

# Frontend
cd frontend
npm run dev

# Access: http://localhost:3000
```

### Testing
```bash
# Backend tests
cd backend
go test ./...

# Frontend tests
cd frontend
npm test

# E2E tests
npm run test:e2e
```

## Future Enhancements

1. **Multi-cluster support**: Manage models across multiple K8s clusters
2. **Cost tracking**: Integrate with cloud billing APIs
3. **A/B testing**: Built-in traffic splitting for model versions
4. **Marketplace**: Community-contributed model templates
5. **CLI integration**: Web console can generate CLI commands
6. **Audit logs**: Track all changes with PostgreSQL backend
7. **Alerts**: Integration with AlertManager for service health
8. **Model versioning**: Track and compare model versions

## References

- [Next.js Documentation](https://nextjs.org/docs)
- [Kubernetes client-go](https://github.com/kubernetes/client-go)
- [shadcn/ui Components](https://ui.shadcn.com)
- [HuggingFace API](https://huggingface.co/docs/hub/api)
