package workload_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocktesting "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

// Plan is the pure decision layer: the tests below assert the Decision
// it produces for each precedence branch — which pass-level actions are
// selected, in what order — and that producing it performs ZERO
// mutations (every ReconcileInput mutation callback is wired to a
// recorder that fails the test on invocation).

// forbidMutations wires every mutation callback on input to a t.Error
// recorder, so any write attempted during Plan fails the test.
func forbidMutations(t *testing.T, input *workload.ReconcileInput) {
	t.Helper()
	input.MutateInstance = func(_ context.Context, idx int32, _ func(*workload.InstanceStatus) bool) error {
		t.Errorf("Plan must not call MutateInstance (idx=%d)", idx)
		return nil
	}
	input.ApplyInstanceMutations = func(_ context.Context, muts []workload.InstanceMutation) error {
		t.Errorf("Plan must not call ApplyInstanceMutations (%d mutations)", len(muts))
		return nil
	}
	input.RemoveInstance = func(_ context.Context, idx int32) (bool, error) {
		t.Errorf("Plan must not call RemoveInstance (idx=%d)", idx)
		return false, nil
	}
	input.WriteAggregateCondition = func(_ context.Context, cond metav1.Condition) error {
		t.Errorf("Plan must not call WriteAggregateCondition (%s)", cond.Type)
		return nil
	}
	input.WarnInstanceFailed = func(idx int32, _, _ string) {
		t.Errorf("Plan must not call WarnInstanceFailed (idx=%d)", idx)
	}
	input.WarnRetryHeld = func(rev string, _ int32, _ string) {
		t.Errorf("Plan must not call WarnRetryHeld (%s)", rev)
	}
	input.MutateMigration = func(_ context.Context, uuid string, _ func(*workload.MigrationRecord) bool) error {
		t.Errorf("Plan must not call MutateMigration (%s)", uuid)
		return nil
	}
	input.AppendMigration = func(_ context.Context, rec workload.MigrationRecord) error {
		t.Errorf("Plan must not call AppendMigration (%s)", rec.RequestUUID)
		return nil
	}
	input.MutateRetryBlock = func(_ context.Context, rev string, _ func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
		t.Errorf("Plan must not call MutateRetryBlock (%s)", rev)
		return nil
	}
	input.UpdateGate = func(_ workload.UpdateStrategyType, _, _ int32) (bool, string) {
		t.Error("Plan must not consult UpdateGate (Execute owns the consult)")
		return true, ""
	}
}

// planSnapshot builds a snapshot over pre-bucketed pods (both read
// sources) so Plan needs no client.
func planSnapshot(input workload.ReconcileInput, byIdx map[int32][]*corev1.Pod) *workload.ObservedSnapshot {
	return workload.SnapshotWithPodsForTest(input, byIdx)
}

// planOrFail runs Plan (nil target) and fails the test on error.
func planOrFail(t *testing.T, input workload.ReconcileInput, plan workload.ComponentPlan, snapshot *workload.ObservedSnapshot) workload.Decision {
	t.Helper()
	return planTargetOrFail(t, input, plan, nil, snapshot)
}

// planTargetOrFail runs Plan against a target ControllerRevision and
// fails the test on error.
func planTargetOrFail(t *testing.T, input workload.ReconcileInput, plan workload.ComponentPlan, target *appsv1.ControllerRevision, snapshot *workload.ObservedSnapshot) workload.Decision {
	t.Helper()
	d, err := workload.Plan(context.Background(), input, plan, target, snapshot)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return d
}

// actionKinds projects the Decision's action kinds in order.
func actionKinds(d workload.Decision) []workload.ActionKind {
	kinds := make([]workload.ActionKind, 0, len(d.Actions))
	for _, a := range d.Actions {
		kinds = append(kinds, a.Kind)
	}
	return kinds
}

// kindsEqual compares two ordered ActionKind slices.
func kindsEqual(got, want []workload.ActionKind) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// findAction returns the first action of the given kind, or nil.
func findAction(d workload.Decision, kind workload.ActionKind) *workload.PlannedAction {
	for i := range d.Actions {
		if d.Actions[i].Kind == kind {
			return &d.Actions[i]
		}
	}
	return nil
}

// TestPlan_ScaleDown_SelectsExtras asserts an observed index the plan
// no longer covers is selected for scale-down, and nothing is selected
// when observation matches the plan.
func TestPlan_ScaleDown_SelectsExtras(t *testing.T) {
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Phase: workload.InstancePhaseReady},
		{Index: 1, Phase: workload.InstancePhaseReady}, // extra
	}
	plan := minimalPlan() // covers only index 0

	d := planOrFail(t, in, plan, planSnapshot(in, nil))
	sd := findAction(d, workload.ActionScaleDown)
	if sd == nil {
		t.Fatalf("expected a ScaleDown action, got %v", actionKinds(d))
	}
	if len(sd.Extras) != 1 || sd.Extras[0] != 1 {
		t.Errorf("ScaleDown extras = %v, want [1]", sd.Extras)
	}

	// Converged shape: no extras → no ScaleDown action.
	in.ObservedState.InstanceStatuses = in.ObservedState.InstanceStatuses[:1]
	d = planOrFail(t, in, plan, planSnapshot(in, nil))
	if findAction(d, workload.ActionScaleDown) != nil {
		t.Errorf("converged plan must not select ScaleDown, got %v", actionKinds(d))
	}
}

func TestPlan_DeleteOwnedDesiredIndexFinishesBeforeRecreate(t *testing.T) {
	input := minimalInput(t)
	forbidMutations(t, &input)
	plan := minimalPlan()
	input.ObservedState.InstanceStatuses = []workload.InstanceStatus{{
		Index: 0, Incarnation: 3, Phase: workload.InstancePhaseDeleting,
		Operation: &workload.InstanceOperation{ID: "delete-0", Type: workload.InstanceOperationDelete, Step: "Drain"},
	}}
	decision := planOrFail(t, input, plan, planSnapshot(input, nil))
	if len(decision.Actions) == 0 || decision.Actions[0].Kind != workload.ActionScaleDown {
		t.Fatalf("actions = %v, want ScaleDown first", actionKinds(decision))
	}
	if len(decision.Actions[0].Extras) != 0 {
		t.Fatalf("desired Delete-owned index must not be reclassified as a fresh extra: %v", decision.Actions[0].Extras)
	}
}

