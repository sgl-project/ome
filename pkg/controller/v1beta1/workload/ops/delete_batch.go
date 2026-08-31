package ops

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/obsmetrics"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/drain"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// deleteDrainKey is shared by every Pod in an Instance so Pod deletion clears
// the Instance's scale-down serving hold.
func deleteDrainKey(idx int32) string {
	return strconv.Itoa(int(idx))
}

// DeleteBatchResult describes whether the scale-down action still owns the
// pipeline and whether a fresh status commit requires an immediate replan.
type DeleteBatchResult struct {
	InProgress        bool
	ImmediateRequeue  bool
	SelectedPodCost   int32
	Deferred          int
	Oversized         bool
	RequeueAfter      time.Duration
	PolicyDeadlineDue bool
}

type deleteBatchCandidate struct {
	status workload.InstanceStatus
	pods   []*corev1.Pod
	cost   int32
}

type deleteBatchSelection struct {
	candidates []deleteBatchCandidate
	deferred   int
	oversized  bool
	fresh      bool
}

// DeleteBatch advances one durable, gang-atomic scale-down wave. Fresh
// candidates are committed as one status transaction and never receive an
// external effect in the admission pass. Persisted Delete-owned candidates
// resume from the authoritative Pod snapshot on later passes.
func DeleteBatch(
	ctx context.Context,
	deps workload.Deps,
	input workload.ReconcileInput,
	plan workload.ComponentPlan,
	extras []int32,
	podsByInstance map[int32][]*corev1.Pod,
) (DeleteBatchResult, error) {
	if deps.Client == nil {
		return DeleteBatchResult{}, fmt.Errorf("DeleteBatch: nil client")
	}
	if input.OwnerObject == nil || input.OwnerObject.GetUID() == "" {
		return DeleteBatchResult{}, fmt.Errorf("DeleteBatch: owner with UID is required")
	}
	selection, err := selectDeleteBatch(input.ObservedState.InstanceStatuses, extras, podsByInstance, input.ScaleDownPodBatchSize)
	if err != nil {
		return DeleteBatchResult{}, err
	}
	result := DeleteBatchResult{
		InProgress: len(selection.candidates) > 0 || selection.deferred > 0,
		Deferred:   selection.deferred,
		Oversized:  selection.oversized,
	}
	for _, candidate := range selection.candidates {
		result.SelectedPodCost += candidate.cost
	}
	obsmetrics.SetScaleDownActivePods(input.Key.Namespace, input.Key.OwnerName, string(plan.Component), result.SelectedPodCost)
	obsmetrics.SetScaleDownDeferredInstances(input.Key.Namespace, input.Key.OwnerName, string(plan.Component), result.Deferred)
	admittedInstances := 0
	defer func() {
		budgetValue := any("unbounded")
		if input.ScaleDownPodBatchSize != nil {
			budgetValue = *input.ScaleDownPodBatchSize
		}
		logf.FromContext(ctx).Info("OMENative scale-down wave",
			"namespace", input.Key.Namespace,
			"isvc", input.Key.OwnerName,
			"component", plan.Component,
			"podBudget", budgetValue,
			"activePodCost", result.SelectedPodCost,
			"activeInstances", len(selection.candidates),
			"admittedInstances", admittedInstances,
			"deferredInstances", result.Deferred)
	}()
	if len(selection.candidates) == 0 {
		return result, nil
	}
	if selection.fresh {
		committed, err := admitDeleteBatch(ctx, input, plan, selection.candidates)
		if err != nil {
			if errors.Is(err, workload.ErrStatusMutationPrecondition) || errors.Is(err, workload.ErrStatusOwnerGone) {
				result.ImmediateRequeue = true
				return result, nil
			}
			return DeleteBatchResult{}, fmt.Errorf("DeleteBatch: admit wave: %w", err)
		}
		result.ImmediateRequeue = committed
		if committed {
			admittedInstances = len(selection.candidates)
			obsmetrics.RecordScaleDownBatchPods(string(plan.Component), result.SelectedPodCost)
			if result.Oversized {
				obsmetrics.RecordScaleDownOversizedBatch(string(plan.Component))
			}
		}
		if result.ImmediateRequeue || !committed {
			return result, nil
		}
	} else {
		absent, err := preflightDeleteOwnedBatch(ctx, input, selection.candidates)
		if err != nil {
			if errors.Is(err, workload.ErrStatusMutationPrecondition) || errors.Is(err, workload.ErrStatusOwnerGone) {
				result.ImmediateRequeue = true
				return result, nil
			}
			return DeleteBatchResult{}, fmt.Errorf("DeleteBatch: verify owned wave: %w", err)
		}
		if len(absent) > 0 {
			for index := range absent {
				deps.ExpectationsCache().Forget(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, index)
			}
			result.ImmediateRequeue = true
			return result, nil
		}
	}

	completed, requeueAt, err := driveDeleteBatch(ctx, deps, input, plan, selection.candidates)
	if err != nil {
		if errors.Is(err, podreadiness.ErrPodIdentityChanged) {
			result.ImmediateRequeue = true
			return result, nil
		}
		return DeleteBatchResult{}, err
	}
	if !requeueAt.IsZero() {
		remaining := requeueAt.Sub(input.Now())
		if remaining <= 0 {
			result.PolicyDeadlineDue = true
		} else {
			result.RequeueAfter = remaining
		}
	}
	if len(completed) == 0 {
		return result, nil
	}
	committed, err := completeDeleteBatch(ctx, deps, input, completed)
	if err != nil {
		if errors.Is(err, workload.ErrStatusMutationPrecondition) || errors.Is(err, workload.ErrStatusOwnerGone) {
			result.ImmediateRequeue = true
			return result, nil
		}
		return DeleteBatchResult{}, fmt.Errorf("DeleteBatch: complete wave: %w", err)
	}
	result.ImmediateRequeue = committed
	return result, nil
}

