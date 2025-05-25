---
title: "DedicatedAICluster"
linkTitle: "DedicatedAICluster"
weight: 70
description: >
  Understanding dedicated AI clusters for resource isolation and management
---

DedicatedAICluster (DAC) provides dedicated compute resources for AI/ML workloads with guaranteed capacity, isolation, and specialized hardware configurations. DACs ensure predictable performance and resource availability for critical workloads.

## Overview

DedicatedAICluster enables you to:

- **Guarantee Resources**: Reserve dedicated compute capacity for high-priority workloads
- **Isolate Workloads**: Separate compute resources by team, project, or workload type
- **Optimize Hardware**: Configure specialized GPU, CPU, and networking configurations
- **Manage Capacity**: Track and allocate GPU resources efficiently
- **Control Scheduling**: Apply priority classes and advanced scheduling policies

## DedicatedAICluster Specification

### Basic Structure

```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: production-training-cluster
spec:
  profile: h100-8gpu-profile
  count: 4
  resources:
    requests:
      nvidia.com/gpu: 32  # 4 nodes * 8 GPUs
      memory: 1024Gi      # 4 nodes * 256Gi
      cpu: 128            # 4 nodes * 32 cores
    limits:
      nvidia.com/gpu: 32
      memory: 1024Gi
      cpu: 128
  nodeSelector:
    gpu-type: h100
    network: rdma
  priorityClassName: high-priority-ai
  compartmentID: ocid1.compartment.oc1..xxxxx
```

### Key Fields

#### Profile Reference
References a DedicatedAIClusterProfile for standardized configurations:

```yaml
spec:
  profile: "h100-8gpu-profile"  # References DedicatedAIClusterProfile
```

#### Resource Allocation
Defines the total resources available in the cluster:

```yaml
spec:
  count: 8                      # Number of nodes/units
  resources:
    requests:
      nvidia.com/gpu: 64        # Total GPUs across all nodes
      memory: 2048Gi           # Total memory
      cpu: 256                 # Total CPU cores
    limits:
      nvidia.com/gpu: 64
      memory: 2048Gi
      cpu: 256
```

#### Node Selection and Affinity
Controls where cluster resources are allocated:

```yaml
spec:
  nodeSelector:
    gpu-type: "h100"
    network-type: "rdma"
    zone: "us-phoenix-1a"
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: nvidia.com/gpu.memory
            operator: Gt
            values: ["80Gi"]
  tolerations:
  - key: "gpu-intensive"
    operator: "Equal"
    value: "true"
    effect: "NoSchedule"
```

## DedicatedAIClusterProfile

Profiles provide reusable templates for common cluster configurations:

### H100 8-GPU Profile
```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAIClusterProfile
metadata:
  name: h100-8gpu-profile
spec:
  count: 1                      # Per-node specification
  resources:
    requests:
      nvidia.com/gpu: 8
      memory: 256Gi
      cpu: 32
    limits:
      nvidia.com/gpu: 8
      memory: 256Gi
      cpu: 32
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node.kubernetes.io/instance-type
            operator: In
            values: ["BM.GPU.H100.8"]
  nodeSelector:
    gpu-type: h100
    memory-type: hbm
  tolerations:
  - key: "gpu-dedicated"
    operator: "Equal"
    value: "h100"
    effect: "NoSchedule"
  priorityClassName: "gpu-workloads"
```

### A100 4-GPU Profile
```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAIClusterProfile
metadata:
  name: a100-4gpu-profile
spec:
  count: 1
  resources:
    requests:
      nvidia.com/gpu: 4
      memory: 128Gi
      cpu: 16
    limits:
      nvidia.com/gpu: 4
      memory: 128Gi
      cpu: 16
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node.kubernetes.io/instance-type
            operator: In
            values: ["BM.GPU4.A100"]
  nodeSelector:
    gpu-type: a100
  priorityClassName: "standard-gpu-workloads"
```

