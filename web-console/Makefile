# OME Web Console Makefile
# Run 'make help' to see available commands

.PHONY: help dev dev-frontend dev-backend build build-frontend build-backend \
        run-backend start-frontend install install-frontend install-backend \
        clean lint test

# Colors for terminal output
CYAN := \033[36m
GREEN := \033[32m
YELLOW := \033[33m
RESET := \033[0m

# Directories
FRONTEND_DIR := frontend
BACKEND_DIR := backend
BACKEND_BIN := $(BACKEND_DIR)/bin/api

# Default target
.DEFAULT_GOAL := help

##@ Development

dev: ## Start both frontend and backend in development mode
	@echo "$(CYAN)Starting OME Web Console...$(RESET)"
	@$(MAKE) -j2 dev-frontend dev-backend

dev-frontend: ## Start frontend development server (port 3000)
	@echo "$(GREEN)Starting frontend on http://localhost:3000$(RESET)"
	@cd $(FRONTEND_DIR) && npm run dev

dev-backend: ## Start backend development server (port 8080)
	@echo "$(GREEN)Starting backend on http://localhost:8080$(RESET)"
	@cd $(BACKEND_DIR) && go run ./cmd/api

##@ Building

build: build-backend build-frontend ## Build both frontend and backend

build-frontend: ## Build frontend for production
	@echo "$(CYAN)Building frontend...$(RESET)"
	@cd $(FRONTEND_DIR) && npm run build
	@echo "$(GREEN)Frontend build complete$(RESET)"

build-backend: ## Build backend binary
	@echo "$(CYAN)Building backend...$(RESET)"
	@cd $(BACKEND_DIR) && go build -o bin/api ./cmd/api
	@echo "$(GREEN)Backend binary: $(BACKEND_BIN)$(RESET)"

##@ Running

run-backend: $(BACKEND_BIN) ## Run the backend binary
	@echo "$(GREEN)Running backend on http://localhost:$${PORT:-8080}$(RESET)"
	@cd $(BACKEND_DIR) && ./bin/api

start-frontend: ## Start frontend production server
	@echo "$(GREEN)Starting frontend production server...$(RESET)"
	@cd $(FRONTEND_DIR) && npm run start

##@ Dependencies

install: install-backend install-frontend ## Install all dependencies

install-frontend: ## Install frontend dependencies
	@echo "$(CYAN)Installing frontend dependencies...$(RESET)"
	@cd $(FRONTEND_DIR) && npm install
	@echo "$(GREEN)Frontend dependencies installed$(RESET)"

install-backend: ## Download Go modules
	@echo "$(CYAN)Downloading Go modules...$(RESET)"
	@cd $(BACKEND_DIR) && go mod download
	@echo "$(GREEN)Go modules downloaded$(RESET)"

##@ Quality

lint: lint-frontend lint-backend ## Run all linters

lint-frontend: ## Run frontend linter
	@echo "$(CYAN)Linting frontend...$(RESET)"
	@cd $(FRONTEND_DIR) && npm run lint

lint-backend: ## Run backend linter
	@echo "$(CYAN)Linting backend...$(RESET)"
	@cd $(BACKEND_DIR) && go vet ./...

test: test-backend ## Run all tests

test-backend: ## Run backend tests
	@echo "$(CYAN)Running backend tests...$(RESET)"
	@cd $(BACKEND_DIR) && go test -v ./...

##@ Utilities

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(RESET)"
	@rm -rf $(BACKEND_DIR)/bin
	@rm -rf $(FRONTEND_DIR)/.next
	@rm -rf $(FRONTEND_DIR)/out
	@echo "$(GREEN)Clean complete$(RESET)"

check-backend: ## Check if backend is running
	@curl -s http://localhost:8080/health > /dev/null 2>&1 && \
		echo "$(GREEN)Backend is running$(RESET)" || \
		echo "$(YELLOW)Backend is not running$(RESET)"

check-frontend: ## Check if frontend is running
	@curl -s http://localhost:3000 > /dev/null 2>&1 && \
		echo "$(GREEN)Frontend is running$(RESET)" || \
		echo "$(YELLOW)Frontend is not running$(RESET)"

status: check-backend check-frontend ## Check status of both services

##@ Help

help: ## Show this help message
	@echo "$(CYAN)OME Web Console$(RESET)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  $(CYAN)%-18s$(RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(GREEN)%s$(RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
	@echo ""
	@echo "$(YELLOW)Examples:$(RESET)"
	@echo "  make dev              # Start development servers"
	@echo "  make build            # Build for production"
	@echo "  make install          # Install dependencies"
	@echo "  PORT=9090 make run-backend  # Run backend on custom port"

# Ensure backend binary exists before running
$(BACKEND_BIN):
	@$(MAKE) build-backend
