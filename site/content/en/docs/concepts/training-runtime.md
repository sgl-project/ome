---
title: "TrainingRuntime"
linkTitle: "TrainingRuntime"
weight: 60
description: >
  Understanding training runtimes in OME for distributed training workloads
---

TrainingRuntime defines the execution environment and configuration for distributed training workloads in OME. It provides templates and policies for creating JobSets that orchestrate multi-node training jobs.

## Overview

TrainingRuntime enables you to:

- **Define Training Environments**: Specify container configurations, frameworks, and runtime parameters
- **Configure Distributed Training**: Set up PyTorch, MPI, or other distributed training frameworks
- **Enable Gang Scheduling**: Coordinate resource allocation across multiple nodes
- **Support Elastic Training**: Allow dynamic scaling of training workloads
- **Standardize Deployments**: Create reusable templates for consistent training environments

## TrainingRuntime Specification

### Basic Structure

```yaml
apiVersion: ome.io/v1beta1
kind: TrainingRuntime
metadata:
  name: pytorch-distributed
  namespace: training
spec:
  mlPolicy:
    numNodes: 4
    torch:
      numProcPerNode: "auto"
  podGroupPolicy:
    coscheduling:
      scheduleTimeoutSeconds: 300
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            parallelism: 4
            completions: 4
            template:
              spec:
                containers:
                - name: pytorch
                  image: pytorch/pytorch:2.0.1-cuda11.7-cudnn8-devel
                  command: ["torchrun"]
                  args: ["--nnodes=4", "--nproc_per_node=auto", "train.py"]
```

### Key Components

#### ML Policy Configuration
Defines machine learning-specific parameters:

```yaml
spec:
  mlPolicy:
    numNodes: 8                    # Number of training nodes
    torch:                         # PyTorch configuration
      numProcPerNode: "gpu"        # Processes per node: auto, cpu, gpu, or integer
      elasticPolicy:               # Optional: Enable elastic training
        minNodes: 2
        maxNodes: 16
        maxRestarts: 3
```

#### Pod Group Policy
Enables gang scheduling for coordinated resource allocation:

```yaml
spec:
  podGroupPolicy:
    coscheduling:
      scheduleTimeoutSeconds: 600  # Timeout for scheduling all pods
```

#### JobSet Template
Defines the structure of training jobs:

```yaml
spec:
  template:
    metadata:
      labels:
        training-framework: pytorch
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                containers:
                - name: trainer
                  image: training-image:latest
                  resources:
                    requests:
                      nvidia.com/gpu: 1
                    limits:
                      nvidia.com/gpu: 1
```

## Distributed Training Frameworks

### PyTorch Distributed Training

#### Basic PyTorch Configuration
```yaml
apiVersion: ome.io/v1beta1
kind: TrainingRuntime
metadata:
  name: pytorch-ddp
spec:
  mlPolicy:
    numNodes: 4
    torch:
      numProcPerNode: "gpu"        # Use all available GPUs
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            parallelism: 4
            completions: 4
            template:
              spec:
                containers:
                - name: pytorch-trainer
                  image: pytorch/pytorch:2.0.1-cuda11.7-cudnn8-devel
                  command: ["torchrun"]
                  args:
                  - "--nnodes=4"
                  - "--nproc_per_node=auto"
                  - "--rdzv_backend=c10d"
                  - "--rdzv_endpoint=$MASTER_ADDR:$MASTER_PORT"
                  - "train.py"
                  env:
                  - name: NCCL_DEBUG
                    value: INFO
                  resources:
                    requests:
                      nvidia.com/gpu: 4
                    limits:
                      nvidia.com/gpu: 4
```

#### Elastic PyTorch Training
Supports dynamic scaling during training:

