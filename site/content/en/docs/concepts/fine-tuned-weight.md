---
title: "FineTunedWeight"
linkTitle: "FineTunedWeight"
weight: 30
description: >
  Understanding fine-tuned model weights in OME
---

FineTunedWeight represents fine-tuned model weights that are derived from base models through training processes. These weights can be applied to base models to create specialized versions for specific tasks or domains.

## Overview

Fine-tuned weights enable you to:

- **Customize Models**: Adapt base models for specific use cases or domains
- **Preserve Base Models**: Keep original models intact while creating specialized versions
- **Efficient Storage**: Store only the difference (delta) weights rather than complete models
- **Version Management**: Track different fine-tuning iterations and experiments
- **Composition**: Apply multiple fine-tuned weights to create combined capabilities

## FineTunedWeight Specification

### Basic Structure

```yaml
apiVersion: ome.io/v1beta1
kind: FineTunedWeight
metadata:
  name: llama-legal-adapter
spec:
  baseModelRef:
    name: llama-3-8b
  modelType: LoRA
  hyperParameters:
    rank: 16
    alpha: 32
    dropout: 0.1
    target_modules: ["q_proj", "v_proj"]
  storage:
    storageUri: oci://myns/my-bucket/legal-adapter-weights/
  trainingJobRef:
    name: legal-fine-tuning-job
    namespace: training
```

### Key Fields

#### Model Reference
```yaml
spec:
  baseModelRef:
    name: "base-model-name"        # Required: Name of the base model
    namespace: "base-namespace"    # Optional: Namespace for namespaced base models
```

#### Model Type
Specifies the fine-tuning methodology:
```yaml
spec:
  modelType: "LoRA"  # LoRA, Adapter, Distillation, QLoRA, etc.
```

#### Hyperparameters
Training-specific parameters stored as flexible JSON:
```yaml
spec:
  hyperParameters:
    rank: 16
    alpha: 32
    dropout: 0.1
    target_modules: ["q_proj", "v_proj", "k_proj", "o_proj"]
    learning_rate: 0.0001
    batch_size: 4
```

#### Storage Configuration
```yaml
spec:
  storage:
    storageUri: "oci://namespace/bucket/path/"
    parameters:
      region: "us-phoenix-1"
      authentication: "instance_principal"
    nodeSelector:
      node-type: "gpu"
```

#### Training Job Reference
Links to the training job that produced these weights:
```yaml
spec:
  trainingJobRef:
    name: "training-job-name"
    namespace: "training-namespace"
```

## Fine-Tuning Types

### LoRA (Low-Rank Adaptation)
Parameter-efficient fine-tuning that adds trainable low-rank matrices:

```yaml
apiVersion: ome.io/v1beta1
kind: FineTunedWeight
metadata:
  name: lora-chatbot-weights
spec:
  baseModelRef:
    name: llama-3-8b
  modelType: LoRA
  hyperParameters:
    rank: 32
    alpha: 64
    dropout: 0.05
    target_modules: ["q_proj", "v_proj", "k_proj", "o_proj"]
  storage:
    storageUri: oci://myns/models/lora-chatbot/
```

### QLoRA (Quantized LoRA)
Combines quantization with LoRA for memory-efficient training:

```yaml
apiVersion: ome.io/v1beta1
kind: FineTunedWeight
metadata:
  name: qlora-summary-weights
spec:
  baseModelRef:
    name: mistral-7b
  modelType: QLoRA
  hyperParameters:
    rank: 16
    alpha: 32
    quantization: "4bit"
    compute_dtype: "bfloat16"
  storage:
    storageUri: oci://myns/models/qlora-summary/
```

### Adapter Layers
Full adapter layers for more comprehensive fine-tuning:

```yaml
apiVersion: ome.io/v1beta1
kind: FineTunedWeight
metadata:
  name: adapter-translation-weights
spec:
  baseModelRef:
    name: t5-large
  modelType: Adapter
  hyperParameters:
    hidden_size: 768
    adapter_size: 64
    activation: "relu"
  storage:
    storageUri: oci://myns/models/translation-adapter/
```

