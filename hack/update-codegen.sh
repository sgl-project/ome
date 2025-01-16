#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/internal/tools
CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
SCRIPT_ROOT="${CURRENT_DIR}/.."
CODEGEN_VERSION=$(cd "${KUBE_ROOT}" && grep 'k8s.io/code-generator' go.mod | awk '{print $2}')
echo "KUBE_ROOT: ${KUBE_ROOT}"
if [ -z "${GOPATH:-}" ]; then
    GOPATH=$(go env GOPATH)
    export GOPATH
fi

echo "Using code-generator version: ${CODEGEN_VERSION}"
CODEGEN_PKG=$(cd "${KUBE_ROOT}" && go list -f '{{.Dir}}' -m k8s.io/code-generator@"${CODEGEN_VERSION}")
THIS_PKG="bitbucket.oci.oraclecorp.com/genaicore/ome"

# shellcheck source=/dev/null
source "${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}"

kube::codegen::gen_client \
    --with-watch \
    --output-dir "${SCRIPT_ROOT}/pkg/client" \
    --output-pkg "${THIS_PKG}/pkg/client" \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"