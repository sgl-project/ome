# OME Web Console - Implementation Plan

## Project Overview

Build a modern web interface for managing OME resources with focus on user experience, type safety, and Kubernetes integration.

---

## 🎯 Current Status (Updated 2025-01-23)

### ✅ Completed Phases
- **Phase 1**: Foundation - Backend API + Frontend Setup
- **Phase 2 (Week 3-4)**: Models Management - Full CRUD + HuggingFace Integration
- **Phase 3 (Week 5)**: Runtimes Management - Full CRUD (list, detail, create, delete)
- **Phase 5 (Partial)**: UI/UX Polish - Modern design system with gradient effects and animations

### 🚀 Live Services
- **Backend API**: Running on `http://localhost:8080`
- **Frontend UI**: Running on `http://localhost:3000`
- **Kubernetes**: Connected to cluster via kubeconfig

### 📊 Implementation Progress
- ✅ **Phase 1**: 100% Complete
- ✅ **Phase 2**: 100% Complete (CRUD + HuggingFace import wizard)
- ⏳ **Phase 3**: 75% Complete (CRUD done, intelligence features pending)
- ⏸️ **Phase 4**: Services Management (basic list view done, deployment wizard pending)
- ⏳ **Phase 5**: 40% Complete (UI/UX polish done, YAML editor pending)
- ⏸️ **Phase 6**: Not started

### 🎨 Key Features Implemented
- **Models Management**:
  - Full CRUD for both ClusterBaseModel (cluster-scoped) and BaseModel (namespace-scoped)
  - HuggingFace import wizard with 3-step process (search → scope → review)
  - Auto-detection of model format, architecture, and configuration from HF model hub
  - Model scope selector (cluster vs namespace)
  - Namespace filtering and sorting capabilities
- **Runtimes Management**:
  - List, create, view details, delete with confirmation modal
  - Namespace filtering support
  - Sortable table columns
- **Inference Services**:
  - Basic list view with status indicators
  - Real-time status updates (Ready, Running, Pending, Failed)
- **Type-Safe Forms**: React Hook Form + Zod validation for both resources
- **Modern UI/UX**:
  - Gradient purple/fading titles across all pages
  - Animated stat cards with hover effects and custom SVG icons
  - Consistent button styling with gradient effects (primary to accent)
  - Backdrop blur and glassmorphism effects
  - Smooth transitions and staggered animations
  - Custom gradient borders for primary actions
  - Landing page with bold aesthetic at root route (`/`)
  - Dashboard moved to `/dashboard` route
- **API Integration**: TanStack Query for data fetching and mutations
- **HuggingFace Integration**: Search, metadata retrieval, config.json parsing, format detection

### 📝 Next Steps
1. Implement edit functionality for Models and Runtimes
2. Complete Services deployment wizard (Phase 4)
3. Add search and filtering capabilities for resource lists
4. Add runtime intelligence features (auto-selection, compatibility checking)
5. Implement YAML editor with Monaco (Phase 5)

---

## Project Structure

