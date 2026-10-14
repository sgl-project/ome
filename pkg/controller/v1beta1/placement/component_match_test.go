package placement

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// componentSource declares per-component terms for an Engine, a Decoder and,
// when router is true, a Router.
func componentSource(mode v1beta1.PlacementMode, terms v1beta1.ComponentClusterAffinity, router bool) *v1beta1.InferenceService {
	source := isvcPlacement(&v1beta1.PlacementSpec{Policy: v1beta1.PlacementPolicyComponentAffinity, Mode: mode, ComponentAffinity: &terms})
	source.Spec.Engine = &v1beta1.EngineSpec{}
	source.Spec.Decoder = &v1beta1.DecoderSpec{}
	if router {
		source.Spec.Router = &v1beta1.RouterSpec{}
	}
	return source
}

func componentFleet() []v1beta1.WorkloadCluster {
	return []v1beta1.WorkloadCluster{
		wc("member-b", true, map[string]string{"accelerator": "gpu-a", "region": "east"}),
		wc("member-a", true, map[string]string{"accelerator": "gpu-a", "region": "west"}),
		wc("member-c", true, map[string]string{"accelerator": "gpu-b", "region": "west"}),
	}
}

func TestComponentAffinityMatchesThePrimaryTerms(t *testing.T) {
	for _, tt := range []struct {
		name    string
		source  func() *v1beta1.InferenceService
		want    []string
		wantErr string
	}{
		{name: "Single matches the engine's clusters", source: func() *v1beta1.InferenceService {
			return componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: testAffinity("accelerator=gpu-a"), Decoder: testAffinity("accelerator=gpu-a")}, false)
		}, want: []string{"member-a", "member-b"}},
		{name: "All matches the engine's clusters", source: func() *v1beta1.InferenceService {
			return componentSource(v1beta1.PlacementModeAll, v1beta1.ComponentClusterAffinity{Engine: testAffinity("region=west"), Decoder: testAffinity("region=west")}, false)
		}, want: []string{"member-a", "member-c"}},
		{name: "a decoder-only service matches the decoder's clusters", source: func() *v1beta1.InferenceService {
			source := componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Decoder: testAffinity("region=east")}, false)
			source.Spec.Engine = nil
			return source
		}, want: []string{"member-b"}},
		{name: "Split matches the engine's clusters", source: func() *v1beta1.InferenceService {
			return componentSource(v1beta1.PlacementModeSplit, v1beta1.ComponentClusterAffinity{Engine: testAffinity("accelerator=gpu-a"), Decoder: testAffinity("accelerator=gpu-a")}, false)
		}, want: []string{"member-a", "member-b"}},
		{name: "a Split router entry waits for controller support", source: func() *v1beta1.InferenceService {
			gpuA := testAffinity("accelerator=gpu-a")
			return componentSource(v1beta1.PlacementModeSplit, v1beta1.ComponentClusterAffinity{Engine: gpuA, Decoder: gpuA, Router: gpuA}, true)
		}, wantErr: "componentAffinity.router is not implemented for Split modes"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := MatchCandidates(tt.source(), componentFleet())
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("candidates (-want +got):\n%s", diff)
			}
		})
	}
}

func TestComponentClustersDiffer(t *testing.T) {
	gpuA := testAffinity("accelerator=gpu-a")
	for _, tt := range []struct {
		name    string
		source  *v1beta1.InferenceService
		edit    func([]v1beta1.WorkloadCluster)
		wantErr string
	}{
		{name: "the same terms match the same clusters",
			source: componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: gpuA, Decoder: gpuA}, false)},
		{name: "different terms that match the same clusters agree",
			source: componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: gpuA, Decoder: testAffinity("metadata.name in (member-a,member-b)")}, false)},
		{name: "a narrower decoder leaves a cluster with prefill but no decode",
			source:  componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: gpuA, Decoder: testAffinity("accelerator=gpu-a", "region=west")}, false),
			wantErr: "member-b matches engine but not decoder"},
		{name: "a broader router reaches a cluster without model servers",
			source:  componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: gpuA, Decoder: gpuA, Router: testAffinity("region=west")}, true),
			wantErr: "member-b matches engine and decoder but not router; member-c matches router but not engine and decoder"},
		{name: "a router without terms runs on every home",
			source: componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: gpuA, Decoder: gpuA}, true)},
		{name: "a deleted registration is not compared",
			source: componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: gpuA, Decoder: testAffinity("accelerator=gpu-a", "region=west")}, false),
			edit: func(clusters []v1beta1.WorkloadCluster) {
				now := metav1.Now()
				clusters[0].DeletionTimestamp = &now
			}},
		{name: "the shared list always agrees", source: isvcReq("accelerator=gpu-a", "")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			clusters := componentFleet()
			if tt.edit != nil {
				tt.edit(clusters)
			}
			err := componentClustersDiffer(tt.source, clusters)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestComponentSelectorsNeverReadAsUnconstrained(t *testing.T) {
	source := componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: testAffinity("accelerator=gpu-a")}, false)
	if _, err := componentSelectors(source); err == nil {
		t.Fatal("a declared decoder without terms compiled")
	}
	if _, err := placementSelector(source); err == nil {
		t.Fatal("a declared decoder without terms produced a selector")
	}
}