// TestPlan_Restart_SelectsTriggeredInstances asserts the restart
// selection: a Ready Instance whose live pod count is below desired is
// selected (with the trigger reason), healthy Instances are not, and
// the pass is skipped entirely for non-RecreateInstance policies.
func TestPlan_Restart_SelectsTriggeredInstances(t *testing.T) {
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady},
		{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseReady},
	}
	plan := workload.ComponentPlan{
		Component:     workload.ComponentEngine,
		Replicas:      2,
		RestartPolicy: workload.RestartPolicyRecreateInstance,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
	}
	// Index 0 has its pod; index 1 lost its pod (restart trigger).
	pods := map[int32][]*corev1.Pod{
		0: {enginePod("llama-70b", "prod", 0)},
	}

	d := planOrFail(t, in, plan, planSnapshot(in, pods))
	ra := findAction(d, workload.ActionRestart)
	if ra == nil {
		t.Fatalf("expected a Restart action, got %v", actionKinds(d))
	}
	if len(ra.Restarts) != 1 || ra.Restarts[0].Instance.Index != 1 {
		t.Fatalf("restart selection = %+v, want index 1 only", ra.Restarts)
	}
	if ra.Restarts[0].Reason == "" {
		t.Errorf("restart selection must carry the trigger reason")
	}

	// Same shape without the RecreateInstance policy: no restart pass.
	plan.RestartPolicy = ""
	d = planOrFail(t, in, plan, planSnapshot(in, pods))
	if findAction(d, workload.ActionRestart) != nil {
		t.Errorf("restart must not be planned without RestartPolicyRecreateInstance, got %v", actionKinds(d))
	}
}

// TestPlan_ScaleDownPrecedesRestart asserts the precedence ordering:
// when both an extra index and a restart trigger exist, ScaleDown is
// planned before Restart.
func TestPlan_ScaleDownPrecedesRestart(t *testing.T) {
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady}, // pod lost → restart
		{Index: 5, Incarnation: 1, Phase: workload.InstancePhaseReady}, // extra → scale-down
	}
	plan := minimalPlan()
	plan.RestartPolicy = workload.RestartPolicyRecreateInstance

	d := planOrFail(t, in, plan, planSnapshot(in, nil))
	want := []workload.ActionKind{workload.ActionScaleDown, workload.ActionRestart, workload.ActionCreate}
	if !kindsEqual(actionKinds(d), want) {
		t.Errorf("decision = %v, want %v", actionKinds(d), want)
	}
}

// TestPlan_FullPrecedenceOrder pins the complete precedence: with
// every pass triggered at once, the Decision lists scale-down >
// restart > migration expiry > migration > update > create, in that
// order. Selection is not execution — the executor stops at the first
// action whose op outcome requires a requeue, so a later-listed
// selection may not run this reconcile.
func TestPlan_FullPrecedenceOrder(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	target := updateTarget()
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.Clock = clocktesting.NewFakeClock(now)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		// Pod lost → restart trigger; prior revision → update trigger.
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
		// Extra → scale-down.
		{Index: 9, Incarnation: 1, Phase: workload.InstancePhaseReady},
	}
	in.ObservedState.Migrations = []workload.MigrationRecord{
		{RequestUUID: "u-expired", Trigger: workload.MigrationTriggerManual,
			Phase: workload.MigrationPhaseAccepted, SourceInstance: 0,
			Deadline: metav1.NewTime(now.Add(-time.Minute))},
		{RequestUUID: "u-drive", Trigger: workload.MigrationTriggerManual,
			Phase: workload.MigrationPhaseAccepted, SourceInstance: 0,
			StartedAt: metav1.NewTime(now.Add(-time.Hour))},
	}
	plan := minimalPlan()
	plan.RestartPolicy = workload.RestartPolicyRecreateInstance
	plan.MigrationMode = workload.MigrationModeAuto

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	want := []workload.ActionKind{
		workload.ActionScaleDown,
		workload.ActionRestart,
		workload.ActionMigrateExpiry,
		workload.ActionMigrate,
		workload.ActionUpdate,
		workload.ActionCreate,
	}
	if !kindsEqual(actionKinds(d), want) {
		t.Errorf("decision = %v, want %v", actionKinds(d), want)
	}
	if !d.Escalate {
		t.Errorf("non-paused decision must enable escalation")
	}
}

// TestPlan_Create_AlwaysPlannedUnlessPaused asserts the create pass
// closes every non-paused decision, last in precedence.
func TestPlan_Create_AlwaysPlannedUnlessPaused(t *testing.T) {
	in := minimalInput(t)
	forbidMutations(t, &in)
	plan := minimalPlan()

	d := planOrFail(t, in, plan, planSnapshot(in, nil))
	kinds := actionKinds(d)
	if len(kinds) == 0 || kinds[len(kinds)-1] != workload.ActionCreate {
		t.Errorf("decision = %v, want Create last", kinds)
	}

	plan.Paused = true
	d = planOrFail(t, in, plan, planSnapshot(in, nil))
	if findAction(d, workload.ActionCreate) != nil {
		t.Errorf("paused decision must not plan Create, got %v", actionKinds(d))
	}
}

// TestPlan_MigrateExpiry_SelectedFromDeadline asserts the expiry
// selection: a non-terminal Manual record past its Deadline plans
// MigrateExpiry even when MigrationMode=Never (a mode flip must not
// strand the record), ordered before the drive action; a record within
// its Deadline plans no expiry.
func TestPlan_MigrateExpiry_SelectedFromDeadline(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.Clock = clocktesting.NewFakeClock(now)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Phase: workload.InstancePhaseReady},
	}
	in.ObservedState.Migrations = []workload.MigrationRecord{{
		RequestUUID: "u-expired", Trigger: workload.MigrationTriggerManual,
		Phase: workload.MigrationPhaseAccepted, SourceInstance: 0,
		Deadline: metav1.NewTime(now.Add(-time.Minute)),
	}}
	plan := minimalPlan()
	plan.MigrationMode = workload.MigrationModeNever

	d := planOrFail(t, in, plan, planSnapshot(in, nil))
	if findAction(d, workload.ActionMigrateExpiry) == nil {
		t.Errorf("expired record must plan MigrateExpiry, got %v", actionKinds(d))
	}
	if findAction(d, workload.ActionMigrate) != nil {
		t.Errorf("MigrationMode=Never must not plan a Migrate drive, got %v", actionKinds(d))
	}

	// Same record still within its Deadline: no expiry.
	in.ObservedState.Migrations[0].Deadline = metav1.NewTime(now.Add(time.Minute))
	d = planOrFail(t, in, plan, planSnapshot(in, nil))
	if findAction(d, workload.ActionMigrateExpiry) != nil {
		t.Errorf("unexpired record must not plan MigrateExpiry, got %v", actionKinds(d))
	}
}

