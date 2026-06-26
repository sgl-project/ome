package runtimerevision

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestHash_NilSpec_Error(t *testing.T) {
	if _, _, err := Hash(nil); err == nil {
		t.Fatal("expected error on nil spec")
	}
}

func TestHash_Deterministic(t *testing.T) {
	spec := &v1beta1.ServingRuntimeSpec{
		Disabled: ptr.To(true),
		SupportedModelFormats: []v1beta1.SupportedModelFormat{{
			ModelFormat:    &v1beta1.ModelFormat{Name: "safetensors"},
			ModelFramework: &v1beta1.ModelFrameworkSpec{Name: "transformers"},
		}},
	}
	full1, short1, err := Hash(spec)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	full2, short2, err := Hash(spec.DeepCopy())
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if full1 != full2 || short1 != short2 {
		t.Fatalf("non-deterministic hash: %s vs %s", full1, full2)
	}
	if len(short1) != 8 {
		t.Fatalf("short hash length = %d, want 8", len(short1))
	}
	if len(full1) != 64 {
		t.Fatalf("full hash length = %d, want 64", len(full1))
	}
}

func TestHash_DifferentSpecs_DifferentHash(t *testing.T) {
	spec1 := &v1beta1.ServingRuntimeSpec{
		EngineConfig: &v1beta1.EngineSpec{Runner: &v1beta1.RunnerSpec{
			Container: corev1.Container{Name: "ome-container", Image: "a"},
		}},
	}
	spec2 := &v1beta1.ServingRuntimeSpec{
		EngineConfig: &v1beta1.EngineSpec{Runner: &v1beta1.RunnerSpec{
			Container: corev1.Container{Name: "ome-container", Image: "b"},
		}},
	}
	_, s1, _ := Hash(spec1)
	_, s2, _ := Hash(spec2)
	if s1 == s2 {
		t.Fatal("different specs hashed to same short hash")
	}
}

func TestHash_MapOrderingStable(t *testing.T) {
	// Hash relies on encoding/json sorting string-keyed map keys.
	// Two semantically-equal specs declared with different map
	// iteration orders must produce the same hash.
	specA := &v1beta1.ServingRuntimeSpec{
		ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
			NodeSelector: map[string]string{"a": "1", "b": "2", "c": "3"},
		},
	}
	specB := &v1beta1.ServingRuntimeSpec{
		ServingRuntimePodSpec: v1beta1.ServingRuntimePodSpec{
			NodeSelector: map[string]string{"c": "3", "a": "1", "b": "2"},
		},
	}
	_, sA, _ := Hash(specA)
	_, sB, _ := Hash(specB)
	if sA != sB {
		t.Fatalf("map ordering changed hash: %s vs %s", sA, sB)
	}
}
