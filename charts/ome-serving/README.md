# OME Serving Helm Chart

Deploy ClusterBaseModels, ClusterServingRuntimes, and InferenceServices for LLM serving with sglang.

## Features

- **Auto-detection**: Model architecture, transformers version, size range, and served name are automatically detected from the model name
- **165 Models**: Built-in registry supports Qwen, Llama, DeepSeek, Mistral, Gemma, Phi, and more
- **Simplified Storage**: Use `hfModelId` for HuggingFace or `oci` for OCI Object Storage
- **Scope Options**: Create ClusterBaseModel (cluster-wide) or BaseModel (namespace-scoped)
- **PD Mode**: Support for Prefill-Decode disaggregated serving with `pdMode: true`

## Installation

```bash
helm install ome-serving ./charts/ome-serving -f values.yaml
```

## Configuration

### Minimal Model Configuration

Users only need to specify model name, storage, and GPU count. All architecture details are auto-detected:

```yaml
models:
  qwen3-8b:
    enabled: true
    vendor: qwen
    capabilities: [TEXT_TO_TEXT]
    hfModelId: Qwen/Qwen3-8B
    path: /raid/models/qwen/qwen3-8b
    runtime:
      gpus: 1
```

**That's it!** The chart auto-detects:
- `architecture: Qwen3ForCausalLM`
- `transformersVersion: "4.51.0"`
- `sizeRange: ["7B", "9B"]`
- `servedName: Qwen/Qwen3-8B`

### Values Structure

```yaml
# Global defaults for sglang runtime
defaults:
  image: docker.io/lmsysorg/sglang:v0.5.5.post3-cu129-amd64
  routerImage: fra.ocir.io/idqj093njucb/smg:v0.2.3.post1-dev
  memFrac: "0.9"
  minReplicas: 1
  maxReplicas: 1

# GPU resource presets (auto-selected based on gpus count)
gpuPresets:
  1: { cpu: 10, memory: 30Gi }
  2: { cpu: 20, memory: 80Gi }
  4: { cpu: 20, memory: 160Gi }
  8: { cpu: 40, memory: 320Gi }

# Models configuration
models:
  <model-name>:              # Must match registry entry
    enabled: true|false
    clusterScope: true       # Create ClusterBaseModel (default: true)
    namespaceScope: false    # Create BaseModel (default: false)
    namespace: <name>        # Required if namespaceScope: true
    createModel: true        # Create model resource (default: true)
    createRuntime: true      # Create runtime resource (default: true)

    vendor: <vendor-name>
    capabilities: [TEXT_TO_TEXT|IMAGE_TEXT_TO_TEXT|EMBEDDING|...]

    # Storage (choose one)
    hfModelId: <org>/<model>           # HuggingFace shorthand
    # OR
    oci:
      namespace: <oci-ns>
      bucket: <bucket>
      object: <object-path>
    # OR
    storageUri: <full-uri>             # Direct override

    path: /raid/models/<vendor>/<model>

    # Runtime config
    runtime:
      gpus: <count>                    # 1, 2, 4, or 8 (REQUIRED)
      # Optional overrides:
      # image: custom-image:tag
      # routerImage: custom-router:tag
      # memFrac: "0.85"
      # extraArgs: ["--flag", "value"]
```

## Examples

### Small Model (1 GPU)

```yaml
models:
  qwen3-0-6b:
    enabled: true
    vendor: qwen
    capabilities: [TEXT_TO_TEXT]
    hfModelId: Qwen/Qwen3-0.6B
    path: /raid/models/qwen/qwen3-0-6b
    runtime:
      gpus: 1
```

### Large Model (4 GPUs)

```yaml
models:
  deepseek-r1-distill-llama-70b:
    enabled: true
    vendor: deepseek
    capabilities: [TEXT_TO_TEXT]
    hfModelId: deepseek-ai/DeepSeek-R1-Distill-Llama-70B
    path: /raid/models/deepseek/deepseek-r1-distill-llama-70b
    runtime:
      gpus: 4
```

### Embedding Model

```yaml
models:
  qwen3-embedding-0-6b:
    enabled: true
    vendor: qwen
    capabilities: [EMBEDDING]
    hfModelId: Qwen/Qwen3-Embedding-0.6B
    path: /raid/models/qwen/qwen3-embedding-0-6b
    runtime:
      gpus: 1
```

### Vision Model

```yaml
models:
  qwen2-5-vl-7b-instruct:
    enabled: true
    vendor: qwen
    capabilities: [IMAGE_TEXT_TO_TEXT]
    hfModelId: Qwen/Qwen2.5-VL-7B-Instruct
    path: /raid/models/qwen/qwen2-5-vl-7b-instruct
    runtime:
      gpus: 1
```

