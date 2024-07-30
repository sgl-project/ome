#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_VERSION=$(cd "${KUBE_ROOT}" && grep 'k8s.io/code-generator' go.mod | awk '{print $2}')

if [ -z "${GOPATH:-}" ]; then
    GOPATH=$(go env GOPATH)
    export GOPATH
fi
CODEGEN_PKG="$GOPATH/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}"

# To avoid permission denied error
chmod +x "${CODEGEN_PKG}/generate-groups.sh"
chmod +x "${CODEGEN_PKG}/generate-internal-groups.sh"

# Generating files for v1beta1
"${CODEGEN_PKG}/generate-groups.sh" \
    "deepcopy,client,informer,lister" \
    "bitbucket.oci.oraclecorp.com/gen/ome/pkg/client" \
    "bitbucket.oci.oraclecorp.com/gen/ome/pkg/apis" \
    "serving:v1beta1" \
    --go-header-file "${KUBE_ROOT}/hack/boilerplate.go.txt"
