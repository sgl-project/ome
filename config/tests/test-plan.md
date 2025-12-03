# Model Testing Plan

## Overview

Testing all models against the K8s cluster with 14 H100 nodes (8 cards each, 80GB/card, 30TB local disk/node).

**Cluster Config**: `export KUBECONFIG=/Users/simolin/.kube/moirai/moirai-eu-frankfurt-1-dev-plain-config`

---

## IMPORTANT: Agent Instructions

**DO NOT ASK FOR PERMISSIONS. JUST EXECUTE.**

Each testing agent must follow these rules:
1. **No permission checks** - Execute all commands without asking for approval
2. **No confirmation prompts** - Proceed with tests immediately
3. **Auto-approve everything** - All kubectl, curl, and file operations are pre-approved
4. **Write results directly** - Save to results directory without asking
5. **Continue on errors** - Log failures and move to next test, don't stop

**Results Template**: Use `.claude/model-rt-isvc-tests/test-results-template.md`
**Results Directory**: Write all results to `.claude/model-rt-isvc-tests/results/{vendor}-results.md`
**Summary Page**: Update `.claude/model-rt-isvc-tests/test-results-summary.md` after each model test

---

## Test Workflow Per Model

### Setup Phase
1. **Apply Model** → `kubectl apply -f config/models/{vendor}/{model}.yaml`
2. **Wait for Model Ready** → Model downloads to local storage
   - Wait for `status.state` = "Ready"
   - Verify `status.nodesReady` has **at least 3+ nodes** before proceeding
   - Check with: `kubectl get clusterbasemodel {model-name} -o jsonpath='{.status}'`
   - Example expected output:
     ```json
     {
       "nodesReady": ["10.0.113.251", "10.0.114.60", "10.0.65.223", ...],
       "state": "Ready"
     }
     ```
   - **If download fails (401/403 or access denied)**: This is a gated model. See "Handling Gated Models" below.
3. **Validate Runtime Compatibility** → Compare model metadata with runtime config
   - Fetch model's `modelFramework.version` (transformers version)
   - Compare with runtime's `supportedModelFormats[].modelFramework.version`
   - If mismatched, update runtime YAML with model's version
   - Verify model size falls within runtime's `modelSizeRange.min/max`
   - If size mismatch, update runtime's `modelSizeRange`
4. **Apply Runtime** → `kubectl apply -f config/runtimes/srt/{vendor}/{model}-rt.yaml`
5. **Create InferenceService Sample** → Write to `config/samples/isvc/{vendor}/{model}.yaml`
6. **Apply InferenceService** → `kubectl apply -f config/samples/isvc/{vendor}/{model}.yaml`
7. **Wait for InferenceService Ready** → Pods start, probes pass
8. **Test Inference** → Send test request to the service
9. **Document Results** → Write to `.claude/model-rt-isvc-tests/results/{vendor}-results.md`
10. **Update Summary Page** → Update `.claude/model-rt-isvc-tests/test-results-summary.md`
    - Update the vendor section: Change model status from `⏳ Not Tested` to `✅ Passed`, `❌ Failed`, or `⏭️ Skipped`
    - Add test date (YYYY-MM-DD format) and notes/failure reason
    - Update Quick Stats counters (increment Passed/Failed/Skipped, decrement Not Tested)
    - Add row to appropriate "Results by Status" section (Passed/Failed/Skipped)
    - Add entry to "Test Execution Log" at bottom

### Cleanup Phase (After Testing)
11. **Delete InferenceService** → `kubectl delete -f config/samples/isvc/{vendor}/{model}.yaml`
12. **Wait for Pods Terminated** → Ensure all pods are gone
13. **Delete Namespace** → `kubectl delete namespace {model-name}`
14. **Delete Runtime** → `kubectl delete -f config/runtimes/srt/{vendor}/{model}-rt.yaml`
15. **Delete Model** → `kubectl delete -f config/models/{vendor}/{model}.yaml`
16. **Verify Cleanup** → Confirm no orphaned resources remain

