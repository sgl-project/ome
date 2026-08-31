package workload

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func intPtr(v int) *int       { return &v }
func int32Ptr(v int32) *int32 { return &v }
func boolPtr(v bool) *bool    { return &v }
func restartPolicyPtr(v RestartPolicy) *RestartPolicy {
	return &v
}
func readyPolicyPtr(v InstanceReadyPolicy) *InstanceReadyPolicy {
	return &v
}

// singlePodDesired builds a single-pod WorkloadDesiredSpec for tests.
// replicas <= 0 mirrors the production path where MinReplicas=nil/0
// defaults to 1.
func singlePodDesired(replicas int32, lifecycle Lifecycle) WorkloadDesiredSpec {
	return WorkloadDesiredSpec{
		Replicas:  replicas,
		Runners:   []Runner{{Name: "default", Size: 1}},
		Lifecycle: lifecycle,
	}
}

func multiPodDesired(replicas, workerSize int32, lifecycle Lifecycle) WorkloadDesiredSpec {
	return WorkloadDesiredSpec{
		Replicas: replicas,
		MultiPod: true,
		Runners: []Runner{
			{Name: "leader", Size: 1},
			{Name: "worker", Size: workerSize},
		},
		Lifecycle: lifecycle,
	}
}

// TestBuildPlan_MultiPodEmitsLeaderWorkerRunners pins the multi-pod
// layout: one leader runner of size 1 plus one worker runner of size
// WorkerSize.
func TestBuildPlan_MultiPodEmitsLeaderWorkerRunners(t *testing.T) {
	desired := multiPodDesired(2, 3, Lifecycle{})
	plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := plan.Replicas, int32(2); got != want {
		t.Errorf("Replicas: got %d want %d", got, want)
	}
	if got, want := len(plan.Instances), 2; got != want {
		t.Fatalf("len(Instances): got %d want %d", got, want)
	}
	for i, inst := range plan.Instances {
		if int(inst.Index) != i {
			t.Errorf("Instances[%d].Index: got %d want %d", i, inst.Index, i)
		}
		want := []RunnerPlan{
			{Name: "leader", Size: 1},
			{Name: "worker", Size: 3},
		}
		if diff := cmp.Diff(want, inst.Runners); diff != "" {
			t.Errorf("Instances[%d].Runners mismatch (-want +got):\n%s", i, diff)
		}
	}
}

// TestBuildPlan_MultiPodDefaults pins the defaults for multi-pod
// Instances: RestartPolicy=RecreateInstance, ReadyPolicy=AllPodReady.
// Single-pod defaults stay None for both.
func TestBuildPlan_MultiPodDefaults(t *testing.T) {
	desired := multiPodDesired(1, 1, Lifecycle{})
	plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.RestartPolicy != RestartPolicyRecreateInstance {
		t.Errorf("RestartPolicy: got %q want RecreateInstanceOnPodRestart", plan.RestartPolicy)
	}
	if plan.ReadyPolicy != InstanceReadyPolicyAllPodReady {
		t.Errorf("ReadyPolicy: got %q want AllPodReady", plan.ReadyPolicy)
	}
}

func TestBuildPlan_ProjectsPausedCircuitBreaker(t *testing.T) {
	desired := multiPodDesired(1, 1, Lifecycle{})
	desired.Paused = true

	plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !plan.Paused {
		t.Fatal("Paused: got false want true")
	}
}

// TestBuildPlan_MultiPodWithZeroWorkerSize confirms WorkerSize=0 still
// produces leader+worker entries; the webhook validator rejects orphan
// leader without worker.size>0, but BuildPlan stays defensive.
func TestBuildPlan_MultiPodWithZeroWorkerSize(t *testing.T) {
	desired := multiPodDesired(1, 0, Lifecycle{})
	plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(plan.Instances[0].Runners), 2; got != want {
		t.Fatalf("Runners length: got %d want %d (leader + worker even if size=0)", got, want)
	}
	if plan.Instances[0].Runners[0].Size != 1 || plan.Instances[0].Runners[0].Name != "leader" {
		t.Errorf("leader runner shape: got %+v", plan.Instances[0].Runners[0])
	}
	if plan.Instances[0].Runners[1].Size != 0 || plan.Instances[0].Runners[1].Name != "worker" {
		t.Errorf("worker runner shape: got %+v", plan.Instances[0].Runners[1])
	}
}