### With Extra Arguments

```yaml
models:
  llama-3-1-70b-instruct:
    enabled: true
    vendor: meta
    capabilities: [TEXT_TO_TEXT]
    hfModelId: meta-llama/Meta-Llama-3.1-70B-Instruct
    path: /raid/models/meta/llama-3-1-70b-instruct
    runtime:
      gpus: 4
      extraArgs:
        - --enable-torch-compile
        - --max-running-requests
        - "512"
```

### Namespace-Scoped Model

```yaml
models:
  qwen3-4b:
    enabled: true
    clusterScope: false      # Don't create ClusterBaseModel
    namespaceScope: true     # Create BaseModel instead
    namespace: ml-team       # Target namespace

    vendor: qwen
    capabilities: [TEXT_TO_TEXT]
    hfModelId: Qwen/Qwen3-4B
    path: /raid/models/qwen/qwen3-4b
    runtime:
      gpus: 1
```

### OCI Object Storage

```yaml
models:
  my-custom-model:
    enabled: true
    vendor: custom
    capabilities: [TEXT_TO_TEXT]
    oci:
      namespace: my-oci-namespace
      bucket: my-bucket
      object: models/my-model
    path: /raid/models/custom/my-model
    key: oci-credentials  # Optional: secret key
    runtime:
      gpus: 2
```

### Runtime Only (No Model Creation)

Create only the ClusterServingRuntime without creating a model resource. Useful when the model already exists or is managed externally:

```yaml
models:
  qwen3-8b:
    enabled: true
    createModel: false     # Don't create ClusterBaseModel/BaseModel
    createRuntime: true    # Only create ClusterServingRuntime
    vendor: qwen
    capabilities: [TEXT_TO_TEXT]
    hfModelId: Qwen/Qwen3-8B
    runtime:
      gpus: 1
```

### Model Only (No Runtime Creation)

Create only the model resource without a runtime. Useful when using a shared runtime:

```yaml
models:
  qwen3-8b:
    enabled: true
    createModel: true      # Create ClusterBaseModel
    createRuntime: false   # Don't create ClusterServingRuntime
    vendor: qwen
    capabilities: [TEXT_TO_TEXT]
    hfModelId: Qwen/Qwen3-8B
    path: /raid/models/qwen/qwen3-8b
    runtime:
      gpus: 1
```

### PD Mode (Prefill-Decode Disaggregated)

For models that support disaggregated serving, enable PD mode to deploy with separate prefill (engine) and decode (decoder) components. PD mode uses RDMA/InfiniBand for high-performance inter-node communication.

#### Basic PD Mode Configuration

```yaml
models:
  kimi-k2-instruct:
    enabled: true
    pdMode: true           # Enable PD mode
    vendor: moonshot
    capabilities: [TEXT_TO_TEXT]
    hfModelId: moonshotai/Kimi-K2-Instruct
    runtime:
      gpus: 8
    # Optional: customize replicas for each component
    engine:
      minReplicas: 1
      maxReplicas: 2
    decoder:
      minReplicas: 1
      maxReplicas: 2
    router:
      minReplicas: 1
      maxReplicas: 1
```

#### Advanced PD Mode Configuration (RDMA Settings)

```yaml
models:
  mistral-7b-instruct:
    enabled: true
    pdMode: true
    vendor: mistral
    capabilities: [TEXT_TO_TEXT]
    hfModelId: mistralai/Mistral-7B-Instruct-v0.2
    runtime:
      gpus: 2
      ibDevice: mlx5_0      # InfiniBand device (default: mlx5_0)
      rdmaProfile: oci-roce # RDMA profile (default: oci-roce)
```

#### PD Mode Default Settings

```yaml
defaults:
  # ... other defaults ...
  ibDevice: mlx5_0        # InfiniBand device for RDMA
  rdmaProfile: oci-roce   # RDMA profile for network
```

#### What PD Mode Configures

When `pdMode: true`:

1. **Engine (Prefill)**: Runs with `--disaggregation-mode prefill`
   - Adds RDMA annotations for auto-injection
   - Enables `hostNetwork: true` for direct network access
   - Uses `/health` endpoint for probes

2. **Decoder**: New component with `--disaggregation-mode decode`
   - Same resources and configuration as engine
   - RDMA-enabled for high-speed communication

3. **Router**: Configured for PD disaggregation
   - Adds `--pd-disaggregation` flag
   - Uses `--prefill-selector` and `--decode-selector` instead of single selector
   - Routes requests between prefill and decode pods