### Cleanup Commands Reference
```bash
# Delete in reverse order of creation
kubectl delete inferenceservice {model-name} -n {model-name}
kubectl delete namespace {model-name} --wait=true
kubectl delete clusterservingruntime srt-{model-name}
kubectl delete clusterbasemodel {model-name}

# Verify cleanup
kubectl get all -n {model-name}  # Should return "No resources found"
kubectl get clusterservingruntime srt-{model-name}  # Should return "not found"
kubectl get clusterbasemodel {model-name}  # Should return "not found"

# Optional: Clean up downloaded model files (if disk space needed)
# ssh to node and remove: /raid/models/{vendor}/{model-name}
```

### Important Cleanup Notes
- **Always delete InferenceService first** - This triggers pod termination
- **Wait for namespace deletion** - Use `--wait=true` to ensure complete cleanup
- **Model files persist on disk** - Downloaded models stay in `/raid/models/` for reuse
- **Disk cleanup** - Only delete model files if disk space is critical (re-download is slow)

---

## Models by Vendor

**Total Models**: 145

### adept (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| persimmon-8b-chat | 8B | Chat | 1 |

### Alibaba-NLP (2 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| gme-Qwen2-VL-2B-Instruct | 2B | Vision | 1 |
| gte-Qwen2-7B-instruct | 7B | Embedding | 1 |

### allenai (2 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| OLMo-2-1124-7B-Instruct | 7B | Dense | 1 |
| OLMoE-1B-7B-0924 | 7B | MoE | 1 |

### arcee-ai (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| AFM-4.5B-Base | 4.5B | Base | 1 |

### BAAI (3 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| bge-large-en-v1.5 | Small | Embedding | 1 |
| bge-m3 | Small | Embedding | 1 |
| bge-reranker-v2-m3 | Small | Reranker | 1 |

### baichuan-inc (2 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Baichuan2-7B-Chat | 7B | Chat | 1 |
| Baichuan2-13B-Chat | 13B | Chat | 2 |

### baidu (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| ERNIE-4.5-21B-A3B-PT | 21B | MoE | 2 |

### bigcode (3 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| starcoder2-3b | 3B | Code | 1 |
| starcoder2-7b | 7B | Code | 1 |
| starcoder2-15b | 15B | Code | 2 |

### CofeAI (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Tele-FLM | 52B | Dense | 4 |

### CohereForAI (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| c4ai-command-r-v01 | 35B | Chat | 4 |

### databricks (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| dbrx-instruct | 132B | MoE | 8 |

### deepseek-ai (14 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| DeepSeek-R1-Distill-Qwen-1.5B | 1.5B | Distill | 1 |
| DeepSeek-R1-Distill-Qwen-7B | 7B | Distill | 1 |
| Janus-Pro-7B | 7B | Vision | 1 |
| DeepSeek-R1-Distill-Llama-8B | 8B | Distill | 1 |
| DeepSeek-R1-Distill-Qwen-14B | 14B | Distill | 2 |
| DeepSeek-R1-Distill-Qwen-32B | 32B | Distill | 2 |
| DeepSeek-R1-Distill-Llama-70B | 70B | Distill | 4 |
| deepseek-vl2 | Large | Vision | 4 |
| DeepSeek-V2 | Large | MoE | 8 |
| DeepSeek-V2.5 | Large | MoE | 8 |
| DeepSeek-V3 | 671B | MoE | 32+ |
| DeepSeek-V3-0324 | 671B | MoE | 32+ |
| DeepSeek-R1 | 671B | MoE | 32+ |
| DeepSeek-R1-Zero | 671B | MoE | 32+ |

### Efficient-Large-Model (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| NVILA-8B | 8B | Vision | 1 |

