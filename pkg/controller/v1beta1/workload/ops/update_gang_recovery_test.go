package ops

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestGangSurge_RestoresMissingTargetMarkerBeforeEffects(t *testing.T) {
	legacyResetExpectations(t)
	const isvcName, namespace = "gang-resume", "test-ns"
	surgeIndex := int32(2)
	revision := "gang-resume-engine-newrev"
	source := workload.InstanceStatus{
		Index:           0,
		Incarnation:     3,
		Phase:           workload.InstancePhaseUpdating,
		RunningRevision: "gang-resume-engine-oldrev",
		TargetRevision:  revision,
		Operation: &workload.InstanceOperation{
			ID:             "gang-update-0",
			Type:           workload.InstanceOperationUpdate,
			Step:           updateStepSurge,
			TargetRevision: revision,
			SurgeIndex:     &surgeIndex,
		},
	}
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{source.Index: cloneTerminalStatus(source)},
	}
	input := workload.ReconcileInput{
		OwnerObject: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}},
		Key: workload.Key{
			Namespace: namespace,
			OwnerName: isvcName,
			Component: workload.ComponentEngine,
			SelectorLabels: map[string]string{
				constants.InferenceServicePodLabelKey: isvcName,
				constants.OMEComponentLabel:           string(workload.ComponentEngine),
				query.LabelManagedBy:                  query.ManagedByOMENative,
			},
		},
		ObservedState: workload.WorkloadObservedState{
			InstanceStatuses: []workload.InstanceStatus{cloneTerminalStatus(source)},
		},
		MutateInstance: func(context.Context, int32, func(*workload.InstanceStatus) bool) error {
			t.Fatal("native recovery bypassed the atomic status adapter")
			return nil
		},
		FinalizeInstanceResources:            func(context.Context, int32) (bool, error) { return true, nil },
		ApplyInstanceMutationsWithRetryBlock: store.apply,
	}
	c := legacyNewFakeClient(t)
	ensureCalls := 0
	deps := legacyTestDeps(c)
	deps.EnsureGangPodGroup = func(context.Context, workload.ReconcileInput, workload.ComponentPlan, workload.InstancePlan) (string, error) {
		ensureCalls++
		return "", nil
	}
	plan := legacyMultiPodComponentPlan(workload.UpdateStrategySurgeThenDrain)
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: revision}}

	done, err := gangSurgeUpdate(context.Background(), deps, input, plan, plan.Instances[0], target)
	if err != nil || done {
		t.Fatalf("resume pass: done=%v err=%v", done, err)
	}
	marker, found := store.statuses[surgeIndex]
	if !found || !gangSurgeTargetMatches(&marker, revision) {
		t.Fatalf("restored target marker: %+v", marker)
	}
	if store.writes != 1 {
		t.Fatalf("target marker status writes=%d want 1", store.writes)
	}
	if ensureCalls != 0 {
		t.Fatalf("PodGroup ensure calls=%d want 0 before marker round-trip", ensureCalls)
	}
	pods, err := query.LiveListPodsForInstance(context.Background(), c, namespace, isvcName, workload.ComponentEngine, surgeIndex)
	if err != nil {
		t.Fatal(err)
	}
	if len(pods) != 0 {
		t.Fatalf("surge effects ran before marker round-trip: %d pods", len(pods))
	}
}

