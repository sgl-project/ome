package types

// ConditionType / ConditionReason are workload-internal identifiers
// stamped on metav1.Condition entries. Values match the legacy
// omenative/status strings byte-for-byte so operator dashboards keep
// matching.
type ConditionType string

func (t ConditionType) String() string { return string(t) }

type ConditionReason string

func (r ConditionReason) String() string { return string(r) }

const (
	// ConditionGangSchedulingUnavailable is True when the Component
	// has at least one multi-pod Instance but the scheduler-plugins
	// PodGroup CRD is missing. The reconciler still creates pods —
	// gang scheduling is a soft requirement so workloads proceed
	// without blocking — but partial-gang placement is possible and
	// the runtime may hang.
	ConditionGangSchedulingUnavailable ConditionType = "GangSchedulingUnavailable"
)

const (
	// ReasonPodGroupCRDNotInstalled stamps the
	// GangSchedulingUnavailable condition when the
	// scheduler-plugins PodGroup CRD is missing.
	ReasonPodGroupCRDNotInstalled ConditionReason = "PodGroupCRDNotInstalled"
	// ReasonGangSchedulingAvailable stamps Status=False (CRD present,
	// or Component is single-pod).
	ReasonGangSchedulingAvailable ConditionReason = "GangSchedulingAvailable"
)
