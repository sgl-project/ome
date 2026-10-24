package allocation

import (
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func wholeMemberMove() Transition {
	in := moving()
	in.Surge = 4
	return in
}

func advanceWholeMembers(t *testing.T, in Transition) Step {
	t.Helper()
	before := Transition{
		From:    Plan{Targets: maps.Clone(in.From.Targets), Unassigned: in.From.Unassigned},
		Current: maps.Clone(in.Current), Desired: Plan{Targets: maps.Clone(in.Desired.Targets), Unassigned: in.Desired.Unassigned},
		Members: maps.Clone(in.Members), Surge: in.Surge, RolloutReserved: in.RolloutReserved,
	}
	got, err := AdvanceWholeMembers(in)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(before, in); diff != "" {
		t.Fatalf("planning mutated its input (-want +got):\n%s", diff)
	}
	return got
}

func TestWholeMemberTransitionBudget(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*Transition)
		want   Step
	}{
		{name: "full destination fits", want: Step{Targets: map[string]int32{"a": 4, "b": 4}, Reason: "AwaitingMemberConvergence"}},
		{name: "zero allowance", change: func(in *Transition) { in.Surge = 0 },
			want: Step{Targets: map[string]int32{"a": 4, "b": 0}, Reason: "SurgeBudgetExhausted"}},
		{name: "partial destination cannot fit", change: func(in *Transition) { in.Surge = 3 },
			want: Step{Targets: map[string]int32{"a": 4, "b": 0}, Reason: "SurgeBudgetExhausted"}},
		{name: "pending rollout shares allowance", change: func(in *Transition) { in.RolloutReserved = 1 },
			want: Step{Targets: map[string]int32{"a": 4, "b": 0}, Reason: "SurgeBudgetExhausted"}},
		{name: "physical surplus shares allowance", change: func(in *Transition) { in.Members["a"] = serving(5) },
			want: Step{Targets: map[string]int32{"a": 4, "b": 0}, Reason: "SurgeBudgetExhausted"}},
		{name: "rollout and physical surplus both fit", change: func(in *Transition) {
			in.RolloutReserved, in.Surge = 1, 6
			in.Members["a"] = serving(5)
		}, want: Step{Targets: map[string]int32{"a": 4, "b": 4}, Reason: "AwaitingMemberConvergence"}},
		{name: "unknown destination", change: func(in *Transition) { in.Members["b"] = MemberState{} },
			want: Step{Targets: map[string]int32{"a": 4, "b": 0}, Reason: "ReplacementNotReady"}},
		{name: "ineligible destination", change: func(in *Transition) { in.Members["b"] = MemberState{Known: true} },
			want: Step{Targets: map[string]int32{"a": 4, "b": 0}, Reason: "ReplacementNotReady"}},
		{name: "unknown donor", change: func(in *Transition) { in.Members["a"] = MemberState{} },
			want: Step{Targets: map[string]int32{"a": 4, "b": 0}, Reason: "ObservationUnknown"}},
		{name: "confirmed absent donor releases separately", change: func(in *Transition) { in.Members["a"] = MemberState{Known: true, Absent: true} },
			want: Step{Targets: map[string]int32{"a": 0, "b": 0}, Reason: "AwaitingMemberConvergence"}},
		{name: "wide totals", change: func(in *Transition) {
			in.From.Targets["a"], in.Current["a"], in.Desired.Targets["b"] = math.MaxInt32, math.MaxInt32, math.MaxInt32
			in.Surge, in.Members["a"] = math.MaxInt32, serving(math.MaxInt32)
		}, want: Step{Targets: map[string]int32{"a": math.MaxInt32, "b": math.MaxInt32}, Reason: "AwaitingMemberConvergence"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := wholeMemberMove()
			if tt.change != nil {
				tt.change(&in)
			}
			if diff := cmp.Diff(tt.want, advanceWholeMembers(t, in)); diff != "" {
				t.Fatalf("step (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWholeMemberTransitionRetirement(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*Transition)
		want   Step
	}{
		{name: "partial readiness cannot reduce donor", change: func(in *Transition) {
			member := in.Members["b"]
			member.Ready = 3
			in.Members["b"] = member
		}, want: Step{Targets: map[string]int32{"a": 4, "b": 4}, Reason: "ReplacementNotReady"}},
		{name: "ready without routing", change: func(in *Transition) {
			member := in.Members["b"]
			member.Routable = false
			in.Members["b"] = member
		}, want: Step{Targets: map[string]int32{"a": 4, "b": 4}, Reason: "ReplacementNotReady"}},
		{name: "application not acknowledged", change: func(in *Transition) {
			member := in.Members["b"]
			member.Applied = false
			in.Members["b"] = member
		}, want: Step{Targets: map[string]int32{"a": 4, "b": 4}, Reason: "ReplacementNotReady"}},
		{name: "ready and routable starts drain", want: Step{Targets: map[string]int32{"a": 4, "b": 4}, Drain: []string{"a"}, Reason: "AwaitingMemberConvergence"}},
		{name: "drain acknowledged permits zero floor", change: func(in *Transition) {
			in.Members["a"] = MemberState{Known: true, Applied: true, Drained: true, Occupied: 4}
		},
			want: Step{Targets: map[string]int32{"a": 0, "b": 4}, Reason: "AwaitingMemberConvergence"}},
		{name: "terminating member is still occupied", change: func(in *Transition) {
			in.Current["a"] = 0
			in.Members["a"] = MemberState{Known: true, Applied: true, Drained: true, Occupied: 4}
		}, want: Step{Targets: map[string]int32{"a": 0, "b": 4}, Reason: "AwaitingMemberConvergence"}},
		{name: "physical cleanup completes", change: func(in *Transition) {
			in.Current["a"] = 0
			in.Members["a"] = MemberState{Known: true, Absent: true}
		}, want: Step{Targets: map[string]int32{"a": 0, "b": 4}, Complete: true}},
		{name: "retarget resumes a drained desired member", change: func(in *Transition) {
			in.Desired.Targets = map[string]int32{"a": 4}
			in.Members["a"] = MemberState{Known: true, Applied: true, Drained: true, Occupied: 4}
		}, want: Step{Targets: map[string]int32{"a": 4, "b": 4}, Resume: []string{"a"}, Reason: "AwaitingMemberConvergence"}},
		{name: "unassigned floor protects donor", change: func(in *Transition) {
			in.Current = map[string]int32{"a": 4}
			in.Desired = Plan{Unassigned: 4}
			delete(in.Members, "b")
		}, want: Step{Targets: map[string]int32{"a": 4}, Reason: "ReplacementNotReady"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := wholeMemberMove()
			in.Current["b"], in.Members["b"] = 4, serving(4)
			if tt.change != nil {
				tt.change(&in)
			}
			if diff := cmp.Diff(tt.want, advanceWholeMembers(t, in)); diff != "" {
				t.Fatalf("step (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWholeMemberTransitionRetarget(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*Transition)
		want   Step
	}{
		{name: "cancel wholly unready surplus", change: func(in *Transition) { in.Members["b"] = MemberState{Known: true, Applied: true, Occupied: 4} },
			want: Step{Targets: map[string]int32{"a": 4, "b": 4, "c": 0}, Drain: []string{"b"}, Reason: "AwaitingMemberConvergence"}},
		{name: "cancel partially ready surplus", change: func(in *Transition) {
			in.Members["b"] = MemberState{Known: true, Applied: true, Routable: true, Ready: 2, Occupied: 4}
		},
			want: Step{Targets: map[string]int32{"a": 4, "b": 4, "c": 0}, Drain: []string{"b"}, Reason: "AwaitingMemberConvergence"}},
		{name: "cancel fully ready surplus", want: Step{Targets: map[string]int32{"a": 4, "b": 4, "c": 0}, Drain: []string{"b"}, Reason: "AwaitingMemberConvergence"}},
		{name: "requested cleanup cannot fund new member", change: func(in *Transition) {
			in.Current["b"] = 0
			in.Members["b"] = MemberState{Known: true, Applied: true, Drained: true, Occupied: 4}
		}, want: Step{Targets: map[string]int32{"a": 4, "b": 0, "c": 0}, Reason: "SurgeBudgetExhausted"}},
		{name: "confirmed cleanup releases allowance", change: func(in *Transition) {
			in.Current["b"] = 0
			in.Members["b"] = MemberState{Known: true, Absent: true}
		}, want: Step{Targets: map[string]int32{"a": 4, "b": 0, "c": 4}, Reason: "AwaitingMemberConvergence"}},
		{name: "surplus still backs an earlier reduction", change: func(in *Transition) {
			in.Current["a"], in.Members["a"] = 2, serving(2)
			in.Members["b"] = MemberState{Known: true, Applied: true, Routable: true, Ready: 2, Occupied: 4}
		}, want: Step{Targets: map[string]int32{"a": 2, "b": 4, "c": 0}, Reason: "SurgeBudgetExhausted"}},
		{name: "retired original cannot retire its replacement", change: func(in *Transition) {
			in.Current["a"] = 0
			in.Members["a"] = MemberState{Known: true, Absent: true}
		}, want: Step{Targets: map[string]int32{"a": 0, "b": 4, "c": 4}, Reason: "AwaitingMemberConvergence"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := wholeMemberMove()
			in.Current["b"], in.Members["b"] = 4, serving(4)
			in.Desired = Plan{Targets: map[string]int32{"c": 4}}
			in.Members["c"] = MemberState{Known: true, Eligible: true}
			if tt.change != nil {
				tt.change(&in)
			}
			if diff := cmp.Diff(tt.want, advanceWholeMembers(t, in)); diff != "" {
				t.Fatalf("step (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWholeMemberTransitionIndependentTargets(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   Transition
		want Step
	}{
		{name: "initial members proceed independently", in: Transition{
			Desired: Plan{Targets: map[string]int32{"a": 4, "b": 4, "c": 4}},
			Members: map[string]MemberState{"a": {Known: true, Eligible: true}, "b": {}, "c": {Known: true, Eligible: true}},
		}, want: Step{Targets: map[string]int32{"a": 4, "b": 0, "c": 4}, Reason: "AwaitingMemberConvergence"}},
		{name: "later smaller member fits remaining allowance", in: Transition{
			From: Plan{Targets: map[string]int32{"a": 4}}, Current: map[string]int32{"a": 4},
			Desired: Plan{Targets: map[string]int32{"b": 3, "c": 1}}, Surge: 1,
			Members: map[string]MemberState{"a": serving(4), "b": {Known: true, Eligible: true}, "c": {Known: true, Eligible: true}},
		}, want: Step{Targets: map[string]int32{"a": 4, "b": 0, "c": 1}, Reason: "AwaitingMemberConvergence"}},
		{name: "one replacement cannot drain two members", in: Transition{
			From: Plan{Targets: map[string]int32{"a": 2, "b": 2}}, Current: map[string]int32{"a": 2, "b": 2, "c": 4},
			Desired: Plan{Targets: map[string]int32{"c": 4}}, Surge: 4,
			Members: map[string]MemberState{"a": serving(2), "b": serving(2), "c": {Known: true, Applied: true, Routable: true, Ready: 3, Occupied: 4}},
		}, want: Step{Targets: map[string]int32{"a": 2, "b": 2, "c": 4}, Drain: []string{"a"}, Reason: "AwaitingMemberConvergence"}},
		{name: "explicit lower policy applies in full", in: Transition{
			From: Plan{Targets: map[string]int32{"a": 4}}, Current: map[string]int32{"a": 4},
			Desired: Plan{Targets: map[string]int32{"a": 2}}, Members: map[string]MemberState{"a": serving(4)},
		}, want: Step{Targets: map[string]int32{"a": 2}, Reason: "AwaitingMemberConvergence"}},
		{name: "increased policy is not partially stamped", in: Transition{
			From: Plan{Targets: map[string]int32{"a": 4}}, Current: map[string]int32{"a": 4},
			Desired: Plan{Targets: map[string]int32{"a": 6}}, Members: map[string]MemberState{"a": serving(5)},
		}, want: Step{Targets: map[string]int32{"a": 4}, Reason: "SurgeBudgetExhausted"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, advanceWholeMembers(t, tt.in)); diff != "" {
				t.Fatalf("step (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWholeMemberTransitionsConvergeWithinSharedBudget(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e"}
	for scenario := range 200 {
		t.Run(fmt.Sprintf("scenario_%03d", scenario), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(uint64(scenario), 17))
			floor := int32(1 + rng.IntN(8))
			partition := func() Plan {
				order := rng.Perm(len(names))
				return Plan{Targets: map[string]int32{names[order[0]]: floor, names[order[1]]: floor}}
			}
			reserved := int32(rng.IntN(3))
			in := Transition{From: partition(), Desired: partition(), Surge: int64(floor + reserved), RolloutReserved: reserved, Members: map[string]MemberState{}}
			in.Current = maps.Clone(in.From.Targets)
			for _, name := range names {
				member := serving(in.Current[name])
				member.Routable = in.Current[name] > 0
				in.Members[name] = member
			}
			for iteration := range 128 {
				if iteration == 3 || iteration == 7 {
					in.Desired = partition()
				}
				step := advanceWholeMembers(t, in)
				var occupied, ready int64
				occupied = int64(reserved)
				for _, name := range names {
					target, member := step.Targets[name], in.Members[name]
					if target != in.Current[name] && target != in.Desired.Targets[name] {
						t.Fatalf("partial member target at iteration %d: input %+v step %+v", iteration, in, step)
					}
					occupied += int64(max(target, member.Occupied))
					if member.Routable && !slices.Contains(step.Drain, name) {
						ready += int64(min(target, member.Ready))
					}
				}
				if limit := int64(2*floor) + in.Surge; occupied > limit {
					t.Fatalf("occupied %d exceeds allowance %d at iteration %d: input %+v step %+v", occupied, limit, iteration, in, step)
				}
				if ready < int64(2*floor) {
					t.Fatalf("serving floor lost at iteration %d: input %+v step %+v", iteration, in, step)
				}
				if step.Complete && iteration > 7 {
					compareTargets(t, in.Desired.Targets, step.Targets)
					return
				}
				for _, name := range names {
					member := in.Members[name]
					if slices.Contains(step.Drain, name) {
						member.Drained, member.Routable = true, false
					}
					if slices.Contains(step.Resume, name) {
						member.Drained = false
					}
					// Projection and publication complete on separate observations;
					// until then pending removals still occupy the shared allowance.
					if iteration%2 == 0 {
						member.Occupied, member.Ready, member.Applied = step.Targets[name], step.Targets[name], true
					} else if !member.Drained {
						member.Routable = member.Ready > 0
					}
					in.Members[name] = member
				}
				in.Current = step.Targets
			}
			t.Fatalf("transition did not converge: %+v", in)
		})
	}
}
