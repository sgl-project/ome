---
title: "Base Model"
date: 2023-03-14
weight: 4
description: >
  Base Model defines foundation models that can be automatically downloaded, parsed, and served across your cluster.
---

## What is a Base Model?

A Base Model in OME is a Kubernetes resource that represents a foundation AI model (like GPT, Llama, or Mistral) that you want to use for inference workloads. Think of it as a blueprint that tells OME where to find your model, how to download it, and where to store it on your cluster nodes.

When you create a BaseModel resource, OME automatically handles the complex process of downloading the model files, parsing the model's configuration to understand its capabilities, and making it available across your cluster nodes where AI workloads can use it.

## BaseModel vs ClusterBaseModel

OME provides two types of model resources:

**BaseModel** is namespace-scoped, meaning it exists within a specific Kubernetes namespace. If you create a BaseModel in the "team-a" namespace, only workloads in that namespace can use it. This is perfect for team-specific models or when you want to isolate model access.

**ClusterBaseModel** is cluster-scoped, meaning it's available to workloads in any namespace across your entire cluster. This is ideal for organization-wide models that multiple teams need to access, like a shared Llama-3 model that everyone uses.

Both types use exactly the same specification format - the only difference is their visibility scope.

## How the Model Agent Works

OME deploys a component called the **Model Agent** as a DaemonSet, which means it runs on every single node in your Kubernetes cluster. This agent is the workhorse that handles all model operations.

Here's what happens when you create a BaseModel:

1. **Discovery**: The Model Agent on each node watches for new BaseModel and ClusterBaseModel resources using Kubernetes informers. When you create a new model resource, every agent immediately knows about it.

2. **Node Selection**: The agent checks if the current node should download this model based on node selectors and affinity rules you've configured. Not every node needs every model - you might want large models only on GPU nodes, for example.

3. **Download Process**: If the node is selected, the agent starts downloading the model from your specified storage location. This happens in parallel across all selected nodes.

4. **Parsing and Validation**: Once downloaded, the agent looks for a `config.json` file in the model directory and automatically parses it to understand the model's architecture, capabilities, and requirements.

5. **Status Updates**: The agent updates the model's status to reflect whether the download succeeded or failed, and labels the node to indicate model availability.

6. **Monitoring**: The agent continuously monitors the model's health and can re-download if files become corrupted.

## Storage: Where Your Models Live

OME supports multiple storage backends because different organizations have different infrastructure setups. Let's explore each option in detail.

### Oracle Cloud Infrastructure (OCI) Object Storage

OCI Object Storage is Oracle's cloud storage service, similar to Amazon S3. If your models are stored in OCI, you'll use URIs that follow this specific format:

```
oci://n/{namespace}/b/{bucket}/o/{object_path}
```

Let's break this down:
- `oci://` - This tells OME you're using OCI Object Storage
- `n/{namespace}` - The OCI tenancy namespace (not to be confused with Kubernetes namespaces)
- `b/{bucket}` - The storage bucket name where your model files live
- `o/{object_path}` - The path within the bucket to your model files

Here's a real example:

```yaml
storage:
  storageUri: "oci://n/mycompany/b/ai-models/o/llama/llama-3-70b-instruct/"
  path: "/raid/models/llama-3-70b-instruct"
  parameters:
    region: "us-phoenix-1"
  storageKey: "oci-credentials"
```

The `path` field specifies where on each node's local filesystem the model should be stored. The `parameters` section lets you specify OCI-specific settings like the region. The `storageKey` references a Kubernetes Secret containing your OCI credentials.

### Hugging Face Hub

Hugging Face Hub is the most popular repository for open-source AI models. If you want to use models directly from Hugging Face, OME can download them automatically.

The URI format is:
```
hf://{model-id}[@{branch}]
```

Examples:
- `hf://meta-llama/Llama-3.3-70B-Instruct` - Downloads from the main branch
- `hf://microsoft/Phi-3-vision-128k-instruct@v1.0` - Downloads from a specific branch/tag

```yaml
storage:
  storageUri: "hf://meta-llama/Llama-3.3-70B-Instruct"
  path: "/models/llama-3.3-70b"
  storageKey: "huggingface-token"
```

The Model Agent uses the Hugging Face Hub API to download all model files, including weights, tokenizer files, and configuration. If the model is private or gated, you'll need to provide a Hugging Face access token in the `storageKey` secret.

