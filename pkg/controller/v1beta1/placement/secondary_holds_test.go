package placement

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/placement/allocation"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workloadcluster"
)

func TestSecondaryFloorHolds(t *testing.T) {
	serving := func(applied, next, ready int32) secondaryHome {
		return secondaryHome{Applied: applied, Next: next, Ready: ready, Known: true, Serving: true}
	}
	for _, tt := range []struct {
		name    string
		minimum int32
		homes   map[string]secondaryHome
		want    map[string]int32
	}{
		{name: "a falling floor keeps its ready instances while replacements start", minimum: 12,
			homes: map[string]secondaryHome{"member-a": serving(9, 7, 9), "member-b": serving(5, 5, 3)},
			want:  map[string]int32{"member-a": 9}},
		{name: "ready replacements release the old floor", minimum: 12,
			homes: map[string]secondaryHome{"member-a": serving(9, 7, 9), "member-b": serving(5, 5, 5)},
			want:  map[string]int32{}},
		{name: "a partly ready replacement releases part of the floor", minimum: 12,
			homes: map[string]secondaryHome{"member-a": serving(9, 7, 9), "member-b": serving(5, 5, 4)},
			want:  map[string]int32{"member-a": 8}},
		{name: "unready instances above the next floor leave at once", minimum: 12,
			homes: map[string]secondaryHome{"member-a": serving(9, 7, 6), "member-b": serving(5, 5, 3)},
			want:  map[string]int32{}},
		{name: "a lowered fleet minimum releases ready instances", minimum: 10,
			homes: map[string]secondaryHome{"member-a": serving(9, 8, 9), "member-b": serving(3, 2, 3)},
			want:  map[string]int32{}},
		{name: "an unobserved home keeps its applied floor", minimum: 12,
			homes: map[string]secondaryHome{"member-a": {Applied: 9, Next: 7}, "member-b": serving(5, 5, 5)},
			want:  map[string]int32{"member-a": 9}},
		{name: "ready instances on a home that does not serve release nothing", minimum: 12,
			homes: map[string]secondaryHome{"member-a": serving(9, 7, 9), "member-b": {Applied: 5, Next: 5, Ready: 5, Known: true}},
			want:  map[string]int32{"member-a": 9}},
		{name: "releases are granted in name order", minimum: 12,
			homes: map[string]secondaryHome{"member-a": serving(5, 4, 5), "member-b": serving(5, 4, 5), "member-c": serving(4, 4, 3)},
			want:  map[string]int32{"member-b": 5}},
		{name: "a rising floor is never held", minimum: 12,
			homes: map[string]secondaryHome{"member-a": serving(3, 5, 0)},
			want:  map[string]int32{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, secondaryFloorHolds(tt.minimum, tt.homes)); diff != "" {
				t.Errorf("holds (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSplitSecondaryHolds(t *testing.T) {
	type home struct {
		current, desired, decoder, engineReady, decoderReady int32
		drain                                                bool
	}
	decoderMember := func(engine, decoder int32) *v1beta1.InferenceService {
		member := ratioSplitSource(int(engine), ptr.To(int(decoder)))
		member.Spec.Placement = nil
		return member
	}
	for _, tt := range []struct {
		name   string
		source func() *v1beta1.InferenceService
		homes  map[string]home
		want   map[string]map[v1beta1.ComponentType]int32
	}{
		{name: "a re-apportioned floor keeps ready decoders before the engines move", homes: map[string]home{
			"member-a": {current: 3, desired: 3, decoder: 9, engineReady: 3, decoderReady: 9},
			"member-b": {current: 1, desired: 2, decoder: 3, engineReady: 1, decoderReady: 3},
		}, want: map[string]map[v1beta1.ComponentType]int32{"member-a": {v1beta1.DecoderComponent: 9}, "member-b": {v1beta1.DecoderComponent: 3}}},
		{name: "a moved floor keeps ready decoders until the replacements are ready", homes: map[string]home{
			"member-a": {current: 3, desired: 3, decoder: 9, engineReady: 3, decoderReady: 9},
			"member-b": {current: 2, desired: 2, decoder: 5, engineReady: 2, decoderReady: 3},
		}, want: map[string]map[v1beta1.ComponentType]int32{"member-a": {v1beta1.DecoderComponent: 9}}},
		{name: "ready replacements release the old home", homes: map[string]home{
			"member-a": {current: 3, desired: 3, decoder: 9, engineReady: 3, decoderReady: 9},
			"member-b": {current: 2, desired: 2, decoder: 5, engineReady: 2, decoderReady: 5},
		}, want: map[string]map[v1beta1.ComponentType]int32{}},
		{name: "a home asked to drain is not held", homes: map[string]home{
			"member-a": {current: 3, desired: 3, decoder: 9, engineReady: 3, decoderReady: 9},
			"member-b": {current: 1, desired: 2, decoder: 3, engineReady: 1, decoderReady: 3, drain: true},
		}, want: map[string]map[v1beta1.ComponentType]int32{"member-a": {v1beta1.DecoderComponent: 9}}},
		{name: "a service without decoders holds nothing", source: func() *v1beta1.InferenceService {
			source := ratioSplitSource(5, nil)
			source.Spec.Decoder = nil
			return source
		}, homes: map[string]home{
			"member-a": {current: 3, desired: 3, engineReady: 3},
			"member-b": {current: 1, desired: 2, engineReady: 1},
		}},
		{name: "Single placement holds nothing", source: func() *v1beta1.InferenceService {
			source := ratioSplitSource(5, ptr.To(12))
			source.Spec.Placement.Mode = v1beta1.PlacementModeSingle
			return source
		}, homes: map[string]home{
			"member-a": {current: 3, desired: 3, decoder: 9, engineReady: 3, decoderReady: 9},
			"member-b": {current: 1, desired: 2, decoder: 3, engineReady: 1, decoderReady: 3},
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := ratioSplitSource(5, ptr.To(12))
			if tt.source != nil {
				source = tt.source()
			}
			source.Spec.Placement.ClusterAffinity = weightedAffinity(map[string]int32{"member-a": 2, "member-b": 1})
			source.Status.Placement = &v1beta1.PlacementStatus{Plan: &v1beta1.PlacementPlanStatus{ID: "plan-a"}}
			observations := map[string]plannedHomeObservation{}
			for _, name := range []string{"member-a", "member-b"} {
				h := tt.homes[name]
				source.Status.Placement.Candidates = append(source.Status.Placement.Candidates, v1beta1.CandidatePlacement{Cluster: name,
					Allocation: &v1beta1.CandidateAllocationStatus{ClusterUID: types.UID("uid-" + name), CurrentReplicas: h.current, DesiredReplicas: h.desired, DrainRequested: h.drain}})
				observations[name] = plannedHomeObservation{
					Member:         decoderMember(h.current, h.decoder),
					ComponentReady: map[v1beta1.ComponentType]int32{v1beta1.EngineComponent: h.engineReady, v1beta1.DecoderComponent: h.decoderReady},
					Home:           allocation.Home{Known: true, Routable: true},
				}
			}
			got, err := splitSecondaryHolds(source, observations)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, got, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("holds (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHeldFloorsMessage(t *testing.T) {
	got := heldFloorsMessage(map[string]map[v1beta1.ComponentType]int32{
		"member-b": {v1beta1.DecoderComponent: 3},
		"member-a": {v1beta1.DecoderComponent: 9},
	})
	want := "member-a keeps 9 decoder replicas; member-b keeps 3 decoder replicas until their replacements on other clusters are ready"
	if got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestPlannedMemberHoldsRaiseSplitFloors(t *testing.T) {
	for _, tt := range []struct {
		name  string
		holds map[v1beta1.ComponentType]int32
		want  [2]int
	}{
		{name: "the apportioned floor applies without a hold", want: [2]int{4, 4}},
		{name: "a hold above the floor raises the band", holds: map[v1beta1.ComponentType]int32{v1beta1.DecoderComponent: 6}, want: [2]int{6, 6}},
		{name: "a hold below the floor is ignored", holds: map[v1beta1.ComponentType]int32{v1beta1.DecoderComponent: 3}, want: [2]int{4, 4}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := testScheme(t)
			source := plannedTestSource()
			source.Spec.Decoder = ratioSplitSource(2, ptr.To(4)).Spec.Decoder
			member := DeriveISVC(source, "", "")
			member.UID = "member-service-a"
			worker := emptyWorker(scheme)
			if err := worker.Create(ctx, member); err != nil {
				t.Fatal(err)
			}
			clusters := identifiedTestClusters{fakeClusters: fakeClusters{m: map[string]workloadcluster.SelectivelyCachingClient{"member-a": workloadcluster.NewNeverCachingClient(worker)}}, uid: "member-a-uid"}
			r, _ := newPlacer(scheme, clusters, source.DeepCopy(), plannedTestRegistration())
			if err := r.placePlannedMember(ctx, source, source.Status.Placement.Candidates[0], false, tt.holds); err != nil {
				t.Fatal(err)
			}
			stored := &v1beta1.InferenceService{}
			if err := worker.Get(ctx, client.ObjectKeyFromObject(member), stored); err != nil {
				t.Fatal(err)
			}
			got := [2]int{*stored.Spec.Decoder.MinReplicas, stored.Spec.Decoder.MaxReplicas}
			if got != tt.want {
				t.Errorf("decoder band = %v, want %v", got, tt.want)
			}
			if *stored.Spec.Engine.MinReplicas != 2 {
				t.Errorf("engine floor = %d, want 2", *stored.Spec.Engine.MinReplicas)
			}
		})
	}
}
