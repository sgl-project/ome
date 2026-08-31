package ops

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

type terminalMutationStore struct {
	ownerUID   k8stypes.UID
	statuses   map[int32]workload.InstanceStatus
	retryBlock map[string]workload.RetryBlock
	readErr    error
	applyErr   error
	applyCall  int
	writes     int
	log        *[]string
}

func (s *terminalMutationStore) apply(
	_ context.Context,
	mutations []workload.InstanceMutation,
	targetRevision string,
	mutateRetryBlock func(*workload.RetryBlock) workload.RetryBlockDisposition,
) error {
	s.applyCall++
	if s.log != nil {
		*s.log = append(*s.log, "status")
	}
	if s.readErr != nil {
		return s.readErr
	}
	snapshot := workload.InstanceMutationSnapshot{
		OwnerUID:  s.ownerUID,
		Instances: cloneTerminalStatuses(s.statuses),
	}
	for _, mutation := range mutations {
		if mutation.BatchPrecondition != nil && !mutation.BatchPrecondition(snapshot) {
			return workload.ErrStatusMutationPrecondition
		}
	}

	next := cloneTerminalStatuses(s.statuses)
	type callback struct {
		fn      func(*workload.InstanceStatus, *workload.InstanceStatus)
		before  *workload.InstanceStatus
		current *workload.InstanceStatus
	}
	callbacks := make([]callback, 0, len(mutations))
	changed := false
	for _, mutation := range mutations {
		current, found := next[mutation.Index]
		if mutation.Remove {
			if !found {
				continue
			}
			before := cloneTerminalStatus(current)
			if mutation.Precondition != nil && !mutation.Precondition(&before) {
				continue
			}
			delete(next, mutation.Index)
			changed = true
			if mutation.OnCommit != nil {
				callbacks = append(callbacks, callback{fn: mutation.OnCommit, before: &before})
			}
			continue
		}
		if !found {
			current = workload.InstanceStatus{Index: mutation.Index}
		}
		before := cloneTerminalStatus(current)
		if mutation.Precondition != nil && !mutation.Precondition(&before) {
			continue
		}
		if !mutation.Mutate(&current) {
			continue
		}
		next[mutation.Index] = current
		changed = true
		if mutation.OnCommit != nil {
			committed := cloneTerminalStatus(current)
			callbacks = append(callbacks, callback{fn: mutation.OnCommit, before: &before, current: &committed})
		}
	}
	nextRetryBlocks := cloneTerminalRetryBlocks(s.retryBlock)
	retryBlockChanged := false
	if mutateRetryBlock != nil {
		block, found := nextRetryBlocks[targetRevision]
		if !found {
			block = workload.RetryBlock{TargetRevision: targetRevision}
		}
		switch mutateRetryBlock(&block) {
		case workload.RetryBlockPersist:
			block.TargetRevision = targetRevision
			nextRetryBlocks[targetRevision] = block
			retryBlockChanged = true
		case workload.RetryBlockRemove:
			if found {
				delete(nextRetryBlocks, targetRevision)
				retryBlockChanged = true
			}
		}
	}
	if !changed && !retryBlockChanged {
		return nil
	}
	if s.applyErr != nil {
		return s.applyErr
	}
	s.statuses = next
	s.retryBlock = nextRetryBlocks
	s.writes++
	for _, callback := range callbacks {
		callback.fn(callback.before, callback.current)
	}
	return nil
}

func cloneTerminalRetryBlocks(in map[string]workload.RetryBlock) map[string]workload.RetryBlock {
	out := make(map[string]workload.RetryBlock, len(in))
	for revision, block := range in {
		copy := block
		if block.NextRetryAt != nil {
			next := *block.NextRetryAt
			copy.NextRetryAt = &next
		}
		if block.FirstFailureAt != nil {
			first := *block.FirstFailureAt
			copy.FirstFailureAt = &first
		}
		if block.LastFailureAt != nil {
			last := *block.LastFailureAt
			copy.LastFailureAt = &last
		}
		out[revision] = copy
	}
	return out
}

func cloneTerminalStatuses(in map[int32]workload.InstanceStatus) map[int32]workload.InstanceStatus {
	out := make(map[int32]workload.InstanceStatus, len(in))
	for index, status := range in {
		out[index] = cloneTerminalStatus(status)
	}
	return out
}

func cloneTerminalStatus(status workload.InstanceStatus) workload.InstanceStatus {
	out := status
	if status.Operation != nil {
		operation := *status.Operation
		operation.HintTargetNodes = append([]string(nil), status.Operation.HintTargetNodes...)
		if status.Operation.SurgeIndex != nil {
			surge := *status.Operation.SurgeIndex
			operation.SurgeIndex = &surge
		}
		out.Operation = &operation
	}
	out.NodesOccupied = append([]string(nil), status.NodesOccupied...)
	out.Conditions = append([]metav1.Condition(nil), status.Conditions...)
	return out
}
