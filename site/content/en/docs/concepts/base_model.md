---
title: "Base Model"
date: 2023-03-14
weight: 4
description: >
  Base Model is a resource that defines the foundation model to be used for inference.
---

A _BaseModel_ and _ClusterBaseModel_ are resources that define the foundation models used for inference in OME. The main difference between them is their scope:

- _BaseModel_ is namespace-scoped, meaning it's specific to a particular namespace
- _ClusterBaseModel_ is cluster-scoped, meaning it's available across all namespaces in the cluster

Both resources share the same specification structure but differ in their visibility and access patterns. ClusterBaseModels are typically used for organization-wide models that should be available to all teams, while BaseModels are used for team-specific or project-specific models.

## Model Specification

The following example shows a BaseModel configuration:

```yaml
apiVersion: ome.io/v1beta1
kind: BaseModel
metadata:
  name: llama-2-70b
spec:
  modelFormat:
    name: safetensors
    version: "1"
  modelType: transformer
  modelArchitecture: LlamaForCausalLM
  modelParameterSize: "70B"
  modelCapabilities:
    - TEXT_GENERATION
  storage:
    path: "oci://my-namespace/my-bucket/llama-2-70b"
  servingMode:
    - On-demand
  maxTokens: 4096
  isLongTermSupported: true
```

## Spec Attributes

Available attributes in the BaseModel/ClusterBaseModel spec:

| Attribute             | Description                                                               |
|-----------------------|---------------------------------------------------------------------------|
| `modelFormat`         | Defines the format of the model (e.g., safetensors, ONNX) and its version |
| `modelType`           | The type of model architecture (e.g., transformer)                        |
| `modelArchitecture`   | Specific model architecture (e.g., LlamaForCausalLM, GemmaForCausalLM)    |
| `modelParameterSize`  | Size of the model parameters (e.g., "70B")                                |
| `modelCapabilities`   | List of model capabilities (e.g., TEXT_GENERATION, TEXT_EMBEDDINGS)       |
| `modelConfiguration`  | Optional JSON configuration specific to the model                         |
| `storage`             | Storage configuration including path and credentials                      |
| `servingMode`         | Model serving modes (On-demand or Dedicated)                              |
| `maxTokens`           | Maximum number of tokens the model can process                            |
| `isLongTermSupported` | Indicates if the model is long-term supported                             |
| `deprecationTime`     | Optional timestamp when the model was deprecated                          |

## Supported Models

OME supports a wide range of models optimized for different tasks:

### Open SourceLarge Language Models

**Meta's Llama Family**:
- Llama 3.1 (70B, 405B) Instruct models
- Llama 3.2 Vision models (11B, 90B)
- Llama 3.2 Instruct models (1B, 3B)
- Llama 3.3 70B Instruct model

**Microsoft Models**:
- Phi-3 Vision 128k Instruct (4.15B) - Multimodal capabilities with extended context

**Mistral Models**:
- E5 Mistral 7B Instruct - Optimized for text embeddings

### Cohere Models

**Embedding Models**:
- Embed English (v2.0, v3.0)
- Embed English Light (v2.0, v3.0)
- Embed Multilingual (v3.0)
- Embed Multilingual Light (v3.0)

**Command Models**:
- Command-R (16k and 128k context variants)
- Command-R Plus (various versions)
- Command-R with different batch sizes and tensor parallel configurations

Models are categorized by:
- Size (e.g., SMALL, MEDIUM, LARGE)
- Experimental status
- Internal/External availability
- Lifecycle phase (e.g., ACTIVE)

## Storage Configuration

The storage configuration supports both local paths and OCI object storage paths:

```yaml
storage:
  path: "/mnt/models/mistral-7b"  # Local path where model will be stored
  storageUri: "oci://n/idlsnvn0f2is/b/model-store/o/olm/action-mistral-7b-v0.2-finetuned-model-v0.0"  # OCI object storage path
  parameters:
    region: "us-phoenix-1"
  storageKey: "my-storage-key"
```

## Model Capabilities

OME supports various model capabilities:

- `TEXT_GENERATION`: For text generation models
- `TEXT_SUMMARIZATION`: For text summarization models
- `TEXT_EMBEDDINGS`: For text embedding models

## Serving Modes

Models can be served in two modes:

- `On-demand`: The model is loaded only when needed
- `Dedicated`: The model remains loaded and dedicated resources are allocated

## Model Lifecycle

The model goes through various lifecycle states:

- `Creating`: Initial state when the model is being created
- `Importing`: Model is being imported from storage
- `In_Transit`: Model is being transferred
- `Ready`: Model is ready for serving
- `Failed`: Model failed to load or encountered an error

## Status

The status field provides information about the current state of the model:

```yaml
status:
  state: Ready
  nodesReady:
    - node1
    - node2
  nodesFailed: []
```

This shows which nodes have successfully loaded the model and which ones have failed.
