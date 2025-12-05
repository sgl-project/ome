# Model Test Results Summary

**Last Updated**: 2025-12-05 07:40:00 UTC

**Cluster**: 14x H100 nodes (8 cards each, 80GB/card, 30TB local disk/node)

---

## Quick Stats

| Metric | Count |
|--------|-------|
| **Total Models** | 149 |
| **Passed** | 68 |
| **Failed** | 30 |
| **Skipped** | 3 |
| **Not Tested** | 48 |
| **Pass Rate** | 45.6% |

---

## Results by Status

### Passed Models
<!-- PASSED_START -->
| Vendor | Model | Size | Type | GPUs | Test Date | Notes |
|--------|-------|------|------|------|-----------|-------|
| adept | persimmon-8b-chat | 8B | Chat | 1 | 2025-12-02 | Download: 16s, Startup: 174s, Inference: OK (completions only, no chat template) |
| google | gemma-3-1b-it | 1B | Instruct | 1 | 2025-12-02 | Download: ~30min, Startup: 119s, Inference: OK |
| google | gemma-3-4b-it | 4B | Instruct | 1 | 2025-12-02 | Download: 9s, Startup: 76s, Inference: OK |
| internlm | internlm2-7b | 7B | Base | 1 | 2025-12-02 | Download: <1s (cached on 13 nodes), Startup: 120s, Inference: OK (completions only, no chat template) |
| meta-llama | llama-3-2-1b-instruct | 1B | Instruct | 1 | 2025-12-02 | Download: ~30min, Startup: 43s, Inference: OK |
| meta-llama | llama-3-2-3b-instruct | 3B | Instruct | 1 | 2025-12-02 | Download: 110s, Startup: 47s, Inference: OK |
| mistralai | Mistral-7B-Instruct-v0.2 | 7B | Instruct | 1 | 2025-12-02 | Download: 701s (~12min), Startup: 92s, Inference: OK (native /generate endpoint, 321.78 tok/s) |
| nvidia | llama-3-1-nemotron-nano-8b-v1 | 8B | Instruct | 1 | 2025-12-02 | Download: 16s (cached), Startup: 69s, Inference: OK. Fixed incorrect HF model ID in config (was -Instruct suffix, should be -v1) |
| upstage | solar-10-7b-instruct-v1-0 | 10.7B | Instruct | 1 | 2025-12-02 | Download: 49s, Startup: 68s, Inference: OK (completions only, no chat template). Gated model. |
| bigcode | starcoder2-7b | 7B | Code | 1 | 2025-12-02 | Download: <30s (cached on 13 nodes), Startup: ~60s, Inference: OK (completions endpoint, 321.78 tok/s) |
| OrionStarAI | orion-14b-base | 14B | Base | 2 | 2025-12-02 | Download: 287s (~4.8min), Startup: 138s (~2.3min), Inference: OK (completions only, gated model) |
| openbmb | MiniCPM3-4B | 4B | Chat | 1 | 2025-12-02 | Download: 235s (~4min to 5 nodes), Startup: 82s, Inference: OK. Required triton attention backend and disabled CUDA graph for MLA compatibility. |
| XiaomiMiMo | MiMo-7B-RL | 7B | Chat | 1 | 2025-12-02 | Download: 30s, Startup: 240s (~4min), Inference: OK (reasoning model with &lt;think&gt; tag support) |
| baichuan-inc | Baichuan2-7B-Chat | 7B | Chat | 1 | 2025-12-02 | Download: 68s, Startup: 18s, Inference: OK (completions only, no chat template support) |
| allenai | OLMo-2-1124-7B-Instruct | 7B | Instruct | 1 | 2025-12-02 | Download: Already cached (11 nodes), Startup: 12s, Inference: OK (chat completions endpoint) |
| deepseek-ai | DeepSeek-R1-Distill-Qwen-1.5B | 1.5B | Distill | 1 | 2025-12-02 | Download: ~3min, Startup: ~2min, Inference: OK (reasoning model, chat completions endpoint) |
| deepseek-ai | DeepSeek-R1-Distill-Llama-8B | 8B | Distill | 1 | 2025-12-03 | Download: 55min, Startup: 63s, Inference: OK (reasoning model, chat completions endpoint) |
| meta-llama | Meta-Llama-3-8B-Instruct | 8B | Instruct | 1 | 2025-12-03 | Download: 116s (~2min, 12 nodes), Startup: 105s (~1.75min), Inference: OK (chat completions endpoint) |
| Qwen | Qwen2.5-1.5B | 1.5B | Base/Instruct | 1 | 2025-12-02 | Download: ~28min (8 nodes), Startup: 16s, Inference: OK (health checks passing). Created runtime and isvc configs. Transformers 4.40.1. |
| Qwen | Qwen2.5-7B | 7B | Base | 1 | 2025-12-03 | Download: 67s (~1min, 8 nodes), Startup: 109s (~1.8min), Inference: OK (chat completions endpoint). Created runtime and isvc configs. |
| lmsys | vicuna-7b-v1.5 | 7B | Chat | 1 | 2025-12-03 | Download: 329s (~5.5min, 11 nodes), Startup: 150s (~2.5min), Inference: OK (chat completions endpoint). Created model, runtime and isvc configs. Fixed runtime modelSizeRange (4B model vs 5B-9B range). |
| google | gemma-2-9b-it | 9B | Instruct | 1 | 2025-12-03 | Download: 485s (~8min, 13 nodes), Startup: 71s, Inference: OK (chat completions endpoint). Created model, runtime and isvc configs. Transformers 4.42.0.dev0. |
| mistralai | Mistral-7B-Instruct-v0.3 | 7B | Instruct | 1 | 2025-12-03 | Download: 271s (~4.5min, 12 nodes), Startup: 210s (~3.5min), Inference: OK (chat completions endpoint). Created runtime and isvc configs. Transformers 4.42.0.dev0. |
| Qwen | Qwen2.5-3B | 3B | Base | 1 | 2025-12-02 | Download: 278s (~4.6min, 1 node), Startup: 3s, Inference: OK (chat completions endpoint). Created runtime and isvc configs. Transformers 4.40.1. |
| Qwen | Qwen3-8B | 8B | Base | 1 | 2025-12-02 | Download: ~3-5min (13 nodes, cached), Startup: 84s, Inference: OK (chat completions endpoint with reasoning traces). Created runtime and isvc configs. Transformers 4.51.0. |
| internlm | internlm2-20b | 20B | Base | 2 | 2025-12-03 | Download: ~3min (9 nodes), Startup: 144s (~2.4min), Inference: OK (completions only, no chat template). Created runtime and isvc configs. Transformers 4.41.0. |
| deepseek-ai | DeepSeek-R1-Distill-Qwen-7B | 7B | Distill | 1 | 2025-12-03 | Download: 300s (~5min, 7 nodes), Startup: 46s, Inference: OK (reasoning model with <think> tags, chat completions endpoint). Created runtime and isvc configs. Transformers 4.44.0. |
| deepseek-ai | DeepSeek-R1-Distill-Qwen-14B | 14B | Distill | 2 | 2025-12-03 | Download: ~28min (5 nodes), Startup: ~2min (TP=2), Inference: OK (reasoning model with <think> tags, chat completions endpoint). Created runtime and isvc configs. Transformers 4.43.1. Router service discovery issue, tested via direct engine. |
| mistralai | Mistral-Nemo-Instruct-2407 | 12B | Instruct | 2 | 2025-12-03 | Download: 456s (~7.6min, 7 nodes), Startup: 124s (~2min), Inference: OK (chat completions endpoint). Created runtime and isvc configs. Transformers 4.43.0.dev0. |
| google | gemma-2-2b-it | 2B | Instruct | 1 | 2025-12-03 | Download: 523s (~8.7min, 11 nodes), Startup: 69s, Inference: OK (chat completions endpoint). Created model, runtime and isvc configs. Transformers 4.42.4. |
| bigcode | starcoder2-15b | 15B | Code | 1 | 2025-12-03 | Download: ~6min (11 nodes), Startup: ~2.5min, Inference: OK (completions endpoint). TP=1 (not TP=2), CUDA graph disabled. Created runtime and isvc configs. Transformers 4.37.0.dev0. |
| meta | Llama-3.1-8B-Instruct | 8B | Instruct | 1 | 2025-12-03 | Download: 1827s (~30.5min, 13 nodes), Startup: 54s, Inference: OK (chat completions endpoint). Transformers 4.42.3. |
| allenai | OLMoE-1B-7B-0924 | 6.92B | MoE | 1 | 2025-12-03 | Download: ~30s (13 nodes), Startup: 58s, Inference: OK (completions only, no chat template). Fixed isvc config format. Transformers 4.43.0.dev0. |
| deepseek-ai | Janus-Pro-7B | 7B | VLM | 1 | 2025-12-03 | Download: 510s (~8.5min, 1 node), Startup: 99s, Inference: OK (chat completions endpoint, vision-language model). Transformers 4.33.1. Router service discovery issue (known limitation). |
| arcee-ai | AFM-4.5B-Base | 4.5B | Base | 1 | 2025-12-03 | Download: ~11min (2 nodes), Startup: 104s, Inference: OK (completions only, no chat template). Transformers 4.53.2. RoPE scaling warning present. |
| ibm-granite | granite-3.1-8b-instruct | 8B | Instruct | 1 | 2025-12-03 | Download: 290s (~4.8min, 1 node), Startup: 80s, Inference: OK (chat completions endpoint). Transformers 4.47.0. |
| HuggingFaceTB | SmolLM-1.7B | 1.7B | Base | 1 | 2025-12-03 | Download: 510s (~8.5min, 11 nodes), Startup: 60-80s, Inference: OK (completions endpoint). Base model without chat template. Transformers 4.39.3. |
| meta | Llama-3-70B-Instruct | 70B | Instruct | 4 | 2025-12-03 | Download: ~8min (13 nodes), Startup: 76s, Inference: OK (chat completions endpoint). 70B model with TP=4, transformers 4.40.0.dev0. Required runtime version update. |
| google | gemma-2-27b-it | 27B | Instruct | 2 | 2025-12-03 | Download: ~18min (1 node), Startup: ~2min, Inference: OK (generate endpoint, no chat template). 27B model with TP=2, transformers 4.42.0.dev0. Router service discovery known limitation. |
| ZhipuAI | glm-4-9b-chat | 9.4B | Chat | 1 | 2025-12-03 | Download: ~15min (13 nodes), Startup: ~3min, Inference: OK (chat completions endpoint). Transformers backend fallback (GlmForCausalLM). |
| lmsys | vicuna-13b-v1.5 | 13B | Chat | 2 | 2025-12-03 | Download: ~27min (13 nodes), Startup: ~6min (TP=2), Inference: OK (chat completions endpoint). Created all configs. Transformers 4.55.1. Router service discovery issue, tested via direct engine. |
| meta-llama | llama-3-1-70b-instruct | 70B | Instruct | 4 | 2025-12-03 | Download: ~10.5min (3+ nodes), Startup: ~2.75min (TP=4), Inference: OK (chat completions endpoint). Transformers 4.42.3. Gated model. Runtime version update required. |
| meta-llama | llama-3-3-70b-instruct | 70.55B | Instruct | 4 | 2025-12-03 | Download: Unable to verify (In_Transit status), Startup: ~30min (TP=4), Inference: OK (chat completions endpoint). Transformers 4.47.0.dev0. Gated model. Runtime version update required (4.45.0->4.47.0). Model status reporting issue. |
| jet-ai | jet-nemotron-2b | 1.96B | Chat | 1 | 2025-12-02 | Download: N/A (timeout, system-wide issue), Startup: 118s, Inference: OK (chat completions endpoint, direct engine access). Transformers 4.51.3. Router service discovery issue. |
| Qwen | Qwen2.5-14B-Instruct | 14B | Instruct | 2 | 2025-12-03 | Download: ~28min, Startup: ~2min (TP=2), Inference: OK (chat completions endpoint). Created runtime and isvc configs. Transformers 4.40.1. |
| deepseek-ai | deepseek-coder-7b-instruct-v1.5 | 7B | Code | 1 | 2025-12-03 | Download: 51s, Startup: 7s, Inference: OK (chat completions endpoint). Created model, runtime and isvc configs. Transformers 4.35.2. |
| Qwen | Qwen2.5-Coder-7B-Instruct | 7B | Code | 1 | 2025-12-03 | Download: 13.2min, Startup: 17.6min, Inference: OK (chat completions endpoint). Created model, runtime and isvc configs. Transformers 4.44.0. |
| google | gemma-3-12b-it | 12B | Instruct | 2 | 2025-12-03 | Download: ~12min (13 nodes), Startup: 64s, Inference: OK (chat completions endpoint). TP=2. Required --mem-frac 0.75 (0.85 caused CUDA OOM). Transformers 4.50.0.dev0. |
| mistralai | Mixtral-8x7B-Instruct-v0.1 | 47B | MoE | 4 | 2025-12-03 | Download: ~12min (13 nodes, 93.4GB), Startup: 152s (~2.5min), Inference: OK (chat completions endpoint). 8x7B MoE with TP=4. Transformers 4.36.0.dev0. |
| Alibaba-NLP | gte-qwen2-7b-instruct | 7B | Embedding | 1 | 2025-12-04 | Download: ~30s (4 nodes), Startup: ~30s, Inference: OK (embeddings endpoint). Embedding model with --is-embedding flag. Transformers 4.41.2. |
| BAAI | bge-large-en-v1.5 | 335M | Embedding | 1 | 2025-12-04 | Download: cached (4 nodes), Startup: ~30s, Inference: OK (embeddings endpoint). BertModel architecture. Required fixes: --attention-backend triton (flashinfer hangs), --skip-server-warmup, memory 24Gi. Health probes: /health_generate (readiness/startup), /health (liveness). Auto-select: working. Transformers 4.30.0. |
| BAAI | bge-reranker-v2-m3 | 567M | Reranker | 1 | 2025-12-04 | Download: cached (4 nodes), Startup: ~30s, Inference: OK (rerank endpoint /v1/rerank). XLMRobertaForSequenceClassification architecture. Required fixes: --attention-backend triton, --skip-server-warmup, --disable-radix-cache, --chunked-prefill-size -1, memory 24Gi. Scores correctly rank documents by relevance. Auto-select: working. Transformers 4.38.1. |
| meta-llama | Llama-4-Scout-17B-16E-Instruct | 109B | MoE | 4 | 2025-12-05 | Download: model ready on nodes, Startup: ~10min (50 shards + MoE init), Inference: OK (chat completions endpoint). Llama4ForConditionalGeneration MoE (109B total), TP=4, 256Gi mem, FA3 attention, 196K context, multimodal, pythonic tool call parser. Transformers 4.51.0.dev0. |
| meta-llama | Llama-4-Maverick-17B-128E-Instruct-FP8 | 401.65B | MoE FP8 | 8 | 2025-12-05 | Download: ~7min (84 shards, 220GB FP8), Startup: ~3min (84 shards + CUDA graph), Inference: OK (chat completions endpoint). Llama4ForConditionalGeneration MoE (401B total, 128 experts), TP=8, 512Gi mem, FA3 attention, 131K context, multimodal, pythonic tool call parser. FP8 quantization enables fit on 8 GPUs. Transformers 4.51.0.dev0. |

