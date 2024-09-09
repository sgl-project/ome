PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
BIN_DIR ?= $(PROJECT_DIR)/bin
TOOLS_DIR := $(PROJECT_DIR)/hack/internal/tools

ifeq (,$(shell go env GOBIN))
	GOBIN=$(shell go env GOPATH)/bin
else
	GOBIN=$(shell go env GOBIN)
endif
GO_CMD ?= go

# Use go.mod go version as source.
GOLANGCI_LINT_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' github.com/golangci/golangci-lint)
CONTROLLER_GEN_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' sigs.k8s.io/controller-tools)
KUSTOMIZE_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' sigs.k8s.io/kustomize/kustomize/v4)
ENVTEST_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' sigs.k8s.io/controller-runtime/tools/setup-envtest)
KIND_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' sigs.k8s.io/kind)
YQ_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' github.com/mikefarah/yq/v4)
HELM_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' helm.sh/helm/v3)
HELM_DOCS_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' github.com/norwoodj/helm-docs)
HUGO_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' github.com/gohugoio/hugo)
MDTOC_VERSION ?= $(shell cd $(TOOLS_DIR); $(GO_CMD) list -m -f '{{.Version}}' sigs.k8s.io/mdtoc)



##@ Tools

GOLANGCI_LINT = $(PROJECT_DIR)/bin/golangci-lint
.PHONY: golangci-lint
golangci-lint: ## Download golangci-lint locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

CONTROLLER_GEN = $(PROJECT_DIR)/bin/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

KUSTOMIZE = $(PROJECT_DIR)/bin/kustomize
.PHONY: kustomize
kustomize: ## Download kustomize locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install sigs.k8s.io/kustomize/kustomize/v4@$(KUSTOMIZE_VERSION)

ENVTEST = $(PROJECT_DIR)/bin/setup-envtest
.PHONY: envtest
envtest: ## Download envtest-setup locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)


KIND = $(PROJECT_DIR)/bin/kind
.PHONY: kind
kind: ## Download kind locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install sigs.k8s.io/kind@$(KIND_VERSION)

YQ = $(PROJECT_DIR)/bin/yq
.PHONY: yq
yq: ## Download yq locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install github.com/mikefarah/yq/v4@$(YQ_VERSION)

HELM = $(PROJECT_DIR)/bin/helm
.PHONY: helm
helm: ## Download helm locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install helm.sh/helm/v3/cmd/helm@$(HELM_VERSION)


HELM_DOCS = $(PROJECT_DIR)/bin/helm-docs
.PHONY: helm-document
helm-document: ## Download helm-docs locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin GO111MODULE=on $(GO_CMD) install github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELM_DOCS_VERSION)


HUGO = $(PROJECT_DIR)/bin/hugo
.PHONY: hugo
hugo: ## Download hugo locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin CGO_ENABLED=1 $(GO_CMD) install -tags extended github.com/gohugoio/hugo@$(HUGO_VERSION)

MDTOC = $(PROJECT_DIR)/bin/mdtoc
.PHONY: mdtoc
mdtoc: ## Download mdtoc locally if necessary.
	@GOBIN=$(PROJECT_DIR)/bin CGO_ENABLED=1 $(GO_CMD) install sigs.k8s.io/mdtoc@$(MDTOC_VERSION)