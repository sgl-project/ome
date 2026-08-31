package ops

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clocktesting "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

type deleteMutationStore struct {
	uid        types.UID
	generation int64
	statuses   map[int32]workload.InstanceStatus
	writes     int
	mutations  []int
}

func newDeleteMutationStore(owner client.Object, statuses []workload.InstanceStatus) *deleteMutationStore {
	store := &deleteMutationStore{
		uid: owner.GetUID(), generation: owner.GetGeneration(),
		statuses: make(map[int32]workload.InstanceStatus, len(statuses)),
	}
	for _, status := range statuses {
		store.statuses[status.Index] = cloneDeleteInstanceStatus(status)
	}
	return store
}

func (s *deleteMutationStore) apply(_ context.Context, mutations []workload.InstanceMutation, _ string, _ func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
	snapshot := workload.InstanceMutationSnapshot{
		OwnerUID: s.uid, OwnerGeneration: s.generation,
		Instances: make(map[int32]workload.InstanceStatus, len(s.statuses)),
	}
	for index, status := range s.statuses {
		snapshot.Instances[index] = cloneDeleteInstanceStatus(status)
	}
	for _, mutation := range mutations {
		if mutation.BatchPrecondition != nil && !mutation.BatchPrecondition(snapshot) {
			return workload.ErrStatusMutationPrecondition
		}
	}
	type commit struct {
		callback func(*workload.InstanceStatus, *workload.InstanceStatus)
		before   *workload.InstanceStatus
		after    *workload.InstanceStatus
	}
	commits := make([]commit, 0, len(mutations))
	for _, mutation := range mutations {
		status, found := s.statuses[mutation.Index]
		if mutation.Remove {
			if !found {
				continue
			}
			before := cloneDeleteInstanceStatus(status)
			delete(s.statuses, mutation.Index)
			commits = append(commits, commit{callback: mutation.OnCommit, before: &before})
			continue
		}
		before := cloneDeleteInstanceStatus(status)
		if !found {
			status.Index = mutation.Index
		}
		if !mutation.Mutate(&status) {
			continue
		}
		s.statuses[mutation.Index] = cloneDeleteInstanceStatus(status)
		after := cloneDeleteInstanceStatus(status)
		var beforePtr *workload.InstanceStatus
		if found {
			beforePtr = &before
		}
		commits = append(commits, commit{callback: mutation.OnCommit, before: beforePtr, after: &after})
	}
	if len(commits) == 0 {
		return nil
	}
	s.writes++
	s.mutations = append(s.mutations, len(mutations))
	for _, committed := range commits {
		if committed.callback != nil {
			committed.callback(committed.before, committed.after)
		}
	}
	return nil
}

func TestDeleteBatchAdmissionCommitsBeforeEffects(t *testing.T) {
	owner := deleteBatchOwner()
	statuses := []workload.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: workload.InstancePhaseReady},
		{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseReady},
		{Index: 2, Incarnation: 1, Phase: workload.InstancePhaseReady},
	}
	pods := map[int32][]*corev1.Pod{
		0: {deleteBatchPod(0)}, 1: {deleteBatchPod(1)}, 2: {deleteBatchPod(2)},
	}
	objects := []client.Object{pods[0][0], pods[1][0], pods[2][0]}
	c := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).WithObjects(objects...).Build()
	store := newDeleteMutationStore(owner, statuses)
	budget := int32(2)
	input := deleteBatchInput(owner, statuses)
	input.ScaleDownPodBatchSize = &budget
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	result, err := DeleteBatch(context.Background(), workload.Deps{Client: c, Expectations: workload.NewExpectations()},
		input, deleteBatchPlan(), []int32{0, 1, 2}, pods)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ImmediateRequeue || !result.InProgress || result.SelectedPodCost != 2 || result.Deferred != 1 {
		t.Fatalf("result = %+v", result)
	}
	if store.writes != 1 || len(store.mutations) != 1 || store.mutations[0] != 2 {
		t.Fatalf("status writes/mutations = %d/%v, want 1/[2]", store.writes, store.mutations)
	}
	if !deleteOwned(store.statuses[2]) || !deleteOwned(store.statuses[1]) || deleteOwned(store.statuses[0]) {
		t.Fatalf("admitted statuses = %+v", store.statuses)
	}
	list := &corev1.PodList{}
	if err := c.List(context.Background(), list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 3 {
		t.Fatalf("admission pass deleted Pods: got %d want 3", len(list.Items))
	}
}

