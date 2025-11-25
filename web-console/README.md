# OME Web Console

A modern web interface for managing OME (Open Model Engine) resources including models, serving runtimes, and inference services.

## Table of Contents

- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Development](#development)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [API Reference](#api-reference)
- [Make Commands](#make-commands)
- [Troubleshooting](#troubleshooting)
- [Contributing](#contributing)

---

## Quick Start

Get up and running in under 2 minutes:

```bash
# 1. Install dependencies
make install

# 2. Copy environment file and configure
cp backend/.env.example backend/.env

# 3. Start development servers (frontend + backend)
make dev
```

The console will be available at:
- **Frontend**: http://localhost:3000
- **Backend API**: http://localhost:8080

---

## Prerequisites

Before you begin, ensure you have the following installed:

| Tool | Version | Check Command |
|------|---------|---------------|
| Go | 1.24+ | `go version` |
| Node.js | 18+ | `node --version` |
| npm | 9+ | `npm --version` |
| kubectl | Latest | `kubectl version --client` |

You also need:
- Access to a Kubernetes cluster with OME CRDs installed
- `kubectl` configured with cluster access (`kubectl cluster-info` should work)

### Verify Prerequisites

```bash
# Quick check all prerequisites
go version && node --version && npm --version && kubectl cluster-info
```

---

## Installation

### 1. Clone and Navigate

```bash
cd web-console
```

### 2. Install Dependencies

```bash
# Install both frontend and backend dependencies
make install

# Or install separately:
make install-frontend  # npm install
make install-backend   # go mod download
```

### 3. Configure Environment

```bash
# Copy the example environment file
cp backend/.env.example backend/.env

# Edit if needed (defaults work for local development)
```

**Backend `.env` file:**
```bash
PORT=8080
GIN_MODE=debug
KUBECONFIG=~/.kube/config
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
```

**Frontend `.env.local` file (optional):**
```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

---

## Development

### Starting Development Servers

```bash
# Start both frontend and backend concurrently
make dev

# Or run them in separate terminals:
make dev-backend   # Terminal 1: Backend on :8080
make dev-frontend  # Terminal 2: Frontend on :3000
```

### Code Quality

```bash
# Run all linters
make lint

# Run linters separately
make lint-frontend  # ESLint
make lint-backend   # go vet

# Format code
make format

# Format separately
make format-frontend  # Prettier
make format-backend   # gofmt

# Check formatting without changes
make format-check

# TypeScript type checking
make typecheck

# Run tests
make test
```

### Building for Production

```bash
# Build both frontend and backend
make build

# Build separately
make build-frontend  # Creates .next/ directory
make build-backend   # Creates backend/bin/api binary

# Run production builds
make start-frontend  # Runs Next.js production server
make run-backend     # Runs compiled Go binary
```

### Useful Development Commands

```bash
# Check if services are running
make status

# Clean build artifacts
make clean

# Show all available commands
make help
```

---

## Configuration

### Backend Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | API server port |
| `GIN_MODE` | `debug` | Gin framework mode (`debug`, `release`) |
| `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig file |
| `KUBERNETES_IN_CLUSTER` | `false` | Set to `true` when running in-cluster |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000,http://localhost:3001` | Comma-separated allowed origins |

### Frontend Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Backend API URL |

---

## Architecture

```
web-console/
├── frontend/                # Next.js 14 React application
│   ├── src/
│   │   ├── app/            # App Router pages and layouts
│   │   │   └── (dashboard)/# Dashboard pages (models, runtimes, services)
│   │   ├── components/     # Reusable UI components
│   │   │   ├── ui/         # Base UI components (Button, Table, etc.)
│   │   │   ├── forms/      # Form components
│   │   │   └── layout/     # Layout components (Sidebar, etc.)
│   │   ├── lib/            # Utilities, hooks, and API clients
│   │   │   ├── api/        # API client functions
│   │   │   ├── hooks/      # React Query hooks
│   │   │   └── types/      # TypeScript type definitions
│   │   └── hooks/          # Custom React hooks
│   └── public/             # Static assets
│
├── backend/                 # Go API server
│   ├── cmd/api/            # Application entrypoint
│   └── internal/
│       ├── api/            # Server setup and routing
│       ├── handlers/       # HTTP request handlers
│       ├── k8s/            # Kubernetes client and operations
│       ├── middleware/     # HTTP middleware (logging)
│       └── services/       # Business logic services
│
└── Makefile                # Development automation
```

### Tech Stack

**Frontend:**
- Next.js 14 (App Router)
- React 18 + TypeScript
- TailwindCSS
- TanStack Query (React Query)
- React Hook Form + Zod

**Backend:**
- Go 1.24+
- Gin web framework
- client-go (Kubernetes client)
- Zap logger

---

## API Reference

### Health Check
```
GET /health
```

### Models
```
GET    /api/v1/models                    # List ClusterBaseModels
GET    /api/v1/models/:name              # Get ClusterBaseModel
POST   /api/v1/models                    # Create ClusterBaseModel
PUT    /api/v1/models/:name              # Update ClusterBaseModel
DELETE /api/v1/models/:name              # Delete ClusterBaseModel

# Namespace-scoped BaseModels
GET    /api/v1/namespaces/:ns/models           # List BaseModels
GET    /api/v1/namespaces/:ns/models/:name     # Get BaseModel
POST   /api/v1/namespaces/:ns/models           # Create BaseModel
PUT    /api/v1/namespaces/:ns/models/:name     # Update BaseModel
DELETE /api/v1/namespaces/:ns/models/:name     # Delete BaseModel
```

### Runtimes
```
GET    /api/v1/runtimes                  # List ClusterServingRuntimes
GET    /api/v1/runtimes/:name            # Get ClusterServingRuntime
POST   /api/v1/runtimes                  # Create ClusterServingRuntime
PUT    /api/v1/runtimes/:name            # Update ClusterServingRuntime
DELETE /api/v1/runtimes/:name            # Delete ClusterServingRuntime
POST   /api/v1/runtimes/:name/clone      # Clone a runtime
GET    /api/v1/runtimes/compatible       # Find compatible runtimes
GET    /api/v1/runtimes/recommend        # Get runtime recommendation
POST   /api/v1/runtimes/validate         # Validate runtime config
GET    /api/v1/runtimes/fetch-yaml       # Fetch YAML from URL
```

### Services
```
GET    /api/v1/services                  # List InferenceServices
GET    /api/v1/services/:name            # Get InferenceService
POST   /api/v1/services                  # Create InferenceService
PUT    /api/v1/services/:name            # Update InferenceService
DELETE /api/v1/services/:name            # Delete InferenceService
GET    /api/v1/services/:name/status     # Get service status
```

### HuggingFace Integration
```
GET /api/v1/huggingface/models/search         # Search HuggingFace models
GET /api/v1/huggingface/models/:id/info       # Get model info
GET /api/v1/huggingface/models/:id/config     # Get model config
```

### Other
```
GET /api/v1/namespaces                   # List namespaces
GET /api/v1/accelerators                 # List accelerators
GET /api/v1/events                       # SSE stream for real-time updates
POST /api/v1/validate/yaml               # Validate YAML
POST /api/v1/validate/model              # Validate model resource
POST /api/v1/validate/runtime            # Validate runtime resource
```

---

## Make Commands

### Development
| Command | Description |
|---------|-------------|
| `make dev` | Start both frontend and backend |
| `make dev-frontend` | Start frontend dev server (port 3000) |
| `make dev-backend` | Start backend dev server (port 8080) |

### Building
| Command | Description |
|---------|-------------|
| `make build` | Build both frontend and backend |
| `make build-frontend` | Build frontend for production |
| `make build-backend` | Build backend binary |

### Running
| Command | Description |
|---------|-------------|
| `make run-backend` | Run backend binary |
| `make start-frontend` | Run frontend production server |

### Dependencies
| Command | Description |
|---------|-------------|
| `make install` | Install all dependencies |
| `make install-frontend` | Install frontend dependencies (npm) |
| `make install-backend` | Download Go modules |

### Quality
| Command | Description |
|---------|-------------|
| `make lint` | Run all linters |
| `make lint-frontend` | Run ESLint |
| `make lint-backend` | Run go vet |
| `make format` | Format all code |
| `make format-check` | Check formatting |
| `make typecheck` | Run TypeScript type checking |
| `make test` | Run all tests |
| `make test-backend` | Run backend tests |
| `make test-frontend` | Run frontend tests |

### Utilities
| Command | Description |
|---------|-------------|
| `make status` | Check if services are running |
| `make clean` | Clean build artifacts |
| `make help` | Show all available commands |

---

## Troubleshooting

### Backend won't connect to Kubernetes cluster

```bash
# 1. Verify kubectl access
kubectl cluster-info
kubectl get nodes

# 2. Check kubeconfig path
echo $KUBECONFIG
cat ~/.kube/config | head -20

# 3. Verify OME CRDs are installed
kubectl get crd | grep ome.io
```

### Frontend can't reach backend

```bash
# 1. Verify backend is running
curl http://localhost:8080/health

# 2. Check for CORS issues (look at browser console)
# Backend allows localhost:3000 and localhost:3001 by default

# 3. Verify API URL in frontend
cat frontend/.env.local
```

### Port already in use

```bash
# Find process using port
lsof -i :8080  # Backend
lsof -i :3000  # Frontend

# Kill process
kill -9 <PID>

# Or use a different port
PORT=9090 make run-backend
```

### TypeScript errors

```bash
# Run type checking
make typecheck

# Clear Next.js cache and rebuild
rm -rf frontend/.next
make build-frontend
```

### Go module issues

```bash
# Clear module cache
go clean -modcache

# Re-download dependencies
cd backend && go mod download
```

### Real-time updates not working

The backend uses Server-Sent Events (SSE) for real-time updates. Check:

```bash
# Test SSE endpoint
curl -N http://localhost:8080/api/v1/events

# Verify informers are running (check backend logs)
make dev-backend
```

---

## Contributing

### Code Style

- **Go**: Follow standard Go conventions, use `gofmt`
- **TypeScript**: Follow ESLint rules, use Prettier for formatting
- **Commits**: Use conventional commit messages

### Before Submitting

```bash
# Run all checks
make lint
make typecheck
make test
make format-check
```

### Project Structure Conventions

- UI components go in `frontend/src/components/ui/`
- Form components go in `frontend/src/components/forms/`
- API hooks go in `frontend/src/lib/hooks/`
- Backend handlers go in `backend/internal/handlers/`
- Add new API routes in `backend/internal/api/server.go`

---

## License

See the main OME repository for license information.
