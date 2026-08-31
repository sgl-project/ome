package ops

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestSelectDeleteBatchFreshGangAwarePrefix(t *testing.T) {
	tests := []struct {
		name      string
		indices   []int32
		costs     map[int32]int
		budget    *int32
		want      []int32
		deferred  int
		oversized bool
		wantCost  int32
	}{
		{name: "unbounded", indices: []int32{0, 1, 2}, costs: map[int32]int{0: 1, 1: 1, 2: 1}, want: []int32{2, 1, 0}, wantCost: 3},
		{name: "exact", indices: []int32{0, 1, 2}, costs: map[int32]int{0: 2, 1: 3, 2: 5}, budget: int32Pointer(8), want: []int32{2, 1}, deferred: 1, wantCost: 8},
		{name: "under", indices: []int32{0, 1}, costs: map[int32]int{0: 2, 1: 3}, budget: int32Pointer(8), want: []int32{1, 0}, wantCost: 5},
		{name: "oversized first", indices: []int32{0, 1}, costs: map[int32]int{0: 1, 1: 9}, budget: int32Pointer(8), want: []int32{1}, deferred: 1, oversized: true, wantCost: 9},
		{name: "non fitting gang closes prefix", indices: []int32{0, 1, 2}, costs: map[int32]int{0: 1, 1: 8, 2: 4}, budget: int32Pointer(5), want: []int32{2}, deferred: 2, wantCost: 4},
		{name: "podless costs one", indices: []int32{0, 1, 2}, costs: map[int32]int{}, budget: int32Pointer(2), want: []int32{2, 1}, deferred: 1, wantCost: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			statuses := make([]workload.InstanceStatus, 0, len(test.indices))
			pods := make(map[int32][]*corev1.Pod, len(test.costs))
			for _, index := range test.indices {
				statuses = append(statuses, workload.InstanceStatus{Index: index, Incarnation: 1, Phase: workload.InstancePhaseReady})
				pods[index] = deleteSelectionPods(index, test.costs[index])
			}
			selection, err := selectDeleteBatch(statuses, test.indices, pods, test.budget)
			if err != nil {
				t.Fatalf("selectDeleteBatch: %v", err)
			}
			if got := selectedDeleteIndices(selection); !equalInt32Slices(got, test.want) {
				t.Fatalf("selected indices = %v, want %v", got, test.want)
			}
			if selection.deferred != test.deferred {
				t.Errorf("deferred = %d, want %d", selection.deferred, test.deferred)
			}
			if selection.oversized != test.oversized {
				t.Errorf("oversized = %t, want %t", selection.oversized, test.oversized)
			}
			var cost int32
			for _, candidate := range selection.candidates {
				cost += candidate.cost
			}
			if cost != test.wantCost {
				t.Errorf("selected cost = %d, want %d", cost, test.wantCost)
			}
		})
	}
}

