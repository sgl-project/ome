---
title: "Training Job"
date: 2023-03-14
weight: 1
description: >
  TrainingJob manages distributed training workloads for fine-tuning and creating custom models.
---

A _TrainingJob_ is a resource that manages distributed training workloads for fine-tuning models and creating custom AI models. It supports various training frameworks and hyperparameter tuning configurations.

## Overview

TrainingJobs provide a declarative way to run distributed training workloads on Kubernetes. They integrate with training runtimes to provide framework-specific optimizations and support features like hyperparameter tuning, gang scheduling, and distributed training patterns.

## Example Configuration

```yaml
apiVersion: ome.io/v1beta1
kind: TrainingJob
metadata:
  name: llama-finetune
spec:
  runtimeRef:
    name: pytorch-runtime
    kind: ClusterTrainingRuntime
  trainer:
    image: "my-training-image:latest"
    command: ["python", "train.py"]
    args: ["--model", "llama-7b", "--epochs", "10"]
    numNodes: 4
    numProcPerNode: "2"
    resourcesPerNode:
      requests:
        cpu: "8"
        memory: "32Gi"
        nvidia.com/gpu: "2"
      limits:
        cpu: "8"
        memory: "32Gi"
        nvidia.com/gpu: "2"
  modelConfig:
    inputModel: "llama-7b-base"
    outputModel:
      storageUri: "oci://my-bucket/models/llama-7b-finetuned"
  datasets:
    storageUri: "oci://my-bucket/datasets/instruction-data"
  hyperParameterTuningConfig:
    method: "random"
    maxTrials: 10
    metric:
      name: "accuracy"
      goal: "maximize"
    parameters:
      learning_rate:
        type: "double"
        min: 0.0001
        max: 0.01
      batch_size:
        type: "int"
        values: [8, 16, 32]
```

## Spec Attributes

| Attribute                     | Description                                              | Required |
|------------------------------|----------------------------------------------------------|----------|
| `runtimeRef`                 | Reference to TrainingRuntime or ClusterTrainingRuntime  | Yes      |
| `trainer`                    | Trainer container and resource configuration             | Yes      |
| `modelConfig`                | Input and output model configuration                     | Yes      |
| `datasets`                   | Training datasets configuration                          | Yes      |
| `hyperParameterTuningConfig` | Hyperparameter tuning configuration                     | No       |
| `suspend`                    | Whether to suspend the running TrainingJob              | No       |
| `labels`                     | Labels to apply to derivative JobSet and Jobs           | No       |
| `annotations`                | Annotations to apply to derivative JobSet and Jobs      | No       |
| `compartmentID`              | OCI compartment ID for the training job                 | No       |

## Runtime Reference

TrainingJobs reference a training runtime that defines the execution environment:

```yaml
runtimeRef:
  name: pytorch-runtime
  kind: ClusterTrainingRuntime  # or TrainingRuntime
  apiGroup: ome.io  # defaults to ome.io
```

The runtime provides:
- Framework-specific configurations (PyTorch, MPI)
- Gang scheduling policies
- Resource management templates
- Container base configurations

## Trainer Configuration

The trainer defines the main training container and resources:

```yaml
trainer:
  image: "pytorch/pytorch:latest"
  command: ["python"]
  args: ["-m", "torch.distributed.launch", "train.py"]
  env:
    - name: "CUDA_VISIBLE_DEVICES"
      value: "0,1"
  numNodes: 2
  numProcPerNode: "gpu"  # or "auto", "cpu", or integer
  resourcesPerNode:
    requests:
      cpu: "4"
      memory: "16Gi"
      nvidia.com/gpu: "2"
```

### Number of Processes Per Node

- **"auto"**: Automatically determine based on available resources
- **"cpu"**: One process per CPU core
- **"gpu"**: One process per GPU
- **Integer**: Specific number of processes

## Model Configuration

Defines input and output models for the training job:

```yaml
modelConfig:
  inputModel: "llama-7b-base"  # Reference to BaseModel
  outputModel:
    storageUri: "oci://bucket/path/to/output"
    parameters:
      region: "us-phoenix-1"
```

The input model is used as the starting point for training, and the output model location stores the fine-tuned weights.

## Dataset Configuration

Defines the training datasets:

```yaml
datasets:
  storageUri: "oci://bucket/datasets/my-training-data"
  parameters:
    format: "jsonl"
    compression: "gzip"
  storageKey: "dataset-credentials"
```

Supported storage types:
- **OCI Object Storage**: `oci://namespace/bucket/path`
- **Persistent Volumes**: `pvc://pvc-name/path`
- **HTTP/HTTPS**: Direct download URLs

## Hyperparameter Tuning

TrainingJobs support hyperparameter optimization:

