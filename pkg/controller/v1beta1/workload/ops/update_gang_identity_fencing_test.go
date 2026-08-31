package ops

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestGangSurge_StaleSnapshotDoesNotClaimOccupiedTarget(t *testing.T) {
	legacyResetExpectations(t)
	const targetRevision = "stale-claim-engine-newrev"
	source := workload.InstanceStatus{
		Index:           0,
		Incarnation:     3,
		Phase:           workload.InstancePhaseReady,
		RunningRevision: "stale-claim-engine-oldrev",
	}
	occupied := workload.InstanceStatus{
		Index:           1,
		Incarnation:     8,
		Phase:           workload.InstancePhaseReady,
		RunningRevision: "unrelated-revision",
	}
	sourceIdentity := captureTerminalInstanceIdentity(&source)
	occupiedIdentity := captureTerminalInstanceIdentity(&occupied)
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{
			source.Index:   cloneTerminalStatus(source),
			occupied.Index: cloneTerminalStatus(occupied),
		},
		retryBlock: map[string]workload.RetryBlock{
			targetRevision: {TargetRevision: targetRevision, State: workload.RetryBlockBackoff},
		},
	}
	input := gangSurgeRecoveryInput("owner-a", "stale-claim", "test-ns", store, cloneTerminalStatus(source))
	plan := legacyMultiPodComponentPlan(workload.UpdateStrategySurgeThenDrain)
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: targetRevision}}

	done, err := gangSurgeUpdate(context.Background(), legacyTestDeps(legacyNewFakeClient(t)), input, plan, plan.Instances[0], target)
	if err != nil || done {
		t.Fatalf("stale claim: done=%v err=%v", done, err)
	}
	if store.writes != 0 {
		t.Fatalf("stale claim status writes=%d want 0", store.writes)
	}
	if current := store.statuses[source.Index]; !sourceIdentity.matches(current) {
		t.Fatalf("source changed without owning the target: %+v", current)
	}
	if current := store.statuses[occupied.Index]; !occupiedIdentity.matches(current) {
		t.Fatalf("occupied target changed: %+v", current)
	}
	if block := store.retryBlock[targetRevision]; block.State != workload.RetryBlockBackoff {
		t.Fatalf("retry block state=%q want %q", block.State, workload.RetryBlockBackoff)
	}
}

func TestStartGangSurge_ClaimsPairInOneWrite(t *testing.T) {
	const targetRevision = "atomic-claim-engine-newrev"
	source := workload.InstanceStatus{
		Index:           0,
		Incarnation:     3,
		Phase:           workload.InstancePhaseReady,
		RunningRevision: "atomic-claim-engine-oldrev",
	}
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{source.Index: cloneTerminalStatus(source)},
		retryBlock: map[string]workload.RetryBlock{
			targetRevision: {TargetRevision: targetRevision, State: workload.RetryBlockBackoff},
		},
	}
	input := gangSurgeRecoveryInput("owner-a", "atomic-claim", "test-ns", store, cloneTerminalStatus(source))

	claimed, err := startGangSurge(context.Background(), input, &source, 1, targetRevision, time.Minute)
	if err != nil || !claimed {
		t.Fatalf("atomic claim: claimed=%v err=%v", claimed, err)
	}
	if store.writes != 1 {
		t.Fatalf("atomic claim status writes=%d want 1", store.writes)
	}
	persistedSource := store.statuses[source.Index]
	if persistedSource.Operation == nil || persistedSource.Operation.Step != updateStepSurge ||
		persistedSource.Operation.SurgeIndex == nil || *persistedSource.Operation.SurgeIndex != 1 {
		t.Fatalf("source claim: %+v", persistedSource)
	}
	persistedTarget := store.statuses[1]
	if !gangSurgeTargetMatches(&persistedTarget, targetRevision) {
		t.Fatalf("target claim: %+v", persistedTarget)
	}
	if block := store.retryBlock[targetRevision]; block.State != workload.RetryBlockRetryInProgress {
		t.Fatalf("retry block state=%q want %q", block.State, workload.RetryBlockRetryInProgress)
	}
}