func TestSelectDeleteBatchEightPodGangs(t *testing.T) {
	const instances = 20
	statuses := make([]workload.InstanceStatus, 0, instances)
	extras := make([]int32, 0, instances)
	pods := make(map[int32][]*corev1.Pod, instances)
	for index := int32(0); index < instances; index++ {
		statuses = append(statuses, workload.InstanceStatus{Index: index, Phase: workload.InstancePhaseReady})
		extras = append(extras, index)
		pods[index] = deleteSelectionPods(index, 8)
	}
	selection, err := selectDeleteBatch(statuses, extras, pods, int32Pointer(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.candidates) != 12 || selection.deferred != 8 {
		t.Fatalf("selected/deferred = %d/%d, want 12/8", len(selection.candidates), selection.deferred)
	}
	var cost int32
	for _, candidate := range selection.candidates {
		cost += candidate.cost
	}
	if cost != 96 {
		t.Fatalf("cost = %d, want 96", cost)
	}
}

func TestSelectDeleteBatchDeleteOwnedOrderingAndFreshClosure(t *testing.T) {
	t0 := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	statuses := []workload.InstanceStatus{
		{Index: 9, Phase: workload.InstancePhaseReady},
		deleteOwnedStatus(2, t0.Add(time.Minute)),
		deleteOwnedStatus(7, t0),
		deleteOwnedStatus(5, t0),
	}
	pods := map[int32][]*corev1.Pod{
		2: deleteSelectionPods(2, 2),
		7: deleteSelectionPods(7, 2),
		5: deleteSelectionPods(5, 2),
		9: deleteSelectionPods(9, 1),
	}
	selection, err := selectDeleteBatch(statuses, []int32{9}, pods, int32Pointer(4))
	if err != nil {
		t.Fatal(err)
	}
	if selection.fresh {
		t.Fatal("Delete-owned work must close fresh admission")
	}
	if got, want := selectedDeleteIndices(selection), []int32{7, 5}; !equalInt32Slices(got, want) {
		t.Fatalf("selected = %v, want %v", got, want)
	}
	if selection.deferred != 2 {
		t.Fatalf("deferred = %d, want 2 (one owned and one fresh)", selection.deferred)
	}
}

func TestSelectDeleteBatchCountsTerminatingPods(t *testing.T) {
	now := metav1.Now()
	pods := deleteSelectionPods(3, 3)
	pods[1].DeletionTimestamp = &now
	selection, err := selectDeleteBatch(
		[]workload.InstanceStatus{{Index: 3, Phase: workload.InstancePhaseReady}},
		[]int32{3}, map[int32][]*corev1.Pod{3: pods}, int32Pointer(2),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.candidates) != 1 || selection.candidates[0].cost != 3 || !selection.oversized {
		t.Fatalf("selection = %+v, want one oversized cost-3 candidate", selection)
	}
}

func TestSelectDeleteBatchSortsCopiedPodSlices(t *testing.T) {
	original := []*corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "ns-b", Name: "pod-a"}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "ns-a", Name: "pod-z"}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "ns-a", Name: "pod-a"}},
	}
	selection, err := selectDeleteBatch(
		[]workload.InstanceStatus{{Index: 3, Phase: workload.InstancePhaseReady}},
		[]int32{3}, map[int32][]*corev1.Pod{3: original}, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.candidates) != 1 {
		t.Fatalf("selected candidates = %d, want 1", len(selection.candidates))
	}
	if got, want := podKeys(selection.candidates[0].pods), []string{"ns-a/pod-a", "ns-a/pod-z", "ns-b/pod-a"}; !equalStringSlices(got, want) {
		t.Fatalf("selected Pod order = %v, want %v", got, want)
	}
	if got, want := podKeys(original), []string{"ns-b/pod-a", "ns-a/pod-z", "ns-a/pod-a"}; !equalStringSlices(got, want) {
		t.Fatalf("authoritative Pod bucket was mutated: got %v, want %v", got, want)
	}
}

func TestSelectDeleteBatchRejectsInvalidBudget(t *testing.T) {
	for _, budget := range []int32{0, -1} {
		_, err := selectDeleteBatch(
			[]workload.InstanceStatus{{Index: 0, Phase: workload.InstancePhaseReady}},
			[]int32{0}, nil, &budget,
		)
		if err == nil {
			t.Errorf("budget %d: expected error", budget)
		}
	}
}

func TestSelectDeleteBatchTwoThousandDeterministic(t *testing.T) {
	const replicas = 2000
	statuses := make([]workload.InstanceStatus, 0, replicas-1)
	extras := make([]int32, 0, replicas-1)
	pods := make(map[int32][]*corev1.Pod, replicas-1)
	for index := int32(1); index < replicas; index++ {
		statuses = append(statuses, workload.InstanceStatus{Index: index, Phase: workload.InstancePhaseReady})
		extras = append(extras, index)
		pods[index] = deleteSelectionPods(index, 1)
	}
	selection, err := selectDeleteBatch(statuses, extras, pods, int32Pointer(100))
	if err != nil {
		t.Fatal(err)
	}
	if len(selection.candidates) != 100 || selection.deferred != 1899 {
		t.Fatalf("selected/deferred = %d/%d, want 100/1899", len(selection.candidates), selection.deferred)
	}
	if selection.candidates[0].status.Index != 1999 || selection.candidates[99].status.Index != 1900 {
		t.Fatalf("selected bounds = %d..%d, want 1999..1900", selection.candidates[0].status.Index, selection.candidates[99].status.Index)
	}
}

func TestDeleteBatchTwoThousandWaveAndWriteBounds(t *testing.T) {
	tests := []struct {
		name          string
		podCost       int
		withPodGroups bool
		wantWaves     int
		wantPasses    int
		wantWrites    int
	}{
		{name: "single pod", podCost: 1, wantWaves: 20, wantPasses: 61, wantWrites: 40},
		{name: "eight pod gang without PodGroups", podCost: 8, wantWaves: 167, wantPasses: 502, wantWrites: 334},
		{name: "eight pod gang with PodGroups", podCost: 8, withPodGroups: true, wantWaves: 167, wantPasses: 669, wantWrites: 334},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			waves, passes, writes := driveImmediateDeleteConvergence(t, 1999, test.podCost, 100, test.withPodGroups)
			if waves != test.wantWaves || passes != test.wantPasses || writes != test.wantWrites {
				t.Fatalf("waves/passes/writes = %d/%d/%d, want %d/%d/%d",
					waves, passes, writes, test.wantWaves, test.wantPasses, test.wantWrites)
			}
		})
	}
}