### Persistent Volume Claims (PVC)

If your models are already stored in Kubernetes persistent volumes, you can reference them directly:

```
pvc://{pvc-name}/{sub-path}
```

```yaml
storage:
  storageUri: "pvc://model-storage/llama-models/llama-3-70b"
  path: "/local/models/llama-3-70b"
```

This tells OME to copy the model from the specified path within the PVC to the local path on each node. This is useful when you have a shared storage system like NFS or when you've pre-loaded models into persistent volumes.

### Vendor Storage

For proprietary or vendor-specific storage systems:

```
vendor://{vendor-name}/{resource-type}/{resource-path}
```

```yaml
storage:
  storageUri: "vendor://nvidia/models/llama-70b-tensorrt"
  path: "/opt/models/llama-70b-tensorrt"
```

This is an extensible format that allows integration with vendor-specific model repositories.

## Authentication: Securing Access to Your Models

Different storage backends require different authentication methods. OME supports multiple authentication strategies to work with your existing security infrastructure.

### OCI Authentication

For OCI Object Storage, OME supports four authentication methods:

**Instance Principal** is the simplest method when running on OCI compute instances. The compute instance itself has an identity that can access OCI services without storing credentials. You just specify:

```yaml
storage:
  storageUri: "oci://n/mycompany/b/models/o/llama-70b/"
  parameters:
    auth_type: "InstancePrincipal"
```

**User Principal** uses specific user credentials stored in a Kubernetes Secret:

```yaml
storage:
  storageUri: "oci://n/mycompany/b/models/o/llama-70b/"
  storageKey: "oci-user-credentials"
  parameters:
    auth_type: "UserPrincipal"
```

The secret would contain your OCI user's API key, tenancy OCID, user OCID, and private key.

**Resource Principal** is used when running in OCI Container Engine for Kubernetes (OKE) with resource principals enabled.

**OKE Workload Identity** is the newest method that uses Kubernetes service accounts mapped to OCI identities.

### Hugging Face Authentication

For Hugging Face models, especially private or gated models, you need an access token:

```yaml
storage:
  storageUri: "hf://meta-llama/Llama-3.3-70B-Instruct"
  storageKey: "hf-token"
```

The secret should contain a key named `token` with your Hugging Face access token as the value.

## Node Affinity: Controlling Where Models Go

Not every model needs to be on every node. Large language models can be hundreds of gigabytes, and you might have different types of nodes in your cluster. Node affinity lets you control precisely which nodes should download and store each model.

### Simple Node Selection with nodeSelector

The simplest way to control model placement is with `nodeSelector`, which requires nodes to have specific labels:

```yaml
storage:
  storageUri: "oci://n/mycompany/b/models/o/llama-70b/"
  nodeSelector:
    node.kubernetes.io/instance-type: "GPU.A100.4"
    models.ome.io/storage-tier: "fast-ssd"
```

This means the model will only be downloaded to nodes that have both labels: the instance type must be "GPU.A100.4" AND the storage tier must be "fast-ssd". If a node is missing either label, it won't get the model.

### Advanced Node Affinity

For more complex scenarios, use `nodeAffinity` with match expressions:

```yaml
storage:
  storageUri: "oci://n/mycompany/b/models/o/large-model/"
  nodeAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      nodeSelectorTerms:
      - matchExpressions:
        - key: "node.kubernetes.io/instance-type"
          operator: In
          values: ["GPU.A100.4", "GPU.H100.8"]
        - key: "models.ome.io/available-storage"
          operator: Gt
          values: ["500Gi"]
```

This configuration means: "Download this model to nodes that have either GPU.A100.4 OR GPU.H100.8 instance types, AND have more than 500Gi of available storage."