```
ome/
├── web-console/
│   ├── frontend/                 # Next.js application
│   │   ├── src/
│   │   │   ├── app/             # Next.js 14 App Router
│   │   │   │   ├── (dashboard)/ # Route group for authenticated pages
│   │   │   │   │   ├── page.tsx                    # Dashboard
│   │   │   │   │   ├── models/
│   │   │   │   │   │   ├── page.tsx                # Models list
│   │   │   │   │   │   ├── new/page.tsx            # Create model
│   │   │   │   │   │   ├── [name]/
│   │   │   │   │   │   │   ├── page.tsx            # Model details
│   │   │   │   │   │   │   └── edit/page.tsx       # Edit model
│   │   │   │   │   │   └── import/page.tsx         # HF import wizard
│   │   │   │   │   ├── runtimes/
│   │   │   │   │   │   ├── page.tsx                # Runtimes list
│   │   │   │   │   │   ├── new/page.tsx            # Create runtime
│   │   │   │   │   │   └── [name]/
│   │   │   │   │   │       ├── page.tsx            # Runtime details
│   │   │   │   │   │       └── edit/page.tsx       # Edit runtime
│   │   │   │   │   ├── services/
│   │   │   │   │   │   ├── page.tsx                # Services list
│   │   │   │   │   │   ├── deploy/page.tsx         # Deploy service
│   │   │   │   │   │   └── [name]/
│   │   │   │   │   │       ├── page.tsx            # Service details
│   │   │   │   │   │       ├── edit/page.tsx       # Edit service
│   │   │   │   │   │       ├── metrics/page.tsx    # Metrics dashboard
│   │   │   │   │   │       └── logs/page.tsx       # Logs viewer
│   │   │   │   │   └── accelerators/
│   │   │   │   │       └── page.tsx                # Accelerators list
│   │   │   │   ├── api/             # API routes (Next.js)
│   │   │   │   │   └── proxy/       # Proxy to Go backend
│   │   │   │   ├── layout.tsx
│   │   │   │   └── globals.css
│   │   │   ├── components/
│   │   │   │   ├── ui/              # shadcn/ui components
│   │   │   │   ├── models/
│   │   │   │   │   ├── ModelCard.tsx
│   │   │   │   │   ├── ModelForm.tsx
│   │   │   │   │   └── ModelTable.tsx
│   │   │   │   ├── runtimes/
│   │   │   │   │   ├── RuntimeCard.tsx
│   │   │   │   │   ├── RuntimeForm.tsx
│   │   │   │   │   └── RuntimeWizard.tsx
│   │   │   │   ├── services/
│   │   │   │   │   ├── ServiceCard.tsx
│   │   │   │   │   ├── ServiceForm.tsx
│   │   │   │   │   ├── MetricsChart.tsx
│   │   │   │   │   └── LogsViewer.tsx
│   │   │   │   ├── shared/
│   │   │   │   │   ├── YamlEditor.tsx       # Monaco editor
│   │   │   │   │   ├── ResourceBadge.tsx
│   │   │   │   │   ├── StatusIndicator.tsx
│   │   │   │   │   └── LoadingSpinner.tsx
│   │   │   │   └── layout/
│   │   │   │       ├── Sidebar.tsx
│   │   │   │       ├── Header.tsx
│   │   │   │       └── Breadcrumbs.tsx
│   │   │   ├── lib/
│   │   │   │   ├── api/             # API client
│   │   │   │   │   ├── client.ts            # Axios/Fetch wrapper
│   │   │   │   │   ├── models.ts
│   │   │   │   │   ├── runtimes.ts
│   │   │   │   │   ├── services.ts
│   │   │   │   │   └── huggingface.ts
│   │   │   │   ├── hooks/           # React hooks
│   │   │   │   │   ├── useModels.ts
│   │   │   │   │   ├── useRuntimes.ts
│   │   │   │   │   ├── useServices.ts
│   │   │   │   │   └── useWebSocket.ts
│   │   │   │   ├── types/           # TypeScript types
│   │   │   │   │   ├── model.ts
│   │   │   │   │   ├── runtime.ts
│   │   │   │   │   ├── service.ts
│   │   │   │   │   └── k8s.ts
│   │   │   │   ├── utils/
│   │   │   │   │   ├── validation.ts        # Zod schemas
│   │   │   │   │   ├── yaml.ts              # YAML parsing
│   │   │   │   │   └── formatting.ts
│   │   │   │   └── store/           # Zustand store
│   │   │   │       └── ui-state.ts
│   │   │   └── styles/
│   │   ├── public/
│   │   ├── package.json
│   │   ├── tsconfig.json
│   │   ├── tailwind.config.js
│   │   └── next.config.js
│   │
│   └── backend/                   # Go API server
│       ├── cmd/
│       │   └── api/
│       │       └── main.go                   # Entry point
│       ├── internal/
│       │   ├── api/
│       │   │   ├── server.go                 # HTTP server setup
│       │   │   ├── routes.go                 # Route definitions
│       │   │   └── middleware/
│       │   │       ├── auth.go
│       │   │       ├── cors.go
│       │   │       └── logging.go
│       │   ├── handlers/         # HTTP handlers
│       │   │   ├── models.go
│       │   │   ├── runtimes.go
│       │   │   ├── services.go
│       │   │   ├── huggingface.go
│       │   │   └── validation.go
│       │   ├── services/         # Business logic
│       │   │   ├── model_service.go
│       │   │   ├── runtime_service.go
│       │   │   ├── inference_service.go
│       │   │   └── hf_client.go
│       │   ├── k8s/              # Kubernetes client
│       │   │   ├── client.go
│       │   │   ├── models.go
│       │   │   ├── runtimes.go
│       │   │   └── services.go
│       │   ├── validation/       # Validation logic
│       │   │   ├── model.go
│       │   │   ├── runtime.go
│       │   │   └── service.go
│       │   └── models/           # Data models
│       │       ├── model.go
│       │       ├── runtime.go
│       │       └── service.go
│       ├── pkg/
│       │   └── utils/
│       ├── go.mod
│       └── go.sum
│
└── deployment/
    ├── docker/
    │   ├── Dockerfile.frontend
    │   ├── Dockerfile.backend
    │   └── Dockerfile.combined      # Single image
    ├── kubernetes/
    │   ├── deployment.yaml
    │   ├── service.yaml
    │   ├── ingress.yaml
    │   ├── rbac.yaml
    │   └── configmap.yaml
    └── helm/
        └── ome-console/
            ├── Chart.yaml
            ├── values.yaml
            └── templates/
```