### google (9 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| gemma-3-1b-it | 1B | Instruct | 1 |
| gemma-2b | 2B | Base | 1 |
| gemma-2-2b | 2B | Base | 1 |
| gemma-3-4b-it | 4B | Instruct | 1 |
| gemma-7b | 7B | Base | 1 |
| gemma-2-9b | 9B | Base | 1 |
| gemma-3-12b-it | 12B | Instruct | 2 |
| gemma-2-27b | 27B | Base | 2 |
| gemma-3-27b-it | 27B | Instruct | 2 |

### HuggingFaceTB (3 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| SmolLM-135M | 135M | Base | 1 |
| SmolLM-360M | 360M | Base | 1 |
| SmolLM-1.7B | 1.7B | Base | 1 |

### ibm-granite (5 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| granite-3.0-2b-instruct | 2B | Instruct | 1 |
| granite-3.1-2b-instruct | 2B | Instruct | 1 |
| granite-3.0-3b-a800m-instruct | 3B | MoE | 1 |
| granite-3.0-8b-instruct | 8B | Instruct | 1 |
| granite-3.1-8b-instruct | 8B | Instruct | 1 |

### inclusionAI (2 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Ling-lite | Small | MoE | 1 |
| Ling-plus | Large | MoE | 4 |

### intfloat (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| e5-mistral-7b-instruct | 7B | Embedding | 1 |

### internlm (3 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| internlm2-7b | 7B | Base | 1 |
| internlm2-7b-reward | 7B | Reward | 1 |
| internlm2-20b | 20B | Base | 2 |

### jason9693 (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Qwen2.5-1.5B-apeach | 1.5B | Fine-tuned | 1 |

### jet-ai (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Jet-Nemotron-2B | 2B | Chat | 1 |

### LGAI-EXAONE (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| EXAONE-3.5-7.8B-Instruct | 7.8B | Instruct | 1 |

### liuhaotian (4 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| llava-v1.5-7b | 7B | Vision | 1 |
| llava-v1.6-vicuna-7b | 7B | Vision | 1 |
| llava-v1.5-13b | 13B | Vision | 2 |
| llava-v1.6-vicuna-13b | 13B | Vision | 2 |

### lmms-lab (3 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| llava-onevision-qwen2-7b-ov | 7B | Vision | 1 |
| llava-next-8b | 8B | Vision | 1 |
| llava-next-72b | 72B | Vision | 4 |

### lmsys (2 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| gpt-oss-20b | 20B | Base | 2 |
| gpt-oss-120b | 120B | Base | 8 |

### meta (18 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Llama-3.2-1B-Instruct | 1B | Instruct | 1 |
| Llama-3.2-3B-Instruct | 3B | Instruct | 1 |
| Llama-2-7b-hf | 7B | Base | 1 |
| Llama-3.1-8B-Instruct | 8B | Instruct | 1 |
| Meta-Llama-3-8B-Instruct | 8B | Instruct | 1 |
| Llama-3.2-11B-Vision-Instruct | 11B | Vision | 2 |
| Llama-2-13b-hf | 13B | Base | 2 |
| Llama-4-Scout-17B-16E-Instruct | 17B | MoE | 2 |
| Llama-4-Maverick-17B-128E-Instruct | 17B | MoE | 2 |
| Llama-4-Maverick-17B-128E-Instruct-FP8 | 17B | MoE | 2 |
| Llama-2-70b-hf | 70B | Base | 4 |
| Llama-3-70B-Instruct | 70B | Instruct | 4 |
| Llama-3.1-70B-Instruct | 70B | Instruct | 4 |
| Llama-3.3-70B-instruct | 70B | Instruct | 4 |
| Llama-3.3-70B-Instruct-FP8-dynamic | 70B | Instruct | 4 |
| Llama-3.2-90B-Vision-Instruct | 90B | Vision | 8 |
| Llama-3.2-90B-Vision-Instruct-FP8 | 90B | Vision | 4 |
| Llama-3.1-405B-Instruct-FP8 | 405B | Instruct | 16 |