func TestBuildPlan_SinglePod_DefaultsFromDefaulter(t *testing.T) {
	// Simulates a WorkloadDesiredSpec produced by an adapter whose source
	// went through the mutating webhook defaulter — lifecycle is fully
	// populated.
	desired := singlePodDesired(2, Lifecycle{
		RestartPolicy: restartPolicyPtr(RestartPolicyNone),
		ReadyPolicy:   readyPolicyPtr(InstanceReadyPolicyNone),
		UpdateStrategy: &UpdateStrategy{
			Type: UpdateStrategyInPlaceIfPossible,
			InPlaceUpdateStrategy: &InPlaceUpdateStrategy{
				GracePeriodSeconds:          int32Ptr(30),
				MarkNotReadyDuringLifecycle: boolPtr(true),
			},
		},
		InstanceReadyTimeout: &metav1.Duration{Duration: 30 * time.Minute},
		MigrationPolicy:      &MigrationPolicy{Mode: MigrationModeAuto},
	})
	plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := ComponentPlan{
		Component: ComponentEngine,
		Replicas:  2,
		Instances: []InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Incarnation: 1, Runners: []RunnerPlan{{Name: "default", Size: 1}}},
		},
		RestartPolicy: RestartPolicyNone,
		UpdateStrategy: UpdateStrategy{
			Type: UpdateStrategyInPlaceIfPossible,
			InPlaceUpdateStrategy: &InPlaceUpdateStrategy{
				GracePeriodSeconds:          int32Ptr(30),
				MarkNotReadyDuringLifecycle: boolPtr(true),
			},
		},
		ReadyPolicy:          InstanceReadyPolicyNone,
		InstanceReadyTimeout: 30 * time.Minute,
		MigrationMode:        MigrationModeAuto,
	}
	if diff := cmp.Diff(want, plan); diff != "" {
		t.Fatalf("plan mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildPlan_SinglePod_InlineDefaultsWhenWebhookSkipped(t *testing.T) {
	// Simulates a pre-defaulter object — adapter projected an empty
	// lifecycle. BuildPlan applies the same defaults inline.
	desired := singlePodDesired(1, Lifecycle{})
	plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Replicas != 1 {
		t.Errorf("Replicas: got %d want 1", plan.Replicas)
	}
	if plan.RestartPolicy != RestartPolicyNone {
		t.Errorf("RestartPolicy: got %q want None", plan.RestartPolicy)
	}
	if plan.ReadyPolicy != InstanceReadyPolicyNone {
		t.Errorf("ReadyPolicy: got %q want None", plan.ReadyPolicy)
	}
	if plan.UpdateStrategy.Type != UpdateStrategySurgeThenDrain {
		t.Errorf("UpdateStrategy.Type: got %q want SurgeThenDrain (default)", plan.UpdateStrategy.Type)
	}
	if plan.UpdateStrategy.InPlaceUpdateStrategy == nil ||
		plan.UpdateStrategy.InPlaceUpdateStrategy.GracePeriodSeconds == nil ||
		*plan.UpdateStrategy.InPlaceUpdateStrategy.GracePeriodSeconds != 30 {
		t.Errorf("gracePeriodSeconds: got %+v want 30", plan.UpdateStrategy.InPlaceUpdateStrategy)
	}
	if plan.InstanceReadyTimeout != 30*time.Minute {
		t.Errorf("InstanceReadyTimeout: got %v want 30m", plan.InstanceReadyTimeout)
	}
	if plan.MigrationMode != MigrationModeAuto {
		t.Errorf("MigrationMode: got %q want auto", plan.MigrationMode)
	}
}

func TestBuildPlan_ReplicaCount(t *testing.T) {
	tests := []struct {
		name   string
		minRep *int
		want   int32
	}{
		{name: "nil MinReplicas defaults to 1", minRep: nil, want: 1},
		{name: "zero MinReplicas defaults to 1", minRep: intPtr(0), want: 1},
		{name: "explicit 1", minRep: intPtr(1), want: 1},
		{name: "explicit 4", minRep: intPtr(4), want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rep int32
			if tt.minRep != nil {
				rep = int32(*tt.minRep)
			}
			desired := singlePodDesired(rep, Lifecycle{})
			plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.Replicas != tt.want {
				t.Errorf("Replicas: got %d want %d", plan.Replicas, tt.want)
			}
			if int32(len(plan.Instances)) != tt.want {
				t.Errorf("len(Instances): got %d want %d", len(plan.Instances), tt.want)
			}
			for i, inst := range plan.Instances {
				if inst.Index != int32(i) {
					t.Errorf("Instances[%d].Index: got %d", i, inst.Index)
				}
				if len(inst.Runners) != 1 ||
					inst.Runners[0].Name != "default" ||
					inst.Runners[0].Size != 1 {
					t.Errorf("Instances[%d].Runners: got %+v want [{default 1}]", i, inst.Runners)
				}
			}
		})
	}
}

