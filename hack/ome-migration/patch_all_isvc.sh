#!/bin/bash

set -u
set -e

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $1"
}

export KUBECONFIG=$HOME/.kube/gais-clusters/dp-us-chicago-1-preprod-config
REGION="us-chicago-1"
ENV="ppe"
TARGET="isvc"

FILE_DIR="./$ENV-$TARGET-$REGION-patch"

mkdir -p "$FILE_DIR"

ALL_ISVC_YAML=$(kubectl get InferenceService.ome.oracle.com --all-namespaces -o yaml)
echo "$ALL_ISVC_YAML" > "$FILE_DIR/all_isvc.yaml"

ISVC_ARRAY=( $(yq e -o=j -I=0 '.items[]' "$FILE_DIR/all_isvc.yaml") )

for isvc in "${ISVC_ARRAY[@]}"; do
	isvc_name=$(jq -r '.metadata.name' <<< "$isvc")
  isvc_namespace=$(jq -r '.metadata.namespace' <<< "$isvc")
  log "Start patch InferenceService "$isvc_name" in namespace "$isvc_namespace""
  kubectl patch InferenceService.ome.oracle.com "$isvc_name" -n "$namespace_name" --type='json' -p='[{"op": "add", "path": "/metadata/annotations", "value": {"ome.oracle.com/skip-reconcile": "true"}}]'
done