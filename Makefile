# Check Go version and set environment at the start
ifeq ($(shell which go),/opt/go-1.19.13/bin/go)
    export GOROOT := /opt/go-1.23.0
    export PATH := $(GOROOT)/bin:$(PATH)
endif

# Define the directory containing the charts
CHARTS_DIR := ./charts

# Define the registry and image tagging
REGISTRY     ?= ord.ocir.io/idqj093njucb/ome
TAG          ?= $(GIT_TAG)
ARCH         ?= linux/amd64
MANAGER_IMG  ?= $(REGISTRY)/manager:$(TAG)

# Git version and commit information for build
version_pkg = github.com/sgl-project/sgl-ome/pkg/version
GIT_TAG ?= $(shell git describe --tags --dirty --always)
LD_FLAGS += -X '$(version_pkg).GitVersion=$(GIT_TAG)'
LD_FLAGS += -X '$(version_pkg).GitCommit=$(shell git rev-parse HEAD)'

# Get the currently used Golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
	GOBIN=$(shell go env GOPATH)/bin
else
	GOBIN=$(shell go env GOBIN)
endif

# Go command configurations
GO_CMD ?= go
GO_FMT ?= gofmt
# Use go.mod go version as a single source of truth for the Go version
GO_VERSION := $(shell awk '/^go /{print $$2}' go.mod|head -n1)

# Determine Docker build command (use nerdctl if available)
DOCKER_BUILD_CMD = $(shell if command -v nerdctl &> /dev/null; then echo nerdctl; else echo docker; fi)

# CRD Options
CRD_OPTIONS ?= "crd:maxDescLen=0"

# Self-signed CA configuration
OME_ENABLE_SELF_SIGNED_CA ?= false

# ENVTEST K8s version configuration
ENVTEST_K8S_VERSION = 1.27

# Image configuration for success and error scenarios
SUCCESS_200_ISVC_IMG ?= success-200-isvc
ERROR_404_ISVC_IMG ?= error-404-isvc

# Local binary installation path
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# CPU/Memory limits for controller-manager
OME_CONTROLLER_CPU_LIMIT ?= 100m
OME_CONTROLLER_MEMORY_LIMIT ?= 300Mi
$(shell perl -pi -e 's/cpu:.*/cpu: $(OME_CONTROLLER_CPU_LIMIT)/' config/default/manager_resources_patch.yaml)
$(shell perl -pi -e 's/memory:.*/memory: $(OME_CONTROLLER_MEMORY_LIMIT)/' config/default/manager_resources_patch.yaml)

OLD_IMAGE=odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke/go-boringcrypto-4493:go1.24.1-51 AS builder
NEW_IMAGE=odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke-golang-fips:go1.24.1-51 AS builder

# Default to not waiting for controller
WAIT_FOR_CONTROLLER ?= false

.PHONY: all
all: test ## 🎯 Run all tests
	@echo "🎯 Running all tests..."

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

##@ 📚 Documentation
.PHONY: help
help: ## 📖 Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\n📚 Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-25s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

include Makefile-deps.mk

##@ 🛠️  Development

