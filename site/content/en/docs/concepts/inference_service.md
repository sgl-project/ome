---
title: "Inference Service"
date: 2023-03-14
weight: 5
description: >
  InferenceService is a resource that manages the deployment and serving of machine learning models.
---

An _InferenceService_ is the primary resource in OME that manages the lifecycle of model serving. It orchestrates the deployment of models, handles scaling, and provides endpoints for model inference.

## Core Components

An InferenceService consists of several key components:

1. **Predictor**: The main component that handles model serving
2. **Model Configuration**: Defines the model and runtime settings
3. **Serving Configuration**: Controls how the model is served and scaled

## Example Configuration

Here's an example of an InferenceService configuration:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-chat
spec:
  predictor:
    model:
      baseModel: llama-2-70b
      runtime: vllm-text-generation
      modelConfiguration:
        maxTokens: 4096
        temperature: 0.7
    resources:
      requests:
        cpu: "4"
        memory: "16Gi"
        nvidia.com/gpu: "1"
      limits:
        cpu: "4"
        memory: "16Gi"
        nvidia.com/gpu: "1"
```

## Spec Attributes

Available attributes in the InferenceService spec:

| Attribute                   | Description                                       |
|-----------------------------|---------------------------------------------------|
| `predictor`                 | Required. Defines the model serving configuration |
| `predictor.model`           | Specifies the model and runtime to use            |
| `predictor.model.baseModel` | References a BaseModel/ClusterBaseModel resource  |
| `predictor.model.runtime`   | References a ServingRuntime/ClusterServingRuntime |
| `predictor.resources`       | Resource requirements for the serving pod         |
| `kedaConfig`                | Optional. Configures autoscaling behavior         |
| `compartmentID`             | Optional. Specifies the OCI compartment ID        |

## Runtime and InferenceService Integration

When an InferenceService is deployed, the controller merges configurations from multiple sources:

1. **Runtime Selection**:
   - If `predictor.model.runtime` is specified, the controller looks for a matching ServingRuntime/ClusterServingRuntime
   - If not specified, the controller automatically selects a compatible runtime based on:
     - Model format compatibility
     - Model architecture support
     - Model size requirements
     - Runtime priority settings

2. **Container Configuration Merging**:
   - Base container configuration comes from the ServingRuntime
   - InferenceService's predictor configuration overrides runtime defaults:
     - Resources (CPU, memory, GPU)
     - Environment variables
     - Volume mounts
     - Command and arguments

3. **Model Integration**:
   - Model path and format from BaseModel/ClusterBaseModel
   - Runtime-specific model configuration from InferenceService
   - Storage configuration for model artifacts

Example of configuration merging:

```yaml
# ServingRuntime configuration
apiVersion: ome.io/v1beta1
kind: ServingRuntime
spec:
  containers:
    - name: ome-container
      image: vllm-base:latest
      env:
        - name: DEFAULT_MODEL_PATH
          value: /mnt/models
---
# InferenceService overrides
apiVersion: ome.io/v1beta1
kind: InferenceService
spec:
  predictor:
    model:
      runtime: vllm-text-generation
      modelConfiguration:
        maxTokens: 4096
    resources:
      limits:
        nvidia.com/gpu: "1"
```

The resulting deployment will:
1. Use the base image from ServingRuntime
2. Keep default environment variables from ServingRuntime
3. Apply resource limits from InferenceService
4. Configure model-specific parameters from InferenceService

## Reconciliation Process

The InferenceService controller performs several steps during reconciliation:

1. **Base Model Resolution**:
   - Locates the specified BaseModel/ClusterBaseModel
   - Validates model format and capabilities

2. **Runtime Selection**:
   - Finds a compatible ServingRuntime/ClusterServingRuntime
   - Validates runtime support for the model

3. **Storage Preparation**:
   - Creates PersistentVolume and PersistentVolumeClaim
   - Configures model storage and access

4. **Deployment Management**:
   - Creates and manages Kubernetes resources (Deployments, Services)
   - Configures networking and routing

5. **Scaling Configuration**:
   - Sets up KEDA ScaledObjects for autoscaling
   - Configures HPA (Horizontal Pod Autoscaler) if specified

## Status

The InferenceService status provides information about the deployment:

```yaml
status:
  url: http://llama-chat.default.example.com
  conditions:
    - type: Ready
      status: "True"
  components:
    predictor:
      url: http://llama-chat-predictor.default.example.com
      latestReadyRevision: llama-chat-predictor-00001
```

## Deployment Modes

OME supports three deployment modes for inference services, each optimized for different use cases:

### Raw Deployment Mode (Default)

Raw deployment mode uses standard Kubernetes Deployments for more control:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-chat
  annotations:
    ome.io/deploymentMode: "RawDeployment"
spec:
  predictor:
    model:
      baseModel: llama-2-70b
      runtime: vllm-text-generation
```

Key features:
- Direct control over pod lifecycle
- Standard Kubernetes HPA support
- No cold starts
- Best for stable, predictable workloads

### Serverless Mode

Serverless mode leverages Knative for automatic scaling and serverless capabilities:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-chat
  annotations:
    ome.io/deploymentMode: "Serverless"
spec:
  predictor:
    model:
      baseModel: llama-2-70b
      runtime: vllm-text-generation
```

Key features:
- Automatic scaling based on request load
- Scale-to-zero when idle
- Request-based autoscaling
- Best for variable workloads and cost optimization

### Multi-node Ray vLLM Mode

Multi-node Ray vLLM mode enables distributed model serving across multiple nodes:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-chat
  annotations:
    ome.io/deploymentMode: "MultiNodeRayVLLM"
spec:
  predictor:
    model:
      baseModel: llama-2-70b
      runtime: vllm-text-generation
    workerSpec:
      worldSize: 4  # Number of worker nodes
      resources:
        requests:
          nvidia.com/gpu: "1"
        limits:
          nvidia.com/gpu: "1"
```

Key features:
- Distributed model serving using Ray clusters and vLLM
- Multi-GPU and multi-node support
- Optimized for large language models that require multiple nodes

## Deployment Mode Selection

Choose the deployment mode based on your requirements:

| Requirement                     | Recommended Mode          |
|--------------------------------|--------------------------|
| Stable, predictable load       | Raw Deployment           |
| No cold starts                 | Raw Deployment           |
| Variable workload              | Serverless               |
| Cost optimization              | Serverless               |
| Multi-GPU inference           | Multi-node Ray vLLM      |

## Traffic Management

InferenceService supports traffic management features:

- **Canary Deployments**: Gradually roll out new model versions
- **A/B Testing**: Test different model configurations
- **Traffic Splitting**: Route requests between model versions