func driveImmediateDeleteConvergence(t *testing.T, instances, podCost int, budgetValue int32, withPodGroups bool) (waves, passes, writes int) {
	t.Helper()
	owner := deleteBatchOwner()
	statuses := make([]workload.InstanceStatus, 0, instances)
	pods := make(map[int32][]*corev1.Pod, instances)
	for index := int32(1); index <= int32(instances); index++ {
		statuses = append(statuses, workload.InstanceStatus{Index: index, Incarnation: 1, Phase: workload.InstancePhaseReady})
		pods[index] = deleteSelectionPods(index, podCost)
		for ordinal, pod := range pods[index] {
			pod.Namespace = owner.Namespace
			pod.UID = k8stypes.UID(fmt.Sprintf("pod-%d-%d-uid", index, ordinal))
		}
	}
	store := newDeleteMutationStore(owner, statuses)
	client := newDeleteFailureBaseClient(t)
	expectations := workload.NewExpectations()
	podGroupDeleteAccepted := make(map[int32]bool)

	for {
		observed := make([]workload.InstanceStatus, 0, len(store.statuses))
		for _, status := range store.statuses {
			observed = append(observed, cloneDeleteInstanceStatus(status))
		}
		sort.Slice(observed, func(i, j int) bool { return observed[i].Index < observed[j].Index })
		extras := make([]int32, 0, len(observed))
		active := make([]int32, 0)
		for _, status := range observed {
			extras = append(extras, status.Index)
			if deleteOwned(status) {
				active = append(active, status.Index)
			}
		}
		fresh := len(observed) > 0 && len(active) == 0
		input := deleteBatchInput(owner, observed)
		input.ScaleDownPodBatchSize = &budgetValue
		input.ApplyInstanceMutationsWithRetryBlock = store.apply
		input.FinalizeInstanceResources = func(_ context.Context, index int32) (bool, error) {
			if !withPodGroups {
				return true, nil
			}
			if podGroupDeleteAccepted[index] {
				delete(podGroupDeleteAccepted, index)
				return true, nil
			}
			podGroupDeleteAccepted[index] = true
			return false, nil
		}

		result, err := DeleteBatch(context.Background(), workload.Deps{
			Client: client, Expectations: expectations,
		}, input, deleteBatchPlan(), extras, pods)
		if err != nil {
			t.Fatal(err)
		}
		passes++
		if len(observed) == 0 {
			if result.InProgress || result.ImmediateRequeue {
				t.Fatalf("final verification remained active: %+v", result)
			}
			break
		}
		if fresh {
			waves++
			if !result.ImmediateRequeue {
				t.Fatalf("wave %d admission did not end the pass: %+v", waves, result)
			}
			continue
		}
		for _, index := range active {
			if len(pods[index]) > 0 {
				pods[index] = nil
			}
		}
	}
	return waves, passes, store.writes
}

func deleteOwnedStatus(index int32, started time.Time) workload.InstanceStatus {
	return workload.InstanceStatus{
		Index:       index,
		Incarnation: 4,
		Phase:       workload.InstancePhaseDeleting,
		Operation: &workload.InstanceOperation{
			ID:        fmt.Sprintf("delete-%d", index),
			Type:      workload.InstanceOperationDelete,
			Step:      "Drain",
			StartedAt: metav1.NewTime(started),
		},
	}
}

func deleteSelectionPods(index int32, count int) []*corev1.Pod {
	pods := make([]*corev1.Pod, 0, count)
	for ordinal := 0; ordinal < count; ordinal++ {
		pods = append(pods, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d-%d", index, ordinal)}})
	}
	return pods
}

func selectedDeleteIndices(selection deleteBatchSelection) []int32 {
	indices := make([]int32, 0, len(selection.candidates))
	for _, candidate := range selection.candidates {
		indices = append(indices, candidate.status.Index)
	}
	return indices
}

func int32Pointer(value int32) *int32 { return &value }

func equalInt32Slices(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func podKeys(pods []*corev1.Pod) []string {
	keys := make([]string, 0, len(pods))
	for _, pod := range pods {
		keys = append(keys, pod.Namespace+"/"+pod.Name)
	}
	return keys
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