func TestBuildPlan_IncarnationDefaultsToOneOnFirstReconcile(t *testing.T) {
	// Observed state has no InstanceStatuses — every Instance gets
	// Incarnation=1.
	desired := singlePodDesired(3, Lifecycle{})
	plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, inst := range plan.Instances {
		if inst.Incarnation != 1 {
			t.Errorf("Instances[%d].Incarnation: got %d want 1", inst.Index, inst.Incarnation)
		}
	}
}

func TestBuildPlan_IncarnationPreservedFromStatus(t *testing.T) {
	// Observed state carries InstanceStatuses with explicit Incarnations —
	// BuildPlan reads them back instead of resetting to 1.
	desired := singlePodDesired(2, Lifecycle{})
	observed := WorkloadObservedState{
		InstanceStatuses: []InstanceStatus{
			{Index: 0, Incarnation: 4},
			{Index: 1, Incarnation: 2},
		},
	}
	plan, err := BuildPlan(ComponentEngine, desired, observed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[int32]int64{0: 4, 1: 2}
	for _, inst := range plan.Instances {
		if got := inst.Incarnation; got != want[inst.Index] {
			t.Errorf("Instances[%d].Incarnation: got %d want %d", inst.Index, got, want[inst.Index])
		}
	}
}

func TestBuildPlan_IncarnationScopedToWorkload(t *testing.T) {
	// The observed state is per-workload now (Source.ObservedState
	// returns only this Component's statuses), so cross-Component leakage
	// is impossible at the call site. The test pins the BuildPlan contract:
	// no InstanceStatuses → default to 1 for every Instance.
	desired := singlePodDesired(1, Lifecycle{})
	plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Instances[0].Incarnation != 1 {
		t.Errorf("Engine[0].Incarnation: got %d want 1", plan.Instances[0].Incarnation)
	}
}

func TestBuildPlan_RestartPolicyExplicitWinsOverDefault(t *testing.T) {
	desired := singlePodDesired(0, Lifecycle{
		RestartPolicy: restartPolicyPtr(RestartPolicyRecreateInstance),
	})
	plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Single-pod defaults to None; an explicit RecreateInstance must win.
	if plan.RestartPolicy != RestartPolicyRecreateInstance {
		t.Errorf("RestartPolicy: got %q want RecreateInstanceOnPodRestart", plan.RestartPolicy)
	}
}

func TestLowestUnusedIndex(t *testing.T) {
	used := map[int32]struct{}{0: {}, 1: {}, 3: {}}
	if got := lowestUnusedIndex(used); got != 2 {
		t.Errorf("got %d want 2", got)
	}
	if got := lowestUnusedIndex(map[int32]struct{}{}); got != 0 {
		t.Errorf("empty set: got %d want 0", got)
	}
}

func TestInstancePlanIndices_MultiReplicaMigrationPreservesUnrelatedInstance(t *testing.T) {
	// Regression: with replicas=2 and statuses {0 Ready, 1 Ready},
	// migrating instance 0 (surge at 2) must yield plan {0, 1, 2}.
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseMigrating},
		{Index: 1, Phase: InstancePhaseReady},
		{Index: 2, Phase: InstancePhaseCreating, Operation: &InstanceOperation{Type: InstanceOperationMigrate}},
	}
	got := instancePlanIndices(instances, 2)
	hit := map[int32]bool{}
	for _, idx := range got {
		hit[idx] = true
	}
	if !hit[0] || !hit[1] || !hit[2] {
		t.Errorf("plan must include {0,1,2}; got %v", got)
	}
}

func TestInstancePlanIndices_PreservesSparseMigrationLayout(t *testing.T) {
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseMigrating},
		{Index: 2, Phase: InstancePhaseCreating, Operation: &InstanceOperation{Type: InstanceOperationMigrate}},
	}
	got := instancePlanIndices(instances, 1)
	// Both migration-in-flight indices must be in the plan even though
	// replicas=1.
	hit := map[int32]bool{}
	for _, idx := range got {
		hit[idx] = true
	}
	if !hit[0] || !hit[2] {
		t.Errorf("plan must include both migration indices; got %v", got)
	}
}

func TestInstancePlanIndices_MigrationReadyTargetReplacesSource(t *testing.T) {
	const revision = "comp-rev-current"
	targetIndex := int32(1)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseMigrating, RunningRevision: revision,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate, RequestUUID: "request-a", SurgeIndex: &targetIndex}},
		{Index: targetIndex, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: revision},
	}
	indices := instancePlanIndices(instances, 1)
	if diff := cmp.Diff([]int32{targetIndex}, indices); diff != "" {
		t.Fatalf("promoted migration target must replace its source (-want +got):\n%s", diff)
	}
	plan := ComponentPlan{Instances: []InstancePlan{{Index: targetIndex}}}
	if extras := ExtraInstanceIndices(instances, plan, false); len(extras) != 0 {
		t.Fatalf("normal scale-down selected migration source as extra: %v", extras)
	}
}