.PHONY: manifests
manifests: controller-gen yq ## 📄 Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	@echo "\n📦 Kubernetes Manifest Generation Starting..."
	
	@echo "\n🔧 Step 1: Generating CRD manifests..."
	@$(CONTROLLER_GEN) $(CRD_OPTIONS) paths=./pkg/apis/ome/... output:crd:dir=config/crd/full
	@echo "✅ CRD manifests generated"
	
	@echo "\n🔑 Step 2: Generating RBAC manifests..."
	@$(CONTROLLER_GEN) rbac:roleName=ome-manager-role paths=./pkg/controller/... output:rbac:artifacts:config=config/rbac
	@echo "✅ RBAC manifests generated"
	
	@echo "\n📝 Step 3: Generating object boilerplate..."
	@$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths=./pkg/apis/ome/v1beta1
	@echo "✅ Object boilerplate generated"

	@echo "\n🔄 Step 4: Applying CRD fixes and modifications..."
	@echo "  • Fixing stored versions..."
	@perl -pi -e 's/storedVersions: null/storedVersions: []/g' config/crd/full/ome.io_inferenceservices.yaml
	@echo "  • Fixing conditions..."
	@perl -pi -e 's/conditions: null/conditions: []/g' config/crd/full/ome.io_inferenceservices.yaml
	@echo "  • Updating type definitions..."
	@perl -pi -e 's/Any/string/g' config/crd/full/ome.io_inferenceservices.yaml
	@echo "  • Updating framework properties..."
	@$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	@echo "  • Optimizing CRD size..."
	@$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.ephemeralContainers)' -i config/crd/full/ome.io_inferenceservices.yaml
	@echo "  • Updating probe configurations..."
	@$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.readinessProbe.properties.httpGet.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	@$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.livenessProbe.properties.httpGet.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	@$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.readinessProbe.properties.tcpSocket.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	@$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.livenessProbe.properties.tcpSocket.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	@$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.containers.items.properties.livenessProbe.properties.httpGet.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	@$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.containers.items.properties.readinessProbe.properties.httpGet.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	@echo "  • Setting protocol defaults..."
	@$(YQ) '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | .. | select(has("protocol")) | path' config/crd/full/ome.io_inferenceservices.yaml -o j | jq -r '. | map(select(numbers)="["+tostring+"]") | join(".")' | awk '{print "."$$0".protocol.default"}' | xargs -n1 -I{} $(YQ) '{} = "TCP"' -i config/crd/full/ome.io_inferenceservices.yaml
	@$(YQ) '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | .. | select(has("protocol")) | path' config/crd/full/ome.io_clusterservingruntimes.yaml -o j | jq -r '. | map(select(numbers)="["+tostring+"]") | join(".")' | awk '{print "."$$0".protocol.default"}' | xargs -n1 -I{} $(YQ) '{} = "TCP"' -i config/crd/full/ome.io_clusterservingruntimes.yaml
	@$(YQ) '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | .. | select(has("protocol")) | path' config/crd/full/ome.io_servingruntimes.yaml -o j | jq -r '. | map(select(numbers)="["+tostring+"]") | join(".")' | awk '{print "."$$0".protocol.default"}' | xargs -n1 -I{} $(YQ) '{} = "TCP"' -i config/crd/full/ome.io_servingruntimes.yaml
	@echo "✅ CRD modifications complete"

	@echo "\n📋 Step 5: Generating minimal CRDs..."
	@./hack/minimal-crdgen.sh
	@echo "✅ Minimal CRDs generated"

	@echo "\n📁 Step 6: Copying manifests to Helm charts..."
	@cp config/crd/full/ome* charts/ome-crd/templates/ && cp config/rbac/role.yaml charts/ome-resources/templates/ome-controller/rbac/role.yaml
	@echo "✅ Manifests copied to Helm charts"

	@echo "\n🎉 Manifest generation completed successfully!\n"

.PHONY: generate
generate: controller-gen ## 🔄 Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations and client-go libraries.
	@echo "\n📦 Code Generation Process Starting..."
	@echo "\n🔧 Step 1: Setting up Go environment..."
	@go env -w GOFLAGS=-mod=mod
	@echo "✅ Go environment configured"
	
	@echo "\n🔄 Step 2: Generating Kubernetes client-go code..."
	@if ! hack/update-codegen.sh 2>generate.err; then \
		echo "❌ Error during code generation:"; \
		cat generate.err; \
		rm generate.err; \
		exit 1; \
	fi
	@rm -f generate.err
	@echo "✅ Client-go code generation complete"
	
	@echo "\n📝 Step 3: Generating OpenAPI specifications..."
	@if ! hack/update-openapigen.sh 2>openapi.err; then \
		echo "❌ Error during OpenAPI generation:"; \
		cat openapi.err; \
		rm openapi.err; \
		exit 1; \
	fi
	@rm -f openapi.err
	@echo "✅ OpenAPI generation complete"
	
	@echo "\n🎉 Code generation process completed successfully!\n"

