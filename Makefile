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
version_pkg = bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/version
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

OLD_IMAGE=odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke/go-boringcrypto-4493:1.23.3-30 AS builder
NEW_IMAGE=odo-docker-signed-local.artifactory.oci.oraclecorp.com/oke/go-boringcrypto-4493:go1.23.3-30 AS builder

.PHONY: all
all: test

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

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

include Makefile-deps.mk

##@ Development

.PHONY: manifests
manifests: controller-gen yq ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths=./pkg/apis/ome/... output:crd:dir=config/crd/full
	$(CONTROLLER_GEN) rbac:roleName=ome-manager-role paths=./pkg/controller/... output:rbac:artifacts:config=config/rbac

	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths=./pkg/apis/ome/v1beta1

	#TODO Remove this until new controller-tools is released
	perl -pi -e 's/storedVersions: null/storedVersions: []/g' config/crd/full/ome.io_inferenceservices.yaml
	perl -pi -e 's/conditions: null/conditions: []/g' config/crd/full/ome.io_inferenceservices.yaml
	perl -pi -e 's/Any/string/g' config/crd/full/ome.io_inferenceservices.yaml
	#remove the required property on framework as name field needs to be optional
	$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	#remove ephemeralContainers properties for compress crd size https://github.com/kubeflow/kfserving/pull/1141#issuecomment-714170602
	$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.ephemeralContainers)' -i config/crd/full/ome.io_inferenceservices.yaml
	#knative does not allow setting port on liveness or readiness probe
	$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.readinessProbe.properties.httpGet.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.livenessProbe.properties.httpGet.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.readinessProbe.properties.tcpSocket.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.livenessProbe.properties.tcpSocket.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.containers.items.properties.livenessProbe.properties.httpGet.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	$(YQ) 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.containers.items.properties.readinessProbe.properties.httpGet.required)' -i config/crd/full/ome.io_inferenceservices.yaml
	#With v1 and newer kubernetes protocol requires default
	$(YQ) '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | .. | select(has("protocol")) | path' config/crd/full/ome.io_inferenceservices.yaml -o j | jq -r '. | map(select(numbers)="["+tostring+"]") | join(".")' | awk '{print "."$$0".protocol.default"}' | xargs -n1 -I{} $(YQ) '{} = "TCP"' -i config/crd/full/ome.io_inferenceservices.yaml
	$(YQ) '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | .. | select(has("protocol")) | path' config/crd/full/ome.io_clusterservingruntimes.yaml -o j | jq -r '. | map(select(numbers)="["+tostring+"]") | join(".")' | awk '{print "."$$0".protocol.default"}' | xargs -n1 -I{} $(YQ) '{} = "TCP"' -i config/crd/full/ome.io_clusterservingruntimes.yaml
	$(YQ) '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | .. | select(has("protocol")) | path' config/crd/full/ome.io_servingruntimes.yaml -o j | jq -r '. | map(select(numbers)="["+tostring+"]") | join(".")' | awk '{print "."$$0".protocol.default"}' | xargs -n1 -I{} $(YQ) '{} = "TCP"' -i config/crd/full/ome.io_servingruntimes.yaml
	./hack/minimal-crdgen.sh
	cp config/crd/full/ome* charts/ome-crd/templates/ && cp config/rbac/role.yaml charts/ome-resources/templates/ome-controller/rbac/role.yaml

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations and client-go libraries.
	go env -w GOFLAGS=-mod=mod
	hack/update-codegen.sh
	hack/update-openapigen.sh

.PHONY: fmt
fmt: ## Run go fmt against code.
	$(GO_CMD) fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	$(GO_CMD) vet ./...

.PHONY: tidy
tidy: ## Run go mod tidy.
	$(GO_CMD) mod tidy

.PHONY: ci-lint
ci-lint: golangci-lint ## Run golangci-lint against code.
	$(GOLANGCI_LINT) run --timeout 15m0s

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint against code and fix linting issues.
	$(GOLANGCI_LINT) run --fix --timeout 15m0s

.PHONY: helm-lint
helm-lint: helm ## Lint all charts
	@for chart in $(CHARTS_DIR)/*/; do \
	  echo "Linting $$chart..."; \
	  if ! $(HELM) lint $$chart; then \
	    echo "Error: Linting failed for $$chart" >&2; \
	    exit 1; \
	  fi \
	done

.PHONY: helm-doc
helm-doc: helm-docs ## Generate Helm chart documentation via helm-docs
	$(HELM_DOCS) --chart-search-root=charts --output-file=README.md


.PHONY: helm-version-update
helm-version-update: yq ## Update Helm chart version
	@for chart in $(CHARTS_DIR)/*/; do \
		echo "Updating $$chart..."; \
		$(YQ) e -i '.version = "$(GIT_TAG)"' "$${chart}/Chart.yaml"; \
		$(YQ) e -i '.appVersion = "$(GIT_TAG)"' "$${chart}/Chart.yaml"; \
	done

