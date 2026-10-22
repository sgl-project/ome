package allocation

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func serving(count int32) MemberState {
	return MemberState{Known: true, Eligible: true, Applied: true, Routable: true, Ready: count, Occupied: count}
}

func moving() Transition {
	budget := int32(1)
	return Transition{
		From: Plan{Targets: map[string]int32{"a": 4}}, Current: map[string]int32{"a": 4},
		Desired: Plan{Targets: map[string]int32{"b": 4}}, Surge: int64(budget),
		Members: map[string]MemberState{"a": serving(4), "b": {Known: true, Eligible: true}},
	}
}

func advance(t *testing.T, in Transition) Step {
	t.Helper()
	before := maps.Clone(in.Current)
	step, err := Advance(in)
	require.NoError(t, err)
	if diff := cmp.Diff(before, in.Current); diff != "" {
		t.Fatalf("planning must not mutate persisted targets (-want +got):\n%s", diff)
	}
	return step
}

func TestTransitionWaitsForReadyRoutableReplacementAndRemoval(t *testing.T) {
	in := moving()
	for next := int32(1); next <= 4; next++ {
		step := advance(t, in)
		if diff := cmp.Diff(map[string]int32{"a": 5 - next, "b": next}, step.Targets); diff != "" {
			t.Fatalf("result mismatch (-want +got):\n%s", diff)
		}
		in.Current = step.Targets
		in.Members["b"] = MemberState{Known: true, Eligible: true, Applied: true, Occupied: next}
		step = advance(t, in)
		if diff := cmp.Diff(in.Current, step.Targets); diff != "" {
			t.Fatalf("admission without readiness cannot release the donor (-want +got):\n%s", diff)
		}
		in.Members["b"] = MemberState{Known: true, Eligible: true, Applied: true, Occupied: next, Ready: next}
		step = advance(t, in)
		if diff := cmp.Diff(in.Current, step.Targets); diff != "" {
			t.Fatalf("readiness without routability cannot release the donor (-want +got):\n%s", diff)
		}
		in.Members["b"] = serving(next)
		step = advance(t, in)
		if next == 4 {
			if diff := cmp.Diff([]string{"a"}, step.Drain); diff != "" {
				t.Fatalf("result mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(in.Current, step.Targets); diff != "" {
				t.Fatalf("traffic must drain before a zero-target member is removed (-want +got):\n%s", diff)
			}
			member := in.Members["a"]
			member.Drained = true
			member.Routable = false
			in.Members["a"] = member
			step = advance(t, in)
		}
		if diff := cmp.Diff(map[string]int32{"a": 4 - next, "b": next}, step.Targets); diff != "" {
			t.Fatalf("result mismatch (-want +got):\n%s", diff)
		}
		in.Current = step.Targets
		step = advance(t, in)
		if diff := cmp.Diff(in.Current, step.Targets); diff != "" {
			t.Fatalf("a requested reduction is not released capacity (-want +got):\n%s", diff)
		}
		require.False(t, step.Complete)
		in.Members["a"] = serving(4 - next)
	}
	step := advance(t, in)
	require.True(t, step.Complete)
	if diff := cmp.Diff(int32(4), in.From.Targets["a"]); diff != "" {
		t.Fatalf("the original allocation survives the entire move (-want +got):\n%s", diff)
	}
}

func TestTransitionSurgeIsSharedWithRollout(t *testing.T) {
	tests := []struct {
		name      string
		allowance int64
		rollout   int32
		want      Step
	}{
		{name: "rollout consumes allowance", allowance: 1, rollout: 1,
			want: Step{Targets: map[string]int32{"a": 4, "b": 0}, Reason: "SurgeBudgetExhausted"}},
		{name: "released allowance permits growth", allowance: 1,
			want: Step{Targets: map[string]int32{"a": 4, "b": 1}, Reason: "AwaitingMemberConvergence"}},
		{name: "zero forbids excess", allowance: 0,
			want: Step{Targets: map[string]int32{"a": 4, "b": 0}, Reason: "SurgeBudgetExhausted"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := moving()
			in.Surge, in.RolloutReserved = tt.allowance, tt.rollout
			got := advance(t, in)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("step mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTransitionRetargetDoesNotManufactureSurge(t *testing.T) {
	in := moving()
	in.Current = advance(t, in).Targets
	in.Members["b"] = serving(1)
	in.Desired = Plan{Targets: map[string]int32{"c": 4}}
	in.Members["c"] = MemberState{Known: true, Eligible: true}
	step := advance(t, in)
	if diff := cmp.Diff([]string{"b"}, step.Drain); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
	compareTargets(t, in.Current, step.Targets)
	in.Members["b"] = MemberState{Known: true, Applied: true, Drained: true, Ready: 1, Occupied: 1}
	step = advance(t, in)
	if diff := cmp.Diff(map[string]int32{"a": 4, "b": 0, "c": 0}, step.Targets); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
	in.Current = step.Targets
	step = advance(t, in)
	require.Zero(t, step.Targets["c"], "a terminating surplus still occupies the allowance")
	in.Members["b"] = MemberState{Known: true, Absent: true}
	step = advance(t, in)
	if diff := cmp.Diff(int32(1), step.Targets["c"]); diff != "" {
		t.Fatalf("observed removal permits progress to the new destination (-want +got):\n%s", diff)
	}
}

func TestTransitionRetargetPreservesEarlierReplacement(t *testing.T) {
	in := moving()
	in.Current = map[string]int32{"a": 3, "b": 1}
	in.Members["a"], in.Members["b"] = serving(3), serving(1)
	in.Desired = Plan{Targets: map[string]int32{"c": 4}}
	in.Members["c"] = MemberState{Known: true, Eligible: true}
	step := advance(t, in)
	require.Empty(t, step.Drain, "the abandoned destination still backs an original replica")
	if diff := cmp.Diff(map[string]int32{"a": 3, "b": 1, "c": 1}, step.Targets); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
	in.Current = step.Targets
	in.Members["c"] = serving(1)
	step = advance(t, in)
	if diff := cmp.Diff([]string{"b"}, step.Drain); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(int32(3), step.Targets["a"]); diff != "" {
		t.Fatalf("drain and reduction cannot spend the same replacement (-want +got):\n%s", diff)
	}
}

func TestTransitionCancelsUnadmittedSurplus(t *testing.T) {
	in := moving()
	in.Current["b"] = 1
	in.Members["b"] = MemberState{Known: true, Applied: true, Occupied: 1}
	in.Desired = Plan{Targets: map[string]int32{"c": 4}}
	in.Members["c"] = MemberState{Known: true, Eligible: true}
	step := advance(t, in)
	if diff := cmp.Diff([]string{"b"}, step.Drain); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
	member := in.Members["b"]
	member.Drained = true
	in.Members["b"] = member
	step = advance(t, in)
	if diff := cmp.Diff(map[string]int32{"a": 4, "b": 0, "c": 0}, step.Targets); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
}

func TestTransitionUnknownMemberPreservesAllocations(t *testing.T) {
	in := moving()
	in.Members["a"] = MemberState{}
	step := advance(t, in)
	compareTargets(t, in.Current, step.Targets)
	if diff := cmp.Diff("ObservationUnknown", step.Reason); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
	in.Members["a"] = MemberState{Known: true, Absent: true}
	step = advance(t, in)
	require.Zero(t, step.Targets["a"], "authoritative absence can release the old request")
	require.Zero(t, step.Targets["b"], "release and growth remain separate steps")
	in.Current = step.Targets
	step = advance(t, in)
	if diff := cmp.Diff(int32(4), step.Targets["b"]); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
}

func TestTransitionInitialPlacementAndExplicitScaleDown(t *testing.T) {
	in := Transition{
		Desired: Plan{Targets: map[string]int32{"a": 2, "b": 2, "c": 0}},
		Members: map[string]MemberState{"a": {Known: true, Eligible: true}, "b": {Known: true, Eligible: true}},
	}
	step := advance(t, in)
	if diff := cmp.Diff(in.Desired.Targets, step.Targets); diff != "" {
		t.Fatalf("initial placement needs no migration surplus (-want +got):\n%s", diff)
	}
	in.Current = step.Targets
	in.Members["a"], in.Members["b"] = serving(2), serving(2)
	require.True(t, advance(t, in).Complete)
	in.From = in.Desired
	in.Desired = Plan{Targets: map[string]int32{"a": 1, "b": 1}}
	step = advance(t, in)
	if diff := cmp.Diff(map[string]int32{"a": 1, "b": 1, "c": 0}, step.Targets); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
	in.Current = step.Targets
	step = advance(t, in)
	require.True(t, step.Complete, "allocation completion is distinct from member admission")
}

func TestTransitionUnassignedFloorRetainsExistingMember(t *testing.T) {
	in := moving()
	in.Desired = Plan{Unassigned: 4}
	step := advance(t, in)
	compareTargets(t, in.Current, step.Targets)
	require.Empty(t, step.Drain)
	require.False(t, step.Complete)
}

func TestTransitionDrainCreditCannotBeSpentTwice(t *testing.T) {
	budget := int32(1)
	in := Transition{
		From:    Plan{Targets: map[string]int32{"a": 1, "b": 1}},
		Current: map[string]int32{"a": 1, "b": 1, "c": 1},
		Desired: Plan{Targets: map[string]int32{"c": 2}}, Surge: int64(budget),
		Members: map[string]MemberState{"a": serving(1), "b": serving(1), "c": serving(1)},
	}
	step := advance(t, in)
	if diff := cmp.Diff([]string{"a"}, step.Drain); diff != "" {
		t.Fatalf("one ready replacement cannot drain two old members (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(in.Current, step.Targets); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
}

func TestTransitionWaitsForEligibilityAndAppliedTargets(t *testing.T) {
	in := moving()
	in.Members["b"] = MemberState{Known: true}
	step := advance(t, in)
	compareTargets(t, in.Current, step.Targets)
	in.Current["b"] = 1
	in.Members["b"] = MemberState{Known: true, Eligible: true, Occupied: 1, Ready: 1, Routable: true}
	step = advance(t, in)
	if diff := cmp.Diff(in.Current, step.Targets); diff != "" {
		t.Fatalf("an observation from another applied plan cannot fund removal (-want +got):\n%s", diff)
	}
	in.Desired = Plan{Targets: maps.Clone(in.Current)}
	require.False(t, advance(t, in).Complete, "a member must acknowledge its persisted request")
}

func TestTransitionResumedMemberWaitsForServingReplacement(t *testing.T) {
	for _, planner := range []struct {
		name    string
		advance func(Transition) (Step, error)
	}{
		{name: "replica steps", advance: Advance},
		{name: "whole members", advance: AdvanceWholeMembers},
	} {
		t.Run(planner.name, func(t *testing.T) {
			for _, tt := range []struct {
				name    string
				applied bool
				routed  bool
				want    Step
			}{
				{name: "awaiting publication", applied: true,
					want: Step{Targets: map[string]int32{"a": 4, "b": 4}, Reason: "ReplacementNotReady"}},
				{name: "awaiting application", routed: true,
					want: Step{Targets: map[string]int32{"a": 4, "b": 4}, Reason: "ReplacementNotReady"}},
				{name: "verified serving", applied: true, routed: true,
					want: Step{Targets: map[string]int32{"a": 4, "b": 4}, Drain: []string{"b"}, Reason: "AwaitingMemberConvergence"}},
			} {
				t.Run(tt.name, func(t *testing.T) {
					in := moving()
					in.Current["b"], in.Members["b"] = 4, serving(4)
					in.Desired = Plan{Targets: map[string]int32{"a": 4}}
					in.Members["a"] = MemberState{Known: true, Applied: tt.applied, Routable: tt.routed, Ready: 4, Occupied: 4}
					got, err := planner.advance(in)
					require.NoError(t, err)
					if diff := cmp.Diff(tt.want, got); diff != "" {
						t.Fatalf("step (-want +got):\n%s", diff)
					}
				})
			}
		})
	}
}

func TestTransitionUnpublishedReadyMemberIsNotSurplus(t *testing.T) {
	for _, planner := range []struct {
		name    string
		advance func(Transition) (Step, error)
	}{
		{name: "replica steps", advance: Advance},
		{name: "whole members", advance: AdvanceWholeMembers},
	} {
		t.Run(planner.name, func(t *testing.T) {
			// The first plan never completed, so the serving member has no original
			// floor. Losing its publication must not turn it into surplus.
			in := Transition{
				Current: map[string]int32{"a": 3, "b": 3}, Desired: Plan{Targets: map[string]int32{"b": 3}}, Surge: 3,
				Members: map[string]MemberState{
					"a": {Known: true, Applied: true, Ready: 3, Occupied: 3},
					"b": {Known: true, Eligible: true, Applied: true, Occupied: 3},
				},
			}
			got, err := planner.advance(in)
			require.NoError(t, err)
			want := Step{Targets: map[string]int32{"a": 3, "b": 3}, Reason: "ReplacementNotReady"}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("unpublished member (-want +got):\n%s", diff)
			}
			in.Members["b"] = serving(3)
			got, err = planner.advance(in)
			require.NoError(t, err)
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("serving replacement cannot retire an unpublished member (-want +got):\n%s", diff)
			}
			in.Members["a"] = serving(3)
			got, err = planner.advance(in)
			require.NoError(t, err)
			want.Drain, want.Reason = []string{"a"}, "AwaitingMemberConvergence"
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("published serving replacement (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTransitionAccountsForUnplannedOccupancy(t *testing.T) {
	in := moving()
	in.Members["c"] = serving(1)
	step := advance(t, in)
	require.Zero(t, step.Targets["b"], "owned instances outside the plan still consume surge")
	require.Contains(t, step.Targets, "c", "an unplanned member must receive a zero target")
	if diff := cmp.Diff([]string{"c"}, step.Drain); diff != "" {
		t.Fatalf("unplanned member must drain (-want +got):\n%s", diff)
	}
	in.Members["c"] = MemberState{Occupied: 1}
	if diff := cmp.Diff("ObservationUnknown", advance(t, in).Reason); diff != "" {
		t.Fatalf("result mismatch (-want +got):\n%s", diff)
	}
}

func TestTransitionProtectsReplicasDuringInitialRetarget(t *testing.T) {
	tests := []struct {
		name    string
		from    Plan
		current map[string]int32
		members map[string]MemberState
		want    map[string]int32
	}{
		{name: "first full allocation", current: map[string]int32{"a": 4}, members: map[string]MemberState{"a": serving(4)}, want: map[string]int32{"a": 4, "b": 1}},
		{name: "first serving replica", current: map[string]int32{"a": 1}, members: map[string]MemberState{"a": serving(1)}, want: map[string]int32{"a": 1, "b": 4}},
		{name: "previously unassigned floor", from: Plan{Unassigned: 4}, current: map[string]int32{"a": 4}, members: map[string]MemberState{"a": serving(4)}, want: map[string]int32{"a": 4, "b": 1}},
		{name: "unplanned serving replica", members: map[string]MemberState{"a": serving(1)}, want: map[string]int32{"a": 0, "b": 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := moving()
			in.From, in.Current, in.Members = tt.from, tt.current, tt.members
			in.Members["b"] = MemberState{Known: true, Eligible: true}
			got := advance(t, in)
			want := Step{Targets: tt.want, Reason: "AwaitingMemberConvergence"}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatalf("retarget must grow before draining (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTransitionRejectsInvalidAccounting(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Transition)
	}{
		{"negative unassigned", func(in *Transition) { in.From.Unassigned = -1 }},
		{"negative desired", func(in *Transition) { in.Desired.Targets["b"] = -1 }},
		{"negative current", func(in *Transition) { in.Current["a"] = -1 }},
		{"empty target name", func(in *Transition) { in.Current[""] = 1 }},
		{"negative allowance", func(in *Transition) { in.Surge = -1 }},
		{"negative rollout reservation", func(in *Transition) { in.RolloutReserved = -1 }},
		{"ready exceeds occupied", func(in *Transition) { in.Members["a"] = MemberState{Known: true, Ready: 5, Occupied: 4} }},
		{"absent but occupied", func(in *Transition) { in.Members["a"] = MemberState{Known: true, Absent: true, Occupied: 1} }},
		{"drained but routable", func(in *Transition) { in.Members["a"] = MemberState{Known: true, Drained: true, Routable: true} }},
		{"empty member name", func(in *Transition) { in.Members[""] = MemberState{} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, planner := range []struct {
				name    string
				advance func(Transition) (Step, error)
			}{
				{name: "replica steps", advance: Advance},
				{name: "whole members", advance: AdvanceWholeMembers},
			} {
				t.Run(planner.name, func(t *testing.T) {
					in := moving()
					tt.change(&in)
					got, err := planner.advance(in)
					require.Error(t, err)
					if diff := cmp.Diff(Step{}, got); diff != "" {
						t.Fatalf("invalid accounting returned a step (-want +got):\n%s", diff)
					}
				})
			}
		})
	}
}

func TestTransitionsConvergeWithinSharedBudget(t *testing.T) {
	names := []string{"a", "b", "c", "d"}
	for scenario := range 1000 {
		t.Run(fmt.Sprintf("scenario_%04d", scenario), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(uint64(scenario), 31))
			partition := func(total int32) Plan {
				plan := Plan{Targets: map[string]int32{}}
				for range total {
					plan.Targets[names[rng.IntN(len(names))]]++
				}
				return plan
			}
			floor := int32(1 + rng.IntN(30))
			budget := int32(1 + rng.IntN(5))
			in := Transition{From: partition(floor), Desired: partition(floor), Surge: int64(budget),
				RolloutReserved: int32(rng.IntN(int(budget))), Members: map[string]MemberState{}}
			in.Current = maps.Clone(in.From.Targets)
			if scenario%2 == 0 {
				in.From = Plan{Unassigned: floor}
			}
			for _, name := range names {
				in.Members[name] = serving(in.Current[name])
			}
			complete := false
			for iteration := range 256 {
				if iteration == 3 || iteration == 7 {
					in.Desired = partition(floor)
				}
				step := advance(t, in)
				for _, name := range step.Resume {
					member := in.Members[name]
					member.Drained, member.Routable = false, true
					in.Members[name] = member
				}
				var requested, ready int32
				for _, name := range names {
					requested += step.Targets[name]
					member := in.Members[name]
					if member.Routable {
						ready += min(member.Ready, step.Targets[name])
					}
				}
				for _, name := range step.Drain {
					member := in.Members[name]
					if member.Routable {
						ready -= min(member.Ready, step.Targets[name])
					}
					member.Drained, member.Routable = true, false
					in.Members[name] = member
				}
				require.LessOrEqual(t, requested+in.RolloutReserved, floor+budget, "scenario %d iteration %d", scenario, iteration)
				require.GreaterOrEqual(t, ready, floor, "a move or drain must retain the serving floor: scenario %d iteration %d: input %+v step %+v", scenario, iteration, in, step)
				if step.Complete {
					compareTargets(t, in.Desired.Targets, step.Targets)
					complete = true
					break
				}
				require.True(t, len(step.Drain) > 0 || len(step.Resume) > 0 || !cmp.Equal(step.Targets, in.Current), "transition stalled: scenario %d iteration %d: %+v", scenario, iteration, in)
				for _, name := range names {
					if step.Targets[name] != in.Current[name] {
						in.Members[name] = serving(step.Targets[name])
					}
				}
				in.Current = step.Targets
			}
			require.True(t, complete, "scenario %d did not converge", scenario)
		})
	}
}

func compareTargets(t *testing.T, want, got map[string]int32) {
	t.Helper()
	want, got = maps.Clone(want), maps.Clone(got)
	maps.DeleteFunc(want, func(_ string, value int32) bool { return value == 0 })
	maps.DeleteFunc(got, func(_ string, value int32) bool { return value == 0 })
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("target mismatch (-want +got):\n%s", diff)
	}
}

func TestMemberViewPredicatesReadOneCluster(t *testing.T) {
	applied := MemberState{Known: true, Eligible: true, Applied: true}
	for _, tt := range []struct {
		name   string
		member memberView
		want   map[string]bool
	}{
		{"an unknown member with an original target is standing", memberView{original: 2}, map[string]bool{"unknownStanding": true}},
		{"an unknown member with nothing is not standing", memberView{}, map[string]bool{}},
		{"a drained member both plans keep resumes", memberView{current: 2, goal: 2, state: MemberState{Known: true, Drained: true}}, map[string]bool{"needsResume": true, "unsettled": true}},
		{"a target above both plans is surplus", memberView{original: 1, current: 3, goal: 2, state: applied}, map[string]bool{"aboveBothPlans": true, "reducible": true}},
		{"occupied replicas under no target must drain", memberView{state: MemberState{Known: true, Occupied: 2}}, map[string]bool{"occupiedWithoutTarget": true, "unsettled": true}},
		{"a ready routable member under a zero target still serves", memberView{state: MemberState{Known: true, Routable: true, Ready: 1, Occupied: 1}}, map[string]bool{"routableWithoutTarget": true, "occupiedWithoutTarget": true, "unsettled": true}},
		{"an empty eligible member grows without an applied target", memberView{goal: 3, state: MemberState{Known: true, Eligible: true}}, map[string]bool{"growable": true}},
		{"a member with a target grows only once it is applied", memberView{current: 1, goal: 3, state: MemberState{Known: true, Eligible: true}}, map[string]bool{"unsettled": true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := map[string]bool{
				"unknownStanding":       tt.member.unknownStanding(),
				"needsResume":           tt.member.needsResume(),
				"aboveBothPlans":        tt.member.aboveBothPlans(),
				"occupiedWithoutTarget": tt.member.occupiedWithoutTarget(),
				"routableWithoutTarget": tt.member.routableWithoutTarget(),
				"reducible":             tt.member.reducible(),
				"growable":              tt.member.growable(),
				"unsettled":             tt.member.unsettled(),
			}
			for name, value := range got {
				if value != tt.want[name] {
					t.Errorf("%s = %v, want %v", name, value, tt.want[name])
				}
			}
		})
	}
}

// TestTransitionReadyReplicasCoverGrowth raises both members' targets from 6 to
// 8 while each already serves 16 ready replicas, so the replicas in use leave
// no surge allowance. A move that is not whole-member raises the targets anyway,
// because no replica starts. A whole-member move still waits for budget, and so
// does growth on a member that does not serve.
func TestTransitionReadyReplicasCoverGrowth(t *testing.T) {
	in := Transition{
		From: Plan{Targets: map[string]int32{"a": 6, "b": 6}}, Current: map[string]int32{"a": 6, "b": 6},
		Desired: Plan{Targets: map[string]int32{"a": 8, "b": 8}}, Surge: 16,
		Members: map[string]MemberState{"a": serving(16), "b": serving(16)},
	}
	if diff := cmp.Diff(Step{Targets: map[string]int32{"a": 8, "b": 8}, Reason: "AwaitingMemberConvergence"}, advance(t, in)); diff != "" {
		t.Fatalf("ready replicas must cover the growth (-want +got):\n%s", diff)
	}
	whole, err := AdvanceWholeMembers(in)
	require.NoError(t, err)
	if diff := cmp.Diff(Step{Targets: map[string]int32{"a": 6, "b": 6}, Reason: "SurgeBudgetExhausted"}, whole); diff != "" {
		t.Fatalf("a whole-member move must wait for budget (-want +got):\n%s", diff)
	}
	in.Members["b"] = MemberState{Known: true, Eligible: true, Applied: true, Ready: 16, Occupied: 16}
	if diff := cmp.Diff(Step{Targets: map[string]int32{"a": 8, "b": 6}, Reason: "AwaitingMemberConvergence"}, advance(t, in)); diff != "" {
		t.Fatalf("a member that does not serve must not cover its growth (-want +got):\n%s", diff)
	}
}
