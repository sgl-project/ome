package workload

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeadlineExceededReason is the LastFailure.Reason stamped when an
// Instance is failed because its in-flight Operation.Deadline elapsed.
// Distinct from the kubelet waiting-state reasons the stuck-pod
// escalator records so operators can tell a broad-timeout backstop
// ("gang never became Ready") from a fast terminal-state escalation
// ("pod stuck in CrashLoopBackOff").
const DeadlineExceededReason = "DeadlineExceeded"

// operationDeadlinePassed reports whether s is in a transient phase with
// an in-flight Operation whose Deadline lies in the past. A zero
// Deadline (the metav1.Time zero value) is treated as "never expires" so
// an unset field can't trip the timeout — see plan.go's
// InstanceReadyTimeoutOrDefault for where the deadline window comes from.
func operationDeadlinePassed(s *InstanceStatus, now time.Time) bool {
	if s == nil || s.Operation == nil {
		return false
	}
	if s.Operation.Deadline.IsZero() {
		return false
	}
	if !isTransientPhase(s.Phase) {
		return false
	}
	return now.After(s.Operation.Deadline.Time)
}

// isTransientPhase reports whether the phase is an in-flight operation
// phase the InstanceReadyTimeout backstop bounds (Creating / Updating /
// Restarting / Migrating). Terminal phases (Ready / Failed / Deleting)
// and the empty zero value are not.
func isTransientPhase(p InstancePhase) bool {
	switch p {
	case InstancePhaseCreating,
		InstancePhaseUpdating,
		InstancePhaseRestarting,
		InstancePhaseMigrating:
		return true
	default:
		return false
	}
}

// PodAdmissionGated reports whether a pod is still held by a scheduling
// gate — queued for admission (e.g. by Kueue) and not yet allowed to run.
// A gated pod is waiting on an external admission authority, not stuck, so
// the InstanceReadyTimeout clock must not run while it is gated.
func PodAdmissionGated(pod *corev1.Pod) bool {
	return pod != nil && len(pod.Spec.SchedulingGates) > 0
}

// ReconcileGatedDeadlines keeps InstanceReadyTimeout measured from
// admission, not from operation start, so a workload an external admission
// authority (e.g. Kueue) legitimately holds queued is not failed by the
// deadline backstop (the escalation pass additionally skips gated
// Instances outright, covering the window before a gate-enter is parked).
//
// For each transient-phase Instance with an in-flight Operation:
//   - gated  -> PARK: zero the deadline (the "never expires" sentinel
//     the deadline predicate already honors). The clock pauses.
//   - un-gated with a parked (zero) deadline -> RESTART: now+timeout. The
//     clock starts from admission.
//   - un-gated with a live (non-zero) deadline -> untouched, so the
//     no-gate path behaves exactly as the per-op writer stamped it.
//
// Edge-triggered: it writes only on the gate-enter (park) and gate-exit
// (restart) transitions — a workload queued for hours does not churn its
// status. `gated` maps Instance index -> "any of its pods is gated".
func ReconcileGatedDeadlines(ctx context.Context, input ReconcileInput, instances []InstanceStatus, gated map[int32]bool, timeout time.Duration) error {
	now := input.Now()
	for i := range instances {
		s := &instances[i]
		if s.Operation == nil || !isTransientPhase(s.Phase) {
			continue
		}
		switch {
		case gated[s.Index] && !s.Operation.Deadline.IsZero():
			if err := setInstanceDeadline(ctx, input, s.Index, metav1.Time{}); err != nil {
				return fmt.Errorf("park deadline for gated instance %d: %w", s.Index, err)
			}
		case !gated[s.Index] && s.Operation.Deadline.IsZero():
			if err := setInstanceDeadline(ctx, input, s.Index, metav1.NewTime(now.Add(timeout))); err != nil {
				return fmt.Errorf("restart deadline for ungated instance %d: %w", s.Index, err)
			}
		}
	}
	return nil
}

// setInstanceDeadline writes Operation.Deadline via MutateInstance,
// preserving the rest of the Operation. No-op when the slot is gone, has
// no Operation, or already holds the target deadline.
func setInstanceDeadline(ctx context.Context, input ReconcileInput, idx int32, deadline metav1.Time) error {
	return input.MutateInstance(ctx, idx, func(s *InstanceStatus) bool {
		if s.Phase == "" || s.Operation == nil {
			return false
		}
		if s.Operation.Deadline.Time.Equal(deadline.Time) {
			return false
		}
		s.Operation.Deadline = deadline
		return true
	})
}

// deadlineFailureMessage formats the operator-facing reason passed to
// WarnInstanceFailed. Names the operation type + step that timed out so
// `kubectl describe` points at what was in flight.
func deadlineFailureMessage(op *InstanceOperation) string {
	if op == nil {
		return DeadlineExceededReason
	}
	return fmt.Sprintf("%s: %s/%s exceeded InstanceReadyTimeout", DeadlineExceededReason, op.Type, op.Step)
}

// deadlineFailedMutation builds the Phase=Failed stamp for an elapsed
// Operation deadline, recording the DeadlineExceeded diagnostic on
// LastFailure. Mirrors stuckPodFailedMutation's guard logic: a
// fresh-empty slot (Phase=="") from the writer's append path is a
// sentinel for a slot deleted out from under us (don't resurrect), an
// already-Failed slot is a no-op, and any other phase flips to Failed.
// A slot whose Operation is already gone is also a no-op: the attempt
// concluded (or was disposed) since the deadline was observed, so there
// is nothing in flight left to expire.
//
// The Operation is preserved — operators want to see WHAT was in flight
// when the deadline elapsed.
func deadlineFailedMutation(now time.Time, op *InstanceOperation) func(*InstanceStatus) bool {
	return func(s *InstanceStatus) bool {
		if s.Phase == "" {
			return false
		}
		if s.Phase == InstancePhaseFailed {
			return false
		}
		if s.Operation == nil {
			return false
		}
		s.Phase = InstancePhaseFailed
		s.LastFailure = &InstanceTermination{
			Reason:  DeadlineExceededReason,
			Message: deadlineFailureMessage(op),
			Time:    metav1.NewTime(now),
		}
		return true
	}
}
