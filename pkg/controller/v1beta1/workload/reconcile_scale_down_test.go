package workload_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocktesting "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/ops"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

func TestReconcileDeleteOwnedReboundFinishesBeforeRecreate(t *testing.T) {
	ctx := context.Background()
	scheme := makeScheme(t)
	in := minimalInput(t)
	plan := minimalPlan()
	plan.InstanceReadyTimeout = time.Minute
	started := metav1.NewTime(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))
	in.Clock = clocktesting.NewFakeClock(started.Add(2 * time.Minute))
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{{
		Index:       0,
		Incarnation: 1,
		Phase:       workload.InstancePhaseDeleting,
		Operation: &workload.InstanceOperation{
			ID:             "delete-0-rebound",
			Type:           workload.InstanceOperationDelete,
			Step:           "Drain",
			StartedAt:      started,
			LastProgressAt: started,
			Deadline:       metav1.NewTime(started.Add(time.Minute)),
		},
	}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: in.Key.Namespace,
		Name:      query.PodName(in.Key.OwnerName, workload.ComponentEngine, 0, "default", 0),
		UID:       "rebound-pod-uid",
		Labels: map[string]string{
			constants.InferenceServicePodLabelKey: in.Key.OwnerName,
			constants.OMEComponentLabel:           string(workload.ComponentEngine),
			query.LabelManagedBy:                  query.ManagedByOMENative,
			query.LabelInstanceIdx:                "0",
		},
	}}
	base := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()
	expectations := workload.NewExpectations()
	deps := workload.Deps{Client: base, APIReader: base, Expectations: expectations}
	store := installTestAtomicMutationStore(&in)
	warnings := 0
	in.WarnInstanceFailed = func(int32, string, string) { warnings++ }
	finalized := 0
	in.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
		finalized++
		return true, nil
	}
	in.AuthoritativePods = &workload.ComponentPodSnapshot{
		Pods:       []*corev1.Pod{pod},
		ByInstance: map[int32][]*corev1.Pod{0: {pod}},
	}

	result, err := workload.Reconcile(ctx, deps, in, plan, nil)
	if err != nil {
		t.Fatalf("delete pass: %v", err)
	}
	if result.Requeue || result.RequeueAfter != testScaleDownRequeueInterval {
		t.Fatalf("delete pass result = %+v", result)
	}
	status := store.status(0)
	if store.writes != 0 || status == nil || status.Phase != workload.InstancePhaseDeleting ||
		status.Operation == nil || status.Operation.Type != workload.InstanceOperationDelete || status.LastFailure != nil {
		t.Fatalf("delete pass altered durable status: writes=%d status=%+v", store.writes, store.status(0))
	}
	if warnings != 0 {
		t.Fatalf("delete pass emitted %d generic failure warnings", warnings)
	}
	if err := base.Get(ctx, client.ObjectKeyFromObject(pod), &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("delete pass did not remove old Pod: %v", err)
	}
	if finalized != 0 {
		t.Fatalf("resources finalized before Pods disappeared: %d", finalized)
	}

	expectations.ObservedDelete(in.Key.Namespace, in.Key.OwnerName, in.Key.Component, 0)
	store.sync(&in)
	in.AuthoritativePods = &workload.ComponentPodSnapshot{ByInstance: map[int32][]*corev1.Pod{}}
	result, err = workload.Reconcile(ctx, deps, in, plan, nil)
	if err != nil {
		t.Fatalf("completion pass: %v", err)
	}
	if !result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("completion pass must force a fresh plan, got %+v", result)
	}
	if finalized != 1 || store.writes != 1 || store.status(0) != nil {
		t.Fatalf("completion state = finalized:%d writes:%d status:%+v", finalized, store.writes, store.status(0))
	}
	pods := &corev1.PodList{}
	if err := base.List(ctx, pods, client.InNamespace(in.Key.Namespace)); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 0 {
		t.Fatalf("completion pass recreated %d Pods from its stale plan", len(pods.Items))
	}

	store.sync(&in)
	result, err = workload.Reconcile(ctx, deps, in, plan, nil)
	if err != nil {
		t.Fatalf("recreate pass: %v", err)
	}
	if result.RequeueAfter != ops.CreateRequeueInterval {
		t.Fatalf("recreate pass result = %+v", result)
	}
	status = store.status(0)
	if status == nil || status.Phase != workload.InstancePhaseCreating || store.writes != 2 {
		t.Fatalf("recreated status/writes = %+v/%d", status, store.writes)
	}
	pods = &corev1.PodList{}
	if err := base.List(ctx, pods, client.InNamespace(in.Key.Namespace)); err != nil {
		t.Fatal(err)
	}
	if len(pods.Items) != 1 {
		t.Fatalf("recreate pass Pods = %d, want 1", len(pods.Items))
	}
}

