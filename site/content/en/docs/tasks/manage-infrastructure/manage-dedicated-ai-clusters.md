---
title: "Manage Dedicated AI Clusters"
linkTitle: "Manage Dedicated AI Clusters"
weight: 10
description: >
  Learn how to create, configure, and manage DedicatedAIClusters for isolated AI workloads
---

This guide walks you through creating and managing DedicatedAIClusters (DACs) to provide isolated, guaranteed compute resources for your AI/ML workloads.

## Prerequisites

Before you begin, make sure you have:

- OME installed and running in your Kubernetes cluster
- Appropriate RBAC permissions for DedicatedAICluster resources
- Access to OCI compartments and capacity reservations (if using)
- Understanding of your GPU resource requirements

## Creating Your First DedicatedAICluster

### Step 1: Define a DedicatedAIClusterProfile

First, create a profile that defines the resource template:

```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAIClusterProfile
metadata:
  name: h100-8gpu-standard
spec:
  count: 1                      # Resources per node
  resources:
    requests:
      nvidia.com/gpu: 8
      memory: 256Gi
      cpu: 32
    limits:
      nvidia.com/gpu: 8
      memory: 256Gi
      cpu: 32
  nodeSelector:
    gpu-type: h100
    node-class: bare-metal
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: node.kubernetes.io/instance-type
            operator: In
            values: ["BM.GPU.H100.8"]
  tolerations:
  - key: "gpu-dedicated"
    operator: "Equal"
    value: "h100"
    effect: "NoSchedule"
  priorityClassName: "high-priority-gpu"
```

Apply the profile:

```bash
kubectl apply -f h100-profile.yaml
```

### Step 2: Create the DedicatedAICluster

Now create a cluster using the profile:

```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: production-training-cluster
  labels:
    environment: production
    team: ml-engineering
    purpose: training
spec:
  profile: h100-8gpu-standard
  count: 4                      # 4 nodes total
  resources:
    requests:
      nvidia.com/gpu: 32        # 4 nodes × 8 GPUs
      memory: 1024Gi           # 4 nodes × 256Gi
      cpu: 128                 # 4 nodes × 32 cores
    limits:
      nvidia.com/gpu: 32
      memory: 1024Gi
      cpu: 128
  priorityClassName: production-workloads
  compartmentID: ocid1.compartment.oc1..aaaaaa
```

Apply the cluster:

```bash
kubectl apply -f production-cluster.yaml
```

### Step 3: Verify Cluster Creation

Check the cluster status:

```bash
# List all clusters
kubectl get dedicatedaiclusters

# Get detailed status
kubectl describe dedicatedaicluster production-training-cluster

# Monitor cluster state
kubectl get dedicatedaicluster production-training-cluster -w
```

Expected output:
```
NAME                           COUNT   STATUS   AGE
production-training-cluster    4       ACTIVE   5m

# Detailed status should show:
Status:
  Allocated Gpu:    0
  Available Gpu:    32
  Dac Lifecycle State: ACTIVE
  Lifecycle Detail: Cluster is healthy and ready for workloads
```

## Managing Cluster Capacity

### Monitoring Resource Allocation

Check current GPU allocation:

```bash
# Quick status check
kubectl get dedicatedaicluster production-training-cluster -o jsonpath='{.status.availableGpu}'

# Detailed resource view
kubectl get dedicatedaicluster production-training-cluster -o yaml | grep -A 10 status:
```

### Scaling the Cluster

To add more nodes to your cluster:

```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: production-training-cluster
spec:
  profile: h100-8gpu-standard
  count: 6                      # Increased from 4 to 6
  resources:
    requests:
      nvidia.com/gpu: 48        # Updated: 6 nodes × 8 GPUs
      memory: 1536Gi           # Updated: 6 nodes × 256Gi
      cpu: 192                 # Updated: 6 nodes × 32 cores
    limits:
      nvidia.com/gpu: 48
      memory: 1536Gi
      cpu: 192
```

Apply the changes:

```bash
kubectl apply -f production-cluster.yaml

# Monitor the scaling operation
kubectl get dedicatedaicluster production-training-cluster -w
```

## Using Capacity Reservations

### Creating a Capacity Reservation

First, create a capacity reservation:

```yaml
apiVersion: ome.io/v1beta1
kind: CapacityReservation
metadata:
  name: ml-team-gpu-reservation
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: h100-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 64
      - name: "memory"
        nominalQuota: 16Ti
      - name: "cpu"
        nominalQuota: 1024
  cohort: ml-workloads
  priorityClassName: high-priority
  allowBorrowing: true
  compartmentID: ocid1.compartment.oc1..aaaaaa
```

### Linking DAC to Capacity Reservation

Update your DAC to use the reservation:

```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: production-training-cluster
spec:
  profile: h100-8gpu-standard
  count: 4
  capacityReservationId: "ml-team-gpu-reservation"
  resources:
    requests:
      nvidia.com/gpu: 32
      memory: 1024Gi
      cpu: 128
  compartmentID: ocid1.compartment.oc1..aaaaaa
```

## Deploying Workloads to DAC

### Training Job on DAC

Deploy a training job to your dedicated cluster:

```yaml
apiVersion: ome.io/v1beta1
kind: TrainingJob
metadata:
  name: large-model-training
  labels:
    ome.io/cluster: production-training-cluster
spec:
  runtimeRef:
    name: pytorch-distributed
  trainer:
    image: pytorch/pytorch:2.0.1-cuda11.7-cudnn8-devel
    numNodes: 4
    resourcesPerNode:
      requests:
        nvidia.com/gpu: 8
        memory: 256Gi
        cpu: 32
    command: ["torchrun"]
    args:
    - "--nnodes=4"
    - "--nproc_per_node=8"
    - "train.py"
  datasets:
    storageUri: oci://training-data/datasets/large-corpus/
  modelConfig:
    outputModel:
      storageUri: oci://model-outputs/large-model-v1/
  # DAC-specific scheduling
  nodeSelector:
    ome.io/cluster: production-training-cluster
  tolerations:
  - key: "gpu-dedicated"
    operator: "Equal"
    value: "h100"
    effect: "NoSchedule"
```

### Inference Service on DAC

Deploy an inference service:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: high-priority-llm
  labels:
    ome.io/cluster: production-training-cluster
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
          nvidia.com/gpu: 8
        limits:
          nvidia.com/gpu: 8
    nodeSelector:
      ome.io/cluster: production-training-cluster
    tolerations:
    - key: "gpu-dedicated"
      operator: "Equal"
      value: "h100"
      effect: "NoSchedule"
```

## Multi-Environment Setup

### Development Cluster

Create a smaller cluster for development:

```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAIClusterProfile
metadata:
  name: a100-4gpu-dev
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
  nodeSelector:
    gpu-type: a100
    environment: development
  priorityClassName: "development-workloads"
---
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: development-cluster
  labels:
    environment: development
spec:
  profile: a100-4gpu-dev
  count: 2
  resources:
    requests:
      nvidia.com/gpu: 8
      memory: 256Gi
      cpu: 32
  priorityClassName: development-workloads
```

### Production Cluster

Create a high-availability production cluster:

```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: production-ha-cluster
  labels:
    environment: production
    ha: enabled
spec:
  profile: h100-8gpu-standard
  count: 8
  resources:
    requests:
      nvidia.com/gpu: 64
      memory: 2048Gi
      cpu: 256
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: topology.kubernetes.io/zone
            operator: In
            values: ["us-phoenix-1a", "us-phoenix-1b"]
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 50
        preference:
          matchExpressions:
          - key: topology.kubernetes.io/zone
            operator: In
            values: ["us-phoenix-1a"]
  priorityClassName: production-critical
  compartmentID: ocid1.compartment.oc1..production
```

## Monitoring and Maintenance

### Health Checks

Create a monitoring script:

```bash
#!/bin/bash
# check-dac-health.sh

DAC_NAME=${1:-production-training-cluster}

echo "=== DAC Health Check: $DAC_NAME ==="

# Check overall status
echo "Overall Status:"
kubectl get dedicatedaicluster $DAC_NAME -o custom-columns=NAME:.metadata.name,STATUS:.status.dacLifecycleState,AVAILABLE_GPU:.status.availableGpu,ALLOCATED_GPU:.status.allocatedGpu

# Check conditions
echo -e "\nConditions:"
kubectl get dedicatedaicluster $DAC_NAME -o jsonpath='{.status.conditions[*].type}' | tr ' ' '\n'

