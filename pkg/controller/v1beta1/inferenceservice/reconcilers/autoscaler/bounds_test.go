package autoscaler

import (
	"testing"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// RawEffectiveComponentBounds must mirror rawMinMaxReplicas' arithmetic
// (minus its KEDA precondition) so a rendered {{ .MinReplicas }} literal
// always equals the bound the Raw dispatch stamps on the scaler.
func TestRawEffectiveComponentBounds(t *testing.T) {
	cases := []struct {
		name string
		ext  *v1beta1.ComponentExtensionSpec
		want Bounds
	}{
		{"nil spec defaults", nil, Bounds{Min: 1, Max: 1}},
		{"omitted bounds default", &v1beta1.ComponentExtensionSpec{}, Bounds{Min: 1, Max: 1}},
		{"min zero is preserved", &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(0), MaxReplicas: 4}, Bounds{Min: 0, Max: 4}},
		{"min zero max zero clamps max to one", &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(0)}, Bounds{Min: 0, Max: 1}},
		{"max zero clamps up to min", &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(2)}, Bounds{Min: 2, Max: 2}},
		{"max below min clamps up", &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(3), MaxReplicas: 2}, Bounds{Min: 3, Max: 3}},
		{"declared band passes through", &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(2), MaxReplicas: 8}, Bounds{Min: 2, Max: 8}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := RawEffectiveComponentBounds(tc.ext); got != tc.want {
				t.Fatalf("RawEffectiveComponentBounds = %+v, want %+v", got, tc.want)
			}
		})
	}

	// Divergence pin: the shared bounds floor min to 1, the raw bounds keep 0.
	ext := &v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(0), MaxReplicas: 4}
	if got := EffectiveComponentBounds(ext); got.Min != 1 {
		t.Fatalf("EffectiveComponentBounds floors min to 1, got %+v", got)
	}
	if got := RawEffectiveComponentBounds(ext); got.Min != 0 {
		t.Fatalf("RawEffectiveComponentBounds must keep min 0, got %+v", got)
	}
}