.PHONY: generate-python-sdk
generate-python-sdk: generate ## 🔄 Generate OME python SDK.
	@echo "\n📋 Step 4: Generating Python SDK..."
	@if ! hack/python-sdk/client-gen.sh 2>pythonclientgen.err; then \
  		echo "❌ Error during Python SDK generation:"; \
		cat pythonclientgen.err; \
		rm pythonclientgen.err; \
		exit 1; \
	fi
	@rm -f pythonclientgen.err
	@echo "\n🎉 PythonSDK generation complete"

.PHONY: fmt
fmt: install-goimports ## 🧹 Run go fmt and goimports against code
	@echo "🧹 Formatting Go code..."
	@$(GO_CMD) fmt ./...
	@echo "🧹 Organizing imports in Go files..."
	@find . -name '*.go' -not -path '*/vendor/*' -not -exec grep -q '// Code generated' {} \; -exec $(GOIMPORTS) -w {} +
	@echo "✅ Formatting complete"

.PHONY: vet
vet: ## 🔍 Run go vet against code
	@echo "🔍 Checking code with go vet..."
	@$(GO_CMD) vet -structtag=false ./...
	@echo "✅ Vet checks passed"

.PHONY: tidy
tidy: ## 📦 Run go mod tidy
	@echo "📦 Tidying Go modules..."
	@$(GO_CMD) mod tidy
	@echo "✅ Dependencies cleaned up"

.PHONY: ci-lint
ci-lint: golangci-lint ## 🔎 Run golangci-lint against code.
	@echo "🔎 Running golangci-lint..."
	$(GOLANGCI_LINT) run --timeout 15m0s
	@echo "✅ Linting complete"

.PHONY: lint-fix
lint-fix: golangci-lint ## 🔧 Run golangci-lint against code and fix linting issues.
	@echo "🔧 Running golangci-lint with auto-fix..."
	$(GOLANGCI_LINT) run --fix --timeout 15m0s
	@echo "✅ Auto-fix complete"

.PHONY: helm-lint
helm-lint: helm ## ⎈ Lint all charts
	@echo "⎈ Linting Helm charts..."
	@for chart in $(CHARTS_DIR)/*/; do \
	  echo "🔍 Linting $$chart..."; \
	  if ! $(HELM) lint $$chart; then \
	    echo "❌ Error: Linting failed for $$chart" >&2; \
	    exit 1; \
	  fi \
	done
	@echo "✅ Helm lint complete"

.PHONY: helm-doc
helm-doc: helm-docs ## 📚 Generate Helm chart documentation via helm-docs
	@echo "📚 Generating Helm documentation..."
	$(HELM_DOCS) --chart-search-root=charts --output-file=README.md
	@echo "✅ Documentation generated"

.PHONY: helm-version-update
helm-version-update: yq ## 🔄 Update Helm chart version
	@echo "🔄 Updating Helm chart versions..."
	@for chart in $(CHARTS_DIR)/*/; do \
		echo "📝 Updating $$chart..."; \
		$(YQ) e -i '.version = "$(GIT_TAG)"' "$${chart}/Chart.yaml"; \
		$(YQ) e -i '.appVersion = "$(GIT_TAG)"' "$${chart}/Chart.yaml"; \
	done
	@echo "✅ Version updates complete"

##@ 🛠️  Build

.PHONY: ome-manager
ome-manager: ## 🏗️  Build ome-manager binary.
	@echo "🏗️  Building ome-manager..."
	$(GO_BUILD_ENV) $(GO_CMD) build -ldflags="$(LD_FLAGS)" -o bin/manager ./cmd/manager
	@echo "✅ Build complete"

.PHONY: model-controller
model-controller: ## 🎮 Build model-controller binary.
	@echo "🎮 Building model-controller..."
	$(GO_BUILD_ENV) $(GO_CMD) build -ldflags="$(LD_FLAGS)" -o bin/model-controller ./cmd/model-controller
	@echo "✅ Build complete"

.PHONY: model-agent
model-agent: ## 🤖 Build model-agent binary.
	@echo "🤖 Building model-agent..."
	$(GO_BUILD_ENV) $(GO_CMD) build -ldflags="$(LD_FLAGS)" -o bin/model-agent ./cmd/model-agent
	@echo "✅ Build complete"