func TestDeleteBatchBlockedInstanceDoesNotBlockPeer(t *testing.T) {
	owner := deleteBatchOwner()
	t0 := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	statuses := []workload.InstanceStatus{deleteOwnedStatus(1, t0), deleteOwnedStatus(0, t0)}
	pod0, pod1 := deleteBatchPod(0), deleteBatchPod(1)
	pods := map[int32][]*corev1.Pod{0: {pod0}, 1: {pod1}}
	c := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).WithObjects(pod0, pod1).Build()
	expectations := workload.NewExpectations()
	expectations.ExpectDeletes("prod", "llama", workload.ComponentEngine, 1, 1)
	store := newDeleteMutationStore(owner, statuses)
	input := deleteBatchInput(owner, statuses)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply

	result, err := DeleteBatch(context.Background(), workload.Deps{Client: c, Expectations: expectations},
		input, deleteBatchPlan(), nil, pods)
	if err != nil {
		t.Fatal(err)
	}
	if result.ImmediateRequeue || !result.InProgress {
		t.Fatalf("result = %+v", result)
	}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod1), &corev1.Pod{}); err != nil {
		t.Fatalf("blocked instance Pod should remain: %v", err)
	}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod0), &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("eligible peer Pod should be deleted, got %v", err)
	}
}

func TestDeleteBatchCompletionFinalizesThenRemovesOnce(t *testing.T) {
	owner := deleteBatchOwner()
	t0 := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	statuses := []workload.InstanceStatus{deleteOwnedStatus(3, t0), deleteOwnedStatus(2, t0)}
	store := newDeleteMutationStore(owner, statuses)
	input := deleteBatchInput(owner, statuses)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply
	var finalized []int32
	input.FinalizeInstanceResources = func(_ context.Context, index int32) (bool, error) {
		if store.writes != 0 {
			t.Fatalf("status removed before resource finalization")
		}
		finalized = append(finalized, index)
		return true, nil
	}
	expectations := workload.NewExpectations()
	expectations.ExpectDeletes("prod", "llama", workload.ComponentEngine, 2, 1)
	expectations.ExpectDeletes("prod", "llama", workload.ComponentEngine, 3, 1)

	result, err := DeleteBatch(context.Background(), workload.Deps{
		Client: fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build(), Expectations: expectations,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ImmediateRequeue || !result.InProgress {
		t.Fatalf("result = %+v", result)
	}
	if len(finalized) != 2 || finalized[0] != 3 || finalized[1] != 2 {
		t.Fatalf("finalized = %v, want [3 2]", finalized)
	}
	if store.writes != 1 || len(store.mutations) != 1 || store.mutations[0] != 2 || len(store.statuses) != 0 {
		t.Fatalf("store = writes:%d mutations:%v statuses:%v", store.writes, store.mutations, store.statuses)
	}
	for _, index := range []int32{2, 3} {
		if !expectations.Satisfied("prod", "llama", workload.ComponentEngine, index) {
			t.Errorf("expectations for confirmed-absent index %d were not forgotten", index)
		}
	}
}

func TestDeleteBatchAdmissionRejectsStaleOwnerOrLifecycleIdentity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*deleteMutationStore)
	}{
		{name: "owner UID", mutate: func(store *deleteMutationStore) { store.uid = "replacement-uid" }},
		{name: "generation", mutate: func(store *deleteMutationStore) { store.generation++ }},
		{name: "incarnation", mutate: func(store *deleteMutationStore) {
			status := store.statuses[1]
			status.Incarnation++
			store.statuses[1] = status
		}},
		{name: "phase", mutate: func(store *deleteMutationStore) {
			status := store.statuses[1]
			status.Phase = workload.InstancePhaseUpdating
			store.statuses[1] = status
		}},
		{name: "operation", mutate: func(store *deleteMutationStore) {
			status := store.statuses[1]
			status.Operation = &workload.InstanceOperation{ID: "restart-1", Type: workload.InstanceOperationRestart}
			store.statuses[1] = status
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			owner := deleteBatchOwner()
			statuses := []workload.InstanceStatus{{Index: 1, Incarnation: 2, Phase: workload.InstancePhaseReady}}
			store := newDeleteMutationStore(owner, statuses)
			test.mutate(store)
			input := deleteBatchInput(owner, statuses)
			input.ApplyInstanceMutationsWithRetryBlock = store.apply
			pod := deleteBatchPod(1)
			c := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).WithObjects(pod).Build()
			result, err := DeleteBatch(context.Background(), workload.Deps{Client: c}, input, deleteBatchPlan(),
				[]int32{1}, map[int32][]*corev1.Pod{1: {pod}})
			if err != nil {
				t.Fatal(err)
			}
			if !result.ImmediateRequeue || store.writes != 0 {
				t.Fatalf("result/writes = %+v/%d, want replan with zero writes", result, store.writes)
			}
			if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), &corev1.Pod{}); err != nil {
				t.Fatalf("stale admission affected Pod: %v", err)
			}
		})
	}
}

