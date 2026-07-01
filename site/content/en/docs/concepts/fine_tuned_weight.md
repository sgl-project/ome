---
title: "Fine Tuned Weight"
linkTitle: "Fine Tuned Weight"
weight: 10
description: >
  Fine Tuned Weight represents weights fine-tuned from a base model - such as LoRA adapters - that can be served on top of an existing BaseModel.
---

## What is a Fine Tuned Weight?

A FineTunedWeight in OME is a Kubernetes resource that represents a set of weights that were fine-tuned from an existing [Base Model](/ome/docs/concepts/base_model). Rather than describing a full, standalone model, it points back to a base model and carries only the artifacts and metadata specific to the fine-tuning - for example a LoRA adapter, an added adapter module, or a distilled variant.

Keeping fine-tuned weights as their own resource lets you manage many customizations of the same base model independently: each FineTunedWeight has its own storage location, its own fine-tuning metadata, and its own lifecycle, while sharing the underlying base model. When an [InferenceService](/ome/docs/concepts/inference_service) references one or more fine-tuned weights, OME serves them on top of the referenced base model.

FineTunedWeight is a **cluster-scoped** resource, so a fine-tuned weight is available to workloads in any namespace across the cluster.

## Basic Example

Here is a simple FineTunedWeight that references a base model and stores a LoRA adapter:

```yaml
apiVersion: ome.io/v1beta1
kind: FineTunedWeight
metadata:
  name: llama-70b-finance-lora
spec:
  # Reference to the base model this weight was fine-tuned from
  baseModelRef:
    name: llama-3-70b-instruct
    namespace: default

  # Fine-tuning method
  modelType: LoRA

  # Hyperparameters used for fine-tuning (generic JSON)
  hyperParameters:
    lora_rank: 16
    lora_alpha: 32
    learning_rate: 1e-4

  # Storage for the fine-tuned weights (same StorageSpec as base models)
  storage:
    storageUri: oci://n/mycompany/b/fine-tuned/o/llama-70b-finance-lora/
    path: /raid/fine-tuned/llama-70b-finance-lora
```

## Specification Reference

Available attributes in the FineTunedWeight spec:

| Attribute         | Type            | Required | Description                                                                                     |
|-------------------|-----------------|----------|-------------------------------------------------------------------------------------------------|
| `baseModelRef`    | ObjectReference | Yes      | Reference to the base model that this weight is fine-tuned from                                  |
| `baseModelRef.name`      | string   | Yes      | Name of the referenced base model                                                               |
| `baseModelRef.namespace` | string   | No       | Namespace of the referenced base model                                                          |
| `modelType`       | string          | Yes      | Fine-tuning method, e.g., `LoRA`, `Adapter`, `Distillation`, `Tfew`                             |
| `hyperParameters` | RawExtension    | Yes      | Hyperparameters used for fine-tuning, stored as generic JSON                                     |
| `configuration`   | RawExtension    | No       | Additional configuration for the fine-tuned weight, stored as generic JSON                       |
| `storage`         | StorageSpec     | Yes      | Storage configuration for the fine-tuned weights (see [Storage](#storage))                       |
| `trainingJobRef`  | ObjectReference | No       | Reference to the training job that produced this weight                                          |
| `displayName`     | string          | No       | User-friendly name of the fine-tuned weight                                                      |
| `version`         | string          | No       | Version of the fine-tuned weight                                                                 |
| `disabled`        | boolean         | No       | Whether the fine-tuned weight is disabled                                                        |
| `vendor`          | string          | No       | Vendor of the fine-tuned weight                                                                  |

For the complete, generated field-level reference, see the [OME v1beta1 API reference](/ome/docs/reference/ome.v1beta1).

### Model Type

The `modelType` field records the fine-tuning method used to produce the weights. Common values are:

| Type           | Description                                                        |
|----------------|--------------------------------------------------------------------|
| `LoRA`         | Low-Rank Adaptation adapter applied on top of the base model       |
| `Adapter`      | An adapter module added to the base model                          |
| `Distillation` | Weights produced through knowledge distillation                    |
| `Tfew`         | T-Few style parameter-efficient fine-tuning                        |

### Storage

FineTunedWeight uses the same `StorageSpec` as [BaseModel](/ome/docs/concepts/base_model#storage-backends), so the fine-tuned artifacts can live in any of the supported storage backends (OCI Object Storage, Hugging Face Hub, PVC, or vendor storage) and use the same node-selection and authentication options. Point `storage.storageUri` at the location of the fine-tuned weights and `storage.path` at where they should be downloaded on the node:

```yaml
spec:
  storage:
    storageUri: oci://n/mycompany/b/fine-tuned/o/llama-70b-finance-lora/
    path: /raid/fine-tuned/llama-70b-finance-lora
    storageKey: oci-model-credentials
    parameters:
      region: us-phoenix-1
      auth_type: InstancePrincipal
```

### Training Job Reference

If the weights were produced by a training job, you can record a reference to it with `trainingJobRef`. This is optional and is used to trace a fine-tuned weight back to the job that produced it:

```yaml
spec:
  trainingJobRef:
    name: llama-finance-training-job
    namespace: training
```

## Complete Configuration Example

Here is a fuller FineTunedWeight showing the available options:

```yaml
apiVersion: ome.io/v1beta1
kind: FineTunedWeight
metadata:
  name: llama-70b-finance-lora
spec:
  # Reference to the base model
  baseModelRef:
    name: llama-3-70b-instruct
    namespace: default

  # Fine-tuning metadata
  modelType: LoRA
  displayName: "Llama 3 70B Finance LoRA"
  version: "1.0"
  vendor: "acme-ml"

  # Hyperparameters used for fine-tuning
  hyperParameters:
    lora_rank: 16
    lora_alpha: 32
    learning_rate: 1e-4

  # Additional configuration
  configuration:
    target_modules:
      - q_proj
      - v_proj

  # Storage for fine-tuned weights
  storage:
    storageUri: oci://n/mycompany/b/fine-tuned/o/llama-70b-finance-lora/
    path: /raid/fine-tuned/llama-70b-finance-lora
    storageKey: oci-model-credentials
    parameters:
      region: us-phoenix-1
      auth_type: InstancePrincipal

  # Training job that produced this weight
  trainingJobRef:
    name: llama-finance-training-job
    namespace: training
```

## Using Fine Tuned Weights in an InferenceService

Fine-tuned weights are consumed by an [InferenceService](/ome/docs/concepts/inference_service) through the model reference. The `spec.model` field of an InferenceService (a `ModelRef`) has an optional `fineTunedWeights` field - a list of FineTunedWeight names to apply on top of the referenced base model.

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-finance-chat
spec:
  model:
    name: llama-3-70b-instruct
    fineTunedWeights:
      - llama-70b-finance-lora
  engine:
    minReplicas: 1
    maxReplicas: 3
```

Here the InferenceService serves the `llama-3-70b-instruct` base model with the `llama-70b-finance-lora` fine-tuned weight applied. OME resolves each referenced FineTunedWeight, ensures its artifacts are available, and configures the serving runtime accordingly.

## Model Status and Lifecycle

Like BaseModel, a FineTunedWeight tracks its readiness across the nodes in the cluster. The status contains:

| Field         | Type     | Description                                              |
|---------------|----------|----------------------------------------------------------|
| `state`       | string   | Overall state (e.g., `Creating`, `Ready`, `Failed`)      |
| `lifecycle`   | string   | Lifecycle stage of the fine-tuned weight                 |
| `nodesReady`  | []string | Nodes where the fine-tuned weight is ready               |
| `nodesFailed` | []string | Nodes where the fine-tuned weight failed                 |

## Next Steps

- [Base Model](/ome/docs/concepts/base_model) - the foundation models that fine-tuned weights build on
- [Inference Service](/ome/docs/concepts/inference_service) - how to serve a model, with or without fine-tuned weights
- [OME v1beta1 API reference](/ome/docs/reference/ome.v1beta1) - the full field-level API reference