---

## Phase 1: Foundation (Week 1-2) ✅ **COMPLETED**

### **Week 1: Backend Setup** ✅

#### Day 1-2: Project Initialization
- [x] Initialize Go module
- [x] Set up project structure
- [x] Configure Gin framework
- [x] Set up logging (zap)
- [x] Configure environment variables

#### Day 3-4: Kubernetes Client
- [x] Initialize client-go
- [x] Create dynamic client for CRDs
- [x] Implement informers for caching
- [x] Test connection to cluster
- [x] Handle RBAC permissions

#### Day 5-7: Basic API Endpoints
- [x] Models CRUD endpoints
- [x] Runtimes CRUD endpoints
- [x] Services CRUD endpoints
- [x] Error handling middleware
- [x] API documentation (basic)

**Deliverables:**
- ✅ Go backend serving REST API on port 8080
- ✅ Connected to Kubernetes cluster
- ✅ Basic CRUD operations working

### **Week 2: Frontend Setup** ✅

#### Day 1-2: Next.js Project
- [x] Initialize Next.js 14 with TypeScript
- [x] Set up Tailwind CSS
- [x] Install basic UI components
- [x] Configure app router structure with (dashboard) route group
- [x] Set up layout components (Sidebar)

#### Day 3-4: API Client & State
- [x] Create API client with Fetch
- [x] Set up TanStack Query
- [x] Type definitions for Models, Runtimes, Services CRDs
- [x] Error handling utilities

#### Day 5-7: Basic Pages
- [x] Dashboard page (basic)
- [x] Models list page
- [x] Runtimes list page
- [x] Services list page
- [x] Accelerators list page
- [x] Basic navigation with Sidebar

**Deliverables:**
- ✅ Next.js app running on port 3000
- ✅ Basic pages rendering with stats
- ✅ API client connecting to backend

---

## Phase 2: Models Management (Week 3-4) ⏳ **IN PROGRESS**

### **Week 3: Models CRUD** ✅

#### Day 1-3: Models List & Details
- [x] Model list table component with stats
- [x] Model card/row component with clickable names
- [x] Model details page at `/models/[name]`
- [x] Status indicators (Ready, Failed, In_Transit)
- [ ] Search and filtering (not yet implemented)

#### Day 4-5: Create Model Form
- [x] Form builder with React Hook Form
- [x] Zod validation schema (`/lib/validation/model-schema.ts`)
- [x] Form sections (Basic Info, Model Format, Framework, Storage)
- [x] Dropdowns for format/framework selection
- [ ] Preview YAML (not yet implemented)

#### Day 6-7: Edit & Delete
- [ ] Edit model page (button ready, page not implemented)
- [x] Delete confirmation modal (reusable `Modal` component)
- [x] Delete functionality with API integration
- [ ] Bulk operations (not yet implemented)
- [x] Error handling

**Deliverables:**
- ✅ Complete models management UI (list, detail, create, delete)
- ✅ Create and delete models working
- ✅ Form validation with Zod
- ⏸️ Edit functionality pending

### **Week 4: HuggingFace Integration** ✅ **COMPLETED**

#### Day 1-3: HF Search & API
- [x] HuggingFace API client (backend - `/pkg/huggingface/client.go`)
- [x] Search endpoint (`/api/v1/huggingface/models/search`)
- [x] Model metadata retrieval (`/api/v1/huggingface/models/:modelId/info`)
- [x] config.json parsing (`/api/v1/huggingface/models/:modelId/config`)
- [x] Auto-detection logic (format detection, size estimation)
- [x] BaseModel support (namespace-scoped models)
- [x] Model scope selector (cluster vs namespace)