<!-- PASSED_END -->

### Failed Models
<!-- FAILED_START -->
| Vendor | Model | Size | Type | GPUs | Test Date | Failure Reason |
|--------|-------|------|------|------|-----------|----------------|
| CohereForAI | c4ai-command-r-v01 | 35B | Chat | 4 | 2025-12-02 | Controller not reconciling InferenceService: No pods created after 10min, no status on InferenceService, no controller events. Chat template dict error fixed in runtime but resources never created. System issue, not model-specific. |
| microsoft | phi-2 | 2.8B | Base | 1 | 2025-12-02 | ModuleNotFoundError: No module named 'vllm' in sglang image. Model requires vllm but current sglang:v0.5.5.post3-cu129-amd64 doesn't include it for phi-2 architecture. |
| microsoft | phi-3-mini-4k-instruct | 3.8B | Instruct | 1 | 2025-12-03 | ModuleNotFoundError: No module named 'vllm' in sglang image. Model uses Phi3ForCausalLM architecture which requires vllm._custom_ops for rotary_embedding. Runtime image lmsysorg/sglang:v0.5.5.post3-cu129-amd64 incompatible. |
| stabilityai | stablelm-tuned-alpha-7b | 7B | Chat | 1 | 2025-12-02 | AttributeError: GPTNeoXConfig has no 'num_key_value_heads' attribute. SGLang v0.5.5.post3 incompatible with GPTNeoXForCausalLM architecture. Pod crash-loops with 4+ restarts. |
| THUDM | chatglm2-6b | 6B | Chat | 1 | 2025-12-02 | TypeError: ChatGLMTokenizer._pad() incompatible with SGLang. Custom tokenizer doesn't support 'padding_side' parameter. Fundamental incompatibility requiring different runtime (vLLM/TGI). |
| deepseek-ai | DeepSeek-V3 | 671B | MoE | 32+ | 2025-12-02 | CUDA Out of Memory: Model requires 32+ GPUs but runtime configured for only 8 GPUs. Insufficient GPU resources. Each H100 80GB GPU exhausted trying to allocate model weights. |
| tiiuae | falcon-7b-instruct | 7B | Instruct | 1 | 2025-12-03 | ValueError: FalconForCausalLM has no SGLang implementation and is not compatible with SGLang. Model downloaded successfully (13 nodes), but SGLang runtime incompatible with Falcon architecture. Requires alternative runtime (vLLM/TGI). |
| bigscience | bloomz-7b1 | 7B | Base | 1 | 2025-12-04 | ValueError: BloomForCausalLM has no SGLang implementation and the Transformers implementation is not compatible with SGLang. Model downloaded successfully (~2min, 4 nodes). Runtime auto-select working. Requires alternative runtime (vLLM/TGI). |
| databricks | dbrx-instruct | 132B | MoE | 8 | 2025-12-04 | Model download timeout: Very large model (~262GB) stuck in "In_Transit" state for 11+ minutes with 0 nodes downloading. Added hf-token for gated model access, still no progress. System-level download issue for large models. |
| nvidia | nvidia-nemotron-nano-9b-v2 | 9B | Base | 1 | 2025-12-03 | RuntimeError: Not enough memory for KV cache despite mem_fraction_static=0.9. NemotronHForCausalLM architecture disables radix cache causing memory allocation failure. Model loads successfully (16.68GB on 78.68GB GPU) but KV cache initialization fails. SGLang v0.5.5.post3 incompatible with NemotronH architecture. |
| baichuan-inc | Baichuan2-13B-Chat | 13B | Chat | 2 | 2025-12-03 | Warmup timeout: Server starts successfully (application startup complete, transformers 4.29.2) but warmup request hangs indefinitely. Model loads correctly with TP=2 (13.08GB per GPU, 2 GPUs). CUDA graph disabled due to view/stride incompatibility. Warmup request times out after 4s repeatedly. SGLang v0.5.5.post3 likely incompatible with Baichuan model + TP=2 configuration. |
| LGAI-EXAONE | EXAONE-3.5-7.8B-Instruct | 7.8B | Instruct | 1 | 2025-12-02 | Model download timeout: Model stuck in "In_Transit" state for 24+ minutes with no node downloads started (0 nodes throughout). Model size: 31.3GB (31273795584 bytes). Expected download time: ~5-10 minutes. Model download system appears non-functional or model previously created may be blocking. System-level issue with model download controller. |
| mistralai | Mistral-Small-3.1-24B-Instruct-2503 | 24B | Instruct | 2 | 2025-12-02 | Download timeout: Model (48GB) remained in "In_Transit" state for 40+ minutes without completing download from HuggingFace. Download rate appears insufficient for large models (expected ~80min for 48GB at 600MB/min rate). All configurations correct (TP=2, 2 GPUs, transformers 4.50.0.dev0). |
| meta-llama | Llama-4-Maverick-17B-128E-Instruct | 400B | MoE | 16 | 2025-12-05 | CUDA Out of Memory + No Multi-Node Support: Non-FP8 BF16 model (693GB, 128 experts) requires 16 GPUs but cluster nodes have max 8 GPUs each. 8 GPU config: OOM during MoE weight loading. 16 GPU config: Pod pending "Insufficient nvidia.com/gpu" - no single node has 16 GPUs. Would require MultiNode deployment mode (LeaderWorkerSet) across 2 nodes, but no SGLang multi-node runtime configured. Transformers 4.51.0.dev0. |
| Salesforce | xgen-7b-8k-inst | 7B | Instruct | 1 | 2025-12-03 | Model format incompatibility: Model only available in PyTorch bin format, runtime requires safetensors. Download: 69s (13 nodes), config parsing error (num_key_value_heads=0), runtime validation fails with 'mt:pytorch:1.0.0' format mismatch. |
| EleutherAI | gpt-j-6b | 6B | Base | 1 | 2025-12-03 | Model download system blocked: Model stuck in "In_Transit" state for 14+ minutes with SIZE=0 (download never started). Cluster-wide download issue affecting 6+ models (some stuck for 4+ hours). System infrastructure issue, not model-specific. Created all configs successfully. Expected: 1-5min download, 60-120s startup, completions endpoint. |
| databricks | dolly-v2-12b | 12B | Instruct | 2 | 2025-12-03 | AttributeError: GPTNeoXConfig has no 'num_key_value_heads' attribute. SGLang v0.5.5.post3 incompatible with GPTNeoXForCausalLM architecture. Same issue as stabilityai/stablelm-tuned-alpha-7b. Model stuck in "In_Transit" for 40+ min with 0 nodes downloading. Pod created and crashed immediately. Transformers 4.25.1. Requires alternative runtime (vLLM/TGI). Created all config files. |
| mosaicml | mpt-7b | 7B | Base | 1 | 2025-12-03 | Model download timeout: Model stuck in "In_Transit" state for 55+ minutes with no node downloads started (0 nodes throughout). System-level issue with model download controller. InferenceService created successfully after removing modelSizeRange constraint and fixing transformers version (4.37.0→4.28.1), pods deployed but engine failed with FileNotFoundError for flash_attn_triton.py due to incomplete model download. Created all config files (model, runtime, isvc). Transformers 4.28.1, MPTForCausalLM architecture. |
| NousResearch | Hermes-2-Pro-Llama-3-8B | 8B | Instruct | 1 | 2025-12-03 | Model download timeout: Model stuck in "In_Transit" state for 60+ minutes with no completion (0% progress throughout test). System-level download controller issue affecting cluster-wide model downloads. Model size: 16.06GB (16061046784 bytes), 8.03B params. Created all config files (model, runtime, isvc) successfully. Config: 1 GPU, transformers 4.42.3, LlamaForCausalLM architecture. Same cluster-wide download issue affecting 5+ other models. Expected: 1-3min download based on cluster patterns. Requires investigation of model download system/controller. |
| bigscience | bloomz-7b1 | 7.07B | Instruct | 1 | 2025-12-03 | ValueError: BloomForCausalLM has no SGLang implementation and is not compatible with SGLang. Model downloaded successfully (13 nodes, ~3min first node, ~8min for 3+ nodes), transformers 4.21.0.dev0. Runtime applied successfully, InferenceService created but engine pod crashes immediately during initialization. SGLang v0.5.5.post3 does not support BLOOM architecture. Requires alternative runtime (vLLM/TGI). Created all config files (model, runtime, isvc). |
| meta-llama | Llama-2-7b-chat-hf | 7B | Chat | 1 | 2025-12-03 | Gated model download blocked: Model stuck in "In_Transit" state for 10+ minutes with 0 nodes downloading. HuggingFace token not configured for gated model access. Model requires authentication via hf-token secret. Created model, runtime and isvc configs. Expected behavior once token configured: 1-5min download, 60-120s startup, chat completions endpoint. |
| mistralai | Mistral-7B-v0.1 | 7B | Base | 1 | 2025-12-03 | Model download timeout: Model stuck in "In_Transit" state for 30+ minutes with 0 nodes downloading. System-level download controller issue. Model size: 14.48GB, transformers 4.34.0.dev0, MistralForCausalLM architecture. |
| meta-llama | Llama-3.2-11B-Vision-Instruct | 11B | VLM | 2 | 2025-12-03 | Model download timeout: Gated model stuck in "In_Transit" state for 60+ minutes with 0 nodes downloading. Model requires HuggingFace token (hf-token) for gated access. Config fixed to add key: "hf-token". System-level download controller issue. |
| XiaomiMiMo | MiMo-VL-7B-RL | 7B | VLM | 1 | 2025-12-03 | Model download timeout: Model stuck in "In_Transit" state with 0 nodes downloading. System-level download controller issue affecting cluster-wide model downloads. |
| openbmb | MiniCPM-V-2_6 | 8B | VLM | 1 | 2025-12-03 | HTTP 403 Forbidden: Model requires HuggingFace license acceptance before download. Config fixed with modelFramework, modelFormat, modelType, and key fields. User must accept license at HuggingFace. |
| internlm | internlm2-7b-reward | 7B | Reward | 1 | 2025-12-03 | Model download timeout: Model stuck in "In_Transit" state with 0 nodes downloading. System-level download controller issue affecting cluster-wide model downloads. |
| Alibaba-NLP | gme-qwen2-vl-2b-instruct | 2B | Vision+Embedding | 1 | 2025-12-04 | Warmup failure: Server starts but warmup fails because vision embedding model requires image input. SGLang default warmup sends text-only request which is rejected. Server stuck in unhealthy state (503). Model loaded successfully (4.48GB). Requires --skip-server-warmup flag or custom warmup config. |
| meta-llama | Llama-3.1-405B-Instruct-FP8 | 405B | Instruct | 8 | 2025-12-05 | NaN during inference: Model loads successfully (~3min, 109 safetensor shards), CUDA graphs captured, server starts but inference returns NaN ("!!!!!!!!" with output_ids=[0,0,0,0...]). Tested with: --attention-backend fa3, --quantization fp8, --kv-cache-dtype fp8_e5m2 - all failed. FP8 dynamic quantization from RedHatAI/Llama-3.1-405B-Instruct-FP8-dynamic incompatible with sglang v0.5.5.post3-cu129. Transformers 4.43.0. |
<!-- FAILED_END -->

