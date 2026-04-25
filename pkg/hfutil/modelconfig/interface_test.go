package modelconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatParamCount(t *testing.T) {
	testCases := []struct {
		count    int64
		expected string
	}{
		{0, "0"},
		{100, "100"},
		{999, "999"},
		{1000, "1K"},
		{1500, "1.5K"},
		{10000, "10K"},
		{10500, "10.5K"},
		{150000, "150K"},
		{151500, "151.5K"},
		{1000000, "1M"},
		{1500000, "1.5M"},
		{10000000, "10M"},
		{10500000, "10.5M"},
		{150000000, "150M"},
		{151500000, "151.5M"},
		{1000000000, "1B"},
		{1500000000, "1.5B"},
		{10000000000, "10B"},
		{10500000000, "10.5B"},
		{150000000000, "150B"},
		{151500000000, "151.5B"},
		{685000000000, "685B"},
		{1000000000000, "1T"},
		{1500000000000, "1.5T"},
		{1512300000000, "1.51T"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := FormatParamCount(tc.count)
			if result != tc.expected {
				t.Errorf("FormatParamCount(%d) = %s, expected %s", tc.count, result, tc.expected)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	testCases := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{999, "999 B"},
		{1000, "1000 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1024 * 10, "10.00 KB"},
		{1024*1024 - 1, "1024.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1.5, "1.50 MB"},
		{1024 * 1024 * 10, "10.00 MB"},
		{1024*1024*1024 - 1, "1024.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 1.5, "1.50 GB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
		{1024 * 1024 * 1024 * 1024 * 1.5, "1.50 TB"},
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			result := FormatSize(tc.bytes)
			if result != tc.expected {
				t.Errorf("FormatSize(%d) = %s, expected %s", tc.bytes, result, tc.expected)
			}
		})
	}
}

func TestGenericModelLoading(t *testing.T) {
	// Test cases for different model types
	testCases := []struct {
		name           string
		configFile     string
		expectedType   string
		expectedParams int64
		hasVision      bool
	}{
		{
			name:           "Llama",
			configFile:     "llama3.json",
			expectedType:   "llama",
			expectedParams: 70_000_000_000, // 70B (actual parameter count from llama3.json)
			hasVision:      false,
		},
		{
			name:           "Mistral",
			configFile:     "mistral.json",
			expectedType:   "mistral",
			expectedParams: 7_000_000_000, // 7B
			hasVision:      false,
		},
		{
			name:           "Mixtral",
			configFile:     "mixtral.json",
			expectedType:   "mixtral",
			expectedParams: 46_700_000_000, // Updated to match actual model params
			hasVision:      false,
		},
		{
			name:           "DeepSeek V3",
			configFile:     "deepseek_v3.json",
			expectedType:   "deepseek_v3",
			expectedParams: 685_000_000_000, // Updated to match actual model params
			hasVision:      false,
		},
		{
			name:           "Phi-3 Vision",
			configFile:     "phi3_v.json",
			expectedType:   "phi3_v",
			expectedParams: 14_000_000_000, // 14B
			hasVision:      true,
		},
		{
			name:           "Qwen2",
			configFile:     "qwen2_7b.json",
			expectedType:   "qwen2",
			expectedParams: 7_000_000_000, // 7B
			hasVision:      false,
		},
		{
			name:           "Mistral 7B Instruct",
			configFile:     "mistral_7b_instruct.json",
			expectedType:   "mistral",
			expectedParams: 7_000_000_000, // 7B
			hasVision:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := filepath.Join("testdata", tc.configFile)

			// Skip test if file doesn't exist
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				t.Skipf("Skipping test for %s: file %s not found", tc.name, configPath)
				return
			}

			// Load the model configuration
			config, err := LoadModelConfig(configPath)
			if err != nil {
				t.Fatalf("Failed to load model config for %s: %v", tc.name, err)
			}

			// Check model type
			modelType := config.GetModelType()
			if modelType != tc.expectedType {
				t.Errorf("Expected model type %s but got %s", tc.expectedType, modelType)
			}

			// Check parameter count
			paramCount := config.GetParameterCount()
			if paramCount != tc.expectedParams {
				t.Errorf("Expected parameter count %d but got %d", tc.expectedParams, paramCount)
			}

			// Check vision capabilities
			hasVision := config.HasVision()
			if hasVision != tc.hasVision {
				t.Errorf("Expected HasVision() to be %v but got %v", tc.hasVision, hasVision)
			}

			// Verify that non-zero values are returned
			contextLength := config.GetContextLength()
			if contextLength <= 0 {
				t.Errorf("Expected positive context length but got %d", contextLength)
			}

			modelSize := config.GetModelSizeBytes()
			if modelSize <= 0 {
				t.Errorf("Expected positive model size but got %d", modelSize)
			}

			// Verify transformers version is not empty
			transformersVersion := config.GetTransformerVersion()
			if transformersVersion == "" {
				t.Errorf("Expected non-empty transformers version")
			}
		})
	}
}

func TestUnsupportedModelType(t *testing.T) {
	configPath := filepath.Join("testdata", "clip_vision_model.json")

	// Verify the test file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: file %s not found", configPath)
		return
	}

	// Try to load the unsupported model type - should now succeed with generic config
	config, err := LoadModelConfig(configPath)

	// Verify that no error was returned (we now use generic fallback)
	if err != nil {
		t.Fatalf("Expected no error when loading unsupported model type with generic fallback, but got: %v", err)
	}

	// Verify we got a valid config back
	if config == nil {
		t.Fatalf("Expected a config but got nil")
	}

	// Check that the model type is preserved
	modelType := config.GetModelType()
	if modelType != "clip_vision_model" {
		t.Errorf("Expected model type 'clip_vision_model' but got '%s'", modelType)
	}

	// Verify that generic config returns reasonable values
	t.Logf("Generic config for unsupported model type: modelType=%s, params=%d, context=%d",
		config.GetModelType(), config.GetParameterCount(), config.GetContextLength())
}