func TestClusterEventsReachEveryComponent(t *testing.T) {
	source := componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: testAffinity("region=west"), Decoder: testAffinity("region=west")}, true)
	source.Spec.Placement.ComponentAffinity.Router = testAffinity("accelerator=gpu-a")
	for _, tt := range []struct {
		name   string
		labels map[string]string
		want   bool
	}{
		{name: "a cluster only the router matches", labels: map[string]string{"accelerator": "gpu-a", "region": "east"}, want: true},
		{name: "a cluster the model components match", labels: map[string]string{"accelerator": "gpu-b", "region": "west"}, want: true},
		{name: "a cluster no component matches", labels: map[string]string{"accelerator": "gpu-b", "region": "east"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := clusterAffectsISVC(source, "member-x", labels.Set(tt.labels)); got != tt.want {
				t.Errorf("clusterAffectsISVC = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestPlacementInputConditionPerComponent(t *testing.T) {
	gpuA := testAffinity("accelerator=gpu-a")
	for _, tt := range []struct {
		name                         string
		source                       *v1beta1.InferenceService
		wantStatus                   corev1.ConditionStatus
		wantReason, wantMessagePiece string
	}{
		{name: "differing components hold the intent",
			source:     componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: gpuA, Decoder: testAffinity("region=east")}, false),
			wantStatus: corev1.ConditionFalse, wantReason: "ComponentClustersDiffer", wantMessagePiece: "member-a matches engine but not decoder"},
		{name: "an unmatched term is named by its component",
			source:     componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: gpuA, Decoder: append(testAffinity("accelerator=gpu-a"), testAffinity("accelerator=gpu-z")...)}, false),
			wantStatus: corev1.ConditionTrue, wantReason: "ValidPlacementIntent", wantMessagePiece: "componentAffinity.decoder terms [1] match no registered WorkloadCluster"},
		{name: "the shared list keeps its message",
			source:     isvcPlacement(&v1beta1.PlacementSpec{Policy: v1beta1.PlacementPolicyClusterAffinity, Mode: v1beta1.PlacementModeSingle, ClusterAffinity: append(testAffinity("accelerator=gpu-a"), testAffinity("accelerator=gpu-z")...)}),
			wantStatus: corev1.ConditionTrue, wantReason: "ValidPlacementIntent", wantMessagePiece: "; clusterAffinity terms [1] match no registered WorkloadCluster"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := withPlacementInputCondition(context.Background(), tt.source, componentFleet())
			got := placementInputForWrite(ctx, tt.source).cond
			if got.Status != tt.wantStatus || got.Reason != tt.wantReason || !strings.Contains(got.Message, tt.wantMessagePiece) {
				t.Errorf("condition = %s/%s %q, want %s/%s containing %q", got.Status, got.Reason, got.Message, tt.wantStatus, tt.wantReason, tt.wantMessagePiece)
			}
		})
	}
}

func TestMemberCopiesCarryOnePolicyAnnotation(t *testing.T) {
	gpuA := testAffinity("accelerator=gpu-a")
	shared := isvcReq("accelerator=gpu-a", "")
	shared.UID = "uid-1"
	perComponent := componentSource(v1beta1.PlacementModeSingle, v1beta1.ComponentClusterAffinity{Engine: gpuA, Decoder: gpuA}, false)
	perComponent.UID = "uid-1"
	for _, source := range []*v1beta1.InferenceService{shared, perComponent} {
		derived := DeriveISVC(source, "", "")
		if got := derived.Annotations["ome.io/placement-policy"]; got != string(v1beta1.PlacementPolicyClusterAffinity) {
			t.Errorf("policy %s: member annotation = %q, want ClusterAffinity", source.Spec.Placement.Policy, got)
		}
		if derived.Spec.Placement != nil {
			t.Errorf("policy %s: member copy keeps spec.placement", source.Spec.Placement.Policy)
		}
	}
}