func selectDeleteBatch(
	statuses []workload.InstanceStatus,
	extras []int32,
	podsByInstance map[int32][]*corev1.Pod,
	budget *int32,
) (deleteBatchSelection, error) {
	if budget != nil && *budget <= 0 {
		return deleteBatchSelection{}, fmt.Errorf("DeleteBatch: scale-down Pod batch size must be positive")
	}
	extraSet := make(map[int32]struct{}, len(extras))
	for _, index := range extras {
		extraSet[index] = struct{}{}
	}

	owned := make([]deleteBatchCandidate, 0)
	fresh := make([]deleteBatchCandidate, 0, len(extras))
	for _, original := range statuses {
		status := cloneDeleteInstanceStatus(original)
		candidate := deleteBatchCandidate{
			status: status,
			pods:   append([]*corev1.Pod(nil), podsByInstance[status.Index]...),
		}
		sort.SliceStable(candidate.pods, func(i, j int) bool {
			left, right := candidate.pods[i], candidate.pods[j]
			if left == nil || right == nil {
				return left == nil && right != nil
			}
			if left.Namespace != right.Namespace {
				return left.Namespace < right.Namespace
			}
			return left.Name < right.Name
		})
		candidate.cost = int32(len(candidate.pods))
		if candidate.cost == 0 {
			candidate.cost = 1
		}
		if deleteOwned(status) {
			owned = append(owned, candidate)
			continue
		}
		if _, ok := extraSet[status.Index]; ok {
			fresh = append(fresh, candidate)
		}
	}

	pool := owned
	selection := deleteBatchSelection{}
	blockedFresh := 0
	if len(pool) == 0 {
		pool = fresh
		selection.fresh = true
		sort.Slice(pool, func(i, j int) bool { return pool[i].status.Index > pool[j].status.Index })
	} else {
		blockedFresh = len(fresh)
		sort.SliceStable(pool, func(i, j int) bool {
			iStarted := pool[i].status.Operation.StartedAt.Time
			jStarted := pool[j].status.Operation.StartedAt.Time
			if iStarted.Equal(jStarted) {
				return pool[i].status.Index > pool[j].status.Index
			}
			return iStarted.Before(jStarted)
		})
	}

	var selectedCost int32
	for _, candidate := range pool {
		if budget != nil && len(selection.candidates) > 0 && selectedCost+candidate.cost > *budget {
			break
		}
		if budget != nil && len(selection.candidates) == 0 && candidate.cost > *budget {
			selection.oversized = true
		}
		selection.candidates = append(selection.candidates, candidate)
		selectedCost += candidate.cost
	}
	selection.deferred = len(pool) - len(selection.candidates) + blockedFresh
	return selection, nil
}