```yaml
apiVersion: ome.io/v1beta1
kind: TrainingRuntime
metadata:
  name: pytorch-elastic
spec:
  mlPolicy:
    torch:
      numProcPerNode: "gpu"
      elasticPolicy:
        minNodes: 2
        maxNodes: 8
        maxRestarts: 5
        metrics:
        - type: Resource
          resource:
            name: cpu
            target:
              type: Utilization
              averageUtilization: 80
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                containers:
                - name: elastic-trainer
                  image: pytorch/pytorch:2.0.1-cuda11.7-cudnn8-devel
                  command: ["torchrun"]
                  args:
                  - "--nnodes=2:8"
                  - "--nproc_per_node=auto"
                  - "--max_restarts=5"
                  - "elastic_train.py"
```

### MPI-based Training

#### OpenMPI Configuration
```yaml
apiVersion: ome.io/v1beta1
kind: TrainingRuntime
metadata:
  name: mpi-horovod
spec:
  mlPolicy:
    numNodes: 4
    mpi:
      numProcPerNode: 4
      mpiImplementation: OpenMPI
      sshAuthMountPath: "/root/.ssh"
      runLauncherAsNode: false
  template:
    spec:
      replicatedJobs:
      - name: launcher
        template:
          spec:
            parallelism: 1
            completions: 1
            template:
              spec:
                containers:
                - name: mpi-launcher
                  image: horovod/horovod:0.28.1-tf2.11.0-torch1.13.1-mxnet1.9.1-py3.8-cuda11.8
                  command: ["mpirun"]
                  args:
                  - "-np"
                  - "16"  # 4 nodes * 4 processes
                  - "--hostfile"
                  - "/etc/mpi/hostfile"
                  - "python"
                  - "train.py"
      - name: worker
        template:
          spec:
            parallelism: 4
            completions: 4
            template:
              spec:
                containers:
                - name: mpi-worker
                  image: horovod/horovod:0.28.1-tf2.11.0-torch1.13.1-mxnet1.9.1-py3.8-cuda11.8
                  command: ["/usr/sbin/sshd", "-D"]
```

#### Intel MPI Configuration
```yaml
spec:
  mlPolicy:
    numNodes: 8
    mpi:
      numProcPerNode: 2
      mpiImplementation: Intel
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                containers:
                - name: intel-mpi-trainer
                  image: intel/oneapi-hpckit:2023.2.0-devel-ubuntu20.04
                  command: ["mpirun"]
                  args:
                  - "-n"
                  - "16"
                  - "python"
                  - "train.py"
```

## Gang Scheduling

### Co-scheduling Configuration
Ensures all training pods are scheduled simultaneously:

```yaml
spec:
  podGroupPolicy:
    coscheduling:
      scheduleTimeoutSeconds: 600   # Wait up to 10 minutes for all pods
```

### Integration with Volcano
For environments using Volcano scheduler:

```yaml
# Note: Future support for Volcano configuration
spec:
  podGroupPolicy:
    volcano:
      minMember: 4
      queue: "training-queue"
      priorityClassName: "high-priority"
```

## Resource Management

### GPU Allocation
```yaml
spec:
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                nodeSelector:
                  accelerator: nvidia-tesla-v100
                containers:
                - name: trainer
                  resources:
                    requests:
                      nvidia.com/gpu: 8
                    limits:
                      nvidia.com/gpu: 8
                      memory: 64Gi
                      cpu: 16
```

### RDMA and High-Performance Networking
```yaml
spec:
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                nodeSelector:
                  network: rdma-enabled
                containers:
                - name: trainer
                  securityContext:
                    capabilities:
                      add: ["IPC_LOCK"]
                  volumeMounts:
                  - name: rdma-devices
                    mountPath: /dev/infiniband
                volumes:
                - name: rdma-devices
                  hostPath:
                    path: /dev/infiniband
```

## ClusterTrainingRuntime

For cluster-wide training runtime definitions:

```yaml
apiVersion: ome.io/v1beta1
kind: ClusterTrainingRuntime
metadata:
  name: global-pytorch-runtime
spec:
  mlPolicy:
    torch:
      numProcPerNode: "auto"
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                containers:
                - name: pytorch-trainer
                  image: pytorch/pytorch:latest
                  command: ["torchrun"]
                  args: ["--nnodes=${NUM_NODES}", "--nproc_per_node=auto", "train.py"]
```

## Best Practices