// TestPlan_Migrate_SelectsOldestManualRecord asserts the drive
// selection: terminal and Auto records are never work; the oldest
// non-terminal Manual record is selected; expiry precedes the drive.
func TestPlan_Migrate_SelectsOldestManualRecord(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.Clock = clocktesting.NewFakeClock(now)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Phase: workload.InstancePhaseReady},
	}
	in.ObservedState.Migrations = []workload.MigrationRecord{
		{RequestUUID: "u-done", Trigger: workload.MigrationTriggerManual,
			Phase: workload.MigrationPhaseCompleted, SourceInstance: 0},
		{RequestUUID: "u-auto", Trigger: workload.MigrationTriggerAuto,
			Phase: workload.MigrationPhaseRelocated, SourceInstance: 0},
		{RequestUUID: "u-newer", Trigger: workload.MigrationTriggerManual,
			Phase: workload.MigrationPhaseAccepted, SourceInstance: 0,
			StartedAt: metav1.NewTime(now.Add(-time.Minute)),
			// Expired: also plans the expiry action ahead of the drive.
			Deadline: metav1.NewTime(now.Add(-time.Second))},
		{RequestUUID: "u-older", Trigger: workload.MigrationTriggerManual,
			Phase: workload.MigrationPhaseSurgePending, SourceInstance: 0,
			StartedAt: metav1.NewTime(now.Add(-time.Hour))},
	}
	plan := minimalPlan()
	plan.MigrationMode = workload.MigrationModeAuto

	d := planOrFail(t, in, plan, planSnapshot(in, nil))
	kinds := actionKinds(d)
	wantPrefix := []workload.ActionKind{workload.ActionMigrateExpiry, workload.ActionMigrate}
	if len(kinds) < 2 || !kindsEqual(kinds[:2], wantPrefix) {
		t.Fatalf("decision = %v, want prefix %v", kinds, wantPrefix)
	}
	ma := findAction(d, workload.ActionMigrate)
	if ma.Migration == nil || ma.Migration.Record.RequestUUID != "u-older" {
		t.Errorf("drive selection = %+v, want oldest non-terminal Manual record u-older", ma.Migration)
	}

	// Only terminal/Auto records: nothing to drive.
	in.ObservedState.Migrations = in.ObservedState.Migrations[:2]
	d = planOrFail(t, in, plan, planSnapshot(in, nil))
	if findAction(d, workload.ActionMigrate) != nil || findAction(d, workload.ActionMigrateExpiry) != nil {
		t.Errorf("terminal/Auto records must plan no migration work, got %v", actionKinds(d))
	}
}

