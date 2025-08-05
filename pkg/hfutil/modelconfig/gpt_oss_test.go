package modelconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadGptOssConfig(t *testing.T) {
	// Create a temporary config file for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Sample GPT-OSS 20B configuration based on the provided config
	configData := map[string]interface{}{
		"architectures":           []string{"GptOssForCausalLM"},
		"model_type":              "gpt_oss",
		"transformers_version":    "4.55.0.dev0",
		"torch_dtype":             "bfloat16",
		"hidden_size":             2880,
		"intermediate_size":       2880,
		"num_hidden_layers":       24,
		"num_attention_heads":     64,
		"num_key_value_heads":     8,
		"max_position_embeddings": 131072,
		"vocab_size":              201088,
		"head_dim":                64,
		"num_local_experts":       32,
		"num_experts_per_tok":     4,
		"experts_per_token":       4,
		"output_router_logits":    false,
		"router_aux_loss_coef":    0.9,
		"eos_token_id":            200002,
		"pad_token_id":            199999,
		"hidden_act":              "silu",
		"rms_norm_eps":            0.00001,
		"rope_theta":              150000.0,
		"attention_dropout":       0.0,
		"attention_bias":          true,
		"sliding_window":          128,
		"swiglu_limit":            7.0,
		"initial_context_length":  4096,
		"layer_types": []string{
			"sliding_attention", "full_attention", "sliding_attention", "full_attention",
			"sliding_attention", "full_attention", "sliding_attention", "full_attention",
			"sliding_attention", "full_attention", "sliding_attention", "full_attention",
		},
		"rope_scaling": map[string]interface{}{
			"beta_fast":                        32.0,
			"beta_slow":                        1.0,
			"factor":                           32.0,
			"original_max_position_embeddings": 4096,
			"rope_type":                        "yarn",
			"truncate":                         false,
		},
		"quantization_config": map[string]interface{}{
			"modules_to_not_convert": []string{
				"model.layers.*.self_attn",
				"model.layers.*.mlp.router",
				"model.embed_tokens",
				"lm_head",
			},
			"quant_method": "mxfp4",
		},
		"initializer_range":   0.02,
		"use_cache":           true,
		"tie_word_embeddings": false,
	}

	// Write config to temporary file
	configBytes, err := json.MarshalIndent(configData, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, configBytes, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load the config
	config, err := LoadGptOssConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load GPT-OSS config: %v", err)
	}

	// Test basic properties
	if config.GetModelType() != "gpt_oss" {
		t.Errorf("Expected model type 'gpt_oss', got '%s'", config.GetModelType())
	}

	if config.GetArchitecture() != "GptOssForCausalLM" {
		t.Errorf("Expected architecture 'GptOssForCausalLM', got '%s'", config.GetArchitecture())
	}

	if config.HiddenSize != 2880 {
		t.Errorf("Expected hidden size 2880, got %d", config.HiddenSize)
	}

	if config.NumHiddenLayers != 24 {
		t.Errorf("Expected 24 hidden layers, got %d", config.NumHiddenLayers)
	}

	if config.NumLocalExperts != 32 {
		t.Errorf("Expected 32 local experts, got %d", config.NumLocalExperts)
	}

	if config.NumExpertsPerTok != 4 {
		t.Errorf("Expected 4 experts per token, got %d", config.NumExpertsPerTok)
	}

	if config.GetContextLength() != 131072 {
		t.Errorf("Expected context length 131072, got %d", config.GetContextLength())
	}

	// Test parameter count estimation
	paramCount := config.GetParameterCount()
	if paramCount != 20_000_000_000 {
		t.Errorf("Expected parameter count 20B, got %d", paramCount)
	}

	// Test quantization
	if config.GetQuantizationType() != "mxfp4" {
		t.Errorf("Expected quantization type 'mxfp4', got '%s'", config.GetQuantizationType())
	}

	// Test torch dtype
	if config.GetTorchDtype() != "bfloat16" {
		t.Errorf("Expected torch dtype 'bfloat16', got '%s'", config.GetTorchDtype())
	}

	// Test vision capability
	if config.HasVision() {
		t.Error("GPT-OSS should not have vision capabilities")
	}

	// Test model size estimation
	modelSize := config.GetModelSizeBytes()
	expectedSize := EstimateModelSizeBytes(paramCount, "bfloat16")
	if modelSize != expectedSize {
		t.Errorf("Expected model size %d bytes, got %d bytes", expectedSize, modelSize)
	}
}

func TestGptOssConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      GptOssConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: GptOssConfig{
				HiddenSize:            2880,
				NumHiddenLayers:       24,
				NumAttentionHeads:     64,
				NumKeyValueHeads:      8,
				VocabSize:             201088,
				MaxPositionEmbeddings: 131072,
				NumLocalExperts:       32,
				NumExpertsPerTok:      4,
			},
			expectError: false,
		},
		{
			name: "invalid hidden size",
			config: GptOssConfig{
				HiddenSize:            0,
				NumHiddenLayers:       24,
				NumAttentionHeads:     64,
				NumKeyValueHeads:      8,
				VocabSize:             201088,
				MaxPositionEmbeddings: 131072,
				NumLocalExperts:       32,
				NumExpertsPerTok:      4,
			},
			expectError: true,
		},
		{
			name: "invalid experts config",
			config: GptOssConfig{
				HiddenSize:            2880,
				NumHiddenLayers:       24,
				NumAttentionHeads:     64,
				NumKeyValueHeads:      8,
				VocabSize:             201088,
				MaxPositionEmbeddings: 131072,
				NumLocalExperts:       32,
				NumExpertsPerTok:      0,
				ExpertsPerToken:       0,
			},
			expectError: true,
		},
		{
			name: "invalid kv heads ratio",
			config: GptOssConfig{
				HiddenSize:            2880,
				NumHiddenLayers:       24,
				NumAttentionHeads:     8,
				NumKeyValueHeads:      64, // More KV heads than attention heads
				VocabSize:             201088,
				MaxPositionEmbeddings: 131072,
				NumLocalExperts:       32,
				NumExpertsPerTok:      4,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError && err == nil {
				t.Error("Expected validation error, but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
		})
	}
}

func TestGptOssHuggingFaceModelInterface(t *testing.T) {
	config := &GptOssConfig{
		BaseModelConfig: BaseModelConfig{
			ModelType:          "gpt_oss",
			Architectures:      []string{"GptOssForCausalLM"},
			TorchDtype:         "bfloat16",
			TransformerVersion: "4.55.0.dev0",
		},
		HiddenSize:            2880,
		NumHiddenLayers:       24,
		NumAttentionHeads:     64,
		NumKeyValueHeads:      8,
		MaxPositionEmbeddings: 131072,
		VocabSize:             201088,
		NumLocalExperts:       32,
		NumExpertsPerTok:      4,
		QuantizationConfig: &struct {
			ModulesToNotConvert []string `json:"modules_to_not_convert"`
			QuantMethod         string   `json:"quant_method"`
		}{
			QuantMethod: "mxfp4",
		},
	}

	// Test that it implements HuggingFaceModel interface
	var _ HuggingFaceModel = config

	// Test interface methods
	if config.GetParameterCount() <= 0 {
		t.Error("GetParameterCount should return positive value")
	}

	if config.GetTransformerVersion() != "4.55.0.dev0" {
		t.Errorf("Expected transformer version '4.55.0.dev0', got '%s'", config.GetTransformerVersion())
	}

	if config.GetQuantizationType() != "mxfp4" {
		t.Errorf("Expected quantization type 'mxfp4', got '%s'", config.GetQuantizationType())
	}

	if config.GetArchitecture() != "GptOssForCausalLM" {
		t.Errorf("Expected architecture 'GptOssForCausalLM', got '%s'", config.GetArchitecture())
	}

	if config.GetModelType() != "gpt_oss" {
		t.Errorf("Expected model type 'gpt_oss', got '%s'", config.GetModelType())
	}

	if config.GetContextLength() != 131072 {
		t.Errorf("Expected context length 131072, got %d", config.GetContextLength())
	}

	if config.GetModelSizeBytes() <= 0 {
		t.Error("GetModelSizeBytes should return positive value")
	}

	if config.GetTorchDtype() != "bfloat16" {
		t.Errorf("Expected torch dtype 'bfloat16', got '%s'", config.GetTorchDtype())
	}

	if config.HasVision() {
		t.Error("GPT-OSS should not have vision capabilities")
	}
}