.PHONY: ome-agent
ome-agent: ## 🔄 Build ome-agent binary.
	@echo "🔄 Building ome-agent..."
	$(GO_BUILD_ENV) $(GO_CMD) build -ldflags="$(LD_FLAGS)" -o bin/ome-agent ./cmd/ome-agent
	@echo "✅ Build complete"

.PHONY: multinode-prober
multinode-prober: ## 🔍 Build multinode-prober binary.
	@echo "🔍 Building multinode-prober..."
	$(GO_BUILD_ENV) $(GO_CMD) build -ldflags="$(LD_FLAGS)" -o bin/multinode-prober ./cmd/multinode-prober
	@echo "✅ Build complete"

.PHONY: run-ome-manager
run-ome-manager: manifests generate fmt vet ## Run ome-manager binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	@echo "🏃‍♂️ Running ome-manager..."
	$(GO_BUILD_ENV) $(GO_CMD) run ./cmd/manager/main.go

.PHONY: run-model-controller
run-model-controller: fmt vet ## Run model-controller binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	@echo "🏃‍♂️ Running model-controller..."
	$(GO_BUILD_ENV) $(GO_CMD) run ./cmd/model-controller/main.go

.PHONY: run-model-agent
run-model-agent: fmt vet ## Run model-agent binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	@echo "🏃‍♂️ Running model-agent..."
	$(GO_BUILD_ENV) $(GO_CMD) run ./cmd/model-agent/main.go

.PHONY: run-ome-agent-enigma
run-ome-agent-enigma: fmt vet ome-agent ## Run ome-agent binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	@echo "🏃‍♂️ Running ome-agent enigma..."
	bin/ome-agent enigma -d -c config/ome-agent/ome-agent.yaml

.PHONY: run-ome-agent-hf-download
run-ome-agent-hf-download: fmt vet ome-agent ## Run ome-agent binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	@echo "🏃‍♂️ Running ome-agent hf-download..."
	bin/ome-agent hf-download -d -c config/ome-agent/ome-agent.yaml


.PHONY: run-ome-agent-fine-tuned-adapter
run-ome-agent-fine-tuned-adapter: fmt vet ome-agent ## Run ome-agent binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	@echo "🏃‍♂️ Running ome-agent fine-tuned-adapter..."
	bin/ome-agent fine-tuned-adapter -d -c config/ome-agent/ome-agent.yaml



.PHONY: ome-image
ome-image: fmt vet ## Build ome-manager image.
	@echo "🚀 Building ome-manager image..."
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/manager.Dockerfile -t $(MANAGER_IMG)
	@echo "✅ Image built"

.PHONY: model-controller-image
model-controller-image: fmt vet ## Build model-controller image.
	@echo "🚀 Building model-controller image..."
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/model-controller.Dockerfile -t $(REGISTRY)/model-controller:$(TAG)
	@echo "✅ Image built"

.PHONY: model-agent-image
model-agent-image: fmt vet ## Build model-agent image.
	@echo "🚀 Building model-agent image..."
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/model-agent.Dockerfile -t $(REGISTRY)/model-agent:$(TAG)
	@echo "✅ Image built"

.PHONY: multinode-prober-image
multinode-prober-image: fmt vet ## Build multinode-prober image.
	@echo "🚀 Building multinode-prober image..."
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/multinode-prober.Dockerfile -t $(REGISTRY)/multinode-prober:$(TAG)
	@echo "✅ Image built"

.PHONY: ome-agent-image
ome-agent-image: fmt vet ## Build ome-agent image.
	@echo "🚀 Building ome-agent image..."
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/ome-agent.Dockerfile -t $(REGISTRY)/ome-agent:$(TAG)
	@echo "✅ Image built"


.PHONY: telepresence
telepresence: ## 🌐 Setup telepresence
	@echo "🌐 Configuring Telepresence for local development..."
	@hack/telepresence-setup.sh
	@echo "✅ Telepresence ready - happy coding!"