func admitDeleteBatch(ctx context.Context, input workload.ReconcileInput, plan workload.ComponentPlan, candidates []deleteBatchCandidate) (bool, error) {
	mutations := make([]workload.InstanceMutation, 0, len(candidates))
	expected := make(map[int32]workload.InstanceStatus, len(candidates))
	for _, candidate := range candidates {
		expected[candidate.status.Index] = cloneDeleteInstanceStatus(candidate.status)
		now := metav1.NewTime(input.Now())
		operation := workload.InstanceOperation{
			ID:             fmt.Sprintf("delete-%d-%d", candidate.status.Index, now.Unix()),
			Type:           workload.InstanceOperationDelete,
			Step:           "Drain",
			StartedAt:      now,
			LastProgressAt: now,
			Deadline:       metav1.NewTime(now.Add(plan.InstanceReadyTimeout)),
		}
		index := candidate.status.Index
		incarnation := candidate.status.Incarnation
		mutation := workload.InstanceMutation{
			Index: index,
			Mutate: func(status *workload.InstanceStatus) bool {
				status.Phase = workload.InstancePhaseDeleting
				status.Operation = cloneDeleteOperation(&operation)
				return true
			},
			Postcondition: func(status *workload.InstanceStatus) bool {
				return status != nil && status.Index == index && status.Incarnation == incarnation &&
					status.Phase == workload.InstancePhaseDeleting && status.Operation != nil &&
					status.Operation.ID == operation.ID && status.Operation.Type == operation.Type && status.Operation.Step == operation.Step
			},
		}
		mutations = append(mutations, mutation)
	}
	mutations[0].BatchPrecondition = deleteAdmissionGuard(input, expected, input.ObservedState.InstanceStatuses)
	return applyDeleteMutationBatch(ctx, input, mutations)
}

func preflightDeleteOwnedBatch(ctx context.Context, input workload.ReconcileInput, candidates []deleteBatchCandidate) (map[int32]struct{}, error) {
	if input.ApplyInstanceMutationsWithRetryBlock == nil {
		return nil, fmt.Errorf("DeleteBatch: owner-aware atomic status adapter is required")
	}
	absent := make(map[int32]struct{})
	mutations := make([]workload.InstanceMutation, 0, len(candidates))
	for _, candidate := range candidates {
		mutations = append(mutations, workload.InstanceMutation{
			Index: candidate.status.Index,
			Mutate: func(*workload.InstanceStatus) bool {
				return false
			},
		})
	}
	if len(mutations) == 0 {
		return absent, nil
	}
	ownerUID := input.OwnerObject.GetUID()
	mutations[0].BatchPrecondition = func(snapshot workload.InstanceMutationSnapshot) bool {
		clear(absent)
		if snapshot.OwnerUID != ownerUID {
			return false
		}
		for _, candidate := range candidates {
			current, found := snapshot.Instances[candidate.status.Index]
			if !found {
				absent[candidate.status.Index] = struct{}{}
				continue
			}
			if current.Incarnation != candidate.status.Incarnation || current.Phase != workload.InstancePhaseDeleting ||
				!sameDeleteOperation(current.Operation, candidate.status.Operation) {
				return false
			}
		}
		return true
	}
	if err := input.ApplyInstanceMutationsWithRetryBlock(ctx, mutations, "", nil); err != nil {
		return nil, err
	}
	return absent, nil
}