func TestInstancePlanIndices_MigrationRetiringSourceDoesNotDisplaceSteadyInstance(t *testing.T) {
	const revision = "comp-rev-current"
	targetIndex := int32(8)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseMigrating, RunningRevision: revision,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate, RequestUUID: "request-a", SurgeIndex: &targetIndex}},
		{Index: 1, Phase: InstancePhaseReady},
		{Index: 2, Phase: InstancePhaseReady},
		{Index: 3, Phase: InstancePhaseReady},
		{Index: 4, Phase: InstancePhaseReady},
		{Index: 5, Phase: InstancePhaseReady},
		{Index: 6, Phase: InstancePhaseReady},
		{Index: 7, Phase: InstancePhaseCreating},
		{Index: targetIndex, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: revision},
	}
	want := []int32{1, 2, 3, 4, 5, 6, 7, 8}
	if diff := cmp.Diff(want, instancePlanIndices(instances, 8)); diff != "" {
		t.Fatalf("retiring migration source displaced a steady instance (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_MigrationSourceRequiresPromotedTargetProof(t *testing.T) {
	const revision = "comp-rev-current"
	targetIndex := int32(2)
	source := InstanceStatus{Index: 0, Phase: InstancePhaseMigrating, RunningRevision: revision,
		Operation: &InstanceOperation{Type: InstanceOperationMigrate, RequestUUID: "request-a", SurgeIndex: &targetIndex}}
	tests := []struct {
		name   string
		target InstanceStatus
	}{
		{
			name:   "wrong revision",
			target: InstanceStatus{Index: targetIndex, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: "comp-rev-other"},
		},
		{
			name: "operation still active",
			target: InstanceStatus{Index: targetIndex, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: revision,
				Operation: &InstanceOperation{Type: InstanceOperationMigrate}},
		},
		{
			name: "target revision still set",
			target: InstanceStatus{Index: targetIndex, Incarnation: 1, Phase: InstancePhaseReady,
				RunningRevision: revision, TargetRevision: revision},
		},
		{
			name:   "incarnation does not match a fresh surge",
			target: InstanceStatus{Index: targetIndex, Incarnation: 2, Phase: InstancePhaseReady, RunningRevision: revision},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			instances := []InstanceStatus{
				source,
				{Index: 1, Phase: InstancePhaseCreating, RunningRevision: revision},
				test.target,
			}
			want := []int32{0, 1, 2}
			if diff := cmp.Diff(want, instancePlanIndices(instances, 2)); diff != "" {
				t.Fatalf("unproven migration target released its source (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInstancePlanIndices_SharedMigrationTargetKeepsAllParticipants(t *testing.T) {
	const revision = "comp-rev-current"
	sharedTarget := int32(3)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseMigrating, RunningRevision: revision,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate, RequestUUID: "request-a", SurgeIndex: &sharedTarget}},
		{Index: 1, Phase: InstancePhaseMigrating, RunningRevision: revision,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate, RequestUUID: "request-b", SurgeIndex: &sharedTarget}},
		{Index: 2, Phase: InstancePhaseCreating, RunningRevision: revision},
		{Index: sharedTarget, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: revision},
	}
	want := []int32{0, 1, 2, 3}
	if diff := cmp.Diff(want, instancePlanIndices(instances, 3)); diff != "" {
		t.Fatalf("shared migration target released a source or sibling (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_WrongPhaseMigrationReferenceBlocksRetirement(t *testing.T) {
	const revision = "comp-rev-current"
	sharedTarget := int32(3)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseMigrating, RunningRevision: revision,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate, RequestUUID: "request-a", SurgeIndex: &sharedTarget}},
		{Index: 1, Phase: InstancePhaseFailed, RunningRevision: revision,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate, RequestUUID: "request-stale", SurgeIndex: &sharedTarget}},
		{Index: 2, Phase: InstancePhaseCreating, RunningRevision: revision},
		{Index: sharedTarget, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: revision},
	}
	want := []int32{0, 1, 2, 3}
	if diff := cmp.Diff(want, instancePlanIndices(instances, 2)); diff != "" {
		t.Fatalf("wrong-phase migration reference released a valid source (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_MixedHandoffsSharingTargetKeepAllParticipants(t *testing.T) {
	const (
		oldRevision = "comp-rev-oldbbbb"
		newRevision = "comp-rev-newaaaa"
	)
	sharedTarget := int32(3)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseMigrating, RunningRevision: newRevision,
			Operation: &InstanceOperation{Type: InstanceOperationMigrate, RequestUUID: "request-a", SurgeIndex: &sharedTarget}},
		{Index: 1, Phase: InstancePhaseUpdating, RunningRevision: oldRevision, TargetRevision: newRevision,
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepSurgeDrain,
				SurgeIndex: &sharedTarget, TargetRevision: newRevision}},
		{Index: 2, Phase: InstancePhaseCreating, RunningRevision: oldRevision},
		{Index: sharedTarget, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: newRevision},
	}
	want := []int32{0, 1, 2, 3}
	if diff := cmp.Diff(want, instancePlanIndices(instances, 3)); diff != "" {
		t.Fatalf("mixed handoffs sharing one target released a participant (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_GangSurgePinsPair(t *testing.T) {
	// replicas=2 with a gang surge in flight: source 0 (Updating, surge→3),
	// unrelated sibling 1 (Ready), replacement 3 (GangSurgeTarget). The
	// plan must include all three — the source counts toward the steady
	// budget (so sibling 1 isn't dropped into scale-down) and the surge
	// target is pinned as the transient +1.
	k := int32(3)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseUpdating, Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: "Surge", SurgeIndex: &k}},
		{Index: 1, Phase: InstancePhaseReady},
		{Index: 3, Phase: InstancePhaseCreating, Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepGangSurgeTarget}},
	}
	got := instancePlanIndices(instances, 2)
	hit := map[int32]bool{}
	for _, idx := range got {
		hit[idx] = true
	}
	if !hit[0] || !hit[1] || !hit[3] {
		t.Errorf("plan must include source 0, sibling 1, surge target 3; got %v", got)
	}
}