func TestDeleteBatchAdmissionAllowsConcurrentDerivedCounterChange(t *testing.T) {
	owner := deleteBatchOwner()
	statuses := []workload.InstanceStatus{{Index: 1, Incarnation: 2, Phase: workload.InstancePhaseReady, PodCount: 1}}
	store := newDeleteMutationStore(owner, statuses)
	status := store.statuses[1]
	status.PodCount = 9
	status.ReadyPodCount = 8
	store.statuses[1] = status
	input := deleteBatchInput(owner, statuses)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply
	c := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build()
	result, err := DeleteBatch(context.Background(), workload.Deps{Client: c}, input, deleteBatchPlan(),
		[]int32{1}, map[int32][]*corev1.Pod{1: deleteSelectionPods(1, 1)})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ImmediateRequeue || store.writes != 1 || !deleteOwned(store.statuses[1]) {
		t.Fatalf("result/store = %+v/%+v", result, store)
	}
	if store.statuses[1].PodCount != 9 || store.statuses[1].ReadyPodCount != 8 {
		t.Fatalf("concurrent counters were lost: %+v", store.statuses[1])
	}
}

func TestDeleteBatchCompletionRejectsWholeStaleBatchAndKeepsExpectations(t *testing.T) {
	owner := deleteBatchOwner()
	t0 := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	statuses := []workload.InstanceStatus{deleteOwnedStatus(1, t0), deleteOwnedStatus(0, t0)}
	store := newDeleteMutationStore(owner, statuses)
	changed := store.statuses[0]
	changed.Operation.ID = "replacement-delete"
	store.statuses[0] = changed
	input := deleteBatchInput(owner, statuses)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply
	finalized := 0
	input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
		finalized++
		return true, nil
	}
	expectations := workload.NewExpectations()
	expectations.ExpectDeletes("prod", "llama", workload.ComponentEngine, 0, 1)
	expectations.ExpectDeletes("prod", "llama", workload.ComponentEngine, 1, 1)

	result, err := DeleteBatch(context.Background(), workload.Deps{
		Client: fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build(), Expectations: expectations,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.ImmediateRequeue || store.writes != 0 || len(store.statuses) != 2 || finalized != 0 {
		t.Fatalf("result/store = %+v/%+v", result, store)
	}
	for _, index := range []int32{0, 1} {
		if expectations.Satisfied("prod", "llama", workload.ComponentEngine, index) {
			t.Errorf("expectations for stale index %d were forgotten before confirmed removal", index)
		}
	}
}

func TestDeleteBatchPreflightConfirmsPriorRemovalBeforeEffects(t *testing.T) {
	owner := deleteBatchOwner()
	statuses := []workload.InstanceStatus{deleteOwnedStatus(1, time.Now())}
	store := newDeleteMutationStore(owner, nil)
	input := deleteBatchInput(owner, statuses)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply
	finalized := 0
	input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
		finalized++
		return true, nil
	}
	expectations := workload.NewExpectations()
	expectations.ExpectDeletes("prod", "llama", workload.ComponentEngine, 1, 1)

	result, err := DeleteBatch(context.Background(), workload.Deps{
		Client: fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build(), Expectations: expectations,
	}, input, deleteBatchPlan(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ImmediateRequeue || store.writes != 0 || finalized != 0 {
		t.Fatalf("result/writes/finalized = %+v/%d/%d", result, store.writes, finalized)
	}
	if !expectations.Satisfied("prod", "llama", workload.ComponentEngine, 1) {
		t.Fatal("authoritatively absent status did not release delete expectations")
	}
}

func TestDeleteBatchRequiresOwnerAwareAtomicAdapter(t *testing.T) {
	owner := deleteBatchOwner()
	statuses := []workload.InstanceStatus{{Index: 1, Incarnation: 1, Phase: workload.InstancePhaseReady}}
	pod := deleteBatchPod(1)
	client := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).WithObjects(pod).Build()

	t.Run("owner UID", func(t *testing.T) {
		input := deleteBatchInput(&corev1.ConfigMap{}, statuses)
		input.ApplyInstanceMutationsWithRetryBlock = newDeleteMutationStore(owner, statuses).apply
		if _, err := DeleteBatch(context.Background(), workload.Deps{Client: client}, input, deleteBatchPlan(), []int32{1}, map[int32][]*corev1.Pod{1: {pod}}); err == nil {
			t.Fatal("expected missing owner UID to fail closed")
		}
	})

	t.Run("strong adapter", func(t *testing.T) {
		input := deleteBatchInput(owner, statuses)
		if _, err := DeleteBatch(context.Background(), workload.Deps{Client: client}, input, deleteBatchPlan(), []int32{1}, map[int32][]*corev1.Pod{1: {pod}}); err == nil {
			t.Fatal("expected missing owner-aware atomic adapter to fail closed")
		}
	})
}

func TestDeleteBatchIncompleteResourceFinalizationRetainsStatus(t *testing.T) {
	owner := deleteBatchOwner()
	statuses := []workload.InstanceStatus{deleteOwnedStatus(1, time.Now())}
	store := newDeleteMutationStore(owner, statuses)
	input := deleteBatchInput(owner, statuses)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply
	finalized := 0
	input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
		finalized++
		return false, nil
	}
	expectations := workload.NewExpectations()
	expectations.ExpectDeletes("prod", "llama", workload.ComponentEngine, 1, 1)

	result, err := DeleteBatch(context.Background(), workload.Deps{
		Client: fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build(), Expectations: expectations,
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.InProgress || result.ImmediateRequeue || finalized != 1 || store.writes != 0 || len(store.statuses) != 1 {
		t.Fatalf("result/finalized/store = %+v/%d/%+v", result, finalized, store)
	}
	if !deleteOwned(store.statuses[1]) {
		t.Fatalf("incomplete finalization changed status: %+v", store.statuses[1])
	}
	if expectations.Satisfied("prod", "llama", workload.ComponentEngine, 1) {
		t.Fatal("incomplete finalization cleared delete expectations")
	}
}

func TestDeleteBatchResourceFinalizationFailureRetainsStatus(t *testing.T) {
	owner := deleteBatchOwner()
	statuses := []workload.InstanceStatus{deleteOwnedStatus(1, time.Now())}
	store := newDeleteMutationStore(owner, statuses)
	input := deleteBatchInput(owner, statuses)
	input.ApplyInstanceMutationsWithRetryBlock = store.apply
	input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
		return false, errors.New("injected PodGroup delete failure")
	}
	_, err := DeleteBatch(context.Background(), workload.Deps{
		Client: fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build(),
	}, input, deleteBatchPlan(), nil, map[int32][]*corev1.Pod{})
	if err == nil || store.writes != 0 || len(store.statuses) != 1 {
		t.Fatalf("err/store = %v/%+v", err, store)
	}
}

func TestDeleteBatchAdmissionMetricsRequireConfirmedCommit(t *testing.T) {
	component := workload.ComponentType("metric-admission-commit")
	owner := deleteBatchOwner()
	statuses := []workload.InstanceStatus{{Index: 4, Incarnation: 2, Phase: workload.InstancePhaseReady}}
	store := newDeleteMutationStore(owner, statuses)
	input := deleteBatchInput(owner, statuses)
	input.Key.Component = component
	input.ApplyInstanceMutationsWithRetryBlock = store.apply
	plan := deleteBatchPlan()
	plan.Component = component
	budget := int32(1)
	input.ScaleDownPodBatchSize = &budget
	pods := map[int32][]*corev1.Pod{4: deleteSelectionPods(4, 2)}
	client := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build()

	startBatchCount, startBatchSum := deleteMetricHistogram(t, "ome_omenative_scale_down_batch_pods", string(component))
	startOversized := deleteMetricCounter(t, "ome_omenative_scale_down_oversized_batch_total", string(component))

	store.uid = "replacement-uid"
	result, err := DeleteBatch(context.Background(), workload.Deps{Client: client}, input, plan, []int32{4}, pods)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ImmediateRequeue || store.writes != 0 {
		t.Fatalf("rejected admission result/writes = %+v/%d", result, store.writes)
	}
	assertDeleteAdmissionMetrics(t, component, startBatchCount, startBatchSum, startOversized, 0)

	store.uid = owner.GetUID()
	result, err = DeleteBatch(context.Background(), workload.Deps{Client: client}, input, plan, []int32{4}, pods)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ImmediateRequeue || store.writes != 1 || !deleteOwned(store.statuses[4]) {
		t.Fatalf("committed admission result/store = %+v/%+v", result, store)
	}
	assertDeleteAdmissionMetrics(t, component, startBatchCount, startBatchSum, startOversized, 1)

	result, err = DeleteBatch(context.Background(), workload.Deps{Client: client}, input, plan, []int32{4}, pods)
	if err != nil {
		t.Fatal(err)
	}
	if !result.ImmediateRequeue || store.writes != 1 {
		t.Fatalf("stale retry result/writes = %+v/%d", result, store.writes)
	}
	assertDeleteAdmissionMetrics(t, component, startBatchCount, startBatchSum, startOversized, 1)
}

func TestDeleteBatchDurationMetricRequiresConfirmedCompletion(t *testing.T) {
	now := time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)

	t.Run("confirmed completion records once", func(t *testing.T) {
		component := workload.ComponentType("metric-completion-commit")
		owner := deleteBatchOwner()
		statuses := []workload.InstanceStatus{deleteOwnedStatus(4, now.Add(-45*time.Second))}
		store := newDeleteMutationStore(owner, statuses)
		input := deleteBatchInput(owner, statuses)
		input.Key.Component = component
		input.ApplyInstanceMutationsWithRetryBlock = store.apply
		input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) { return true, nil }
		plan := deleteBatchPlan()
		plan.Component = component
		client := fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build()

		startCount, startSum := deleteMetricHistogram(t, "ome_omenative_scale_down_instance_duration_seconds", string(component))
		result, err := DeleteBatch(context.Background(), workload.Deps{Client: client}, input, plan, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !result.ImmediateRequeue || store.writes != 1 || len(store.statuses) != 0 {
			t.Fatalf("completion result/store = %+v/%+v", result, store)
		}
		assertDeleteDurationMetric(t, component, startCount, startSum, 1, 45)

		result, err = DeleteBatch(context.Background(), workload.Deps{Client: client}, input, plan, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !result.ImmediateRequeue || store.writes != 1 {
			t.Fatalf("stale completion retry result/writes = %+v/%d", result, store.writes)
		}
		assertDeleteDurationMetric(t, component, startCount, startSum, 1, 45)
	})

	t.Run("resource finalization failure records nothing", func(t *testing.T) {
		component := workload.ComponentType("metric-completion-finalizer-failure")
		owner := deleteBatchOwner()
		statuses := []workload.InstanceStatus{deleteOwnedStatus(4, now.Add(-30*time.Second))}
		store := newDeleteMutationStore(owner, statuses)
		input := deleteBatchInput(owner, statuses)
		input.Key.Component = component
		input.ApplyInstanceMutationsWithRetryBlock = store.apply
		input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) {
			return false, errors.New("injected finalization failure")
		}
		plan := deleteBatchPlan()
		plan.Component = component
		startCount, startSum := deleteMetricHistogram(t, "ome_omenative_scale_down_instance_duration_seconds", string(component))

		_, err := DeleteBatch(context.Background(), workload.Deps{
			Client: fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build(),
		}, input, plan, nil, nil)
		if err == nil || store.writes != 0 {
			t.Fatalf("finalization err/writes = %v/%d", err, store.writes)
		}
		assertDeleteDurationMetric(t, component, startCount, startSum, 0, 0)
	})

	t.Run("status write failure records nothing", func(t *testing.T) {
		component := workload.ComponentType("metric-completion-status-failure")
		owner := deleteBatchOwner()
		statuses := []workload.InstanceStatus{deleteOwnedStatus(4, now.Add(-30*time.Second))}
		input := deleteBatchInput(owner, statuses)
		input.Key.Component = component
		input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) { return true, nil }
		input.ApplyInstanceMutationsWithRetryBlock = func(context.Context, []workload.InstanceMutation, string, func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
			return errors.New("injected status write failure")
		}
		plan := deleteBatchPlan()
		plan.Component = component
		startCount, startSum := deleteMetricHistogram(t, "ome_omenative_scale_down_instance_duration_seconds", string(component))

		_, err := DeleteBatch(context.Background(), workload.Deps{
			Client: fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build(),
		}, input, plan, nil, nil)
		if err == nil {
			t.Fatal("expected status write failure")
		}
		assertDeleteDurationMetric(t, component, startCount, startSum, 0, 0)
	})

	t.Run("unconfirmed status no-op records nothing", func(t *testing.T) {
		component := workload.ComponentType("metric-completion-unconfirmed")
		owner := deleteBatchOwner()
		statuses := []workload.InstanceStatus{deleteOwnedStatus(4, now.Add(-30*time.Second))}
		input := deleteBatchInput(owner, statuses)
		input.Key.Component = component
		input.FinalizeInstanceResources = func(context.Context, int32) (bool, error) { return true, nil }
		input.ApplyInstanceMutationsWithRetryBlock = func(context.Context, []workload.InstanceMutation, string, func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
			return nil
		}
		plan := deleteBatchPlan()
		plan.Component = component
		startCount, startSum := deleteMetricHistogram(t, "ome_omenative_scale_down_instance_duration_seconds", string(component))

		_, err := DeleteBatch(context.Background(), workload.Deps{
			Client: fake.NewClientBuilder().WithScheme(deleteBatchScheme(t)).Build(),
		}, input, plan, nil, nil)
		if err == nil {
			t.Fatalf("expected unconfirmed adapter error, got %v", err)
		}
		assertDeleteDurationMetric(t, component, startCount, startSum, 0, 0)
	})
}