# Check node health
echo -e "\nNode Health:"
kubectl get nodes -l ome.io/cluster=$DAC_NAME -o custom-columns=NAME:.metadata.name,STATUS:.status.conditions[?@.type==\"Ready\"].status,GPU:.status.allocatable.nvidia\.com/gpu

# Check workload allocation
echo -e "\nWorkloads on DAC:"
kubectl get pods -A -o wide | grep $(kubectl get nodes -l ome.io/cluster=$DAC_NAME -o jsonpath='{.items[*].metadata.name}' | tr ' ' '|')
```

### Resource Utilization Monitoring

Set up Prometheus monitoring:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dac-monitoring-rules
data:
  rules.yaml: |
    groups:
    - name: dac-monitoring
      rules:
      - alert: DACGPUUtilizationHigh
        expr: (dac_allocated_gpu / dac_available_gpu) > 0.9
        for: 5m
        labels:
          severity: warning
          cluster: "{{ $labels.dac_name }}"
        annotations:
          summary: "DAC {{ $labels.dac_name }} GPU utilization is high ({{ $value }}%)"
          
      - alert: DACNodeDown
        expr: dac_lifecycle_state != "ACTIVE"
        for: 2m
        labels:
          severity: critical
          cluster: "{{ $labels.dac_name }}"
        annotations:
          summary: "DAC {{ $labels.dac_name }} is not in ACTIVE state"
          
      - alert: DACCapacityLow
        expr: dac_available_gpu < 8
        for: 10m
        labels:
          severity: warning
          cluster: "{{ $labels.dac_name }}"
        annotations:
          summary: "DAC {{ $labels.dac_name }} has low available capacity ({{ $value }} GPUs)"
```

### Automated Scaling

Create a HorizontalPodAutoscaler for workload-based scaling:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dac-autoscaling-script
data:
  scale-dac.sh: |
    #!/bin/bash
    DAC_NAME=$1
    TARGET_GPU_UTILIZATION=${2:-80}
    
    # Get current utilization
    CURRENT_ALLOCATED=$(kubectl get dedicatedaicluster $DAC_NAME -o jsonpath='{.status.allocatedGpu}')
    CURRENT_AVAILABLE=$(kubectl get dedicatedaicluster $DAC_NAME -o jsonpath='{.status.availableGpu}')
    CURRENT_TOTAL=$((CURRENT_ALLOCATED + CURRENT_AVAILABLE))
    
    if [ $CURRENT_TOTAL -eq 0 ]; then
      echo "No GPU resources found"
      exit 1
    fi
    
    UTILIZATION_PERCENT=$((CURRENT_ALLOCATED * 100 / CURRENT_TOTAL))
    
    echo "Current utilization: ${UTILIZATION_PERCENT}%"
    
    if [ $UTILIZATION_PERCENT -gt $TARGET_GPU_UTILIZATION ]; then
      echo "Scaling up DAC $DAC_NAME"
      # Add scaling logic here
    elif [ $UTILIZATION_PERCENT -lt $((TARGET_GPU_UTILIZATION - 20)) ]; then
      echo "Scaling down DAC $DAC_NAME"
      # Add scaling logic here
    else
      echo "No scaling needed"
    fi
```

## Troubleshooting Common Issues

### Cluster Stuck in CREATING State

1. Check node availability:
```bash
kubectl get nodes -l gpu-type=h100
kubectl describe nodes -l gpu-type=h100
```

2. Verify profile configuration:
```bash
kubectl describe dedicatedaiclusterprofile h100-8gpu-standard
```

3. Check resource conflicts:
```bash
kubectl get dedicatedaiclusters -o wide
kubectl get capacityreservations
```

### GPU Resources Not Available

1. Verify GPU device plugins:
```bash
kubectl get daemonset -n kube-system nvidia-device-plugin-daemonset
kubectl logs -n kube-system -l name=nvidia-device-plugin-ds
```

2. Check node GPU status:
```bash
kubectl describe node gpu-node-1 | grep -A 10 Allocatable
```

3. Verify workload scheduling:
```bash
kubectl get pods -o wide | grep gpu
kubectl describe pod workload-pod | grep -A 5 Events
```

### Performance Issues

1. Check network configuration:
```bash
# Verify RDMA devices
kubectl exec -it training-pod -- ls /dev/infiniband/

# Check network policies
kubectl get networkpolicies -A
```

2. Monitor resource utilization:
```bash
# GPU utilization
kubectl exec -it training-pod -- nvidia-smi

# Network throughput
kubectl exec -it training-pod -- iperf3 -c target-node
```

## Best Practices

### Resource Planning

1. **Right-size your clusters**: Start with smaller clusters and scale based on actual usage
2. **Use profiles**: Create reusable profiles for different workload types
3. **Plan for overhead**: Reserve 10-15% capacity for system overhead and scheduling flexibility

### Security and Isolation

1. **Use tolerations**: Ensure workloads are properly isolated using taints and tolerations
2. **Network policies**: Implement network policies to control traffic between clusters
3. **RBAC**: Set up appropriate role-based access control

### Cost Optimization

1. **Capacity reservations**: Use capacity reservations for predictable workloads
2. **Mixed instance types**: Combine different GPU types based on workload requirements
3. **Scheduling policies**: Use priority classes to optimize resource allocation

### Monitoring and Alerting

1. **Set up comprehensive monitoring**: Monitor cluster health, resource utilization, and workload performance
2. **Define SLOs**: Establish service level objectives for cluster availability and performance
3. **Automate responses**: Set up automated scaling and remediation for common issues

This guide provides a solid foundation for managing DedicatedAIClusters in production environments. Adapt the examples to your specific requirements and operational procedures. 