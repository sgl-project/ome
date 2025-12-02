# Supported Models Reference

This document provides a comprehensive reference of all models supported by SGLang and vLLM, their configurations, access requirements, and specifications.

> **Sources**:
> - [SGLang Documentation](https://docs.sglang.io/supported_models/)
> - [vLLM Documentation](https://docs.vllm.ai/en/latest/models/supported_models/)

## Table of Contents

- [Generative Models (Text-to-Text)](#generative-models-text-to-text)
- [Multimodal Language Models](#multimodal-language-models)
- [Embedding Models](#embedding-models)
- [Reward Models](#reward-models)
- [Rerank Models](#rerank-models)
- [vLLM Additional Models](#vllm-additional-models)
- [Model Status in OME](#model-status-in-ome)

---

## Generative Models (Text-to-Text)

### Meta Llama Family

| Model                     | HuggingFace ID                                  | Parameters        | Architecture         | Size   | Context | Token Required | OME Status       |
|---------------------------|-------------------------------------------------|-------------------|----------------------|--------|---------|----------------|------------------|
| Llama 2 7B                | `meta-llama/Llama-2-7b-hf`                      | 7B                | LlamaForCausalLM     | 13 GB  | 4K      | Yes            | Missing          |
| Llama 2 13B               | `meta-llama/Llama-2-13b-hf`                     | 13B               | LlamaForCausalLM     | 26 GB  | 4K      | Yes            | Missing          |
| Llama 2 70B               | `meta-llama/Llama-2-70b-hf`                     | 69B               | LlamaForCausalLM     | 138 GB | 4K      | Yes            | Missing          |
| Llama 3 8B Instruct       | `meta-llama/Meta-Llama-3-8B-Instruct`           | 8B                | LlamaForCausalLM     | 16 GB  | 8K      | Yes            | Missing          |
| Llama 3 70B Instruct      | `meta-llama/Meta-Llama-3-70B-Instruct`          | 70B               | LlamaForCausalLM     | 140 GB | 8K      | Yes            | Configured       |
| Llama 3.1 8B Instruct     | `meta-llama/Llama-3.1-8B-Instruct`              | 8B                | LlamaForCausalLM     | 16 GB  | 128K    | Yes            | Configured       |
| Llama 3.1 70B Instruct    | `meta-llama/Llama-3.1-70B-Instruct`             | 70B               | LlamaForCausalLM     | 140 GB | 128K    | Yes            | Configured       |
| Llama 3.1 405B Instruct   | `meta-llama/Llama-3.1-405B-Instruct`            | 405B              | LlamaForCausalLM     | 810 GB | 128K    | Yes            | Configured (FP8) |
| Llama 3.2 1B Instruct     | `meta-llama/Llama-3.2-1B-Instruct`              | 1B                | LlamaForCausalLM     | 2 GB   | 128K    | Yes            | Configured       |
| Llama 3.2 3B Instruct     | `meta-llama/Llama-3.2-3B-Instruct`              | 3B                | LlamaForCausalLM     | 6 GB   | 128K    | Yes            | Configured       |
| Llama 3.3 70B Instruct    | `meta-llama/Llama-3.3-70B-Instruct`             | 70B               | LlamaForCausalLM     | 140 GB | 128K    | Yes            | Configured       |
| Llama 4 Scout 17B-16E     | `meta-llama/Llama-4-Scout-17B-16E-Instruct`     | 17B (16 experts)  | Llama4ForCausalLM    | 109 GB | 10M     | Yes            | Configured       |
| Llama 4 Maverick 17B-128E | `meta-llama/Llama-4-Maverick-17B-128E-Instruct` | 17B (128 experts) | Llama4ForCausalLM    | 800 GB | 1M      | Yes            | Configured       |

### Qwen Family (Alibaba)

| Model              | HuggingFace ID                     | Parameters            | Architecture        | Size   | Context | Token Required | OME Status |
|--------------------|------------------------------------|-----------------------|---------------------|--------|---------|----------------|------------|
| Qwen3 0.6B         | `Qwen/Qwen3-0.6B`                  | 0.6B                  | Qwen3ForCausalLM    | 1.2 GB | 40K     | No             | Configured |
| Qwen3 1.7B         | `Qwen/Qwen3-1.7B`                  | 1.7B                  | Qwen3ForCausalLM    | 3.4 GB | 40K     | No             | Missing    |
| Qwen3 4B           | `Qwen/Qwen3-4B`                    | 4.0B                  | Qwen3ForCausalLM    | 8 GB   | 40K     | No             | Missing    |
| Qwen3 8B           | `Qwen/Qwen3-8B`                    | 8.2B                  | Qwen3ForCausalLM    | 16 GB  | 40K     | No             | Missing    |
| Qwen3 14B          | `Qwen/Qwen3-14B`                   | 14.8B                 | Qwen3ForCausalLM    | 30 GB  | 40K     | No             | Missing    |
| Qwen3 32B          | `Qwen/Qwen3-32B`                   | 32.8B                 | Qwen3ForCausalLM    | 66 GB  | 40K     | No             | Missing    |
| Qwen3 30B-A3B      | `Qwen/Qwen3-30B-A3B`               | 30B total (3B active) | Qwen3MoeForCausalLM | 60 GB  | 40K     | No             | Configured |
| Qwen3 Next 80B-A3B | `Qwen/Qwen3-Next-80B-A3B-Instruct` | 80B total (3B active) | Qwen3MoeForCausalLM | 160 GB | 40K     | No             | Configured |
| Qwen2.5 0.5B       | `Qwen/Qwen2.5-0.5B`                | 0.49B                 | Qwen2ForCausalLM    | 1 GB   | 32K     | No             | Missing    |
| Qwen2.5 1.5B       | `Qwen/Qwen2.5-1.5B`                | 1.54B                 | Qwen2ForCausalLM    | 3 GB   | 32K     | No             | Missing    |
| Qwen2.5 3B         | `Qwen/Qwen2.5-3B`                  | 3.09B                 | Qwen2ForCausalLM    | 6 GB   | 32K     | No             | Missing    |
| Qwen2.5 7B         | `Qwen/Qwen2.5-7B`                  | 7.61B                 | Qwen2ForCausalLM    | 15 GB  | 128K    | No             | Missing    |
| Qwen2.5 14B        | `Qwen/Qwen2.5-14B`                 | 14.7B                 | Qwen2ForCausalLM    | 29 GB  | 128K    | No             | Missing    |
| Qwen2.5 32B        | `Qwen/Qwen2.5-32B`                 | 32.5B                 | Qwen2ForCausalLM    | 65 GB  | 128K    | No             | Missing    |
| Qwen2.5 72B        | `Qwen/Qwen2.5-72B`                 | 72.7B                 | Qwen2ForCausalLM    | 145 GB | 128K    | No             | Missing    |

### DeepSeek Family

| Model                         | HuggingFace ID                              | Parameters              | Architecture              | Size    | Context | Token Required | OME Status |
|-------------------------------|---------------------------------------------|-------------------------|---------------------------|---------|---------|----------------|------------|
| DeepSeek-V2                   | `deepseek-ai/DeepSeek-V2`                   | 236B total (21B active) | DeepseekV2ForCausalLM     | 472 GB  | 128K    | No             | Missing    |
| DeepSeek-V2.5                 | `deepseek-ai/DeepSeek-V2.5`                 | 236B total (21B active) | DeepseekV2ForCausalLM     | 472 GB  | 128K    | Yes            | Missing    |
| DeepSeek-V3                   | `deepseek-ai/DeepSeek-V3`                   | 671B total (37B active) | DeepseekV3ForCausalLM     | 1.3 TB  | 128K    | No             | Configured |
| DeepSeek-V3-0324              | `deepseek-ai/DeepSeek-V3-0324`              | 671B total (37B active) | DeepseekV3ForCausalLM     | 1.3 TB  | 128K    | No             | Configured |
| DeepSeek-R1                   | `deepseek-ai/DeepSeek-R1`                   | 671B total (37B active) | DeepseekV3ForCausalLM     | 1.3 TB  | 128K    | No             | Configured |
| DeepSeek-R1-Zero              | `deepseek-ai/DeepSeek-R1-Zero`              | 671B total (37B active) | DeepseekV3ForCausalLM     | 1.3 TB  | 128K    | No             | Missing    |
| DeepSeek-R1-Distill-Llama-8B  | `deepseek-ai/DeepSeek-R1-Distill-Llama-8B`  | 8B                      | LlamaForCausalLM          | 16 GB   | 128K    | No             | Missing    |
| DeepSeek-R1-Distill-Llama-70B | `deepseek-ai/DeepSeek-R1-Distill-Llama-70B` | 71B                     | LlamaForCausalLM          | 142 GB  | 128K    | No             | Missing    |
| DeepSeek-R1-Distill-Qwen-1.5B | `deepseek-ai/DeepSeek-R1-Distill-Qwen-1.5B` | 1.5B                    | Qwen2ForCausalLM          | 3 GB    | 128K    | No             | Missing    |
| DeepSeek-R1-Distill-Qwen-7B   | `deepseek-ai/DeepSeek-R1-Distill-Qwen-7B`   | 8B                      | Qwen2ForCausalLM          | 16 GB   | 128K    | No             | Missing    |
| DeepSeek-R1-Distill-Qwen-14B  | `deepseek-ai/DeepSeek-R1-Distill-Qwen-14B`  | 15B                     | Qwen2ForCausalLM          | 30 GB   | 128K    | No             | Missing    |
| DeepSeek-R1-Distill-Qwen-32B  | `deepseek-ai/DeepSeek-R1-Distill-Qwen-32B`  | 33B                     | Qwen2ForCausalLM          | 66 GB   | 128K    | No             | Missing    |

### Mistral Family

| Model                    | HuggingFace ID                                  | Parameters  | Architecture           | Size   | Context | Token Required | OME Status |
|--------------------------|-------------------------------------------------|-------------|------------------------|--------|---------|----------------|------------|
| Mistral 7B v0.1          | `mistralai/Mistral-7B-v0.1`                     | 7B          | MistralForCausalLM     | 14 GB  | 32K     | No             | Missing    |
| Mistral 7B Instruct v0.2 | `mistralai/Mistral-7B-Instruct-v0.2`            | 7B          | MistralForCausalLM     | 14 GB  | 32K     | No             | Configured |
| Mistral 7B Instruct v0.3 | `mistralai/Mistral-7B-Instruct-v0.3`            | 7B          | MistralForCausalLM     | 14 GB  | 32K     | No             | Missing    |
| Mistral Small 3.1 24B    | `mistralai/Mistral-Small-3.1-24B-Instruct-2503` | 24B         | MistralForCausalLM     | 48 GB  | 128K    | No             | Configured |
| Mixtral 8x7B             | `mistralai/Mixtral-8x7B-v0.1`                   | 47B total   | MixtralForCausalLM     | 94 GB  | 32K     | No             | Missing    |
| Mixtral 8x7B Instruct    | `mistralai/Mixtral-8x7B-Instruct-v0.1`          | 47B total   | MixtralForCausalLM     | 94 GB  | 32K     | No             | Missing    |
| Mixtral 8x22B            | `mistralai/Mixtral-8x22B-v0.1`                  | 141B total  | MixtralForCausalLM     | 282 GB | 64K     | No             | Missing    |
| Mistral NeMo 12B         | `mistralai/Mistral-Nemo-Instruct-2407`          | 12B         | MistralForCausalLM     | 24 GB  | 128K    | No             | Missing    |

### Google Gemma Family

| Model          | HuggingFace ID          | Parameters | Architecture                   | Size  | Context | Token Required | OME Status |
|----------------|-------------------------|------------|--------------------------------|-------|---------|----------------|------------|
| Gemma 2B       | `google/gemma-2b`       | 2B         | GemmaForCausalLM               | 5 GB  | 8K      | Yes            | Missing    |
| Gemma 7B       | `google/gemma-7b`       | 7B         | GemmaForCausalLM               | 17 GB | 8K      | Yes            | Missing    |
| Gemma 2 2B     | `google/gemma-2-2b`     | 3B         | Gemma2ForCausalLM              | 6 GB  | 8K      | Yes            | Missing    |
| Gemma 2 9B     | `google/gemma-2-9b`     | 9B         | Gemma2ForCausalLM              | 18 GB | 8K      | Yes            | Missing    |
| Gemma 2 27B    | `google/gemma-2-27b`    | 27B        | Gemma2ForCausalLM              | 54 GB | 8K      | Yes            | Missing    |
| Gemma 3 1B IT  | `google/gemma-3-1b-it`  | 1B         | Gemma3ForCausalLM              | 2 GB  | 128K    | Yes            | Configured |
| Gemma 3 4B IT  | `google/gemma-3-4b-it`  | 4B         | Gemma3ForConditionalGeneration | 8 GB  | 128K    | Yes            | Configured |
| Gemma 3 12B IT | `google/gemma-3-12b-it` | 12B        | Gemma3ForConditionalGeneration | 24 GB | 128K    | Yes            | Missing    |
| Gemma 3 27B IT | `google/gemma-3-27b-it` | 27B        | Gemma3ForConditionalGeneration | 54 GB | 128K    | Yes            | Missing    |

### Microsoft Phi Family

| Model           | HuggingFace ID                       | Parameters                | Architecture          | Size   | Context | Token Required | OME Status |
|-----------------|--------------------------------------|---------------------------|-----------------------|--------|---------|----------------|------------|
| Phi-1.5         | `microsoft/phi-1_5`                  | 1.3B                      | PhiForCausalLM        | 2.6 GB | 2K      | No             | Missing    |
| Phi-2           | `microsoft/phi-2`                    | 2.7B                      | PhiForCausalLM        | 5.4 GB | 2K      | No             | Missing    |
| Phi-3 Mini 4K   | `microsoft/Phi-3-mini-4k-instruct`   | 3.8B                      | Phi3ForCausalLM       | 7.6 GB | 4K      | No             | Missing    |
| Phi-3 Mini 128K | `microsoft/Phi-3-mini-128k-instruct` | 3.8B                      | Phi3ForCausalLM       | 7.6 GB | 128K    | No             | Missing    |
| Phi-3 Small 8K  | `microsoft/Phi-3-small-8k-instruct`  | 7B                        | Phi3SmallForCausalLM  | 14 GB  | 8K      | No             | Missing    |
| Phi-3 Medium 4K | `microsoft/Phi-3-medium-4k-instruct` | 14B                       | Phi3ForCausalLM       | 28 GB  | 4K      | No             | Missing    |
| Phi-3.5 Mini    | `microsoft/Phi-3.5-mini-instruct`    | 3.8B                      | Phi3ForCausalLM       | 7.6 GB | 128K    | No             | Missing    |
| Phi-3.5 MoE     | `microsoft/Phi-3.5-MoE-instruct`     | 41.9B total (6.6B active) | PhiMoEForCausalLM     | 84 GB  | 128K    | No             | Configured |
| Phi-4           | `microsoft/phi-4`                    | 14B                       | Phi3ForCausalLM       | 28 GB  | 16K     | No             | Missing    |

### NVIDIA Nemotron Family

| Model                  | HuggingFace ID                            | Parameters | Architecture              | Size   | Context | Token Required | OME Status |
|------------------------|-------------------------------------------|------------|---------------------------|--------|---------|----------------|------------|
| Nemotron Nano 8B v1    | `nvidia/Llama-3.1-Nemotron-Nano-8B-v1`    | 8B         | LlamaForCausalLM          | 16 GB  | 128K    | No             | Configured |
| Nemotron Nano 9B v2    | `nvidia/NVIDIA-Nemotron-Nano-9B-v2`       | 9B         | NemotronNanoForCausalLM   | 18 GB  | 128K    | No             | Configured |
| Nemotron Super 49B v1  | `nvidia/Llama-3_3-Nemotron-Super-49B-v1`  | 49B        | LlamaForCausalLM          | 98 GB  | 128K    | No             | Configured |
| Nemotron Ultra 253B v1 | `nvidia/Llama-3_1-Nemotron-Ultra-253B-v1` | 253B       | LlamaForCausalLM          | 506 GB | 128K    | No             | Configured |
| Jet-Nemotron 2B        | `jet-ai/Jet-Nemotron-2B`                  | 2B         | NemotronNanoForCausalLM   | 4 GB   | 128K    | No             | Configured |

### IBM Granite Family

| Model                    | HuggingFace ID                              | Parameters             | Architecture          | Size  | Context | Token Required | OME Status |
|--------------------------|---------------------------------------------|------------------------|-----------------------|-------|---------|----------------|------------|
| Granite 3.0 2B           | `ibm-granite/granite-3.0-2b-instruct`       | 2.5B                   | GraniteForCausalLM    | 5 GB  | 4K      | No             | Missing    |
| Granite 3.0 8B           | `ibm-granite/granite-3.0-8b-instruct`       | 8.1B                   | GraniteForCausalLM    | 16 GB | 4K      | No             | Missing    |
| Granite 3.0 3B A800M MoE | `ibm-granite/granite-3.0-3b-a800m-instruct` | 3B total (800M active) | GraniteMoeForCausalLM | 6 GB  | 4K      | No             | Configured |
| Granite 3.1 2B           | `ibm-granite/granite-3.1-2b-instruct`       | 2.5B                   | GraniteForCausalLM    | 5 GB  | 128K    | No             | Missing    |
| Granite 3.1 8B           | `ibm-granite/granite-3.1-8b-instruct`       | 8B                     | GraniteForCausalLM    | 16 GB | 128K    | No             | Configured |

### Other Generative Models

| Model                   | HuggingFace ID                         | Parameters              | Architecture                    | Size   | Context | Token Required | OME Status |
|-------------------------|----------------------------------------|-------------------------|---------------------------------|--------|---------|----------------|------------|
| GPT-OSS 20B             | `openai/gpt-oss-20b`                   | 20B                     | GPTOSSForCausalLM               | 40 GB  | -       | No             | Configured |
| GPT-OSS 120B            | `openai/gpt-oss-120b`                  | 120B                    | GPTOSSForCausalLM               | 240 GB | -       | No             | Configured |
| ChatGLM2 6B             | `THUDM/chatglm2-6b`                    | 6B                      | ChatGLMForConditionalGeneration | 12 GB  | 32K     | No             | Configured |
| GLM-4 9B Chat           | `ZhipuAI/glm-4-9b-chat`                | 9B                      | ChatGLMForConditionalGeneration | 18 GB  | 1M      | No             | Configured |
| InternLM2 7B            | `internlm/internlm2-7b`                | 7B                      | InternLM2ForCausalLM            | 14 GB  | 32K     | No             | Configured |
| InternLM2 20B           | `internlm/internlm2-20b`               | 20B                     | InternLM2ForCausalLM            | 40 GB  | 32K     | No             | Missing    |
| EXAONE 3.5 7.8B         | `LGAI-EXAONE/EXAONE-3.5-7.8B-Instruct` | 7.8B                    | ExaoneForCausalLM               | 16 GB  | 32K     | No             | Configured |
| Baichuan2 7B            | `baichuan-inc/Baichuan2-7B-Chat`       | 7B                      | BaichuanForCausalLM             | 14 GB  | 4K      | No             | Missing    |
| Baichuan2 13B           | `baichuan-inc/Baichuan2-13B-Chat`      | 13B                     | BaichuanForCausalLM             | 26 GB  | 4K      | No             | Configured |
| XVERSE MoE A36B         | `xverse/XVERSE-MoE-A36B`               | 255B total (36B active) | XverseMoeForCausalLM            | 510 GB | 256K    | No             | Configured |
| SmolLM 135M             | `HuggingFaceTB/SmolLM-135M`            | 135M                    | LlamaForCausalLM                | 0.3 GB | 2K      | No             | Missing    |
| SmolLM 360M             | `HuggingFaceTB/SmolLM-360M`            | 360M                    | LlamaForCausalLM                | 0.7 GB | 2K      | No             | Missing    |
| SmolLM 1.7B             | `HuggingFaceTB/SmolLM-1.7B`            | 1.7B                    | LlamaForCausalLM                | 3.4 GB | 2K      | No             | Configured |
| MiniCPM3 4B             | `openbmb/MiniCPM3-4B`                  | 4B                      | MiniCPMForCausalLM              | 8 GB   | 32K     | No             | Configured |
| MiMo 7B RL              | `XiaomiMiMo/MiMo-7B-RL`                | 7B                      | MiMoForCausalLM                 | 14 GB  | 128K    | No             | Configured |
| ERNIE-4.5 21B A3B       | `baidu/ERNIE-4.5-21B-A3B-PT`           | 21B total (3B active)   | ErnieMoeForCausalLM             | 42 GB  | -       | No             | Configured |
| OLMo 2 7B               | `allenai/OLMo-2-1124-7B-Instruct`      | 7B                      | OlmoForCausalLM                 | 14 GB  | 4K      | No             | Configured |
| OLMoE 1B-7B             | `allenai/OLMoE-1B-7B-0924`             | 7B total (1B active)    | OlmoeForCausalLM                | 14 GB  | 4K      | No             | Configured |
| MiniMax-M2              | `minimax/MiniMax-M2`                   | Unknown                 | MiniMaxForCausalLM              | -      | -       | No             | Configured |
| StableLM Tuned Alpha 7B | `stabilityai/stablelm-tuned-alpha-7b`  | 7B                      | StableLmForCausalLM             | 14 GB  | 4K      | No             | Configured |
| Command-R v01           | `CohereForAI/c4ai-command-r-v01`       | 35B                     | CohereForCausalLM               | 70 GB  | 128K    | No             | Configured |
| DBRX Instruct           | `databricks/dbrx-instruct`             | 132B total (36B active) | DbrxForCausalLM                 | 264 GB | 32K     | No             | Configured |
| Grok-1                  | `xai-org/grok-1`                       | 314B                    | Grok1ModelForCausalLM           | 628 GB | 8K      | No             | Configured |
| Arcee AFM 4.5B          | `arcee-ai/AFM-4.5B-Base`               | 4.5B                    | LlamaForCausalLM                | 9 GB   | -       | No             | Configured |
| Persimmon 8B            | `adept/persimmon-8b-chat`              | 8B                      | PersimmonForCausalLM            | 16 GB  | 16K     | No             | Configured |
| SOLAR 10.7B             | `upstage/SOLAR-10.7B-Instruct-v1.0`    | 10.7B                   | LlamaForCausalLM                | 21 GB  | 4K      | No             | Configured |
| Tele-FLM                | `CofeAI/Tele-FLM`                      | 52B                     | TeleFLMForCausalLM              | 104 GB | -       | No             | Configured |
| Ling Lite               | `inclusionAI/Ling-lite`                | 16.8B                   | LingForCausalLM                 | 34 GB  | -       | No             | Configured |
| Ling Plus               | `inclusionAI/Ling-plus`                | 290B total              | LingMoeForCausalLM              | 580 GB | -       | No             | Configured |
| Orion 14B               | `OrionStarAI/Orion-14B-Base`           | 14B                     | OrionForCausalLM                | 28 GB  | -       | No             | Configured |
| StarCoder2 3B           | `bigcode/starcoder2-3b`                | 3B                      | Starcoder2ForCausalLM           | 6 GB   | 16K     | No             | Missing    |
| StarCoder2 7B           | `bigcode/starcoder2-7b`                | 7B                      | Starcoder2ForCausalLM           | 14 GB  | 16K     | No             | Configured |
| StarCoder2 15B          | `bigcode/starcoder2-15b`               | 15B                     | Starcoder2ForCausalLM           | 30 GB  | 16K     | No             | Missing    |
| Kimi-K2 Instruct        | `moonshotai/Kimi-K2-Instruct`          | 1T total (32B active)   | KimiMoeForCausalLM              | -      | 128K    | No             | Configured |

---

## Multimodal Language Models

### Meta Llama Vision

| Model                | HuggingFace ID                             | Parameters | Architecture                      | Size   | Context | Token Required | OME Status |
|----------------------|--------------------------------------------|------------|-----------------------------------|--------|---------|----------------|------------|
| Llama 3.2 11B Vision | `meta-llama/Llama-3.2-11B-Vision-Instruct` | 11B        | MllamaForConditionalGeneration    | 22 GB  | 128K    | Yes            | Configured |
| Llama 3.2 90B Vision | `meta-llama/Llama-3.2-90B-Vision-Instruct` | 90B        | MllamaForConditionalGeneration    | 180 GB | 128K    | Yes            | Configured |

### Qwen Vision

| Model              | HuggingFace ID                     | Parameters              | Architecture                    | Size   | Context | Token Required | OME Status |
|--------------------|------------------------------------|-------------------------|---------------------------------|--------|---------|----------------|------------|
| Qwen-VL            | `Qwen/Qwen-VL`                     | 9.6B                    | QWenLMHeadModel                 | 19 GB  | 32K     | No             | Missing    |
| Qwen-VL-Chat       | `Qwen/Qwen-VL-Chat`                | 9.6B                    | QWenLMHeadModel                 | 19 GB  | 32K     | No             | Missing    |
| Qwen2-VL 2B        | `Qwen/Qwen2-VL-2B-Instruct`        | 2B                      | Qwen2VLForConditionalGeneration | 4 GB   | 32K     | No             | Missing    |
| Qwen2-VL 7B        | `Qwen/Qwen2-VL-7B-Instruct`        | 7B                      | Qwen2VLForConditionalGeneration | 14 GB  | 32K     | No             | Missing    |
| Qwen2-VL 72B       | `Qwen/Qwen2-VL-72B-Instruct`       | 72B                     | Qwen2VLForConditionalGeneration | 144 GB | 32K     | No             | Missing    |
| Qwen3-VL 235B-A22B | `Qwen/Qwen3-VL-235B-A22B-Instruct` | 235B total (22B active) | Qwen3VLForConditionalGeneration | 470 GB | 32K     | No             | Configured |

### DeepSeek Vision

| Model          | HuggingFace ID              | Parameters | Architecture                    | Size  | Context | Token Required | OME Status |
|----------------|-----------------------------|------------|---------------------------------|-------|---------|----------------|------------|
| DeepSeek-VL2   | `deepseek-ai/deepseek-vl2`  | 27B        | DeepseekVLV2ForCausalLM         | 54 GB | 128K    | No             | Configured |
| Janus-Pro 7B   | `deepseek-ai/Janus-Pro-7B`  | 7B         | MultiModalityCausalLM           | 14 GB | 128K    | No             | Configured |

### Microsoft Phi Vision

| Model             | HuggingFace ID                         | Parameters | Architecture      | Size   | Context | Token Required | OME Status |
|-------------------|----------------------------------------|------------|-------------------|--------|---------|----------------|------------|
| Phi-3 Vision 128K | `microsoft/Phi-3-vision-128k-instruct` | 4.2B       | Phi3VForCausalLM  | 8.4 GB | 128K    | No             | Configured |
| Phi-4 Multimodal  | `microsoft/Phi-4-multimodal-instruct`  | 5.6B       | Phi4MMForCausalLM | 11 GB  | 16K     | No             | Configured |

### LLaVA Family

| Model              | HuggingFace ID                         | Parameters | Architecture                           | Size   | Context | Token Required | OME Status |
|--------------------|----------------------------------------|------------|----------------------------------------|--------|---------|----------------|------------|
| LLaVA v1.5 7B      | `liuhaotian/llava-v1.5-7b`             | 7B         | LlavaForConditionalGeneration          | 14 GB  | 4K      | No             | Missing    |
| LLaVA v1.5 13B     | `liuhaotian/llava-v1.5-13b`            | 13B        | LlavaForConditionalGeneration          | 26 GB  | 4K      | No             | Configured |
| LLaVA v1.6 7B      | `liuhaotian/llava-v1.6-vicuna-7b`      | 7B         | LlavaForConditionalGeneration          | 14 GB  | 4K      | No             | Missing    |
| LLaVA v1.6 13B     | `liuhaotian/llava-v1.6-vicuna-13b`     | 13B        | LlavaForConditionalGeneration          | 26 GB  | 4K      | No             | Missing    |
| LLaVA-NeXT 8B      | `lmms-lab/llava-next-8b`               | 8B         | LlavaNextForConditionalGeneration      | 16 GB  | 32K     | No             | Missing    |
| LLaVA-NeXT 72B     | `lmms-lab/llava-next-72b`              | 72B        | LlavaNextForConditionalGeneration      | 144 GB | 32K     | No             | Configured |
| LLaVA-OneVision 7B | `lmms-lab/llava-onevision-qwen2-7b-ov` | 7B         | LlavaOnevisionForConditionalGeneration | 14 GB  | 32K     | No             | Configured |

### Other Multimodal Models

| Model                 | HuggingFace ID                                  | Parameters | Architecture                       | Size  | Context | Token Required | OME Status |
|-----------------------|-------------------------------------------------|------------|------------------------------------|-------|---------|----------------|------------|
| Gemma 3 4B IT         | `google/gemma-3-4b-it`                          | 4B         | Gemma3ForConditionalGeneration     | 8 GB  | 128K    | Yes            | Configured |
| Gemma 3 12B IT        | `google/gemma-3-12b-it`                         | 12B        | Gemma3ForConditionalGeneration     | 24 GB | 128K    | Yes            | Missing    |
| Gemma 3 27B IT        | `google/gemma-3-27b-it`                         | 27B        | Gemma3ForConditionalGeneration     | 54 GB | 128K    | Yes            | Missing    |
| MiniCPM-V 2.6         | `openbmb/MiniCPM-V-2_6`                         | 8B         | MiniCPMV                           | 16 GB | 32K     | No             | Configured |
| MiMo-VL 7B RL         | `XiaomiMiMo/MiMo-VL-7B-RL`                      | 7B         | MiMoVLForConditionalGeneration     | 14 GB | 128K    | No             | Configured |
| Kimi-VL A3B           | `moonshotai/Kimi-VL-A3B-Instruct`               | 3B active  | KimiVLForConditionalGeneration     | -     | 128K    | No             | Configured |
| Mistral Small 3.1 24B | `mistralai/Mistral-Small-3.1-24B-Instruct-2503` | 24B        | PixtralForConditionalGeneration    | 48 GB | 128K    | No             | Configured |
| GLM-4.5V              | `zai-org/GLM-4.5V`                              | Unknown    | ChatGLMForConditionalGeneration    | -     | 1M      | No             | Configured |
| DotsVLM               | `rednote-hilab/dots.vlm1.inst`                  | Unknown    | DotsVLMForConditionalGeneration    | -     | -       | No             | Configured |
| DotsVLM-OCR           | `rednote-hilab/dots.ocr`                        | Unknown    | DotsVLMForConditionalGeneration    | -     | -       | No             | Configured |
| NVILA 8B              | `Efficient-Large-Model/NVILA-8B`                | 8B         | NVILAForConditionalGeneration      | 16 GB | 128K    | No             | Configured |
| Nemotron Nano 12B VL  | `nvidia/NVIDIA-Nemotron-Nano-12B-v2-VL-BF16`    | 12B        | NemotronNanoVLForConditionalGen    | 24 GB | 128K    | No             | Configured |
| GME-Qwen2-VL 2B       | `Alibaba-NLP/gme-Qwen2-VL-2B-Instruct`          | 2B         | GMEQwen2VLForConditionalGeneration | 4 GB  | 32K     | No             | Configured |

---

## Embedding Models

| Model                  | HuggingFace ID                         | Parameters | Architecture              | Size   | Embedding Dim | Token Required | OME Status |
|------------------------|----------------------------------------|------------|---------------------------|--------|---------------|----------------|------------|
| E5-Mistral 7B          | `intfloat/e5-mistral-7b-instruct`      | 7B         | MistralModel              | 14 GB  | 4096          | No             | Configured |
| GTE-Qwen2 7B           | `Alibaba-NLP/gte-Qwen2-7B-instruct`    | 7B         | Qwen2Model                | 14 GB  | 3584          | No             | Configured |
| BGE Large EN v1.5      | `BAAI/bge-large-en-v1.5`               | 335M       | BertModel                 | 0.7 GB | 1024          | No             | Configured |
| BGE M3                 | `BAAI/bge-m3`                          | 365M       | XLMRobertaModel           | 0.7 GB | 1024          | No             | Missing    |
| Qwen3-Embedding 0.6B   | `Qwen/Qwen3-Embedding-0.6B`            | 0.6B       | Qwen3Model                | 1.2 GB | 1024          | No             | Configured |
| Qwen3-Embedding 4B     | `Qwen/Qwen3-Embedding-4B`              | 4B         | Qwen3Model                | 8 GB   | 2560          | No             | Configured |
| Qwen3-Embedding 8B     | `Qwen/Qwen3-Embedding-8B`              | 8B         | Qwen3Model                | 16 GB  | 4096          | No             | Missing    |
| CLIP ViT Large Patch14 | `openai/clip-vit-large-patch14-336`    | 428M       | CLIPModel                 | 0.9 GB | 768           | No             | Configured |
| GME-Qwen2-VL 2B        | `Alibaba-NLP/gme-Qwen2-VL-2B-Instruct` | 2B         | GMEQwen2VLModel           | 4 GB   | 1536          | No             | Configured |

**Note**: Embedding models require the `--is-embedding` flag when launching. Some models (like BGE) require specific attention backends (`triton` or `torch_native`).

---

## Reward Models

| Model                        | HuggingFace ID                             | Parameters | Architecture                    | Size   | Context | Token Required | OME Status |
|------------------------------|--------------------------------------------|------------|---------------------------------|--------|---------|----------------|------------|
| Skywork-Reward-Llama-3.1-8B  | `Skywork/Skywork-Reward-Llama-3.1-8B-v0.2` | 8B         | LlamaForSequenceClassification  | 16 GB  | 128K    | No             | Missing    |
| Skywork-Reward-Gemma-2-27B   | `Skywork/Skywork-Reward-Gemma-2-27B-v0.2`  | 27B        | Gemma2ForSequenceClassification | 54 GB  | 8K      | No             | Missing    |
| InternLM2 7B Reward          | `internlm/internlm2-7b-reward`             | 7B         | InternLM2ForRewardModel         | 14 GB  | 32K     | No             | Missing    |
| Qwen2.5-Math-RM-72B          | `Qwen/Qwen2.5-Math-RM-72B`                 | 72B        | Qwen2ForRewardModel             | 144 GB | 4K      | No             | Missing    |
| Qwen2.5 1.5B Apeach          | `jason9693/Qwen2.5-1.5B-apeach`            | 1.5B       | Qwen2ForSequenceClassification  | 3 GB   | 32K     | No             | Missing    |

**Note**: Reward models require the `--is-embedding` flag when launching and output scalar reward scores for RLHF applications.

---

## Rerank Models

| Model              | HuggingFace ID            | Parameters | Architecture                        | Size   | Token Required | OME Status |
|--------------------|---------------------------|------------|-------------------------------------|--------|----------------|------------|
| BGE-Reranker-v2-M3 | `BAAI/bge-reranker-v2-m3` | 568M       | XLMRobertaForSequenceClassification | 1.1 GB | No             | Configured |

**Note**: Rerank models require the `--is-embedding` flag and only support `triton` and `torch_native` attention backends.

---

## vLLM Additional Models

The following models are supported by vLLM but may not be explicitly listed in SGLang documentation. Many share common architectures with SGLang-supported models.

### vLLM Text-Only Generative Models

| Model                | HuggingFace ID                            | Parameters            | Architecture              | Context | Token Required |
|----------------------|-------------------------------------------|-----------------------|---------------------------|---------|----------------|
| Aquila 7B            | `BAAI/Aquila-7B`                          | 7B                    | AquilaForCausalLM         | 2K      | No             |
| AquilaChat 7B        | `BAAI/AquilaChat-7B`                      | 7B                    | AquilaForCausalLM         | 2K      | No             |
| Arctic Base          | `Snowflake/snowflake-arctic-base`         | 480B (17B active)     | ArcticForCausalLM         | 4K      | No             |
| Arctic Instruct      | `Snowflake/snowflake-arctic-instruct`     | 480B (17B active)     | ArcticForCausalLM         | 4K      | No             |
| Bloom                | `bigscience/bloom`                        | 176B                  | BloomForCausalLM          | 2K      | No             |
| Bloomz               | `bigscience/bloomz`                       | 176B                  | BloomForCausalLM          | 2K      | No             |
| BART Base            | `facebook/bart-base`                      | 140M                  | BartForConditionalGen     | 1K      | No             |
| BART Large CNN       | `facebook/bart-large-cnn`                 | 400M                  | BartForConditionalGen     | 1K      | No             |
| ChatGLM3 6B          | `THUDM/chatglm3-6b`                       | 6B                    | ChatGLMModel              | 32K     | No             |
| Cohere2 Command R7B  | `CohereForAI/c4ai-command-r7b-12-2024`    | 7B                    | Cohere2ForCausalLM        | 128K    | No             |
| DeciLM 7B            | `Deci/DeciLM-7B`                          | 7B                    | DeciLMForCausalLM         | 8K      | No             |
| DeciLM 7B Instruct   | `Deci/DeciLM-7B-instruct`                 | 7B                    | DeciLMForCausalLM         | 8K      | No             |
| DeepSeek LLM 7B      | `deepseek-ai/deepseek-llm-7b-chat`        | 7B                    | DeepseekForCausalLM       | 4K      | No             |
| DeepSeek LLM 67B     | `deepseek-ai/deepseek-llm-67b-base`       | 67B                   | DeepseekForCausalLM       | 4K      | No             |
| Falcon 7B            | `tiiuae/falcon-7b`                        | 7B                    | FalconForCausalLM         | 2K      | No             |
| Falcon 40B           | `tiiuae/falcon-40b`                       | 40B                   | FalconForCausalLM         | 2K      | No             |
| Falcon RW 7B         | `tiiuae/falcon-rw-7b`                     | 7B                    | FalconForCausalLM         | 2K      | No             |
| Falcon Mamba 7B      | `tiiuae/falcon-mamba-7b`                  | 7B                    | FalconMambaForCausalLM    | 8K      | No             |
| GritLM 7B            | `parasail-ai/GritLM-7B-vllm`              | 7B                    | GritLM                    | 4K      | No             |
| GPT-2                | `gpt2`                                    | 124M                  | GPT2LMHeadModel           | 1K      | No             |
| GPT-2 XL             | `gpt2-xl`                                 | 1.5B                  | GPT2LMHeadModel           | 1K      | No             |
| GPT-J 6B             | `EleutherAI/gpt-j-6b`                     | 6B                    | GPTJForCausalLM           | 2K      | No             |
| GPT-NeoX 20B         | `EleutherAI/gpt-neox-20b`                 | 20B                   | GPTNeoXForCausalLM        | 2K      | No             |
| Pythia 12B           | `EleutherAI/pythia-12b`                   | 12B                   | GPTNeoXForCausalLM        | 2K      | No             |
| Dolly v2 12B         | `databricks/dolly-v2-12b`                 | 12B                   | GPTNeoXForCausalLM        | 2K      | No             |
| InternLM 7B          | `internlm/internlm-7b`                    | 7B                    | InternLMForCausalLM       | 8K      | No             |
| InternLM Chat 7B     | `internlm/internlm-chat-7b`               | 7B                    | InternLMForCausalLM       | 8K      | No             |
| InternLM3 8B         | `internlm/internlm3-8b-instruct`          | 8B                    | InternLM3ForCausalLM      | 32K     | No             |
| JAIS 13B             | `inceptionai/jais-13b`                    | 13B                   | JAISLMHeadModel           | 2K      | No             |
| JAIS 30B v3          | `inceptionai/jais-30b-v3`                 | 30B                   | JAISLMHeadModel           | 8K      | No             |
| Jamba 1.5 Large      | `ai21labs/AI21-Jamba-1.5-Large`           | 398B (94B active)     | JambaForCausalLM          | 256K    | No             |
| Jamba 1.5 Mini       | `ai21labs/AI21-Jamba-1.5-Mini`            | 52B (12B active)      | JambaForCausalLM          | 256K    | No             |
| Jamba v0.1           | `ai21labs/Jamba-v0.1`                     | 52B (12B active)      | JambaForCausalLM          | 256K    | No             |
| Mamba 130M           | `state-spaces/mamba-130m-hf`              | 130M                  | MambaForCausalLM          | 2K      | No             |
| Mamba 790M           | `state-spaces/mamba-790m-hf`              | 790M                  | MambaForCausalLM          | 2K      | No             |
| Mamba 2.8B           | `state-spaces/mamba-2.8b-hf`              | 2.8B                  | MambaForCausalLM          | 2K      | No             |
| MiniCPM 2B           | `openbmb/MiniCPM-2B-sft-bf16`             | 2B                    | MiniCPMForCausalLM        | 4K      | No             |
| MiniCPM S 1B         | `openbmb/MiniCPM-S-1B-sft`                | 1B                    | MiniCPMForCausalLM        | 4K      | No             |
| MPT 7B               | `mosaicml/mpt-7b`                         | 7B                    | MPTForCausalLM            | 2K      | No             |
| MPT 30B              | `mosaicml/mpt-30b`                        | 30B                   | MPTForCausalLM            | 8K      | No             |
| Minitron 8B          | `nvidia/Minitron-8B-Base`                 | 8B                    | NemotronForCausalLM       | 4K      | No             |
| Nemotron 340B FP8    | `mgoin/Nemotron-4-340B-Base-hf-FP8`       | 340B                  | NemotronForCausalLM       | 4K      | No             |
| OLMo 1B              | `allenai/OLMo-1B-hf`                      | 1B                    | OLMoForCausalLM           | 2K      | No             |
| OLMo 7B              | `allenai/OLMo-7B-hf`                      | 7B                    | OLMoForCausalLM           | 2K      | No             |
| OLMo2 7B             | `allenai/OLMo2-7B-1124`                   | 7B                    | OLMo2ForCausalLM          | 4K      | No             |
| OPT 66B              | `facebook/opt-66b`                        | 66B                   | OPTForCausalLM            | 2K      | No             |
| OPT IML Max 30B      | `facebook/opt-iml-max-30b`                | 30B                   | OPTForCausalLM            | 2K      | No             |
| Qwen 7B              | `Qwen/Qwen-7B`                            | 7B                    | QWenLMHeadModel           | 8K      | No             |
| Qwen 7B Chat         | `Qwen/Qwen-7B-Chat`                       | 7B                    | QWenLMHeadModel           | 8K      | No             |
| QwQ 32B Preview      | `Qwen/QwQ-32B-Preview`                    | 32B                   | Qwen2ForCausalLM          | 32K     | No             |
| Qwen1.5 MoE A2.7B    | `Qwen/Qwen1.5-MoE-A2.7B`                  | 14B (2.7B active)     | Qwen2MoeForCausalLM       | 32K     | No             |
| StableLM 3B          | `stabilityai/stablelm-3b-4e1t`            | 3B                    | StableLmForCausalLM       | 4K      | No             |
| Solar Pro Preview    | `upstage/solar-pro-preview-instruct`      | 22B                   | SolarForCausalLM          | 4K      | No             |
| TeleChat2 3B         | `TeleAI/TeleChat2-3B`                     | 3B                    | TeleChat2ForCausalLM      | 8K      | No             |
| TeleChat2 7B         | `TeleAI/TeleChat2-7B`                     | 7B                    | TeleChat2ForCausalLM      | 8K      | No             |
| TeleChat2 35B        | `TeleAI/TeleChat2-35B`                    | 35B                   | TeleChat2ForCausalLM      | 8K      | No             |
| XVERSE 7B Chat       | `xverse/XVERSE-7B-Chat`                   | 7B                    | XverseForCausalLM         | 8K      | No             |
| XVERSE 13B Chat      | `xverse/XVERSE-13B-Chat`                  | 13B                   | XverseForCausalLM         | 8K      | No             |
| XVERSE 65B Chat      | `xverse/XVERSE-65B-Chat`                  | 65B                   | XverseForCausalLM         | 16K     | No             |
| Yi 34B               | `01-ai/Yi-34B`                            | 34B                   | LlamaForCausalLM          | 4K      | No             |
| WizardCoder 15B      | `WizardLM/WizardCoder-15B-V1.0`           | 15B                   | GPTBigCodeForCausalLM     | 8K      | No             |
| StarCoder            | `bigcode/starcoder`                       | 15B                   | GPTBigCodeForCausalLM     | 8K      | No             |
| SantaCoder           | `bigcode/gpt_bigcode-santacoder`          | 1.1B                  | GPTBigCodeForCausalLM     | 2K      | No             |

### vLLM Multimodal Models

| Model                   | HuggingFace ID                              | Parameters        | Architecture                         | Context | Token Required |
|-------------------------|---------------------------------------------|-------------------|--------------------------------------|---------|----------------|
| Aria                    | `rhymes-ai/Aria`                            | 25B               | AriaForConditionalGeneration         | 64K     | No             |
| Blip2 OPT 2.7B          | `Salesforce/blip2-opt-2.7b`                 | 3.8B              | Blip2ForConditionalGeneration        | 32      | No             |
| Blip2 OPT 6.7B          | `Salesforce/blip2-opt-6.7b`                 | 7.8B              | Blip2ForConditionalGeneration        | 32      | No             |
| Chameleon 7B            | `facebook/chameleon-7b`                     | 7B                | ChameleonForConditionalGen           | 4K      | No             |
| DeepSeek-VL2 Tiny       | `deepseek-ai/deepseek-vl2-tiny`             | 3B                | DeepseekVLV2ForCausalLM              | 4K      | No             |
| DeepSeek-VL2 Small      | `deepseek-ai/deepseek-vl2-small`            | 16B               | DeepseekVLV2ForCausalLM              | 4K      | No             |
| Fuyu 8B                 | `adept/fuyu-8b`                             | 8B                | FuyuForCausalLM                      | 16K     | No             |
| GLM-4V 9B               | `THUDM/glm-4v-9b`                           | 9B                | ChatGLMModel                         | 8K      | No             |
| H2OVL Mississippi 800M  | `h2oai/h2ovl-mississippi-800m`              | 800M              | H2OVLChatModel                       | 4K      | No             |
| H2OVL Mississippi 2B    | `h2oai/h2ovl-mississippi-2b`                | 2B                | H2OVLChatModel                       | 4K      | No             |
| Idefics3 8B Llama3      | `HuggingFaceM4/Idefics3-8B-Llama3`          | 8B                | Idefics3ForConditionalGeneration     | 8K      | No             |
| InternVL2.5 4B          | `OpenGVLab/InternVL2_5-4B`                  | 4B                | InternVLChatModel                    | 8K      | No             |
| InternVL2 4B            | `OpenGVLab/InternVL2-4B`                    | 4B                | InternVLChatModel                    | 8K      | No             |
| Mono-InternVL 2B        | `OpenGVLab/Mono-InternVL-2B`                | 2B                | InternVLChatModel                    | 8K      | No             |
| LLaVA 1.5 7B HF         | `llava-hf/llava-1.5-7b-hf`                  | 7B                | LlavaForConditionalGeneration        | 4K      | No             |
| LLaVA v1.6 Mistral 7B   | `llava-hf/llava-v1.6-mistral-7b-hf`         | 7B                | LlavaNextForConditionalGeneration    | 4K      | No             |
| LLaVA-NeXT-Video 7B     | `llava-hf/LLaVA-NeXT-Video-7B-hf`           | 7B                | LlavaNextVideoForConditionalGen      | 4K      | No             |
| LLaVA-OneVision 0.5B    | `llava-hf/llava-onevision-qwen2-0.5b-ov-hf` | 0.5B              | LlavaOnevisionForConditionalGen      | 32K     | No             |
| Mantis 8B               | `TIGER-Lab/Mantis-8B-siglip-llama3`         | 8B                | LlavaForConditionalGeneration        | 8K      | No             |
| MiniCPM-V 2             | `openbmb/MiniCPM-V-2`                       | 3B                | MiniCPMV                             | 2K      | No             |
| MiniCPM-Llama3-V 2.5    | `openbmb/MiniCPM-Llama3-V-2_5`              | 8B                | MiniCPMV                             | 8K      | No             |
| Molmo 7B D              | `allenai/Molmo-7B-D-0924`                   | 7B                | MolmoForCausalLM                     | 4K      | No             |
| Molmo 72B               | `allenai/Molmo-72B-0924`                    | 72B               | MolmoForCausalLM                     | 4K      | No             |
| NVLM-D 72B              | `nvidia/NVLM-D-72B`                         | 72B               | NVLM_D_Model                         | 4K      | No             |
| PaliGemma 3B 224        | `google/paligemma-3b-pt-224`                | 3B                | PaliGemmaForConditionalGeneration    | 4K      | Yes            |
| PaliGemma 3B Mix        | `google/paligemma-3b-mix-224`               | 3B                | PaliGemmaForConditionalGeneration    | 4K      | Yes            |
| PaliGemma2 3B           | `google/paligemma2-3b-ft-docci-448`         | 3B                | PaliGemmaForConditionalGeneration    | 4K      | Yes            |
| Phi-3.5 Vision          | `microsoft/Phi-3.5-vision-instruct`         | 4.2B              | Phi3VForCausalLM                     | 128K    | No             |
| Pixtral 12B             | `mistralai/Pixtral-12B-2409`                | 12B               | PixtralForConditionalGeneration      | 128K    | No             |
| Qwen2-Audio 7B          | `Qwen/Qwen2-Audio-7B-Instruct`              | 7B                | Qwen2AudioForConditionalGeneration   | 32K     | No             |
| QVQ 72B Preview         | `Qwen/QVQ-72B-Preview`                      | 72B               | Qwen2VLForConditionalGeneration      | 32K     | No             |
| Ultravox v0.3           | `fixie-ai/ultravox-v0_3`                    | 8B                | UltravoxModel                        | 4K      | No             |

### vLLM Pooling/Embedding Models

| Model                    | HuggingFace ID                           | Parameters | Architecture          | Type            |
|--------------------------|------------------------------------------|------------|-----------------------|-----------------|
| BGE Base EN v1.5         | `BAAI/bge-base-en-v1.5`                  | 110M       | BertModel             | Embedding       |
| BGE Multilingual Gemma2  | `BAAI/bge-multilingual-gemma2`           | 9B         | Gemma2Model           | Embedding       |
| Multilingual E5 Large    | `intfloat/multilingual-e5-large`         | 560M       | XLMRobertaModel       | Embedding       |
| RoBERTa Large v1         | `sentence-transformers/all-roberta-large-v1` | 355M   | RobertaModel          | Embedding       |
| Qwen2 7B Embed           | `ssmits/Qwen2-7B-Instruct-embed-base`    | 7B         | Qwen2Model            | Embedding       |
| VLM2Vec Full             | `TIGER-Lab/VLM2Vec-Full`                 | 4B         | Phi3VForCausalLM      | Multimodal Emb  |
| E5-V                     | `royokong/e5-v`                          | 7B         | LlavaNextForConditionalGen | Multimodal Emb |
| DSE Qwen2 2B             | `MrLight/dse-qwen2-2b-mrl-v1`            | 2B         | Qwen2VLForConditionalGen | Multimodal Emb |

### vLLM Reward/Classification Models

| Model                    | HuggingFace ID                           | Parameters | Architecture                      | Type            |
|--------------------------|------------------------------------------|------------|-----------------------------------|-----------------|
| InternLM2 1.8B Reward    | `internlm/internlm2-1_8b-reward`         | 1.8B       | InternLM2ForRewardModel           | Reward          |
| Math Shepherd 7B PRM     | `peiyi9979/math-shepherd-mistral-7b-prm` | 7B         | LlamaForCausalLM                  | Reward (PRM)    |
| Qwen2.5 Math PRM 7B      | `Qwen/Qwen2.5-Math-PRM-7B`               | 7B         | Qwen2ForProcessRewardModel        | Reward (PRM)    |
| Qwen2.5 Math PRM 72B     | `Qwen/Qwen2.5-Math-PRM-72B`              | 72B        | Qwen2ForProcessRewardModel        | Reward (PRM)    |
| Jamba Tiny Reward        | `ai21labs/Jamba-tiny-reward-dev`         | 900M       | JambaForSequenceClassification    | Classification  |
| MS Marco MiniLM L6       | `cross-encoder/ms-marco-MiniLM-L-6-v2`   | 23M        | BertForSequenceClassification     | Cross-Encoder   |
| Quora RoBERTa Base       | `cross-encoder/quora-roberta-base`       | 125M       | RobertaForSequenceClassification  | Cross-Encoder   |

---

## Model Status in OME

### Summary Statistics

| Category          | Total in SGLang | Configured in OME | Missing |
|-------------------|-----------------|-------------------|---------|
| Generative Models | ~100            | 55                | ~45     |
| Multimodal Models | ~25             | 18                | ~7      |
| Embedding Models  | 10              | 8                 | 2       |
| Reward Models     | 5               | 0                 | 5       |
| Rerank Models     | 1               | 1                 | 0       |


### Legend

- **Configured**: Model has a YAML configuration in `config/models/`
- **Missing**: Model is supported by SGLang but not yet configured in OME
- **Token Required**: Model requires HuggingFace token for access (gated model)

### HuggingFace Token Access

Models marked with "Token Required: Yes" are gated on HuggingFace and require:

1. A HuggingFace account
2. Acceptance of the model's license agreement
3. A HuggingFace token configured in the `key` field of the model spec

Example configuration for gated models:

```yaml
spec:
  storage:
    storageUri: hf://meta-llama/Llama-3.1-8B-Instruct
    path: /raid/models/meta/llama-3-1-8b-instruct
    key: "hf-token"  # Reference to secret containing HF token
```

---

## Architecture Reference

### Common Architecture Classes

| Architecture Class                  | Model Family                      | Type       |
|-------------------------------------|-----------------------------------|------------|
| `LlamaForCausalLM`                  | Llama, Solar, SmolLM              | Text       |
| `Llama4ForCausalLM`                 | Llama 4                           | Text (MoE) |
| `MistralForCausalLM`                | Mistral                           | Text       |
| `MixtralForCausalLM`                | Mixtral                           | Text (MoE) |
| `Qwen2ForCausalLM`                  | Qwen2.5, DeepSeek-R1-Distill-Qwen | Text       |
| `Qwen3ForCausalLM`                  | Qwen3                             | Text       |
| `Qwen3MoeForCausalLM`               | Qwen3 MoE                         | Text (MoE) |
| `GemmaForCausalLM`                  | Gemma 1                           | Text       |
| `Gemma2ForCausalLM`                 | Gemma 2                           | Text       |
| `Gemma3ForCausalLM`                 | Gemma 3 (text-only)               | Text       |
| `Gemma3ForConditionalGeneration`    | Gemma 3 (multimodal)              | VLM        |
| `PhiForCausalLM`                    | Phi-1.5, Phi-2                    | Text       |
| `Phi3ForCausalLM`                   | Phi-3, Phi-3.5, Phi-4             | Text       |
| `PhiMoEForCausalLM`                 | Phi-3.5 MoE                       | Text (MoE) |
| `DeepseekV2ForCausalLM`             | DeepSeek-V2                       | Text (MoE) |
| `DeepseekV3ForCausalLM`             | DeepSeek-V3, R1                   | Text (MoE) |
| `GraniteForCausalLM`                | Granite                           | Text       |
| `GraniteMoeForCausalLM`             | Granite MoE                       | Text (MoE) |
| `Starcoder2ForCausalLM`             | StarCoder2                        | Code       |
| `InternLM2ForCausalLM`              | InternLM2                         | Text       |
| `MllamaForConditionalGeneration`    | Llama 3.2 Vision                  | VLM        |
| `Qwen2VLForConditionalGeneration`   | Qwen2-VL                          | VLM        |
| `LlavaForConditionalGeneration`     | LLaVA                             | VLM        |
| `LlavaNextForConditionalGeneration` | LLaVA-NeXT                        | VLM        |
| `BertModel`                         | BGE                               | Embedding  |
| `XLMRobertaModel`                   | BGE-M3                            | Embedding  |
| `LlamaForSequenceClassification`    | Skywork-Reward-Llama              | Reward     |
| `Gemma2ForSequenceClassification`   | Skywork-Reward-Gemma              | Reward     |

---

## Model Capabilities

| Capability        | Description                    | Example Models                     |
|-------------------|--------------------------------|------------------------------------|
| TEXT_TO_TEXT      | Text generation and chat       | Llama, Qwen, Mistral, DeepSeek     |
| TEXT_TO_EMBEDDING | Text embeddings for search/RAG | E5, BGE, GTE, Qwen3-Embedding      |
| IMAGE_TO_TEXT     | Image understanding            | LLaVA, Qwen-VL, MiniCPM-V, Gemma 3 |
| TEXT_TO_IMAGE     | Image generation               | Janus-Pro (unified)                |
| AUDIO_TO_TEXT     | Audio understanding            | Phi-4 Multimodal                   |
| REWARD_SCORING    | RLHF reward scoring            | Skywork-Reward, InternLM2-Reward   |
| RERANKING         | Document reranking             | BGE-Reranker                       |

---

## References

### SGLang Documentation
- [SGLang Generative Models](https://docs.sglang.io/supported_models/generative_models.html)
- [SGLang Multimodal Models](https://docs.sglang.io/supported_models/multimodal_language_models.html)
- [SGLang Embedding Models](https://docs.sglang.io/supported_models/embedding_models.html)
- [SGLang Reward Models](https://docs.sglang.io/supported_models/reward_models.html)
- [SGLang Rerank Models](https://docs.sglang.io/supported_models/rerank_models.html)

### vLLM Documentation
- [vLLM Supported Models](https://docs.vllm.ai/en/latest/models/supported_models/)
- [vLLM Generative Models](https://docs.vllm.ai/en/latest/models/generative_models/)

### Other Resources
- [HuggingFace Model Hub](https://huggingface.co/models)
