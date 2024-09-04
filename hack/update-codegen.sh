#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/internal/tools
CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
CODEGEN_VERSION=$(cd "${KUBE_ROOT}" && grep 'k8s.io/code-generator' go.mod | awk '{print $2}')
echo "KUBE_ROOT: ${KUBE_ROOT}"
if [ -z "${GOPATH:-}" ]; then
    GOPATH=$(go env GOPATH)
    export GOPATH
fi

echo "Using code-generator version: ${CODEGEN_VERSION}"
CODEGEN_PKG="$GOPATH/pkg/mod/k8s.io/code-generator@${CODEGEN_VERSION}"

# To avoid permission denied error
chmod +x "${CODEGEN_PKG}/generate-groups.sh"
chmod +x "${CODEGEN_PKG}/generate-internal-groups.sh"

# Generating files for v1beta1
"${CODEGEN_PKG}/generate-groups.sh" \
    "deepcopy,client,informer,lister" \
    "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/client" \
    "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis" \
    "serving:v1beta1" \
    --go-header-file "${CURRENT_DIR}/boilerplate.go.txt"