func TestReconcileFreshScaleDownAdmissionEndsThePass(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	expired := metav1.NewTime(now.Add(-time.Minute))
	in := minimalInput(t)
	in.Clock = clocktesting.NewFakeClock(now)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady},
		{
			Index:       1,
			Incarnation: 1,
			PodCount:    1,
			Phase:       workload.InstancePhaseRestarting,
			Operation: &workload.InstanceOperation{
				ID:             "restart-1",
				Type:           workload.InstanceOperationRestart,
				Step:           "WaitReady",
				StartedAt:      metav1.NewTime(now.Add(-2 * time.Minute)),
				LastProgressAt: metav1.NewTime(now.Add(-2 * time.Minute)),
				Deadline:       expired,
			},
		},
	}
	in.ObservedState.RetryBlocks = []workload.RetryBlock{{TargetRevision: "superseded"}}
	in.AuthoritativePods = &workload.ComponentPodSnapshot{ByInstance: map[int32][]*corev1.Pod{}}
	store := installTestAtomicMutationStore(&in)
	in.ApplyInstanceMutations = func(ctx context.Context, mutations []workload.InstanceMutation) error {
		return store.apply(ctx, mutations, "", nil)
	}
	warnings := 0
	in.WarnInstanceFailed = func(int32, string, string) { warnings++ }
	prunes := 0
	in.MutateRetryBlock = func(context.Context, string, func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
		prunes++
		return nil
	}
	plan := minimalPlan()
	plan.InstanceReadyTimeout = time.Minute
	client := fake.NewClientBuilder().WithScheme(makeScheme(t)).Build()

	result, err := workload.Reconcile(ctx, workload.Deps{Client: client, APIReader: client}, in, plan, nil)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !result.Requeue || result.RequeueAfter != 0 {
		t.Fatalf("result = %+v, want immediate requeue", result)
	}
	status := store.status(1)
	if status == nil || status.Phase != workload.InstancePhaseDeleting || status.Operation == nil ||
		status.Operation.Type != workload.InstanceOperationDelete || status.LastFailure != nil {
		t.Fatalf("admitted status = %+v, want durable Delete ownership", status)
	}
	if store.writes != 1 {
		t.Fatalf("status writes = %d, want only the Delete admission", store.writes)
	}
	if warnings != 0 || prunes != 0 {
		t.Fatalf("post-boundary effects: warnings=%d RetryBlock prunes=%d, want 0/0", warnings, prunes)
	}
}