Models that support PD mode: `kimi-k2-instruct`, `deepseek-rdma`, `llama-3-1-70b-instruct`, `llama-3-2-1b-instruct`, `llama-3-2-3b-instruct`, `llama-3-3-70b-instruct`, `llama-4-maverick-17b-128e-instruct-fp8`, `llama-4-scout-17b-16e-instruct`, `mistral-7b-instruct`, `mixtral-8x7b-instruct`

## Supported Models

The chart includes a built-in registry of **165 models**. Model names in values.yaml must match registry entries exactly.

### Qwen3
`qwen3-0-6b`, `qwen3-32b`, `qwen3-4b`, `qwen3-8b`, `qwen3-embedding-0-6b`, `qwen3-embedding-4b`, `qwen3-next-80b-a3b-instruct`

### Qwen3 (VL/MoE)
`qwen3-30b-a3b`, `qwen3-vl-235b-a22b-instruct`

### Qwen2.5 VL
`mimo-vl-7b-rl`, `qwen2-5-vl-7b-instruct`

### Qwen2 VL
`gme-qwen2-vl-2b-instruct`, `qwen2-vl-7b-instruct`

### Qwen2/Qwen1.5
`deepseek-r1-distill-qwen-1-5b`, `deepseek-r1-distill-qwen-14b`, `deepseek-r1-distill-qwen-32b`, `deepseek-r1-distill-qwen-7b`, `gte-qwen2-7b-instruct`, `qwen-7b-chat`, `qwen1-5-110b-chat`, `qwen1-5-32b-chat`, `qwen1-5-72b-chat`, `qwen1-5-7b-chat`, `qwen2-5-1-5b`, `qwen2-5-14b`, `qwen2-5-32b-instruct`, `qwen2-5-3b`, `qwen2-5-72b-instruct`, `qwen2-5-7b`, `qwen2-5-coder-32b-instruct`, `qwen2-5-coder-7b-instruct`, `qwen2-72b-instruct`, `qwen2-7b-instruct`, `skywork-or1-7b-preview`

### Meta Llama 4
`llama-4-maverick-17b-128e-instruct`, `llama-4-maverick-17b-128e-instruct-fp8`, `llama-4-maverick-17b-128e-instruct-fp8-grpc`, `llama-4-scout-17b-16e-instruct`

### Meta Llama Vision
`llama-3-2-11b-vision-instruct`, `llama-3-2-90b-vision-instruct`, `llama-3-2-90b-vision-instruct-fp8`

### Meta Llama
`deepseek-coder-7b-instruct-v1-5`, `deepseek-llm-7b-chat`, `deepseek-r1-distill-llama-70b`, `deepseek-r1-distill-llama-8b`, `falcon3-10b-instruct`, `hermes-2-pro-llama-3-8b`, `llama-2-13b`, `llama-2-13b-chat-hf`, `llama-2-70b`, `llama-2-70b-chat-hf`, `llama-2-7b`, `llama-2-7b-chat-hf`, `llama-3-1-405b-instruct-fp8`, `llama-3-1-70b-instruct`, `llama-3-1-8b-instruct`, `llama-3-1-8b-instruct-grpc`, `llama-3-1-nemotron-70b-instruct-hf`, `llama-3-1-nemotron-nano-8b-v1`, `llama-3-1-nemotron-ultra-253b-v1`, `llama-3-2-1b-instruct`, `llama-3-2-3b-instruct`, `llama-3-3-70b-instruct`, `llama-3-3-70b-instruct-fp8-dynamic`, `llama-3-70b-instruct`, `llama-3-8b-instruct`, `llama-guard-3-8b`, `smollm-1-7b`, `smollm2-1-7b-instruct`, `solar-10-7b-instruct-v1-0`, `vicuna-13b-v1-5`, `vicuna-7b-v1-5`

### LLaVA
`llava-next-72b`, `llava-onevision-qwen2-7b-ov`, `llava-v1-5-13b`, `nvila-8b`

### DeepSeek V3
`deepseek-rdma`, `deepseek-v3`, `deepseek-v3-0324`, `kimi-k2-instruct`

### DeepSeek V2
`deepseek-v2-lite-chat`

### DeepSeek VL
`deepseek-vl2`

### DeepSeek Janus
`janus-pro-7b`

### Mistral 3
`mistral-small-3-1-24b-instruct-2503`

### Mistral (Mixtral)
`mixtral-8x22b`, `mixtral-8x7b`, `mixtral-8x7b-instruct`

### Mistral
`e5-mistral-7b-instruct`, `mistral-7b-instruct`, `mistral-7b-instruct-v0-2`, `mistral-7b-instruct-v0-3`, `mistral-nemo-instruct-2407`

### Google Gemma 3
`gemma-3-12b-it`, `gemma-3-1b-it`, `gemma-3-4b-it`

