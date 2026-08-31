package ops

import (
	"context"
	"errors"
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// terminalInstanceIdentity is the lifecycle-owned portion of an Instance
// status. Runtime counters, conditions, node history, and operation timestamps
// may change without transferring ownership of the terminal action.
type terminalInstanceIdentity struct {
	index           int32
	incarnation     int64
	phase           workload.InstancePhase
	runningRevision string
	targetRevision  string
	activeOrdinal   int32
	operation       terminalOperationIdentity
}

type terminalOperationIdentity struct {
	present         bool
	id              string
	operationType   workload.InstanceOperationType
	step            string
	requestUUID     string
	targetRevision  string
	retryCount      int32
	reason          string
	fromNode        string
	hintTargetNodes []string
	surgeIndex      int32
	hasSurgeIndex   bool
}

type terminalIdentityGuardState struct {
	matched bool
	absent  bool
}

func captureTerminalInstanceIdentity(status *workload.InstanceStatus) terminalInstanceIdentity {
	identity := terminalInstanceIdentity{}
	if status == nil {
		return identity
	}
	identity.index = status.Index
	identity.incarnation = status.Incarnation
	identity.phase = status.Phase
	identity.runningRevision = status.RunningRevision
	identity.targetRevision = status.TargetRevision
	identity.activeOrdinal = status.ActiveOrdinal
	if status.Operation == nil {
		return identity
	}
	identity.operation = terminalOperationIdentity{
		present:         true,
		id:              status.Operation.ID,
		operationType:   status.Operation.Type,
		step:            status.Operation.Step,
		requestUUID:     status.Operation.RequestUUID,
		targetRevision:  status.Operation.TargetRevision,
		retryCount:      status.Operation.RetryCount,
		reason:          status.Operation.Reason,
		fromNode:        status.Operation.FromNode,
		hintTargetNodes: append([]string(nil), status.Operation.HintTargetNodes...),
	}
	if status.Operation.SurgeIndex != nil {
		identity.operation.hasSurgeIndex = true
		identity.operation.surgeIndex = *status.Operation.SurgeIndex
	}
	return identity
}

func (identity terminalInstanceIdentity) matches(status workload.InstanceStatus) bool {
	if status.Index != identity.index ||
		status.Incarnation != identity.incarnation ||
		status.Phase != identity.phase ||
		status.RunningRevision != identity.runningRevision ||
		status.TargetRevision != identity.targetRevision ||
		status.ActiveOrdinal != identity.activeOrdinal {
		return false
	}
	if status.Operation == nil {
		return !identity.operation.present
	}
	if !identity.operation.present ||
		status.Operation.ID != identity.operation.id ||
		status.Operation.Type != identity.operation.operationType ||
		status.Operation.Step != identity.operation.step ||
		status.Operation.RequestUUID != identity.operation.requestUUID ||
		status.Operation.TargetRevision != identity.operation.targetRevision ||
		status.Operation.RetryCount != identity.operation.retryCount ||
		status.Operation.Reason != identity.operation.reason ||
		status.Operation.FromNode != identity.operation.fromNode ||
		!slices.Equal(status.Operation.HintTargetNodes, identity.operation.hintTargetNodes) {
		return false
	}
	if status.Operation.SurgeIndex == nil {
		return !identity.operation.hasSurgeIndex
	}
	return identity.operation.hasSurgeIndex && *status.Operation.SurgeIndex == identity.operation.surgeIndex
}

func terminalIdentityGuard(
	input workload.ReconcileInput,
	index int32,
	expected *workload.InstanceStatus,
) (func(workload.InstanceMutationSnapshot) bool, *terminalIdentityGuardState) {
	identity := captureTerminalInstanceIdentity(expected)
	ownerUID := input.OwnerObject.GetUID()
	state := &terminalIdentityGuardState{}
	return func(snapshot workload.InstanceMutationSnapshot) bool {
		state.matched = false
		state.absent = false
		if snapshot.OwnerUID != ownerUID {
			return false
		}
		current, found := snapshot.Instances[index]
		if !found {
			state.absent = true
			return true
		}
		state.matched = expected != nil && identity.matches(current)
		return state.matched
	}, state
}

func applyTerminalInstanceMutations(ctx context.Context, input workload.ReconcileInput, mutations []workload.InstanceMutation) error {
	if input.ApplyInstanceMutationsWithRetryBlock == nil {
		return fmt.Errorf("terminal lifecycle requires the owner-aware atomic status adapter")
	}
	return input.ApplyInstanceMutationsWithRetryBlock(ctx, mutations, "", nil)
}

func validateTerminalMutationOwner(input workload.ReconcileInput) error {
	if input.OwnerObject == nil || input.OwnerObject.GetUID() == "" {
		return fmt.Errorf("terminal lifecycle requires a non-empty status owner UID")
	}
	if input.ApplyInstanceMutationsWithRetryBlock == nil {
		return fmt.Errorf("terminal lifecycle requires the owner-aware atomic status adapter")
	}
	return nil
}

// transitionTerminalOperationStep persists a terminal-cleanup ownership marker
// against the exact lifecycle identity that selected the work. Timestamp
// normalization and derived status changes do not invalidate that ownership.
func transitionTerminalOperationStep(
	ctx context.Context,
	input workload.ReconcileInput,
	expected *workload.InstanceStatus,
	step string,
	updateProgress bool,
) (bool, error) {
	if expected == nil || expected.Operation == nil {
		return false, nil
	}
	if err := validateTerminalMutationOwner(input); err != nil {
		return false, err
	}
	before := captureTerminalInstanceIdentity(expected)
	after := before
	after.operation.step = step
	ownerUID := input.OwnerObject.GetUID()
	confirmed := false
	committed := false
	mutate := func(status *workload.InstanceStatus) bool {
		if after.matches(*status) {
			confirmed = true
			return false
		}
		if !before.matches(*status) {
			return false
		}
		status.Operation.Step = step
		if updateProgress {
			status.Operation.LastProgressAt = metav1.NewTime(input.Now())
		}
		return true
	}
	mutation := workload.InstanceMutation{
		Index:  expected.Index,
		Mutate: mutate,
		BatchPrecondition: func(snapshot workload.InstanceMutationSnapshot) bool {
			confirmed = false
			if snapshot.OwnerUID != ownerUID {
				return false
			}
			current, found := snapshot.Instances[expected.Index]
			if !found {
				return false
			}
			if after.matches(current) {
				confirmed = true
				return true
			}
			return before.matches(current)
		},
		Postcondition: func(status *workload.InstanceStatus) bool {
			return status != nil && after.matches(*status)
		},
		OnCommit: func(_, _ *workload.InstanceStatus) {
			committed = true
		},
	}

	err := applyTerminalInstanceMutations(ctx, input, []workload.InstanceMutation{mutation})
	if errors.Is(err, workload.ErrStatusMutationPrecondition) || errors.Is(err, workload.ErrStatusOwnerGone) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return committed || confirmed, nil
}

// finalizeAndRemoveInstance applies the shared terminal ordering for lifecycle
// paths outside ordinary scale-down. Per-Instance resource finalization always
// requires the owner-aware atomic adapter. Deployment modes without those
// resources retain the RemoveInstance compatibility path.
func finalizeAndRemoveInstance(
	ctx context.Context,
	deps workload.Deps,
	input workload.ReconcileInput,
	index int32,
	expected *workload.InstanceStatus,
) (bool, error) {
	if expected != nil && expected.Index != index {
		return false, fmt.Errorf("terminal finalization identity index %d does not match target %d", expected.Index, index)
	}
	if input.ApplyInstanceMutationsWithRetryBlock == nil && input.FinalizeInstanceResources == nil {
		if input.RemoveInstance == nil {
			return false, fmt.Errorf("terminal lifecycle requires a status removal adapter")
		}
		if _, err := input.RemoveInstance(ctx, index); err != nil {
			return false, err
		}
		deps.ExpectationsCache().Forget(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, index)
		return true, nil
	}
	if err := validateTerminalMutationOwner(input); err != nil {
		return false, err
	}

	if input.FinalizeInstanceResources != nil {
		guard, state := terminalIdentityGuard(input, index, expected)
		preflight := workload.InstanceMutation{
			Index:             index,
			Mutate:            func(*workload.InstanceStatus) bool { return false },
			BatchPrecondition: guard,
		}
		err := applyTerminalInstanceMutations(ctx, input, []workload.InstanceMutation{preflight})
		if errors.Is(err, workload.ErrStatusMutationPrecondition) || errors.Is(err, workload.ErrStatusOwnerGone) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if state.absent && expected != nil {
			deps.ExpectationsCache().Forget(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, index)
			return true, nil
		}
		if !state.absent && !state.matched {
			return false, nil
		}
		complete, err := input.FinalizeInstanceResources(ctx, index)
		if err != nil {
			return false, err
		}
		if !complete {
			return false, nil
		}
	}

	guard, state := terminalIdentityGuard(input, index, expected)
	committed := false
	mutation := workload.InstanceMutation{
		Index:             index,
		Remove:            true,
		BatchPrecondition: guard,
		OnCommit: func(_, _ *workload.InstanceStatus) {
			committed = true
			deps.ExpectationsCache().Forget(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, index)
		},
	}

	err := applyTerminalInstanceMutations(ctx, input, []workload.InstanceMutation{mutation})
	if errors.Is(err, workload.ErrStatusMutationPrecondition) || errors.Is(err, workload.ErrStatusOwnerGone) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if state.absent {
		deps.ExpectationsCache().Forget(input.Key.Namespace, input.Key.OwnerName, input.Key.Component, index)
	}
	return committed || state.absent, nil
}
