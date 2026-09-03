package types

// RolloutHoldGate names the layer that most recently denied a
// per-Instance Update for a Component. Values mirror
// v1beta1.RolloutHoldGate byte-for-byte; the adapter converts by simple
// string cast, workload code never imports the CRD type.
type RolloutHoldGate string

const (
	RolloutHoldGateRatio      RolloutHoldGate = "Ratio"
	RolloutHoldGateSequential RolloutHoldGate = "Sequential"
	RolloutHoldGateBudget     RolloutHoldGate = "Budget"
	RolloutHoldGateRetryBlock RolloutHoldGate = "RetryBlock"
	RolloutHoldGateHeld       RolloutHoldGate = "Held"
)

// RolloutHold is the workload-side mirror of the most recent per-Instance
// Update denial observed for a Component this pass — the transient fact
// executeUpdatePass discovers via RecordRolloutHold, before the adapter's
// status writer decides whether it changes the persisted Since. No
// timestamp: persistence (including the churn-safe Since anchor) is the
// v1beta1 status writer's concern, not the dispatcher's.
type RolloutHold struct {
	// Gate names the layer that produced Reason.
	Gate RolloutHoldGate
	// Reason is the operator-facing denial string from the gate that
	// produced it.
	Reason string
	// Target is the ControllerRevision name the held Update was aimed at.
	Target string
}
