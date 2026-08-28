#!/usr/bin/env bash

set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"
trap stop_nested_tunnel EXIT

scenario_namespace="alfred-e2e"
scenario_label="alfred-e2e.ome.io/scenario=fragmentation"
gpu_product="ALFRED-E2E-H100"

start_nested_tunnel

echo "Resetting the nested fragmentation fixture"
nested_kubectl delete namespace "${scenario_namespace}" \
  --ignore-not-found --wait=true
nested_kubectl delete nodes -l "${scenario_label}" \
  --ignore-not-found --wait=true
nested_kubectl create namespace "${scenario_namespace}"

for index in $(seq 0 9); do
  node_name="alfred-e2e-gpu-${index}"
  nested_kubectl apply -f - <<EOF
apiVersion: v1
kind: Node
metadata:
  name: ${node_name}
  annotations:
    node.alpha.kubernetes.io/ttl: "0"
    kwok.x-k8s.io/node: fake
  labels:
    alfred-e2e.ome.io/scenario: fragmentation
    kubernetes.io/arch: amd64
    kubernetes.io/hostname: ${node_name}
    kubernetes.io/os: linux
    nvidia.com/gpu.product: ${gpu_product}
spec:
  taints:
    - key: kwok.x-k8s.io/node
      value: fake
      effect: NoSchedule
status:
  allocatable:
    cpu: "32"
    memory: 256Gi
    pods: "110"
    nvidia.com/gpu: "8"
  capacity:
    cpu: "32"
    memory: 256Gi
    pods: "110"
    nvidia.com/gpu: "8"
  phase: Running
EOF
done

for index in $(seq 0 9); do
  nested_kubectl wait --for=condition=Ready \
    "node/alfred-e2e-gpu-${index}" --timeout=120s
done

nested_kubectl -n "${scenario_namespace}" apply -f - <<'EOF'
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: fragmented
  annotations:
    alfred.ome.io/movable: "true"
spec:
  engine:
    minReplicas: 10
    maxReplicas: 10
EOF

for index in $(seq 0 9); do
  node_name="alfred-e2e-gpu-${index}"
  nested_kubectl -n "${scenario_namespace}" apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: fragmented-${index}
  labels:
    alfred-e2e.ome.io/scenario: fragmentation
    ome.io/inferenceservice: fragmented
    component: engine
spec:
  nodeName: ${node_name}
  tolerations:
    - key: kwok.x-k8s.io/node
      operator: Exists
      effect: NoSchedule
  containers:
    - name: mock-engine
      image: fake-image
      resources:
        requests:
          nvidia.com/gpu: "1"
        limits:
          nvidia.com/gpu: "1"
EOF
done

nested_kubectl -n "${scenario_namespace}" wait --for=condition=Ready pod \
  -l "${scenario_label}" --timeout=120s

for index in $(seq 0 9); do
  nested_kubectl -n "${scenario_namespace}" patch "pod/fragmented-${index}" \
    --subresource=status --type=merge \
    -p '{"status":{"startTime":"2020-01-01T00:00:00Z"}}' >/dev/null
done

nested_kubectl -n "${scenario_namespace}" apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: pending-8gpu
  labels:
    alfred-e2e.ome.io/scenario: fragmentation
spec:
  nodeSelector:
    nvidia.com/gpu.product: ${gpu_product}
  tolerations:
    - key: kwok.x-k8s.io/node
      operator: Exists
      effect: NoSchedule
  containers:
    - name: pending-demand
      image: fake-image
      resources:
        requests:
          nvidia.com/gpu: "8"
        limits:
          nvidia.com/gpu: "8"
EOF

# verify.sh owns its own port-forward. Release this script's tunnel first so
# the verifier can bind the same deterministic local port.
stop_nested_tunnel
"${script_dir}/verify.sh"
