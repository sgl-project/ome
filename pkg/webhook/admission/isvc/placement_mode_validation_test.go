package isvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/runtimeselector"
)

// placementModeFixedText opens every refusal of a placement mode change.
const placementModeFixedText = "spec.placement.mode is fixed for the life of the service"

// noPlacementBlockWord is how the refusal names a service without a
// spec.placement block.
const noPlacementBlockWord = "unset (no spec.placement block)"

// placementISVC is a service with the given placement block (nil for a
// service that lives on this cluster alone) and a runtime name, which is all
// the unrelated rules read.
func placementISVC(placement *v1beta1.PlacementSpec) *v1beta1.InferenceService {
	return &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: "test-isvc", Namespace: "default"},
		Spec: v1beta1.InferenceServiceSpec{
			Runtime:   &v1beta1.ServingRuntimeRef{Name: "test-runtime"},
			Placement: placement,
		},
	}
}

// placedIn is a valid placement block for one mode; the two Split modes
// carry the fleet count they distribute.
func placedIn(mode v1beta1.PlacementMode) *v1beta1.PlacementSpec {
	p := &v1beta1.PlacementSpec{Policy: v1beta1.PlacementPolicyClusterAffinity, Mode: mode}
	if mode == v1beta1.PlacementModeSplit || mode == v1beta1.PlacementModeSplitByCapacity {
		p.Split = &v1beta1.SplitSpec{Replicas: ptr.To[int32](2)}
	}
	return p
}

func placementValidator(t *testing.T) *InferenceServiceValidator {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1beta1.AddToScheme(scheme))
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	return &InferenceServiceValidator{Client: c, Reader: c, RuntimeSelector: runtimeselector.New(c)}
}

// TestInferenceService_PlacementModeChangeIsRefused: a running service keeps
// the placement mode it was created with. An update that names another mode,
// adds the placement block to a service created without one, or removes the
// block is refused, and the refusal names the mode the service has and the
// one the update asked for.
func TestInferenceService_PlacementModeChangeIsRefused(t *testing.T) {
	tests := []struct {
		name     string
		old, new *v1beta1.PlacementSpec
		from, to string
	}{
		{name: "All to Split", old: placedIn(v1beta1.PlacementModeAll), new: placedIn(v1beta1.PlacementModeSplit), from: "All", to: "Split"},
		{name: "Split to All", old: placedIn(v1beta1.PlacementModeSplit), new: placedIn(v1beta1.PlacementModeAll), from: "Split", to: "All"},
		{name: "Single to Split", old: placedIn(v1beta1.PlacementModeSingle), new: placedIn(v1beta1.PlacementModeSplit), from: "Single", to: "Split"},
		{name: "Split to SplitByCapacity", old: placedIn(v1beta1.PlacementModeSplit), new: placedIn(v1beta1.PlacementModeSplitByCapacity), from: "Split", to: "SplitByCapacity"},
		{name: "placement block added", old: nil, new: placedIn(v1beta1.PlacementModeSplit), from: noPlacementBlockWord, to: "Split"},
		{name: "placement block removed", old: placedIn(v1beta1.PlacementModeAll), new: nil, from: "All", to: noPlacementBlockWord},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := placementValidator(t).ValidateUpdate(context.Background(), placementISVC(tt.old), placementISVC(tt.new))
			require.Error(t, err)
			assert.Contains(t, err.Error(), placementModeFixedText)
			assert.Contains(t, err.Error(), "from "+tt.from+" to "+tt.to)
		})
	}
}

// TestInferenceService_PlacementModeKeptAdmitsOtherEdits: with the mode kept,
// the rest of the spec stays editable, the placement block included.
func TestInferenceService_PlacementModeKeptAdmitsOtherEdits(t *testing.T) {
	modes := []v1beta1.PlacementMode{"", v1beta1.PlacementModeSingle, v1beta1.PlacementModeAll, v1beta1.PlacementModeSplit, v1beta1.PlacementModeSplitByCapacity}
	for _, mode := range modes {
		name := string(mode)
		if mode == "" {
			name = "no placement block"
		}
		t.Run(name, func(t *testing.T) {
			var placement *v1beta1.PlacementSpec
			if mode != "" {
				placement = placedIn(mode)
			}
			oldIsvc := placementISVC(placement)
			newIsvc := oldIsvc.DeepCopy()
			newIsvc.Spec.Runtime = &v1beta1.ServingRuntimeRef{Name: "another-runtime"}
			if p := newIsvc.Spec.Placement; p != nil {
				p.ClusterAffinity = []v1beta1.ClusterAffinityTerm{{MatchFields: []v1beta1.ClusterSelectorRequirement{
					{Key: "metadata.name", Operator: corev1.NodeSelectorOpIn, Values: []string{"member-a"}},
				}}}
				if p.Split != nil {
					p.Split.Replicas = ptr.To[int32](3)
				}
			}
			_, err := placementValidator(t).ValidateUpdate(context.Background(), oldIsvc, newIsvc)
			require.NoError(t, err)
		})
	}
}

// TestInferenceService_PlacementModeCreateAdmitsEveryMode: the mode is the
// operator's choice at create.
func TestInferenceService_PlacementModeCreateAdmitsEveryMode(t *testing.T) {
	for _, mode := range []v1beta1.PlacementMode{v1beta1.PlacementModeSingle, v1beta1.PlacementModeAll, v1beta1.PlacementModeSplit, v1beta1.PlacementModeSplitByCapacity} {
		t.Run(string(mode), func(t *testing.T) {
			_, err := placementValidator(t).ValidateCreate(context.Background(), placementISVC(placedIn(mode)))
			require.NoError(t, err)
		})
	}
}
