package workload

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Surge-pair pinning vs replica drops: a replica count lowered while a
// surge pair (migration or gang update) is in flight must keep BOTH
// pair members in the plan — the pair is released only through its own
// state machine, never by scale-down — and shed the budget overflow
// from the unpinned siblings instead.

func TestInstancePlanIndices_ReplicaDropKeepsMigrationPair(t *testing.T) {
	surge := int32(3)
	back := int32(0)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseMigrating,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate, SurgeIndex: &surge}},
		{Index: 1, Phase: InstancePhaseReady},
		{Index: 3, Phase: InstancePhaseCreating,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate, SurgeIndex: &back}},
	}
	if diff := cmp.Diff([]int32{0, 3}, instancePlanIndices(instances, 1)); diff != "" {
		t.Fatalf("replica drop must keep the in-flight migration pair and shed the Ready sibling (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_ReplicaDropKeepsGangSurgePair(t *testing.T) {
	surge := int32(3)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseUpdating, TargetRevision: "comp-rev-v2bbbbbb",
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: "Surge",
				SurgeIndex: &surge, TargetRevision: "comp-rev-v2bbbbbb"}},
		{Index: 1, Phase: InstancePhaseReady},
		{Index: 3, Phase: InstancePhaseCreating, TargetRevision: "comp-rev-v2bbbbbb",
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepGangSurgeTarget,
				TargetRevision: "comp-rev-v2bbbbbb"}},
	}
	if diff := cmp.Diff([]int32{0, 3}, instancePlanIndices(instances, 1)); diff != "" {
		t.Fatalf("replica drop must keep the in-flight gang surge pair and shed the Ready sibling (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_MigrationTargetNotReadyKeepsSourcePinned(t *testing.T) {
	// The migration-side release branch demands a Ready target before
	// the source leaves the plan; a target still Creating keeps both
	// pinned even though the source references it.
	surge := int32(1)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseMigrating,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate, SurgeIndex: &surge}},
		{Index: 1, Phase: InstancePhaseCreating,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate}},
	}
	if diff := cmp.Diff([]int32{0, 1}, instancePlanIndices(instances, 1)); diff != "" {
		t.Fatalf("a not-yet-Ready migration target must keep the source pinned (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_GangSurgeDrainReleaseDemandsExactTargetProof(t *testing.T) {
	// A gang source in the durable drain step is released only when the
	// referenced target is a settled Ready instance promoted on the
	// operation's EXACT pinned revision. Any weaker target state keeps
	// the pair pinned.
	newRev, oldRev := "comp-rev-newaaaa", "comp-rev-oldbbbb"
	zero := int32(0)
	source := InstanceStatus{Index: 1, Phase: InstancePhaseUpdating,
		RunningRevision: oldRev, TargetRevision: newRev,
		Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepSurgeDrain,
			SurgeIndex: &zero, TargetRevision: newRev}}
	sibling := InstanceStatus{Index: 2, Phase: InstancePhaseReady, RunningRevision: newRev}

	tests := []struct {
		name   string
		target InstanceStatus
	}{
		{
			name: "target promoted on a different revision",
			target: InstanceStatus{Index: 0, Phase: InstancePhaseReady,
				RunningRevision: "comp-rev-otherccc"},
		},
		{
			name: "target still carrying an operation",
			target: InstanceStatus{Index: 0, Phase: InstancePhaseReady, RunningRevision: newRev,
				Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepGangSurgeTarget,
					TargetRevision: newRev}},
		},
		{
			name: "target with a pending TargetRevision",
			target: InstanceStatus{Index: 0, Phase: InstancePhaseReady,
				RunningRevision: newRev, TargetRevision: newRev},
		},
		{
			name: "target not yet Ready",
			target: InstanceStatus{Index: 0, Phase: InstancePhaseCreating,
				RunningRevision: newRev},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			instances := []InstanceStatus{test.target, source, sibling}
			if diff := cmp.Diff([]int32{0, 1, 2}, instancePlanIndices(instances, 2)); diff != "" {
				t.Fatalf("unproven target must keep the drain-step source pinned (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInstancePlanIndices_GangSurgeDrainMissingTargetKeepsSourcePinned(t *testing.T) {
	// A drain-step source whose referenced target status vanished has no
	// promotion proof: the source must stay in the plan so the update
	// state machine can recover, and the dangling reference must not
	// materialize a phantom index.
	missing := int32(5)
	instances := []InstanceStatus{
		{Index: 1, Phase: InstancePhaseUpdating,
			RunningRevision: "comp-rev-oldbbbb", TargetRevision: "comp-rev-newaaaa",
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepSurgeDrain,
				SurgeIndex: &missing, TargetRevision: "comp-rev-newaaaa"}},
	}
	if diff := cmp.Diff([]int32{1}, instancePlanIndices(instances, 1)); diff != "" {
		t.Fatalf("missing target must keep the source pinned without inventing its index (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_OccupiedReferencedTargetStaysPinned(t *testing.T) {
	// Conflict recovery: a fresh gang-surge claim can reference an index
	// occupied by an unrelated settled instance. Both the source and the
	// occupant stay in the plan until the update state machine resets the
	// claim — neither may fall into scale-down while the pair is ambiguous.
	occupied := int32(1)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseUpdating, TargetRevision: "comp-rev-newaaaa",
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: "Surge",
				SurgeIndex: &occupied, TargetRevision: "comp-rev-newaaaa"}},
		{Index: 1, Phase: InstancePhaseReady, RunningRevision: "comp-rev-otherccc"},
		{Index: 2, Phase: InstancePhaseReady, RunningRevision: "comp-rev-newaaaa"},
	}
	if diff := cmp.Diff([]int32{0, 1, 2}, instancePlanIndices(instances, 2)); diff != "" {
		t.Fatalf("occupied referenced target must stay pinned at replicas=2 (-want +got):\n%s", diff)
	}
	// Under a replica drop the pinned pair still wins; the unreferenced
	// sibling is the scale-down extra.
	if diff := cmp.Diff([]int32{0, 1}, instancePlanIndices(instances, 1)); diff != "" {
		t.Fatalf("occupied referenced target must stay pinned at replicas=1 (-want +got):\n%s", diff)
	}
}
