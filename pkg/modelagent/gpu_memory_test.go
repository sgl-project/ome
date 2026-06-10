package modelagent

import (
	"testing"
)

func TestLoadGPUShapeMemoryGBFromEnv_Unset(t *testing.T) {
	t.Setenv(GPUShapeMemoryGBEnvVar, "")
	m, err := loadGPUShapeMemoryGBFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// H100 default is the node-total (8 × 80 GiB).
	if got, want := m["H100"], int64(640); got != want {
		t.Errorf("H100 default = %d, want %d", got, want)
	}
}

func TestLoadGPUShapeMemoryGBFromEnv_OverrideRoundtrip(t *testing.T) {
	t.Setenv(GPUShapeMemoryGBEnvVar, `{"H100":1128,"A10":192}`)
	m, err := loadGPUShapeMemoryGBFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := m["H100"], int64(1128); got != want {
		t.Errorf("H100 override = %d, want %d", got, want)
	}
	// Default values are dropped when env var is set (we trust the operator's map).
	if _, present := m["B200"]; present {
		t.Errorf("expected B200 absent when override map provided")
	}
}

func TestLoadGPUShapeMemoryGBFromEnv_EmptyMapFallsBack(t *testing.T) {
	t.Setenv(GPUShapeMemoryGBEnvVar, `{}`)
	m, err := loadGPUShapeMemoryGBFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := m["H100"]; !ok {
		t.Errorf("empty override should fall back to defaults")
	}
}

func TestLoadGPUShapeMemoryGBFromEnv_BadJSON(t *testing.T) {
	t.Setenv(GPUShapeMemoryGBEnvVar, `{`)
	_, err := loadGPUShapeMemoryGBFromEnv()
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestAvailableNodeVRAMBytes(t *testing.T) {
	cases := []struct {
		name  string
		shape string
		want  int64
	}{
		{"H100 node (8×80)", "H100", 640 * (1 << 30)},
		{"A10 node (4×24)", "A10", 96 * (1 << 30)},
		{"H200 node (8×141)", "H200", 1128 * (1 << 30)},
		{"unknown shape fails open", "unknown-shape", 0},
		{"empty shape fails open", "", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AvailableNodeVRAMBytes(tc.shape)
			if got != tc.want {
				t.Errorf("AvailableNodeVRAMBytes(%q) = %d, want %d", tc.shape, got, tc.want)
			}
		})
	}
}