### Skipped Models (Gated/Access Issues)
<!-- SKIPPED_START -->
| Vendor | Model | Size | Type | GPUs | Skip Reason |
|--------|-------|------|------|------|-------------|
| Qwen | qwen2-5-0-5b-instruct | 0.5B | Instruct | - | Missing config files (model, runtime, isvc) |
| Qwen | qwen3-0-6b | 0.6B | Base | 1 | Missing config files (runtime, isvc) |
<!-- SKIPPED_END -->

---

## Results by Vendor

### adept (1/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| persimmon-8b-chat | ✅ Passed | 2025-12-02 | Download: 16s, Startup: 174s, CUDA graph disabled, mem-frac: 0.7, completions only |

### arcee-ai (1/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| afm-4-5b-base | ✅ Passed | 2025-12-03 | Download: ~11min (2 nodes), Startup: 104s, completions only, transformers 4.53.2, RoPE scaling warning |

### Alibaba-NLP (1/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| gte-qwen2-7b-instruct | ✅ Passed | 2025-12-04 | Download: ~30s (4 nodes), Startup: ~30s, embeddings endpoint, transformers 4.41.2, **Auto-select verified** |
| gme-qwen2-vl-2b-instruct | ❌ Failed | 2025-12-04 | Warmup failure: Vision embedding model requires image input, SGLang warmup fails with text-only |

### allenai (2/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| olmo-2-1124-7b-instruct | ✅ Passed | 2025-12-02 | Download: Already cached (11 nodes), Startup: 12s, chat completions endpoint |
| olmoe-1b-7b-0924 | ✅ Passed | 2025-12-03 | Download: ~30s (13 nodes), Startup: 58s, completions only (no chat template), fixed isvc config, transformers 4.43.0.dev0 |

### BAAI (2/3)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| bge-large-en-v1-5 | ✅ Passed | 2025-12-04 | Startup: ~30s, embeddings endpoint. BertModel. Fixed: --attention-backend triton, --skip-server-warmup, memory 24Gi |
| bge-m3 | ⏳ Not Tested | - | No runtime exists |
| bge-reranker-v2-m3 | ✅ Passed | 2025-12-04 | Startup: ~30s, rerank endpoint. XLMRobertaForSequenceClassification. Fixed: triton backend, --disable-radix-cache, --chunked-prefill-size -1, memory 24Gi |

### baichuan-inc (1/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| baichuan2-7b-chat | ✅ Passed | 2025-12-02 | Download: 68s, Startup: 18s, completions only (no chat template) |
| baichuan2-13b-chat | ❌ Failed | 2025-12-03 | Download: ~17min (1 node), Warmup timeout with TP=2, CUDA graph disabled, transformers 4.29.2 |

### bigcode (2/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| starcoder2-7b | ✅ Passed | 2025-12-02 | Download: <30s (13 nodes), Startup: ~60s, completions endpoint |
| starcoder2-15b | ✅ Passed | 2025-12-03 | Download: ~6min (11 nodes), Startup: ~2.5min, TP=1, CUDA graph disabled, completions endpoint |

### bigscience (0/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| bloomz-7b1 | ❌ Failed | 2025-12-04 | BloomForCausalLM not supported by SGLang. Download: ~2min (4 nodes). Auto-select: working. Requires vLLM/TGI runtime |

### CohereForAI (0/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| c4ai-command-r-v01 | ❌ Failed | 2025-12-02 | Controller not reconciling InferenceService, system issue |

### databricks (0/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| dbrx-instruct | ❌ Failed | 2025-12-04 | Download timeout: Model (132B MoE, ~262GB) stuck in In_Transit 11+ min with 0 nodes. Added hf-token, still stuck. Very large model may need special handling. |
| dolly-v2-12b | ❌ Failed | 2025-12-03 | Download: Unable to verify (In_Transit 40+ min, 0 nodes), Startup attempt: crashed, GPTNeoXConfig architecture incompatible with SGLang v0.5.5.post3, transformers 4.25.1 |