func TestGenericConfigFallback(t *testing.T) {
	// Create a temporary config file with an unsupported model type
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Write a config with common fields but unsupported model type
	configJSON := `{
		"model_type": "falcon",
		"architectures": ["FalconForCausalLM"],
		"hidden_size": 4544,
		"num_hidden_layers": 32,
		"num_attention_heads": 71,
		"intermediate_size": 18176,
		"max_position_embeddings": 2048,
		"vocab_size": 65024,
		"torch_dtype": "bfloat16",
		"transformers_version": "4.30.0"
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load generic config: %v", err)
	}

	// Verify basic fields are parsed correctly
	if config.GetModelType() != "falcon" {
		t.Errorf("Expected model type 'falcon', got '%s'", config.GetModelType())
	}

	if config.GetArchitecture() != "FalconForCausalLM" {
		t.Errorf("Expected architecture 'FalconForCausalLM', got '%s'", config.GetArchitecture())
	}

	if config.GetContextLength() != 2048 {
		t.Errorf("Expected context length 2048, got %d", config.GetContextLength())
	}

	if config.GetTorchDtype() != "bfloat16" {
		t.Errorf("Expected torch_dtype 'bfloat16', got '%s'", config.GetTorchDtype())
	}

	if config.GetTransformerVersion() != "4.30.0" {
		t.Errorf("Expected transformers_version '4.30.0', got '%s'", config.GetTransformerVersion())
	}

	// Verify parameter estimation works (should be non-zero based on architecture)
	paramCount := config.GetParameterCount()
	if paramCount <= 0 {
		t.Errorf("Expected positive parameter count from estimation, got %d", paramCount)
	}
	t.Logf("Estimated parameter count for Falcon: %s (%d)", FormatParamCount(paramCount), paramCount)

	// Verify model size estimation works
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected positive model size, got %d", modelSize)
	}
	t.Logf("Estimated model size: %s", FormatSize(modelSize))
}

func TestGenericMultimodalFallback(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Nemotron-style config: top-level model_type with nested llm_config
	configJSON := `{
		"architectures": ["Reasoning_V3"],
		"model_type": "Reasoning_V3",
		"torch_dtype": "bfloat16",
		"max_sequence_length": 131072,
		"llm_config": {
			"architectures": ["CausalLM"],
			"model_type": "multi_h",
			"transformers_version": "4.55.4",
			"torch_dtype": "bfloat16",
			"hidden_size": 2688,
			"num_hidden_layers": 52,
			"num_attention_heads": 32,
			"intermediate_size": 1856,
			"max_position_embeddings": 262144,
			"vocab_size": 131072,
			"n_routed_experts": 128,
			"n_shared_experts": 1,
			"moe_intermediate_size": 1856
		},
		"vision_config": {
			"architectures": ["RADIOModel"],
			"max_resolution": 2048
		},
		"sound_config": {
			"model_type": "parakeet",
			"hidden_size": 1024
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.GetModelType() != "Reasoning_V3" {
		t.Errorf("Expected model type 'Reasoning_V3', got '%s'", config.GetModelType())
	}

	if config.GetTransformerVersion() != "4.55.4" {
		t.Errorf("Expected transformers_version '4.55.4', got '%s'", config.GetTransformerVersion())
	}

	// Context length should use max_position_embeddings from llm_config (262144)
	// since it's probed and takes priority over max_sequence_length
	if config.GetContextLength() != 262144 {
		t.Errorf("Expected context length 262144, got %d", config.GetContextLength())
	}

	if !config.HasVision() {
		t.Error("Expected HasVision() to return true")
	}

	if config.IsEmbedding() {
		t.Error("Expected IsEmbedding() to return false")
	}

	// Parameter count should use MoE estimation (~68.8B), not dense (~4.86B)
	paramCount := config.GetParameterCount()
	if paramCount < 60_000_000_000 || paramCount > 80_000_000_000 {
		t.Errorf("Expected MoE parameter count in range [60B, 80B], got %d (%s)",
			paramCount, FormatParamCount(paramCount))
	}
	t.Logf("Estimated parameter count: %s (%d)", FormatParamCount(paramCount), paramCount)
}

func TestGenericMultimodalFallback_TextConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	configJSON := `{
		"architectures": ["SomeVisionModel"],
		"model_type": "some_vision_model",
		"text_config": {
			"hidden_size": 4096,
			"num_hidden_layers": 32,
			"intermediate_size": 11008,
			"max_position_embeddings": 4096,
			"vocab_size": 32000,
			"transformers_version": "4.40.0",
			"torch_dtype": "float16"
		},
		"vision_config": {
			"hidden_size": 1024,
			"num_hidden_layers": 24
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.GetTransformerVersion() != "4.40.0" {
		t.Errorf("Expected transformers_version '4.40.0', got '%s'", config.GetTransformerVersion())
	}
	if config.GetContextLength() != 4096 {
		t.Errorf("Expected context length 4096, got %d", config.GetContextLength())
	}
	if config.GetTorchDtype() != "float16" {
		t.Errorf("Expected torch_dtype 'float16', got '%s'", config.GetTorchDtype())
	}
	if !config.HasVision() {
		t.Error("Expected HasVision() to return true")
	}
	if config.GetParameterCount() <= 0 {
		t.Error("Expected positive parameter count")
	}
}

func TestGenericMultimodalFallback_LanguageConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	configJSON := `{
		"architectures": ["SomeVLModel"],
		"model_type": "some_vl_model",
		"language_config": {
			"hidden_size": 2048,
			"num_hidden_layers": 24,
			"intermediate_size": 5504,
			"max_position_embeddings": 4096,
			"vocab_size": 100000,
			"transformers_version": "4.38.0"
		},
		"vision_config": {
			"hidden_size": 1152
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.GetTransformerVersion() != "4.38.0" {
		t.Errorf("Expected transformers_version '4.38.0', got '%s'", config.GetTransformerVersion())
	}
	if config.GetContextLength() != 4096 {
		t.Errorf("Expected context length 4096, got %d", config.GetContextLength())
	}
	if !config.HasVision() {
		t.Error("Expected HasVision() to return true")
	}
}

func TestGenericMoEFallback(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Flat MoE config (like Mixtral) with num_local_experts variant
	configJSON := `{
		"architectures": ["SomeMoEModel"],
		"model_type": "some_moe",
		"hidden_size": 4096,
		"num_hidden_layers": 32,
		"intermediate_size": 14336,
		"max_position_embeddings": 32768,
		"vocab_size": 32000,
		"torch_dtype": "bfloat16",
		"transformers_version": "4.36.0",
		"num_local_experts": 8,
		"moe_intermediate_size": 14336
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Should use MoE estimation with 8 experts
	paramCount := config.GetParameterCount()
	// Dense estimate would be ~12 * 4096^2 * 32 + embeddings ≈ 6.6B
	// MoE estimate with 8 experts should be much higher
	if paramCount < 30_000_000_000 {
		t.Errorf("Expected MoE parameter count > 30B, got %d (%s)",
			paramCount, FormatParamCount(paramCount))
	}
	t.Logf("Estimated MoE parameter count: %s (%d)", FormatParamCount(paramCount), paramCount)
}

func TestGenericContextLengthMaxSequenceLength(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")

	// Config with max_sequence_length but no max_position_embeddings at top level
	configJSON := `{
		"architectures": ["SomeModel"],
		"model_type": "some_model",
		"max_sequence_length": 131072,
		"hidden_size": 2048,
		"num_hidden_layers": 24,
		"vocab_size": 32000,
		"torch_dtype": "bfloat16",
		"transformers_version": "4.40.0"
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.GetContextLength() != 131072 {
		t.Errorf("Expected context length 131072 from max_sequence_length, got %d", config.GetContextLength())
	}

	if config.HasVision() {
		t.Error("Expected HasVision() to return false for non-multimodal config")
	}
}