##@ Build

.PHONY: ome-manager
ome-manager: ## Build ome-manager binary.
	$(GO_BUILD_ENV) $(GO_CMD) build -ldflags="$(LD_FLAGS)" -o bin/manager ./cmd/manager

.PHONY: model-controller
model-controller: ## Build model-controller binary.
	$(GO_BUILD_ENV) $(GO_CMD) build -ldflags="$(LD_FLAGS)" -o bin/model-controller ./cmd/model-controller

.PHONY: model-agent
model-agent: ## Build model-agent binary.
	$(GO_BUILD_ENV) $(GO_CMD) build -ldflags="$(LD_FLAGS)" -o bin/model-agent ./cmd/model-agent

.PHONY: ome-agent
ome-agent: ## Build ome-agent binary.
	$(GO_BUILD_ENV) $(GO_CMD) build -ldflags="$(LD_FLAGS)" -o bin/ome-agent ./cmd/ome-agent

.PHONY: multinode-prober
multinode-prober: ## Build multinode-prober binary.
	$(GO_BUILD_ENV) $(GO_CMD) build -ldflags="$(LD_FLAGS)" -o bin/multinode-prober ./cmd/multinode-prober

.PHONY: run-ome-manager
run-ome-manager: manifests generate fmt vet ## Run ome-manager binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	$(GO_BUILD_ENV) $(GO_CMD) run ./cmd/manager/main.go

.PHONY: run-model-controller
run-model-controller: fmt vet ## Run model-controller binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	$(GO_BUILD_ENV) $(GO_CMD) run ./cmd/model-controller/main.go

.PHONY: run-model-agent
run-model-agent: fmt vet ## Run model-agent binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	$(GO_BUILD_ENV) $(GO_CMD) run ./cmd/model-agent/main.go

.PHONY: run-multinode-prober
run-multinode-prober: manifests generate fmt vet ## Run multinode-prober binary from local host against the configured vLLM endpoint.
	$(GO_BUILD_ENV) $(GO_CMD) run ./cmd/multinode-prober/main.go

.PHONY: run-ome-agent-enigma
run-ome-agent-enigma: fmt vet ome-agent ## Run ome-agent binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	bin/ome-agent enigma -d -c config/ome-agent/ome-agent.yaml

.PHONY: run-ome-agent-hf-download
run-ome-agent-hf-download: fmt vet ome-agent ## Run ome-agent binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	bin/ome-agent hf-download -d -c config/ome-agent/ome-agent.yaml

.PHONY: run-ome-agent-os-replica
run-ome-agent-os-replica: fmt vet ome-agent ## Run ome-agent binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	bin/ome-agent replica -d -c config/ome-agent/ome-agent.yaml

.PHONY: run-ome-agent-training-agent
run-ome-agent-training-agent: fmt vet ome-agent ## Run ome-agent binary from local host against the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	bin/ome-agent training-agent -d -c config/ome-agent/ome-agent.yaml

.PHONY: ome-image
ome-image: fmt vet ## Build ome-manager image.
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/manager.Dockerfile -t $(MANAGER_IMG)

.PHONY: model-controller-image
model-controller-image: fmt vet ## Build model-controller image.
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/model-controller.Dockerfile -t $(REGISTRY)/model-controller:$(TAG)

.PHONY: model-agent-image
model-agent-image: fmt vet ## Build model-agent image.
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/model-agent.Dockerfile -t $(REGISTRY)/model-agent:$(TAG)

.PHONY: multinode-prober-image
multinode-prober-image: fmt vet ## Build multinode-prober image.
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/multinode-prober.Dockerfile -t $(REGISTRY)/multinode-prober:$(TAG)

.PHONY: ome-agent-image
ome-agent-image: fmt vet ## Build ome-agent image.
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/ome-agent.Dockerfile -t $(REGISTRY)/ome-agent:$(TAG)

.PHONY: telepresence
telepresence: ## Setup telepresence for local development.
	hack/telepresence-setup.sh

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: kustomize ## Deploy controller in the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	# Remove the certmanager certificate if OME_ENABLE_SELF_SIGNED_CA is not false
	cd config/default && if [ ${OME_ENABLE_SELF_SIGNED_CA} != false ]; then \
	echo > ../certmanager/certificate.yaml; \
	else git checkout HEAD -- ../certmanager/certificate.yaml; fi;
	kubectl apply -k config/default
	if [ ${OME_ENABLE_SELF_SIGNED_CA} != false ]; then ./hack/self-signed-ca.sh; fi;
	kubectl wait --for=condition=ready pod -l control-plane=ome-controller-manager -n ome --timeout=300s
	kubectl apply -k config/clusterresources
	git checkout HEAD -- config/certmanager/certificate.yaml

