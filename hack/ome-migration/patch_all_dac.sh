#!/bin/bash

set -u
set -e

log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') [INFO] $1"
}

export KUBECONFIG=$HOME/.kube/gais-clusters/dp-us-chicago-1-preprod-config
REGION="us-chicago-1"
ENV="ppe"
TARGET="dac"

FILE_DIR="./$ENV-$TARGET-$REGION-patch"

mkdir -p "$FILE_DIR"

ALL_DAC_YAML=$(kubectl get DedicatedAiCluster.ome.oracle.com --all-namespaces -o yaml)
echo "$ALL_DAC_YAML" > "$FILE_DIR/all_dac.yaml"

DAC_ARRAY=( $(yq e -o=j -I=0 '.items[]' "$FILE_DIR/all_dac.yaml") )

for dac in "${DAC_ARRAY[@]}"; do
	dac_name=$(jq -r '.metadata.name' <<< "$dac")
  log "Start patch DAC "$dac_name""
  kubectl patch DedicatedAiCluster.ome.oracle.com "$dac_name" --type='json' -p='[{"op": "add", "path": "/metadata/annotations", "value": {"ome.oracle.com/skip-reconcile": "true"}}]'
done
