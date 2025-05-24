#!/bin/bash

set -u
set -e

export KUBECONFIG=$HOME/.kube/gais-clusters/dp-us-chicago-1-preprod-config
dac=$1

kubectl patch DedicatedAiClusters.ome.oracle.com $dac --type='json' -p='[
{"op": "add", "path": "/metadata/annotations", "value":
{"ome.oracle.com/skip-reconcile": "true"}}]'