func driveDeleteBatch(
	ctx context.Context,
	deps workload.Deps,
	input workload.ReconcileInput,
	plan workload.ComponentPlan,
	candidates []deleteBatchCandidate,
) ([]deleteBatchCandidate, time.Time, error) {
	for _, candidate := range candidates {
		for _, pod := range candidate.pods {
			if pod.UID == "" {
				return nil, time.Time{}, fmt.Errorf("DeleteBatch: refuse to delete pod %s/%s without an observed UID", pod.Namespace, pod.Name)
			}
		}
	}
	expectationsSatisfied := make(map[int32]bool, len(candidates))
	var requeueAt time.Time
	for _, candidate := range candidates {
		for _, pod := range candidate.pods {
			if pod.DeletionTimestamp != nil {
				next, err := escalateStuckTerminatingWithDeadline(ctx, deps, input, pod, candidate.status.Index)
				if err != nil {
					if input.ScaleDownRequeueInterval <= 0 {
						return nil, time.Time{}, fmt.Errorf("DeleteBatch: evaluate stuck-Terminating pod %s: %w", pod.Name, err)
					}
					logf.FromContext(ctx).V(1).Info("stuck-Terminating escalation deferred", "pod", pod.Name, "error", err.Error())
				}
				requeueAt = earlierTime(requeueAt, next)
			}
		}
		expectationsSatisfied[candidate.status.Index] = deps.ExpectationsCache().Satisfied(
			input.Key.Namespace, input.Key.OwnerName, input.Key.Component, candidate.status.Index)
	}

	// Every serving hold in the selected wave lands before any member Pod can
	// be deleted, preserving gang drain atomicity across Instance boundaries.
	for _, candidate := range candidates {
		for _, pod := range candidate.pods {
			if !podreadiness.IsServing(pod) {
				continue
			}
			if err := podreadiness.MarkPodNotServing(ctx, deps.Client, deps.Reader(), pod,
				podreadiness.WriterDeleteDrain, deleteDrainKey(candidate.status.Index)); err != nil {
				return nil, time.Time{}, fmt.Errorf("DeleteBatch: mark not serving (instance=%d, pod=%s): %w", candidate.status.Index, pod.Name, err)
			}
		}
	}

	drainer := drain.NewBatcher(deps.Reader(), input.Key.Namespace)
	completed := make([]deleteBatchCandidate, 0)
	for _, candidate := range candidates {
		if len(candidate.pods) == 0 {
			if input.FinalizeInstanceResources != nil {
				complete, err := input.FinalizeInstanceResources(ctx, candidate.status.Index)
				if err != nil {
					return nil, time.Time{}, fmt.Errorf("DeleteBatch: finalize resources (instance=%d): %w", candidate.status.Index, err)
				}
				if !complete {
					continue
				}
			}
			completed = append(completed, candidate)
			continue
		}
		if !expectationsSatisfied[candidate.status.Index] {
			continue
		}
		drained := true
		for _, pod := range candidate.pods {
			hash := pod.Labels[query.LabelRevisionHash]
			if hash == "" {
				continue
			}
			serviceName := query.PerRevisionServiceName(input.Key.OwnerName, plan.Component, hash)
			podDrained, err := drainer.IsPodDrained(ctx, serviceName, pod)
			if err != nil {
				return nil, time.Time{}, fmt.Errorf("DeleteBatch: check drain (instance=%d, pod=%s): %w", candidate.status.Index, pod.Name, err)
			}
			if !podDrained {
				drained = false
			}
		}
		if !drained {
			continue
		}
		for _, pod := range candidate.pods {
			if pod.DeletionTimestamp != nil {
				continue
			}
			uid := pod.UID
			deps.ExpectationsCache().ExpectDeletes(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, candidate.status.Index, 1)
			if err := deps.Client.Delete(ctx, pod, client.Preconditions{UID: &uid}); err != nil {
				deps.ExpectationsCache().ObservedDelete(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, candidate.status.Index)
				// Absence and UID mismatch both prove the observed Pod identity
				// does not occupy its name.
				if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
					continue
				}
				return nil, time.Time{}, fmt.Errorf("DeleteBatch: delete pod %s/%s: %w", pod.Namespace, pod.Name, err)
			}
		}
	}
	return completed, requeueAt, nil
}

func earlierTime(current, candidate time.Time) time.Time {
	if candidate.IsZero() {
		return current
	}
	if current.IsZero() || candidate.Before(current) {
		return candidate
	}
	return current
}

func completeDeleteBatch(ctx context.Context, deps workload.Deps, input workload.ReconcileInput, candidates []deleteBatchCandidate) (bool, error) {
	expected := make(map[int32]workload.InstanceStatus, len(candidates))
	mutations := make([]workload.InstanceMutation, 0, len(candidates))
	for _, candidate := range candidates {
		expected[candidate.status.Index] = cloneDeleteInstanceStatus(candidate.status)
		index := candidate.status.Index
		mutations = append(mutations, workload.InstanceMutation{
			Index:  index,
			Remove: true,
			OnCommit: func(previous, _ *workload.InstanceStatus) {
				deps.ExpectationsCache().Forget(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, index)
				if previous != nil && previous.Operation != nil && !previous.Operation.StartedAt.IsZero() {
					seconds := input.Now().Sub(previous.Operation.StartedAt.Time).Seconds()
					if seconds >= 0 {
						obsmetrics.RecordScaleDownInstanceDuration(string(input.Key.Component), seconds)
					}
				}
			},
		})
	}
	mutations[0].BatchPrecondition = deleteCompletionGuard(input, expected)
	return applyDeleteMutationBatch(ctx, input, mutations)
}