### microsoft (11 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| phi-1_5 | 1.5B | Base | 1 |
| phi-2 | 2.7B | Base | 1 |
| Phi-3-mini-4k-instruct | 3.8B | Instruct | 1 |
| Phi-3-mini-128k-instruct | 3.8B | Instruct | 1 |
| Phi-3.5-mini-instruct | 3.8B | Instruct | 1 |
| phi-4 | 14B | Base | 2 |
| Phi-4-multimodal-instruct | 14B | Multimodal | 2 |
| Phi-3-small-8k-instruct | 7B | Instruct | 1 |
| Phi-3-vision-128k-instruct | 4B | Vision | 1 |
| Phi-3-medium-4k-instruct | 14B | Instruct | 2 |
| Phi-3.5-MoE-instruct | 42B | MoE | 4 |

### minimax (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| MiniMax-M2 | Large | MoE | 8 |

### mistralai (8 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Mistral-7B-v0.1 | 7B | Base | 1 |
| Mistral-7B-Instruct-v0.2 | 7B | Instruct | 1 |
| Mistral-7B-Instruct-v0.3 | 7B | Instruct | 1 |
| Mistral-Nemo-Instruct-2407 | 12B | Instruct | 2 |
| Mistral-Small-3.1-24B-Instruct-2503 | 24B | Instruct | 2 |
| Mixtral-8x7B-v0.1 | 47B | MoE | 4 |
| Mixtral-8x7B-Instruct-v0.1 | 47B | MoE | 4 |
| Mixtral-8x22B-v0.1 | 141B | MoE | 8 |

### moonshotai (2 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Kimi-VL-A3B-Instruct | Large | Vision | 4 |
| Kimi-K2-Instruct | 1000B+ | MoE | 32+ |

### nvidia (5 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Llama-3.1-Nemotron-Nano-8B-v1 | 8B | Instruct | 1 |
| NVIDIA-Nemotron-Nano-9B-v2 | 9B | Instruct | 1 |
| NVIDIA-Nemotron-Nano-12B-v2-VL-BF16 | 12B | Vision | 2 |
| Llama-3_3-Nemotron-Super-49B-v1 | 49B | Instruct | 4 |
| Llama-3_1-Nemotron-Ultra-253B-v1 | 253B | Instruct | 16 |

### openai (3 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| clip-vit-large-patch14-336 | Small | Vision | 1 |
| gpt-oss-20b | 20B | Base | 2 |
| gpt-oss-120b | 120B | Base | 8 |

### openbmb (2 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| MiniCPM3-4B | 4B | Chat | 1 |
| MiniCPM-V-2_6 | 8B | Vision | 1 |

### OrionStarAI (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Orion-14B-Base | 14B | Base | 2 |

### Qwen (26 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Qwen2.5-0.5B | 0.5B | Base | 1 |
| Qwen3-0.6B | 0.6B | Base | 1 |
| Qwen3-Embedding-0.6B | 0.6B | Embedding | 1 |
| Qwen2.5-1.5B | 1.5B | Base | 1 |
| Qwen3-1.7B | 1.7B | Base | 1 |
| Qwen2-VL-2B-Instruct | 2B | Vision | 1 |
| Qwen2.5-3B | 3B | Base | 1 |
| Qwen3-4B | 4B | Base | 1 |
| Qwen3-Embedding-4B | 4B | Embedding | 1 |
| Qwen2.5-7B | 7B | Base | 1 |
| Qwen2-VL-7B-Instruct | 7B | Vision | 1 |
| Qwen-VL | 7B | Vision | 1 |
| Qwen-VL-Chat | 7B | Vision | 1 |
| Qwen3-8B | 8B | Base | 1 |
| Qwen3-Embedding-8B | 8B | Embedding | 1 |
| Qwen2.5-14B | 14B | Base | 2 |
| Qwen3-14B | 14B | Base | 2 |
| Qwen3-30B-A3B | 30B | MoE | 2 |
| Qwen2.5-32B | 32B | Base | 2 |
| Qwen3-32B | 32B | Base | 2 |
| Qwen2.5-72B | 72B | Base | 4 |
| Qwen2-VL-72B-Instruct | 72B | Vision | 4 |
| Qwen2.5-Math-RM-72B | 72B | Reward | 4 |
| Qwen3-Next-80B-A3B-Instruct | 80B | MoE | 4 |
| Qwen3-VL-235B-A22B-Instruct | 235B | Vision+MoE | 16 |