##@ 🚀 Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: delete-webhooks
delete-webhooks: ## 🧹 Delete validation/mutation webhook configurations
	@echo "🧹 Deleting ValidatingWebhookConfigurations..."
	@$(YQ) eval 'select(.kind == "ValidatingWebhookConfiguration").metadata.name' config/webhook/manifests.yaml | \
		grep -v '^---$$' | \
		xargs -I{} sh -c 'if kubectl delete validatingwebhookconfigurations.admissionregistration.k8s.io {} 2>/dev/null; then echo "✅ Successfully deleted {}"; else echo "⚠️  Not found - {}"; fi'
	@echo "\n🧹 Deleting MutatingWebhookConfigurations..."
	@$(YQ) eval 'select(.kind == "MutatingWebhookConfiguration").metadata.name' config/webhook/manifests.yaml | \
		grep -v '^---$$' | \
		xargs -I{} sh -c 'if kubectl delete mutatingwebhookconfigurations.admissionregistration.k8s.io {} 2>/dev/null; then echo "✅ Successfully deleted {}"; else echo "⚠️  Not found - {}"; fi'
	@echo "\nWebhook cleanup completed with clear status!"

.PHONY: install
install: kustomize ## 🚀 Deploy controller in the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	@echo "\n📦 OME Deployment Process Starting..."
	@echo "Current KUBECONFIG: $(KUBECONFIG)"
	@echo "## 💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥 ##"
	@echo "Are you really sure you want to completely re-install in [$(value KUBECONFIG)] environment ?"
	@echo "## 💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥 ##"
	@read -p "Press enter to continue"
	
	@echo "\n🔧 Step 1: Configuring certificates..."
	@cd config/default && if [ ${OME_ENABLE_SELF_SIGNED_CA} != false ]; then \
		echo "  • Using self-signed CA"; \
		echo > ../certmanager/certificate.yaml; \
	else \
		echo "  • Using certmanager certificate"; \
		git checkout HEAD -- ../certmanager/certificate.yaml; \
	fi
	@echo "✅ Certificate configuration complete"
	
	@echo "\n🚀 Step 2: Deploying OME components..."
	@echo "  • Applying kustomize configuration..."
	kubectl apply --server-side --force-conflicts -k config/default
	
	@if [ ${OME_ENABLE_SELF_SIGNED_CA} != false ]; then \
		echo "  • Setting up self-signed CA..."; \
		./hack/self-signed-ca.sh; \
	fi
	
	@if [ ${WAIT_FOR_CONTROLLER} = true ]; then \
		echo "\n⏳ Step 3: Waiting for OME controller to be ready..."; \
		kubectl wait --for=condition=ready pod -l control-plane=ome-controller-manager -n ome --timeout=300s; \
	fi
	
	@echo "\n🔄 Step 4: Applying cluster resources..."
	kubectl apply --server-side --force-conflicts -k config/clusterresources
	
	@echo "\n🧹 Step 5: Cleanup..."
	@git checkout HEAD -- config/certmanager/certificate.yaml
	@echo "✅ Cleanup complete"
	
	@echo "\n🎉 OME deployment completed successfully!\n"

.PHONY: uninstall
uninstall: kustomize ## 🧹 Uninstall controller from the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	@echo "Current KUBECONFIG: $(KUBECONFIG)"
	@echo "## 💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥 ##"
	@echo "Are you really sure you want to completely destroy [$(value KUBECONFIG)] environment ?"
	@echo "## 💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥💥 ##"
	@read -p "Press enter to continue"
	kubectl delete --ignore-not-found=$(ignore-not-found) -k config/default
	kubectl delete --ignore-not-found=$(ignore-not-found) -k config/clusterresources
	@echo "✅ Controller uninstalled"

.PHONY: kustomize-validate
kustomize-validate: kustomize ## 🔍 Validate kustomize configuration without applying to cluster
	@echo "\n🔍 Validating kustomize configuration..."
	@cd config/default && $(KUSTOMIZE) build --load-restrictor=LoadRestrictionsNone . > /dev/null && echo "✅ Default configuration is valid" || (echo "❌ Error in default configuration" && exit 1)
	@cd config/clusterresources && $(KUSTOMIZE) build --load-restrictor=LoadRestrictionsNone . > /dev/null && echo "✅ Cluster resources configuration is valid" || (echo "❌ Error in cluster resources configuration" && exit 1)
	@echo "\n✅ Kustomize validation completed successfully!\n"