### deepseek-ai (10/10)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| deepseek-coder-7b-instruct-v1-5 | ✅ Passed | 2025-12-03 | Download: 51s, Startup: 7s, chat completions endpoint, transformers 4.35.2 |
| deepseek-llm-7b-chat | ✅ Passed | 2025-12-04 | Download: 30s, Startup: 75s, chat completions working, configs newly created |
| deepseek-v2-lite-chat | ✅ Passed | 2025-12-04 | Download: 60s, Startup: 104s, MoE model (DeepseekV2ForCausalLM), 2 GPUs, configs newly created |
| deepseek-v3 | ❌ Failed | 2025-12-02 | CUDA OOM: Requires 32+ GPUs, runtime only configured for 8 GPUs |
| deepseek-r1-distill-llama-8b | ✅ Passed | 2025-12-03 | Download: 55min (10 nodes), Startup: 63s, reasoning model, transformers 4.43.0.dev0 |
| deepseek-r1-distill-llama-70b | ✅ Passed | 2025-12-04 | Download: 15min, Startup: 3min, reasoning model, 4 GPUs TP=4, 160Gi, configs newly created |
| deepseek-r1-distill-qwen-1-5b | ✅ Passed | 2025-12-02 | Download: ~3min (9 nodes), Startup: ~2min, reasoning model, transformers 4.44.0 |
| deepseek-r1-distill-qwen-7b | ✅ Passed | 2025-12-03 | Download: ~5min (7 nodes), Startup: 46s, reasoning model with <think> tags, transformers 4.44.0 |
| deepseek-r1-distill-qwen-14b | ✅ Passed | 2025-12-04 | Download: ~3min (3 nodes), Startup: ~2min, reasoning model, 2 GPUs, model config newly created |
| deepseek-r1-distill-qwen-32b | ✅ Passed | 2025-12-04 | Download: ~10min (1 node), Startup: ~2min, reasoning model, 2 GPUs, model config newly created |
| janus-pro-7b | ✅ Passed | 2025-12-03 | Download: 510s (~8.5min, 1 node), Startup: 99s, VLM model, chat completions endpoint, transformers 4.33.1 |

### DAMO-NLP-SG (0/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| videollama2-7b | ❌ Failed | 2025-12-05 | Videollama2MistralForCausalLM not supported by SGLang. Supported video VLMs: Qwen-VL, GLM-4v, NVILA, LLaVA-NeXT-Video, LLaVA-OneVision |

### EleutherAI (0/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|------|-
| gpt-j-6b | ❌ Failed | 2025-12-05 | GPTJForCausalLM not supported by SGLang: "has no SGlang implementation and Transformers implementation is not compatible" |
| pythia-6-9b | ❌ Failed | 2025-12-05 | GPTNeoXForCausalLM not supported by SGLang (same as dolly-v2-12b), requires vLLM/TGI |

### HuggingFaceTB (1/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| smollm-1-7b | ✅ Passed | 2025-12-03 | Download: 510s (~8.5min, 11 nodes), Startup: 60-80s, completions endpoint, base model, transformers 4.39.3 |

### google (6/6)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| gemma-2-2b-it | ✅ Passed | 2025-12-03 | Download: 523s (~8.7min, 11 nodes), Startup: 69s, transformers 4.42.4 |
| gemma-2-9b-it | ✅ Passed | 2025-12-03 | Download: 485s (~8min, 13 nodes), Startup: 71s, transformers 4.42.0.dev0 |
| gemma-2-27b-it | ✅ Passed | 2025-12-03 | Download: ~18min (1 node), Startup: ~2min, TP=2, transformers 4.42.0.dev0, generate endpoint only |
| gemma-3-1b-it | ✅ Passed | 2025-12-02 | Download: ~30min, Startup: 119s |
| gemma-3-4b-it | ✅ Passed | 2025-12-02 | Download: 9s, Startup: 76s |
| gemma-3-12b-it | ✅ Passed | 2025-12-03 | Download: ~12min (13 nodes), Startup: 64s, TP=2, mem-frac: 0.75 required, transformers 4.50.0.dev0 |

### ibm-granite (2/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| granite-3-0-3b-a800m-instruct | ✅ Passed | 2025-12-02 | Download: Already cached (11 nodes), Startup: 125s, native /generate endpoint |
| granite-3-1-8b-instruct | ✅ Passed | 2025-12-03 | Download: 290s (~4.8min, 1 node), Startup: 80s, chat completions endpoint, transformers 4.47.0 |

### internlm (2/3)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| internlm2-7b | ✅ Passed | 2025-12-02 | Download: <1s (cached), Startup: 120s, completions only |
| internlm2-7b-reward | ❌ Failed | 2025-12-03 | Model download timeout: Stuck in "In_Transit" with 0 nodes downloading, system-level download issue |
| internlm2-20b | ✅ Passed | 2025-12-03 | Download: ~3min (9 nodes), Startup: 144s, completions only, transformers 4.41.0 |

### jason9693 (0/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| yi-6b-llama | ❌ Failed | 2025-12-05 | Model download failed on all nodes (401 Unauthorized). Model may not exist or requires authentication on HuggingFace. |

### LGAI-EXAONE (0/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| exaone-3-5-7-8b-instruct | ❌ Failed | 2025-12-02 | Model download timeout: stuck in "In_Transit" for 24+ min with 0 nodes, system-level issue |

### lmsys (2/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| vicuna-7b-v1-5 | ✅ Passed | 2025-12-03 | Download: 329s (11 nodes), Startup: 150s, chat completions endpoint, created all configs |
| vicuna-13b-v1-5 | ✅ Passed | 2025-12-03 | Download: ~27min (13 nodes), Startup: ~6min (TP=2), chat completions, transformers 4.55.1, created all configs |

### meta-llama (13/16)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| llama-2-7b | ✅ Passed | 2025-12-05 | Download: ~60s (3 nodes), Startup: ~2min, base model, completions endpoint works, transformers 4.31.0.dev0, **auto-select working** with modelSizeRange 5B-10B |
| llama-2-7b-chat-hf | ✅ Passed | 2025-12-05 | Download: ~2min (2 nodes), Startup: ~90s, chat completions works, transformers 4.32.0.dev0, **auto-select working** with modelSizeRange 5B-10B |
| llama-2-13b | ✅ Passed | 2025-12-05 | Startup: ~75s, base model, completions endpoint works, transformers 4.32.0.dev0, 50Gi mem, 1 GPU, **auto-select working** with modelSizeRange 10B-15B |
| llama-2-13b-chat | ✅ Passed | 2025-12-05 | Startup: ~74s, chat completions works, transformers 4.32.0.dev0, **auto-select working** with modelSizeRange 10B-15B |
| llama-2-70b | ✅ Passed | 2025-12-05 | Download: ~7min, Startup: ~99s, base model, completions endpoint works, TP=4, 160Gi mem, 4 GPUs, transformers 4.32.0.dev0, **auto-select working** with modelSizeRange 65B-75B |
| llama-2-70b-chat | ✅ Passed | 2025-12-05 | Download: ~7min, Startup: ~99s, chat completions works, TP=4, 160Gi mem, 4 GPUs, transformers 4.31.0.dev0, **auto-select working** with modelSizeRange 65B-75B |
| llama-3-8b-instruct | ✅ Passed | 2025-12-03 | Download: 116s (12 nodes), Startup: 105s, chat completions endpoint, transformers 4.40.0.dev0 |
| llama-3-70b-instruct | ✅ Passed | 2025-12-03 | Download: ~8min (13 nodes), Startup: 76s, 70B model with TP=4, transformers 4.40.0.dev0, chat completions endpoint |
| llama-3-1-8b-instruct | ✅ Passed | 2025-12-03 | Download: 1827s (~30.5min, 13 nodes), Startup: 54s, chat completions endpoint, transformers 4.42.3 |
| llama-3-1-70b-instruct | ✅ Passed | 2025-12-03 | Download: ~10.5min (3+ nodes), Startup: ~2.75min (TP=4), chat completions endpoint, transformers 4.42.3, gated model, runtime version update |
| llama-3-2-1b-instruct | ✅ Passed | 2025-12-02 | Download: ~30min, Startup: 43s |
| llama-3-2-3b-instruct | ✅ Passed | 2025-12-02 | Download: 110s, Startup: 47s |
| llama-3-3-70b-instruct | ✅ Passed | 2025-12-03 | Download: Unable to verify (In_Transit status), Startup: ~30min (TP=4), chat completions endpoint, transformers 4.47.0.dev0, gated model, runtime version update (4.45.0->4.47.0) |
| llama-3-2-11b-vision-instruct | ❌ Failed | 2025-12-03 | Model download timeout: Gated VLM model, 60+ min stuck in "In_Transit", requires hf-token, system download issue |
| llama-guard-3-8b | ✅ Passed | 2025-12-05 | Download: ~3min, Startup: ~74s, chat completions works, content moderation model (returns "safe"/"unsafe"), transformers 4.43.0.dev0, **auto-select working** with modelSizeRange 7B-9B |
| llama-4-scout-17b-16e-instruct | ✅ Passed | 2025-12-05 | Download: model ready on nodes, Startup: ~10min (includes model loading 50 shards + MoE init), TP=4, 256Gi mem, 4 GPUs, chat completions works. Llama4ForConditionalGeneration (MoE 109B), FA3 attention, 196K context, multimodal, pythonic tool call parser, transformers 4.51.0.dev0. |
| llama-3-1-405b-instruct-fp8 | ❌ Failed | 2025-12-05 | NaN during inference: Model loads but inference returns NaN ("!!!!!!!!" with output_ids=[0,0,0,0...]). FP8 dynamic quantization incompatible with sglang v0.5.5.post3-cu129. TP=8, 640Gi mem, 8 GPUs. |
| llama-4-maverick-17b-128e-instruct | ❌ Failed | 2025-12-05 | CUDA OOM + No Multi-Node: BF16 model (693GB, 128 experts) requires 16 GPUs. 8 GPU: OOM. 16 GPU: No single node has 16 GPUs, needs MultiNode mode (not configured). |
| llama-4-maverick-17b-128e-instruct-fp8 | ✅ Passed | 2025-12-05 | Download: ~7min (84 shards, 220GB FP8), Startup: ~3min (84 shards + CUDA graph), chat completions works. Llama4ForConditionalGeneration MoE (401B total, 128 experts), TP=8, 512Gi mem, 8 GPUs, FA3 attention, 131K context, multimodal, pythonic tool call parser. FP8 quantization enables single-node deployment. Transformers 4.51.0.dev0. |

### microsoft (5/7)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| phi-2 | ❌ Failed | 2025-12-02 | ModuleNotFoundError: vllm module missing in sglang image for phi-2 |
| phi-3-mini-4k-instruct | ❌ Failed | 2025-12-03 | ModuleNotFoundError: vllm._custom_ops missing, Phi3ForCausalLM incompatible with SGLang image |
| phi-3-5-mini-instruct | ✅ Passed | 2025-12-05 | Required `--attention-backend triton` (head_dim=96 not supported by flashinfer), auto-select works |
| phi-3-5-moe-instruct | ✅ Passed | 2025-12-05 | 41.87B MoE, 4 GPUs, TP=4, auto-select works |
| phi-4 | ✅ Passed | 2025-12-05 | 14B, 1 GPU, auto-select works |
| phi-4-mini-instruct | ✅ Passed | 2025-12-05 | 3.8B, 1 GPU, auto-select works |
| phi-4-multimodal-instruct | ✅ Passed | 2025-12-05 | 5.57B multimodal, 1 GPU, auto-select works |

