---
title: "Inference Service"
date: 2023-03-14
weight: 30
description: >
  InferenceService is the primary resource that manages the deployment and serving of machine learning models in OME.
---

## What is an InferenceService?

An InferenceService is the central Kubernetes resource in OME that orchestrates the complete lifecycle of model serving. It acts as a declarative specification that describes how you want your AI models deployed, scaled, and served across your cluster.

Think of InferenceService as the "deployment blueprint" for your AI workloads. It brings together models (defined by BaseModel/ClusterBaseModel), runtimes (defined by ServingRuntime/ClusterServingRuntime), and infrastructure configuration to create a complete serving solution.

## Architecture Overview

OME uses a **component-based architecture** where InferenceService can be composed of multiple specialized components:

- **Model**: References the AI model to serve (BaseModel/ClusterBaseModel)
- **Runtime**: References the serving runtime environment (ServingRuntime/ClusterServingRuntime)  
- **Engine**: Main inference component that processes requests
- **Decoder**: Optional component for disaggregated serving (prefill-decode separation)
- **Router**: Optional component for request routing and load balancing

### New vs Deprecated Architecture

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
spec:
  model:
    name: llama-3-70b-instruct
  runtime:
    name: vllm-text-generation
  engine:
    minReplicas: 1
    maxReplicas: 3
    resources:
      requests:
        nvidia.com/gpu: "1"
```

## Component Types

### Engine Component

The **Engine** is the primary inference component that processes model requests. It handles model loading, inference execution, and response generation.

```yaml
spec:
  engine:
    # Pod-level configuration
    serviceAccountName: custom-sa
    nodeSelector:
      accelerator: nvidia-a100
    
    # Component configuration  
    minReplicas: 1
    maxReplicas: 10
    scaleMetric: cpu
    scaleTarget: 70
    
    # Container configuration
    runner:
      image: custom-vllm:latest
      resources:
        requests:
          nvidia.com/gpu: "2"
        limits:
          nvidia.com/gpu: "2"
      env:
        - name: CUDA_VISIBLE_DEVICES
          value: "0,1"
```

### Decoder Component

The **Decoder** is used for disaggregated serving architectures where the prefill (prompt processing) and decode (token generation) phases are separated for better resource utilization.

```yaml
spec:
  decoder:
    minReplicas: 2
    maxReplicas: 8
    runner:
      resources:
        requests:
          cpu: "4"
          memory: "8Gi"
```

### Router Component

The **Router** handles request routing, cache awareness load balancing, or prefill and decode disaggregation load balancing.

```yaml
spec:
  router:
    minReplicas: 1
    maxReplicas: 3
    config:
      routing_strategy: "round_robin"
      health_check_interval: "30s"
    runner:
      resources:
        requests:
          cpu: "1"
          memory: "2Gi"
```

## Deployment Modes

OME automatically selects the optimal deployment mode based on your configuration:

| Mode                              | Description                                 | Use Cases                                                         | Infrastructure                                                                |
|-----------------------------------|---------------------------------------------|-------------------------------------------------------------------|-------------------------------------------------------------------------------|
| **Raw Deployment**                | Standard Kubernetes Deployment              | Stable workloads, predictable traffic, no cold starts             | Kubernetes Deployments + Services                                             |
| **Serverless**                    | Knative-based auto-scaling                  | Variable workloads, cost optimization, scale-to-zero              | Knative Serving                                                               |
| **Multi-Node**                    | Distributed inference across multiple nodes | Large models (DeepSeek), models that can not fit in a single node | LeaderWorkerSet                                                               |
| **Prefill-Decode Disaggregation** | Disaggregated serving architecture          | Maximizing resource utilization, better performance,              | Raw Deployments or LeaderWorkerSet(if the model can not fit in a single node) |

### Raw Deployment Mode (Default)

Uses standard Kubernetes Deployments with full control over pod lifecycle and scaling.

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-chat
spec:
  model:
    name: llama-3-70b-instruct
  engine:
    minReplicas: 2
    maxReplicas: 10
```

This deployment mode offers direct Kubernetes management with standard HPA-based autoscaling, no cold starts, and is ideal for stable, predictable workloads.

### Serverless Mode

Leverages Knative Serving for automatic scaling including scale-to-zero capabilities.

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService  
metadata:
  name: llama-chat
spec:
  model:
    name: llama-3-70b-instruct
  engine:
    minReplicas: 0  # Enables scale-to-zero
    maxReplicas: 10
    scaleTarget: 10  # Concurrent requests per pod