The supported operators are:
- **In**: The node's label value must be one of the listed values
- **NotIn**: The node's label value must NOT be one of the listed values
- **Exists**: The node must have this label key (value doesn't matter)
- **DoesNotExist**: The node must NOT have this label key
- **Gt**: For numeric values, the node's value must be greater than the specified value
- **Lt**: For numeric values, the node's value must be less than the specified value

### Real-World Node Affinity Examples

**GPU-Specific Models**: Ensure large language models only go to nodes with appropriate GPUs:
```yaml
nodeSelector:
  accelerator: "nvidia-a100"
  gpu-memory: "80gb"
```

**Storage Requirements**: Target nodes with sufficient fast storage:
```yaml
nodeAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
    nodeSelectorTerms:
    - matchExpressions:
      - key: "storage.ome.io/nvme-capacity"
        operator: Gt
        values: ["1000Gi"]
```

**Geographic Distribution**: Control model placement across regions:
```yaml
nodeSelector:
  topology.kubernetes.io/region: "us-west-2"
  topology.kubernetes.io/zone: "us-west-2a"
```

## Automatic Model Discovery and Parsing

One of OME's most powerful features is its ability to automatically understand your models. When the Model Agent downloads a model, it doesn't just copy files - it intelligently parses the model's configuration to extract important metadata.

### How Model Parsing Works

After downloading model files, the Model Agent looks for a `config.json` file in the model directory. This file, standard in most modern AI models, contains crucial information about the model's architecture, capabilities, and requirements.

The agent uses specialized parsers for different model architectures. Currently supported model types include:

**Llama Family Models** (including Llama 3, 3.1, 3.2, and 4): The agent recognizes various Llama configurations, from the 1B parameter Llama 3.2 models up to the massive 405B parameter Llama 3.1 models. It automatically detects whether it's a base model or an instruct-tuned variant.

**DeepSeek Models**: Including the latest DeepSeek V3 with its mixture-of-experts architecture. The agent understands the complex MoE configuration and correctly calculates the effective parameter count.

**Mistral and Mixtral**: Both the standard Mistral models and the mixture-of-experts Mixtral models are supported, with automatic detection of the expert configuration.

**Microsoft Phi Models**: Including both text-only Phi models and the multimodal Phi-3 Vision models that can process both text and images.

**Qwen Models**: The Qwen2 family with various parameter sizes and context lengths.

**Multimodal Models**: Models that can process both text and images, like MLlama (Llama Vision) models.

### What Information Gets Extracted

From the `config.json` file and model structure analysis, the Model Agent automatically determines:

**Model Type**: The fundamental architecture family (e.g., "llama", "mistral", "deepseek_v3"). This helps OME understand how to work with the model.

**Model Architecture**: The specific implementation class (e.g., "LlamaForCausalLM", "MistralForCausalLM"). This tells OME exactly which code path to use when loading the model.

**Parameter Count**: The agent tries to get an accurate count by parsing SafeTensors files, which contain the actual model weights. If that fails, it estimates based on the architecture configuration. This is crucial for resource planning.

**Context Length**: The maximum number of tokens the model can process in a single request. This varies widely - some models handle 4K tokens, others can handle 128K or more. The agent also detects RoPE scaling configurations for extended context.

**Framework Information**: Which AI framework the model uses (usually "transformers") and what version. This ensures compatibility.

**Data Type**: Whether the model uses float32, float16, bfloat16, int8, int4, or other numeric formats. This affects memory usage and performance calculations.

**Quantization**: If the model has been quantized (compressed) to use less memory, the agent detects the quantization method (fp8, int4, etc.).


### SafeTensors Integration for Accurate Analysis

OME includes sophisticated SafeTensors parsing capabilities that provide highly accurate model analysis:

**Precise Parameter Counting:**
Instead of relying on potentially incorrect configuration files, the Model Agent can parse SafeTensors files directly to count parameters precisely. This is especially important for:

### Hugging Face Integration Features

The Model Agent provides deep integration with Hugging Face Hub, going beyond simple file downloads:

**Repository Analysis:**
- **Branch detection**: Automatically detects and uses the correct branch (main, fp16, gguf, etc.)
- **File filtering**: Downloads only necessary files based on model format requirements
- **Revision handling**: Supports specific commits, tags, or branch heads
- **LFS support**: Seamlessly handles Git LFS files without user intervention

**Progress Monitoring:**
For Hugging Face downloads, the agent provides detailed progress tracking:

```
Downloading model files: [████████████████████████████████████████] 100%
├── config.json: 2.1 KB [✓]
├── tokenizer.json: 17.2 MB [✓]
├── pytorch_model-00001-of-00008.bin: 9.9 GB [✓]
├── pytorch_model-00002-of-00008.bin: 9.9 GB [✓]
└── ... (continuing for all model shards)
Total: 76.3 GB downloaded
```

**Authentication and Rate Limiting:**
- Respects Hugging Face API rate limits
- Supports authentication tokens for private repositories
- Implements exponential backoff for transient failures
- Handles quota exhaustion gracefully with informative error messages

**Model Card Integration:**
The agent can optionally download and parse model cards (README.md files) to extract additional metadata like:
- Model description and intended use cases
- Training data information
- Performance benchmarks
- License information

### Disabling Automatic Parsing

Sometimes you might want to specify model information manually, perhaps because you have a custom model format or want to override the detected values. You can disable automatic parsing with an annotation:

```yaml
apiVersion: ome.io/v1beta1
kind: BaseModel
metadata:
  name: custom-model
  annotations:
    ome.io/skip-config-parsing: "true"
spec:
  modelType: "custom"
  modelArchitecture: "CustomForCausalLM"
  modelParameterSize: "70B"
  maxTokens: 4096
  modelCapabilities:
    - TEXT_GENERATION
```

## Model Status and Lifecycle Management

OME provides comprehensive tracking of model status across your cluster. Understanding this system helps you monitor model availability and troubleshoot issues.

### Model States

Each model on each node goes through a lifecycle with these states:

**Ready**: The model has been successfully downloaded, validated, and is available for use. Workloads can now use this model on this node.

**Updating**: The model is currently being downloaded or updated. This might take several minutes or hours depending on model size and network speed.

**Failed**: Something went wrong during download, validation, or parsing. Check the Model Agent logs for details about what failed.

**Deleted**: The model was removed from this node, either because the BaseModel resource was deleted or because node affinity rules changed.

### Node Status Tracking with ConfigMaps

OME stores detailed model status information in Kubernetes ConfigMaps, one per node. These ConfigMaps are created in the same namespace where the Model Agent runs (typically "ome").

Here's what a status ConfigMap looks like:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: worker-node-1
  namespace: ome
  labels:
    models.ome/basemodel-status: "true"
data:
  "default_llama-70b": |
    {
      "name": "llama-70b",
      "status": "Ready",
      "config": {
        "modelType": "llama",
        "modelArchitecture": "LlamaForCausalLM",
        "modelParameterSize": "70B",
        "maxTokens": 4096,
        "modelCapabilities": ["TEXT_GENERATION", "CHAT"]
      }
    }
  "team-a_custom-model": |
    {
      "name": "custom-model",
      "status": "Failed",
      "config": null
    }
```

The ConfigMap key format is `{namespace}_{model-name}` for BaseModels, or just `{model-name}` for ClusterBaseModels. This allows you to see exactly which models are available on each node and their current status.

### Node Labels for Quick Discovery

In addition to ConfigMaps, OME automatically labels nodes to indicate model availability. Each model gets a unique label based on its UID:

```bash
kubectl get nodes -l "models.ome/12345678-1234-1234-1234-123456789abc=Ready"
```

This shows all nodes where the model with that UID is in "Ready" state. This labeling system allows workload schedulers to quickly find nodes with specific models without parsing ConfigMaps.

## Advanced Features and Performance

### Model Agent Configuration

The Model Agent can be configured with several performance and reliability parameters:

**Download Configuration:**
- `--download-retry` (default: 3): Number of retry attempts for failed downloads
- `--concurrency` (default: 4): Number of concurrent file downloads per model
- `--multipart-concurrency` (default: 4): Number of concurrent chunks for large file downloads
- `--num-download-worker` (default: 5): Number of parallel download workers across all models

**Node and Storage Configuration:**
- `--models-root-dir` (default: `/mnt/models`): Root directory for storing models on nodes
- `--node-label-retry` (default: 5): Number of retries for updating node labels
- `--port` (default: 8080): HTTP port for health checks and metrics

**Logging and Monitoring:**
- `--log-level` (default: "info"): Log verbosity (debug, info, warn, error)
- `--namespace` (default: "ome"): Kubernetes namespace for ConfigMaps and status tracking

### Advanced Download and Verification

**Bulk Download with Optimization:**
For OCI Object Storage, the Model Agent uses sophisticated bulk download strategies:

- **Concurrent file downloads**: Multiple files are downloaded simultaneously based on the `concurrency` setting
- **Multipart downloads**: Large files (>200MB) are split into chunks and downloaded in parallel
- **Resume capability**: Interrupted downloads automatically resume from the last successfully downloaded chunk
- **Prefix stripping**: Object prefixes are automatically stripped to create clean local directory structures

**Comprehensive Integrity Verification:**
Every downloaded file undergoes rigorous verification:

1. **Size verification**: Actual file size is compared against expected size from object metadata
2. **Checksum verification**: MD5 hashes are computed and verified against object storage metadata
3. **Atomic operations**: Files are downloaded to temporary locations and only moved to final destinations after successful verification
4. **Automatic retry**: Failed verifications trigger automatic re-download of corrupted files

The verification process is tracked with detailed metrics, including verification duration and failure rates.

### Thread Safety and Concurrent Operations

The Model Agent is designed for safe concurrent operations across multiple models and nodes:

**ConfigMap Coordination:**
A sophisticated mutex-based locking system ensures that ConfigMap updates (which track model status across nodes) are thread-safe. This prevents race conditions when multiple models are being processed simultaneously on the same node.

**Model Update Handling:**
When an existing model is updated, the agent intelligently handles the transition:

1. **Change Detection**: Uses deep comparison to detect actual changes in model specifications
2. **Graceful Updates**: Sets model status to "Updating" before starting the new download
3. **Override Downloads**: Uses `DownloadOverride` tasks to replace existing models
4. **Rollback Safety**: Maintains previous model versions until new downloads are verified

### Model Lifecycle Management

**Task Types:**
The Model Agent processes three types of operations:

1. **Download**: Initial download of a new model
2. **DownloadOverride**: Replace an existing model with an updated version
3. **Delete**: Remove a model from the node and clean up storage

**State Transitions:**
Models progress through well-defined states:

- `Updating` → `Ready`: Successful download and verification
- `Updating` → `Failed`: Download or verification failure
- `Ready` → `Updating`: Model update initiated
- `Ready` → `Deleted`: Model removal requested

### Health Checks and Monitoring

**Health Check Endpoints:**
The Model Agent exposes HTTP endpoints for cluster health monitoring:

- `/healthz`: General health check that verifies model root directory accessibility
- `/livez`: Kubernetes liveness probe endpoint
- `/metrics`: Prometheus metrics endpoint for detailed operational metrics

**Comprehensive Metrics:**
The agent provides detailed metrics for production monitoring:

```
# Download metrics
model_agent_downloads_success_total{model_type, namespace, name}
model_agent_downloads_failed_total{model_type, namespace, name}
model_agent_download_duration_seconds{model_type, namespace, name}

# Verification metrics  
model_agent_verifications_total{model_type, namespace, name, result}
model_agent_verification_duration_seconds
model_agent_md5_checksum_failed_total{model_type, namespace, name}

# Transfer metrics
model_agent_download_bytes_total{model_type, namespace, name}

# Runtime metrics
go_goroutines_current
go_memory_alloc_bytes
go_gc_pause_duration_seconds_custom
```

All metrics are labeled with model identifiers, enabling detailed dashboards and alerting.

### Data Type and Quantization Support

OME automatically detects and handles various model data types and quantization schemes:

**Supported Data Types:**
- `float32`/`float` (4 bytes per parameter)
- `bfloat16`/`bf16` (2 bytes per parameter) 
- `float16`/`fp16`/`half` (2 bytes per parameter)
- `int8` (1 byte per parameter)
- `fp8`/`float8`/`e4m3` (1 byte per parameter)
- `int4`/`4bit` (0.5 bytes per parameter)

**Quantization Detection:**
The system automatically detects quantization schemes from model configurations:
- FP8 quantization for memory-efficient inference
- INT4 quantization for extreme compression
- Custom quantization configurations from various frameworks

This information is used for accurate memory usage estimation and model size calculations.

## Complete Configuration Example

Here's a comprehensive example showing all the features working together:

```yaml
apiVersion: ome.io/v1beta1
kind: ClusterBaseModel
metadata:
  name: llama-3-70b-instruct
  labels:
    vendor: "meta"
    model-family: "llama"
    parameter-size: "70b"
spec:
  # Basic model information
  vendor: "meta"
  version: "3.1"
  disabled: false
  
  # Model capabilities (these will be auto-detected from config.json)
  modelType: "llama"
  modelArchitecture: "LlamaForCausalLM"
  modelParameterSize: "70B"
  maxTokens: 8192
  modelCapabilities:
    - TEXT_GENERATION
    - CHAT
  
  # Model format and framework information
  modelFormat:
    name: "safetensors"
    version: "1.0.0"
  modelFramework:
    name: "transformers"
    version: "4.36.0"
  
  # Storage configuration
  storage:
    # Where to download the model from
    storageUri: "oci://n/ai-models/b/llm-store/o/meta/llama-3.1-70b-instruct/"
    
    # Where to store it locally on each node
    path: "/raid/models/llama-3.1-70b-instruct"
    
    # Secret containing OCI credentials
    storageKey: "oci-model-credentials"
    
    # OCI-specific parameters
    parameters:
      region: "us-phoenix-1"
      auth_type: "InstancePrincipal"
    
    # Only download to nodes with appropriate hardware
    nodeSelector:
      node.kubernetes.io/instance-type: "GPU.A100.4"
      models.ome.io/storage-tier: "nvme"
    
    # Advanced node selection rules
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
        - matchExpressions:
          # Must have either A100 or H100 GPUs
          - key: "accelerator.nvidia.com/gpu-product"
            operator: In
            values: ["A100-SXM4-80GB", "H100-SXM5-80GB"]
          # Must have at least 200GB available storage
          - key: "models.ome.io/available-storage"
            operator: Gt
            values: ["200Gi"]
          # Must be in the correct availability zone
          - key: "topology.kubernetes.io/zone"
            operator: In
            values: ["us-phoenix-1a", "us-phoenix-1b"]
  
  # How the model can be served
  servingMode:
    - "On-demand"
    - "Dedicated"
  
  # Additional metadata for organization
  additionalMetadata:
    license: "Llama 3.1 Community License"
    description: "Meta Llama 3.1 70B Instruct model optimized for chat and instruction following"
    use_cases: "chat,assistant,instruction_following,code_generation"
    cost_center: "ai-research"
    owner: "ml-platform-team"
```

## Troubleshooting Common Issues

### Authentication Problems

**Symptom**: Downloads fail with "403 Forbidden" or "401 Unauthorized" errors.

**Solution**: Verify your storage credentials. For OCI, ensure your Instance Principal or User Principal has the necessary permissions to read from the specified bucket. For Hugging Face, verify your access token is valid and has permission to access the model.

### Node Selection Issues

**Symptom**: Models never download, or only download to some nodes.

**Solution**: Check your nodeSelector and nodeAffinity rules. Use `kubectl get nodes --show-labels` to see what labels your nodes actually have. Make sure at least some nodes match your selection criteria.

### Storage Space Problems

**Symptom**: Downloads fail with "no space left on device" errors.

**Solution**: Ensure nodes have sufficient disk space. Large models can be 100GB or more. Consider using node affinity rules to target nodes with adequate storage.

### Network Connectivity

**Symptom**: Downloads timeout or fail with network errors.

**Solution**: Verify that nodes can reach your storage endpoints. For OCI Object Storage, ensure nodes can reach the OCI API endpoints. For Hugging Face, ensure access to huggingface.co.

### Model Format Issues

**Symptom**: Models download but parsing fails.

**Solution**: Verify the model directory contains a valid `config.json` file. If you have a custom model format, consider disabling automatic parsing and specifying metadata manually.

## Best Practices for Production

### Storage Strategy

Use fast local storage (NVMe SSDs) for the model path to ensure quick model loading. Network storage can be slow and create bottlenecks during inference.

Consider using OCI Object Storage or similar cloud storage for centralized model management, then cache locally on nodes for performance.

### Security

Store all credentials in Kubernetes Secrets, never in plain text in your YAML files. Use appropriate RBAC to control who can create and modify BaseModel resources.

Consider using workload identity or instance principals instead of long-lived API keys when possible.

### Resource Planning

Large language models require significant storage and memory. Plan your node capacity accordingly. A 70B parameter model in float16 format requires about 140GB of storage and similar amounts of memory when loaded.

Use node affinity to ensure models only go to nodes that can handle them. Don't put a 70B model on a node with only 32GB of RAM.

### Monitoring

Set up monitoring for the Model Agent metrics to track download success rates, duration, and failures. Create alerts for persistent download failures.

Monitor node storage usage to ensure you don't run out of space as you add more models.

### Model Organization

Use consistent naming conventions for your models. Include version information and parameter size in the name when helpful.

Use labels and annotations to organize models by team, use case, or other relevant categories.

Consider using ClusterBaseModels for widely-used models and BaseModels for team-specific or experimental models.

This comprehensive guide should give you everything you need to understand and effectively use OME's model management capabilities. The system is designed to handle the complexity of modern AI model deployment while providing the flexibility to work with your existing infrastructure and security requirements.
