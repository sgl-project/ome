package ops

import (
	"context"
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// updateStep distinguishes which mode wrote the current Operation. Without
// it, a strategy change mid-rollout (InPlaceIfPossible → RecreatePod)
// would let recreate's idempotency check match the in-place state, skip
// the Incarnation bump, classify every pod as fresh, and never recreate.
const (
	updateStepInPlace = "InPlace"
	updateStepDrain   = "Drain"
	// updateStepSurge is the SurgeThenDrain entry step: surge pod
	// being created at the other ordinal slot. Transitions to
	// updateStepSurgeDrain once the surge pod is Ready and the old
	// pod's serving gate is being flipped.
	updateStepSurge = workload.UpdateStepSurge
	// updateStepSurgeDrain is the SurgeThenDrain drain phase. Distinct
	// from updateStepDrain so surge-budget accounting
	// (workload.CurrentSurgeInFlight, coordination
	// GateContext.CheckSurge) can recognize in-flight surge operations
	// by step name alone (the surge contributes an extra pod alive —
	// the unavailability gate doesn't apply; the surge gate does).
	updateStepSurgeDrain = workload.UpdateStepSurgeDrain
	// updateStepSurgeDrainSettle is entered after EndpointSlice confirms
	// the old pod is ineligible for new traffic. The controller
	// keeps the pod alive in this step so persistent load-balancer
	// connections can age out while both the router and its workers are
	// still available, then deletes it after the configured grace period.
	updateStepSurgeDrainSettle = "SurgeDrainSettle"
)

// UpdateStepInPlace is the exported alias for the in-place step name.
// Used by the stuck-operation deadline machinery to identify in-flight
// in-place updates (Type=Update, Step=InPlace) that need to fall back
// to recreate when their deadline elapses.
const UpdateStepInPlace = updateStepInPlace

// UpdateStepDrain is the exported alias for the recreate-drain step name.
const UpdateStepDrain = updateStepDrain

// UpdateStepSurge is the exported alias for the surge entry step name.
const UpdateStepSurge = updateStepSurge

// UpdateStepSurgeDrain is the exported alias for the surge-drain step name.
const UpdateStepSurgeDrain = updateStepSurgeDrain

// UpdateStepSurgeDrainSettle is the exported alias for the post-unroute
// connection-settle step.
const UpdateStepSurgeDrainSettle = updateStepSurgeDrainSettle

func isSurgeUpdateStep(step string) bool {
	return step == updateStepSurge ||
		step == updateStepSurgeDrain ||
		step == updateStepSurgeDrainSettle
}

// flipRetryBlockOnAttemptStart marks an existing due-Backoff RetryBlock
// RetryInProgress once a fresh Update attempt actually starts — i.e.
// after every dispatcher budget/coordination gate admitted it (flipping
// at detect time would strand RetryInProgress when a budget denies the
// start). No block, or any non-Backoff state, records nothing: a fresh
// start with no prior failure needs no block. Idempotent across passes.
func flipRetryBlockOnAttemptStart(ctx context.Context, input workload.ReconcileInput, targetRev string) error {
	if input.MutateRetryBlock == nil {
		return nil
	}
	if err := input.MutateRetryBlock(ctx, targetRev, markRetryBlockAttemptStarted); err != nil {
		return fmt.Errorf("mark retry in progress (rev=%s): %w", targetRev, err)
	}
	return nil
}

func markRetryBlockAttemptStarted(rb *workload.RetryBlock) workload.RetryBlockDisposition {
	if rb.State != workload.RetryBlockBackoff {
		return workload.RetryBlockUnchanged
	}
	rb.State = workload.RetryBlockRetryInProgress
	return workload.RetryBlockPersist
}

// patchInstanceStatusUpdating idempotently stamps Phase=Updating with the
// Update operation in Step=InPlace at targetRev. Skip-write requires
// Step==InPlace so a previous recreate-step write doesn't short-circuit.
func patchInstanceStatusUpdating(ctx context.Context, input workload.ReconcileInput, idx int32, targetRev string, timeout time.Duration) error {
	now := metav1.NewTime(input.Now())
	err := input.MutateInstance(ctx, idx, func(s *workload.InstanceStatus) bool {
		if s.Phase == workload.InstancePhaseUpdating &&
			s.Operation != nil && s.Operation.Type == workload.InstanceOperationUpdate &&
			s.Operation.Step == updateStepInPlace &&
			s.TargetRevision == targetRev {
			return false
		}
		s.Phase = workload.InstancePhaseUpdating
		s.TargetRevision = targetRev
		s.Operation = &workload.InstanceOperation{
			ID:             fmt.Sprintf("update-%d-%d", idx, now.Unix()),
			Type:           workload.InstanceOperationUpdate,
			Step:           updateStepInPlace,
			TargetRevision: targetRev,
			StartedAt:      now,
			LastProgressAt: now,
			Deadline:       metav1.NewTime(now.Add(timeout)),
		}
		return true
	})
	if err != nil {
		return err
	}
	return flipRetryBlockOnAttemptStart(ctx, input, targetRev)
}

// patchInstanceStatusRecreatingForUpdate is the recreate entry point.
// Bumps Incarnation (like Restart) and stamps Phase=Updating with
// Step=Drain. reason is the "revision <from> → <to>" cause string recorded
// on Operation.Reason so a revision-roll recreate is distinguishable in
// status from a pod-failure Restart. Returns the post-write Incarnation.
// The idempotency guard requires Step==Drain so a prior in-place pass at
// the same target doesn't block the bump.
func patchInstanceStatusRecreatingForUpdate(ctx context.Context, input workload.ReconcileInput, idx int32, targetRev, reason string, timeout time.Duration) (int64, error) {
	var observedIncarnation int64
	err := input.MutateInstance(ctx, idx, func(s *workload.InstanceStatus) bool {
		if s.Phase == workload.InstancePhaseUpdating &&
			s.Operation != nil && s.Operation.Type == workload.InstanceOperationUpdate &&
			s.Operation.Step == updateStepDrain &&
			s.TargetRevision == targetRev && s.Incarnation > 0 {
			observedIncarnation = s.Incarnation
			return false
		}
		if s.Incarnation == 0 {
			s.Incarnation = 1
		}
		s.Incarnation++
		observedIncarnation = s.Incarnation
		s.Phase = workload.InstancePhaseUpdating
		s.TargetRevision = targetRev
		now := metav1.NewTime(input.Now())
		s.Operation = &workload.InstanceOperation{
			ID:             fmt.Sprintf("update-%d-%d", idx, now.Unix()),
			Type:           workload.InstanceOperationUpdate,
			Step:           updateStepDrain,
			TargetRevision: targetRev,
			Reason:         reason,
			StartedAt:      now,
			LastProgressAt: now,
			Deadline:       metav1.NewTime(now.Add(timeout)),
		}
		return true
	})
	if err != nil {
		return observedIncarnation, err
	}
	return observedIncarnation, flipRetryBlockOnAttemptStart(ctx, input, targetRev)
}

// (patchInstanceStatusReadyOnRevision lives in create.go — the in-place,
// recreate, and surge promote paths share it. surge variant that also
// advances ActiveOrdinal is patchInstanceStatusReadyOnRevisionWithOrdinal
// below.)

// patchInstanceStatusSurgingForUpdate is the SurgeThenDrain entry
// point. Stamps Phase=Updating + Op{Step=Surge, TargetRevision} without
// bumping Incarnation (different pod NAME — ordinal slot — keeps old
// and new distinct without reusing identity). ActiveOrdinal is not yet
// advanced; it stays on the old slot until promote completes. The
// idempotency guard accepts every surge lifecycle step so the helper
// does not regress an in-flight drain or settle back to Surge when
// surgeUpdate re-runs the entry stamp on a subsequent pass; it does
// require Type==Update so a prior recreate / in-place write at the
// same target doesn't short-circuit (those writes use the same
// TargetRevision but a different Step namespace).
func patchInstanceStatusSurgingForUpdate(ctx context.Context, input workload.ReconcileInput, idx int32, targetRev string, timeout time.Duration) error {
	now := metav1.NewTime(input.Now())
	err := input.MutateInstance(ctx, idx, func(s *workload.InstanceStatus) bool {
		// Already surging toward this target — no-op. The Phase check
		// admits BOTH an in-flight Updating instance AND one the stuck-pod
		// escalator has since flipped to Failed. Resurrecting a Failed
		// instance back to Updating (with a fresh Operation + deadline)
		// re-arms the escalator into a Failed<->Updating write storm when
		// the target revision itself is bad and there is no corrective edit
		// to drive to — the surge pod never goes Ready, so every pass
		// re-stamps Updating and the escalator re-stamps Failed. A real
		// corrective edit changes TargetRevision, so the guard falls
		// through and the re-surge proceeds.
		if s.Operation != nil && s.Operation.Type == workload.InstanceOperationUpdate &&
			isSurgeUpdateStep(s.Operation.Step) &&
			s.TargetRevision == targetRev &&
			(s.Phase == workload.InstancePhaseUpdating || s.Phase == workload.InstancePhaseFailed) {
			return false
		}
		s.Phase = workload.InstancePhaseUpdating
		s.TargetRevision = targetRev
		s.Operation = &workload.InstanceOperation{
			ID:             fmt.Sprintf("update-%d-%d", idx, now.Unix()),
			Type:           workload.InstanceOperationUpdate,
			Step:           updateStepSurge,
			TargetRevision: targetRev,
			StartedAt:      now,
			LastProgressAt: now,
			Deadline:       metav1.NewTime(now.Add(timeout)),
		}
		return true
	})
	if err != nil {
		return err
	}
	return flipRetryBlockOnAttemptStart(ctx, input, targetRev)
}

// patchInstanceStatusSurgeStepDrain transitions the surge operation
// from Step=Surge to Step=SurgeDrain once the surge pod is Ready and
// we're about to flip serving gates. Distinct from updateStepDrain
// (which recreate uses) so surge-budget accounting can identify
// in-flight surges by step name. Updates LastProgressAt for
// the stuck-operation timeout machinery. Idempotent — no-op if
// already on SurgeDrain.
func patchInstanceStatusSurgeStepDrain(ctx context.Context, input workload.ReconcileInput, idx int32) error {
	now := metav1.NewTime(input.Now())
	return input.MutateInstance(ctx, idx, func(s *workload.InstanceStatus) bool {
		if s.Operation == nil ||
			s.Operation.Type != workload.InstanceOperationUpdate {
			return false
		}
		if s.Operation.Step == updateStepSurgeDrain || s.Operation.Step == updateStepSurgeDrainSettle {
			return false
		}
		s.Operation.Step = updateStepSurgeDrain
		s.Operation.LastProgressAt = now
		return true
	})
}

// patchInstanceStatusSurgeStepSettle records the instant EndpointSlice
// convergence was first observed. LastProgressAt is the durable start of the
// connection-settle interval, so controller restarts do not shorten the wait.
func patchInstanceStatusSurgeStepSettle(ctx context.Context, input workload.ReconcileInput, idx int32) error {
	now := metav1.NewTime(input.Now())
	return input.MutateInstance(ctx, idx, func(s *workload.InstanceStatus) bool {
		if s.Operation == nil || s.Operation.Type != workload.InstanceOperationUpdate {
			return false
		}
		if s.Operation.Step == updateStepSurgeDrainSettle {
			return false
		}
		s.Operation.Step = updateStepSurgeDrainSettle
		s.Operation.LastProgressAt = now
		return true
	})
}

// patchInstanceStatusReadyOnRevisionWithOrdinal is the surge-promote
// terminator: clears Operation, advances ActiveOrdinal to the new slot,
// stamps RunningRevision=rev and Phase=Ready. Distinct from
// patchInstanceStatusReadyOnRevision because surge needs to ALSO bump
// ActiveOrdinal — the old pod is gone, the surge pod is the new
// canonical pod, and future restart / scale-up paths must address that
// slot. Success at rev also prunes rev's RetryBlock.
func patchInstanceStatusReadyOnRevisionWithOrdinal(ctx context.Context, input workload.ReconcileInput, idx int32, rev string, newOrdinal int32) error {
	err := input.MutateInstance(ctx, idx, func(s *workload.InstanceStatus) bool {
		if s.Phase == workload.InstancePhaseReady &&
			s.Operation == nil &&
			s.RunningRevision == rev &&
			s.TargetRevision == "" &&
			s.ActiveOrdinal == newOrdinal {
			return false
		}
		markReadyTransition(s, input.Now())
		s.RunningRevision = rev
		s.TargetRevision = ""
		s.Operation = nil
		s.ActiveOrdinal = newOrdinal
		return true
	})
	if err != nil {
		return err
	}
	return pruneRetryBlockOnPromote(ctx, input, rev)
}

// patchInstanceStatusGangSurging stamps the SOURCE Instance of a gang
// surge: Phase=Updating, Op{Update, Step=Surge, SurgeIndex=k}. Step=Surge
// makes CurrentSurgeInFlight / the coordination CheckSurge gate count it
// against MaxSurge; the SurgeIndex pointer distinguishes a gang surge
// (new index) from a single-pod surge (ActiveOrdinal toggle). Idempotent.
func patchInstanceStatusGangSurging(ctx context.Context, input workload.ReconcileInput, idx, surgeIdx int32, targetRev string, timeout time.Duration) error {
	now := metav1.NewTime(input.Now())
	err := input.MutateInstance(ctx, idx, func(s *workload.InstanceStatus) bool {
		return stampGangSurgeSource(s, idx, surgeIdx, targetRev, timeout, now)
	})
	if err != nil {
		return err
	}
	return flipRetryBlockOnAttemptStart(ctx, input, targetRev)
}

// patchInstanceStatusGangSurgeTarget stamps the TARGET (surge) index of a
// gang surge: Phase=Creating, Op{Update, Step=GangSurgeTarget}. Same
// attempt patchInstanceStatusGangSurging already flipped the RetryBlock
// for — no flip here (once per attempt, source side). The
// marker pins the index in instancePlanIndices so EnsurePodGroups creates
// its PodGroup and scale-down won't delete it. Creates the InstanceStatus
// if absent. Idempotent.
func patchInstanceStatusGangSurgeTarget(ctx context.Context, input workload.ReconcileInput, surgeIdx int32, targetRev string, timeout time.Duration) error {
	now := metav1.NewTime(input.Now())
	return input.MutateInstance(ctx, surgeIdx, func(s *workload.InstanceStatus) bool {
		return stampGangSurgeTarget(s, surgeIdx, targetRev, timeout, now)
	})
}

func stampGangSurgeSource(s *workload.InstanceStatus, idx, surgeIdx int32, targetRev string, timeout time.Duration, now metav1.Time) bool {
	if s.Phase == workload.InstancePhaseUpdating && s.Operation != nil &&
		s.Operation.Type == workload.InstanceOperationUpdate &&
		s.Operation.Step == updateStepSurge && s.Operation.SurgeIndex != nil &&
		*s.Operation.SurgeIndex == surgeIdx && s.TargetRevision == targetRev {
		return false
	}
	k := surgeIdx
	s.Phase = workload.InstancePhaseUpdating
	s.TargetRevision = targetRev
	s.Operation = &workload.InstanceOperation{
		ID:             fmt.Sprintf("gangsurge-%d-%d", idx, now.Unix()),
		Type:           workload.InstanceOperationUpdate,
		Step:           updateStepSurge,
		SurgeIndex:     &k,
		TargetRevision: targetRev,
		StartedAt:      now,
		LastProgressAt: now,
		Deadline:       metav1.NewTime(now.Add(timeout)),
	}
	return true
}

func stampGangSurgeTarget(s *workload.InstanceStatus, surgeIdx int32, targetRev string, timeout time.Duration, now metav1.Time) bool {
	if gangSurgeTargetClaimMatches(s, targetRev) || !emptyGangSurgeTargetSlot(s) {
		return false
	}
	s.Incarnation = 1
	s.Phase = workload.InstancePhaseCreating
	s.TargetRevision = targetRev
	s.Operation = &workload.InstanceOperation{
		ID:             fmt.Sprintf("gangsurgetarget-%d-%d", surgeIdx, now.Unix()),
		Type:           workload.InstanceOperationUpdate,
		Step:           workload.UpdateStepGangSurgeTarget,
		TargetRevision: targetRev,
		StartedAt:      now,
		LastProgressAt: now,
		Deadline:       metav1.NewTime(now.Add(timeout)),
	}
	return true
}

func startGangSurge(
	ctx context.Context,
	input workload.ReconcileInput,
	source *workload.InstanceStatus,
	surgeIdx int32,
	targetRev string,
	timeout time.Duration,
) (bool, error) {
	if source == nil {
		return false, nil
	}
	if input.ApplyInstanceMutationsWithRetryBlock == nil {
		if err := patchInstanceStatusGangSurging(ctx, input, source.Index, surgeIdx, targetRev, timeout); err != nil {
			return false, err
		}
		if err := patchInstanceStatusGangSurgeTarget(ctx, input, surgeIdx, targetRev, timeout); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := validateTerminalMutationOwner(input); err != nil {
		return false, err
	}

	now := metav1.NewTime(input.Now())
	sourceIdentity := captureTerminalInstanceIdentity(source)
	desiredSource := cloneTerminalStatusValue(*source)
	stampGangSurgeSource(&desiredSource, source.Index, surgeIdx, targetRev, timeout, now)
	desiredSourceIdentity := captureTerminalInstanceIdentity(&desiredSource)
	desiredTarget := workload.InstanceStatus{Index: surgeIdx}
	stampGangSurgeTarget(&desiredTarget, surgeIdx, targetRev, timeout, now)
	desiredTargetIdentity := captureTerminalInstanceIdentity(&desiredTarget)
	ownerUID := input.OwnerObject.GetUID()
	committed := false
	sourceMutation := workload.InstanceMutation{
		Index: source.Index,
		Mutate: func(status *workload.InstanceStatus) bool {
			return stampGangSurgeSource(status, source.Index, surgeIdx, targetRev, timeout, now)
		},
		BatchPrecondition: func(snapshot workload.InstanceMutationSnapshot) bool {
			if snapshot.OwnerUID != ownerUID {
				return false
			}
			currentSource, found := snapshot.Instances[source.Index]
			if !found || !sourceIdentity.matches(currentSource) {
				return false
			}
			currentTarget, found := snapshot.Instances[surgeIdx]
			return !found || emptyGangSurgeTargetSlot(&currentTarget)
		},
		Postcondition: func(status *workload.InstanceStatus) bool {
			return status != nil && desiredSourceIdentity.matches(*status)
		},
		OnCommit: func(_, _ *workload.InstanceStatus) { committed = true },
	}
	targetMutation := workload.InstanceMutation{
		Index: surgeIdx,
		Mutate: func(status *workload.InstanceStatus) bool {
			return stampGangSurgeTarget(status, surgeIdx, targetRev, timeout, now)
		},
		Postcondition: func(status *workload.InstanceStatus) bool {
			return status != nil && desiredTargetIdentity.matches(*status)
		},
	}
	err := input.ApplyInstanceMutationsWithRetryBlock(ctx,
		[]workload.InstanceMutation{sourceMutation, targetMutation}, targetRev, markRetryBlockAttemptStarted)
	if errors.Is(err, workload.ErrStatusMutationPrecondition) || errors.Is(err, workload.ErrStatusOwnerGone) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return committed, nil
}

type gangSurgeTargetMarkerResolution uint8

const (
	gangSurgeTargetMarkerUnclaimed gangSurgeTargetMarkerResolution = iota
	gangSurgeTargetMarkerActive
	gangSurgeTargetMarkerRestored
	gangSurgeTargetMarkerCleanup
)

// restoreInstanceStatusGangSurgeTarget closes the resumable gap between the
// source's committed surge claim and its target marker. The result identifies
// the authoritative marker state; Restored requires a fresh reconcile before
// external effects. The strong path binds target creation to the exact source
// operation on every conflict retry.
func restoreInstanceStatusGangSurgeTarget(
	ctx context.Context,
	input workload.ReconcileInput,
	source *workload.InstanceStatus,
	surgeIdx int32,
	targetRev string,
	timeout time.Duration,
) (gangSurgeTargetMarkerResolution, error) {
	if source == nil || source.Operation == nil {
		return gangSurgeTargetMarkerUnclaimed, nil
	}
	if input.ApplyInstanceMutationsWithRetryBlock == nil {
		if input.FinalizeInstanceResources != nil {
			return gangSurgeTargetMarkerUnclaimed, fmt.Errorf("gang surge target recovery requires the owner-aware atomic status adapter")
		}
		if err := patchInstanceStatusGangSurgeTarget(ctx, input, surgeIdx, targetRev, timeout); err != nil {
			return gangSurgeTargetMarkerUnclaimed, err
		}
		return gangSurgeTargetMarkerRestored, nil
	}
	if err := validateTerminalMutationOwner(input); err != nil {
		return gangSurgeTargetMarkerUnclaimed, err
	}

	now := metav1.NewTime(input.Now())
	desired := workload.InstanceStatus{
		Index:          surgeIdx,
		Incarnation:    1,
		Phase:          workload.InstancePhaseCreating,
		TargetRevision: targetRev,
		Operation: &workload.InstanceOperation{
			ID:             fmt.Sprintf("gangsurgetarget-%d-%d", surgeIdx, now.Unix()),
			Type:           workload.InstanceOperationUpdate,
			Step:           workload.UpdateStepGangSurgeTarget,
			TargetRevision: targetRev,
			StartedAt:      now,
			LastProgressAt: now,
			Deadline:       metav1.NewTime(now.Add(timeout)),
		},
	}
	sourceIdentity := captureTerminalInstanceIdentity(source)
	ownerUID := input.OwnerObject.GetUID()
	resolution := gangSurgeTargetMarkerUnclaimed
	mutation := workload.InstanceMutation{
		Index: surgeIdx,
		Mutate: func(status *workload.InstanceStatus) bool {
			if gangSurgeActiveTargetClaimMatches(status, targetRev) {
				resolution = gangSurgeTargetMarkerActive
				return false
			}
			if gangSurgeCleanupTargetClaimMatches(status, targetRev) {
				resolution = gangSurgeTargetMarkerCleanup
				return false
			}
			if !emptyGangSurgeTargetSlot(status) {
				return false
			}
			*status = cloneTerminalStatusValue(desired)
			return true
		},
		BatchPrecondition: func(snapshot workload.InstanceMutationSnapshot) bool {
			resolution = gangSurgeTargetMarkerUnclaimed
			if snapshot.OwnerUID != ownerUID {
				return false
			}
			currentSource, found := snapshot.Instances[source.Index]
			if !found || !sourceIdentity.matches(currentSource) {
				return false
			}
			currentTarget, found := snapshot.Instances[surgeIdx]
			if !found {
				return true
			}
			if gangSurgeActiveTargetClaimMatches(&currentTarget, targetRev) {
				resolution = gangSurgeTargetMarkerActive
				return true
			}
			if gangSurgeCleanupTargetClaimMatches(&currentTarget, targetRev) {
				resolution = gangSurgeTargetMarkerCleanup
				return true
			}
			return false
		},
		Postcondition: func(status *workload.InstanceStatus) bool {
			return sameRestoredGangSurgeTarget(status, &desired)
		},
		OnCommit: func(_, _ *workload.InstanceStatus) {
			resolution = gangSurgeTargetMarkerRestored
		},
	}
	err := applyTerminalInstanceMutations(ctx, input, []workload.InstanceMutation{mutation})
	if errors.Is(err, workload.ErrStatusMutationPrecondition) || errors.Is(err, workload.ErrStatusOwnerGone) {
		return gangSurgeTargetMarkerUnclaimed, nil
	}
	if err != nil {
		return gangSurgeTargetMarkerUnclaimed, err
	}
	return resolution, nil
}

func emptyGangSurgeTargetSlot(status *workload.InstanceStatus) bool {
	return status != nil && status.Incarnation == 0 && status.Phase == workload.InstancePhaseEmpty &&
		status.RunningRevision == "" && status.TargetRevision == "" && status.Operation == nil &&
		status.PodCount == 0 && status.ServingPodCount == 0 && status.AvailablePodCount == 0 &&
		!status.Admitted && len(status.Conditions) == 0 && status.LastFailure == nil
}

func sameRestoredGangSurgeTarget(current, desired *workload.InstanceStatus) bool {
	return current != nil && desired != nil && gangSurgeTargetMatches(current, desired.TargetRevision) &&
		current.Index == desired.Index && current.Incarnation == desired.Incarnation &&
		current.Operation.ID == desired.Operation.ID
}

func cloneTerminalStatusValue(status workload.InstanceStatus) workload.InstanceStatus {
	copy := status
	if status.Operation != nil {
		operation := *status.Operation
		operation.HintTargetNodes = append([]string(nil), status.Operation.HintTargetNodes...)
		if status.Operation.SurgeIndex != nil {
			surgeIndex := *status.Operation.SurgeIndex
			operation.SurgeIndex = &surgeIndex
		}
		copy.Operation = &operation
	}
	copy.NodesOccupied = append([]string(nil), status.NodesOccupied...)
	copy.Conditions = append([]metav1.Condition(nil), status.Conditions...)
	return copy
}
