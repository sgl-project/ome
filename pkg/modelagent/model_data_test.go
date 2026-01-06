package modelagent

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

func TestConvertMetadataToModelConfig(t *testing.T) {
	// Helper to create a string pointer
	strPtr := func(s string) *string {
		return &s
	}

	tests := []struct {
		name     string
		metadata ModelMetadata
		expected *ModelConfig
	}{
		{
			name: "Complete metadata",
			metadata: ModelMetadata{
				ModelType:          "llama",
				ModelArchitecture:  "LlamaModel",
				ModelFramework:     &v1beta1.ModelFrameworkSpec{Name: "transformers", Version: strPtr("4.34.0")},
				ModelFormat:        v1beta1.ModelFormat{Name: "safetensors", Version: strPtr("1.0.0")},
				ModelParameterSize: "7.11B",
				MaxTokens:          4096,
				ModelCapabilities:  []string{"TEXT_GENERATION", "CHAT_COMPLETION"},
				ApiCapabilities:    []v1beta1.ModelAPICapability{v1beta1.ModelAPICapabilityOpenAIv1ChatCompletions},
				Quantization:       "FP16",
				DecodedModelConfiguration: map[string]interface{}{
					"hidden_size":         4096,
					"num_hidden_layers":   32,
					"num_attention_heads": 32,
				},
				Artifact: Artifact{
					Sha:           "123abc",
					ParentPath:    map[string]string{"parentModelName": "/path1"},
					ChildrenPaths: []string{"/path2", "/path3"},
				},
			},
			expected: &ModelConfig{
				ModelType:          "llama",
				ModelArchitecture:  "LlamaModel",
				ModelFramework:     map[string]string{"name": "transformers", "version": "4.34.0"},
				ModelFormat:        map[string]string{"name": "safetensors", "version": "1.0.0"},
				ModelParameterSize: "7.11B",
				MaxTokens:          4096,
				ModelCapabilities:  []string{"TEXT_GENERATION", "CHAT_COMPLETION"},
				ApiCapabilities:    []string{"OPENAI_V1_CHAT_COMPLETIONS"},
				Quantization:       "FP16",
				DecodedModelConfiguration: map[string]interface{}{
					"hidden_size":         4096,
					"num_hidden_layers":   32,
					"num_attention_heads": 32,
				},
				Artifact: Artifact{
					Sha:           "123abc",
					ParentPath:    map[string]string{"parentModelName": "/path1"},
					ChildrenPaths: []string{"/path2", "/path3"},
				},
			},
		},
		{
			name: "Minimal metadata",
			metadata: ModelMetadata{
				ModelType:          "mistral",
				ModelArchitecture:  "MistralModel",
				ModelParameterSize: "7B",
			},
			expected: &ModelConfig{
				ModelType:          "mistral",
				ModelArchitecture:  "MistralModel",
				ModelParameterSize: "7B",
			},
		},
		{
			name: "Metadata with ModelFramework but no version",
			metadata: ModelMetadata{
				ModelType:          "phi",
				ModelArchitecture:  "Phi3Model",
				ModelFramework:     &v1beta1.ModelFrameworkSpec{Name: "transformers"},
				ModelParameterSize: "3.8B",
				MaxTokens:          8192,
			},
			expected: &ModelConfig{
				ModelType:          "phi",
				ModelArchitecture:  "Phi3Model",
				ModelFramework:     map[string]string{"name": "transformers"},
				ModelParameterSize: "3.8B",
				MaxTokens:          8192,
			},
		},
		{
			name: "Metadata with ModelFormat but no version",
			metadata: ModelMetadata{
				ModelType:          "qwen",
				ModelArchitecture:  "QwenModel",
				ModelFormat:        v1beta1.ModelFormat{Name: "safetensors"},
				ModelParameterSize: "7B",
			},
			expected: &ModelConfig{
				ModelType:          "qwen",
				ModelArchitecture:  "QwenModel",
				ModelFormat:        map[string]string{"name": "safetensors"},
				ModelParameterSize: "7B",
			},
		},
		{
			name: "Metadata with raw ModelConfiguration to decode",
			metadata: ModelMetadata{
				ModelType:          "deepseek",
				ModelArchitecture:  "DeepseekModel",
				ModelParameterSize: "7B",
				ModelConfiguration: []byte(`{"vocab_size": 32000, "hidden_size": 4096, "context_length": 4096}`),
			},
			expected: &ModelConfig{
				ModelType:          "deepseek",
				ModelArchitecture:  "DeepseekModel",
				ModelParameterSize: "7B",
				DecodedModelConfiguration: map[string]interface{}{
					"vocab_size":     float64(32000),
					"hidden_size":    float64(4096),
					"context_length": float64(4096),
				},
			},
		},
		{
			name: "Metadata with Quantization",
			metadata: ModelMetadata{
				ModelType:          "mixtral",
				ModelArchitecture:  "MixtralModel",
				ModelParameterSize: "8x7B",
				Quantization:       "QINT8",
			},
			expected: &ModelConfig{
				ModelType:          "mixtral",
				ModelArchitecture:  "MixtralModel",
				ModelParameterSize: "8x7B",
				Quantization:       "QINT8",
			},
		},
		{
			name: "Metadata with minimum Artifact",
			metadata: ModelMetadata{
				ModelType:          "mixtral",
				ModelArchitecture:  "MixtralModel",
				ModelParameterSize: "8x7B",
				Artifact: Artifact{
					Sha: "123abc",
				},
			},
			expected: &ModelConfig{
				ModelType:          "mixtral",
				ModelArchitecture:  "MixtralModel",
				ModelParameterSize: "8x7B",
				Artifact: Artifact{
					Sha: "123abc",
				},
			},
		},
		{
			name: "Metadata with minimum Artifact2",
			metadata: ModelMetadata{
				ModelType:          "mixtral",
				ModelArchitecture:  "MixtralModel",
				ModelParameterSize: "8x7B",
				Artifact: Artifact{
					Sha:        "123abc",
					ParentPath: make(map[string]string),
				},
			},
			expected: &ModelConfig{
				ModelType:          "mixtral",
				ModelArchitecture:  "MixtralModel",
				ModelParameterSize: "8x7B",
				Artifact: Artifact{
					Sha:        "123abc",
					ParentPath: map[string]string{},
				},
			},
		},
		{
			name: "Metadata with minimum Artifact3",
			metadata: ModelMetadata{
				ModelType:          "mixtral",
				ModelArchitecture:  "MixtralModel",
				ModelParameterSize: "8x7B",
				Artifact: Artifact{
					Sha:           "123abc",
					ChildrenPaths: []string{},
				},
			},
			expected: &ModelConfig{
				ModelType:          "mixtral",
				ModelArchitecture:  "MixtralModel",
				ModelParameterSize: "8x7B",
				Artifact: Artifact{
					Sha:           "123abc",
					ChildrenPaths: []string{},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ConvertMetadataToModelConfig(tc.metadata)

			// Compare the result with the expected value
			if !reflect.DeepEqual(result, tc.expected) {
				resultJSON, _ := json.MarshalIndent(result, "", "  ")
				expectedJSON, _ := json.MarshalIndent(tc.expected, "", "  ")
				t.Errorf("ConvertMetadataToModelConfig() returned incorrect result.\nGot:\n%s\nWant:\n%s",
					string(resultJSON), string(expectedJSON))
			}
		})
	}
}

func TestModelEntryMarshaling(t *testing.T) {
	// Test model entry JSON marshaling and unmarshaling
	modelEntry := ModelEntry{
		Name:   "llama-70b",
		Status: ModelStatusReady,
		Config: &ModelConfig{
			ModelType:          "llama",
			ModelArchitecture:  "LlamaModel",
			ModelParameterSize: "70B",
			MaxTokens:          4096,
			ModelCapabilities:  []string{"TEXT_GENERATION", "CHAT_COMPLETION"},
			ApiCapabilities:    []string{string(v1beta1.ModelAPICapabilityOpenAIv1ChatCompletions)},
		},
	}

	// Marshal to JSON
	data, err := json.Marshal(modelEntry)
	if err != nil {
		t.Fatalf("Failed to marshal ModelEntry: %v", err)
	}

	// Unmarshal back
	var unmarshaled ModelEntry
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal ModelEntry: %v", err)
	}

	// Verify fields match
	if unmarshaled.Name != modelEntry.Name {
		t.Errorf("Name mismatch: got %s, want %s", unmarshaled.Name, modelEntry.Name)
	}
	if unmarshaled.Status != modelEntry.Status {
		t.Errorf("Status mismatch: got %s, want %s", unmarshaled.Status, modelEntry.Status)
	}
	if !reflect.DeepEqual(unmarshaled.Config, modelEntry.Config) {
		t.Errorf("ConfigAttr mismatch: got %+v, want %+v", unmarshaled.Config, modelEntry.Config)
	}
}

func TestModelStatusConstants(t *testing.T) {
	// Verify model status constants
	tests := []struct {
		status ModelStatus
		value  string
	}{
		{ModelStatusReady, "Ready"},
		{ModelStatusUpdating, "Updating"},
		{ModelStatusFailed, "Failed"},
		{ModelStatusDeleted, "Deleted"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			if string(tc.status) != tc.value {
				t.Errorf("Expected ModelStatus %s to equal %s", tc.status, tc.value)
			}
		})
	}
}
