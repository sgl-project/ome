package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestQwenConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "qwen_7b.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Qwen config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "qwen" {
		t.Errorf("Expected model type 'qwen' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a QwenConfig
	qwenConfig, ok := config.(*QwenConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *QwenConfig, but got %T", config)
	}

	// Check architecture
	expectedArch := "QWenLMHeadModel"
	if config.GetArchitecture() != expectedArch {
		t.Errorf("Expected architecture '%s' but got '%s'", expectedArch, config.GetArchitecture())
	}

	// Check key fields
	if qwenConfig.HiddenSize != 4096 {
		t.Errorf("Expected hidden size to be 4096, but got %d", qwenConfig.HiddenSize)
	}

	if qwenConfig.NumHiddenLayers != 32 {
		t.Errorf("Expected hidden layers to be 32, but got %d", qwenConfig.NumHiddenLayers)
	}

	if qwenConfig.NumAttentionHeads != 32 {
		t.Errorf("Expected attention heads to be 32, but got %d", qwenConfig.NumAttentionHeads)
	}

	if qwenConfig.KvChannels != 128 {
		t.Errorf("Expected kv_channels to be 128, but got %d", qwenConfig.KvChannels)
	}

	if qwenConfig.IntermediateSize != 22016 {
		t.Errorf("Expected intermediate size to be 22016, but got %d", qwenConfig.IntermediateSize)
	}

	if qwenConfig.VocabSize != 151936 {
		t.Errorf("Expected vocab size to be 151936, but got %d", qwenConfig.VocabSize)
	}

	// Check context length (should use seq_length)
	contextLength := config.GetContextLength()
	expectedLength := 8192
	if contextLength != expectedLength {
		t.Errorf("Expected context length to be %d, but got %d", expectedLength, contextLength)
	}

	// Check parameter count (should be approximately 7B)
	paramCount := config.GetParameterCount()
	expectedCount := int64(7_000_000_000) // 7B parameters
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check Qwen v1 specific fields
	if qwenConfig.RotaryEmbBase != 10000 {
		t.Errorf("Expected rotary_emb_base to be 10000, but got %f", qwenConfig.RotaryEmbBase)
	}

	if qwenConfig.RotaryPct != 1.0 {
		t.Errorf("Expected rotary_pct to be 1.0, but got %f", qwenConfig.RotaryPct)
	}

	if qwenConfig.LayerNormEpsilon != 1e-06 {
		t.Errorf("Expected layer_norm_epsilon to be 1e-06, but got %e", qwenConfig.LayerNormEpsilon)
	}

	if !qwenConfig.UseDynamicNTK {
		t.Error("Expected use_dynamic_ntk to be true, but got false")
	}

	if !qwenConfig.UseLogNAttn {
		t.Error("Expected use_logn_attn to be true, but got false")
	}

	if qwenConfig.UseFlashAttn != "auto" {
		t.Errorf("Expected use_flash_attn to be 'auto', but got %v", qwenConfig.UseFlashAttn)
	}

	if !qwenConfig.NoBias {
		t.Error("Expected no_bias to be true, but got false")
	}

	if !qwenConfig.ScaleAttnWeights {
		t.Error("Expected scale_attn_weights to be true, but got false")
	}

	if qwenConfig.TokenizerClass != "QWenTokenizer" {
		t.Errorf("Expected tokenizer_class to be 'QWenTokenizer', but got '%s'", qwenConfig.TokenizerClass)
	}

	// Check vision capability (should be false for this model)
	if config.HasVision() {
		t.Error("Expected HasVision to return false for Qwen v1, but got true")
	}

	// Check model size bytes (should be non-zero)
	modelSize := config.GetModelSizeBytes()
	if modelSize <= 0 {
		t.Errorf("Expected model size bytes to be positive, but got %d", modelSize)
	}

	// Check transformers version
	if config.GetTransformerVersion() != "4.32.0" {
		t.Errorf("Expected transformers version '4.32.0', but got '%s'", config.GetTransformerVersion())
	}

	// Check auto_map parsing
	if qwenConfig.AutoMap == nil {
		t.Error("Expected auto_map to be parsed, but it is nil")
	} else {
		expectedAutoConfig := "configuration_qwen.QWenConfig"
		if qwenConfig.AutoMap.AutoConfig != expectedAutoConfig {
			t.Errorf("Expected AutoConfig to be '%s', but got '%s'", expectedAutoConfig, qwenConfig.AutoMap.AutoConfig)
		}

		expectedAutoModel := "modeling_qwen.QWenLMHeadModel"
		if qwenConfig.AutoMap.AutoModelForCausalLM != expectedAutoModel {
			t.Errorf("Expected AutoModelForCausalLM to be '%s', but got '%s'", expectedAutoModel, qwenConfig.AutoMap.AutoModelForCausalLM)
		}
	}
}