```
This deployment mode leverages Knative Serving for request-based autoscaling, scale-to-zero when idle, and is ideal for variable workloads and cost-sensitive environments.

> **⚠️ WARNING**: This deployment mode leverages Knative Serving for request-based autoscaling, scale-to-zero when idle, and is ideal for variable workloads and cost-sensitive environments; however, it may introduce additional startup latency for large language models due to cold starts and model loading time.

### Multi-Node Mode

Enables distributed model serving across multiple nodes using LeaderWorkerSet or Ray clusters.

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: deepseek-chat
spec:
  model:
    name: deepseek-r1  # Large model requiring multiple GPUs
  engine:
    minReplicas: 1
    maxReplicas: 2
    # Worker node configuration  
    worker:
      size: 1  # Number of worker nodes
```
This deployment mode enables distributed inference using LeaderWorkerSet or Ray, with support for multi-GPU and multi-node setups, and is optimized for large language models through automatic coordination between nodes

> **⚠️ WARNING**: Multi-node configurations typically require high-performance networking such as RoCE or InfiniBand, and performance may vary depending on the underlying network topology and hardware provided by different cloud vendors.

### Disaggregated Serving (Prefill-Decode)

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: deepseek-ep-disaggregated
spec:
  model:
    name: deepseek-r1
  
  # Router handles request routing and load balancing for prefill-decode disaggregation
  router:
    minReplicas: 1
    maxReplicas: 3
  
  # Engine handles prefill phase
  engine:
    minReplicas: 1
    maxReplicas: 3
  
  # Decoder handles token generation
  decoder:
    minReplicas: 2
    maxReplicas: 8
```

## Specification Reference

| Attribute           | Type              | Description                                              |
|---------------------|-------------------|----------------------------------------------------------|
| **Core References** |                   |                                                          |
| `model`             | ModelRef          | Reference to BaseModel/ClusterBaseModel to serve         |
| `runtime`           | ServingRuntimeRef | Reference to ServingRuntime/ClusterServingRuntime to use |
| **Components**      |                   |                                                          |
| `engine`            | EngineSpec        | Main inference component configuration                   |
| `decoder`           | DecoderSpec       | Optional decoder component for disaggregated serving     |
| `router`            | RouterSpec        | Optional router component for request routing            |
| **Autoscaling**     |                   |                                                          |
| `kedaConfig`        | KedaConfig        | KEDA event-driven autoscaling configuration              |

### ModelRef Specification

| Attribute          | Type     | Description                                    |
|--------------------|----------|------------------------------------------------|
| `name`             | string   | Name of the BaseModel/ClusterBaseModel         |
| `kind`             | string   | Resource kind (defaults to "ClusterBaseModel") |
| `apiGroup`         | string   | API group (defaults to "ome.io")               |
| `fineTunedWeights` | []string | Optional fine-tuned weight references          |

### ServingRuntimeRef Specification

| Attribute  | Type   | Description                                         |
|------------|--------|-----------------------------------------------------|
| `name`     | string | Name of the ServingRuntime/ClusterServingRuntime    |
| `kind`     | string | Resource kind (defaults to "ClusterServingRuntime") |
| `apiGroup` | string | API group (defaults to "ome.io")                    |

### Component Configuration

All components (Engine, Decoder, Router) share this common configuration structure:

| Attribute                  | Type               | Description                                               |
|----------------------------|--------------------|-----------------------------------------------------------|
| **Pod Configuration**      |                    |                                                           |
| `serviceAccountName`       | string             | Service account for the component pods                    |
| `nodeSelector`             | map[string]string  | Node labels for pod placement                             |
| `tolerations`              | []Toleration       | Pod tolerations for tainted nodes                         |
| `affinity`                 | Affinity           | Pod affinity and anti-affinity rules                      |
| `volumes`                  | []Volume           | Additional volumes to mount                               |
| `containers`               | []Container        | Additional sidecar containers                             |
| **Scaling Configuration**  |                    |                                                           |
| `minReplicas`              | int                | Minimum number of replicas (default: 1)                   |
| `maxReplicas`              | int                | Maximum number of replicas                                |
| `scaleTarget`              | int                | Target value for autoscaling metric                       |
| `scaleMetric`              | string             | Metric to use for scaling (cpu, memory, concurrency, rps) |
| `containerConcurrency`     | int64              | Maximum concurrent requests per container                 |
| `timeoutSeconds`           | int64              | Request timeout in seconds                                |
| **Traffic Management**     |                    |                                                           |
| `canaryTrafficPercent`     | int64              | Percentage of traffic to route to canary version          |
| **Resource Configuration** |                    |                                                           |
| `runner`                   | RunnerSpec         | Main container configuration                              |
| `leader`                   | LeaderSpec         | Leader node configuration (multi-node only)               |
| `worker`                   | WorkerSpec         | Worker node configuration (multi-node only)               |
| **Deployment Strategy**    |                    |                                                           |
| `deploymentStrategy`       | DeploymentStrategy | Kubernetes deployment strategy (RawDeployment only)       |
| **KEDA Configuration**     |                    |                                                           |
| `kedaConfig`               | KedaConfig         | Component-specific KEDA configuration                     |

### RunnerSpec Configuration

| Attribute      | Type                 | Description                                |
|----------------|----------------------|--------------------------------------------|
| `name`         | string               | Container name                             |
| `image`        | string               | Container image                            |
| `command`      | []string             | Container command                          |
| `args`         | []string             | Container arguments                        |
| `env`          | []EnvVar             | Environment variables                      |
| `resources`    | ResourceRequirements | CPU, memory, and GPU resource requirements |
| `volumeMounts` | []VolumeMount        | Volume mount points                        |

### KEDA Configuration

| Attribute           | Type   | Description                                               |
|---------------------|--------|-----------------------------------------------------------|
| `enableKeda`        | bool   | Whether to enable KEDA autoscaling                        |
| `promServerAddress` | string | Prometheus server URL for metrics                         |
| `customPromQuery`   | string | Custom Prometheus query for scaling                       |
| `scalingThreshold`  | string | Threshold value for scaling decisions                     |
| `scalingOperator`   | string | Comparison operator (GreaterThanOrEqual, LessThanOrEqual) |


## Status and Monitoring

### InferenceService Status

The InferenceService status provides comprehensive information about the deployment state:

```yaml
status:
  url: "http://llama-chat.default.example.com"
  address:
    url: "http://llama-chat.default.svc.cluster.local"
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2024-01-15T10:30:00Z"
    - type: IngressReady
      status: "True"
      lastTransitionTime: "2024-01-15T10:25:00Z"
  components:
    engine:
      url: "http://llama-chat-engine.default.example.com"
      latestReadyRevision: "llama-chat-engine-00001"
      latestCreatedRevision: "llama-chat-engine-00001"
      traffic:
        - revisionName: "llama-chat-engine-00001"
          percent: 100
          latestRevision: true
    router:
      url: "http://llama-chat-router.default.example.com"
      latestReadyRevision: "llama-chat-router-00001"
  modelStatus:
    transitionStatus: "UpToDate"
    modelRevisionStates:
      activeModelState: "Loaded"
      targetModelState: "Loaded"
