---
title: "Concepts"
linkTitle: "Concepts"
weight: 4
description: >
  Core Kueue Concepts
no_list: true
---

This section of the documentation helps you learn about the components, APIs and
abstractions that OME uses to represent model serving, model training, models, and dedicated AI clusters.

## APIs

### [BaseModel](/docs/concepts/base_model)

The BaseModel CRD manages the lifecycle of foundational AI models such as GPT, BERT, and other architectures. These base models can be used for both training and serving.

### [ClusterBaseModel](/docs/concepts/cluster_base_model)

The ClusterBaseModel CRD is similar to BaseModel, but operates at the cluster level, allowing for global model management across namespaces.

### [FineTunedWeight](/docs/concepts/fine_tuned_weight)

The FineTunedWeight CRD manages the weights of models fine-tuned from a base model, allowing for task-specific optimization.


### [DedicatedAICluster](/docs/concepts/dedicated_ai_cluster)

The DedicatedAICluster CRD defines a dedicated cluster for AI workloads,
ensuring resource isolation and optimal performance for large-scale model serving and training.

### [DedicatedAIClusterProfile](/docs/concepts/dedicated_ai_cluster_profile)

The DedicatedAIClusterProfile CRD defines cluster profiles that can be applied to dedicated AI clusters, specifying resource constraints and scheduling preferences.

### [ServingRuntime](/docs/concepts/serving_runtime)

The ServingRuntime CRD manages the runtime environment for model serving, allowing for dynamic scaling and configuration of model-serving containers.
This resource is namespace-scoped and can be used to define serving runtimes for different models.

### [ClusterServingRuntime](/docs/concepts/cluster_serving_runtime)

The ClusterServingRuntime CRD is similar to ServingRuntime, but operates at the cluster level, allowing for global serving runtime management across namespaces.

### [InferenceService](/docs/concepts/inference_service)

The InferenceService CRD manages the entire lifecycle of model-serving workloads, including model versioning, scaling, and traffic routing. 
It supports real-time inference for both single-node and multi-node deployments, ensuring seamless model updates and efficient scaling.