func TestStartGangSurge_RejectsAuthoritativeDrift(t *testing.T) {
	const targetRevision = "guarded-claim-engine-newrev"
	source := workload.InstanceStatus{
		Index:           0,
		Incarnation:     3,
		Phase:           workload.InstancePhaseReady,
		RunningRevision: "guarded-claim-engine-oldrev",
	}
	tests := []struct {
		name     string
		ownerUID types.UID
		prepare  func(map[int32]workload.InstanceStatus)
	}{
		{name: "owner replaced", ownerUID: "owner-b"},
		{
			name:     "source lifecycle changed",
			ownerUID: "owner-a",
			prepare: func(statuses map[int32]workload.InstanceStatus) {
				changed := statuses[source.Index]
				changed.Incarnation++
				statuses[source.Index] = changed
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statuses := map[int32]workload.InstanceStatus{source.Index: cloneTerminalStatus(source)}
			if test.prepare != nil {
				test.prepare(statuses)
			}
			authoritativeSource := statuses[source.Index]
			before := captureTerminalInstanceIdentity(&authoritativeSource)
			store := &terminalMutationStore{ownerUID: test.ownerUID, statuses: statuses}
			input := gangSurgeRecoveryInput("owner-a", "guarded-claim", "test-ns", store, cloneTerminalStatus(source))

			claimed, err := startGangSurge(context.Background(), input, &source, 1, targetRevision, time.Minute)
			if err != nil || claimed || store.writes != 0 {
				t.Fatalf("guarded claim: claimed=%v writes=%d err=%v", claimed, store.writes, err)
			}
			if current := store.statuses[source.Index]; !before.matches(current) {
				t.Fatalf("authoritative source changed: %+v", current)
			}
			if _, found := store.statuses[1]; found {
				t.Fatal("target was created after the claim guard rejected")
			}
		})
	}
}

func TestStartGangSurge_WriteFailureLeavesPairUnchanged(t *testing.T) {
	const targetRevision = "failed-claim-engine-newrev"
	source := workload.InstanceStatus{
		Index:           0,
		Incarnation:     3,
		Phase:           workload.InstancePhaseReady,
		RunningRevision: "failed-claim-engine-oldrev",
	}
	writeErr := errors.New("status write failed")
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{source.Index: cloneTerminalStatus(source)},
		applyErr: writeErr,
	}
	input := gangSurgeRecoveryInput("owner-a", "failed-claim", "test-ns", store, cloneTerminalStatus(source))

	claimed, err := startGangSurge(context.Background(), input, &source, 1, targetRevision, time.Minute)
	if !errors.Is(err, writeErr) || claimed || store.writes != 0 {
		t.Fatalf("failed claim: claimed=%v writes=%d err=%v", claimed, store.writes, err)
	}
	if current := store.statuses[source.Index]; !captureTerminalInstanceIdentity(&source).matches(current) {
		t.Fatalf("source changed after failed write: %+v", current)
	}
	if _, found := store.statuses[1]; found {
		t.Fatal("target was created after failed write")
	}
}

func TestPatchInstanceStatusGangSurgeTarget_DoesNotOverwriteOccupiedSlot(t *testing.T) {
	occupied := workload.InstanceStatus{
		Index:           2,
		Incarnation:     8,
		Phase:           workload.InstancePhaseReady,
		RunningRevision: "unrelated-revision",
	}
	input := workload.ReconcileInput{
		MutateInstance: func(_ context.Context, _ int32, mutate func(*workload.InstanceStatus) bool) error {
			if mutate(&occupied) {
				t.Fatal("occupied target slot was mutated")
			}
			return nil
		},
	}

	if err := patchInstanceStatusGangSurgeTarget(context.Background(), input, occupied.Index, "new-revision", time.Minute); err != nil {
		t.Fatal(err)
	}
	if occupied.Phase != workload.InstancePhaseReady || occupied.RunningRevision != "unrelated-revision" || occupied.Operation != nil {
		t.Fatalf("occupied target slot changed: %+v", occupied)
	}
}

func TestClaimGangSurgeDrain_RejectsAuthoritativePairDrift(t *testing.T) {
	const targetRevision = "pair-guard-engine-newrev"
	surgeIndex := int32(2)
	source := gangSurgeRecoverySource(surgeIndex, targetRevision)
	target := gangSurgeActiveTarget(surgeIndex, targetRevision)
	tests := []struct {
		name   string
		mutate func(map[int32]workload.InstanceStatus)
	}{
		{
			name: "source lifecycle",
			mutate: func(statuses map[int32]workload.InstanceStatus) {
				changed := statuses[source.Index]
				changed.Operation.ID = "replacement-source"
				statuses[source.Index] = changed
			},
		},
		{
			name: "target lifecycle",
			mutate: func(statuses map[int32]workload.InstanceStatus) {
				changed := statuses[target.Index]
				changed.Operation.ID = "replacement-target"
				statuses[target.Index] = changed
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statuses := map[int32]workload.InstanceStatus{
				source.Index: cloneTerminalStatus(source),
				target.Index: cloneTerminalStatus(target),
			}
			test.mutate(statuses)
			store := &terminalMutationStore{ownerUID: "owner-a", statuses: statuses}
			input := gangSurgeRecoveryInput("owner-a", "pair-guard", "test-ns", store,
				cloneTerminalStatus(source), cloneTerminalStatus(target))

			claimed, err := claimGangSurgeDrain(context.Background(), input, &source, &target)
			if err != nil || claimed || store.writes != 0 {
				t.Fatalf("claim with drift: claimed=%v writes=%d err=%v", claimed, store.writes, err)
			}
		})
	}
}