### Google Gemma 2
`gemma-2-27b-it`, `gemma-2-2b-it`, `gemma-2-9b-it`

### Microsoft Phi 4 (Multimodal)
`phi-4-multimodal-instruct`

### Microsoft Phi (MoE)
`phi-3-5-moe-instruct`

### Microsoft Phi 3 Vision
`phi-3-vision-128k-instruct`

### Microsoft Phi
`phi-2`, `phi-3-5-mini-instruct`, `phi-3-mini-4k-instruct`, `phi-4`, `phi-4-mini-instruct`

### GLM 4 Vision
`glm-4-5v`

### GLM
`chatglm2-6b`, `glm-4-9b-chat`

### InternVL
`internvl2-5-8b`

### InternLM
`internlm2-20b`, `internlm2-7b`, `internlm2-7b-reward`

### Embedding Models
`bge-large-en-v1-5`, `bge-m3`, `bge-reranker-v2-m3`

### Code Models
`starcoder2-15b`, `starcoder2-7b`

### NVIDIA Nemotron
`jet-nemotron-2b`, `llama-3-3-nemotron-super-49b-v1`, `nvidia-nemotron-nano-12b-v2-vl-bf16`, `nvidia-nemotron-nano-9b-v2`

### Moonshot Kimi
`kimi-vl-a3b-instruct`

### Xiaomi MiMo
`mimo-7b-rl`

### xAI Grok
`grok-1`, `grok-2`

### MiniCPM
`minicpm-2b-sft-bf16`, `minicpm-v-2-6`, `minicpm3-4b`

### Falcon
`falcon-7b-instruct`

### Bloom
`bloomz-7b1`

### GPT-OSS
`gpt-oss-120b`, `gpt-oss-120b-bf16`, `gpt-oss-120b-grpc`, `gpt-oss-20b`, `gpt-oss-20b-bf16`, `gpt-oss-20b-grpc`

### GPT-like
`dolly-v2-12b`, `gpt-j-6b`, `stablelm-tuned-alpha-7b`, `xgen-7b-8k-inst`

### Cohere
`c4ai-command-r-v01`

### Databricks DBRX
`dbrx-instruct`

### IBM Granite
`granite-3-0-3b-a800m-instruct`, `granite-3-1-8b-instruct`

### LG EXAONE
`exaone-3-5-7-8b-instruct`

### Allen AI OLMo
`olmo-2-1124-7b-instruct`, `olmoe-1b-7b-0924`

### Baichuan
`baichuan2-13b-chat`, `baichuan2-7b-chat`

### Stability AI
`stablelm-2-12b-chat`

### OpenAI CLIP
`clip-vit-large-patch14-336`

### Other Models
`afm-4-5b-base`, `dots-ocr`, `dots-vlm1-inst`, `ernie-4-5-21b-a3b-pt`, `ling-lite`, `ling-plus`, `minimax-m2`, `mpt-7b`, `orion-14b-base`, `persimmon-8b-chat`, `tele-flm`, `xverse-moe-a36b`

## Selective Deployment

```bash
# Enable specific models via CLI
helm install ome-serving ./charts/ome-serving \
  --set models.qwen3-8b.enabled=true \
  --set models.deepseek-r1-distill-llama-70b.enabled=true
```

## Resources Created

For each enabled model, the chart creates:

1. **ClusterBaseModel** - Model definition with storage configuration (if `clusterScope: true`)
2. **BaseModel** - Namespace-scoped model definition (if `namespaceScope: true`)
3. **ClusterServingRuntime** - sglang runtime with probes, volumes, and resource configuration
4. **Namespace** - Dedicated namespace for InferenceService
5. **InferenceService** - Deployment orchestration

## What's Auto-Generated

The following are automatically configured from the model registry:

- Model architecture (e.g., `Qwen3ForCausalLM`, `LlamaForCausalLM`)
- Transformers version compatibility
- Model size range for matching
- Served model name for API compatibility
- Runtime name: `srt-<model-name>`
- Namespace/ISVC name: `<model-name>`
- sglang command with proper flags
- Prometheus annotations
- GPU tolerations
- Shared memory volume (`/dev/shm`)
- Health probes (readiness, liveness, startup)
- Router configuration with service discovery
- Resource requests/limits based on GPU count

## Adding Custom Models

To add a model not in the registry, edit `templates/_helpers.tpl` and add an entry:

```yaml
my-custom-model:
  architecture: LlamaForCausalLM
  transformersVersion: "4.43.0"
  autoSelect: true
  priority: 1
  sizeRange: ["7B", "9B"]
  servedName: my-org/my-custom-model
```

Then use the model name in values.yaml.