#### Day 4-7: Import Wizard
- [x] Search interface (`/models/import/page.tsx`)
- [x] Model selection from search results
- [x] Scope selection (cluster/namespace) with namespace input
- [x] Auto-configuration step with detected metadata
- [x] Review and import with generated model spec
- [x] Progress indicator (multi-step wizard)
- [x] Integration with both ClusterBaseModel and BaseModel APIs

**Deliverables:**
- ✅ HuggingFace import wizard (3-step process: search → scope → review)
- ✅ Auto-detect model architecture from config.json
- ✅ Model format detection (safetensors, pytorch, onnx, tensorflow)
- ✅ Support for both cluster-scoped and namespace-scoped models
- ✅ One-click import with auto-generated specifications

---

## Phase 3: Runtimes Management (Week 5-6) ⏳ **IN PROGRESS**

### **Week 5: Runtimes CRUD** ✅

#### Day 1-3: Runtimes UI
- [x] Runtime list component with stats (Total, Multi-Model, Disabled)
- [x] Runtime details page at `/runtimes/[name]`
- [x] Supported model formats display (table view)
- [x] Resource configuration view (containers, env vars)
- [x] Protocol versions display
- [x] Built-in adapter configuration display

#### Day 4-7: Runtime Creation Wizard
- [x] Create runtime form at `/runtimes/new`
- [x] Model support configuration (dynamic formats array)
- [x] React Hook Form with Zod validation
- [x] Configuration options (replicas, endpoints, multi-model)
- [ ] Template selection (not yet implemented)
- [ ] YAML preview (not yet implemented)

**Deliverables:**
- ✅ Runtimes management UI (list, detail, create, delete)
- ✅ Form-based creation with validation
- ✅ Delete confirmation modal
- ⏸️ Template support pending
- ⏸️ Edit functionality pending

### **Week 6: Runtime Intelligence** ⏸️ **NOT STARTED**

#### Day 1-3: Auto-Selection Logic
- [ ] Runtime matching algorithm
- [ ] Compatibility checking
- [ ] Priority-based selection
- [ ] Recommendation engine

#### Day 4-7: Testing & Validation
- [ ] Runtime configuration validator
- [ ] Resource availability check
- [ ] Test runtime endpoint
- [ ] Clone runtime feature

**Deliverables:**
- ⏸️ Smart runtime selection
- ⏸️ Validation before creation
- ⏸️ Test capabilities

---

## Phase 4: Inference Services (Week 7-8)

### **Week 7: Services Management**

#### Day 1-3: Services List & Details
- [ ] Service list with status
- [ ] Service details page
- [ ] Metrics display
- [ ] Traffic split visualization

#### Day 4-7: Deploy Service
- [ ] Deployment wizard
- [ ] Model + runtime selection
- [ ] Scaling configuration
- [ ] Accelerator selection
- [ ] Deploy action

**Deliverables:**
- ✅ Services list and details
- ✅ Deployment wizard
- ✅ Full deployment flow

### **Week 8: Operations**

#### Day 1-3: Scaling & Updates
- [ ] Scale service component
- [ ] Update service flow
- [ ] Rollback capability
- [ ] Canary deployment UI

#### Day 4-5: Logs & Events
- [ ] Log viewer component
- [ ] Real-time log streaming
- [ ] Filtering and search
- [ ] Event timeline

#### Day 6-7: Metrics Dashboard
- [ ] Metrics API integration
- [ ] Charts with Recharts
- [ ] Real-time updates
- [ ] Custom time ranges

**Deliverables:**
- ✅ Service operations complete
- ✅ Logs and metrics viewing
- ✅ Scaling and updates

---

## Phase 5: Advanced Features (Week 9-10)

### **Week 9: YAML Editor**

#### Day 1-3: Monaco Integration
- [ ] Monaco editor component
- [ ] YAML syntax highlighting
- [ ] Schema validation
- [ ] Auto-completion
- [ ] Diff viewer

#### Day 4-5: Schema Reference
- [ ] Schema documentation panel
- [ ] Field descriptions
- [ ] Example values
- [ ] Validation messages

#### Day 6-7: Template System
- [ ] Template library
- [ ] Template preview
- [ ] Custom templates
- [ ] Import/export templates

**Deliverables:**
- ✅ Advanced YAML editor
- ✅ Schema-aware editing
- ✅ Template support

### **Week 10: Polish & Testing**

#### Day 1-3: UI/UX Refinement
- [ ] Responsive design
- [ ] Loading states
- [ ] Error boundaries
- [ ] Toast notifications
- [ ] Keyboard shortcuts

