GIT_COMMIT     := $(shell git rev-parse HEAD)
GIT_TREE_STATE := $(shell test -n "`git status --porcelain`" && echo "-dirty" || echo "")

REGISTRY     ?= ord.ocir.io/idqj093njucb/ome
TAG          ?= sha-$(GIT_COMMIT)$(GIT_TREE_STATE)
ARCH         ?= linux/amd64
MANAGER_IMG  ?= $(REGISTRY)/manager:$(TAG)


CRD_OPTIONS ?= "crd:maxDescLen=0"
OME_ENABLE_SELF_SIGNED_CA ?= false
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.27
SUCCESS_200_ISVC_IMG ?= success-200-isvc
ERROR_404_ISVC_IMG ?= error-404-isvc

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

DOCKER_BUILD_CMD = $(shell if command -v nerdctl &> /dev/null; then echo nerdctl; else echo docker; fi)

## Tool Binaries
ENVTEST ?= $(LOCALBIN)/setup-envtest
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.15.0

# CPU/Memory limits for controller-manager
OME_CONTROLLER_CPU_LIMIT ?= 100m
OME_CONTROLLER_MEMORY_LIMIT ?= 300Mi
$(shell perl -pi -e 's/cpu:.*/cpu: $(OME_CONTROLLER_CPU_LIMIT)/' config/default/manager_resources_patch.yaml)
$(shell perl -pi -e 's/memory:.*/memory: $(OME_CONTROLLER_MEMORY_LIMIT)/' config/default/manager_resources_patch.yaml)

all: test manager

# Run tests
test: fmt vet manifests envtest
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test $$(go list ./pkg/...) ./cmd/... -coverprofile coverage.out -coverpkg ./pkg/... ./cmd...

# Build manager binary
manager: generate fmt vet go-lint
	go build -o bin/manager ./cmd/manager


# Build manager container image
manager-image: fmt vet
	$(DOCKER_BUILD_CMD) build --platform=$(ARCH) . -f dockerfiles/manager.Dockerfile -t $(MANAGER_IMG)


push-manager-image: manager-image
	$(DOCKER_BUILD_CMD) push $(MANAGER_IMG)


deploy-manager-dev: push-manager-image
	echo "Deploying manager image to dev: $(MANAGER_IMG)"
	./hack/image_patch_dev.sh $(MANAGER_IMG)

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet go-lint
	go run ./cmd/manager/main.go

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	# Remove the certmanager certificate if OME_ENABLE_SELF_SIGNED_CA is not false
	cd config/default && if [ ${OME_ENABLE_SELF_SIGNED_CA} != false ]; then \
	echo > ../certmanager/certificate.yaml; \
	else git checkout HEAD -- ../certmanager/certificate.yaml; fi;
	kubectl apply -k config/default
	if [ ${OME_ENABLE_SELF_SIGNED_CA} != false ]; then ./hack/self-signed-ca.sh; fi;
	kubectl wait --for=condition=ready pod -l control-plane=ome-controller-manager -n ome --timeout=300s
	kubectl apply -k config/clusterresources
	git checkout HEAD -- config/certmanager/certificate.yaml

deploy-ci: manifests
	kubectl apply -k config/overlays/test
	# TODO: Add runtimes as part of default deployment
	kubectl wait --for=condition=ready pod -l control-plane=ome-controller-manager -n ome --timeout=300s
	kubectl apply -k config/overlays/test/clusterresources

deploy-helm: manifests
	helm install ome-crd charts/ome-crd/ --wait --timeout 180s
	helm install ome charts/ome-resources/ --wait --timeout 180s

undeploy:
	kubectl delete -k config/default

undeploy-dev:
	kubectl delete -k config/overlays/development

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) paths=./pkg/apis/serving/... output:crd:dir=config/crd/full
	$(CONTROLLER_GEN) rbac:roleName=ome-manager-role paths=./pkg/controller/... output:rbac:artifacts:config=config/rbac
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths=./pkg/apis/serving/v1beta1

	#TODO Remove this until new controller-tools is released
	perl -pi -e 's/storedVersions: null/storedVersions: []/g' config/crd/full/ome.oracle.com_inferenceservices.yaml
	perl -pi -e 's/conditions: null/conditions: []/g' config/crd/full/ome.oracle.com_inferenceservices.yaml
	perl -pi -e 's/Any/string/g' config/crd/full/ome.oracle.com_inferenceservices.yaml
	#remove the required property on framework as name field needs to be optional
	yq 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.required)' -i config/crd/full/ome.oracle.com_inferenceservices.yaml
	#remove ephemeralContainers properties for compress crd size https://github.com/kubeflow/kfserving/pull/1141#issuecomment-714170602
	yq 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.ephemeralContainers)' -i config/crd/full/ome.oracle.com_inferenceservices.yaml
	#knative does not allow setting port on liveness or readiness probe
	yq 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.readinessProbe.properties.httpGet.required)' -i config/crd/full/ome.oracle.com_inferenceservices.yaml
	yq 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.livenessProbe.properties.httpGet.required)' -i config/crd/full/ome.oracle.com_inferenceservices.yaml
	yq 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.readinessProbe.properties.tcpSocket.required)' -i config/crd/full/ome.oracle.com_inferenceservices.yaml
	yq 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.*.properties.livenessProbe.properties.tcpSocket.required)' -i config/crd/full/ome.oracle.com_inferenceservices.yaml
	yq 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.containers.items.properties.livenessProbe.properties.httpGet.required)' -i config/crd/full/ome.oracle.com_inferenceservices.yaml
	yq 'del(.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.*.properties.containers.items.properties.readinessProbe.properties.httpGet.required)' -i config/crd/full/ome.oracle.com_inferenceservices.yaml
	#With v1 and newer kubernetes protocol requires default
	yq '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | .. | select(has("protocol")) | path' config/crd/full/ome.oracle.com_inferenceservices.yaml -o j | jq -r '. | map(select(numbers)="["+tostring+"]") | join(".")' | awk '{print "."$$0".protocol.default"}' | xargs -n1 -I{} yq '{} = "TCP"' -i config/crd/full/ome.oracle.com_inferenceservices.yaml
	yq '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | .. | select(has("protocol")) | path' config/crd/full/ome.oracle.com_clusterservingruntimes.yaml -o j | jq -r '. | map(select(numbers)="["+tostring+"]") | join(".")' | awk '{print "."$$0".protocol.default"}' | xargs -n1 -I{} yq '{} = "TCP"' -i config/crd/full/ome.oracle.com_clusterservingruntimes.yaml
	yq '.spec.versions[0].schema.openAPIV3Schema.properties.spec.properties | .. | select(has("protocol")) | path' config/crd/full/ome.oracle.com_servingruntimes.yaml -o j | jq -r '. | map(select(numbers)="["+tostring+"]") | join(".")' | awk '{print "."$$0".protocol.default"}' | xargs -n1 -I{} yq '{} = "TCP"' -i config/crd/full/ome.oracle.com_servingruntimes.yaml
	./hack/minimal-crdgen.sh
	cp config/crd/full/ome* charts/ome-crd/templates/


# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

go-lint:
	hack/verify-golint.sh

# Generate code
generate: controller-gen
	go env -w GOFLAGS=-mod=mod
	hack/update-codegen.sh
	hack/update-openapigen.sh


test-qpext:
	cd qpext && go test -v ./... -cover

controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest


telepresence:
	hack/telepresence-setup.sh

helm-docs:
	hack/update-helm-docs.sh
