package snapshot

import (
	"sort"

	"sigs.k8s.io/ome/pkg/constants"
)

// ModelReadyLabelValue is the node-label value the model-agent's node-label
// reconciler writes when a per-node model copy is ready (the other values are
// Updating and Failed). The label key comes from
// constants.GetClusterBaseModelLabel / constants.GetBaseModelLabel — those
// helpers MUST be used to build keys, because long model names are
// SHA256-truncated.
const ModelReadyLabelValue = "Ready"

// modelReadinessLabelKey returns the node-label key that carries per-node
// readiness for the model.
func modelReadinessLabelKey(key ModelKey) string {
	if key.Kind == ModelKindBaseModel {
		return constants.GetBaseModelLabel(key.Namespace, key.Name)
	}
	return constants.GetClusterBaseModelLabel(key.Name)
}

// nodesReadyForModel scans node labels for the model's readiness label and
// returns the sorted list of nodes where the copy is Ready. Node labels are
// the source of truth here (not BaseModel.Status.NodesReady): they are what
// the model-readiness nodeSelector — and therefore the scheduler — actually
// enforces.
func nodesReadyForModel(nodes map[string]*Node, key ModelKey) []string {
	labelKey := modelReadinessLabelKey(key)
	var ready []string
	for name, node := range nodes {
		if node.Labels[labelKey] == ModelReadyLabelValue {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)
	return ready
}