func TestGangSurge_CachedTargetMissingAuthoritativelyRestoresBeforeEffects(t *testing.T) {
	legacyResetExpectations(t)
	const isvcName, namespace = "gang-stale-target", "test-ns"
	surgeIndex := int32(2)
	revision := "gang-stale-target-engine-newrev"
	source := workload.InstanceStatus{
		Index:           0,
		Incarnation:     3,
		Phase:           workload.InstancePhaseUpdating,
		RunningRevision: "gang-stale-target-engine-oldrev",
		TargetRevision:  revision,
		Operation: &workload.InstanceOperation{
			ID:             "gang-update-0",
			Type:           workload.InstanceOperationUpdate,
			Step:           updateStepSurge,
			TargetRevision: revision,
			SurgeIndex:     &surgeIndex,
		},
	}
	marker := workload.InstanceStatus{
		Index:          surgeIndex,
		Incarnation:    1,
		Phase:          workload.InstancePhaseCreating,
		TargetRevision: revision,
		Operation: &workload.InstanceOperation{
			ID:             "gang-update-target-2",
			Type:           workload.InstanceOperationUpdate,
			Step:           workload.UpdateStepGangSurgeTarget,
			TargetRevision: revision,
		},
	}
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{source.Index: cloneTerminalStatus(source)},
	}
	finalizes := 0
	input := workload.ReconcileInput{
		OwnerObject: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}},
		Key: workload.Key{
			Namespace: namespace,
			OwnerName: isvcName,
			Component: workload.ComponentEngine,
			SelectorLabels: map[string]string{
				constants.InferenceServicePodLabelKey: isvcName,
				constants.OMEComponentLabel:           string(workload.ComponentEngine),
				query.LabelManagedBy:                  query.ManagedByOMENative,
			},
		},
		ObservedState: workload.WorkloadObservedState{
			InstanceStatuses: []workload.InstanceStatus{cloneTerminalStatus(source), cloneTerminalStatus(marker)},
		},
		MutateInstance: func(context.Context, int32, func(*workload.InstanceStatus) bool) error {
			t.Fatal("target confirmation bypassed the atomic status adapter")
			return nil
		},
		FinalizeInstanceResources: func(context.Context, int32) (bool, error) {
			finalizes++
			return true, nil
		},
		ApplyInstanceMutationsWithRetryBlock: store.apply,
	}
	c := legacyNewFakeClient(t)
	ensureCalls := 0
	deps := legacyTestDeps(c)
	deps.EnsureGangPodGroup = func(context.Context, workload.ReconcileInput, workload.ComponentPlan, workload.InstancePlan) (string, error) {
		ensureCalls++
		return "", nil
	}
	plan := legacyMultiPodComponentPlan(workload.UpdateStrategySurgeThenDrain)
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: revision}}

	done, err := gangSurgeUpdate(context.Background(), deps, input, plan, plan.Instances[0], target)
	if err != nil || done {
		t.Fatalf("confirmation pass: done=%v err=%v", done, err)
	}
	persisted, found := store.statuses[surgeIndex]
	if !found || !gangSurgeTargetMatches(&persisted, revision) {
		t.Fatalf("authoritative target marker was not restored: %+v", persisted)
	}
	if store.writes != 1 || finalizes != 0 || ensureCalls != 0 {
		t.Fatalf("writes=%d finalizes=%d PodGroup ensures=%d want 1,0,0", store.writes, finalizes, ensureCalls)
	}
	pods, err := query.LiveListPodsForInstance(context.Background(), c, namespace, isvcName, workload.ComponentEngine, surgeIndex)
	if err != nil {
		t.Fatal(err)
	}
	if len(pods) != 0 {
		t.Fatalf("surge effects ran before restored marker round-trip: %d pods", len(pods))
	}
}

