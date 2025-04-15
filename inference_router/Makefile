.PHONY: all build test clean run mockservers run-test test-router run-and-test help

# Define variables
DOCKER_BUILD_CMD = $(shell if command -v nerdctl &> /dev/null; then echo nerdctl; else echo docker; fi)
REGISTRY     ?= ord.ocir.io/idqj093njucb/ome
TAG          ?= $(GIT_TAG)
ARCH         ?= linux/amd64
ROUTER_IMG   ?= $(REGISTRY)/inference-router:$(TAG)

# Git version and commit information
GIT_TAG ?= $(shell git describe --tags --dirty --always)
LD_FLAGS += -X 'main.GitVersion=$(GIT_TAG)'
LD_FLAGS += -X 'main.GitCommit=$(shell git rev-parse HEAD)'

##@ 🎯 General

# Build the inference router
all: build ## 🎯 Build the inference router

##@ 📚 Documentation

.PHONY: help
help: ## 📖 Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n📚 Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ 🛠️ Development

build: ## 🏗️ Build the binary
	@echo "🏗️ Building inference_router..."
	@cargo build
	@echo "✅ Build complete"

clean: ## 🧹 Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@cargo clean
	@echo "✅ Clean complete"

fmt: ## 🧹 Format Rust code
	@echo "🧹 Formatting Rust code..."
	@cargo fmt
	@echo "✅ Format complete"

lint: ## 🔎 Run clippy against code
	@echo "🔎 Running clippy on codebase..."
	@cargo clippy -- -D warnings
	@echo "✅ Lint complete"

test: ## 🧪 Run tests
	@echo "🧪 Running tests..."
	@cargo test
	@echo "✅ Tests complete"

##@ 🚀 Running

run: ## 🏃‍♂️ Run the inference router
	@echo "🏃‍♂️ Running inference_router..."
	@cargo run
	@echo "✅ Application started"

mockservers: ## 🔄 Start mock servers for integration testing
	@echo "🔄 Starting mock servers for integration testing..."
	@cargo run --example mock_servers
	@echo "✅ Mock servers started"

run-test: ## 🚀 Run the router with test configuration
	@echo "🚀 Running router with test configuration..."
	@./scripts/run_test_router.sh
	@echo "✅ Test router running"

test-router: ## 🧪 Test the router with curl commands
	@echo "🧪 Testing router with curl commands..."
	@./scripts/test_router.sh
	@echo "✅ Router tests complete"

run-and-test: ## 🔄 Run the router and test in a single command
	@echo "🔄 Running router and tests in a single command..."
	@./scripts/run_and_test.sh
	@echo "✅ Run and test complete"

validate: ## ✅ Validate a routing configuration
	@echo "✅ Validating router configuration..."
	@cargo run -- validate --graph-json examples/complex-graph.json --verbose
	@echo "✅ Validation complete"

##@ 🐳 Docker

docker-build: ## 🐳 Build Docker image
	@echo "🐳 Building Docker image..."
	@$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -t $(ROUTER_IMG)
	@echo "✅ Image built: $(ROUTER_IMG)"

docker-push: ## 🚀 Push Docker image to registry
	@echo "🚀 Pushing Docker image to registry..."
	@$(DOCKER_BUILD_CMD) push $(ROUTER_IMG)
	@echo "✅ Image pushed: $(ROUTER_IMG)"

##@ 📦 Installation

install: ## 📦 Install the inference router
	@echo "📦 Installing inference_router..."
	@cargo install --path .
	@echo "✅ Installation complete"

##@ 🧪 Testing Examples

example-simple: ## 🔍 Run simple router example
	@echo "🔍 Running simple router example..."
	@cargo run -- --graph-json examples/simple-graph.json
	@echo "✅ Example complete"

example-complex: ## 🔄 Run complex router example
	@echo "🔄 Running complex router example..."
	@cargo run -- --graph-json examples/complex-graph.json
	@echo "✅ Example complete"

example-splitter: ## 🔀 Run splitter router example
	@echo "🔀 Running splitter router example..."
	@cargo run -- --graph-json examples/splitter-graph.json
	@echo "✅ Example complete" 