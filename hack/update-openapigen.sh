#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

KNOWN_VIOLATION_EXCEPTIONS=hack/violation_exceptions.list
CURRENT_VIOLATION_EXCEPTIONS=hack/current_violation_exceptions.list
OPENAPI_SPEC_FILE=pkg/openapi/openapi_generated.go

GOPATH=$(go env GOPATH)
if [[ -z $GOPATH ]]
then
    echo >&2 "Error: GOPATH is not set. Please configure your GOPATH environment variable."
    exit 1
fi
TARGET_DIR="$GOPATH/src/github.com/sgl-project/sgl-ome"
CURRENT_DIR=$(pwd)

# Redirect stdout to /dev/null but keep stderr
exec 3>&1 # Save the current stdout to file descriptor 3
exec 1>/dev/null # Redirect stdout to /dev/null

# Check if the current directory is the target directory
if [[ "$CURRENT_DIR" != "$TARGET_DIR" ]]; then
    echo >&2 "You are not in the target directory: $TARGET_DIR"

    # Check if the target directory exists.
    if [[ -d "$TARGET_DIR" ]]; then
        mv $TARGET_DIR "${TARGET_DIR}_$(date +%Y%m%d_%H%M%S)"
    fi

    echo >&2 "Creating a symbolic link for the target directory ..."
    mkdir -p "$(dirname "$TARGET_DIR")"
    ln -s "$CURRENT_DIR" "$TARGET_DIR"

    # Change to the target directory
    echo >&2 "Changing to the target directory: $TARGET_DIR"
    pushd "$TARGET_DIR" > /dev/null
fi

# Generating OpenAPI specification
go run k8s.io/kube-openapi/cmd/openapi-gen \
    --output-pkg github.com/sgl-project/sgl-ome/pkg/openapi --output-dir "./pkg/openapi" \
    --output-file "openapi_generated.go" \
    -v 5 --go-header-file hack/boilerplate.go.txt \
    -r $CURRENT_VIOLATION_EXCEPTIONS \
    "knative.dev/pkg/apis" \
    "knative.dev/pkg/apis/duck/v1" \
    "./pkg/apis/ome/v1beta1" 2>&1

# Hack, the name is required in openAPI specification even if set "+optional" for v1.Container in PredictorExtensionSpec.
sed -i'.bak' -e 's/Required: \[\]string{\"name\"},//g' $OPENAPI_SPEC_FILE && rm -rf $OPENAPI_SPEC_FILE.bak
sed -i'.bak' -e 's/Required: \[\]string{\"modelFormat\", \"name\"},/Required: \[\]string{\"modelFormat\"},/g' $OPENAPI_SPEC_FILE && rm -rf $OPENAPI_SPEC_FILE.bak

test -f $CURRENT_VIOLATION_EXCEPTIONS || touch $CURRENT_VIOLATION_EXCEPTIONS

# The API rule fails if generated API rule violation report differs from the
# checked-in violation file, prints error message to request developer to
# fix either the API source code, or the known API rule violation file.
if ! diff $CURRENT_VIOLATION_EXCEPTIONS $KNOWN_VIOLATION_EXCEPTIONS > /dev/null; then
    echo >&2 "ERROR: API rule check failed. Reported violations in file $CURRENT_VIOLATION_EXCEPTIONS differ from known violations in file $KNOWN_VIOLATION_EXCEPTIONS."
    exit 1
fi

# Generating swagger file
go run cmd/spec-gen/main.go 0.1 > pkg/openapi/swagger.json 2>&1

# Return to the original directory
if [[ "$CURRENT_DIR" != "$TARGET_DIR" ]]; then
    echo >&2 "Returning to the original directory: $CURRENT_DIR"
    popd > /dev/null
fi

# Restore stdout
exec 1>&3