.PHONY: push-manager-image
push-manager-image: ome-image ## Push manager image to registry.
	@echo "🚀 Pushing manager image to registry..."
	$(DOCKER_BUILD_CMD) push $(MANAGER_IMG)
	@echo "✅ Image pushed"

.PHONY: push-model-controller-image
push-model-controller-image: model-controller-image ## Push model-controller image to registry.
	@echo "🚀 Pushing model-controller image to registry..."
	$(DOCKER_BUILD_CMD) push $(REGISTRY)/model-controller:$(TAG)
	@echo "✅ Image pushed"

.PHONY: push-model-agent-image
push-model-agent-image: model-agent-image ## Push model-agent image to registry.
	@echo "🚀 Pushing model-agent image to registry..."
	$(DOCKER_BUILD_CMD) push $(REGISTRY)/model-agent:$(TAG)
	@echo "✅ Image pushed"

.PHONY: push-multinode-prober-image
push-multinode-prober-image: multinode-prober-image ## Push multinode-prober image to registry.
	@echo "🚀 Pushing multinode-prober image to registry..."
	$(DOCKER_BUILD_CMD) push $(REGISTRY)/multinode-prober:$(TAG)
	@echo "✅ Image pushed"

.PHONY: push-ome-agent-image
push-ome-agent-image: ome-agent-image ## Push ome-agent image to registry.
	@echo "🚀 Pushing ome-agent image to registry..."
	$(DOCKER_BUILD_CMD) push $(REGISTRY)/ome-agent:$(TAG)
	@echo "✅ Image pushed"

.PHONY: patch-manager-dev
patch-manager-dev: push-manager-image ## Deploy manager image to dev cluster.
	@echo "🔄 Patching manager image to dev cluster..."
	echo "Patch manager image to dev: $(MANAGER_IMG)"
	./hack/patch_image_dev.sh $(MANAGER_IMG) manager
	@echo "✅ Patch complete"

.PHONY: patch-model-controller-dev
patch-model-controller-dev: push-model-controller-image ## Deploy model-controller image to dev cluster.
	@echo "🔄 Patching model-controller image to dev cluster..."
	echo "Patch model-controller image to dev: $(REGISTRY)/model-controller:$(TAG)"
	./hack/patch_image_dev.sh $(REGISTRY)/model-controller:$(TAG) model_controller
	@echo "✅ Patch complete"

.PHONY: patch-model-agent-dev
patch-model-agent-dev: push-model-agent-image ## Deploy model-agent image to dev cluster.
	@echo "🔄 Patching model-agent image to dev cluster..."
	echo "Patch model-agent image to dev: $(REGISTRY)/model-agent:$(TAG)"
	./hack/patch_image_dev.sh $(REGISTRY)/model-agent:$(TAG) model_agent
	@echo "✅ Patch complete"

.PHONY: deploy-helm
deploy-helm: manifests helm ## Deploy OME using Helm
	@echo "🚀 Deploying OME using Helm..."
	helm install ome-crd charts/ome-crd/ --wait --timeout 180s
	helm install ome charts/ome-resources/ --wait --timeout 180s
	@echo "✅ Deployment complete"

.PHONY: artifacts
artifacts: kustomize ## Generate artifacts for release.
	@echo "📦 Generating artifacts..."
	$(KUSTOMIZE) build config/default -o artifacts/manifests.yaml
	$(KUSTOMIZE) build config/clusterresources -o artifacts/clusterresources.yaml
	@echo "✅ Artifacts generated"

.PHONY: integration-test
integration-test: fmt vet manifests envtest ## 🧪 Run integration tests
	@echo "🧪 Running integration tests..."
	go test -v ./integration_tests/... -ginkgo.v -ginkgo.trace
	@echo "✅ Integration tests passed"