func TestGangSurge_RestartAfterTargetStampFailureRestoresCleanupBeforeRedirects(t *testing.T) {
	tests := []struct {
		name            string
		prepareSource   func(*workload.InstanceStatus)
		latestRevision  string
		wantSourcePhase workload.InstancePhase
	}{
		{
			name: "failed source",
			prepareSource: func(status *workload.InstanceStatus) {
				status.Phase = workload.InstancePhaseFailed
			},
			latestRevision:  "gang-restart-engine-badrev",
			wantSourcePhase: workload.InstancePhaseFailed,
		},
		{
			name:            "superseded target",
			prepareSource:   func(*workload.InstanceStatus) {},
			latestRevision:  "gang-restart-engine-newerrev",
			wantSourcePhase: workload.InstancePhaseUpdating,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			legacyResetExpectations(t)
			const isvcName, namespace = "gang-restart", "test-ns"
			const committedRevision = "gang-restart-engine-badrev"
			source := workload.InstanceStatus{
				Index:           0,
				Incarnation:     3,
				Phase:           workload.InstancePhaseReady,
				RunningRevision: "gang-restart-engine-oldrev",
			}
			store := &terminalMutationStore{
				ownerUID: "owner-a",
				statuses: map[int32]workload.InstanceStatus{source.Index: cloneTerminalStatus(source)},
			}
			finalizes := 0
			failTargetStamp := true
			targetStampErr := errors.New("injected target marker failure")
			input := workload.ReconcileInput{
				OwnerObject: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{UID: "owner-a"}},
				Key: workload.Key{
					Namespace: namespace,
					OwnerName: isvcName,
					Component: workload.ComponentEngine,
					SelectorLabels: map[string]string{
						constants.InferenceServicePodLabelKey: isvcName,
						constants.OMEComponentLabel:           string(workload.ComponentEngine),
						query.LabelManagedBy:                  query.ManagedByOMENative,
					},
				},
				ObservedState: workload.WorkloadObservedState{
					InstanceStatuses: []workload.InstanceStatus{cloneTerminalStatus(source)},
				},
				MutateInstance: func(_ context.Context, index int32, mutate func(*workload.InstanceStatus) bool) error {
					if index != source.Index && failTargetStamp {
						failTargetStamp = false
						return targetStampErr
					}
					status, found := store.statuses[index]
					if !found {
						status = workload.InstanceStatus{Index: index}
					}
					if mutate(&status) {
						store.statuses[index] = status
					}
					return nil
				},
				FinalizeInstanceResources: func(context.Context, int32) (bool, error) {
					finalizes++
					return true, nil
				},
			}
			plan := legacyMultiPodComponentPlan(workload.UpdateStrategySurgeThenDrain)
			c := legacyNewFakeClient(t)
			deps := legacyTestDeps(c)
			committedTarget := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: committedRevision}}

			done, err := gangSurgeUpdate(context.Background(), deps, input, plan, plan.Instances[0], committedTarget)
			if !errors.Is(err, targetStampErr) || done {
				t.Fatalf("source-stamp pass: done=%v err=%v", done, err)
			}
			persistedSource := store.statuses[source.Index]
			if persistedSource.Operation == nil || persistedSource.Operation.Step != updateStepSurge || persistedSource.Operation.SurgeIndex == nil {
				t.Fatalf("source surge claim was not persisted: %+v", persistedSource)
			}
			surgeIndex := *persistedSource.Operation.SurgeIndex
			if _, found := store.statuses[surgeIndex]; found {
				t.Fatal("target marker unexpectedly persisted during the injected failure")
			}

			test.prepareSource(&persistedSource)
			store.statuses[source.Index] = persistedSource
			input.ObservedState.InstanceStatuses = []workload.InstanceStatus{cloneTerminalStatus(persistedSource)}
			input.ApplyInstanceMutationsWithRetryBlock = store.apply
			latestTarget := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: test.latestRevision}}
			done, err = gangSurgeUpdate(context.Background(), deps, input, plan, plan.Instances[0], latestTarget)
			if err != nil || done {
				t.Fatalf("restart pass: done=%v err=%v", done, err)
			}
			marker, found := store.statuses[surgeIndex]
			if !found || !gangSurgeCleanupTargetClaimMatches(&marker, committedRevision) {
				t.Fatalf("restart exposed a non-terminal target marker: %+v", marker)
			}
			persistedSource = store.statuses[source.Index]
			if persistedSource.Phase != test.wantSourcePhase || persistedSource.Operation == nil || persistedSource.Operation.Step != updateStepSurge {
				t.Fatalf("redirect ran before marker restoration: %+v", persistedSource)
			}
			if finalizes != 0 {
				t.Fatalf("terminal finalization ran before marker restoration: %d calls", finalizes)
			}
			if store.writes != 1 {
				t.Fatalf("atomic marker status writes=%d want 1", store.writes)
			}
		})
	}
}