### rednote-hilab (2 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| dots.ocr | Small | OCR | 1 |
| dots.vlm1.inst | Small | Vision | 1 |

### Skywork (2 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| Skywork-Reward-Llama-3.1-8B-v0.2 | 8B | Reward | 1 |
| Skywork-Reward-Gemma-2-27B-v0.2 | 27B | Reward | 2 |

### stabilityai (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| stablelm-tuned-alpha-7b | 7B | Chat | 1 |

### THUDM (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| chatglm2-6b | 6B | Chat | 1 |

### upstage (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| SOLAR-10.7B-Instruct-v1.0 | 10.7B | Instruct | 2 |

### xai-org (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| grok-1 | 314B | MoE | 16+ |

### XiaomiMiMo (2 models)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| MiMo-7B-RL | 7B | Chat | 1 |
| MiMo-VL-7B-RL | 7B | Vision | 1 |

### xverse (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| XVERSE-MoE-A36B | 36B | MoE | 4 |

### zai-org (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| GLM-4.5V | Large | Vision | 4 |

### ZhipuAI (1 model)
| Model | Size | Type | GPUs |
|-------|------|------|------|
| glm-4-9b-chat | 9B | Chat | 1 |

---

## Testing Strategy

### Approach: Test by Vendor
- Group tests by vendor to maximize weight sharing
- Start with vendors that have small models for validation
- Progress to larger models within each vendor

### Priority Order
1. **Small model vendors first** (validation): HuggingFaceTB, Qwen (small), google (small)
2. **Medium model vendors**: meta, microsoft, mistralai, deepseek-ai
3. **Large model vendors**: nvidia, Qwen (large), meta (large)
4. **XXL models last**: deepseek-ai (V3/R1), moonshotai, xai-org

### Concurrency Guidelines
- **1 GPU models**: Up to 10 concurrent tests
- **2 GPU models**: Up to 5 concurrent tests
- **4 GPU models**: Up to 3 concurrent tests
- **8+ GPU models**: 1 at a time

---

## InferenceService Sample Template

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: {model-name}
  namespace: {model-name}
spec:
  model:
    name: {model-name}
  engine:
    minReplicas: {replicas}
    maxReplicas: {replicas}
  runtime:
    name: srt-{model-name}
  router:
    minReplicas: 1
    maxReplicas: 1
```

---

## Runtime Validation Step (Step 3)

After the model is ready, validate and update the runtime configuration before applying it.

### What to Validate

1. **Transformers Version**: Model's `spec.modelFramework.version` must match runtime's `spec.supportedModelFormats[].modelFramework.version`
2. **Model Size Range**: Model size must fall within runtime's `spec.modelSizeRange.min` and `spec.modelSizeRange.max`
3. **Model Architecture**: Model's `spec.modelArchitecture` should match runtime's `spec.supportedModelFormats[].modelArchitecture`

### Validation Commands

```bash
# Get model's transformers version from the applied ClusterBaseModel
MODEL_TRANSFORMERS_VERSION=$(kubectl get clusterbasemodel {model-name} -o jsonpath='{.spec.modelFramework.version}')
echo "Model transformers version: $MODEL_TRANSFORMERS_VERSION"

# Get model architecture
MODEL_ARCH=$(kubectl get clusterbasemodel {model-name} -o jsonpath='{.spec.modelArchitecture}')
echo "Model architecture: $MODEL_ARCH"

# Check current runtime's transformers version (from YAML file)
yq '.spec.supportedModelFormats[0].modelFramework.version' config/runtimes/srt/{vendor}/{model}-rt.yaml