### Resource Optimization
```yaml
metadata:
  name: optimized-training-runtime
  labels:
    ome.io/optimization-level: high
    ome.io/gpu-type: h100
spec:
  mlPolicy:
    numNodes: 4
    torch:
      numProcPerNode: "gpu"
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                containers:
                - name: trainer
                  env:
                  - name: NCCL_ALGO
                    value: Ring
                  - name: NCCL_PROTO
                    value: Simple
                  - name: CUDA_VISIBLE_DEVICES
                    value: "0,1,2,3,4,5,6,7"
```

### Fault Tolerance
```yaml
spec:
  mlPolicy:
    torch:
      elasticPolicy:
        maxRestarts: 3
  template:
    spec:
      failurePolicy:
        maxRestarts: 3
      replicatedJobs:
      - name: worker
        template:
          spec:
            backoffLimit: 3
            template:
              spec:
                restartPolicy: OnFailure
```

### Environment Configuration
```yaml
spec:
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                containers:
                - name: trainer
                  env:
                  - name: PYTHONPATH
                    value: "/app:/app/src"
                  - name: TORCH_DISTRIBUTED_DEBUG
                    value: "DETAIL"
                  - name: NCCL_SOCKET_IFNAME
                    value: "eth0"
                  volumeMounts:
                  - name: training-code
                    mountPath: /app
                  - name: datasets
                    mountPath: /data
                volumes:
                - name: training-code
                  configMap:
                    name: training-scripts
                - name: datasets
                  persistentVolumeClaim:
                    claimName: training-data
```

## Monitoring and Observability

### Built-in Metrics
```yaml
metadata:
  annotations:
    ome.io/metrics-enabled: "true"
    ome.io/profiling-enabled: "true"
spec:
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                containers:
                - name: trainer
                  ports:
                  - name: metrics
                    containerPort: 8080
                  - name: profiler
                    containerPort: 8081
```

### Logging Configuration
```yaml
spec:
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                containers:
                - name: trainer
                  env:
                  - name: LOG_LEVEL
                    value: INFO
                  - name: LOG_FORMAT
                    value: json
                  volumeMounts:
                  - name: logs
                    mountPath: /var/log/training
                volumes:
                - name: logs
                  emptyDir: {}
```

## Troubleshooting

### Common Issues

**Gang Scheduling Timeouts**
```bash
# Check PodGroup status
kubectl get podgroups -A

# Verify resource availability
kubectl describe nodes

# Check scheduler logs
kubectl logs -n kube-system scheduler-plugins-scheduler
```

**Network Configuration Problems**
```bash
# Verify RDMA devices
kubectl exec -it training-pod -- ls /dev/infiniband/

# Check NCCL configuration
kubectl exec -it training-pod -- nvidia-smi topo -m

# Test inter-node connectivity
kubectl exec -it training-pod -- nccl-test all-reduce
```

**Resource Allocation Issues**
```bash
# Check GPU allocation
kubectl describe node gpu-node-1

# Verify container resources
kubectl describe pod training-worker-0

# Check resource quotas
kubectl get resourcequota -A
```

### Debugging Commands

```bash
# List training runtimes
kubectl get trainingruntimes -A

# Get runtime details
kubectl describe trainingruntime pytorch-runtime

# Check JobSet creation
kubectl get jobsets -A

# Monitor training progress
kubectl logs -f job/training-worker -c trainer
```

## Security Considerations

### RBAC Configuration
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: training-runtime-user
rules:
- apiGroups: ["ome.io"]
  resources: ["trainingruntimes", "clustertrainingruntimes"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: ["jobset.x-k8s.io"]
  resources: ["jobsets"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

### Network Security
```yaml
spec:
  template:
    spec:
      replicatedJobs:
      - name: worker
        template:
          spec:
            template:
              spec:
                securityContext:
                  runAsNonRoot: true
                  runAsUser: 1000
                  fsGroup: 1000
                containers:
                - name: trainer
                  securityContext:
                    allowPrivilegeEscalation: false
                    readOnlyRootFilesystem: true
                    capabilities:
                      drop: ["ALL"]
``` 