func gangSurgeRecoverySource(surgeIndex int32, targetRevision string) workload.InstanceStatus {
	return workload.InstanceStatus{
		Index:           0,
		Incarnation:     4,
		Phase:           workload.InstancePhaseUpdating,
		RunningRevision: "recovery-engine-oldrev",
		TargetRevision:  targetRevision,
		Operation: &workload.InstanceOperation{
			ID:             "gang-update-0",
			Type:           workload.InstanceOperationUpdate,
			Step:           updateStepSurge,
			TargetRevision: targetRevision,
			SurgeIndex:     &surgeIndex,
		},
	}
}

func gangSurgePromotedTarget(index int32, targetRevision string) workload.InstanceStatus {
	return workload.InstanceStatus{
		Index:           index,
		Incarnation:     1,
		Phase:           workload.InstancePhaseReady,
		RunningRevision: targetRevision,
	}
}

func gangSurgeActiveTarget(index int32, targetRevision string) workload.InstanceStatus {
	return workload.InstanceStatus{
		Index:          index,
		Incarnation:    1,
		Phase:          workload.InstancePhaseCreating,
		TargetRevision: targetRevision,
		Operation: &workload.InstanceOperation{
			ID:             "gang-target",
			Type:           workload.InstanceOperationUpdate,
			Step:           workload.UpdateStepGangSurgeTarget,
			TargetRevision: targetRevision,
		},
	}
}

func gangSurgeRecoveryInput(
	ownerUID types.UID,
	isvcName string,
	namespace string,
	store *terminalMutationStore,
	observed ...workload.InstanceStatus,
) workload.ReconcileInput {
	return workload.ReconcileInput{
		OwnerObject: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name: isvcName, Namespace: namespace, UID: ownerUID,
		}},
		OwnerGVK: corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		Key: workload.Key{
			Namespace: namespace,
			OwnerName: isvcName,
			Component: workload.ComponentEngine,
			SelectorLabels: map[string]string{
				constants.InferenceServicePodLabelKey: isvcName,
				constants.OMEComponentLabel:           string(workload.ComponentEngine),
				query.LabelManagedBy:                  query.ManagedByOMENative,
			},
		},
		DesiredSpec: workload.WorkloadDesiredSpec{
			PodSpec:       &corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "example/leader:latest"}}},
			WorkerPodSpec: &corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "example/worker:latest"}}},
		},
		ObservedState: workload.WorkloadObservedState{InstanceStatuses: observed},
		MutateInstance: func(_ context.Context, index int32, mutate func(*workload.InstanceStatus) bool) error {
			status, found := store.statuses[index]
			if !found {
				status = workload.InstanceStatus{Index: index}
			}
			if mutate(&status) {
				store.statuses[index] = cloneTerminalStatus(status)
			}
			return nil
		},
		ApplyInstanceMutationsWithRetryBlock: store.apply,
	}
}

func gangSurgeReadyRecoveryPod(isvcName, namespace string, index int32, runner, revisionHash string) *corev1.Pod {
	pod := gangSurgePod(isvcName, namespace, index, runner, revisionHash)
	now := metav1.Now()
	pod.Status.Conditions = []corev1.PodCondition{
		{Type: corev1.ContainersReady, Status: corev1.ConditionTrue, LastTransitionTime: now},
		{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: now},
		{Type: query.ServingConditionType, Status: corev1.ConditionTrue, LastTransitionTime: now},
	}
	return pod
}

