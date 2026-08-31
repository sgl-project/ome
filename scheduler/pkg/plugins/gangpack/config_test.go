package gangpack

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestDecodeArgs(t *testing.T) {
	configured, err := decodeArgs(&runtime.Unknown{Raw: []byte(`{
		"topologyKey":"topology.ome.io/domain",
		"podGroupTopologyKeyAnnotation":"workloads.example/topology-key",
		"unsupportedPlacementGroupLabel":"workloads.example/placement-group",
		"defaultPermitTimeoutSeconds":90,
		"podGroupSyncTimeoutSeconds":15,
		"gcIntervalSeconds":45
	}`)})
	if err != nil {
		t.Fatalf("decodeArgs: %v", err)
	}
	if configured.TopologyKey != "topology.ome.io/domain" {
		t.Fatalf("TopologyKey = %q", configured.TopologyKey)
	}
	if configured.PodGroupTopologyKeyAnnotation != "workloads.example/topology-key" ||
		configured.UnsupportedPlacementGroupLabel != "workloads.example/placement-group" {
		t.Fatalf("metadata keys = %q/%q", configured.PodGroupTopologyKeyAnnotation, configured.UnsupportedPlacementGroupLabel)
	}
	if got := configured.defaultPermitTimeout(); got != 90*time.Second {
		t.Fatalf("defaultPermitTimeout = %v, want 90s", got)
	}
	if got := configured.podGroupSyncTimeout(); got != 15*time.Second {
		t.Fatalf("podGroupSyncTimeout = %v, want 15s", got)
	}
	if got := configured.gcInterval(); got != 45*time.Second {
		t.Fatalf("gcInterval = %v, want 45s", got)
	}

	if _, err := decodeArgs(nil); err == nil {
		t.Fatal("decodeArgs(nil) succeeded, want required gcIntervalSeconds error")
	}
}

func TestDecodeArgsRejectsInvalidValues(t *testing.T) {
	for _, raw := range []string{
		`{"topologyKey":"not a label key/"}`,
		`{"podGroupTopologyKeyAnnotation":"not a key/"}`,
		`{"unsupportedPlacementGroupLabel":"not a key/"}`,
		`{"defaultPermitTimeoutSeconds":0}`,
		`{"defaultPermitTimeoutSeconds":-1}`,
		`{"podGroupSyncTimeoutSeconds":0}`,
		`{"gcIntervalSeconds":0}`,
		`{"gcIntervlSeconds":45}`,
		`{}`,
	} {
		if _, err := decodeArgs(&runtime.Unknown{Raw: []byte(raw)}); err == nil {
			t.Errorf("decodeArgs(%s) succeeded, want error", raw)
		}
	}
}