func TestPlan_MigrationSurgeWaitsForUpdateSurge(t *testing.T) {
	minusOne := int32(-1)
	allocated := int32(3)
	tests := []struct {
		name          string
		updateStep    string
		surgeInstance *int32
		wantMigrate   bool
	}{
		{name: "fresh record waits", updateStep: "Surge", wantMigrate: false},
		{name: "legacy fresh record waits", updateStep: "Surge", surgeInstance: &minusOne, wantMigrate: false},
		{name: "allocated record resumes", updateStep: "Surge", surgeInstance: &allocated, wantMigrate: true},
		{name: "non-surge update does not block", updateStep: "InPlace", wantMigrate: true},
		{name: "no update operation", wantMigrate: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := minimalInput(t)
			forbidMutations(t, &input)
			statuses := []workload.InstanceStatus{
				{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
				{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
				{Index: 2, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
			}
			if test.updateStep != "" {
				statuses[0].Phase = workload.InstancePhaseUpdating
				statuses[0].Operation = &workload.InstanceOperation{
					Type: workload.InstanceOperationUpdate,
					Step: test.updateStep,
				}
			}
			input.ObservedState.InstanceStatuses = statuses
			phase := workload.MigrationPhaseAccepted
			if test.surgeInstance != nil && *test.surgeInstance >= 0 {
				phase = workload.MigrationPhaseSurgePending
			}
			input.ObservedState.Migrations = []workload.MigrationRecord{{
				RequestUUID:    "migration-during-update",
				Trigger:        workload.MigrationTriggerManual,
				Phase:          phase,
				SourceInstance: 2,
				SurgeInstance:  test.surgeInstance,
			}}
			plan := workload.ComponentPlan{
				Component:      workload.ComponentEngine,
				Replicas:       3,
				MigrationMode:  workload.MigrationModeAuto,
				UpdateStrategy: workload.UpdateStrategy{Type: workload.UpdateStrategySurgeThenDrain},
				Instances: []workload.InstancePlan{
					{Index: 0, Incarnation: 1},
					{Index: 1, Incarnation: 1},
					{Index: 2, Incarnation: 1},
				},
			}

			decision := planTargetOrFail(t, input, plan, updateTarget(), planSnapshot(input, nil))
			if got := findAction(decision, workload.ActionMigrate) != nil; got != test.wantMigrate {
				t.Errorf("Migrate selected = %v, want %v; actions=%v", got, test.wantMigrate, actionKinds(decision))
			}
			if test.updateStep != "" && findAction(decision, workload.ActionUpdate) == nil {
				t.Errorf("Update continuation missing; actions=%v", actionKinds(decision))
			}
		})
	}
}

// updateTarget is the target ControllerRevision the update-selection
// tests roll toward.
func updateTarget() *appsv1.ControllerRevision {
	return &appsv1.ControllerRevision{
		ObjectMeta: metav1.ObjectMeta{Name: "llama-70b-engine-newtarget", Namespace: "prod"},
	}
}

func TestPlan_RemovableInlineFieldsDoNotChangeDecision(t *testing.T) {
	target := updateTarget()
	plan := workload.ComponentPlan{
		Component:     workload.ComponentEngine,
		Replicas:      1,
		RestartPolicy: workload.RestartPolicyRecreateInstance,
		Instances: []workload.InstancePlan{{
			Index: 0, Incarnation: 1,
			Runners: []workload.RunnerPlan{{Name: "leader", Size: 1}, {Name: "worker", Size: 1}},
		}},
	}
	podA := enginePod("llama-70b", "prod", 0)
	podB := enginePod("llama-70b", "prod", 0)
	podB.Name += "-worker"
	staleCachedPod := podA.DeepCopy()
	staleCachedPod.Spec.Containers[0].Image = "test:v0"
	cached := map[int32][]*corev1.Pod{0: {staleCachedPod}}

	tests := []struct {
		name         string
		live         map[int32][]*corev1.Pod
		inlineFields workload.InstanceStatus
		wantActions  []workload.ActionKind
	}{
		{
			name: "persisted observation leads partial live gang",
			live: map[int32][]*corev1.Pod{0: {podA}},
			inlineFields: workload.InstanceStatus{
				ReadyPodCount: 2, ScheduledPodCount: 2, NodesOccupied: []string{"node-a", "node-b"},
			},
			wantActions: []workload.ActionKind{workload.ActionRestart, workload.ActionUpdate, workload.ActionCreate},
		},
		{
			name: "persisted observation trails complete live gang",
			live: map[int32][]*corev1.Pod{0: {podA, podB}},
			inlineFields: workload.InstanceStatus{
				ReadyPodCount: 1, ScheduledPodCount: 1, NodesOccupied: []string{"node-a"},
			},
			wantActions: []workload.ActionKind{workload.ActionUpdate, workload.ActionCreate},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withoutFields := minimalInput(t)
			forbidMutations(t, &withoutFields)
			withoutFields.ObservedState.InstanceStatuses = []workload.InstanceStatus{{
				Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady, PodCount: 2,
			}}
			withFields := minimalInput(t)
			forbidMutations(t, &withFields)
			row := withoutFields.ObservedState.InstanceStatuses[0]
			row.ReadyPodCount = test.inlineFields.ReadyPodCount
			row.ScheduledPodCount = test.inlineFields.ScheduledPodCount
			row.NodesOccupied = test.inlineFields.NodesOccupied
			withFields.ObservedState.InstanceStatuses = []workload.InstanceStatus{row}

			withoutDecision := planTargetOrFail(t, withoutFields, plan, target,
				workload.SnapshotWithDistinctPodsForTest(withoutFields, test.live, cached))
			withDecision := planTargetOrFail(t, withFields, plan, target,
				workload.SnapshotWithDistinctPodsForTest(withFields, test.live, cached))
			if !reflect.DeepEqual(withoutDecision, withDecision) {
				t.Fatalf("removable fields changed the full decision\nwithout values: %#v\nwith values:    %#v", withoutDecision, withDecision)
			}
			if got := actionKinds(withoutDecision); !kindsEqual(got, test.wantActions) {
				t.Fatalf("ordered actions = %v, want %v", got, test.wantActions)
			}
		})
	}
}

func TestPlanObservationReadFailureFailsClosed(t *testing.T) {
	wantErr := errors.New("pod observation failed")
	funcs := interceptor.Funcs{
		List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
			return wantErr
		},
	}
	failing := fake.NewClientBuilder().WithScheme(makeScheme(t)).WithInterceptorFuncs(funcs).Build()
	healthy := fake.NewClientBuilder().WithScheme(makeScheme(t)).Build()
	tests := []struct {
		name     string
		liveFail bool
		target   *appsv1.ControllerRevision
	}{
		{name: "live restart observation", liveFail: true},
		{name: "cached update observation", target: updateTarget()},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := minimalInput(t)
			forbidMutations(t, &input)
			input.ObservedState.InstanceStatuses = []workload.InstanceStatus{{
				Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-revision",
			}}
			plan := minimalPlan()
			deps := workload.Deps{Client: failing, APIReader: healthy}
			if test.liveFail {
				plan.RestartPolicy = workload.RestartPolicyRecreateInstance
				deps.Client, deps.APIReader = healthy, failing
			}
			snapshot := workload.NewObservedSnapshot(deps, input, plan.Component, input.ObservedState.InstanceStatuses)

			decision, err := workload.Plan(context.Background(), input, plan, test.target, snapshot)
			if !errors.Is(err, wantErr) {
				t.Fatalf("Plan error = %v, want %v", err, wantErr)
			}
			if !reflect.DeepEqual(decision, workload.Decision{}) {
				t.Fatalf("failed observation returned a partial decision: %#v", decision)
			}
		})
	}
}

// TestPlan_Update_SelectsTriggeredInstances asserts the update
// selection: an Instance on a prior revision is a fresh start, a
// mid-update Instance is a continuation, an Instance already on the
// target is not listed, and a nil target plans no update pass at all.
func TestPlan_Update_SelectsTriggeredInstances(t *testing.T) {
	target := updateTarget()
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
		{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseUpdating, RunningRevision: "prior-rev"},
		{Index: 2, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: target.Name},
	}
	plan := workload.ComponentPlan{
		Component: workload.ComponentEngine,
		Replicas:  3,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 2, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
	}

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	ua := findAction(d, workload.ActionUpdate)
	if ua == nil {
		t.Fatalf("expected an Update action, got %v", actionKinds(d))
	}
	items := ua.Update.Items
	if len(items) != 2 ||
		items[0].Instance.Index != 0 || !items[0].StartingFresh ||
		items[1].Instance.Index != 1 || items[1].StartingFresh {
		t.Errorf("update items = %+v, want [{0 fresh} {1 continuation}]", items)
	}
	// Empty strategy Type resolves to the SurgeThenDrain default; nil
	// RollingUpdate leaves both per-Component budgets uncapped.
	if ua.Update.Strategy != workload.UpdateStrategySurgeThenDrain {
		t.Errorf("strategy = %q, want SurgeThenDrain default", ua.Update.Strategy)
	}
	if ua.Update.SurgeBudget != workload.BudgetNoLimit || ua.Update.UnavailBudget != workload.BudgetNoLimit {
		t.Errorf("budgets = (%d, %d), want BudgetNoLimit for nil RollingUpdate",
			ua.Update.SurgeBudget, ua.Update.UnavailBudget)
	}

	// Nil target: the update pass is not planned.
	d = planOrFail(t, in, plan, planSnapshot(in, nil))
	if findAction(d, workload.ActionUpdate) != nil {
		t.Errorf("nil target must not plan an update pass, got %v", actionKinds(d))
	}
}