```yaml
hyperParameterTuningConfig:
  method: "random"  # or "grid", "bayes"
  maxTrials: 20
  metric:
    name: "val_accuracy"
    goal: "maximize"  # or "minimize"
  parameters:
    learning_rate:
      type: "double"
      min: 0.0001
      max: 0.01
      scale: "log"
    dropout_rate:
      type: "double"
      values: [0.1, 0.2, 0.3, 0.4, 0.5]
    batch_size:
      type: "int"
      min: 8
      max: 64
      step: 8
```

### Tuning Methods

- **grid**: Exhaustive grid search
- **random**: Random sampling
- **bayes**: Bayesian optimization

### Parameter Types

- **int**: Integer parameters with min/max/step
- **double**: Float parameters with min/max/scale
- **categorical**: Discrete choices from a list

## Status

The TrainingJob status provides detailed information about the training progress:

```yaml
status:
  jobsStatus:
    - name: "llama-finetune-job-0"
      ready: 1
      succeeded: 0
      failed: 0
      active: 1
      suspended: 0
  conditions:
    - type: "Running"
      status: "True"
      lastTransitionTime: "2023-03-14T10:00:00Z"
      reason: "JobStarted"
      message: "Training job is running"
  retryCount: 0
  startTime: "2023-03-14T10:00:00Z"
  lastReconcileTime: "2023-03-14T10:05:00Z"
```

### Condition Types

- **Running**: Training job is currently running
- **Succeeded**: Training job completed successfully
- **Failed**: Training job failed
- **Suspended**: Training job is suspended

## Training Frameworks

### PyTorch Training

```yaml
runtimeRef:
  name: pytorch-runtime
trainer:
  numProcPerNode: "gpu"
  command: ["python"]
  args: ["-m", "torch.distributed.launch", "--nproc_per_node=2", "train.py"]
```

### MPI Training

```yaml
runtimeRef:
  name: mpi-runtime
trainer:
  numProcPerNode: 4
  command: ["mpirun"]
  args: ["-np", "8", "python", "train.py"]
```

## Distributed Training Patterns

### Data Parallel

Multiple replicas of the model process different batches of data:

```yaml
trainer:
  numNodes: 4
  numProcPerNode: "2"
  # Each node processes different data batches
```

### Model Parallel

The model is split across multiple nodes:

```yaml
trainer:
  numNodes: 2
  numProcPerNode: "4"
  env:
    - name: "MODEL_PARALLEL_SIZE"
      value: "2"
```

## Integration with Storage

### Input Model Loading

The controller automatically mounts the input model:

1. Creates PersistentVolume from BaseModel storage
2. Mounts volume to training pods
3. Sets environment variables for model path

### Output Model Storage

Results are automatically stored to the specified location:

1. Training writes outputs to mounted volume
2. Controller uploads to specified storage URI
3. Creates FineTunedWeight resource if successful

## Best Practices

1. **Resource Planning**: Allocate sufficient GPU memory for your model size
2. **Checkpointing**: Enable periodic checkpointing for long-running jobs
3. **Monitoring**: Monitor GPU utilization and training metrics
4. **Hyperparameters**: Start with fewer trials before scaling up tuning
5. **Data Pipeline**: Optimize data loading to avoid GPU idle time

## Examples

### Simple Fine-tuning

```yaml
apiVersion: ome.io/v1beta1
kind: TrainingJob
metadata:
  name: simple-finetune
spec:
  runtimeRef:
    name: pytorch-runtime
  trainer:
    image: "pytorch/pytorch:latest"
    numNodes: 1
    numProcPerNode: "1"
    resourcesPerNode:
      requests:
        nvidia.com/gpu: "1"
  modelConfig:
    inputModel: "bert-base"
    outputModel:
      storageUri: "oci://bucket/bert-finetuned"
  datasets:
    storageUri: "oci://bucket/classification-data"
```

### Multi-node Training with Hyperparameter Tuning

```yaml
apiVersion: ome.io/v1beta1
kind: TrainingJob
metadata:
  name: large-model-training
spec:
  runtimeRef:
    name: pytorch-distributed-runtime
  trainer:
    numNodes: 8
    numProcPerNode: "gpu"
    resourcesPerNode:
      requests:
        nvidia.com/gpu: "4"
        memory: "64Gi"
  modelConfig:
    inputModel: "llama-70b"
    outputModel:
      storageUri: "oci://bucket/llama-70b-custom"
  datasets:
    storageUri: "oci://bucket/large-dataset"
  hyperParameterTuningConfig:
    method: "bayes"
    maxTrials: 50
    metric:
      name: "perplexity"
      goal: "minimize"
```

## Troubleshooting

### Training Job Not Starting

Check the runtime and resource availability:

```bash
kubectl describe trainingjob llama-finetune
kubectl get pods -l job-name=llama-finetune
```

Common issues:
- Insufficient GPU resources
- Runtime not found or invalid
- Dataset access issues

### Failed Training Runs

Check pod logs for training errors:

```bash
kubectl logs -l job-name=llama-finetune -c trainer
```

### Hyperparameter Tuning Issues

Monitor tuning progress and adjust parameters:

```bash
kubectl describe trainingjob llama-finetune
# Check conditions and trial status
``` 