```

### Condition Types

| Condition        | Description                                 |
|------------------|---------------------------------------------|
| `Ready`          | Overall readiness of the InferenceService   |
| `IngressReady`   | Network routing is configured and ready     |
| `EngineReady`    | Engine component is ready to serve requests |
| `DecoderReady`   | Decoder component is ready (if configured)  |
| `RouterReady`    | Router component is ready (if configured)   |
| `PredictorReady` | **Deprecated**: Legacy predictor readiness  |

### Model Status States

| State          | Description                             |
|----------------|-----------------------------------------|
| `Pending`      | Model is not yet registered             |
| `Standby`      | Model is available but not loaded       |
| `Loading`      | Model is currently loading              |
| `Loaded`       | Model is loaded and ready for inference |
| `FailedToLoad` | Model failed to load                    |

## Deployment Mode Selection

Choose the appropriate deployment mode based on your requirements:

| Requirement                         | Recommended Mode |
|-------------------------------------|------------------|
| Stable, predictable load            | Raw Deployment   |
| No cold starts                      | Raw Deployment   |
| Variable workload                   | Serverless       |
| Cost optimization                   | Serverless       |
| Scale-to-zero capability            | Serverless       |
| Large model requiring multiple GPUs | Multi-Node       |
| Distributed inference               | Multi-Node       |
| Maximum performance                 | Multi-Node       |

## Best Practices

### Resource Management

1. **GPU Allocation**: Always specify GPU resources explicitly
```yaml
runner:
  resources:
    requests:
      nvidia.com/gpu: "1"
    limits:
      nvidia.com/gpu: "1"
```

2. **Memory Sizing**: Allow 2-4x model size for memory
```yaml
runner:
  resources:
    requests:
      memory: "32Gi"  # For 8B parameter model
```

3. **CPU Allocation**: Provide adequate CPU for preprocessing
```yaml
runner:
  resources:
    requests:
      cpu: "4"
```

### Scaling Configuration

1. **Set Appropriate Limits**:
```yaml
engine:
  minReplicas: 1     # Prevent scale-to-zero for latency
  maxReplicas: 10    # Control costs
  scaleTarget: 70    # 70% CPU utilization target
```

2. **Use KEDA for Custom Metrics**:
```yaml
kedaConfig:
  enableKeda: true
  customPromQuery: "avg_over_time(vllm:request_latency_seconds{service='%s'}[5m])"
  scalingThreshold: "0.5"  # 500ms latency threshold
```

### Troubleshooting

1. **Check Component Status**:
```bash
kubectl get inferenceservice llama-chat -o yaml
kubectl describe inferenceservice llama-chat
```

2. **Monitor Pod Logs**:
```bash
kubectl logs -l serving.ome.io/inferenceservice=llama-chat
```

3. **Check Resource Usage**:
```bash
kubectl top pods -l serving.ome.io/inferenceservice=llama-chat
```