func TestExecuteActiveScaleDownExcludesDeferredExtrasFromEscalation(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	expiredOperation := func(id string) *workload.InstanceOperation {
		return &workload.InstanceOperation{
			ID:             id,
			Type:           workload.InstanceOperationRestart,
			Step:           "WaitReady",
			StartedAt:      metav1.NewTime(now.Add(-2 * time.Minute)),
			LastProgressAt: metav1.NewTime(now.Add(-2 * time.Minute)),
			Deadline:       metav1.NewTime(now.Add(-time.Minute)),
		}
	}
	in := minimalInput(t)
	in.Clock = clocktesting.NewFakeClock(now)
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, PodCount: 1, Phase: workload.InstancePhaseRestarting, Operation: expiredOperation("retained")},
		{Index: 1, Incarnation: 1, PodCount: 1, Phase: workload.InstancePhaseRestarting, Operation: expiredOperation("deferred-extra")},
		{
			Index:       2,
			Incarnation: 1,
			PodCount:    1,
			Phase:       workload.InstancePhaseDeleting,
			Operation: &workload.InstanceOperation{
				ID:             "active-delete",
				Type:           workload.InstanceOperationDelete,
				Step:           "Drain",
				StartedAt:      metav1.NewTime(now.Add(-time.Minute)),
				LastProgressAt: metav1.NewTime(now.Add(-time.Minute)),
			},
		},
	}
	in.ObservedState.RetryBlocks = []workload.RetryBlock{{TargetRevision: "superseded"}}
	store := installTestAtomicMutationStore(&in)
	in.ApplyInstanceMutations = func(ctx context.Context, mutations []workload.InstanceMutation) error {
		return store.apply(ctx, mutations, "", nil)
	}
	warnings := 0
	in.WarnInstanceFailed = func(int32, string, string) { warnings++ }
	prunes := 0
	in.MutateRetryBlock = func(context.Context, string, func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
		prunes++
		return nil
	}
	pod := enginePod(in.Key.OwnerName, in.Key.Namespace, 2)
	client := fake.NewClientBuilder().WithScheme(makeScheme(t)).WithObjects(pod).Build()
	deps := workload.Deps{Client: client, APIReader: client, Expectations: workload.NewExpectations()}
	snapshot := workload.SnapshotWithPodsForTest(in, map[int32][]*corev1.Pod{2: {pod}})
	decision := workload.Decision{
		Actions:  []workload.PlannedAction{{Kind: workload.ActionScaleDown, Extras: []int32{1, 2}}},
		Escalate: true,
	}

	result, err := workload.Execute(ctx, deps, in, minimalPlan(), nil, snapshot, decision)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Requeue || result.RequeueAfter != testScaleDownRequeueInterval {
		t.Fatalf("result = %+v, want configured scale-down poll", result)
	}
	if got := store.status(0); got == nil || got.Phase != workload.InstancePhaseFailed {
		t.Fatalf("retained expired Instance was not escalated: %+v", got)
	}
	if got := store.status(1); got == nil || got.Phase != workload.InstancePhaseRestarting || got.LastFailure != nil {
		t.Fatalf("deferred scale-down extra was mutated: %+v", got)
	}
	if got := store.status(2); got == nil || got.Phase != workload.InstancePhaseDeleting {
		t.Fatalf("active Delete ownership changed: %+v", got)
	}
	if store.writes != 1 || warnings != 1 || prunes != 1 {
		t.Fatalf("end-of-pass effects: writes=%d warnings=%d prunes=%d, want 1/1/1", store.writes, warnings, prunes)
	}
}

func TestReconcileActiveScaleDownWithoutPollSchedulesForceDeleteBoundary(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	in := minimalInput(t)
	in.Clock = clocktesting.NewFakeClock(now)
	in.ScaleDownRequeueInterval = 0
	in.ForceDelete = &workload.ForceDeletePolicy{
		OverdueSlack:             2 * time.Minute,
		NodeUnreachableThreshold: 5 * time.Minute,
	}
	in.ObservedState.InstanceStatuses = []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady},
		{
			Index:       1,
			Incarnation: 1,
			Phase:       workload.InstancePhaseDeleting,
			Operation: &workload.InstanceOperation{
				ID:        "active-delete",
				Type:      workload.InstanceOperationDelete,
				Step:      "Drain",
				StartedAt: metav1.NewTime(now.Add(-time.Minute)),
			},
		},
	}
	deletionTimestamp := metav1.NewTime(now.Add(-time.Minute))
	pod := enginePod(in.Key.OwnerName, in.Key.Namespace, 1)
	pod.DeletionTimestamp = &deletionTimestamp
	in.AuthoritativePods = &workload.ComponentPodSnapshot{
		Pods:       []*corev1.Pod{pod},
		ByInstance: map[int32][]*corev1.Pod{1: {pod}},
	}
	store := installTestAtomicMutationStore(&in)
	client := fake.NewClientBuilder().WithScheme(makeScheme(t)).Build()

	result, err := workload.Reconcile(ctx, workload.Deps{
		Client: client, APIReader: client, Expectations: workload.NewExpectations(),
	}, in, minimalPlan(), nil)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if result.Requeue || result.RequeueAfter != time.Minute+time.Nanosecond {
		t.Fatalf("result = %+v, want force-delete boundary in 1m+1ns", result)
	}
	if got := store.status(1); got == nil || got.Phase != workload.InstancePhaseDeleting {
		t.Fatalf("active Delete status changed: %+v", got)
	}
}