func TestInstancePlanIndices_OrphanGangSurgeMarkerUnpinned(t *testing.T) {
	// Marker-liveness invariant: a GangSurgeTarget marker whose
	// source no longer carries a SurgeIndex operation referencing it is
	// an orphan — it must FALL OUT of the plan so the scale-down pass
	// reaps the dead replacement gang, instead of leaking it forever.
	// Shape: the corrective roll-back re-adopted the source back to
	// Ready (operation cleared) while the crashed surge gang's marker
	// is still present.
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseReady, RunningRevision: "comp-rev-v1aaaaaa"},
		{Index: 1, Phase: InstancePhaseFailed,
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepGangSurgeTarget}},
	}
	got := instancePlanIndices(instances, 1)
	if len(got) != 1 || got[0] != 0 {
		t.Errorf("orphan marker must be unpinned (plan {0}, marker 1 becomes a scale-down extra); got %v", got)
	}

	// Liveness counter-case: the SAME marker stays pinned while its
	// source op references it — a mid-flight healthy surge is untouched.
	k := int32(1)
	instances[0] = InstanceStatus{Index: 0, Phase: InstancePhaseUpdating, RunningRevision: "comp-rev-v1aaaaaa",
		Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: "Surge", SurgeIndex: &k}}
	got = instancePlanIndices(instances, 1)
	hit := map[int32]bool{}
	for _, idx := range got {
		hit[idx] = true
	}
	if !hit[0] || !hit[1] {
		t.Errorf("live surge pair must stay pinned; got %v", got)
	}
}

