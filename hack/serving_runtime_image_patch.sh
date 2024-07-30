#!/bin/bash



# The script is used to patch serving runtime image.

set -o errexit
set -o nounset
set -o pipefail

export SERVING_RUNTIME_FILE_NAME=$1
export IMG=$2
if [ -z "$IMG" ] || [ -z "$SERVING_RUNTIME_FILE_NAME" ]; then exit; fi
yq eval '.spec.containers[0].image = env(IMG)' config/runtimes/"$SERVING_RUNTIME_FILE_NAME" | kubectl apply -f -
