#!/bin/bash

set -u
set -e

export KUBECONFIG=$HOME/.kube/gais-clusters/dp-us-chicago-1-preprod-config
isvc=$1
namespace=$2

kubectl patch InferenceServices.ome.oracle.com $isvc -n $namespace --type='json' -p='[{"op": "add", "path": "/metadata/annotations", "value": {"ome.oracle.com/skip-reconcile": "true"}}]'