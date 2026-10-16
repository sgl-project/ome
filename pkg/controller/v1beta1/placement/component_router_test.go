package placement

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/placement/allocation"
)

// dividedRouterSource adds a router with its own terms to a per-component
// Split source, which divides router.minReplicas across the homes.
func dividedRouterSource(routers int, routerWeights map[string]int32) *v1beta1.InferenceService {
	twoToOne := map[string]int32{"member-a": 3, "member-b": 1}
	source := componentSplitSource(4, 12, twoToOne, twoToOne)
	source.Spec.Router = &v1beta1.RouterSpec{ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{MinReplicas: ptr.To(routers), MaxReplicas: 8}}
	source.Spec.Placement.ComponentAffinity.Router = weightedAffinity(routerWeights)
	return source
}

// routerFloors plans the source over both members, settles every home, and
// returns each home's router floor; a home without one reports -1.
func routerFloors(t *testing.T, source *v1beta1.InferenceService) (map[string]int32, error) {
	t.Helper()
	proposal, err := desiredSplitPlan(source, twoMembers(), nil)
	if err != nil {
		return nil, err
	}
	candidates := settledCandidates(proposal.Assignments)
	out := map[string]int32{}
	for _, candidate := range candidates {
		floors, err := splitHomeFloors(source, candidates, candidate)
		if err != nil {
			t.Fatalf("%s: %v", candidate.Cluster, err)
		}
		router, divided := floorMap(floors)[v1beta1.RouterComponent]
		if !divided {
			router = -1
		}
		out[candidate.Cluster] = router
	}
	return out, nil
}

func TestDividedRouterFloors(t *testing.T) {
	for _, tt := range []struct {
		name    string
		source  func() *v1beta1.InferenceService
		want    map[string]int32
		wantErr string
	}{
		{name: "routers with the engine's weights follow the engine shares", source: func() *v1beta1.InferenceService {
			return dividedRouterSource(3, map[string]int32{"member-a": 3, "member-b": 1})
		}, want: map[string]int32{"member-a": 2, "member-b": 1}},
		{name: "routers with their own weights split by them", source: func() *v1beta1.InferenceService {
			return dividedRouterSource(4, map[string]int32{"member-a": 1, "member-b": 1})
		}, want: map[string]int32{"member-a": 2, "member-b": 2}},
		{name: "fewer routers than engine homes holds the plan", source: func() *v1beta1.InferenceService {
			return dividedRouterSource(1, map[string]int32{"member-a": 3, "member-b": 1})
		}, wantErr: "router minimum 1 cannot give each of the 2 homes"},
		{name: "without router terms every home runs the full router policy", source: func() *v1beta1.InferenceService {
			source := dividedRouterSource(3, map[string]int32{"member-a": 3, "member-b": 1})
			source.Spec.Placement.ComponentAffinity.Router = nil
			return source
		}, want: map[string]int32{"member-a": -1, "member-b": -1}},
		{name: "split replicas scale the router minimum in the declared ratio", source: func() *v1beta1.InferenceService {
			source := dividedRouterSource(4, map[string]int32{"member-a": 3, "member-b": 1})
			source.Spec.Placement.Split = &v1beta1.SplitSpec{Replicas: ptr.To[int32](8)}
			return source
		}, want: map[string]int32{"member-a": 6, "member-b": 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := routerFloors(t, tt.source())
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
				t.Errorf("router floors (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDividedRouterMemberBounds(t *testing.T) {
	for _, tt := range []struct {
		name    string
		ceiling int32
		want    map[string][2]int
	}{
		{name: "routers are pinned at their share", want: map[string][2]int{"member-a": {2, 2}, "member-b": {1, 1}}},
		{name: "a local ceiling scales by the router ratio", ceiling: 3, want: map[string][2]int{"member-a": {2, 3}, "member-b": {1, 3}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			source := dividedRouterSource(3, map[string]int32{"member-a": 3, "member-b": 1})
			if tt.ceiling > 0 {
				source.Spec.Placement.Split = &v1beta1.SplitSpec{MaxReplicasPerCluster: tt.ceiling}
			}
			proposal, err := desiredSplitPlan(source, twoMembers(), nil)
			if err != nil {
				t.Fatal(err)
			}
			candidates := settledCandidates(proposal.Assignments)
			got := map[string][2]int{}
			for _, candidate := range candidates {
				bounds, err := splitPlannedBounds(source, candidates, candidate)
				if err != nil {
					t.Fatal(err)
				}
				member := source.DeepCopy()
				setPlannedReplicas(member, bounds)
				got[candidate.Cluster] = [2]int{*member.Spec.Router.MinReplicas, member.Spec.Router.MaxReplicas}
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("router bounds (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDividedRouterHoldsUntilReplacementsAreReady(t *testing.T) {
	source := dividedRouterSource(4, map[string]int32{"member-a": 3, "member-b": 1})
	proposal, err := desiredSplitPlan(source, twoMembers(), nil)
	if err != nil {
		t.Fatal(err)
	}
	source.Status.Placement = &v1beta1.PlacementStatus{Plan: &v1beta1.PlacementPlanStatus{ID: "plan-a"}, Candidates: settledCandidates(proposal.Assignments)}
	// The routers move from 2,2 to 3,1 while member-a's third router starts.
	member := func(routers int) *v1beta1.InferenceService {
		copied := source.DeepCopy()
		copied.Spec.Placement = nil
		copied.Spec.Router.MinReplicas = ptr.To(routers)
		return copied
	}
	observations := map[string]plannedHomeObservation{
		"member-a": {Member: member(2), ComponentReady: map[v1beta1.ComponentType]int32{v1beta1.RouterComponent: 2}, Home: allocation.Home{Known: true, Routable: true}},
		"member-b": {Member: member(2), ComponentReady: map[v1beta1.ComponentType]int32{v1beta1.RouterComponent: 2}, Home: allocation.Home{Known: true, Routable: true}},
	}
	got, err := splitSecondaryHolds(source, observations)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[v1beta1.ComponentType]int32{"member-b": {v1beta1.RouterComponent: 2}}
	if diff := cmp.Diff(want, got, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("holds (-want +got):\n%s", diff)
	}
	observations["member-a"] = plannedHomeObservation{Member: member(3), ComponentReady: map[v1beta1.ComponentType]int32{v1beta1.RouterComponent: 3}, Home: allocation.Home{Known: true, Routable: true}}
	if got, err = splitSecondaryHolds(source, observations); err != nil || len(got) > 0 {
		t.Errorf("holds after the replacement is ready = %v, %v; want none", got, err)
	}
}

func TestDividedRouterTermsMustMatchTheSameClusters(t *testing.T) {
	source := dividedRouterSource(3, map[string]int32{"member-a": 3, "member-b": 1})
	source.Spec.Placement.ComponentAffinity.Router = testAffinity("metadata.name in (member-a)")
	if err := componentClustersDiffer(source, twoMembers()); err == nil || !strings.Contains(err.Error(), "member-b matches engine and decoder but not router") {
		t.Fatalf("error = %v, want member-b named", err)
	}
}
