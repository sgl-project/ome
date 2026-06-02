# flash-model-serving

A Helm chart for deploying runtimes to support Project Flash models.

This chart is deployed by the `flash-model-serving` Shepherd flock, separate from the `multi-model-serving-devops` flock that handles self-hosted models.

## Chart structure

- `templates/runtimes/<vendor>/` contains the `ClusterServingRuntime` definitions for Flash serving. Each runtime owns the actual serving contract, including engine/router/decoder configuration, accelerator requirements, `supportedModelFormats`, `modelSizeRange`, and protocol support.
- The `values.yaml` file contains source of truth values for the chart, including OCI configurations, runtime common configurations, and Flash model container repo and tags. NO Shepherd values overwrites needed for container repo and tags.
- `templates/validated_hf_models_configmap.yaml` publishes a curated Flash catalog in the `ome` namespace. The control plane reads `validated_hf_models.yaml` to find exact external Hugging Face model IDs and the `ClusterServingRuntime` resources that can serve them.
- The ConfigMap is not used for runtime selection or request routing, and it does not duplicate accelerator classes. Runtime eligibility and accelerator class ground truth remain determined from the rendered `ClusterServingRuntime` resources and model metadata.
