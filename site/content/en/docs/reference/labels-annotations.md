---
title: "Labels and Annotations"
date: 2023-03-14
weight: 1
description: >
  Standard labels and annotations used by OME.
---

This page lists the labels and annotations that OME uses to manage and identify resources.

## Labels

### Standard Labels

These labels are used across all OME resources for consistent identification and management.

| Label | Description | Example |
|-------|-------------|---------|
| `ome.io/managed-by` | Identifies resources managed by OME | `ome-controller` |
| `ome.io/component` | Component type within OME | `predictor`, `trainer`, `router` |
| `ome.io/part-of` | Parent resource this belongs to | `llama-chat`, `bert-training` |
| `ome.io/version` | OME version that created the resource | `v0.8.0` |

### Inference Service Labels

| Label | Description | Example |
|-------|-------------|---------|
| `serving.ome.io/inferenceservice` | Name of the InferenceService | `llama-chat` |
| `serving.ome.io/component` | Serving component type | `predictor`, `router` |
| `serving.ome.io/revision` | Revision number | `00001`, `00002` |
| `serving.ome.io/model` | Base model name | `llama-7b-instruct` |
| `serving.ome.io/runtime` | Serving runtime used | `vllm-text-generation` |

### Training Job Labels

| Label | Description | Example |
|-------|-------------|---------|
| `training.ome.io/job-name` | Name of the TrainingJob | `llama-finetune` |
| `training.ome.io/job-uid` | UID of the TrainingJob | `abc123-def456-789` |
| `training.ome.io/replica-type` | Type of replica | `launcher`, `worker` |
| `training.ome.io/replica-index` | Index of the replica | `0`, `1`, `2` |
| `training.ome.io/framework` | Training framework | `pytorch`, `mpi` |

### Benchmark Job Labels

| Label | Description | Example |
|-------|-------------|---------|
| `benchmark.ome.io/job-name` | Name of the BenchmarkJob | `performance-test` |
| `benchmark.ome.io/target-service` | Target InferenceService | `llama-chat` |
| `benchmark.ome.io/task-type` | Type of benchmark task | `text-to-text` |

### Infrastructure Labels

| Label | Description | Example |
|-------|-------------|---------|
| `infrastructure.ome.io/cluster` | Dedicated AI Cluster name | `gpu-cluster-1` |
| `infrastructure.ome.io/profile` | Cluster profile used | `h100-profile` |
| `infrastructure.ome.io/capacity-reservation` | Capacity reservation ID | `cap-res-123` |

### Model Labels

| Label | Description | Example |
|-------|-------------|---------|
| `model.ome.io/name` | Model name | `llama-7b-instruct` |
| `model.ome.io/type` | Model type | `text-generation`, `embedding` |
| `model.ome.io/vendor` | Model vendor | `meta`, `openai`, `anthropic` |
| `model.ome.io/format` | Model format | `safetensors`, `onnx` |
| `model.ome.io/size` | Model parameter size | `7b`, `13b`, `70b` |

## Annotations

### Standard Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `ome.io/last-applied-configuration` | Last applied configuration | JSON configuration |
| `ome.io/resource-version` | Internal resource version | `12345` |

### Deployment Configuration Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `ome.io/deploymentMode` | Deployment mode for InferenceService | `Serverless`, `RawDeployment`, `MultiNodeRayVLLM` |
| `ome.io/min-replicas` | Minimum number of replicas | `1` |
| `ome.io/max-replicas` | Maximum number of replicas | `10` |
| `ome.io/target-utilization` | Target resource utilization | `80` |

### Autoscaling Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `autoscaling.ome.io/enable-keda` | Enable KEDA autoscaling | `true`, `false` |
| `autoscaling.ome.io/metric` | Autoscaling metric | `concurrency`, `rps`, `gpu-utilization` |
| `autoscaling.ome.io/target` | Target metric value | `10`, `100` |
| `autoscaling.knative.dev/metric` | Knative autoscaling metric | `concurrency`, `rps` |
| `autoscaling.knative.dev/target` | Knative target value | `10` |

### Model Configuration Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `model.ome.io/storage-uri` | Model storage location | `oci://bucket/models/llama` |
| `model.ome.io/quantization` | Model quantization type | `fp16`, `int8`, `int4` |
| `model.ome.io/max-tokens` | Maximum token length | `4096`, `8192` |
| `model.ome.io/context-length` | Model context length | `2048`, `4096` |