## Status and Lifecycle

FineTunedWeight resources track their availability across nodes:

```yaml
status:
  state: Ready                    # Creating, Importing, In_Transit, Ready, Failed
  lifecycle: Public               # Deprecated, Experiment, Public, Internal
  nodesReady:                    # Nodes where weights are available
    - gpu-node-1
    - gpu-node-2
  nodesFailed: []                # Nodes where download failed
```

### Lifecycle States

- **Creating**: Weight resource is being initialized
- **Importing**: Weights are being downloaded from storage
- **In_Transit**: Weights are being distributed to nodes
- **Ready**: Weights are available for use
- **Failed**: Weight download or validation failed

## Using Fine-Tuned Weights

### In InferenceService

Apply fine-tuned weights to serving deployments:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: specialized-chatbot
spec:
  predictor:
    model:
      name: llama-3-8b
      fineTunedWeights:
        - lora-chatbot-weights
        - domain-specific-adapter
  runtime:
    name: vllm-runtime
```

### Multiple Fine-Tuned Weights

Combine multiple fine-tuned weights for enhanced capabilities:

```yaml
spec:
  predictor:
    model:
      name: base-model
      fineTunedWeights:
        - conversational-lora      # Improves dialogue quality
        - domain-knowledge-adapter # Adds domain expertise
        - safety-filter-weights    # Enhances safety
```

## Storage and Distribution

### OCI Object Storage
```yaml
spec:
  storage:
    storageUri: oci://myns/my-bucket/fine-tuned-weights/
    parameters:
      region: us-phoenix-1
      authentication: instance_principal
```

### Node Affinity
Control where weights are cached:

```yaml
spec:
  storage:
    nodeSelector:
      node-type: gpu
      zone: us-phoenix-1a
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          - key: nvidia.com/gpu.memory
            operator: Gt
            values: ["16Gi"]
```

## Best Practices

### Naming Convention
Use descriptive names that indicate the purpose and base model:
```yaml
metadata:
  name: llama-3-8b-legal-lora-v1
  labels:
    ome.io/base-model: llama-3-8b
    ome.io/domain: legal
    ome.io/method: lora
    ome.io/version: v1
```

### Version Management
Track different iterations:
```yaml
metadata:
  name: chatbot-lora-v2
  labels:
    ome.io/version: v2
    ome.io/previous-version: chatbot-lora-v1
spec:
  version: "2.0"
```

### Resource Organization
Group related weights:
```yaml
metadata:
  name: domain-expert-weights
  labels:
    ome.io/domain: medical
    ome.io/task: question-answering
    ome.io/experiment: exp-2024-01
```

## Monitoring and Observability

### Performance Metrics
Monitor fine-tuned weight performance:
```yaml
metadata:
  annotations:
    ome.io/metrics-enabled: "true"
    ome.io/performance-baseline: base-model-metrics
```

### Usage Tracking
Track which services use specific weights:
```yaml
metadata:
  labels:
    ome.io/used-by: chatbot-service,qa-service
```

## Troubleshooting

### Common Issues

**Weight Loading Failures**
```bash
kubectl describe fineTunedWeight my-weights
# Check events for download or validation errors
```

**Node Distribution Issues**
```bash
kubectl get fineTunedWeight my-weights -o yaml
# Check nodesReady and nodesFailed in status
```

**Compatibility Problems**
```bash
# Verify base model compatibility
kubectl get baseModel base-model-name -o yaml
# Check model architecture and format matching
```

### Debugging Commands

```bash
# List all fine-tuned weights
kubectl get finetunedweights

# Get detailed status
kubectl describe finetunedweight weight-name

# Check weight distribution
kubectl get finetunedweight weight-name -o jsonpath='{.status.nodesReady}'

# View training job reference
kubectl get finetunedweight weight-name -o jsonpath='{.spec.trainingJobRef}'
```

## Security Considerations

### Access Control
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: finetunedweight-user
rules:
- apiGroups: ["ome.io"]
  resources: ["finetunedweights"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
```

### Storage Security
- Use encrypted object storage
- Implement proper IAM policies
- Regular security audits of weight artifacts 