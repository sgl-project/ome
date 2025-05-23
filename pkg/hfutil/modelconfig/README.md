# Hugging Face Model Configuration Parser

This package provides Go utilities for loading, parsing, and analyzing Hugging Face model configurations. It allows you to extract important information from model configuration files without needing to know the specific model type in advance.

## Features

- **Automatic model detection:**  
  Detects and loads configuration for any supported model type.
- **Model metadata extraction:**  
  Extracts key information, including:
  - Parameter count
  - Context window size
  - Model architecture
  - Vision capabilities (for multimodal models)
- **Extensible architecture:**  
  Easy to add support for new model families.
- **Accurate parameter counting:**  
  Handles complex cases such as Mixture of Experts (MoE) and multi-file safetensors models.
- **Comprehensive test coverage:**  
  Unit tests and real-world test data for all supported models.

## Supported Model Families

- LLaMA (including Llama-3, Llama-3.1, Llama-4, Maverick, Scout)
- Mistral & Mixtral (MoE)
- DeepSeek V3
- Phi-3, Phi-3 Vision
- Qwen2
- (See `future work` for upcoming model support)

## Usage

```go
import "path/to/hf_model_config"

config, err := hf_model_config.LoadConfig("path/to/config.json")
if err != nil {
    // handle error
}
fmt.Println("Parameters:", config.GetParameterCount())
fmt.Println("Context window:", config.GetContextLength())
fmt.Println("Architecture:", config.GetArchitecture())
```

See the `examples/` directory for more detailed usage patterns.

## Directory Structure

- `interface.go` – Central interface and loader logic
- `llama.go`, `llama4.go`, `mllama.go` – LLaMA family implementations
- `mistral.go`, `mixtral.go` – Mistral and Mixtral implementations
- `deepseek_v3.go` – DeepSeek V3 implementation
- `phi.go`, `phi3_v.go` – Phi family implementations
- `qwen2.go` – Qwen2 implementation
- `safetensors.go` – Utilities for parameter counting from safetensors files
- `*_test.go` – Unit tests
- `examples/` – Example code
- `testdata/` – Real model configuration files for testing

## Future Work

- **Add support for all GEMMA models**
  - `google/gemma-2b`
  - `google/gemma-2b-it`
  - `google/gemma-7b`
  - `google/gemma-7b-it`
  - `google/gemma-1.1-2b`
  - `google/gemma-1.1-2b-it`
  - `google/gemma-1.1-7b`
  - `google/gemma-1.1-7b-it`
  - *(Check for updates at [Hugging Face GEMMA models](https://huggingface.co/models?search=gemma))*

- **Add support for all Qwen3 models**
  - `Qwen/Qwen3-1.8B`
  - `Qwen/Qwen3-1.8B-Instruct`
  - `Qwen/Qwen3-4B`
  - `Qwen/Qwen3-4B-Instruct`
  - `Qwen/Qwen3-7B`
  - `Qwen/Qwen3-7B-Instruct`
  - `Qwen/Qwen3-14B`
  - `Qwen/Qwen3-14B-Instruct`
  - `Qwen/Qwen3-32B`
  - `Qwen/Qwen3-32B-Instruct`
  - `Qwen/Qwen3-72B`
  - `Qwen/Qwen3-72B-Instruct`
  - *(Check for updates at [Hugging Face Qwen models](https://huggingface.co/models?search=qwen))*

- Continue expanding support for new and emerging Hugging Face model architectures.
- Add more robust error handling and config validation for edge cases.
- Improve documentation and provide more real-world examples.
