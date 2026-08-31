package effective

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	knapis "knative.dev/pkg/apis"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimeselector"
)

func classifyLiveAvailability(live *LiveConfiguration, err error) liveAvailability {
	if err == nil && live != nil && live.Runtime.spec != nil {
		return liveAvailable
	}
	var notFound *runtimeselector.RuntimeNotFoundError
	if errors.As(err, &notFound) {
		return liveNotFound
	}
	var disabled *runtimeselector.RuntimeDisabledError
	if errors.As(err, &disabled) {
		return liveDisabled
	}
	return liveUnreadable
}

func deriveStatusFreshness(generation, observedGeneration int64) StatusFreshness {
	if observedGeneration == 0 {
		return StatusFreshnessUnknown
	}
	if observedGeneration == generation {
		return StatusFreshnessCurrent
	}
	if observedGeneration < generation {
		return StatusFreshnessStale
	}
	return StatusFreshnessInconsistent
}

func deriveSyncTokenState(annotation, status string) SyncTokenState {
	switch {
	case annotation == "" && status == "":
		return SyncTokenStateAbsent
	case annotation == "":
		return SyncTokenStateStatusOnly
	case annotation == status:
		return SyncTokenStateAcknowledged
	default:
		return SyncTokenStatePending
	}
}

func deriveRuntimeDrift[T ~[]knapis.Condition](conditions T) (RuntimeDriftState, RuntimeDriftReason) {
	found := false
	var drift knapis.Condition
	for _, condition := range conditions {
		if string(condition.Type) != constants.RuntimeDriftedConditionType {
			continue
		}
		if found {
			return RuntimeDriftStateMalformed, ""
		}
		found = true
		drift = condition
	}
	if !found {
		return RuntimeDriftStateNotReported, ""
	}
	reason := classifyRuntimeDriftReason(drift.Reason)
	switch drift.Status {
	case corev1.ConditionTrue:
		return RuntimeDriftStateReportedTrue, reason
	case corev1.ConditionFalse:
		return RuntimeDriftStateReportedFalse, reason
	case corev1.ConditionUnknown:
		return RuntimeDriftStateReportedUnknown, reason
	default:
		return RuntimeDriftStateMalformed, reason
	}
}

func classifyRuntimeDriftReason(reason string) RuntimeDriftReason {
	switch RuntimeDriftReason(reason) {
	case RuntimeDriftReasonRevisionMismatch,
		RuntimeDriftReasonRevisionMissing,
		RuntimeDriftReasonSourceRuntimeMissing,
		RuntimeDriftReasonRuntimeMismatch,
		RuntimeDriftReasonPinAdvanced:
		return RuntimeDriftReason(reason)
	default:
		return RuntimeDriftReasonOther
	}
}