## Cluster Lifecycle and Status

### Lifecycle States

DedicatedAICluster tracks its state through several phases:

```yaml
status:
  dacLifecycleState: ACTIVE      # CREATING, ACTIVE, UPDATING, DELETING, FAILED
  availableGpu: 28               # GPUs available for allocation
  allocatedGpu: 4                # GPUs currently allocated
  lifecycleDetail: "Cluster is healthy and ready for workloads"
  conditions:
  - type: Ready
    status: "True"
    reason: "ClusterHealthy"
    message: "All nodes are healthy and resources are available"
```

#### State Descriptions

- **CREATING**: Cluster is being provisioned and configured
- **ACTIVE**: Cluster is operational and accepting workloads
- **UPDATING**: Cluster configuration is being modified
- **DELETING**: Cluster resources are being decommissioned
- **FAILED**: Cluster encountered an error during provisioning or operation

### Resource Tracking

Monitor GPU allocation and availability:

```yaml
status:
  availableGpu: 24              # Total available for new allocations
  allocatedGpu: 8               # Currently allocated to workloads
  conditions:
  - type: ResourcesAvailable
    status: "True"
    reason: "SufficientCapacity"
    message: "75% of GPU capacity available"
```

## Using DedicatedAICluster

### For Training Workloads

```yaml
apiVersion: ome.io/v1beta1
kind: TrainingJob
metadata:
  name: large-model-training
spec:
  runtimeRef:
    name: pytorch-distributed
  trainer:
    numNodes: 4
    resourcesPerNode:
      requests:
        nvidia.com/gpu: 8
  datasets:
    storageUri: oci://ns/bucket/training-data/
  modelConfig:
    outputModel:
      storageUri: oci://ns/bucket/model-output/
  # Scheduling on DedicatedAICluster
  labels:
    ome.io/dedicated-cluster: production-training-cluster
  nodeSelector:
    ome.io/cluster: production-training-cluster
```

### For Inference Services

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: production-llm-service
spec:
  predictor:
    model:
      name: llama-3-70b
    runtime:
      name: vllm-runtime
  engine:
    runner:
      resources:
        requests:
          nvidia.com/gpu: 4
    size: 2                     # Use 2 nodes from the cluster
    nodeSelector:
      ome.io/cluster: production-training-cluster
```

## Capacity Reservations

Link DACs to capacity reservations for guaranteed resource availability:

```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: reserved-cluster
spec:
  profile: h100-8gpu-profile
  count: 8
  capacityReservationId: "ocid1.capacityreservation.oc1..xxxxx"
  resources:
    requests:
      nvidia.com/gpu: 64
      memory: 2048Gi
      cpu: 256
  compartmentID: "ocid1.compartment.oc1..xxxxx"
```

## Multi-Cluster Configurations

### Development Cluster
```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: development-cluster
  labels:
    environment: development
    team: research
spec:
  profile: a100-4gpu-profile
  count: 2
  priorityClassName: development-workloads
  nodeSelector:
    zone: us-phoenix-1a
    cost-tier: standard
```

### Production Cluster
```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: production-cluster
  labels:
    environment: production
    criticality: high
spec:
  profile: h100-8gpu-profile
  count: 16
  priorityClassName: production-workloads
  nodeSelector:
    zone: us-phoenix-1a
    network: rdma
    cost-tier: premium
  tolerations:
  - key: "production-only"
    operator: "Equal"
    value: "true"
    effect: "NoSchedule"
```

### Research Cluster
```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: research-cluster
  labels:
    environment: research
    team: ml-research
spec:
  profile: h100-8gpu-profile
  count: 4
  priorityClassName: research-workloads
  nodeSelector:
    experiment-ready: "true"
  affinity:
    nodeAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        preference:
          matchExpressions:
          - key: network.latency
            operator: In
            values: ["ultra-low"]
