# OME Web Console

A modern web interface for managing OME (Open Model Engine) resources including models, serving runtimes, and inference services.

## Architecture

```
web-console/
├── frontend/          # Next.js 14 React application
│   ├── src/
│   │   ├── app/       # App Router pages and layouts
│   │   ├── components/# Reusable UI components
│   │   ├── lib/       # Utilities, hooks, and API clients
│   │   └── types/     # TypeScript type definitions
│   └── public/        # Static assets
│
└── backend/           # Go API server
    ├── cmd/api/       # Application entrypoint
    ├── internal/
    │   ├── api/       # Server setup and routing
    │   ├── handlers/  # HTTP request handlers
    │   ├── k8s/       # Kubernetes client and informers
    │   └── middleware/# HTTP middleware (CORS, logging)
    └── pkg/           # Shared packages
```

## Prerequisites

- **Go** 1.24+
- **Node.js** 18+
- **npm** 9+
- **kubectl** configured with cluster access
- Access to a Kubernetes cluster with OME CRDs installed

## Quick Start

```bash
# Start both frontend and backend
make dev

# Or start them separately:
make dev-backend   # Backend on :8080
make dev-frontend  # Frontend on :3000
```

## Development

### Backend (Go API Server)

The backend provides a REST API that communicates with the Kubernetes cluster.

```bash
# Build the backend binary
make build-backend

# Run the backend server
make run-backend

# Run with custom port
PORT=9090 make run-backend

# Run with specific kubeconfig
KUBECONFIG=~/.kube/my-cluster make run-backend
```

**API Endpoints:**
- `GET /health` - Health check
- `GET /api/v1/models` - List ClusterBaseModels
- `GET /api/v1/namespaces/:ns/models` - List BaseModels in namespace
- `GET /api/v1/runtimes` - List ClusterServingRuntimes
- `GET /api/v1/services` - List InferenceServices
- `GET /api/v1/huggingface/models/search` - Search HuggingFace models
- `GET /api/v1/events` - SSE stream for real-time updates

### Frontend (Next.js)

The frontend is a React application built with Next.js 14 App Router.

```bash
# Install dependencies
make install-frontend

# Run development server
make dev-frontend

# Build for production
make build-frontend

# Run production build
make start-frontend
```

**Tech Stack:**
- Next.js 14 with App Router
- React 18 with TypeScript
- TailwindCSS for styling
- TanStack Query for data fetching
- React Hook Form + Zod for forms

## Configuration

### Backend Environment Variables

| Variable     | Default          | Description                             |
|--------------|------------------|-----------------------------------------|
| `PORT`       | `8080`           | API server port                         |
| `GIN_MODE`   | `debug`          | Gin framework mode (`debug`, `release`) |
| `KUBECONFIG` | `~/.kube/config` | Path to kubeconfig file                 |

### Frontend Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Backend API URL |

Create a `.env.local` file in the frontend directory:
```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

## Make Commands

```bash
# Development
make dev              # Start both frontend and backend
make dev-frontend     # Start frontend dev server
make dev-backend      # Start backend dev server

# Building
make build            # Build both frontend and backend
make build-frontend   # Build frontend for production
make build-backend    # Build backend binary

# Running
make run-backend      # Run backend binary
make start-frontend   # Run frontend production server

# Dependencies
make install          # Install all dependencies
make install-frontend # Install frontend dependencies
make install-backend  # Download Go modules

# Utilities
make clean            # Clean build artifacts
make lint             # Run linters
make test             # Run tests
make help             # Show available commands
```

## Project Structure

### Frontend Components

```
src/components/
├── layout/
│   └── Sidebar.tsx       # Navigation sidebar
├── ui/
│   ├── Button.tsx        # Button variants with icons
│   ├── DataTable.tsx     # Generic data table
│   ├── StatCard.tsx      # Statistics cards
│   ├── StatusBadge.tsx   # Status indicators
│   ├── LoadingState.tsx  # Loading spinners
│   └── ErrorState.tsx    # Error displays
└── forms/
    └── ...               # Form components
```

### Backend Handlers

```
internal/handlers/
├── models.go         # ClusterBaseModel/BaseModel CRUD
├── runtimes.go       # ClusterServingRuntime operations
├── services.go       # InferenceService management
├── huggingface.go    # HuggingFace API integration
├── namespaces.go     # Namespace listing
├── accelerators.go   # Accelerator information
├── events.go         # SSE event streaming
└── validation.go     # YAML/resource validation
```

## Features

- **Model Management**: Import models from HuggingFace, create custom models, view status
- **Runtime Configuration**: Configure serving runtimes with accelerators and resources
- **Service Deployment**: Deploy and monitor inference services
- **Real-time Updates**: SSE-based live status updates
- **YAML Validation**: Validate Kubernetes resources before applying

## Troubleshooting

### Backend won't connect to cluster
```bash
# Verify kubectl access
kubectl cluster-info

# Check kubeconfig path
echo $KUBECONFIG
```

### Frontend can't reach backend
```bash
# Verify backend is running
curl http://localhost:8080/health

# Check CORS settings in backend
# Backend allows localhost:3000 by default
```

### Port already in use
```bash
# Find process using port
lsof -i :8080
lsof -i :3000

# Kill process
kill -9 <PID>
```

## License

See the main OME repository for license information.