func TestGangSurge_ResumesPromotedTargetWithPodlessSource(t *testing.T) {
	legacyResetExpectations(t)
	const isvcName, namespace = "gang-promoted-resume", "test-ns"
	const targetRevision = "gang-promoted-resume-engine-newrev"
	const targetHash = "newrev"
	surgeIndex := int32(2)
	source := gangSurgeRecoverySource(surgeIndex, targetRevision)
	promoted := gangSurgePromotedTarget(surgeIndex, targetRevision)
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{
			source.Index:   cloneTerminalStatus(source),
			promoted.Index: cloneTerminalStatus(promoted),
		},
	}
	input := gangSurgeRecoveryInput("owner-a", isvcName, namespace, store,
		cloneTerminalStatus(source), cloneTerminalStatus(promoted))
	finalized := 0
	input.FinalizeInstanceResources = func(_ context.Context, index int32) (bool, error) {
		if index != source.Index {
			t.Fatalf("finalized index=%d want source %d", index, source.Index)
		}
		finalized++
		return true, nil
	}
	c := legacyNewFakeClient(t,
		gangSurgeReadyRecoveryPod(isvcName, namespace, surgeIndex, "leader", targetHash),
		gangSurgeReadyRecoveryPod(isvcName, namespace, surgeIndex, "worker", targetHash),
	)
	plan := legacyMultiPodComponentPlan(workload.UpdateStrategySurgeThenDrain)
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: targetRevision}}

	done, err := gangSurgeUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], target)
	if err != nil || !done {
		t.Fatalf("promoted-target resume: done=%v err=%v", done, err)
	}
	if _, found := store.statuses[source.Index]; found {
		t.Fatal("promoted-target resume retained source status")
	}
	persistedTarget, found := store.statuses[surgeIndex]
	if !found || !gangSurgePromotedTargetMatches(&persistedTarget, targetRevision) {
		t.Fatalf("promoted target changed: %+v", persistedTarget)
	}
	if finalized != 1 {
		t.Fatalf("source finalization calls=%d want 1", finalized)
	}
}

func TestGangSurge_PromotedTargetWaitsForResourceAbsence(t *testing.T) {
	legacyResetExpectations(t)
	const isvcName, namespace = "gang-promoted-finalize", "test-ns"
	const targetRevision = "gang-promoted-finalize-engine-newrev"
	surgeIndex := int32(2)
	source := gangSurgeRecoverySource(surgeIndex, targetRevision)
	source.Operation.Step = updateStepSurgeDrain
	promoted := gangSurgePromotedTarget(surgeIndex, targetRevision)
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{
			source.Index:   cloneTerminalStatus(source),
			promoted.Index: cloneTerminalStatus(promoted),
		},
	}
	input := gangSurgeRecoveryInput("owner-a", isvcName, namespace, store,
		cloneTerminalStatus(source), cloneTerminalStatus(promoted))
	resourcesAbsent := false
	finalizations := 0
	input.FinalizeInstanceResources = func(_ context.Context, index int32) (bool, error) {
		if index != source.Index {
			t.Fatalf("finalized index=%d want source %d", index, source.Index)
		}
		finalizations++
		return resourcesAbsent, nil
	}
	c := legacyNewFakeClient(t,
		gangSurgeReadyRecoveryPod(isvcName, namespace, surgeIndex, "leader", "newrev"),
		gangSurgeReadyRecoveryPod(isvcName, namespace, surgeIndex, "worker", "newrev"),
	)
	plan := legacyMultiPodComponentPlan(workload.UpdateStrategySurgeThenDrain)
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: targetRevision}}

	done, err := gangSurgeUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], target)
	if err != nil || done {
		t.Fatalf("delete accepted pass: done=%v err=%v", done, err)
	}
	if _, found := store.statuses[source.Index]; !found {
		t.Fatal("source marker was removed before resource absence")
	}

	resourcesAbsent = true
	input.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		cloneTerminalStatus(store.statuses[source.Index]),
		cloneTerminalStatus(store.statuses[surgeIndex]),
	}
	done, err = gangSurgeUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], target)
	if err != nil || !done {
		t.Fatalf("absence pass: done=%v err=%v", done, err)
	}
	if _, found := store.statuses[source.Index]; found {
		t.Fatal("source marker remained after resource absence")
	}
	if finalizations != 2 {
		t.Fatalf("finalizations=%d want 2", finalizations)
	}
}