// TestPlan_Update_RetryBlockWait asserts a not-yet-due Backoff
// RetryBlock for the target denies the fresh start and surfaces the
// wake-up as Decision.RequeueAfter.
func TestPlan_Update_RetryBlockWait(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	target := updateTarget()
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.Clock = clocktesting.NewFakeClock(now)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
	}
	wake := metav1.NewTime(now.Add(30 * time.Second))
	in.ObservedState.RetryBlocks = []workload.RetryBlock{{
		TargetRevision: target.Name,
		State:          workload.RetryBlockBackoff,
		NextRetryAt:    &wake,
	}}
	plan := minimalPlan()

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	if findAction(d, workload.ActionUpdate) != nil {
		t.Errorf("a not-yet-due Backoff block must deny the fresh start, got %v", actionKinds(d))
	}
	if d.RequeueAfter != 30*time.Second {
		t.Errorf("RequeueAfter = %v, want 30s (the block's wake-up)", d.RequeueAfter)
	}
}

// TestPlan_Update_HeldByPartition asserts the canary hold: an Instance
// below the partition is not selected, unless it is already mid-surge
// (it must finish, not strand its surge pod).
func TestPlan_Update_HeldByPartition(t *testing.T) {
	target := updateTarget()
	partition := int32(1)
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
		{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
	}
	plan := workload.ComponentPlan{
		Component: workload.ComponentEngine,
		Replicas:  2,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
		UpdateStrategy: workload.UpdateStrategy{
			Type:          workload.UpdateStrategySurgeThenDrain,
			RollingUpdate: &workload.RollingUpdate{Partition: &partition},
		},
	}

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	ua := findAction(d, workload.ActionUpdate)
	if ua == nil {
		t.Fatalf("expected an Update action, got %v", actionKinds(d))
	}
	if len(ua.Update.Items) != 1 || ua.Update.Items[0].Instance.Index != 1 {
		t.Errorf("update items = %+v, want index 1 only (index 0 held)", ua.Update.Items)
	}

	// An Instance ALREADY mid-surge is never a hold candidate — it is
	// selected so it can finish (a held mid-surge Instance would strand
	// its surge pod and pin the surge budget). The hold falls to the
	// next old-revision Instance, preserving the Partition count: index
	// 1 is held instead, so exactly one Instance stays on the old
	// revision when the roll settles.
	in.ObservedState.InstanceStatuses[0].Phase = workload.InstancePhaseUpdating
	d = planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	ua = findAction(d, workload.ActionUpdate)
	if ua == nil || len(ua.Update.Items) != 1 || ua.Update.Items[0].Instance.Index != 0 {
		t.Errorf("mid-surge Instance must stay selected and the hold must fall to index 1; got %+v", ua)
	}
}

// TestPlan_Update_PartitionHoldsCountOnSparseIndices asserts the
// partition hold on a SPARSE index set (migration / lowest-unused
// surge allocation leave gaps): Partition counts Instances to hold —
// the lowest-indexed old-revision Instances — not raw index values.
// With old-revision indices {1,2} and Partition=1, exactly the lowest
// (index 1) is held; raw-index comparison would hold nothing and roll
// both.
func TestPlan_Update_PartitionHoldsCountOnSparseIndices(t *testing.T) {
	target := updateTarget()
	partition := int32(1)
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
		{Index: 2, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
	}
	plan := workload.ComponentPlan{
		Component: workload.ComponentEngine,
		Replicas:  2,
		Instances: []workload.InstancePlan{
			{Index: 1, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 2, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
		UpdateStrategy: workload.UpdateStrategy{
			Type:          workload.UpdateStrategySurgeThenDrain,
			RollingUpdate: &workload.RollingUpdate{Partition: &partition},
		},
	}

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	ua := findAction(d, workload.ActionUpdate)
	if ua == nil {
		t.Fatalf("expected an Update action, got %v", actionKinds(d))
	}
	if len(ua.Update.Items) != 1 || ua.Update.Items[0].Instance.Index != 2 {
		t.Errorf("update items = %+v, want index 2 only (index 1 held as rank 0)", ua.Update.Items)
	}

	// Partition == replicas holds the whole steady set: no update items.
	partition = 2
	d = planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	if findAction(d, workload.ActionUpdate) != nil {
		t.Errorf("Partition=replicas must hold every steady Instance, got %v", actionKinds(d))
	}
}

// TestPlan_Update_PartitionHoldSurvivesGangSurgeReindex asserts the
// hold is keyed to revision membership, not index position. Gang surge
// allocates the lowest unused index for its replacement, so a roll of
// {1,2} with Partition=1 promotes the target-revision replacement at
// index 0 — BELOW the held Instance at index 1. A positional hold
// would let the replacement steal the held slot and un-hold index 1,
// rolling it too: 100% blast radius under Partition=1 and the staged
// shape (Partition Instances Ready on the old revision) permanently
// unreachable. The on-target Instance is past holding; index 1 must
// stay held.
func TestPlan_Update_PartitionHoldSurvivesGangSurgeReindex(t *testing.T) {
	target := updateTarget()
	partition := int32(1)
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		// The promoted gang-surge replacement: landed at the freed
		// lowest index, already on the target revision.
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: target.Name},
		// The canary hold: still on the old revision.
		{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
	}
	plan := workload.ComponentPlan{
		Component: workload.ComponentEngine,
		Replicas:  2,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
		UpdateStrategy: workload.UpdateStrategy{
			Type:          workload.UpdateStrategySurgeThenDrain,
			RollingUpdate: &workload.RollingUpdate{Partition: &partition},
		},
	}

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	if ua := findAction(d, workload.ActionUpdate); ua != nil {
		t.Errorf("index 1 must stay held after the replacement re-indexed below it; got update items %+v", ua.Update.Items)
	}
}

// TestPlan_Update_StartingFresh_FailedByOperationType asserts the
// Failed-phase budget exemption is scoped to a preserved UPDATE
// operation — the only shape CurrentUnavailableInFlight charges as
// in-flight. Failed with a non-Update operation (e.g. an expired
// Restart) is not charged, so it must start fresh (budget-gated).
func TestPlan_Update_StartingFresh_FailedByOperationType(t *testing.T) {
	target := updateTarget()
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseFailed, RunningRevision: "prior-rev",
			Operation: &workload.InstanceOperation{Type: workload.InstanceOperationRestart, Step: "Drain"}},
		{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseFailed, RunningRevision: "prior-rev",
			Operation: &workload.InstanceOperation{Type: workload.InstanceOperationUpdate, Step: "Drain"}},
	}
	plan := workload.ComponentPlan{
		Component: workload.ComponentEngine,
		Replicas:  2,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
	}

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	ua := findAction(d, workload.ActionUpdate)
	if ua == nil {
		t.Fatalf("expected an Update action, got %v", actionKinds(d))
	}
	items := ua.Update.Items
	if len(items) != 2 ||
		items[0].Instance.Index != 0 || !items[0].StartingFresh ||
		items[1].Instance.Index != 1 || items[1].StartingFresh {
		t.Errorf("update items = %+v, want [{0 fresh (Failed+Restart)} {1 continuation (Failed+Update)}]", items)
	}
}

