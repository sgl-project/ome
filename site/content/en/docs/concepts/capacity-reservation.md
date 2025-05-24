---
title: "CapacityReservation"
linkTitle: "CapacityReservation"
weight: 80
description: >
  Understanding capacity reservations for guaranteed resource availability
---

CapacityReservation provides a mechanism to reserve and guarantee compute resources for AI/ML workloads. It integrates with Kueue for advanced resource management, ensuring that critical workloads have access to the resources they need when they need them.

## Overview

CapacityReservation enables you to:

- **Guarantee Resources**: Reserve specific amounts of GPU, CPU, and memory capacity
- **Manage Priority**: Control resource allocation through preemption and priority classes
- **Enable Sharing**: Allow resource borrowing between different reservations
- **Track Usage**: Monitor resource utilization across different associations
- **Integrate with Kueue**: Leverage advanced queueing and resource management capabilities

## CapacityReservation Specification

### Basic Structure

```yaml
apiVersion: ome.io/v1beta1
kind: CapacityReservation
metadata:
  name: production-gpu-reservation
  namespace: production
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: h100-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 32
      - name: "memory"
        nominalQuota: 8Ti
      - name: "cpu"
        nominalQuota: 512
  cohort: production-cohort
  priorityClassName: high-priority
  allowBorrowing: true
  compartmentID: ocid1.compartment.oc1..xxxxx
```

### Key Components

#### Resource Groups
Define the types and amounts of resources to reserve:

```yaml
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: h100-8gpu-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 64          # Reserve 64 GPUs
        borrowingLimit: 32        # Can borrow up to 32 additional GPUs
      - name: "memory" 
        nominalQuota: 16Ti        # Reserve 16TB memory
      - name: "cpu"
        nominalQuota: 1024        # Reserve 1024 CPU cores
    - name: a100-4gpu-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 32
      - name: "memory"
        nominalQuota: 8Ti
      - name: "cpu"
        nominalQuota: 512
```

#### Cohort Configuration
Group related reservations for resource sharing:

```yaml
spec:
  cohort: "research-cohort"      # Share resources within cohort
  allowBorrowing: true           # Allow borrowing from other reservations
```

#### Preemption Rules
Control how workloads can preempt others:

```yaml
spec:
  preemptionRule:
    reclaimWithinCohort: Any           # Can preempt any workload in cohort
    borrowWithinCohort:
      policy: LowerPriority            # Can borrow from lower priority
      maxPriorityThreshold: 100
    withinClusterQueue: LowerPriority  # Preempt lower priority within queue
```

## Resource Management Scenarios

### GPU Training Reservation
Reserve resources specifically for training workloads:

```yaml
apiVersion: ome.io/v1beta1
kind: CapacityReservation
metadata:
  name: training-gpu-reservation
  labels:
    workload-type: training
    priority: high
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: training-h100-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 128         # Reserve 128 H100 GPUs for training
      - name: "memory"
        nominalQuota: 32Ti
      - name: "cpu"
        nominalQuota: 2048
  cohort: ai-workloads
  priorityClassName: training-high-priority
  allowBorrowing: false           # Dedicated resources, no borrowing
  compartmentID: ocid1.compartment.oc1..training
```

### Inference Serving Reservation
Reserve resources for production inference services:

```yaml
apiVersion: ome.io/v1beta1
kind: CapacityReservation
metadata:
  name: inference-serving-reservation
  labels:
    workload-type: inference
    environment: production
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: serving-a100-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 32          # Reserve 32 A100 GPUs for inference
        borrowingLimit: 16        # Can borrow 16 more if needed
      - name: "memory"
        nominalQuota: 8Ti
      - name: "cpu"
        nominalQuota: 512
  cohort: production-cohort
  priorityClassName: production-workloads
  allowBorrowing: true
  preemptionRule:
    reclaimWithinCohort: LowerPriority
    withinClusterQueue: LowerPriority
```

### Research and Development Reservation
Flexible reservation for R&D workloads:

```yaml
apiVersion: ome.io/v1beta1
kind: CapacityReservation
metadata:
  name: research-flexible-reservation
  labels:
    workload-type: research
    flexibility: high
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: research-mixed-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 16          # Base allocation
        borrowingLimit: 48        # Can borrow heavily for experiments
      - name: "memory"
        nominalQuota: 4Ti
        borrowingLimit: 8Ti
      - name: "cpu"
        nominalQuota: 256
        borrowingLimit: 512
  cohort: research-cohort
  priorityClassName: research-workloads
  allowBorrowing: true
  preemptionRule:
    reclaimWithinCohort: Any      # Flexible preemption for research
    borrowWithinCohort:
      policy: Any
```

## Status and Monitoring

### Capacity Tracking
Monitor resource allocation and usage:

```yaml
status:
  capacityReservationLifecycleState: Active
  capacity:
  - name: h100-flavor
    resources:
    - name: "nvidia.com/gpu"
      total: 64
    - name: "memory"
      total: 16Ti
    - name: "cpu"
      total: 1024
  allocatable:
  - name: h100-flavor
    resources:
    - name: "nvidia.com/gpu" 
      total: 56                   # 8 GPUs currently unavailable
    - name: "memory"
      total: 14Ti
    - name: "cpu"
      total: 896
```

### Association Usage
Track resource usage by different associations (DACs, workloads):

```yaml
status:
  associationUsages:
  - name: production-training-dac
    usage:
    - name: h100-flavor
      resources:
      - name: "nvidia.com/gpu"
        total: 32               # DAC using 32 GPUs
      - name: "memory"
        total: 8Ti
  - name: llm-inference-service
    usage:
    - name: h100-flavor
      resources:
      - name: "nvidia.com/gpu"
        total: 8               # Inference service using 8 GPUs
```

### Health Conditions
Monitor reservation health and operational status:

```yaml
status:
  conditions:
  - type: Ready
    status: "True"
    reason: "ReservationActive"
    message: "Capacity reservation is active and healthy"
  - type: ResourcesSufficient
    status: "True"
    reason: "CapacityAvailable"
    message: "Sufficient resources available for allocation"
  - type: DACAssociationsHealthy
    status: "True"
    reason: "AllDACsHealthy"
    message: "All associated DACs are in healthy state"
```

## Integration with DedicatedAICluster

### Linking DAC to Capacity Reservation
```yaml
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: training-cluster
spec:
  profile: h100-8gpu-profile
  count: 4
  capacityReservationId: "production-gpu-reservation"
  resources:
    requests:
      nvidia.com/gpu: 32
      memory: 8Ti
      cpu: 512
```

### Multi-DAC Reservation
One reservation can support multiple DACs:

```yaml
apiVersion: ome.io/v1beta1
kind: CapacityReservation
metadata:
  name: multi-dac-reservation
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: h100-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 128         # Support multiple DACs
      - name: "memory"
        nominalQuota: 32Ti
      - name: "cpu"
        nominalQuota: 2048
---
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: training-dac-1
spec:
  capacityReservationId: "multi-dac-reservation"
  count: 4
---
apiVersion: ome.io/v1beta1
kind: DedicatedAICluster
metadata:
  name: training-dac-2
spec:
  capacityReservationId: "multi-dac-reservation"
  count: 4
```

## ClusterCapacityReservation

For cluster-wide capacity reservations:

```yaml
apiVersion: ome.io/v1beta1
kind: ClusterCapacityReservation
metadata:
  name: cluster-wide-gpu-reservation
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: mixed-gpu-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 256         # Cluster-wide GPU pool
      - name: "memory"
        nominalQuota: 64Ti
      - name: "cpu"
        nominalQuota: 4096
  cohort: cluster-wide-cohort
  allowBorrowing: true
```

## Best Practices

### Resource Planning
Plan reservations based on workload patterns:

```yaml
metadata:
  name: workload-aware-reservation
  labels:
    planning-horizon: quarterly
    workload-pattern: burst
    cost-optimization: enabled
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: base-capacity-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 32          # Base capacity for continuous workloads
        borrowingLimit: 64        # Burst capacity for peak periods
```

### Multi-Tenancy
Organize reservations by teams or projects:

```yaml
apiVersion: ome.io/v1beta1
kind: CapacityReservation
metadata:
  name: team-a-reservation
  namespace: team-a
  labels:
    team: team-a
    cost-center: "12345"
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: team-a-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 16
  cohort: team-a-cohort
  priorityClassName: team-a-workloads
```

### Cost Optimization
Balance cost and performance:

```yaml
metadata:
  name: cost-optimized-reservation
  labels:
    cost-tier: spot
    optimization-strategy: aggressive
spec:
  resourceGroups:
  - coveredResources: ["nvidia.com/gpu", "memory", "cpu"]
    flavors:
    - name: spot-gpu-flavor
      resources:
      - name: "nvidia.com/gpu"
        nominalQuota: 64
        borrowingLimit: 128       # High borrowing for spot instances
  allowBorrowing: true
  preemptionRule:
    reclaimWithinCohort: Any      # Aggressive preemption for cost savings
```

## Monitoring and Observability

### Resource Utilization Metrics
```bash
# Check reservation status
kubectl get capacityreservation production-gpu-reservation

# View detailed resource allocation
kubectl describe capacityreservation production-gpu-reservation

# Monitor all reservations
kubectl get capacityreservations -A -o wide
```

### Usage Tracking
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: reservation-monitoring
data:
  prometheus-rules.yaml: |
    groups:
    - name: capacity-reservation-alerts
      rules:
      - alert: CapacityReservationUtilizationHigh
        expr: (sum(capacity_reservation_allocated) / sum(capacity_reservation_total)) > 0.9
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Capacity reservation utilization is high"
      - alert: CapacityReservationFailed
        expr: capacity_reservation_lifecycle_state != "Active"
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Capacity reservation is not in Active state"
```

## Troubleshooting

### Common Issues

**Reservation Stuck in Creating State**
```bash
# Check reservation conditions
kubectl describe capacityreservation my-reservation

# Verify Kueue configuration
kubectl get clusterqueues

# Check resource availability
kubectl describe nodes
```

**Resource Allocation Failures**
```bash
# Check borrowing limits
kubectl get capacityreservation my-reservation -o jsonpath='{.spec.resourceGroups[0].flavors[0].resources}'

# Verify cohort configuration
kubectl get capacityreservations -l cohort=my-cohort

# Check workload queue assignments
kubectl get workloads -A
```

**Preemption Issues**
```bash
# Check preemption rules
kubectl get capacityreservation my-reservation -o jsonpath='{.spec.preemptionRule}'

# Verify priority classes
kubectl get priorityclasses

# Check workload priorities
kubectl get workloads -o custom-columns=NAME:.metadata.name,PRIORITY:.spec.priorityClassName
```

### Debugging Commands

```bash
# List all capacity reservations
kubectl get capacityreservations -A

# Get detailed reservation status
kubectl describe capacityreservation reservation-name

# Check associated DACs
kubectl get dedicatedaiclusters -l capacity-reservation=reservation-name

# Monitor resource usage
kubectl get capacityreservation reservation-name -o jsonpath='{.status.associationUsages}'

# Check Kueue integration
kubectl get clusterqueues
kubectl get localqueues -A
```

## Security Considerations

### Access Control
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: capacity-reservation-user
rules:
- apiGroups: ["ome.io"]
  resources: ["capacityreservations"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: ["kueue.x-k8s.io"]
  resources: ["clusterqueues", "localqueues", "workloads"]
  verbs: ["get", "list", "watch"]
```

### Resource Isolation
```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: reservation-quota
  namespace: production
spec:
  hard:
    ome.io/capacity-reservations: "5"    # Limit number of reservations
    nvidia.com/gpu: "128"               # Total GPU limit
    requests.memory: "32Ti"
    requests.cpu: "2048"
``` 