func TestGangSurge_PromotedPinnedTargetRecreatesMissingPodsBeforeNewerRevision(t *testing.T) {
	legacyResetExpectations(t)
	const isvcName, namespace = "gang-promoted-pinned", "test-ns"
	const committedRevision = "gang-promoted-pinned-engine-rev-v1hash"
	const latestRevision = "gang-promoted-pinned-engine-rev-v2hash"
	surgeIndex := int32(2)
	source := gangSurgeRecoverySource(surgeIndex, committedRevision)
	promoted := gangSurgePromotedTarget(surgeIndex, committedRevision)
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{
			source.Index:   cloneTerminalStatus(source),
			promoted.Index: cloneTerminalStatus(promoted),
		},
	}
	input := gangSurgeRecoveryInput("owner-a", isvcName, namespace, store,
		cloneTerminalStatus(source), cloneTerminalStatus(promoted))
	input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
		t.Fatal("incomplete promoted target reached resource finalization")
		return false, nil
	}
	c := legacyNewFakeClient(t,
		gangSurgeReadyRecoveryPod(isvcName, namespace, surgeIndex, "leader", "v1hash"),
	)
	ensureCalls := 0
	deps := legacyTestDeps(c)
	deps.EnsureGangPodGroup = func(context.Context, workload.ReconcileInput, workload.ComponentPlan, workload.InstancePlan) (string, error) {
		ensureCalls++
		return "", nil
	}
	plan := legacyMultiPodComponentPlan(workload.UpdateStrategySurgeThenDrain)
	latest := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: latestRevision}}

	done, err := gangSurgeUpdate(context.Background(), deps, input, plan, plan.Instances[0], latest)
	if err != nil || done {
		t.Fatalf("pinned promoted-target recovery: done=%v err=%v", done, err)
	}
	pods, err := query.LiveListPodsForInstance(context.Background(), c, namespace, isvcName, workload.ComponentEngine, surgeIndex)
	if err != nil || len(pods) != 2 {
		t.Fatalf("pinned replacement pods: count=%d err=%v", len(pods), err)
	}
	worker, found := query.IndexPodsByName(pods)[query.PodName(isvcName, workload.ComponentEngine, surgeIndex, "worker", 0)]
	if !found || worker.Labels[query.LabelRevisionHash] != "v1hash" {
		t.Fatalf("missing worker was not recreated on pinned revision: %+v", worker)
	}
	if ensureCalls != 1 {
		t.Fatalf("PodGroup ensure calls=%d want 1", ensureCalls)
	}
	persistedSource := store.statuses[source.Index]
	if persistedSource.Operation == nil || persistedSource.Operation.TargetRevision != committedRevision {
		t.Fatalf("source abandoned pinned completion: %+v", persistedSource)
	}
	persistedTarget := store.statuses[surgeIndex]
	if !gangSurgePromotedTargetMatches(&persistedTarget, committedRevision) {
		t.Fatalf("promoted target changed: %+v", persistedTarget)
	}
}