func applyDeleteMutationBatch(ctx context.Context, input workload.ReconcileInput, mutations []workload.InstanceMutation) (bool, error) {
	if len(mutations) == 0 {
		return false, nil
	}
	if input.ApplyInstanceMutationsWithRetryBlock == nil {
		return false, fmt.Errorf("DeleteBatch: owner-aware atomic status adapter is required")
	}
	committed := 0
	for i := range mutations {
		callback := mutations[i].OnCommit
		mutations[i].OnCommit = func(previous, current *workload.InstanceStatus) {
			committed++
			if callback != nil {
				callback(previous, current)
			}
		}
	}
	if err := input.ApplyInstanceMutationsWithRetryBlock(ctx, mutations, "", nil); err != nil {
		return false, err
	}
	if committed != len(mutations) {
		return false, fmt.Errorf("DeleteBatch: status adapter confirmed %d of %d mutations", committed, len(mutations))
	}
	return true, nil
}

func deleteAdmissionGuard(input workload.ReconcileInput, expected map[int32]workload.InstanceStatus, planned []workload.InstanceStatus) func(workload.InstanceMutationSnapshot) bool {
	uid := input.OwnerObject.GetUID()
	generation := input.OwnerObject.GetGeneration()
	plannedIdentities := make(map[int32]workload.InstanceStatus, len(planned))
	for _, status := range planned {
		plannedIdentities[status.Index] = cloneDeleteInstanceStatus(status)
	}
	return func(snapshot workload.InstanceMutationSnapshot) bool {
		if snapshot.OwnerUID != uid || snapshot.OwnerGeneration != generation {
			return false
		}
		if len(snapshot.Instances) != len(plannedIdentities) {
			return false
		}
		for index, plannedStatus := range plannedIdentities {
			current, found := snapshot.Instances[index]
			if !found || !sameDeleteCandidate(current, plannedStatus) {
				return false
			}
		}
		for index, planned := range expected {
			current, found := snapshot.Instances[index]
			if !found || !sameDeleteCandidate(current, planned) {
				return false
			}
		}
		return true
	}
}

func deleteCompletionGuard(input workload.ReconcileInput, expected map[int32]workload.InstanceStatus) func(workload.InstanceMutationSnapshot) bool {
	uid := input.OwnerObject.GetUID()
	return func(snapshot workload.InstanceMutationSnapshot) bool {
		if snapshot.OwnerUID != uid {
			return false
		}
		for index, planned := range expected {
			current, found := snapshot.Instances[index]
			if !found || current.Incarnation != planned.Incarnation || current.Phase != workload.InstancePhaseDeleting ||
				!sameDeleteOperation(current.Operation, planned.Operation) {
				return false
			}
		}
		return true
	}
}

func deleteOwned(status workload.InstanceStatus) bool {
	return status.Phase == workload.InstancePhaseDeleting && status.Operation != nil && status.Operation.Type == workload.InstanceOperationDelete
}

func sameDeleteCandidate(current, planned workload.InstanceStatus) bool {
	return current.Index == planned.Index && current.Incarnation == planned.Incarnation && current.Phase == planned.Phase &&
		reflect.DeepEqual(current.Operation, planned.Operation)
}

func sameDeleteOperation(current, planned *workload.InstanceOperation) bool {
	return current != nil && planned != nil && current.Type == workload.InstanceOperationDelete && planned.Type == workload.InstanceOperationDelete &&
		current.ID == planned.ID
}

func cloneDeleteInstanceStatus(status workload.InstanceStatus) workload.InstanceStatus {
	copy := status
	copy.Operation = cloneDeleteOperation(status.Operation)
	return copy
}

func cloneDeleteOperation(operation *workload.InstanceOperation) *workload.InstanceOperation {
	if operation == nil {
		return nil
	}
	copy := *operation
	copy.HintTargetNodes = append([]string(nil), operation.HintTargetNodes...)
	return &copy
}
