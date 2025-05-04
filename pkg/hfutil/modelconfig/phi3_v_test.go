package modelconfig

import (
	"path/filepath"
	"testing"
)

func TestLoadPhi3VConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "phi3_v.json")

	// Load the config
	config, err := LoadModelConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load Phi3-Vision config: %v", err)
	}

	// Check that it's the correct model type
	if config.GetModelType() != "phi3_v" {
		t.Errorf("Expected model type 'phi3_v' but got '%s'", config.GetModelType())
	}

	// Check that it's parsed as a Phi3VConfig
	phi3Config, ok := config.(*Phi3VConfig)
	if !ok {
		t.Fatalf("Expected config to be of type *Phi3VConfig, but got %T", config)
	}

	// Check key fields
	if phi3Config.HiddenSize != 3072 {
		t.Errorf("Expected hidden size to be 3072, but got %d", phi3Config.HiddenSize)
	}

	if phi3Config.NumHiddenLayers != 32 {
		t.Errorf("Expected hidden layers to be 32, but got %d", phi3Config.NumHiddenLayers)
	}

	if phi3Config.MaxPositionEmbeddings != 131072 {
		t.Errorf("Expected max position embeddings to be 131072, but got %d", phi3Config.MaxPositionEmbeddings)
	}

	// Check vision capability
	if !config.HasVision() {
		t.Error("Expected HasVision to return true for Phi3-Vision, but got false")
	}

	// Check parameter count
	paramCount := config.GetParameterCount()
	expectedCount := int64(14_000_000_000) // 14B parameters
	if paramCount != expectedCount {
		t.Errorf("Expected parameter count to be %d, but got %d", expectedCount, paramCount)
	}

	// Check vision processing fields
	imgProcessor := phi3Config.ImgProcessor
	if imgProcessor == nil {
		t.Errorf("Expected ImgProcessor to be non-nil")
	} else {
		if imgProcessor["name"] != "clip_vision_model" {
			t.Errorf("Expected vision model name to be 'clip_vision_model', but got '%v'", imgProcessor["name"])
		}
	}

	// Check embedding layer fields
	embdLayer := phi3Config.EmbdLayer
	if embdLayer == nil {
		t.Errorf("Expected EmbdLayer to be non-nil")
	} else {
		if embdLayer["embedding_cls"] != "image" {
			t.Errorf("Expected embedding_cls to be 'image', but got '%v'", embdLayer["embedding_cls"])
		}
	}
}