func TestGangSurge_PromotedTargetWithLiveSourceRollsBackWithoutEffects(t *testing.T) {
	legacyResetExpectations(t)
	const isvcName, namespace = "gang-promoted-conflict", "test-ns"
	const targetRevision = "gang-promoted-conflict-engine-newrev"
	surgeIndex := int32(2)
	source := gangSurgeRecoverySource(surgeIndex, targetRevision)
	promoted := gangSurgePromotedTarget(surgeIndex, targetRevision)
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{
			source.Index:   cloneTerminalStatus(source),
			promoted.Index: cloneTerminalStatus(promoted),
		},
	}
	input := gangSurgeRecoveryInput("owner-a", isvcName, namespace, store,
		cloneTerminalStatus(source), cloneTerminalStatus(promoted))
	input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
		t.Fatal("conflicting promoted target reached resource finalization")
		return false, nil
	}
	objects := []client.Object{
		gangSurgeReadyRecoveryPod(isvcName, namespace, source.Index, "leader", "oldrev"),
		gangSurgeReadyRecoveryPod(isvcName, namespace, source.Index, "worker", "oldrev"),
		gangSurgeReadyRecoveryPod(isvcName, namespace, surgeIndex, "leader", "newrev"),
		gangSurgeReadyRecoveryPod(isvcName, namespace, surgeIndex, "worker", "newrev"),
	}
	c := legacyNewFakeClient(t, objects...)
	plan := legacyMultiPodComponentPlan(workload.UpdateStrategySurgeThenDrain)
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: targetRevision}}

	done, err := gangSurgeUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], target)
	if err != nil || done {
		t.Fatalf("promoted-target conflict: done=%v err=%v", done, err)
	}
	persistedSource := store.statuses[source.Index]
	if persistedSource.Phase != workload.InstancePhaseReady || persistedSource.Operation != nil ||
		persistedSource.RunningRevision != source.RunningRevision {
		t.Fatalf("source claim was not rolled back: %+v", persistedSource)
	}
	persistedTarget := store.statuses[surgeIndex]
	if !gangSurgePromotedTargetMatches(&persistedTarget, targetRevision) {
		t.Fatalf("promoted target changed: %+v", persistedTarget)
	}
	for _, index := range []int32{source.Index, surgeIndex} {
		pods, listErr := query.LiveListPodsForInstance(context.Background(), c, namespace, isvcName, workload.ComponentEngine, index)
		if listErr != nil || len(pods) != 2 {
			t.Fatalf("instance %d pods after rollback: count=%d err=%v", index, len(pods), listErr)
		}
	}
}

func TestGangSurge_OccupiedTargetRollsBackAndReallocates(t *testing.T) {
	legacyResetExpectations(t)
	const isvcName, namespace = "gang-occupied-recover", "test-ns"
	const targetRevision = "gang-occupied-recover-engine-newrev"
	surgeIndex := int32(2)
	source := gangSurgeRecoverySource(surgeIndex, targetRevision)
	occupied := workload.InstanceStatus{
		Index:           surgeIndex,
		Incarnation:     8,
		Phase:           workload.InstancePhaseReady,
		RunningRevision: "unrelated-revision",
		ActiveOrdinal:   1,
	}
	occupiedIdentity := captureTerminalInstanceIdentity(&occupied)
	store := &terminalMutationStore{
		ownerUID: "owner-a",
		statuses: map[int32]workload.InstanceStatus{
			source.Index:   cloneTerminalStatus(source),
			occupied.Index: cloneTerminalStatus(occupied),
		},
	}
	input := gangSurgeRecoveryInput("owner-a", isvcName, namespace, store,
		cloneTerminalStatus(source), cloneTerminalStatus(occupied))
	input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
		t.Fatal("occupied target reached resource finalization")
		return false, nil
	}
	occupiedPod := gangSurgeReadyRecoveryPod(isvcName, namespace, surgeIndex, "leader", "unrelated")
	c := legacyNewFakeClient(t, occupiedPod)
	plan := legacyMultiPodComponentPlan(workload.UpdateStrategySurgeThenDrain)
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: targetRevision}}

	done, err := gangSurgeUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], target)
	if err != nil || done {
		t.Fatalf("occupied-target rollback: done=%v err=%v", done, err)
	}
	persistedSource := store.statuses[source.Index]
	if persistedSource.Phase != workload.InstancePhaseReady || persistedSource.Operation != nil ||
		persistedSource.RunningRevision != source.RunningRevision {
		t.Fatalf("source claim was not rolled back: %+v", persistedSource)
	}
	if persistedOccupied := store.statuses[surgeIndex]; !occupiedIdentity.matches(persistedOccupied) {
		t.Fatalf("occupied target changed: %+v", persistedOccupied)
	}
	pods, err := query.LiveListPodsForInstance(context.Background(), c, namespace, isvcName, workload.ComponentEngine, surgeIndex)
	if err != nil || len(pods) != 1 {
		t.Fatalf("occupied target pods after rollback: count=%d err=%v", len(pods), err)
	}

	input.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		cloneTerminalStatus(store.statuses[source.Index]),
		cloneTerminalStatus(store.statuses[surgeIndex]),
	}
	done, err = gangSurgeUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], target)
	if err != nil || done {
		t.Fatalf("reallocation pass: done=%v err=%v", done, err)
	}
	persistedSource = store.statuses[source.Index]
	if persistedSource.Operation == nil || persistedSource.Operation.SurgeIndex == nil ||
		*persistedSource.Operation.SurgeIndex == surgeIndex {
		t.Fatalf("source did not reallocate away from occupied index %d: %+v", surgeIndex, persistedSource)
	}
	if persistedOccupied := store.statuses[surgeIndex]; !occupiedIdentity.matches(persistedOccupied) {
		t.Fatalf("reallocation changed occupied target: %+v", persistedOccupied)
	}
}