```

## Monitoring and Observability

### Resource Utilization
```bash
# Check cluster status
kubectl get dedicatedaicluster production-cluster

# View detailed resource allocation
kubectl describe dedicatedaicluster production-cluster

# Monitor GPU utilization
kubectl get dedicatedaicluster -o jsonpath='{.items[*].status.allocatedGpu}'
```

### Cluster Health Monitoring
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dac-monitoring-config
data:
  alerts.yaml: |
    groups:
    - name: dac-alerts
      rules:
      - alert: DACGPUUtilizationHigh
        expr: (dac_allocated_gpu / dac_available_gpu) > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "DAC GPU utilization is high"
      - alert: DACNodeUnhealthy
        expr: dac_lifecycle_state != "ACTIVE"
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "DAC is not in ACTIVE state"
```

## Best Practices

### Resource Planning
```yaml
metadata:
  name: optimized-cluster
  labels:
    ome.io/resource-class: high-memory
    ome.io/workload-type: training
    ome.io/optimization: cost-effective
spec:
  profile: optimized-profile
  count: 4
  # Plan for 80% utilization
  resources:
    requests:
      nvidia.com/gpu: 32
      memory: 1024Gi      # 25% overhead for system
      cpu: 128
```

### High Availability
```yaml
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: topology.kubernetes.io/zone
            operator: In
            values: ["us-phoenix-1a", "us-phoenix-1b"]  # Multi-AZ deployment
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 50
        preference:
          matchExpressions:
          - key: topology.kubernetes.io/zone
            operator: In
            values: ["us-phoenix-1a"]
```

### Cost Optimization
```yaml
metadata:
  name: cost-optimized-cluster
  labels:
    ome.io/cost-tier: spot
    ome.io/workload-priority: batch
spec:
  nodeSelector:
    node-lifecycle: spot
    cost-optimization: enabled
  tolerations:
  - key: "spot-instance"
    operator: "Equal"
    value: "true"
    effect: "NoSchedule"
```

## Troubleshooting

### Common Issues

**Cluster Stuck in CREATING State**
```bash
# Check cluster events
kubectl describe dedicatedaicluster my-cluster

# Verify node availability
kubectl get nodes -l ome.io/cluster=my-cluster

# Check capacity reservations
kubectl get capacityreservations
```

**Resource Allocation Failures**
```bash
# Check available resources
kubectl get dedicatedaicluster -o jsonpath='{.status.availableGpu}'

# Verify profile configuration
kubectl describe dedicatedaiclusterprofile my-profile

# Check node affinity and tolerations
kubectl get nodes --show-labels
```

**GPU Not Available**
```bash
# Verify GPU allocation
kubectl describe dedicatedaicluster my-cluster

# Check node GPU status
kubectl describe node gpu-node-1

# Verify GPU device plugins
kubectl get daemonset -n kube-system nvidia-device-plugin-daemonset
```

### Debugging Commands

```bash
# List all clusters
kubectl get dedicatedaiclusters

# Get cluster details
kubectl describe dedicatedaicluster cluster-name

# Check profile configuration
kubectl get dedicatedaiclusterprofiles

# Monitor resource allocation
kubectl get dedicatedaicluster -o wide --watch

# Check workload assignments
kubectl get pods -l ome.io/cluster=cluster-name
```

## Security Considerations

### Access Control
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: dac-admin
rules:
- apiGroups: ["ome.io"]
  resources: ["dedicatedaiclusters", "dedicatedaiclusterprofiles"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

### Resource Isolation
```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: dac-resource-quota
  namespace: production
spec:
  hard:
    nvidia.com/gpu: "32"        # Limit GPU allocation per namespace
    requests.memory: "1024Gi"
    requests.cpu: "128"
```

### Network Security
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: dac-network-policy
spec:
  podSelector:
    matchLabels:
      ome.io/cluster: production-cluster
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: production
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: production
``` 