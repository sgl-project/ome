---
title: "Serving Runtime"
date: 2023-03-14
weight: 3
description: >
  Cluster Serving Runtime is a cluster-scoped resource that manages the runtime environment for model serving.
---
The only difference between the two is that one is namespace-scoped and the other is cluster-scoped.

A _ClusterServingRuntime_ defines the templates for Pods that can serve one or more particular model.
Each ClusterServingRuntime defines key information such as the container image of the runtime and a list of the models that the runtime supports.
Other configuration settings for the runtime can be conveyed through environment variables in the container specification.

These CRDs allow for improved flexibility and extensibility, enabling users to quickly define or customize reusable runtimes without having to modify
any controller code or any resources in the controller namespace.

The following is an example of a ClusterServingRuntime:

```yaml
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-text-generation
spec:
  supportedModelFormats:
    - name: safetensors
      modelType: transformer
      modelArchitecture: LlamaForCausalLM
      version: "1"
      autoSelect: true
      priority: 1
  protocolVersions:
    - openAI
  modelSizeRange:
    max: 128B
    min: 60B
  containers:
    - name: ome-container
      image: official-vllm-openai:0.5.3
      resources:
        requests:
          cpu: 128
          memory: 216Gi
          nvidia.com/gpu: 8
        limits:
          cpu: 128
          memory: 216Gi
          nvidia.com/gpu: 8
```

Several out-of-the-box _ClusterServingRuntimes_ are provided with OME so that users can quickly deploy common models without having to define the runtimes themselves.

| Name                                | Supported Model Types | Supported Model Architecture   | Supported Model Format |
|-------------------------------------|-----------------------|--------------------------------|------------------------|
| vllm-mistral-7b-instruct            | safetensors           | MistralForCausalLM             | Transformer            |
| vllm-e5-mistral-7b-instruct         | safetensors           | MistralModel                   | Transformer            |
| vllm-llama-3-1-70b-instruct         | safetensors           | LlamaForCausalLM               | Transformer            |
| vllm-llama-3-1-405b-instruct-fp8    | safetensors           | LlamaForCausalLM               | Transformer            |
| vllm-llama-3-2-1b                   | safetensors           | LlamaForCausalLM               | Transformer            |
| vllm-llama-3-2-3b                   | safetensors           | LlamaForCausalLM               | Transformer            |
| vllm-llama-3-2-11b                  | safetensors           | LlamaForCausalLM               | Transformer            |
| vllm-llama-3-2-90b-vision-fp8       | safetensors           | MllamaForConditionalGeneration | Transformer            |
| vllm-llama-3-3-70b-instruct         | safetensors           | LlamaForCausalLM               | Transformer            |
| vllm-deepseek-v3                    | safetensors           | DeepseekV3ForCausalLM          | Transformer            |
| vllm-deepseek-v3-rdma               | safetensors           | DeepseekV3ForCausalLM          | Transformer            |
| vllm-multi-node-llama-3-1-405b      | safetensors           | LlamaForCausalLM               | Transformer            |
| vllm-multi-node-llama-3-1-405b-rdma | safetensors           | LlamaForCausalLM               | Transformer            |
| command-r-plus                      | tensorrtllm          | CohereForCausalLM              | TensorRTLLM            |
| sglang-small                        | safetensors           | LlamaForCausalLM               | Transformer            |
| sglang-xsmall                       | safetensors           | LlamaForCausalLM               | Transformer            |

## Spec Attributes

Available attributes in the `ServingRuntime` spec:

| Attribute                                    | Description                                                                                                                                                                                                                                                                                                                                                                                                          |
|----------------------------------------------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `disabled`                                   | Disables this runtime                                                                                                                                                                                                                                                                                                                                                                                                |
| `containers`                                 | List of containers associated with the runtime                                                                                                                                                                                                                                                                                                                                                                       |
| `containers[ ].image`                        | The container image for the current container                                                                                                                                                                                                                                                                                                                                                                        |
| `containers[ ].command`                      | Executable command found in the provided image                                                                                                                                                                                                                                                                                                                                                                       |
| `containers[ ].args`                         | List of command line arguments as strings                                                                                                                                                                                                                                                                                                                                                                            |
| `containers[ ].resources`                    | Kubernetes [limits or requests](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#requests-and-limits)                                                                                                                                                                                                                                                                                  |
| `containers[ ].env `                         | List of environment variables to pass to the container                                                                                                                                                                                                                                                                                                                                                               |
| `containers[ ].imagePullPolicy`              | The container image pull policy                                                                                                                                                                                                                                                                                                                                                                                      |
| `containers[ ].workingDir`                   | The working directory for current container                                                                                                                                                                                                                                                                                                                                                                          |
| `containers[ ].livenessProbe`                | Probe for checking container liveness                                                                                                                                                                                                                                                                                                                                                                                |
| `containers[ ].readinessProbe`               | Probe for checking container readiness                                                                                                                                                                                                                                                                                                                                                                               |
| `supportedModelFormats`                      | List of model format, architecture, and type supported by the current runtime                                                                                                                                                                                                                                                                                                                                        |
| `supportedModelFormats[ ].name`              | Name of the model format                                                                                                                                                                                                                                                                                                                                                                                             |
| `supportedModelFormats[ ].modelArchitecture` | Name of the model architecture, used in validating that a model is supported by a runtime.                                                                                                                                                                                                                                                                                                                           |
| `supportedModelFormats[ ].modelType`         | Name of the mode type, such as `Transformer`, `BERT`, and `TensorRTLLM`                                                                                                                                                                                                                                                                                                                                              |
| `supportedModelFormats[ ].version`           | Version of the model format. Used in validating that a model is supported by a runtime. It is recommended to include only the major version here, for example "1" rather than "1.15.4"                                                                                                                                                                                                                               |
| `supportedModelFormats[ ].autoselect`        | Set to true to allow the ServingRuntime to be used for automatic model placement if this model is specified with no explicit runtime. The default value is false.                                                                                                                                                                                                                                                    |
| `supportedModelFormats[ ].priority`          | Priority of this serving runtime for auto selection. This is used to select the serving runtime if more than one serving runtime supports the same model format. <br/>The value should be greater than zero. The higher the value, the higher the priority. Priority is not considered if AutoSelect is either false or not specified. Priority can be overridden by specifying the runtime in the InferenceService. |
| `nodeSelector`                               | Influence Kubernetes scheduling to [assign pods to nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/)                                                                                                                                                                                                                                                                                  |
| `affinity`                                   | Influence Kubernetes scheduling to [assign pods to nodes](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity)                                                                                                                                                                                                                                                       |
| `tolerations`                                | Allow pods to be scheduled onto nodes [with matching taints](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration)                                                                                                                                                                                                                                                                           |
| `modelSizeRange`                             | Model size range is the range of model sizes supported by this runtime                                                                                                                                                                                                                                                                                                                                               |

>**Note:** `ClusterServingRuntime` support the use of template variables of the form `{{.Variable}}` inside the container spec. These should map to fields inside an
InferenceService's [metadata object](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta). The primary use of this is for passing in
InferenceService-specific information, such as a name, to the runtime environment.

>**Note:** The container name must be `ome-container` for the runtime to be recognized by the OME controller. The controller will ignore any other containers defined in the runtime. For the `MultiNodeRayVLLM` deployment mode, multiple containers are not supported, and the distinction between head and worker nodes cannot be specified.

## Using ClusterServingRuntimes

When users define predictor in their InferenceService, they can explicitly specify the name of a _ClusterServingRuntime_ or _ServingRuntime_. For example:

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-3-1-70b
  namespace: llama-3-1-70b
spec:
  predictor:
    model:
      baseModel: llama-3-1-70b
      protocolVersion: openAI
      runtime: vllm-text-generation
    minReplicas: 1
    maxReplicas: 1
```

Here, the runtime specified is `vllm-text-generation`, so the OME controller will first search the namespace for a ServingRuntime with that name. If
none exist, the controller will then search the list of ClusterServingRuntimes.

Users can also implicitly specify the runtime by setting the `autoSelect` field to `true` in the `supportedModelFormats` field of the _ClusterServingRuntime_.
```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: llama-3-1-70b
  namespace: llama-3-1-70b
spec:
  predictor:
    model:
      baseModel: llama-3-1-70b
      protocolVersion: openAI
    minReplicas: 1
    maxReplicas: 1
```

### Model Size Range

The `modelSizeRange` field defines the minimum and maximum model sizes that the runtime can support. This field is optional, but when provided, it helps the controller identify a runtime that matches the model size within the specified range. If multiple runtimes meet the size requirement, the controller will choose the runtime with the range closest to the model size.

### Priority

If more than one serving runtime supports the same model `architecture`, `type`,`format`, `protocolVersion`,
and `size range` with same `version`, then we can optionally specify `priority` for the serving runtime.
Based on the `priority` the runtime is automatically selected if no runtime is explicitly specified. Note that, `priority` is valid only if `autoSelect` is `true`. Higher value means higher priority.

For example, let's consider the serving runtimes `vllm-text-generation` and `vllm-text-generation-2`.
Both the serving runtimes support the `LlamaForCausalLM` model architecture,
`transformer` model type, `safetensors` model format,  version `1` and both supports
the `protocolVersion` openAI. Also note that `autoSelect` is enabled in both the serving runtimes.

```yaml
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-text-generation
spec:
  supportedModelFormats:
    - name: safetensors
      modelType: transformer
      modelArchitecture: LlamaForCausalLM
      version: "1"
      autoSelect: true
      priority: 1
  protocolVersions:
    - openAI
  modelSizeRange:
    max: 128B
    min: 60B
  containers:
    - name: ome-container
      image: official-vllm-openai:0.5.3
      resources:
        requests:
          cpu: 128
          memory: 216Gi
          nvidia.com/gpu: 8
        limits:
          cpu: 128
          memory: 216Gi
          nvidia.com/gpu: 8
```


```yaml
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-text-generation-2
spec:
  supportedModelFormats:
    - name: safetensors
      modelType: transformer
      modelArchitecture: LlamaForCausalLM
      version: "1"
      autoSelect: true
      priority: 1
  protocolVersions:
    - openAI
  modelSizeRange:
    max: 128B
    min: 60B
  containers:
    - name: ome-container
      image: official-vllm-openai:0.5.3
      resources:
        requests:
          cpu: 128
          memory: 216Gi
          nvidia.com/gpu: 8
        limits:
          cpu: 128
          memory: 216Gi
          nvidia.com/gpu: 8
```

**Constraints of priority**

- The higher priority value means higher precedence. The value must be greater than 0.
- The priority is valid only if auto select is enabled otherwise the priority is not considered.
- The serving runtime with priority takes precedence over the serving runtime with priority not specified.
- Two support model formats with the same name and the same version cannot have the same priority.
- If more than one serving runtime supports the model format and none of them specified the priority then, there is no guarantee _which_ runtime will be selected.
- If a serving runtime supports multiple versions of a models, then it should have the same priority.

!!! Warning
If multiple runtimes list the same format and/or version as auto-selectable and the priority is not specified, the runtime is selected based on the `creationTimestamp` i.e. the most recently created runtime is selected. So there is no guarantee _which_ runtime will be selected.
So users and cluster-administrators should enable `autoSelect` with care.