// TestPlan_Update_CoordGateExempt_FailedZeroServing asserts the
// coordination-gate exemption selection on a non-surge strategy: a
// Failed Instance with zero serving pods starts fresh but skips the
// gate consult (its outage is already inside the gate's serving-based
// unavailability count), while a Failed Instance still contributing
// serving pods keeps the consult (its recreate takes real capacity
// offline).
func TestPlan_Update_CoordGateExempt_FailedZeroServing(t *testing.T) {
	target := updateTarget()
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseFailed, RunningRevision: "prior-rev",
			Operation: &workload.InstanceOperation{Type: workload.InstanceOperationRestart, Step: "Drain"}},
		{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseFailed, RunningRevision: "prior-rev",
			PodCount: 1, ServingPodCount: 1},
		{Index: 2, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev",
			PodCount: 1, ServingPodCount: 1},
	}
	plan := workload.ComponentPlan{
		Component: workload.ComponentEngine,
		Replicas:  3,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 2, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
		UpdateStrategy: workload.UpdateStrategy{Type: workload.UpdateStrategyRecreatePod},
	}

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	ua := findAction(d, workload.ActionUpdate)
	if ua == nil {
		t.Fatalf("expected an Update action, got %v", actionKinds(d))
	}
	items := ua.Update.Items
	if len(items) != 3 {
		t.Fatalf("update items = %+v, want 3", items)
	}
	if !items[0].StartingFresh || !items[0].CoordGateExempt {
		t.Errorf("item 0 (Failed+Restart, zero serving) = %+v, want fresh + gate-exempt", items[0])
	}
	if !items[1].StartingFresh || items[1].CoordGateExempt {
		t.Errorf("item 1 (Failed but serving) = %+v, want fresh + gate-consulted", items[1])
	}
	if !items[2].StartingFresh || items[2].CoordGateExempt {
		t.Errorf("item 2 (healthy) = %+v, want fresh + gate-consulted", items[2])
	}
}

// TestPlan_Update_CoordGateExempt_SurgeKeepsConsult asserts the
// exemption never applies under SurgeThenDrain: the surge-side gates
// count surge pods, not serving loss, and a Failed Instance's
// recreate-via-surge genuinely adds one.
func TestPlan_Update_CoordGateExempt_SurgeKeepsConsult(t *testing.T) {
	target := updateTarget()
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseFailed, RunningRevision: "prior-rev",
			Operation: &workload.InstanceOperation{Type: workload.InstanceOperationRestart, Step: "Drain"}},
	}
	plan := workload.ComponentPlan{
		Component: workload.ComponentEngine,
		Replicas:  1,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
		UpdateStrategy: workload.UpdateStrategy{Type: workload.UpdateStrategySurgeThenDrain},
	}

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, nil))
	ua := findAction(d, workload.ActionUpdate)
	if ua == nil {
		t.Fatalf("expected an Update action, got %v", actionKinds(d))
	}
	if len(ua.Update.Items) != 1 || ua.Update.Items[0].CoordGateExempt {
		t.Errorf("update items = %+v, want one gate-consulted item", ua.Update.Items)
	}
}

// TestPlan_Update_AdoptRevision asserts the empty-RunningRevision
// adoption selection: runtime-ready pods that already match the target
// select the backfill stamp as an Item (the write itself belongs to
// the executor — forbidMutations proves Plan never performs it).
func TestPlan_Update_AdoptRevision(t *testing.T) {
	target := updateTarget()
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady},
	}
	plan := minimalPlan()
	pod := enginePod("llama-70b", "prod", 0)
	pod.Spec = *in.DesiredSpec.PodSpec.DeepCopy()
	pod.Status.Conditions = []corev1.PodCondition{{
		Type: corev1.ContainersReady, Status: corev1.ConditionTrue,
	}}

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, map[int32][]*corev1.Pod{0: {pod}}))
	ua := findAction(d, workload.ActionUpdate)
	if ua == nil {
		t.Fatalf("expected an Update action carrying the adoption, got %v", actionKinds(d))
	}
	if len(ua.Update.Items) != 1 || !ua.Update.Items[0].AdoptRevision {
		t.Errorf("update items = %+v, want a single AdoptRevision item", ua.Update.Items)
	}

	// Same shape with a NOT-runtime-ready pod: no adoption (Ready is
	// only stamped on proof), no trigger.
	pod.Status.Conditions = nil
	d = planTargetOrFail(t, in, plan, target, planSnapshot(in, map[int32][]*corev1.Pod{0: {pod}}))
	if findAction(d, workload.ActionUpdate) != nil {
		t.Errorf("unproven pods must not be adopted, got %v", actionKinds(d))
	}
}

