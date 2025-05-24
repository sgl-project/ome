#!/bin/bash

# List of namespaces (space-separated)
# when you use it, update the namespace list
namespaces=("cohere-rerank" "command-r-082024-v1-7-tp1-128k" "command-r-plus-082024-v1-6-128k" "embed-english-03" "embed-multilingual-03" "llama-3-1-70b-instruct" "llama-3-2-90b-vision-instruct-fp8-dynamic" "llama-3-3-70b-instruct" "rerank-english-03" "rerank-multilingual-03")  # Replace with your namespaces

#export KUBECONFIG=$HOME/.kube/gais-clusters/dp-us-chicago-1-preprod-config

# Patch content
patch='{
  "metadata": {
    "labels": {
      "app.kubernetes.io/managed-by": "Helm"
    },
    "annotations": {
      "meta.helm.sh/release-name": "ome-model-serving",
      "meta.helm.sh/release-namespace": "ome"
    }
  }
}'

# Loop through each namespace and apply the patch
for ns in "${namespaces[@]}"; do
  echo "Patching namespace: $ns"
  kubectl patch namespace "$ns" --type='merge' -p "$patch"
done