##@ 🧪 Testing
.PHONY: test
test: test-cmd test-pkg test-internal ## 🧪 Run all tests

.PHONY: test-cmd
test-cmd: fmt vet manifests envtest ## 🧪 Run cmd tests with coverage
	@echo "🧪 Running command tests..."
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO_CMD) test \
		./cmd/... \
		-coverprofile=coverage-cmd.out.tmp \
		--covermode=atomic
	@echo "🔍 Filtering coverage report..."
	@cat coverage-cmd.out.tmp | grep -v "pkg/testing/" | grep -v "pkg/testutils/" | grep -v "_generated.go" | grep -v "zz_generated" > coverage-cmd.out
	@rm coverage-cmd.out.tmp
	@echo "✅ Command tests passed"

.PHONY: test-pkg
test-pkg: fmt vet manifests envtest ## 🧪 Run pkg tests with coverage
	@echo "🧪 Running package tests..."
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO_CMD) test \
		$$(go list ./pkg/... | grep -v ./pkg/apis |grep -v ./pkg/client | grep -v ./pkg/openapi/openapi_generated.go | grep -v ./pkg/apis/ome/v1beta1/zz_generated.deepcopy.go | grep -v ./pkg/testing) \
		-coverprofile=coverage-pkg.out.tmp \
		--covermode=atomic
	@echo "🔍 Filtering coverage report..."
	@cat coverage-pkg.out.tmp | grep -v "/pkg/apis/" | grep -v "pkg/testing/" | grep -v "_generated.go" | grep -v "zz_generated" > coverage-pkg.out
	@rm coverage-pkg.out.tmp
	@echo "✅ Package tests passed"

.PHONY: test-internal
test-internal: fmt vet manifests envtest ## 🧪 Run internal tests with coverage
	@echo "🧪 Running internal tests..."
	@KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO_CMD) test \
		./internal/... \
		-coverprofile=coverage-internal.out.tmp \
		--covermode=atomic
	@echo "🔍 Filtering coverage report..."
	@cat coverage-internal.out.tmp | grep -v "pkg/testing/" | grep -v "_generated.go" | grep -v "zz_generated" > coverage-internal.out
	@rm coverage-internal.out.tmp
	@echo "✅ Internal tests passed"

.PHONY: coverage
coverage: ## Show coverage for all packages
	@echo "\n---------- Coverage Summary ----------"
	@echo "CMD Coverage:"
	@go tool cover -func=coverage-cmd.out | grep -v "100.0%"
	@echo "\nPKG Coverage:"
	@go tool cover -func=coverage-pkg.out | grep -v "100.0%"
	@echo "\nInternal Coverage:"
	@go tool cover -func=coverage-internal.out | grep -v "100.0%"
	@echo "\nTotal Coverage:"
	@cmd_cov=$$(go tool cover -func=coverage-cmd.out | grep total | awk '{sub(/%/,"",$$3); print $$3}'); \
	pkg_cov=$$(go tool cover -func=coverage-pkg.out | grep total | awk '{sub(/%/,"",$$3); print $$3}'); \
	int_cov=$$(go tool cover -func=coverage-internal.out | grep total | awk '{sub(/%/,"",$$3); print $$3}'); \
	echo "CMD: $$cmd_cov%"; \
	echo "PKG: $$pkg_cov%"; \
	echo "Internal: $$int_cov%"; \
	avg_cov=$$(awk "BEGIN {printf \"%.2f\", ($$cmd_cov + $$pkg_cov + $$int_cov) / 3}"); \
	echo "\nAverage Coverage: $$avg_cov%"; \
	if awk "BEGIN {exit !($$avg_cov < 23)}"; then \
		echo "Average coverage $$avg_cov% is below minimum threshold of 23%"; \
		exit 1; \
	fi

.PHONY: update-go-base-image
update-go-base-image: ## Update the go base image in all dockerfiles
	@echo "🔄 Updating go base image..."
	@find . -type f -name "*Dockerfile" | xargs sed -i '' "s|${OLD_IMAGE}|${NEW_IMAGE}|g"
	@echo "✅ Update complete"
