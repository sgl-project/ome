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

### [Base Model](/docs/concepts/base_model)

The BaseModel CRD manages the lifecycle of foundational Hugging Face compatible and TensorRT LLM/AI models such as GPT,
BERT, and other architectures， model type, model format, capabilities, model sizes and model configurations.
These base models can be used for both training and serving.
This resource has both namespace-scoped and cluster-scoped which can be used to define base models for different models.

### [Fine-Tuned Weight](/docs/concepts/fine_tuned_weight)

The FineTunedWeight CRD manages the weights of models fine-tuned from a base model, allowing for task-specific optimization.


### [Dedicated AI Cluster](/docs/concepts/dedicated_ai_cluster)

The DedicatedAICluster CRD defines a dedicated cluster for AI workloads,
ensuring resource isolation and optimal performance for large-scale model serving and training.

### [Dedicated AI Cluster Profile](/docs/concepts/dedicated_ai_cluster_profile)

The DedicatedAIClusterProfile CRD defines cluster profiles that can be applied to dedicated AI clusters, specifying resource constraints and scheduling preferences.

### [Serving Runtime](/docs/concepts/serving_runtime)

The ServingRuntime CRD manages the runtime environment for model serving, allowing for dynamic scaling and configuration of model-serving containers.
This resource has both namespace-scoped and cluster-scoped which can be used to define serving runtimes for different models.


### [Inference Service](/docs/concepts/inference_service)

The InferenceService CRD manages the entire lifecycle of model-serving workloads, including model versioning, scaling, and traffic routing. 
It supports real-time inference for both single-node and multi-node deployments, ensuring seamless model updates and efficient scaling.