.PHONY: uninstall
uninstall: kustomize ## Uninstall controller from the configured Kubernetes cluster in ~/.kube/config or KUBECONFIG env.
	kubectl delete --ignore-not-found=$(ignore-not-found) -k config/default
	kubectl delete --ignore-not-found=$(ignore-not-found) -k config/clusterresources

.PHONY: push-manager-image
push-manager-image: ome-image ## Push manager image to registry.
	$(DOCKER_BUILD_CMD) push $(MANAGER_IMG)

.PHONY: push-model-controller-image
push-model-controller-image: model-controller-image ## Push model-controller image to registry.
	$(DOCKER_BUILD_CMD) push $(REGISTRY)/model-controller:$(TAG)

.PHONY: push-model-agent-image
push-model-agent-image: model-agent-image ## Push model-agent image to registry.
	$(DOCKER_BUILD_CMD) push $(REGISTRY)/model-agent:$(TAG)

.PHONY: push-multinode-prober-image
push-multinode-prober-image: multinode-prober-image ## Push multinode-prober image to registry.
	$(DOCKER_BUILD_CMD) push $(REGISTRY)/multinode-prober:$(TAG)

.PHONY: push-ome-agent-image
push-ome-agent-image: ome-agent-image ## Push ome-agent image to registry.
	$(DOCKER_BUILD_CMD) push $(REGISTRY)/ome-agent:$(TAG)

.PHONY: patch-manager-dev
patch-manager-dev: push-manager-image ## Deploy manager image to dev cluster.
	echo "Patch manager image to dev: $(MANAGER_IMG)"
	./hack/patch_image_dev.sh $(MANAGER_IMG) manager

.PHONY: patch-model-controller-dev
patch-model-controller-dev: push-model-controller-image ## Deploy model-controller image to dev cluster.
	echo "Patch model-controller image to dev: $(REGISTRY)/model-controller:$(TAG)"
	./hack/patch_image_dev.sh $(REGISTRY)/model-controller:$(TAG) model_controller

.PHONY: patch-model-agent-dev
patch-model-agent-dev: push-model-agent-image ## Deploy model-agent image to dev cluster.
	echo "Patch model-agent image to dev: $(REGISTRY)/model-agent:$(TAG)"
	./hack/patch_image_dev.sh $(REGISTRY)/model-agent:$(TAG) model_agent

.PHONY: deploy-helm
deploy-helm: manifests helm ## Deploy OME using Helm
	helm install ome-crd charts/ome-crd/ --wait --timeout 180s
	helm install ome charts/ome-resources/ --wait --timeout 180s

.PHONY: artifacts
artifacts: kustomize ## Generate artifacts for release.
	$(KUSTOMIZE) build config/default -o artifacts/manifests.yaml
	$(KUSTOMIZE) build config/clusterresources -o artifacts/clusterresources.yaml

##@ Test
.PHONY: test
test: test-cmd test-pkg test-internal ## Run all tests

.PHONY: test-cmd
test-cmd: fmt vet manifests envtest ## Run cmd tests with coverage
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO_CMD) test \
		./cmd/... \
		-coverprofile=coverage-cmd.out \
		-cover \
		-covermode=atomic

.PHONY: test-pkg
test-pkg: fmt vet manifests envtest ## Run pkg tests with coverage
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO_CMD) test \
		$$(go list ./pkg/... | grep -v ./pkg/client | grep -v ./pkg/openapi/openapi_generated.go | grep -v ./pkg/apis/ome/v1beta1/zz_generated.deepcopy.go) \
		-coverprofile=coverage-pkg.out \
		-cover \
		-covermode=atomic

.PHONY: test-internal
test-internal: fmt vet manifests envtest ## Run internal tests with coverage
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" $(GO_CMD) test \
		./internal/... \
		-coverprofile=coverage-internal.out \
		-cover \
		-covermode=atomic

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
	if awk "BEGIN {exit !($$avg_cov < 17)}"; then \
		echo "Average coverage $$avg_cov% is below minimum threshold of 17%"; \
		exit 1; \
	fi

.PHONY: update-go-base-image
update-go-base-image: ## Update the go base image in all dockerfiles
	@find . -type f -name "*Dockerfile" | xargs sed -i '' "s|${OLD_IMAGE}|${NEW_IMAGE}|g"
