package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/sgl-project/ome/pkg/hfutil/modelconfig"
)

// Create a temporary example configuration
func createExampleConfig(tmpDir string, modelType string) (string, error) {
	var config map[string]interface{}

	switch modelType {
	case "llama":
		config = map[string]interface{}{
			"model_type":              "llama",
			"architectures":           []string{"LlamaForCausalLM"},
			"hidden_size":             4096,
			"intermediate_size":       11008,
			"num_hidden_layers":       32,
			"num_attention_heads":     32,
			"num_key_value_heads":     32,
			"max_position_embeddings": 4096,
			"vocab_size":              32000,
			"transformers_version":    "4.36.0",
			"torch_dtype":             "bfloat16",
		}
	case "mistral":
		config = map[string]interface{}{
			"model_type":              "mistral",
			"architectures":           []string{"MistralForCausalLM"},
			"hidden_size":             4096,
			"intermediate_size":       14336,
			"num_hidden_layers":       32,
			"num_attention_heads":     32,
			"num_key_value_heads":     8,
			"max_position_embeddings": 32768,
			"rope_theta":              1000000.0,
			"transformers_version":    "4.36.0",
			"torch_dtype":             "bfloat16",
		}
	case "phi3_v":
		config = map[string]interface{}{
			"model_type":              "phi3_v",
			"architectures":           []string{"Phi3VForCausalLM"},
			"hidden_size":             3072,
			"intermediate_size":       8192,
			"num_hidden_layers":       32,
			"num_attention_heads":     24,
			"max_position_embeddings": 131072,
			"img_processor": map[string]interface{}{
				"name": "clip_vision_model",
			},
			"embd_layer": map[string]interface{}{
				"embedding_cls": "image",
			},
			"transformers_version": "4.36.0",
			"torch_dtype":          "bfloat16",
		}
	}

	// Create filename
	filename := filepath.Join(tmpDir, modelType+".json")

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %v", err)
	}

	// Write to file
	if err := ioutil.WriteFile(filename, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write config file: %v", err)
	}

	return filename, nil
}

func main() {
	// Create a temporary directory for the example configs
	tmpDir, err := ioutil.TempDir("", "huggingface-config-examples")
	if err != nil {
		fmt.Printf("Error creating temporary directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Created temporary directory for examples: %s\n\n", tmpDir)

	// List of model types to demonstrate
	modelTypes := []string{
		"llama",
		"mistral",
		"phi3_v",
	}

	for _, modelType := range modelTypes {
		fmt.Printf("===== Loading model: %s =====\n", modelType)

		// Create an example config for this model type
		configPath, err := createExampleConfig(tmpDir, modelType)
		if err != nil {
			fmt.Printf("Error creating example config: %v\n", err)
			continue
		}

		// Load the model config
		config, err := modelconfig.LoadModelConfig(configPath)
		if err != nil {
			fmt.Printf("Error loading model config: %v\n", err)
			continue
		}

		// Use the common interface to access model information
		fmt.Printf("Model type: %s\n", config.GetModelType())
		fmt.Printf("Architecture: %s\n", config.GetArchitecture())
		fmt.Printf("Parameters: %s\n", modelconfig.FormatParamCount(config.GetParameterCount()))
		fmt.Printf("Context length: %d tokens\n", config.GetContextLength())
		fmt.Printf("Model size: %s\n", modelconfig.FormatSize(config.GetModelSizeBytes()))
		fmt.Printf("Has vision capabilities: %v\n", config.HasVision())
		fmt.Printf("Torch dtype: %s\n", config.GetTorchDtype())
		fmt.Printf("Transformers version: %s\n", config.GetTransformerVersion())
		fmt.Println()
	}
}