func TestInstancePlanIndices_GangSurgeReadyTargetRequiresDrainProof(t *testing.T) {
	newRev, oldRev := "comp-rev-newaaaa", "comp-rev-oldbbbb"
	zero := int32(0)
	statuses := func(step string) []InstanceStatus {
		return []InstanceStatus{
			{Index: 0, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: newRev},
			{Index: 1, Phase: InstancePhaseUpdating, RunningRevision: oldRev, TargetRevision: newRev,
				Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: step, SurgeIndex: &zero, TargetRevision: newRev}},
			{Index: 2, Phase: InstancePhaseReady, RunningRevision: newRev},
		}
	}

	// A Ready occupant does not prove that a fresh claim owns the target.
	if diff := cmp.Diff([]int32{0, 1, 2}, instancePlanIndices(statuses("Surge"), 2)); diff != "" {
		t.Fatalf("unconfirmed claim must stay pinned (-want +got):\n%s", diff)
	}

	// SurgeDrain follows target validation and is safe to release after the
	// replacement is promoted.
	if diff := cmp.Diff([]int32{0, 2}, instancePlanIndices(statuses(UpdateStepSurgeDrain), 2)); diff != "" {
		t.Fatalf("validated source should leave the steady plan (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_GangSurgeSourceRequiresPromotedTargetProof(t *testing.T) {
	const (
		oldRevision = "comp-rev-oldbbbb"
		newRevision = "comp-rev-newaaaa"
	)
	targetIndex := int32(2)
	validSource := InstanceStatus{Index: 0, Phase: InstancePhaseUpdating,
		RunningRevision: oldRevision, TargetRevision: newRevision,
		Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepSurgeDrain,
			SurgeIndex: &targetIndex, TargetRevision: newRevision}}
	validTarget := InstanceStatus{Index: targetIndex, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: newRevision}

	wrongIncarnation := validTarget
	wrongIncarnation.Incarnation = 2
	wrongOrdinal := validTarget
	wrongOrdinal.ActiveOrdinal = 1
	wrongTargetRevision := validTarget
	wrongTargetRevision.RunningRevision = oldRevision
	activeTarget := validTarget
	activeTarget.Operation = &InstanceOperation{Type: InstanceOperationUpdate}
	pinnedTarget := validTarget
	pinnedTarget.TargetRevision = newRevision
	wrongPhase := validSource
	wrongPhase.Phase = InstancePhaseFailed
	missingRunningRevision := validSource
	missingRunningRevision.RunningRevision = ""
	missingTargetRevision := validSource
	missingTargetRevision.TargetRevision = ""
	mismatchedTargetRevision := validSource
	mismatchedTargetRevision.TargetRevision = "comp-rev-other"

	tests := []struct {
		name   string
		source InstanceStatus
		target InstanceStatus
	}{
		{name: "target incarnation", source: validSource, target: wrongIncarnation},
		{name: "target active ordinal", source: validSource, target: wrongOrdinal},
		{name: "target running revision", source: validSource, target: wrongTargetRevision},
		{name: "target operation", source: validSource, target: activeTarget},
		{name: "target revision pin", source: validSource, target: pinnedTarget},
		{name: "source phase", source: wrongPhase, target: validTarget},
		{name: "source running revision", source: missingRunningRevision, target: validTarget},
		{name: "source target revision", source: missingTargetRevision, target: validTarget},
		{name: "source revision pin", source: mismatchedTargetRevision, target: validTarget},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			instances := []InstanceStatus{
				test.source,
				{Index: 1, Phase: InstancePhaseCreating, RunningRevision: oldRevision},
				test.target,
			}
			want := []int32{0, 1, 2}
			if diff := cmp.Diff(want, instancePlanIndices(instances, 2)); diff != "" {
				t.Fatalf("unproven gang-surge target released its source (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInstancePlanIndices_GangSurgeRetiringSourceDoesNotDisplaceSteadyInstance(t *testing.T) {
	const (
		oldRevision = "comp-rev-oldbbbb"
		newRevision = "comp-rev-newaaaa"
	)
	targetIndex := int32(8)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseUpdating, RunningRevision: oldRevision, TargetRevision: newRevision,
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepSurgeDrain,
				SurgeIndex: &targetIndex, TargetRevision: newRevision}},
		{Index: 1, Phase: InstancePhaseReady, RunningRevision: oldRevision},
		{Index: 2, Phase: InstancePhaseReady, RunningRevision: oldRevision},
		{Index: 3, Phase: InstancePhaseReady, RunningRevision: oldRevision},
		{Index: 4, Phase: InstancePhaseReady, RunningRevision: oldRevision},
		{Index: 5, Phase: InstancePhaseReady, RunningRevision: oldRevision},
		{Index: 6, Phase: InstancePhaseReady, RunningRevision: oldRevision},
		{Index: 7, Phase: InstancePhaseCreating, TargetRevision: oldRevision},
		{Index: targetIndex, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: newRevision},
	}
	want := []int32{1, 2, 3, 4, 5, 6, 7, 8}
	indices := instancePlanIndices(instances, 8)
	if diff := cmp.Diff(want, indices); diff != "" {
		t.Fatalf("retiring gang-surge source displaced a steady instance (-want +got):\n%s", diff)
	}
	planned := make([]InstancePlan, 0, len(indices))
	for _, index := range indices {
		planned = append(planned, InstancePlan{Index: index})
	}
	if diff := cmp.Diff([]int32{0}, ExtraInstanceIndices(instances, ComponentPlan{Instances: planned}, false)); diff != "" {
		t.Fatalf("normal scale-down ownership crossed the retiring update source (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_ConcurrentRetiringSourcesDoNotDisplaceSteadyInstances(t *testing.T) {
	const (
		oldRevision = "comp-rev-oldbbbb"
		newRevision = "comp-rev-newaaaa"
	)
	targetEight, targetNine := int32(8), int32(9)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseUpdating, RunningRevision: oldRevision, TargetRevision: newRevision,
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepSurgeDrain,
				SurgeIndex: &targetEight, TargetRevision: newRevision}},
		{Index: 1, Phase: InstancePhaseUpdating, RunningRevision: oldRevision, TargetRevision: newRevision,
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepSurgeDrain,
				SurgeIndex: &targetNine, TargetRevision: newRevision}},
		{Index: 2, Phase: InstancePhaseReady, RunningRevision: oldRevision},
		{Index: 3, Phase: InstancePhaseReady, RunningRevision: oldRevision},
		{Index: 4, Phase: InstancePhaseReady, RunningRevision: oldRevision},
		{Index: 5, Phase: InstancePhaseReady, RunningRevision: oldRevision},
		{Index: 6, Phase: InstancePhaseCreating, TargetRevision: oldRevision},
		{Index: 7, Phase: InstancePhaseCreating, TargetRevision: oldRevision},
		{Index: targetEight, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: newRevision},
		{Index: targetNine, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: newRevision},
	}
	want := []int32{2, 3, 4, 5, 6, 7, 8, 9}
	if diff := cmp.Diff(want, instancePlanIndices(instances, 8)); diff != "" {
		t.Fatalf("retiring gang-surge sources displaced steady instances (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_SharedUpdateTargetKeepsAllParticipants(t *testing.T) {
	const (
		oldRevision = "comp-rev-oldbbbb"
		newRevision = "comp-rev-newaaaa"
	)
	sharedTarget := int32(3)
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseUpdating, RunningRevision: oldRevision, TargetRevision: newRevision,
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepSurgeDrain,
				SurgeIndex: &sharedTarget, TargetRevision: newRevision}},
		{Index: 1, Phase: InstancePhaseUpdating, RunningRevision: oldRevision, TargetRevision: newRevision,
			Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepSurgeDrain,
				SurgeIndex: &sharedTarget, TargetRevision: newRevision}},
		{Index: 2, Phase: InstancePhaseCreating, TargetRevision: oldRevision},
		{Index: sharedTarget, Incarnation: 1, Phase: InstancePhaseReady, RunningRevision: newRevision},
	}
	want := []int32{0, 1, 2, 3}
	if diff := cmp.Diff(want, instancePlanIndices(instances, 3)); diff != "" {
		t.Fatalf("shared update target released a source or sibling (-want +got):\n%s", diff)
	}
}

