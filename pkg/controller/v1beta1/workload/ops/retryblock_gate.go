package ops

import (
	"time"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// evaluateRetryBlockGate is the single deny/allow evaluation of a
// persisted RetryBlock, shared by the update trigger gate and the
// create pass. One implementation on purpose: a deadline-disposed
// attempt leaves its instance Failed-with-no-Operation — a fresh start
// — and an ungated create would re-materialize pods at the same bad
// revision forever, bypassing the block the disposition recorded.
//
// attemptInFlight reports whether an authorized attempt at the block's
// revision is currently in flight. It distinguishes a live
// RetryInProgress authorization (deny — exactly one attempt at a time)
// from a leaked one (superseded surge, scale-down, crash), which is
// treated as due so the revision is not silently denied forever; the
// attempt stamp re-confirms the state.
//
// Returns denied plus retryAfter: >0 only for a not-yet-due Backoff
// block (re-evaluate then). Held has no time bound. A nil NextRetryAt
// is immediately due.
//
// A due Backoff allows WITHOUT flipping state — the RetryInProgress
// flip belongs to attempt-stamp time (flipRetryBlockOnAttemptStart),
// after the dispatcher's budget/coordination gates admit the start.
// Flipping here would strand RetryInProgress when a budget denies the
// pass. Unrecognized states fall through un-gated (fail-open
// forward-compat).
func evaluateRetryBlockGate(b *workload.RetryBlock, now time.Time, attemptInFlight bool) (denied bool, retryAfter time.Duration) {
	if b == nil {
		return false, 0
	}
	switch b.State {
	case workload.RetryBlockHeld:
		return true, 0
	case workload.RetryBlockRetryInProgress:
		if attemptInFlight {
			return true, 0
		}
	case workload.RetryBlockBackoff:
		if b.NextRetryAt != nil && now.Before(b.NextRetryAt.Time) {
			return true, b.NextRetryAt.Time.Sub(now)
		}
	}
	return false, 0
}

// anyInFlightCreateAttempt reports whether any Instance carries an in-flight
// Create attempt. TargetRevision is deliberately ignored because an empty
// value is a supported persisted state and the gate must remain conservative.
func anyInFlightCreateAttempt(statuses []workload.InstanceStatus) bool {
	for i := range statuses {
		op := statuses[i].Operation
		if op != nil && op.Type == workload.InstanceOperationCreate &&
			statuses[i].Phase == workload.InstancePhaseCreating {
			return true
		}
	}
	return false
}
