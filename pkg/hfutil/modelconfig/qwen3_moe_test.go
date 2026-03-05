package modelconfig

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestQwen3MoeConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen3_30b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen3Moe config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen3_moe" {
		t.Errorf("Expected model type 'qwen3_moe' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Qwen3Config
	qwen3MoeConfig, ok := config.(*Qwen3MoeConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *Qwen3MoeConfig, but got %T", config)
	}

	// Check key fields
	if qwen3MoeConfig.HiddenSize != 2048 {
		t.Errorf("Expected hidden size to be 2048, but got %d", qwen3MoeConfig.HiddenSize)
	}

	if qwen3MoeConfig.NumHiddenLayers != 48 {
		t.Errorf("Expected hidden layers to be 48, but got %d", qwen3MoeConfig.NumHiddenLayers)
	}

	if qwen3MoeConfig.NumAttentionHeads != 32 {
		t.Errorf("Expected attention heads to be 32, but got %d", qwen3MoeConfig.NumAttentionHeads)
	}

	if qwen3MoeConfig.NumKeyValueHeads != 4 {
		t.Errorf("Expected key-value heads to be 4, but got %d", qwen3MoeConfig.NumKeyValueHeads)
	}

	// Check context length (should use seq_length)
	contextLength := config.GetContextLength()
	expectedLength := 262144
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check parameter count (should be approximately 7B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(30_000_000_000) // 7B parameters
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check RoPE theta value (specific to Qwen3)
	if qwen3MoeConfig.RopeTheta != 10000000.0 {
		t.Errorf("Expected RoPE theta to be 5000000.0, but got %f", qwen3MoeConfig.RopeTheta)
	}

	// Check vision capability (should be false for this model)
	if config.HasVision() {
		t.Error("Expected HasVision to return false for Qwen3, but got true")
	}

	// Check quantization type (should be empty for non-quantized model)
	if config.GetQuantizationType() != "" {
		t.Errorf("Expected empty quantization type, but got '%s'", config.GetQuantizationType())
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}
}

func TestQwen3MoeQuantizationConfig(t *testing.T) {
	jsonData := []byte(`{
		"architectures": ["Qwen3MoeForCausalLM"],
		"model_type": "qwen3_moe",
		"hidden_size": 2048,
		"intermediate_size": 3072,
		"num_hidden_layers": 48,
		"num_attention_heads": 32,
		"num_key_value_heads": 4,
		"max_position_embeddings": 262144,
		"vocab_size": 151936,
		"num_experts": 128,
		"num_experts_per_tok": 8,
		"moe_intermediate_size": 768,
		"torch_dtype": "float8_e4m3fn",
		"quantization_config": {
			"activation_scheme": "dynamic",
			"fmt": "e4m3",
			"quant_method": "fp8",
			"weight_block_size": [128, 128]
		}
	}`)

	config := &Qwen3MoeConfig{}
	if err := json.Unmarshal(jsonData, config); err != nil {
		t.Fatalf("Failed to unmarshal Qwen3Moe FP8 config: %v", err)
	}

	if config.QuantizationConfig == nil {
		t.Fatal("Expected QuantizationConfig to be non-nil")
	}

	if config.GetQuantizationType() != "fp8" {
		t.Errorf("Expected quantization type 'fp8', but got '%s'", config.GetQuantizationType())
	}

	if config.QuantizationConfig.Format != "e4m3" {
		t.Errorf("Expected format 'e4m3', but got '%s'", config.QuantizationConfig.Format)
	}
}