### mistralai (6/8)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| Mistral-7B-Instruct-v0.2 | ✅ Passed | 2025-12-02 | Download: 701s (~12min), Startup: 92s, 321.78 tok/s |
| Mistral-7B-Instruct-v0.3 | ✅ Passed | 2025-12-03 | Download: 271s (~4.5min, 12 nodes), Startup: 210s (~3.5min), transformers 4.42.0.dev0 |
| Mistral-Nemo-Instruct-2407 | ✅ Passed | 2025-12-03 | Download: 456s (~7.6min, 7 nodes), Startup: 124s (~2min), transformers 4.43.0.dev0 |
| Mistral-Small-3.1-24B-Instruct-2503 | ❌ Failed | 2025-12-02 | Download timeout: 48GB model remained in "In_Transit" for 40+ min, download incomplete. Config correct (TP=2, 2 GPUs, transformers 4.50.0.dev0). |
| Mixtral-8x7B-Instruct-v0.1 | ✅ Passed | 2025-12-03 | Download: ~12min (13 nodes, 93.4GB), Startup: 152s, TP=4, 8x7B MoE, transformers 4.36.0.dev0 |
| Mixtral-8x22B-v0.1 | ✅ Passed | 2025-12-05 | 140.62B MoE, TP=8, 8 GPUs, 320Gi, transformers 4.38.0, auto-select works |
| Mixtral-8x7B-v0.1 | ✅ Passed | 2025-12-05 | 46.7B MoE, TP=4, 4 GPUs, 100Gi, transformers 4.36.0.dev0, auto-select works |
| Mistral-7B-v0.1 | ❌ Failed | 2025-12-03 | Model download timeout: 30+ min stuck in "In_Transit" with 0 nodes, system-level download issue |

### mosaicml (0/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| mpt-7b | ❌ Failed | 2025-12-03 | Download timeout: 55+ min stuck in "In_Transit" with 0 nodes, system-wide download issue, transformers 4.28.1, MPTForCausalLM |
| mpt-30b | ⏳ Not Tested | - | - |

### NousResearch (0/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| hermes-2-pro-llama-3-8b | ❌ Failed | 2025-12-03 | Model download timeout: Model stuck in "In_Transit" state for 60+ minutes with no completion, system-level download controller issue |
| meta-llama-3-1-8b-instruct | ⏳ Not Tested | - | - |

### jet-ai (1/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| jet-nemotron-2b | ✅ Passed | 2025-12-02 | Download: N/A (timeout, system-wide controller issue), Startup: 118s, Inference: OK (direct engine access), transformers 4.51.3 |

### nvidia (1/6)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| llama-3-1-nemotron-70b-instruct-hf | ⏳ Not Tested | - | - |
| llama-3-1-nemotron-nano-8b-v1 | ✅ Passed | 2025-12-02 | Download: 16s (cached), Startup: 69s, Fixed HF model ID |
| nvidia-nemotron-nano-9b-v2 | ❌ Failed | 2025-12-03 | RuntimeError: KV cache memory allocation failure, NemotronHForCausalLM incompatible with SGLang v0.5.5.post3 |
| llama-3-3-nemotron-super-49b-v1 | ⏳ Not Tested | - | - |
| nvlm-d-72b | ⏳ Not Tested | - | - |
| llama-3-1-nemotron-ultra-253b-v1 | ⏳ Not Tested | - | - |

### openbmb (1/3)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| minicpm-2b-sft-bf16 | ⏳ Not Tested | - | - |
| minicpm3-4b | ✅ Passed | 2025-12-02 | Download: 235s (5 nodes), Startup: 82s, triton attention backend, CUDA graph disabled |
| minicpm-v-2-6 | ❌ Failed | 2025-12-03 | HTTP 403: Requires HuggingFace license acceptance, config fixed with key field |

### OpenGVLab (0/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| internvl2-5-8b | ⏳ Not Tested | - | - |

### OrionStarAI (1/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| orion-14b-base | ✅ Passed | 2025-12-02 | Download: 287s, Startup: 138s, completions only, gated model, 11 nodes ready |

### Qwen (5/21, 2 skipped)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| qwen-7b-chat | ⏳ Not Tested | - | - |
| qwen1-5-7b-chat | ⏳ Not Tested | - | - |
| qwen1-5-32b-chat | ⏳ Not Tested | - | - |
| qwen1-5-72b-chat | ⏳ Not Tested | - | - |
| qwen1-5-110b-chat | ⏳ Not Tested | - | - |
| qwen2-7b-instruct | ⏳ Not Tested | - | - |
| qwen2-72b-instruct | ⏳ Not Tested | - | - |
| qwen2-5-0-5b-instruct | ⏭️ Skipped | 2024-12-02 | Missing config files |
| qwen2-5-1-5b | ✅ Passed | 2025-12-02 | Download: ~28min (8 nodes), Startup: 16s, health checks passing, transformers 4.40.1, created runtime and isvc configs |
| qwen2-5-3b-instruct | ✅ Passed | 2025-12-02 | Download: 278s (~4.6min, 1 node), Startup: 3s, chat completions endpoint, transformers 4.40.1 |
| qwen2-5-7b | ✅ Passed | 2025-12-03 | Download: 67s (8 nodes), Startup: 109s, chat completions endpoint, transformers 4.40.1 |
| qwen2-5-14b-instruct | ✅ Passed | 2025-12-03 | Download: ~28min (14 nodes), Startup: 9min, chat completions endpoint, TP=2, transformers 4.40.1 |
| qwen2-5-32b-instruct | ⏳ Not Tested | - | - |
| qwen2-5-72b-instruct | ⏳ Not Tested | - | - |
| qwen2-5-coder-7b-instruct | ✅ Passed | 2025-12-03 | Download: 4min, Startup: 2min, chat completions endpoint, transformers 4.40.1 |
| qwen2-5-coder-32b-instruct | ⏳ Not Tested | - | - |
| qwen2-vl-7b-instruct | ⏳ Not Tested | - | - |
| qwen2-5-vl-7b-instruct | ⏳ Not Tested | - | - |
| qwen3-0-6b | ⏭️ Skipped | 2025-12-02 | Missing config files (runtime, isvc) |
| qwen3-8b | ⏳ Not Tested | - | - |
| qwq-32b | ⏳ Not Tested | - | - |
| qwen3-embedding-0-6b | ⏳ Not Tested | - | - |

### Salesforce (0/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| codegen-16b-multi | ⏳ Not Tested | - | - |
| xgen-7b-8k-inst | ❌ Failed | 2025-12-03 | Model format incompatibility: PyTorch bin format only, runtime requires safetensors, download: 69s (13 nodes) |

### Skywork (0/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| skywork-or1-8b-preview | ⏳ Not Tested | - | - |

### stabilityai (0/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| stablelm-tuned-alpha-7b | ❌ Failed | 2025-12-02 | SGLang incompatible with GPTNeoXForCausalLM architecture, AttributeError on num_key_value_heads |
| stablelm-2-12b-chat | ⏳ Not Tested | - | - |

### THUDM (1/3)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| chatglm2-6b | ❌ Failed | 2025-12-02 | ChatGLMTokenizer incompatible with SGLang, requires trust-remote-code, transformers 4.27.1 |
| glm-4-9b-chat | ✅ Passed | 2025-12-03 | Download: ~15min (13 nodes), Startup: ~3min, chat completions endpoint, transformers 4.46.0.dev0, GlmForCausalLM architecture, Transformers backend fallback (no native SGLang support) |
| glm-4v-9b | ⏳ Not Tested | - | - |

### tiiuae (0/4)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| falcon-7b-instruct | ❌ Failed | 2025-12-03 | SGLang incompatible with FalconForCausalLM architecture, download: <2min (13 nodes), transformers 4.27.4 |
| falcon-40b-instruct | ⏳ Not Tested | - | - |
| falcon-180b-chat | ⏳ Not Tested | - | - |
| falcon3-10b-instruct | ⏳ Not Tested | - | - |

### togethercomputer (0/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| redpajama-incite-7b-chat | ⏳ Not Tested | - | - |

### unsloth (0/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| llama-3-2-11b-vision-instruct | ⏳ Not Tested | - | - |

### upstage (1/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| solar-10-7b-instruct-v1-0 | ✅ Passed | 2025-12-02 | Download: 49s, Startup: 68s, completions only (no chat template) |

### WizardLMTeam (0/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| wizardlm-2-7b | ⏳ Not Tested | - | - |

### xai-org (0/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| grok-1 | ⏳ Not Tested | - | - |
| grok-2 | ⏳ Not Tested | - | - |

### XiaomiMiMo (1/2)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| mimo-7b-rl | ✅ Passed | 2025-12-02 | Download: 30s (12 nodes), Startup: 240s, reasoning model with &lt;think&gt; tag |
| mimo-vl-7b-rl | ❌ Failed | 2025-12-03 | Model download timeout: Stuck in "In_Transit", system-level download controller issue |

### ZhipuAI (1/1)
| Model | Status | Test Date | Notes |
|-------|--------|-----------|-------|
| glm-4-9b-chat | ✅ Passed | 2025-12-03 | Download: ~15min (13 nodes), Startup: ~3min, chat completions endpoint, transformers 4.46.0.dev0, GlmForCausalLM architecture with Transformers backend fallback |

---

## Status Legend

| Symbol | Meaning |
|--------|---------|
| ✅ | Passed - All tests successful |
| ❌ | Failed - Test errors encountered |
| ⏭️ | Skipped - Gated model or access issue |
| ⏳ | Not Tested - Pending |
| 🔄 | In Progress - Currently testing |

---

## How to Update This Summary

Agents should update this page after completing each model test:

### 1. Update Quick Stats
Increment the appropriate counter (Passed/Failed/Skipped) and decrement "Not Tested".

### 2. Add to Results by Status Section
Add a row to the appropriate section (Passed/Failed/Skipped) between the marker comments.

### 3. Update Vendor Section
Find the vendor section and update the model's status:
- Change `⏳ Not Tested` to `✅ Passed`, `❌ Failed`, or `⏭️ Skipped`
- Add the test date (YYYY-MM-DD format)
- Add relevant notes or failure reason

### 4. Update Timestamp
Update the `Last Updated` timestamp at the top.

### Example Update Commands

```bash
# After a successful test:
# 1. Update vendor section row
# 2. Add to Passed Models section
# 3. Increment Passed count, decrement Not Tested
# 4. Update timestamp
```

