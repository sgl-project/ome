package services

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestEvaluateRuntimeCompatibility_UsesNestedSignals(t *testing.T) {
	service := &RuntimeIntelligenceService{}
	runtime := newRuntime(
		"vllm-llama-3-3-70b",
		map[string]any{
			"supportedModelFormats": []any{
				map[string]any{
					"modelFormat": map[string]any{
						"name":    "safetensors",
						"version": "1.0.0",
					},
					"modelFramework": map[string]any{
						"name":    "transformers",
						"version": "4.47.0.dev0",
					},
					"modelArchitecture": "LlamaForCausalLM",
					"autoSelect":        true,
				},
			},
			"modelSizeRange": map[string]any{
				"min": "60B",
				"max": "75B",
			},
			"protocolVersions": []any{"openAI"},
			"engineConfig": map[string]any{
				"runner": map[string]any{
					"image": "docker.io/vllm/vllm-openai:v0.9.0.1",
				},
			},
		},
	)

	match := service.evaluateRuntimeCompatibility(runtime, ModelProfile{
		Format:        "safetensors",
		Framework:     "transformers",
		Architecture:  "LlamaForCausalLM",
		ParameterSize: "70B",
	})

	if match.Score <= 0 {
		t.Fatalf("expected positive score, got %d", match.Score)
	}
	if match.Signals.MatchedFormat != "safetensors 1.0.0" {
		t.Fatalf("expected matched format to be parsed from nested modelFormat, got %q", match.Signals.MatchedFormat)
	}
	if match.Signals.MatchedFramework != "transformers 4.47.0.dev0" {
		t.Fatalf("expected matched framework to be parsed from nested modelFramework, got %q", match.Signals.MatchedFramework)
	}
	if match.Signals.MatchedArchitecture != "LlamaForCausalLM" {
		t.Fatalf("expected architecture signal, got %q", match.Signals.MatchedArchitecture)
	}
	if match.Signals.ModelSizeRange != "60B-75B" {
		t.Fatalf("expected size range signal, got %q", match.Signals.ModelSizeRange)
	}
	if match.Signals.RuntimeFamily != "vllm" {
		t.Fatalf("expected runtime family signal, got %q", match.Signals.RuntimeFamily)
	}
	if !containsString(match.Reasons, "Matches framework transformers 4.47.0.dev0") {
		t.Fatalf("expected framework reason, got %v", match.Reasons)
	}
	if !strings.Contains(match.Recommendation, "safetensors") {
		t.Fatalf("expected recommendation to mention matched format, got %q", match.Recommendation)
	}
}

func TestEvaluateRuntimeCompatibility_RejectsOutOfRangeModelSize(t *testing.T) {
	service := &RuntimeIntelligenceService{}
	runtime := newRuntime(
		"vllm-mistral-7b",
		map[string]any{
			"supportedModelFormats": []any{
				map[string]any{
					"modelFormat": map[string]any{
						"name": "safetensors",
					},
					"autoSelect": true,
				},
			},
			"modelSizeRange": map[string]any{
				"min": "7B",
				"max": "9B",
			},
		},
	)

	match := service.evaluateRuntimeCompatibility(runtime, ModelProfile{
		Format:        "safetensors",
		ParameterSize: "70B",
	})

	if match.Score != 0 {
		t.Fatalf("expected incompatible size to zero the score, got %d", match.Score)
	}
	if !containsString(match.Warnings, "Model size 70B is outside runtime range 7B-9B") {
		t.Fatalf("expected size-range warning, got %v", match.Warnings)
	}
}

func TestEvaluateRuntimeCompatibility_WarnsWhenAutoSelectDisabled(t *testing.T) {
	service := &RuntimeIntelligenceService{}
	runtime := newRuntime(
		"sglang-embedding",
		map[string]any{
			"supportedModelFormats": []any{
				map[string]any{
					"modelFormat": map[string]any{
						"name": "safetensors",
					},
					"autoSelect": false,
				},
			},
			"engineConfig": map[string]any{
				"runner": map[string]any{
					"image": "docker.io/lmsysorg/sglang:v0.5.5",
				},
			},
		},
	)

	match := service.evaluateRuntimeCompatibility(runtime, ModelProfile{
		Format: "safetensors",
	})

	if match.Score <= 0 {
		t.Fatalf("expected manual-select runtime to remain compatible, got %d", match.Score)
	}
	if match.Signals.AutoSelectEnabled {
		t.Fatalf("expected autoSelect signal to remain false")
	}
	if !containsString(match.Warnings, "Manual selection required because auto-select is disabled") {
		t.Fatalf("expected auto-select warning, got %v", match.Warnings)
	}
}

func newRuntime(name string, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "ome.io/v1beta1",
			"kind":       "ClusterServingRuntime",
			"metadata": map[string]any{
				"name": name,
			},
			"spec": spec,
		},
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