func TestInstancePlanIndices_NonRetiringSelectionControls(t *testing.T) {
	tests := []struct {
		name      string
		instances []InstanceStatus
	}{
		{
			name: "Ready Instance keeps priority",
			instances: []InstanceStatus{
				{Index: 0, Phase: InstancePhaseReady},
				{Index: 1, Phase: InstancePhaseCreating},
				{Index: 2, Phase: InstancePhaseReady},
			},
		},
		{
			name: "oldest non-Ready Instance remains the fallback",
			instances: []InstanceStatus{
				{Index: 0, Phase: InstancePhaseCreating},
				{Index: 1, Phase: InstancePhaseCreating},
				{Index: 2, Phase: InstancePhaseReady},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := []int32{0, 2}
			if diff := cmp.Diff(want, instancePlanIndices(test.instances, 2)); diff != "" {
				t.Fatalf("non-retiring selection changed (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInstancePlanIndices_SinglePodSurgeNotPinnedAsPair(t *testing.T) {
	// A single-pod surge carries Op.Step=Surge but NO SurgeIndex (it
	// toggles ActiveOrdinal in place). It must NOT be treated as a gang
	// surge pair — replicas=1, one Updating instance → plan stays {0}, no
	// phantom surge index.
	instances := []InstanceStatus{
		{Index: 0, Phase: InstancePhaseUpdating, Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: "Surge"}},
	}
	got := instancePlanIndices(instances, 1)
	if len(got) != 1 || got[0] != 0 {
		t.Errorf("single-pod surge must plan exactly {0}; got %v", got)
	}
}

func TestAllocateSurgeIndex(t *testing.T) {
	// Smoke: with statuses {0, 1, 3}, surge should land at 2.
	instances := []InstanceStatus{
		{Index: 0}, {Index: 1}, {Index: 3},
	}
	if got := AllocateSurgeIndex(instances); got != 2 {
		t.Errorf("got %d want 2", got)
	}
	// Empty status returns 0.
	if got := AllocateSurgeIndex(nil); got != 0 {
		t.Errorf("empty: got %d want 0", got)
	}
	// Shared-surgeIndex regression: an in-flight surge slot
	// recorded in Operation.SurgeIndex must be excluded even though no Instance
	// with that Index exists yet. Without this, every Instance surging in one
	// reconcile pass collides on the same lowest-free index, and when that single
	// shared surge becomes Ready the drain-on-ready logic releases ALL sharing
	// sources at once — the full-fleet wipe. With instances {0(surge→2), 1},
	// indices {0,1,2} are taken, so the next surge must land at 3, not 2.
	surgeAt2 := int32(2)
	withInflightSurge := []InstanceStatus{
		{Index: 0, Operation: &InstanceOperation{SurgeIndex: &surgeAt2}},
		{Index: 1},
	}
	if got := AllocateSurgeIndex(withInflightSurge); got != 3 {
		t.Errorf("in-flight surge slot at 2 must be excluded: got %d want 3", got)
	}
}

// TestPartitionHeldIndices pins the canary hold-membership rule: the
// lowest-indexed `Partition` Instances observed OFF the target revision
// are held; Instances converging to target (mid-update, preserved
// Update op — including gang-surge target markers), transient migration
// surges, and unobserved Instances are never hold candidates. Holding a
// mid-surge Instance would strand its surge pod (never promoted to
// serving) and permanently consume the maxSurge budget, deadlocking the
// roll — the hold falls to the next-lowest old-revision Instance
// instead.
func TestPartitionHeldIndices(t *testing.T) {
	p := func(v int32) *int32 { return &v }
	planFor := func(indices ...int32) []InstancePlan {
		out := make([]InstancePlan, 0, len(indices))
		for _, idx := range indices {
			out = append(out, InstancePlan{Index: idx})
		}
		return out
	}
	const target = "owner-engine-target"
	cases := []struct {
		name     string
		ru       *RollingUpdate
		observed []InstanceStatus
		planned  []InstancePlan
		want     map[int32]bool
	}{
		{
			name:    "nil rollingUpdate holds nothing",
			ru:      nil,
			planned: planFor(0, 1),
			observed: []InstanceStatus{
				{Index: 0, Phase: InstancePhaseReady, RunningRevision: "old"},
				{Index: 1, Phase: InstancePhaseReady, RunningRevision: "old"},
			},
			want: map[int32]bool{},
		},
		{
			name:    "lowest old-revision Instances held, count = Partition",
			ru:      &RollingUpdate{Partition: p(2)},
			planned: planFor(0, 1, 2),
			observed: []InstanceStatus{
				{Index: 0, Phase: InstancePhaseReady, RunningRevision: "old"},
				{Index: 1, Phase: InstancePhaseReady, RunningRevision: "old"},
				{Index: 2, Phase: InstancePhaseReady, RunningRevision: "old"},
			},
			want: map[int32]bool{0: true, 1: true},
		},
		{
			name:    "on-target Instance is past holding; hold keys to revision, not position",
			ru:      &RollingUpdate{Partition: p(1)},
			planned: planFor(0, 1),
			observed: []InstanceStatus{
				{Index: 0, Phase: InstancePhaseReady, RunningRevision: target},
				{Index: 1, Phase: InstancePhaseReady, RunningRevision: "old"},
			},
			want: map[int32]bool{1: true},
		},
		{
			name:    "mid-update Instance must finish — hold falls to next old-revision Instance",
			ru:      &RollingUpdate{Partition: p(1)},
			planned: planFor(0, 1),
			observed: []InstanceStatus{
				{Index: 0, Phase: InstancePhaseUpdating, RunningRevision: "old",
					Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: "Surge"}},
				{Index: 1, Phase: InstancePhaseReady, RunningRevision: "old"},
			},
			want: map[int32]bool{1: true},
		},
		{
			name:    "Failed Update continuation and gang-surge target marker are never held",
			ru:      &RollingUpdate{Partition: p(2)},
			planned: planFor(0, 1, 2),
			observed: []InstanceStatus{
				{Index: 0, Phase: InstancePhaseCreating, RunningRevision: "",
					Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: UpdateStepGangSurgeTarget}},
				{Index: 1, Phase: InstancePhaseFailed, RunningRevision: "old",
					Operation: &InstanceOperation{Type: InstanceOperationUpdate, Step: "Drain"}},
				{Index: 2, Phase: InstancePhaseReady, RunningRevision: "old"},
			},
			want: map[int32]bool{2: true},
		},
		{
			name:    "migration surge target excluded; Migrating source stays a candidate",
			ru:      &RollingUpdate{Partition: p(1)},
			planned: planFor(1, 5),
			observed: []InstanceStatus{
				{Index: 1, Phase: InstancePhaseMigrating, RunningRevision: "old",
					Operation: &InstanceOperation{Type: InstanceOperationMigrate}},
				{Index: 5, Phase: InstancePhaseCreating, RunningRevision: "",
					Operation: &InstanceOperation{Type: InstanceOperationMigrate}},
			},
			want: map[int32]bool{1: true},
		},
		{
			name:     "unobserved planned Instances are not candidates",
			ru:       &RollingUpdate{Partition: p(2)},
			planned:  planFor(0, 1),
			observed: []InstanceStatus{{Index: 1, Phase: InstancePhaseReady, RunningRevision: "old"}},
			want:     map[int32]bool{1: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := PartitionHeldIndices(tc.ru, tc.observed, tc.planned, target)
			if len(got) != len(tc.want) {
				t.Fatalf("held = %v, want %v", got, tc.want)
			}
			for idx := range tc.want {
				if !got[idx] {
					t.Errorf("held = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
