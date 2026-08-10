package components

import (
	"testing"
)

// TestConfigEnvVars_SortedDeterministic pins the env order derived from
// the router config map: map iteration order is randomized per run, and
// an order change churns the pod template hash, rolling the router
// deployment for no spec change.
func TestConfigEnvVars_SortedDeterministic(t *testing.T) {
	config := map[string]string{
		"ZED":       "3",
		"ALPHA":     "1",
		"MIDDLE":    "2",
		"OME_DEBUG": "true",
		"B_FLAG":    "x",
	}

	want := []string{"ALPHA", "B_FLAG", "MIDDLE", "OME_DEBUG", "ZED"}
	first := configEnvVars(config)
	if len(first) != len(want) {
		t.Fatalf("configEnvVars returned %d vars, want %d", len(first), len(want))
	}
	for i, name := range want {
		if first[i].Name != name {
			t.Errorf("env[%d].Name = %q, want %q (sorted key order)", i, first[i].Name, name)
		}
		if first[i].Value != config[name] {
			t.Errorf("env[%d].Value = %q, want %q", i, first[i].Value, config[name])
		}
	}

	// Repeated conversion of the same map must produce the identical
	// sequence — the property the pod template hash depends on.
	for run := 0; run < 20; run++ {
		again := configEnvVars(config)
		for i := range want {
			if again[i] != first[i] {
				t.Fatalf("run %d: env[%d] = %+v, want %+v (order must be stable)", run, i, again[i], first[i])
			}
		}
	}
}

// TestConfigEnvVars_Empty covers nil and empty maps.
func TestConfigEnvVars_Empty(t *testing.T) {
	if got := configEnvVars(nil); len(got) != 0 {
		t.Errorf("configEnvVars(nil) = %v, want empty", got)
	}
	if got := configEnvVars(map[string]string{}); len(got) != 0 {
		t.Errorf("configEnvVars(empty) = %v, want empty", got)
	}
}