func TestGangSurge_LegacyAdapterRecoversPromotedAndOccupiedTargets(t *testing.T) {
	tests := []struct {
		name         string
		targetStatus func(int32, string) workload.InstanceStatus
		withPods     bool
		wantRemoved  bool
	}{
		{
			name:         "promoted target",
			targetStatus: gangSurgePromotedTarget,
			withPods:     true,
			wantRemoved:  true,
		},
		{
			name: "occupied target",
			targetStatus: func(index int32, _ string) workload.InstanceStatus {
				return workload.InstanceStatus{Index: index, Incarnation: 9, Phase: workload.InstancePhaseReady, RunningRevision: "unrelated"}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			legacyResetExpectations(t)
			const isvcName, namespace = "gang-legacy-recovery", "test-ns"
			const targetRevision = "gang-legacy-recovery-engine-newrev"
			surgeIndex := int32(2)
			source := gangSurgeRecoverySource(surgeIndex, targetRevision)
			targetStatus := test.targetStatus(surgeIndex, targetRevision)
			store := &terminalMutationStore{statuses: map[int32]workload.InstanceStatus{
				source.Index:       cloneTerminalStatus(source),
				targetStatus.Index: cloneTerminalStatus(targetStatus),
			}}
			input := gangSurgeRecoveryInput("", isvcName, namespace, store,
				cloneTerminalStatus(source), cloneTerminalStatus(targetStatus))
			input.ApplyInstanceMutationsWithRetryBlock = nil
			input.RemoveInstance = func(_ context.Context, index int32) (bool, error) {
				if _, found := store.statuses[index]; !found {
					return false, nil
				}
				delete(store.statuses, index)
				return true, nil
			}
			var objects []client.Object
			if test.withPods {
				objects = append(objects,
					gangSurgeReadyRecoveryPod(isvcName, namespace, surgeIndex, "leader", "newrev"),
					gangSurgeReadyRecoveryPod(isvcName, namespace, surgeIndex, "worker", "newrev"),
				)
			}
			c := legacyNewFakeClient(t, objects...)
			plan := legacyMultiPodComponentPlan(workload.UpdateStrategySurgeThenDrain)
			target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: targetRevision}}

			done, err := gangSurgeUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], target)
			if err != nil {
				t.Fatal(err)
			}
			_, sourceFound := store.statuses[source.Index]
			if done != test.wantRemoved || sourceFound == test.wantRemoved {
				t.Fatalf("done=%v sourceFound=%v want removed=%v", done, sourceFound, test.wantRemoved)
			}
			persistedTarget := store.statuses[surgeIndex]
			if !captureTerminalInstanceIdentity(&targetStatus).matches(persistedTarget) {
				t.Fatalf("legacy recovery changed target: %+v", persistedTarget)
			}
		})
	}
}
