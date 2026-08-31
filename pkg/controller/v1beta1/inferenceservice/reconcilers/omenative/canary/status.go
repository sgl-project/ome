package canary

import "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"

// setPhase sets the RolloutPhase on a component's status, creating the status
// map and entry as needed. (Map values are structs, so it round-trips through a
// local copy.)
func setPhase(isvc *v1beta1.InferenceService, c v1beta1.ComponentType, phase v1beta1.RolloutPhase) {
	if isvc.Status.Components == nil {
		isvc.Status.Components = map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{}
	}
	cs := isvc.Status.Components[c]
	cs.RolloutPhase = phase
	isvc.Status.Components[c] = cs
}
