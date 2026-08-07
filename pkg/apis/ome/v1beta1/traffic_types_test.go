package v1beta1

import (
	"encoding/json"
	"testing"
)

func TestTrafficSpec_OmitemptyShape(t *testing.T) {
	// An empty TrafficSpec should serialize to "{}", and a nil pointer
	// should be omitted from its parent — verifying the omitempty +
	// pointer wiring so older clients see nothing on ISVCs that don't
	// opt in to traffic policy.
	empty := TrafficSpec{}
	raw, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("Marshal empty: %v", err)
	}
	if string(raw) != `{}` {
		t.Fatalf("expected empty TrafficSpec to serialize to {}, got %s", string(raw))
	}

	type holder struct {
		Traffic *TrafficSpec `json:"traffic,omitempty"`
	}
	rawNil, err := json.Marshal(holder{})
	if err != nil {
		t.Fatalf("Marshal nil holder: %v", err)
	}
	if string(rawNil) != `{}` {
		t.Fatalf("expected nil Traffic to be omitted, got %s", string(rawNil))
	}
}
