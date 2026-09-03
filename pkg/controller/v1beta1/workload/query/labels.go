package query

import corev1 "k8s.io/api/core/v1"

// Label keys and values OMENative stamps onto every managed Pod /
// Service / ControllerRevision / ConfigMap. Shared here so the
// dispatch code (top-level omenative), the ops/ subpackage, and the
// pod / EndpointSlice watch handlers all read the same constants.
const (
	LabelManagedBy           = "ome.io/managed-by"
	LabelInstanceIdx         = "ome.io/instance-index"
	LabelInstanceIncarnation = "ome.io/instance-incarnation"
	LabelRunner              = "ome.io/runner"
	LabelRevisionHash        = "ome.io/revision-hash"
	// LabelPairingProtocol carries a revision's P/D pairing protocol: a label
	// on rendered pods and per-revision routing Services (metadata, not
	// selector — informational), and an annotation on ControllerRevisions
	// (the authoritative revision→protocol record, stamped at create from the
	// hashed payload). Absent or empty means the revision pairs with anything.
	LabelPairingProtocol = "ome.io/pairing-protocol"
	// LabelPodOrdinal stamps the pod-naming ordinal (0 or 1 for single-pod
	// SurgeThenDrain alternation; 0..Size-1 for multi-pod gang members).
	// SurgeThenDrain reads it to partition observed pods into "current"
	// (ordinal == ActiveOrdinal) and "surge" (ordinal == 1-ActiveOrdinal)
	// without having to parse pod names (unsafe when isvc names contain
	// hyphens). Pre-feature pods lack this label and are treated as
	// ordinal=0 by the parser — matches their actual name suffix.
	LabelPodOrdinal = "ome.io/pod-ordinal"
	// LabelPodGroup is the scheduler-plugins coscheduling pod-group label
	// (`scheduling.x-k8s.io/pod-group`). Multi-pod Instances stamp it on
	// every pod so the gang-aware scheduler co-schedules the whole gang —
	// without this, the coscheduler treats each pod independently and
	// partial placement is possible.
	//
	// Matches the upstream constant `schedulingv1alpha1.PodGroupLabel` at
	// the byte level (we don't import the type here to keep the query
	// package dep-free of the optional scheduler-plugins API).
	// Single-pod Instances do NOT carry this label — no PodGroup, no gang.
	LabelPodGroup = "scheduling.x-k8s.io/pod-group"
	// AnnotationTopologyKey carries the resolved Component topology key on
	// PodGroups so topology-aware gang schedulers can place all members in one
	// domain. Fresh gangs use ComponentPlan.TopologyKey. Existing gangs may
	// retain the exact key already present in OME-generated worker affinity so
	// immutable live pods and their PodGroup cannot drift apart during rollout.
	AnnotationTopologyKey = "ome.io/topology-key"
	ManagedByOMENative    = "OMENative"
)

// RevisionHashFromControllerRevisionName extracts the trailing
// hash segment from a ControllerRevision name of the form
// `<isvc>-<component>-<hash>`. Returns "" when the name doesn't
// match the expected shape. Used by the pod renderer to stamp
// ome.io/revision-hash from a target *appsv1.ControllerRevision and
// by the coordination layer to recover the hash for env-var
// injection.
func RevisionHashFromControllerRevisionName(name string) string {
	for i := len(name) - 1; i > 0; i-- {
		if name[i] == '-' {
			return name[i+1:]
		}
	}
	return ""
}

// ServingConditionType is the controller-owned readiness gate
// OMENative flips during drain, update, and migration. Render appends
// it to every Pod's ReadinessGates; the op layer toggles the
// corresponding PodCondition to gate EndpointSlice readiness.
const ServingConditionType corev1.PodConditionType = "ome.io/serving"