#### Day 4-5: Testing
- [ ] Unit tests (backend)
- [ ] Component tests (frontend)
- [ ] Integration tests
- [ ] E2E tests (Playwright)

#### Day 6-7: Documentation
- [ ] User guide
- [ ] API documentation
- [ ] Deployment guide
- [ ] Video tutorials

**Deliverables:**
- ✅ Production-ready UI
- ✅ Comprehensive testing
- ✅ Complete documentation

---

## Phase 6: Deployment & DevOps (Week 11)

### **Week 11: Containerization & K8s**

#### Day 1-2: Docker Images
- [ ] Multi-stage Dockerfile
- [ ] Image optimization
- [ ] CI/CD pipeline (GitHub Actions)
- [ ] Image registry setup

#### Day 3-5: Kubernetes Manifests
- [ ] Deployment YAML
- [ ] Service and Ingress
- [ ] RBAC configuration
- [ ] ConfigMaps and Secrets
- [ ] Helm chart

#### Day 6-7: Testing & Launch
- [ ] Deploy to staging
- [ ] Integration testing
- [ ] Performance testing
- [ ] Production deployment
- [ ] Monitoring setup

**Deliverables:**
- ✅ Docker images published
- ✅ Helm chart ready
- ✅ Deployed to production

---

## Technology Decisions

### **Frontend Stack**
```json
{
  "dependencies": {
    "next": "^14.0.0",
    "react": "^18.2.0",
    "typescript": "^5.3.0",
    "@tanstack/react-query": "^5.0.0",
    "react-hook-form": "^7.49.0",
    "zod": "^3.22.0",
    "zustand": "^4.4.0",
    "@monaco-editor/react": "^4.6.0",
    "axios": "^1.6.0",
    "recharts": "^2.10.0",
    "tailwindcss": "^3.4.0",
    "@radix-ui/react-*": "latest",
    "class-variance-authority": "^0.7.0",
    "clsx": "^2.0.0",
    "lucide-react": "^0.294.0"
  }
}
```

### **Backend Stack**
```go
require (
    github.com/gin-gonic/gin v1.9.1
    k8s.io/client-go v0.28.4
    k8s.io/apimachinery v0.28.4
    sigs.k8s.io/controller-runtime v0.16.3
    github.com/go-playground/validator/v10 v10.16.0
    go.uber.org/zap v1.26.0
    github.com/swaggo/gin-swagger v1.6.0
)
```

---

## Development Commands

```bash
# Backend
cd backend
go run cmd/api/main.go

# Frontend
cd frontend
npm run dev

# Docker build
docker build -f deployment/docker/Dockerfile.combined -t ome-console:latest .

# Kubernetes deploy
kubectl apply -f deployment/kubernetes/

# Helm install
helm install ome-console deployment/helm/ome-console/
```

---

## Success Metrics

### **Performance**
- [ ] Page load time < 2s
- [ ] API response time < 500ms (p95)
- [ ] Real-time updates latency < 1s

### **Functionality**
- [ ] 100% CRUD operations for all resources
- [ ] HuggingFace import success rate > 95%
- [ ] Auto-runtime selection accuracy > 90%

### **Quality**
- [ ] Test coverage > 80%
- [ ] Zero critical bugs in production
- [ ] Lighthouse score > 90

### **Adoption**
- [ ] 50+ models imported via UI
- [ ] 10+ active users within first month
- [ ] Positive user feedback

---

## Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| K8s API changes | High | Use stable API versions, monitor deprecations |
| HuggingFace API limits | Medium | Implement caching, rate limiting |
| Complex CRD schemas | High | Comprehensive validation, user-friendly errors |
| Performance with many resources | Medium | Pagination, virtual scrolling, caching |
| Security vulnerabilities | Critical | Regular dependency updates, security scanning |

---

## Post-Launch Roadmap

### **Q1 2025**
- Multi-cluster support
- Cost tracking integration
- Advanced metrics and alerting
- Community templates marketplace

### **Q2 2025**
- A/B testing for model versions
- Model versioning and comparison
- Audit logs and compliance
- Mobile-responsive improvements

### **Q3 2025**
- AI-powered recommendations
- Automated optimization suggestions
- Integration with external monitoring (Grafana, Prometheus)
- CLI integration (generate CLI commands from UI)

---

This plan provides a clear, actionable roadmap to build the OME Web Console! 🚀