### Training Configuration Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `training.ome.io/distributed-backend` | Distributed training backend | `nccl`, `mpi`, `gloo` |
| `training.ome.io/checkpoint-strategy` | Checkpointing strategy | `epoch`, `step`, `time` |
| `training.ome.io/resume-from` | Resume from checkpoint | `checkpoint-path` |

### Performance Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `performance.ome.io/gpu-memory-fraction` | GPU memory usage fraction | `0.8`, `0.9` |
| `performance.ome.io/batch-size` | Inference batch size | `1`, `8`, `32` |
| `performance.ome.io/max-batch-delay` | Maximum batching delay | `10ms`, `100ms` |

### Security Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `security.ome.io/pod-security-policy` | Pod security policy | `restricted`, `baseline` |
| `security.ome.io/network-policy` | Network policy to apply | `allow-inference-only` |
| `security.ome.io/secret-ref` | Reference to security secret | `model-credentials` |

### Storage Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `storage.ome.io/class` | Storage class to use | `fast-ssd`, `standard` |
| `storage.ome.io/size` | Storage size requirement | `100Gi`, `1Ti` |
| `storage.ome.io/access-mode` | Storage access mode | `ReadWriteOnce`, `ReadOnlyMany` |

### Monitoring Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `monitoring.ome.io/scrape` | Enable Prometheus scraping | `true`, `false` |
| `monitoring.ome.io/port` | Metrics port | `8080`, `9090` |
| `monitoring.ome.io/path` | Metrics endpoint path | `/metrics`, `/stats` |

### OCI Specific Annotations

| Annotation | Description | Example |
|------------|-------------|---------|
| `oci.ome.io/compartment-id` | OCI compartment ID | `ocid1.compartment.oc1..` |
| `oci.ome.io/region` | OCI region | `us-phoenix-1`, `us-ashburn-1` |
| `oci.ome.io/availability-domain` | OCI availability domain | `PHX-AD-1` |

## Usage Examples

### InferenceService with Custom Annotations

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-chat
  annotations:
    ome.io/deploymentMode: "Serverless"
    autoscaling.ome.io/enable-keda: "true"
    autoscaling.ome.io/metric: "concurrency"
    autoscaling.ome.io/target: "10"
    model.ome.io/max-tokens: "4096"
    performance.ome.io/batch-size: "8"
    oci.ome.io/compartment-id: "ocid1.compartment.oc1.."
  labels:
    serving.ome.io/model: "llama-7b-instruct"
    model.ome.io/vendor: "meta"
    model.ome.io/size: "7b"
spec:
  # ... spec configuration
```

### TrainingJob with Labels

```yaml
apiVersion: ome.io/v1beta1
kind: TrainingJob
metadata:
  name: bert-finetune
  annotations:
    training.ome.io/distributed-backend: "nccl"
    training.ome.io/checkpoint-strategy: "epoch"
    oci.ome.io/compartment-id: "ocid1.compartment.oc1.."
  labels:
    training.ome.io/framework: "pytorch"
    model.ome.io/name: "bert-base"
    model.ome.io/type: "classification"
spec:
  # ... spec configuration
```

### BenchmarkJob with Performance Annotations

```yaml
apiVersion: ome.io/v1beta1
kind: BenchmarkJob
metadata:
  name: performance-test
  annotations:
    benchmark.ome.io/target-qps: "100"
    benchmark.ome.io/duration: "10m"
    monitoring.ome.io/scrape: "true"
  labels:
    benchmark.ome.io/target-service: "llama-chat"
    benchmark.ome.io/task-type: "text-to-text"
spec:
  # ... spec configuration
```

## Selector Usage

These labels can be used with kubectl and other Kubernetes tools for resource selection:

```bash
# Get all inference services for a specific model
kubectl get inferenceservice -l serving.ome.io/model=llama-7b-instruct

# Get all training jobs using PyTorch
kubectl get trainingjob -l training.ome.io/framework=pytorch

# Get all resources managed by OME
kubectl get all -l ome.io/managed-by=ome-controller

# Get all benchmark jobs targeting a specific service
kubectl get benchmarkjob -l benchmark.ome.io/target-service=llama-chat
```

## Best Practices

1. **Consistency**: Use standard labels across all related resources
2. **Namespace**: Use appropriate prefixes (`ome.io`, `serving.ome.io`, etc.)
3. **Immutability**: Avoid changing critical labels after resource creation
4. **Documentation**: Document custom labels and annotations in your deployments
5. **Validation**: Validate label values match expected formats and constraints 