// TestPlan_Update_CleanupOnly_RollBackWreckage asserts the wreckage
// scan: an instance the revision-diff trigger declines (target
// == RunningRevision, the corrective roll-back) but that carries a
// live pod on a THIRD revision is selected CleanupOnly — never
// StartingFresh, so it is neither budget-charged nor gated. The same
// instance with only running-revision pods selects nothing.
func TestPlan_Update_CleanupOnly_RollBackWreckage(t *testing.T) {
	target := updateTarget()
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseFailed, RunningRevision: target.Name},
	}
	plan := minimalPlan()

	runningPod := enginePod("llama-70b", "prod", 0)
	runningPod.Labels[query.LabelRevisionHash] = query.RevisionFromName(target.Name).Hash()
	alienPod := enginePod("llama-70b", "prod", 0)
	alienPod.Name += "-alien"
	alienPod.Labels[query.LabelRevisionHash] = "deadrev1"

	d := planTargetOrFail(t, in, plan, target, planSnapshot(in, map[int32][]*corev1.Pod{0: {runningPod, alienPod}}))
	ua := findAction(d, workload.ActionUpdate)
	if ua == nil {
		t.Fatalf("expected an Update action carrying the cleanup, got %v", actionKinds(d))
	}
	if len(ua.Update.Items) != 1 || !ua.Update.Items[0].CleanupOnly ||
		ua.Update.Items[0].StartingFresh || ua.Update.Items[0].Instance.Index != 0 {
		t.Errorf("update items = %+v, want a single CleanupOnly (non-fresh) item for index 0", ua.Update.Items)
	}

	// No alien pod → nothing to clean, no Update action.
	d = planTargetOrFail(t, in, plan, target, planSnapshot(in, map[int32][]*corev1.Pod{0: {runningPod}}))
	if findAction(d, workload.ActionUpdate) != nil {
		t.Errorf("no wreckage must select no update items, got %v", actionKinds(d))
	}

	// Gang shape: a Failed source whose preserved gang-surge operation
	// targets a superseded revision is wreckage even with no alien pod
	// in its own bucket — the abandon continuation must dispatch.
	k := int32(1)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseFailed, RunningRevision: target.Name,
			Operation: &workload.InstanceOperation{
				Type: workload.InstanceOperationUpdate, Step: "Surge", SurgeIndex: &k,
				TargetRevision: "llama-70b-engine-deadrev1",
			}},
	}
	d = planTargetOrFail(t, in, plan, target, planSnapshot(in, map[int32][]*corev1.Pod{0: {runningPod}}))
	ua = findAction(d, workload.ActionUpdate)
	if ua == nil || len(ua.Update.Items) != 1 || !ua.Update.Items[0].CleanupOnly {
		t.Fatalf("expected a CleanupOnly item for the stranded gang continuation, got %v", actionKinds(d))
	}
}

// TestPlan_LiveSurgeUntouchedByCleanup pins the mid-flight safety of
// the wreckage-cleanup machinery: a LIVE (non-Failed) gang surge with its
// marker correctly pinned — source Op.SurgeIndex referencing the
// marker, surge pods on the pinned revision — is selected as a normal
// update continuation, never CleanupOnly, and the marker is neither
// scale-downed nor selected. Holds both while the surge target IS the
// roll target and after the target moved on mid-surge (the superseded
// redirect owns that, not the wreckage scan).
func TestPlan_LiveSurgeUntouchedByCleanup(t *testing.T) {
	for _, tc := range []struct {
		name       string
		liveTarget *appsv1.ControllerRevision
	}{
		{name: "surge toward the current target", liveTarget: updateTarget()},
		{name: "target moved on mid-surge", liveTarget: &appsv1.ControllerRevision{
			ObjectMeta: metav1.ObjectMeta{Name: "llama-70b-engine-newerrev", Namespace: "prod"},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pinned := updateTarget()
			in := minimalInput(t)
			forbidMutations(t, &in)
			k := int32(1)
			in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
				{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseUpdating,
					RunningRevision: "llama-70b-engine-priorrev",
					Operation: &workload.InstanceOperation{
						Type: workload.InstanceOperationUpdate, Step: "Surge",
						SurgeIndex: &k, TargetRevision: pinned.Name,
					}},
				{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseCreating,
					Operation: &workload.InstanceOperation{
						Type: workload.InstanceOperationUpdate, Step: workload.UpdateStepGangSurgeTarget,
						TargetRevision: pinned.Name,
					}},
			}
			plan, err := workload.BuildPlan(workload.ComponentEngine, in.DesiredSpec, in.ObservedState)
			if err != nil {
				t.Fatalf("BuildPlan: %v", err)
			}

			sourcePod := enginePod("llama-70b", "prod", 0)
			surgePod := enginePod("llama-70b", "prod", 1)
			surgePod.Labels[query.LabelRevisionHash] = query.RevisionFromName(pinned.Name).Hash()

			d := planTargetOrFail(t, in, plan, tc.liveTarget, planSnapshot(in, map[int32][]*corev1.Pod{0: {sourcePod}, 1: {surgePod}}))
			if sd := findAction(d, workload.ActionScaleDown); sd != nil {
				t.Fatalf("a live surge pair must not be scale-downed, got extras %v", sd.Extras)
			}
			ua := findAction(d, workload.ActionUpdate)
			if ua == nil {
				t.Fatalf("expected the in-flight surge continuation, got %v", actionKinds(d))
			}
			if len(ua.Update.Items) != 1 {
				t.Fatalf("update items = %+v, want exactly the source continuation", ua.Update.Items)
			}
			item := ua.Update.Items[0]
			if item.Instance.Index != 0 || item.CleanupOnly || item.StartingFresh || item.AdoptRevision {
				t.Errorf("live surge source must be a plain continuation, got %+v", item)
			}
		})
	}
}

// TestPlan_Paused_TruncatesAfterScaleDown asserts a paused plan keeps
// the scale-down selection (a deliberate replica reduction still
// releases capacity) and plans nothing else.
func TestPlan_Paused_TruncatesAfterScaleDown(t *testing.T) {
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Phase: workload.InstancePhaseReady},
		{Index: 1, Phase: workload.InstancePhaseReady}, // extra
	}
	plan := minimalPlan()
	plan.Paused = true
	// A live restart trigger (index 0's pod is gone) must NOT be
	// planned while paused.
	plan.RestartPolicy = workload.RestartPolicyRecreateInstance

	d := planOrFail(t, in, plan, planSnapshot(in, nil))
	if !kindsEqual(actionKinds(d), []workload.ActionKind{workload.ActionScaleDown}) {
		t.Errorf("paused decision = %v, want [ScaleDown] only", actionKinds(d))
	}
	if d.Escalate {
		t.Errorf("paused decision must suspend escalation")
	}

	plan.Paused = false
	d = planOrFail(t, in, plan, planSnapshot(in, nil))
	if !d.Escalate {
		t.Errorf("non-paused decision must enable escalation")
	}
}

