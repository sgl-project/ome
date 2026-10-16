package types

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestDeadlineAt pins the unconfigured case: with no readiness window an
// operation must open with NO deadline (the zero Time the backstop reads
// as "never expires"), never one equal to its own start time, which the
// very next pass would read as already elapsed.
func TestDeadlineAt(t *testing.T) {
	now := metav1.NewTime(time.Now().Truncate(time.Second))

	if got := DeadlineAt(now, 30*time.Minute); !got.Time.Equal(now.Add(30 * time.Minute)) {
		t.Errorf("configured window: got %v want %v", got, now.Add(30*time.Minute))
	}
	for _, timeout := range []time.Duration{0, -time.Minute} {
		if got := DeadlineAt(now, timeout); !got.IsZero() {
			t.Errorf("timeout %v: got deadline %v want the zero Time", timeout, got)
		}
	}
}

// A per-Instance topology key overrides the Component's; a row with none
// falls back so a plan need only name the exceptions.
func TestComponentPlanTopologyKeyForInstance(t *testing.T) {
	plan := ComponentPlan{
		TopologyKey:          "rack",
		InstanceTopologyKeys: map[int32]string{1: "island-a"},
	}
	if got := plan.TopologyKeyForInstance(1); got != "island-a" {
		t.Errorf("per-Instance key: got %q want %q", got, "island-a")
	}
	if got := plan.TopologyKeyForInstance(2); got != "rack" {
		t.Errorf("fallback: got %q want %q", got, "rack")
	}
}

func TestInstancePlanTotalPodsSumsEveryRunner(t *testing.T) {
	plan := InstancePlan{Runners: []RunnerPlan{{Size: 1}, {Size: 4}}}
	if got := plan.TotalPods(); got != 5 {
		t.Errorf("total pods: got %d want 5", got)
	}
	if got := (InstancePlan{}).TotalPods(); got != 0 {
		t.Errorf("no runners: got %d want 0", got)
	}
}

// The surge slot must avoid both the steady-state indices and the slots
// siblings claimed this pass but have not materialized yet: sharing one
// releases every source at once when the shared surge turns Ready.
func TestAllocateSurgeIndexExcludesClaimedSlots(t *testing.T) {
	claimed := int32(1)
	instances := []InstanceStatus{
		{Index: 0, Operation: &InstanceOperation{SurgeIndex: &claimed}},
		{Index: 2},
	}
	if got := AllocateSurgeIndex(instances); got != 3 {
		t.Errorf("lowest slot free of indices and claims: got %d want 3", got)
	}
	if got := AllocateSurgeIndex(nil); got != 0 {
		t.Errorf("an empty Component starts at 0: got %d", got)
	}
	if got := AllocateSurgeIndex([]InstanceStatus{{Index: 1}}); got != 0 {
		t.Errorf("a hole below the highest index is still the lowest free: got %d want 0", got)
	}
}

// TestComponentPlan_RunnersFor pins how an Instance's layout is resolved for
// the revision it renders: the target's layout at the width that revision
// recorded, the target's own layout for the target or a revision recording
// no width, and the Instance's entry when the plan carries no target layout.
func TestComponentPlan_RunnersFor(t *testing.T) {
	gang := []RunnerPlan{{Name: RunnerLeader, Size: 1}, {Name: RunnerWorker, Size: 3}}
	plan := ComponentPlan{
		Runners:              gang,
		WorkerSizeByRevision: map[string]int32{"svc-engine-old00000": 2},
	}
	inst := InstancePlan{Index: 0, Runners: []RunnerPlan{{Name: RunnerLeader, Size: 1}, {Name: RunnerWorker, Size: 9}}}

	if got := plan.RunnersFor(inst, "svc-engine-old00000"); got[1].Size != 2 || got[0].Size != 1 || len(got) != 2 {
		t.Fatalf("a recorded width is rendered: got %+v", got)
	}
	if got := plan.RunnersFor(inst, "svc-engine-new00000"); got[1].Size != 3 {
		t.Fatalf("a revision recording no width renders the target's: got %+v", got)
	}
	if got := plan.RunnersFor(inst, ""); got[1].Size != 3 {
		t.Fatalf("no revision renders the target's layout: got %+v", got)
	}
	if plan.Runners[1].Size != 3 {
		t.Fatalf("RunnersFor must copy, not alias, the target layout: %+v", plan.Runners)
	}
	bare := ComponentPlan{WorkerSizeByRevision: map[string]int32{"svc-engine-old00000": 2}}
	if got := bare.RunnersFor(inst, "svc-engine-old00000"); got[1].Size != 2 {
		t.Fatalf("without a target layout the Instance's entry is the base: got %+v", got)
	}
	single := ComponentPlan{Runners: []RunnerPlan{{Name: RunnerDefault, Size: 1}}, WorkerSizeByRevision: map[string]int32{"svc-router-old00000": 2}}
	if got := single.RunnersFor(InstancePlan{}, "svc-router-old00000"); len(got) != 1 || got[0].Size != 1 {
		t.Fatalf("a single-pod layout has no width to set: got %+v", got)
	}
}