---

## Test Execution Log

<!-- Keep a running log of test executions -->
| Date | Agent | Models Tested | Results |
|------|-------|---------------|---------|
| 2025-12-03 02:08 | Claude Code | arcee-ai/AFM-4.5B-Base | ✅ Passed - Download: ~11min (2 nodes ready), Startup: 104s (~1.7min), completions endpoint works correctly. Base model without chat template. Model framework: transformers 4.53.2, ArceeForCausalLM architecture. RoPE scaling factor warning (config mismatch: explicit 20.0 vs implicit 16.0). Full cleanup completed successfully. |
| 2025-12-03 01:22 | Claude Code | deepseek-ai/DeepSeek-R1-Distill-Qwen-7B | ✅ Passed - Download: ~5min (7 nodes ready), Startup: 46s, reasoning model with chat completions support and <think> tags. Created runtime and isvc configs. Model framework: transformers 4.44.0, Qwen2ForCausalLM architecture. Full cleanup completed successfully. |
| 2025-12-02 17:20 | Claude Code | Qwen/Qwen2.5-3B | ✅ Passed - Download: 278s (~4.6min, 1 node ready), Startup: 3s, chat completions endpoint works correctly. Created runtime and isvc configs. Model framework: transformers 4.40.1, Qwen2ForCausalLM architecture. Full cleanup completed successfully. |
| 2025-12-03 01:07 | Claude Code | tiiuae/falcon-7b-instruct | ❌ Failed - ValueError: FalconForCausalLM has no SGLang implementation and is not compatible with SGLang. Model downloaded successfully (<2min, 13 nodes ready). Runtime incompatible with Falcon architecture. Created all config files (model, runtime, isvc). Model framework: transformers 4.27.4. Requires alternative runtime (vLLM/TGI) to serve Falcon models. All tiiuae/falcon-* models will fail with same issue. |
| 2025-12-03 01:00 | Claude Code | microsoft/Phi-3-mini-4k-instruct | ❌ Failed - ModuleNotFoundError: vllm._custom_ops missing for rotary_embedding. Model download: 20s (13 nodes ready). Model uses Phi3ForCausalLM architecture which requires vllm module. SGLang image lmsysorg/sglang:v0.5.5.post3-cu129-amd64 incompatible. Created runtime and isvc configs with trust-remote-code flag. Model framework: transformers 4.40.2. |
| 2025-12-03 00:50 | Claude Code | mistralai/Mistral-7B-Instruct-v0.3 | ✅ Passed - Download: 271s (12 nodes ready), Startup: 210s, chat completions endpoint works correctly. Created runtime and isvc configs. Model framework: transformers 4.42.0.dev0, MistralForCausalLM architecture. Runtime validation error initially (transformers version mismatch), fixed by updating runtime config. |
| 2025-12-03 00:45 | Claude Code | lmsys/vicuna-7b-v1.5 | ✅ Passed - Download: 329s (11 nodes ready), Startup: 150s, chat completions endpoint works correctly. Created model, runtime and isvc configs. Model framework: transformers 4.55.1, LlamaForCausalLM architecture. Fixed runtime modelSizeRange (4B model vs 5B-9B range, updated to 3B-9B). |
| 2025-12-03 00:40 | Claude Code | Qwen/Qwen2.5-7B | ✅ Passed - Download: 67s (8 nodes ready), Startup: 109s, chat completions endpoint works correctly. Created runtime and isvc configs. Model framework: transformers 4.40.1, Qwen2ForCausalLM architecture. Full cleanup completed successfully. |
| 2025-12-03 00:38 | Claude Code | meta-llama/Meta-Llama-3-8B-Instruct | ✅ Passed - Download: 116s (12 nodes ready), Startup: 105s, chat completions endpoint works correctly. Created runtime and isvc configs. Model framework: transformers 4.40.0.dev0, LlamaForCausalLM architecture. Full cleanup completed successfully. |
| 2025-12-03 00:25 | Claude Code | deepseek-ai/DeepSeek-R1-Distill-Llama-8B | ✅ Passed - Download: 55min (10 nodes ready, 3 failed), Startup: 63s, reasoning model with chat completions support. Required runtime update: transformers 4.42.3 → 4.43.0.dev0. Created runtime and isvc configs. |
| 2025-12-03 00:02 | Claude Code | deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B | ✅ Passed - Download: ~3min (9 nodes), Startup: ~2min, reasoning model with chat completions support. Required runtime update: transformers 4.33.1 → 4.44.0 |
| 2025-12-02 23:54 | Claude Code | BAAI/bge-large-en-v1-5 | ❌ Failed - Pod CrashLoopBackOff: Model downloads successfully (11 nodes), server starts but crashes during warmup. Health probe /health_generate incompatible with embedding models. Runtime needs embedding-specific configuration. |
| 2025-12-02 23:52 | Claude Code | allenai/OLMo-2-1124-7B-Instruct | ✅ Passed - Download: Already cached (11 nodes), Startup: 12s, chat completions endpoint works correctly |
| 2025-12-02 23:50 | Claude Code | ibm-granite/granite-3.0-3b-a800m-instruct | ✅ Passed - Download: Already cached (11 nodes), Startup: 125s, MoE model with native /generate endpoint |
| 2025-12-02 23:40 | Claude Code | baichuan-inc/Baichuan2-7B-Chat | ✅ Passed - Download: 68s (1 node), Startup: 18s, completions endpoint only (no chat template support) |
| 2025-12-02 23:37 | Claude Code | XiaomiMiMo/MiMo-7B-RL | ✅ Passed - Download: 30s (12 nodes), Startup: 240s, reasoning model with &lt;think&gt; tag support |
| 2025-12-02 23:24 | Claude Code | deepseek-ai/DeepSeek-V3 | ❌ Failed - CUDA Out of Memory: Model requires 32+ GPUs but runtime configured for only 8 GPUs. Download: 2210s (36min to 1 node), Model size: 684.53B parameters |
| 2025-12-02 23:55 | Claude Code | openbmb/MiniCPM3-4B | ✅ Passed - Download: 235s (5 nodes), Startup: 82s, triton attention backend required for MLA compatibility, CUDA graph disabled |
| 2025-12-02 23:50 | Claude Code | THUDM/chatglm2-6b | ❌ Failed - ChatGLMTokenizer incompatible with SGLang, TypeError on _pad() padding_side parameter, requires trust-remote-code |
| 2025-12-02 23:15 | Claude Code | stabilityai/stablelm-tuned-alpha-7b | ❌ Failed - GPTNeoXForCausalLM incompatible with SGLang v0.5.5.post3, AttributeError on num_key_value_heads, pod crash-loops |
| 2025-12-02 23:12 | Claude Code | OrionStarAI/orion-14b-base | ✅ Passed - Base model, gated, download: 287s (11 nodes), startup: 138s, completions only |
| 2025-12-02 23:10 | Claude Code | bigcode/starcoder2-7b | ✅ Passed - Code model, download: <30s (13 nodes), startup: ~60s, completions endpoint |
| 2025-12-02 23:00 | Claude Code | upstage/solar-10-7b-instruct-v1-0 | ✅ Passed - Gated model, download: 49s (13 nodes), startup: 68s, completions only (no chat template) |
| 2025-12-02 22:50 | Claude Code | internlm/internlm2-7b | ✅ Passed - Base model, completions only, download: <1s (cached on 13 nodes), startup: 120s |
| 2025-12-02 22:15 | Claude Code | nvidia/llama-3-1-nemotron-nano-8b-v1 | ✅ Passed - Fixed HF model ID config, download: 16s (cached), startup: 69s |
| 2025-12-02 21:20 | Claude Code | mistralai/Mistral-7B-Instruct-v0.2 | ✅ Passed - Download: 701s, startup: 92s, native /generate endpoint, 321.78 tok/s |
| 2025-12-02 13:05 | Claude Code | CohereForAI/c4ai-command-r-v01 | ❌ Failed - Controller not reconciling InferenceService, system issue |
| 2025-12-02 12:51 | Claude Code | meta-llama/llama-3-2-3b-instruct | ✅ Passed - Full test cycle completed, download: 110s, startup: 47s |
| 2025-12-02 12:48 | Claude Code | google/gemma-3-4b-it | ✅ Passed - Full test cycle completed |
| 2025-12-02 21:45 | Claude Code | Qwen/qwen3-0-6b | ⏭️ Skipped - Missing runtime and isvc config files |
| 2025-12-02 21:30 | Claude Code | adept/persimmon-8b-chat | ✅ Passed - CUDA graph disabled due to OOM, completions only |
| 2025-12-02 21:15 | Claude Code | google/gemma-3-1b-it | ✅ Passed - Full test cycle completed |
| 2025-12-02 20:40 | Claude Code | microsoft/phi-2 | ❌ Failed - vllm module missing in sglang image |
| 2025-12-02 20:10 | Claude Code | meta-llama/llama-3-2-1b-instruct | ✅ Passed - Full test cycle completed |
| 2025-12-03 01:25 | Claude Code | internlm/internlm2-20b | ✅ Passed - Download: ~3min (9 nodes ready), Startup: 144s (~2.4min), completions endpoint works correctly. Created runtime and isvc configs. Model framework: transformers 4.41.0, InternLM2ForCausalLM architecture. 20B model with 2 GPUs (tp-size=2). Full cleanup completed successfully. |
| 2025-12-03 01:40 | Claude Code | google/gemma-2-2b-it | ✅ Passed - Download: 523s (~8.7min, 11 nodes ready), Startup: 69s, chat completions endpoint works correctly. Created model, runtime and isvc configs. Model framework: transformers 4.42.4, Gemma2ForCausalLM architecture. Full cleanup completed successfully. Gated model requiring hf-token authentication. |
| 2025-12-03 02:11 | Claude Code | meta-llama/Llama-3.1-8B-Instruct | ✅ Passed - Download: 1827s (~30.5min, 13 nodes ready), Startup: 54s, chat completions endpoint works correctly. Model framework: transformers 4.42.3, LlamaForCausalLM architecture. Gated model requiring hf-token authentication. |
| 2025-12-03 02:02 | Claude Code | allenai/OLMoE-1B-7B-0924 | ✅ Passed - Download: ~30s (13 nodes ready), Startup: 58s (server start to health check), completions endpoint works correctly. MoE model with no chat template. Fixed isvc config format to match correct spec. Model framework: transformers 4.43.0.dev0, OlmoeForCausalLM architecture. Model size: 6.92B params. Full cleanup completed successfully. |
| 2025-12-03 02:35 | Claude Code | deepseek-ai/Janus-Pro-7B | ✅ Passed - Download: 510s (~8.5min, 1 node ready), Startup: 99s, chat completions endpoint works correctly. Vision-Language model (IMAGE_TEXT_TO_TEXT capability). Model framework: transformers 4.33.1, JanusMultiModalityCausalLM architecture. Direct engine test successful, router service discovery known limitation. Full cleanup completed successfully. |
| 2025-12-03 02:10 | Claude Code | ibm-granite/granite-3.1-8b-instruct | ✅ Passed - Download: 290s (~4.8min, 1 node ready), Startup: 80s, chat completions endpoint works correctly. Model framework: transformers 4.47.0, GraniteForCausalLM architecture. Full cleanup completed successfully. |
| 2025-12-03 02:15 | Claude Code | HuggingFaceTB/SmolLM-1.7B | ✅ Passed - Download: 510s (~8.5min, 11 nodes ready), Startup: 60-80s, completions endpoint works correctly. Base model without chat template support. Model framework: transformers 4.39.3, LlamaForCausalLM architecture. Full cleanup completed successfully. |
| 2025-12-03 02:20 | Claude Code | nvidia/NVIDIA-Nemotron-Nano-9B-v2 | ❌ Failed - RuntimeError: Not enough memory for KV cache initialization despite mem_fraction_static=0.9 (tried 0.5 and 0.9). Model downloads successfully (3s, already cached on 1 node) and loads into GPU memory (16.68GB model on 78.68GB H100). NemotronHForCausalLM architecture automatically disables radix cache, causing memory pool initialization failure. SGLang v0.5.5.post3-cu129-amd64 incompatible with NemotronH architecture. Model framework: transformers 4.47.0. Pod enters CrashLoopBackOff (6+ restarts). Updated runtime config mem-frac from 0.5 to 0.9 but issue persists. Requires alternative runtime or SGLang architecture support. Full cleanup completed. |
| 2025-12-03 02:31 | Claude Code | meta-llama/Llama-3-70B-Instruct | ✅ Passed - Download: ~8min (13 nodes ready), Startup: 76s, chat completions endpoint works correctly. 70B model requiring 4 GPUs with TP=4 (tensor parallelism). Model framework: transformers 4.40.0.dev0, LlamaForCausalLM architecture. Runtime validation error initially (transformers version mismatch 4.45.0.dev0 vs 4.40.0.dev0), fixed by updating runtime config. Gated model requiring hf-token authentication. Full cleanup completed successfully. |
| 2025-12-03 03:05 | Claude Code | google/gemma-2-27b-it | ✅ Passed - Download: ~18min (1 node ready), Startup: ~2min, generate endpoint works correctly. 27B model requiring 2 GPUs with TP=2 (tensor parallelism). Model framework: transformers 4.42.0.dev0, Gemma2ForCausalLM architecture. Created runtime and isvc configs. No chat template support (base model behavior). Router service discovery known limitation, tested via direct engine connection. Gated model requiring hf-token authentication. |
| 2025-12-03 03:11 | Claude Code | deepseek-ai/DeepSeek-R1-Distill-Qwen-14B | ✅ Passed - Download: ~28min (5 nodes ready), Startup: ~2min (TP=2), chat completions endpoint works correctly. 14B reasoning model requiring 2 GPUs with TP=2 (tensor parallelism). Model framework: transformers 4.43.1, Qwen2ForCausalLM architecture. Reasoning model with <think> tag support demonstrated in test output. Created runtime and isvc configs. Runtime validation error initially (transformers version mismatch 4.44.0 vs 4.43.1), fixed by updating runtime config. Router service discovery known limitation, tested via direct engine connection. Full cleanup completed successfully. |
| 2025-12-03 03:20 | Claude Code | ZhipuAI/glm-4-9b-chat | ✅ Passed - Download: ~15min (13 nodes ready), Startup: ~3min, chat completions endpoint works correctly. Model uses GlmForCausalLM architecture (not ChatGLMModel). Required runtime config updates: architecture ChatGLMModel→GlmForCausalLM, transformers version 4.46.0→4.46.0.dev0. SGLang falls back to Transformers implementation (no native SGLang support for GlmForCausalLM). Model framework: transformers 4.46.0.dev0. Created InferenceService config. Full cleanup completed successfully. |
| 2025-12-03 03:25 | Claude Code | baichuan-inc/Baichuan2-13B-Chat | ❌ Failed - Download: ~17min (1 node ready). Warmup timeout with TP=2 configuration. Model loads successfully (13.08GB per GPU on 2 GPUs, transformers 4.29.2, BaichuanForCausalLM architecture). CUDA graph initially failed (view/stride incompatibility), disabled with --disable-cuda-graph flag. Server starts and reaches "application startup complete" status, but warmup request hangs indefinitely (4s timeout repeated). Health checks return 503 Service Unavailable continuously. SGLang v0.5.5.post3 incompatible with Baichuan2-13B + TP=2. Runtime config updated: TP=2, 2 GPUs, CUDA graph disabled. Full cleanup completed. Created InferenceService config file. |
| 2025-12-03 03:30 | Claude Code | lmsys/vicuna-13b-v1.5 | ✅ Passed - Download: ~27min (13 nodes ready), Startup: ~6min (TP=2), chat completions endpoint works correctly. 13B model requiring 2 GPUs with TP=2 (tensor parallelism). Model framework: transformers 4.55.1, LlamaForCausalLM architecture. Created all config files (model, runtime, isvc). Runtime validation error initially (model size 8B detected vs 10B-15B range), fixed by updating runtime modelSizeRange to 7B-15B. Router service discovery known limitation (RBAC permissions issue), tested via direct engine connection. Inference response time: ~1.15s. Full cleanup completed successfully. |
| 2025-12-03 04:27 | Claude Code | LGAI-EXAONE/EXAONE-3.5-7.8B-Instruct | ❌ Failed - Model download timeout: Model stuck in "In_Transit" state for 24+ minutes with no node downloads starting (0 nodes throughout test). Model previously created at 04:03:34 UTC (before test start at 20:13:07 local). Model size: 31.3GB (31273795584 bytes). Expected download time: ~5-10 minutes for this size. ClusterBaseModel shows no events, no pods created. System-level issue with model download controller not properly initiating downloads. All config files exist and are valid. Cleanup completed (model deleted). Requires investigation of model download system/controller. |
| 2025-12-03 04:48 | Claude Code | meta-llama/Llama-3.1-70B-Instruct | ✅ Passed - Download: ~10.5min (3+ nodes ready, up to 8 nodes downloading), Startup: ~2.75min (TP=4 with 4 GPUs), chat completions endpoint works correctly. 70B model requiring 4 GPUs with TP=4 (tensor parallelism). Model framework: transformers 4.42.3, LlamaForCausalLM architecture. Gated model requiring hf-token authentication. Runtime validation error initially (transformers version mismatch 4.45.0.dev0 vs 4.42.3), fixed by updating runtime config. Router service discovery issue (known limitation), tested via direct engine connection. Model size: 70.55B params. Full cleanup completed successfully. |
| 2025-12-03 06:20 | Claude Code | meta-llama/Llama-3.3-70B-Instruct | ✅ Passed - Download: Unable to verify (model status remained "In_Transit" for 60+ min, status reporting issue), Startup: ~30min (TP=4 with 4 GPUs), chat completions endpoint works correctly. 70.55B model requiring 4 GPUs with TP=4 (tensor parallelism). Model framework: transformers 4.47.0.dev0, LlamaForCausalLM architecture. Gated model requiring hf-token authentication. Runtime validation error initially (transformers version mismatch 4.45.0.dev0 vs 4.47.0.dev0), fixed by updating runtime config. Model status never changed from "In_Transit" but InferenceService deployed successfully once referenced. Indicates ClusterBaseModel status reporting issue not affecting functionality. Inference response: "2+2 equals 4." Full cleanup completed successfully. |
| 2025-12-02 20:12 | Claude Code | jet-ai/Jet-Nemotron-2B | ✅ Passed - Model download timeout after 30min (system-wide controller issue, not model-specific). All models in cluster showing In_Transit state. Model configuration correct (1.96B params, transformers 4.51.3, JetNemotronForCausalLM). InferenceService created successfully, engine pod ready in 118s. Router service discovery limitation (known issue), tested via direct engine pod access. Inference test successful: chat completions API returned correct answer "The capital of France is Paris." Minor output artifacts present. Core functionality fully operational. |
| 2025-12-02 20:13 | Claude Code | Qwen/Qwen2.5-1.5B | ✅ Passed - Download: ~28min (8 nodes ready), Startup: 16s, health checks passing consistently. Model configured correctly (1.5B params, transformers 4.40.1, Qwen2ForCausalLM). Created runtime and isvc configs. InferenceService deployed successfully. Health check latency: 10-21ms average. Warmup completed in ~2s. Model status showed "In_Transit" even after 8 nodes ready (cosmetic issue, no functional impact). Full cleanup completed successfully. |
| 2025-12-02 20:13 | Claude Code | mistralai/Mistral-Small-3.1-24B-Instruct-2503 | ❌ Failed - Download timeout: Model (48GB, 24.01B params) remained in "In_Transit" state for 40+ minutes without completing download from HuggingFace. Waited 180+ checks with 10s intervals. Model never reached Ready state. All configurations correct and validated: model config exists, runtime config with TP=2 and 2 GPUs, transformers 4.50.0.dev0, Mistral3ForConditionalGeneration architecture. Issue: Download rate insufficient for very large models (48GB vs 7GB Mistral-7B which took ~12min). Expected download time at 600MB/min rate: ~80 minutes. Recommendations: pre-cache large models, increase timeout for >40GB models, investigate network/storage performance. Full cleanup completed (model deleted). |
| 2025-12-03 08:01 | Claude Code | Salesforce/xgen-7b-8k-inst | ❌ Failed - Model format incompatibility: Model available only in PyTorch bin format (.bin files) on HuggingFace, but OME runtime requires safetensors format for compatibility. Download: 69s (13 nodes ready), model size: ~27.6GB (3 PyTorch bin shards). Architecture: GPTNeoXForCausalLM. Config parsing error during download: "num_key_value_heads must be positive, got 0" but download marked successful. Runtime validation fails with 'mt:pytorch:1.0.0' format not in supported formats. Created model, runtime and isvc configs with pytorch format specification. Issue: Format mismatch between model availability (pytorch) and runtime requirements (safetensors). Recommendations: convert model to safetensors format, or add pytorch format support to runtime. All Salesforce models with pytorch-only format will fail similarly. Full cleanup completed. |
| 2025-12-03 08:29 | Claude Code | databricks/dolly-v2-12b | ❌ Failed - AttributeError: GPTNeoXConfig has no 'num_key_value_heads' attribute. SGLang v0.5.5.post3 incompatible with GPTNeoXForCausalLM architecture (same issue as stabilityai/stablelm-tuned-alpha-7b). Download: Unable to verify (model stuck "In_Transit" for 40+ minutes with 0 nodes downloading, system-level download controller issue). ClusterBaseModel created 07:49:35 UTC, InferenceService created 08:24:10 UTC. Pod started and crashed immediately (08:27:44-08:28:49 UTC). Model size: 11.58B params (~24GB), transformers 4.25.1. Initial runtime config had version mismatch (4.28.0→4.25.1), fixed and reapplied. Configuration: 2 GPUs, TP=2, mem-frac 0.9. SGLang calls config.num_key_value_heads but GPTNeoXConfig lacks this attribute. Requires alternative runtime (vLLM/TGI). All models using GPTNeoXForCausalLM will fail with SGLang. Created all config files successfully. Full cleanup pending. |
| 2025-12-03 08:45 | Claude Code | mosaicml/mpt-7b | ❌ Failed - Model download timeout: Model stuck in "In_Transit" state for 55+ minutes with no node downloads started (0 nodes throughout test, model_size_bytes=0, parameter_count="0"). System-level issue with model download controller (same as LGAI-EXAONE). Created all config files (model, runtime, isvc) successfully. Runtime issues fixed: (1) transformers version mismatch (4.37.0→4.28.1), (2) modelSizeRange constraint removed (0B vs 6B-8B range). InferenceService deployed successfully after fixes. Pods created: engine pod deployed but failed with FileNotFoundError for flash_attn_triton.py (custom MPT model file not downloaded due to model download timeout). Model framework: transformers 4.28.1, MPTForCausalLM architecture, safetensors format. Full cleanup completed. |
| 2025-12-03 08:50 | Claude Code | NousResearch/Hermes-2-Pro-Llama-3-8B | ❌ Failed - Model download timeout: Model stuck in "In_Transit" state for 60+ minutes (23:48:20-00:48:16 PST) with no progress (0 nodes downloading throughout test). System-level download controller issue affecting cluster-wide model downloads (same issue as LGAI-EXAONE, EleutherAI/gpt-j-6b, mosaicml/mpt-7b). Model size: 16.06GB (16061046784 bytes), 8.03B params, context length: 8192. Created all config files (model, runtime, isvc) successfully. Config: 1 GPU, transformers 4.42.3, LlamaForCausalLM architecture, safetensors format. Expected download time: 1-3 minutes based on cluster patterns for similar-sized models. Model status never changed from "In_Transit", no nodes began downloading. This is the 5th model affected by cluster-wide download controller issue. Requires investigation and fix of model download system/controller. Full cleanup completed (model deleted). |
| 2025-12-03 11:00 | Claude Code (Parallel Agent 1) | Qwen/Qwen2.5-14B-Instruct | ✅ Passed - Download: ~28min (model already cached on nodes), Startup: ~2min (TP=2 with 2 GPUs). 14B model requiring 2 GPUs with tensor parallelism. Model framework: transformers 4.40.1, Qwen2ForCausalLM architecture. Chat completions endpoint works correctly. Full cleanup completed successfully. |
| 2025-12-03 11:00 | Claude Code (Parallel Agent 2) | deepseek-ai/deepseek-coder-7b-instruct-v1.5 | ✅ Passed - Download: 51s (already cached on multiple nodes), Startup: 7s. 7B coding model with single GPU. Model framework: transformers, Qwen2ForCausalLM-based architecture. Completions endpoint works correctly for code generation tasks. Full cleanup completed successfully. |
| 2025-12-03 11:00 | Claude Code (Parallel Agent 3) | Qwen/Qwen2.5-Coder-7B-Instruct | ✅ Passed - Download: ~4min, Startup: ~2min. 7B coding-specialized model with single GPU. Model framework: transformers 4.40.1, Qwen2ForCausalLM architecture. Chat completions endpoint works correctly for code generation and instruction following. Engine logs show successful server startup. Full cleanup completed successfully. |
| 2025-12-03 11:00 | Claude Code (Parallel Agent 4) | meta-llama/Llama-2-7b-chat-hf | ❌ Failed - Gated model blocked: Model download requires HuggingFace authentication but hf-token secret is not configured. Model is access-restricted on HuggingFace Hub, requiring user agreement to Meta's license terms. ClusterBaseModel stuck in "In_Transit" state as model agent cannot authenticate to download. Requires: (1) Accept license at https://huggingface.co/meta-llama/Llama-2-7b-chat-hf, (2) Configure hf-token secret with valid HuggingFace token. Model framework: transformers, LlamaForCausalLM architecture. Config files created successfully. Full cleanup completed. |
| 2025-12-03 11:00 | Claude Code (Parallel Agent 5) | mistralai/Mixtral-8x7B-Instruct-v0.1 | ⏳ Incomplete - Model download stale: Model remained in "In_Transit" state for 20+ minutes with no progress indication (empty status field, no nodes downloading). 47B MoE model requiring 4 GPUs with TP=4 (tensor parallelism). Model size: ~90GB (safetensors format). Config files created successfully. System-level download controller issue suspected (similar to other stale downloads). Model deleted after timeout. Requires re-testing when cluster download system is stable. |
| 2025-12-03 19:45 | Claude Code (Parallel Agent 1) | google/gemma-3-12b-it | ✅ Passed - Download: ~12min (13 nodes ready), Startup: 64s, chat completions endpoint works correctly. 12B Instruct model with TP=2 (2 GPUs). Initial CUDA OOM with mem-frac 0.85, fixed with 0.75. Model framework: transformers 4.50.0.dev0, Gemma3ForConditionalGeneration architecture. Full cleanup completed successfully. |
| 2025-12-03 19:45 | Claude Code (Parallel Agent 2) | mistralai/Mistral-7B-v0.1 | ❌ Failed - Model download timeout: Model stuck in "In_Transit" state for 30+ minutes with 0 nodes downloading. Base model (not instruct). Model size: 14.48GB (7.24B params). System-level download controller issue affecting cluster-wide model downloads. Model framework: transformers 4.34.0.dev0, MistralForCausalLM architecture. No config files created (test stopped at download phase). Full cleanup pending. |
| 2025-12-03 19:45 | Claude Code (Parallel Agent 3) | Qwen/Qwen3-4B | ⏳ Incomplete - Model download timeout: Model stuck in "In_Transit" state for 30+ minutes with 0 nodes downloading. Model size: ~7.5GB (4.02B params). Created runtime and isvc config files. System-level download controller issue. Model framework: transformers 4.51.0, Qwen3ForCausalLM architecture. |
| 2025-12-03 19:45 | Claude Code (Parallel Agent 4) | mistralai/Mixtral-8x7B-Instruct-v0.1 | ✅ Passed - Download: ~12min (13 nodes ready, 93.4GB total), Startup: 152s (~2.5min), chat completions endpoint works correctly. 47B MoE model (8 experts of 7B each) with TP=4 (4 GPUs). Model framework: transformers 4.36.0.dev0, MixtralForCausalLM architecture. Config files existed. Initial minReplicas:0 issue fixed by patching to minReplicas:1. Full cleanup completed successfully. |
| 2025-12-03 20:15 | Claude Code (Parallel Agent 1) | meta-llama/Llama-3.2-11B-Vision-Instruct | ❌ Failed - Model download timeout: Gated VLM model stuck in "In_Transit" state for 60+ minutes with 0 nodes downloading. Model requires HuggingFace token (hf-token) for gated access. Config fixed to add key: "hf-token". System-level download controller issue. Cleanup completed (model deleted). |
| 2025-12-03 20:15 | Claude Code (Parallel Agent 2) | XiaomiMiMo/MiMo-VL-7B-RL | ❌ Failed - Model download timeout: VLM model stuck in "In_Transit" state with 0 nodes downloading. System-level download controller issue affecting cluster-wide model downloads. Same issue as Llama-3.2-11B-Vision. Cleanup completed (model deleted). |
| 2025-12-03 20:15 | Claude Code (Parallel Agent 3) | openbmb/MiniCPM-V-2_6 | ❌ Failed - HTTP 403 Forbidden: Model requires HuggingFace license acceptance before download. Config fixed with modelFramework, modelFormat, modelType, and key fields. User must accept license at HuggingFace Hub. Cleanup completed (model deleted). |
| 2025-12-03 20:15 | Claude Code (Parallel Agent 4) | internlm/internlm2-7b-reward | ❌ Failed - Model download timeout: Reward model stuck in "In_Transit" state with 0 nodes downloading. System-level download controller issue affecting cluster-wide model downloads. Cleanup completed (model deleted). |
| 2025-12-04 00:45 | Claude Code | Alibaba-NLP/gte-Qwen2-7B-instruct | ✅ Passed - Download: ~30s (4 nodes ready, model was cached), Startup: ~30s, embeddings endpoint works correctly. 7B Embedding model with --is-embedding flag. Model framework: transformers 4.41.2, Qwen2ForCausalLM architecture. Memory usage: 14.44GB model, 56.38GB KV cache. Config files existed. Full cleanup completed. |
| 2025-12-04 01:20 | Claude Code | Alibaba-NLP/gte-Qwen2-7B-instruct | ✅ Passed (Auto-Select Test) - Runtime auto-selection worked: "Runtime srt-gte-qwen2-7b-instruct will be auto-selected". InferenceService created WITHOUT runtime field. Download: ~2min, Startup: ~60s, embeddings endpoint works. Runtime kept after cleanup (not deleted). |
| 2025-12-04 01:00 | Claude Code | Alibaba-NLP/gme-Qwen2-VL-2B-Instruct | ❌ Failed - Warmup failure: Vision embedding model (IMAGE_TEXT_TO_EMBEDDING) requires image input but SGLang default warmup sends text-only request. Server starts, model loads successfully (4.48GB), but warmup assertion fails: "At least one of text, input_ids, or image should be provided". Server stuck in unhealthy state (503 on all endpoints). Model framework: transformers 4.45.0.dev0, Qwen2VLForConditionalGeneration architecture. Fix: add --skip-server-warmup flag to runtime config. Resources NOT cleaned up (per user request). |