// maximalPlanInput builds the all-passes-triggerable fixture: an extra
// index (scale-down), a pod-lost Instance (restart), an expired plus a
// drivable Manual migration record, prior-revision Instances (update),
// and a covering plan (create closes every non-paused decision). Every
// mutation callback is wired to the forbidMutations recorder.
func maximalPlanInput(t *testing.T, now time.Time) (workload.ReconcileInput, workload.ComponentPlan) {
	t.Helper()
	in := minimalInput(t)
	forbidMutations(t, &in)
	in.Clock = clocktesting.NewFakeClock(now)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		// Prior revision → update trigger; index 1 also lost its pod →
		// restart trigger.
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
		{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseReady, RunningRevision: "prior-rev"},
		// Extra → scale-down.
		{Index: 9, Incarnation: 1, Phase: workload.InstancePhaseReady},
	}
	in.ObservedState.Migrations = []workload.MigrationRecord{
		{RequestUUID: "u-expired", Trigger: workload.MigrationTriggerManual,
			Phase: workload.MigrationPhaseAccepted, SourceInstance: 0,
			Deadline: metav1.NewTime(now.Add(-time.Minute))},
		{RequestUUID: "u-drive", Trigger: workload.MigrationTriggerManual,
			Phase: workload.MigrationPhaseAccepted, SourceInstance: 0,
			StartedAt: metav1.NewTime(now.Add(-time.Hour))},
	}
	plan := workload.ComponentPlan{
		Component:     workload.ComponentEngine,
		Replicas:      2,
		RestartPolicy: workload.RestartPolicyRecreateInstance,
		MigrationMode: workload.MigrationModeAuto,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
	}
	return in, plan
}

// maximalPlanKinds is the complete precedence order the maximal fixture
// must produce — a guard that the fixture really triggers every pass,
// so the purity/determinism assertions on it are not vacuous.
var maximalPlanKinds = []workload.ActionKind{
	workload.ActionScaleDown,
	workload.ActionRestart,
	workload.ActionMigrateExpiry,
	workload.ActionMigrate,
	workload.ActionUpdate,
	workload.ActionCreate,
}

// TestPlan_Determinism_IdenticalInputs asserts Plan is a pure function
// of its inputs: two calls over identical inputs (fake clock, same
// snapshot content) produce deep-equal Decisions, with every pass
// triggered.
func TestPlan_Determinism_IdenticalInputs(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	target := updateTarget()
	in, plan := maximalPlanInput(t, now)
	pods := map[int32][]*corev1.Pod{0: {enginePod("llama-70b", "prod", 0)}}

	d1 := planTargetOrFail(t, in, plan, target, planSnapshot(in, pods))
	d2 := planTargetOrFail(t, in, plan, target, planSnapshot(in, pods))

	if !kindsEqual(actionKinds(d1), maximalPlanKinds) {
		t.Fatalf("maximal fixture decision = %v, want %v (fixture must trigger every pass)",
			actionKinds(d1), maximalPlanKinds)
	}
	if !reflect.DeepEqual(d1, d2) {
		t.Errorf("Plan is not deterministic over identical inputs:\n first = %+v\nsecond = %+v", d1, d2)
	}
}

// TestPlan_MaximalFixture_NoWrites proves the purity contract over the
// full decision surface at once: with every pass triggered, Plan invokes
// ZERO ReconcileInput mutation callbacks (forbidMutations, wired by
// maximalPlanInput) and issues ZERO client writes — the snapshot's pod
// reads route through a write-intercepting client that fails the test on
// any Create/Update/Patch/Delete, plain or subresource.
func TestPlan_MaximalFixture_NoWrites(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	target := updateTarget()
	in, plan := maximalPlanInput(t, now)

	podLists := 0
	forbidden := func(op string, obj client.Object) {
		t.Errorf("Plan must not issue client writes (%s %T %s/%s)", op, obj, obj.GetNamespace(), obj.GetName())
	}
	funcs := interceptor.Funcs{
		Create: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.CreateOption) error {
			forbidden("Create", obj)
			return nil
		},
		Update: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.UpdateOption) error {
			forbidden("Update", obj)
			return nil
		},
		Delete: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.DeleteOption) error {
			forbidden("Delete", obj)
			return nil
		},
		DeleteAllOf: func(_ context.Context, _ client.WithWatch, obj client.Object, _ ...client.DeleteAllOfOption) error {
			forbidden("DeleteAllOf", obj)
			return nil
		},
		Patch: func(_ context.Context, _ client.WithWatch, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
			forbidden("Patch", obj)
			return nil
		},
		SubResourceCreate: func(_ context.Context, _ client.Client, sub string, obj client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
			forbidden("SubResource("+sub+").Create", obj)
			return nil
		},
		SubResourceUpdate: func(_ context.Context, _ client.Client, sub string, obj client.Object, _ ...client.SubResourceUpdateOption) error {
			forbidden("SubResource("+sub+").Update", obj)
			return nil
		},
		SubResourcePatch: func(_ context.Context, _ client.Client, sub string, obj client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
			forbidden("SubResource("+sub+").Patch", obj)
			return nil
		},
		List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*corev1.PodList); ok {
				podLists++
			}
			return cl.List(ctx, list, opts...)
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(makeScheme(t)).
		WithObjects(enginePod("llama-70b", "prod", 0)).
		WithInterceptorFuncs(funcs).
		Build()
	deps := workload.Deps{Client: c, APIReader: c}
	snapshot := workload.NewObservedSnapshot(deps, in, plan.Component, in.ObservedState.InstanceStatuses)

	d, err := workload.Plan(context.Background(), in, plan, target, snapshot)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if !kindsEqual(actionKinds(d), maximalPlanKinds) {
		t.Fatalf("maximal fixture decision = %v, want %v (fixture must trigger every pass)",
			actionKinds(d), maximalPlanKinds)
	}
	// Sanity: the snapshot's live (restart) and cached (update) reads
	// both routed through the intercepted client — the zero-writes
	// assertion above actually covered the client surface Plan touches.
	if podLists < 2 {
		t.Errorf("pod List calls = %d, want >= 2 (live + cached read sources must go through the intercepted client)", podLists)
	}
}