# Check current runtime's model size range
yq '.spec.modelSizeRange' config/runtimes/srt/{vendor}/{model}-rt.yaml
```

### Update Runtime if Mismatched

```bash
# Update transformers version in runtime YAML
yq -i '.spec.supportedModelFormats[0].modelFramework.version = "'$MODEL_TRANSFORMERS_VERSION'"' \
  config/runtimes/srt/{vendor}/{model}-rt.yaml

# Update model architecture in runtime YAML
yq -i '.spec.supportedModelFormats[0].modelArchitecture = "'$MODEL_ARCH'"' \
  config/runtimes/srt/{vendor}/{model}-rt.yaml

# Update model size range (example for 7B model)
yq -i '.spec.modelSizeRange.min = "5B"' config/runtimes/srt/{vendor}/{model}-rt.yaml
yq -i '.spec.modelSizeRange.max = "9B"' config/runtimes/srt/{vendor}/{model}-rt.yaml
```

### Model Size Reference

| Model Size | Recommended min | Recommended max |
|------------|-----------------|-----------------|
| 1B         | 0.5B            | 2B              |
| 3B         | 2B              | 5B              |
| 7B         | 5B              | 9B              |
| 8B         | 6B              | 10B             |
| 13B        | 10B             | 15B             |
| 20B        | 15B             | 25B             |
| 70B        | 60B             | 80B             |
| 90B        | 80B             | 100B            |
| 405B       | 350B            | 450B            |

---

## Handling Gated Models

Some HuggingFace models are "gated" and require authentication to download. If a model download fails with 401/403 or "access denied" errors, follow these steps:

### Step 1: Add Comment to Model YAML

Add a comment indicating the model is gated:

```yaml
# This model is gated on HuggingFace and requires authentication
apiVersion: ome.io/v1beta1
kind: ClusterBaseModel
metadata:
  name: {model-name}
```

### Step 2: Add HF Token Key to Storage

Update the model's storage section to include the `key` field:

```yaml
spec:
  storage:
    storageUri: hf://meta-llama/Llama-3.2-3B-Instruct
    path: /raid/models/meta/llama-3-2-3b-instruct
    key: "hf-token"
```

### Step 3: Re-apply and Retry

```bash
# Re-apply the updated model
kubectl apply -f config/models/{vendor}/{model}.yaml

# Wait for model to be ready (should now authenticate)
kubectl wait --for=condition=Ready clusterbasemodel/{model-name} --timeout=30m
```

### yq Command to Add Key

```bash
# Add the hf-token key to an existing model YAML
yq -i '.spec.storage.key = "hf-token"' config/models/{vendor}/{model}.yaml

# Verify the change
yq '.spec.storage' config/models/{vendor}/{model}.yaml
```

### Known Gated Model Vendors

The following vendors typically have gated models requiring authentication:

| Vendor       | Models                                      | Notes                    |
|--------------|---------------------------------------------|--------------------------|
| meta-llama   | Llama-3.x, Llama-4.x series                 | Most Meta models gated   |
| mistralai    | Mistral-* series                            | Some models gated        |
| google       | gemma-* series                              | Gemma models gated       |
| bigcode      | starcoder2-*                                | StarCoder gated          |
| CohereForAI  | c4ai-command-r-*                            | Cohere models gated      |
| deepseek-ai  | DeepSeek-V3, DeepSeek-R1                    | Large DeepSeek gated     |
| xai-org      | grok-*                                      | Grok models gated        |
| moonshotai   | Kimi-*                                      | Kimi models gated        |

### Identifying Gated Models

Signs a model is gated:

1. **Download Error**: `401 Unauthorized` or `403 Forbidden` in model pod logs
2. **Status Message**: ClusterBaseModel status shows "access denied" or "authentication required"
3. **HuggingFace Page**: Model page shows "gated" badge or requires accepting terms

```bash
# Check model status for auth errors
kubectl get clusterbasemodel {model-name} -o jsonpath='{.status}'