func assertDeleteAdmissionMetrics(t *testing.T, component workload.ComponentType, startCount uint64, startSum, startOversized float64, committed uint64) {
	t.Helper()
	count, sum := deleteMetricHistogram(t, "ome_omenative_scale_down_batch_pods", string(component))
	if count-startCount != committed || sum-startSum != float64(committed*2) {
		t.Fatalf("batch metric delta = count:%d sum:%g, want count:%d sum:%d", count-startCount, sum-startSum, committed, committed*2)
	}
	oversized := deleteMetricCounter(t, "ome_omenative_scale_down_oversized_batch_total", string(component))
	if oversized-startOversized != float64(committed) {
		t.Fatalf("oversized metric delta = %g, want %d", oversized-startOversized, committed)
	}
}

func assertDeleteDurationMetric(t *testing.T, component workload.ComponentType, startCount uint64, startSum float64, committed uint64, seconds float64) {
	t.Helper()
	count, sum := deleteMetricHistogram(t, "ome_omenative_scale_down_instance_duration_seconds", string(component))
	if count-startCount != committed || sum-startSum != seconds {
		t.Fatalf("duration metric delta = count:%d sum:%g, want count:%d sum:%g", count-startCount, sum-startSum, committed, seconds)
	}
}

func deleteMetricHistogram(t *testing.T, name, component string) (uint64, float64) {
	t.Helper()
	metric := deleteMetricForComponent(t, name, component)
	if metric == nil || metric.Histogram == nil {
		return 0, 0
	}
	return metric.Histogram.GetSampleCount(), metric.Histogram.GetSampleSum()
}

