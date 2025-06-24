#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/internal/tools
CURRENT_DIR=$(dirname "${BASH_SOURCE[0]}")
SCRIPT_ROOT="${CURRENT_DIR}/.."
CODEGEN_VERSION=$(cd "${KUBE_ROOT}" && grep 'k8s.io/code-generator' go.mod | awk '{print $2}')

if [ -z "${GOPATH:-}" ]; then
    GOPATH=$(go env GOPATH)
    export GOPATH
fi

# Make sure all modules are downloaded.
(cd "${KUBE_ROOT}" && go mod tidy)

CODEGEN_PKG=$(cd "${KUBE_ROOT}" && go list -f '{{.Dir}}' -m k8s.io/code-generator@"${CODEGEN_VERSION}" 2>/dev/null)
THIS_PKG="github.com/sgl-project/ome"

# shellcheck source=/dev/null
source "${CODEGEN_PKG}/kube_codegen.sh"

# Redirect stdout to /dev/null but keep stderr
exec 3>&1 # Save the current stdout to file descriptor 3
exec 1>/dev/null # Redirect stdout to /dev/null

kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}"

kube::codegen::gen_client \
    --with-watch \
    --output-dir "${SCRIPT_ROOT}/pkg/client" \
    --output-pkg "${THIS_PKG}/pkg/client" \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

# Restore stdout
exec 1>&3