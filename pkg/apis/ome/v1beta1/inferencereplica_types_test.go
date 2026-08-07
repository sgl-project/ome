package v1beta1

import (
	"encoding/json"
	"testing"
)

// TestParentReference_JSONShape pins the exact wire shape of the
// ParentReference sub-object so a future field rename doesn't silently
// break stored InferenceReplica objects.
func TestParentReference_JSONShape(t *testing.T) {
	pr := ParentReference{Name: "llama"}
	data, err := json.Marshal(&pr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"name":"llama"}`
	if string(data) != want {
		t.Errorf("ParentReference JSON shape changed:\n want %s\n got %s", want, string(data))
	}
}
