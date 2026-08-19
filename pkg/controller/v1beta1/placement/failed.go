package placement

import "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"

// IsTerminallyFailed reports whether the (derived) ISVC's placement has failed
// in a way that needs human/spec action rather than an automatic re-place. The
// control plane treats this as a failed placement — distinct from a derived
// ISVC that was merely deleted (which is re-placed).
//
// Two failure surfaces are terminal:
//   - An OMENative Instance escalated to Phase=Failed (stuck-pod /
//     InstanceReadyTimeout backstop).
//   - The model lifecycle reached a terminal model-status (BlockedByFailedLoad
//     from a failed model load, or InvalidSpec from runtime-selection / spec
//     validation). These never carry an OMENative Instance failure, so checking
//     only Instance phases would report them as forever-Pending and re-fan-out
//     into the same failure on every poll.
func IsTerminallyFailed(isvc *v1beta1.InferenceService, statuses map[v1beta1.ComponentType]*v1beta1.InferenceReplicaStatus) bool {
	for _, st := range statuses {
		if st == nil {
			continue
		}
		for _, inst := range st.InstanceStatuses {
			if inst.Phase == v1beta1.OMENativeInstanceFailed {
				return true
			}
		}
	}
	switch isvc.Status.ModelStatus.TransitionStatus {
	case v1beta1.BlockedByFailedLoad, v1beta1.InvalidSpec:
		return true
	}
	return false
}