# Check pod logs for download errors
kubectl logs -n ome-system -l app=ome-controller --tail=100 | grep -i "401\|403\|auth\|denied"
```

---

## Commands Reference

```bash
# Set kubeconfig
export KUBECONFIG=/Users/simolin/.kube/moirai/moirai-eu-frankfurt-1-dev-plain-config

# Apply model
kubectl apply -f config/models/{vendor}/{model}.yaml

# Check model status
kubectl get clusterbasemodel {model-name} -o jsonpath='{.status.conditions}'

# Get model transformers version (after model is ready)
kubectl get clusterbasemodel {model-name} -o jsonpath='{.spec.modelFramework.version}'

# Get model size from model YAML (for runtime modelSizeRange validation)
# Check the model tier table above for size reference

# Validate runtime compatibility before applying
# Compare model's spec.modelFramework.version with runtime's spec.supportedModelFormats[0].modelFramework.version
# Compare model size with runtime's spec.modelSizeRange.min/max

# Apply runtime
kubectl apply -f config/runtimes/srt/{vendor}/{model}-rt.yaml

# Check runtime status
kubectl get clusterservingruntime srt-{model-name}

# Create namespace for inference service
kubectl create namespace {model-name}

# Apply inference service
kubectl apply -f config/samples/isvc/{vendor}/{model}.yaml

# Check inference service status
kubectl get inferenceservice -n {model-name}

# Check pods
kubectl get pods -n {model-name}

# Test inference (example)
# Port-forward to the engine service
kubectl port-forward svc/{model-name}-engine 8080:8080 -n {model-name}

# For chat models - use the HuggingFace model ID as served-model-name
# Example: meta-llama/Llama-3.1-8B-Instruct
curl -s http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "{vendor}/{Model-Name}",
    "messages": [{"role": "user", "content": "hello, who are you"}],
    "max_tokens": 100,
    "temperature": 0
  }'

# For completion models (non-chat)
curl -s http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "{vendor}/{Model-Name}",
    "prompt": "Hello, my name is",
    "max_tokens": 50,
    "temperature": 0
  }'

# For embedding models
curl -s http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "{vendor}/{Model-Name}",
    "input": "Hello world"
  }'

# Note: The model name in the request must match the --served-model-name
# argument in the runtime, which is the HuggingFace model ID (e.g., meta-llama/Llama-3.1-8B-Instruct)
```

---

## Output Documentation Structure

Results will be written to `.claude/model-rt-isvc-tests/results/`:
```
.claude/model-rt-isvc-tests/
├── test-plan.md              # This file
├── test-results-template.md  # Template for results
└── results/
    ├── summary.md            # Overall summary
    ├── google-results.md     # Per-vendor results
    ├── meta-results.md
    ├── Qwen-results.md
    ├── ...
    └── failed-models.md      # List of failures with errors
```

**Template Usage**: Copy `test-results-template.md` and fill in for each vendor.
**File Naming**: Use `{vendor}-results.md` (lowercase vendor name)

---

## Resource Limits

| Cluster Resource | Total            | Per Test Limit                       |
|------------------|------------------|--------------------------------------|
| H100 GPUs        | 112 (14x8)       | Max 32 for XXL models                |
| GPU Memory       | 8.96 TB          | 80GB per card                        |
| Local Disk       | 420 TB (14x30TB) | Model storage                        |
| Concurrent Tests | -                | 10 for small, 3 for large, 1 for XXL |

---

## Risk Mitigation

1. **Disk Space**: Monitor `/raid/models` usage, clean up after tests
2. **GPU OOM**: Start with smaller replicas, scale up
3. **Download Failures**: Retry with backoff, check HF token
4. **Timeout**: Large models may take hours to download and start
5. **Cleanup**: Delete InferenceService → Runtime → Model after each test

---

## Next Steps

1. [ ] Start with Phase 1: Test google/gemma-3-1b-it
2. [ ] Validate the complete workflow works
3. [ ] Create sample InferenceService files for all models
4. [ ] Run parallel tests per tier
5. [ ] Document all results
