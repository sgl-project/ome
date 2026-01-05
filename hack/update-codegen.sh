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

CODEGEN_PKG=$(cd "${KUBE_ROOT}" && go list -f '{{.Dir}}' -m k8s.io/code-generator@"${CODEGEN_VERSION}" 2>/dev/null)
THIS_PKG="bitbucket.oci.oraclecorp.com/genaicore/ome"

# Opensource OME version/commit to fetch (can be overridden via environment variable)
OPENSOURCE_VERSION="${OPENSOURCE_VERSION:-3703910cd81030c1c8228644a34a45a61eda1e41}"
OPENSOURCE_REPO_URL="https://github.com/sgl-project/ome/archive/${OPENSOURCE_VERSION}.tar.gz"

# shellcheck source=/dev/null
source "${CODEGEN_PKG}/kube_codegen.sh"

# Redirect stdout to /dev/null but keep stderr
exec 3>&1 # Save the current stdout to file descriptor 3
exec 1>/dev/null # Redirect stdout to /dev/null

# Fetch opensource APIs
TEMP_DIR=$(mktemp -d)
trap "rm -rf ${TEMP_DIR}" EXIT

curl -sSL "${OPENSOURCE_REPO_URL}" | tar -xzf - -C "${TEMP_DIR}"
OPENSOURCE_DIR=$(ls -d "${TEMP_DIR}"/ome-* 2>/dev/null | head -1)

# Copy opensource APIs to pkg/apis, excluding files we want to keep:
# - zz_generated.*.go: Will be regenerated for all types (opensource + ours)
# - v1beta1.go, register.go, groupversion_info.go: Keep our scheme registration
# - doc.go: Keep our package documentation
# - apis.go: Keep our local import path (bitbucket vs github)
if [ -d "${OPENSOURCE_DIR}/pkg/apis" ]; then
    rsync -av \
        --exclude='zz_generated.deepcopy.go' \
        --exclude='zz_generated.defaults.go' \
        --exclude='v1beta1.go' \
        --exclude='register.go' \
        --exclude='groupversion_info.go' \
        --exclude='doc.go' \
        --exclude='apis.go' \
        "${OPENSOURCE_DIR}/pkg/apis/" "${SCRIPT_ROOT}/pkg/apis/"
fi

# Regenerate deepcopy/defaults for ALL types (opensource + ours)
kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}"

# Generate client from pkg/apis (correct import paths)
kube::codegen::gen_client \
    --with-watch \
    --output-dir "${SCRIPT_ROOT}/pkg/client" \
    --output-pkg "${THIS_PKG}/pkg/client" \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

# Restore stdout
exec 1>&3