func TestPromoteGangSurgeTarget_RejectsAuthoritativePairDrift(t *testing.T) {
	const targetRevision = "promotion-guard-engine-newrev"
	surgeIndex := int32(2)
	source := gangSurgeRecoverySource(surgeIndex, targetRevision)
	source.Operation.Step = updateStepSurgeDrain
	target := gangSurgeActiveTarget(surgeIndex, targetRevision)
	tests := []struct {
		name   string
		mutate func(map[int32]workload.InstanceStatus)
	}{
		{
			name: "source lifecycle",
			mutate: func(statuses map[int32]workload.InstanceStatus) {
				changed := statuses[source.Index]
				changed.Operation.ID = "replacement-source"
				statuses[source.Index] = changed
			},
		},
		{
			name: "target lifecycle",
			mutate: func(statuses map[int32]workload.InstanceStatus) {
				changed := statuses[target.Index]
				changed.Operation.ID = "replacement-target"
				statuses[target.Index] = changed
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statuses := map[int32]workload.InstanceStatus{
				source.Index: cloneTerminalStatus(source),
				target.Index: cloneTerminalStatus(target),
			}
			test.mutate(statuses)
			store := &terminalMutationStore{ownerUID: "owner-a", statuses: statuses}
			input := gangSurgeRecoveryInput("owner-a", "promotion-guard", "test-ns", store,
				cloneTerminalStatus(source), cloneTerminalStatus(target))

			promoted, err := promoteGangSurgeTarget(context.Background(), input, &source, &target, targetRevision)
			if err != nil || promoted || store.writes != 0 {
				t.Fatalf("promotion with drift: promoted=%v writes=%d err=%v", promoted, store.writes, err)
			}
			if current := store.statuses[target.Index]; current.Operation == nil || current.Operation.ID != "replacement-target" && test.name == "target lifecycle" {
				t.Fatalf("promotion changed authoritative target: %+v", current)
			}
		})
	}
}

func TestGangSurge_TargetConflictRollbackGuardsAuthoritativePair(t *testing.T) {
	surgeIndex := int32(2)
	const targetRevision = "gang-pair-guard-engine-newrev"
	source := gangSurgeRecoverySource(surgeIndex, targetRevision)
	occupied := workload.InstanceStatus{
		Index:           surgeIndex,
		Incarnation:     8,
		Phase:           workload.InstancePhaseReady,
		RunningRevision: "unrelated-revision",
	}
	tests := []struct {
		name       string
		ownerUID   types.UID
		storeUID   types.UID
		mutateLive func(map[int32]workload.InstanceStatus)
	}{
		{
			name:     "owner changed",
			ownerUID: "owner-a",
			storeUID: "owner-b",
		},
		{
			name:     "source lifecycle changed",
			ownerUID: "owner-a",
			storeUID: "owner-a",
			mutateLive: func(statuses map[int32]workload.InstanceStatus) {
				changed := statuses[source.Index]
				changed.Operation.Step = updateStepSurgeDrain
				statuses[source.Index] = changed
			},
		},
		{
			name:     "target lifecycle changed",
			ownerUID: "owner-a",
			storeUID: "owner-a",
			mutateLive: func(statuses map[int32]workload.InstanceStatus) {
				changed := statuses[occupied.Index]
				changed.Incarnation++
				statuses[occupied.Index] = changed
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statuses := map[int32]workload.InstanceStatus{
				source.Index:   cloneTerminalStatus(source),
				occupied.Index: cloneTerminalStatus(occupied),
			}
			if test.mutateLive != nil {
				test.mutateLive(statuses)
			}
			store := &terminalMutationStore{ownerUID: test.storeUID, statuses: statuses}
			input := gangSurgeRecoveryInput(test.ownerUID, "gang-pair-guard", "test-ns", store,
				cloneTerminalStatus(source), cloneTerminalStatus(occupied))

			reset, err := resetGangSurgeSourceAfterTargetConflict(context.Background(), input, &source, &occupied)
			if err != nil || reset {
				t.Fatalf("guarded rollback: reset=%v err=%v", reset, err)
			}
			if store.writes != 0 {
				t.Fatalf("guarded rollback status writes=%d want 0", store.writes)
			}
		})
	}
}