func deleteMetricCounter(t *testing.T, name, component string) float64 {
	t.Helper()
	metric := deleteMetricForComponent(t, name, component)
	if metric == nil || metric.Counter == nil {
		return 0
	}
	return metric.Counter.GetValue()
}

func deleteMetricForComponent(t *testing.T, name, component string) *dto.Metric {
	t.Helper()
	families, err := metrics.Registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := metric.GetLabel()
			if len(labels) == 1 && labels[0].GetName() == "component" && labels[0].GetValue() == component {
				return metric
			}
		}
	}
	return nil
}

func deleteBatchScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func deleteBatchOwner() *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "ir", Namespace: "prod", UID: types.UID("ir-uid"), Generation: 7,
	}}
}

func deleteBatchInput(owner client.Object, statuses []workload.InstanceStatus) workload.ReconcileInput {
	clock := clocktesting.NewFakeClock(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC))
	return workload.ReconcileInput{
		OwnerObject:   owner,
		Key:           workload.Key{Namespace: "prod", OwnerName: "llama", Component: workload.ComponentEngine},
		ObservedState: workload.WorkloadObservedState{InstanceStatuses: statuses},
		Clock:         clock,
	}
}

func deleteBatchPlan() workload.ComponentPlan {
	return workload.ComponentPlan{Component: workload.ComponentEngine, InstanceReadyTimeout: time.Minute}
}

func deleteBatchPod(index int32) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "pod-" + string(rune('a'+index)), Namespace: "prod", UID: types.UID(fmt.Sprintf("pod-%d-uid", index)),
		Labels: map[string]string{query.LabelInstanceIdx: string(rune('0' + index))},
	}}
}
