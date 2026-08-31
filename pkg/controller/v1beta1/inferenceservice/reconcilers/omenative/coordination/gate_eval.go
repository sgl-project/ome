package coordination

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// EvaluateUpdateGate runs the full coordination gate stack for a single
// per-Instance Update decision. It is the decision site the IR-managed
// OMENative path (inferencereplica.Reconciler) wires into
// workload.ReconcileInput.UpdateGate. Leaving UpdateGate nil silently
// disables coordination: CheckSequential / CheckRatio / CheckSurge /
// CheckUnavailability are all skipped, both Components of a Sequential
// group recreate concurrently, and a group-wide MaxSurge never caps a
// Component whose per-Component budget is larger. Keeping the gate body
// here — in the coordination package the IR path depends on — means
// there is exactly one tested implementation.
//
// Gate order: ratio -> surge | unavailability -> sequential. First denial
// wins; later gates short-circuit. The caller reads only the allowed bit;
// denyReason is for logging/observability.
//
// Ratio projection is strategy-aware (see the projDelta computation
// below): SurgeThenDrain projects +1 (a Ready pod is added before the old
// drains), drain-first strategies project -1. A hardcoded -1 for surge
// rolls mis-models the surge as a capacity loss and deadlocks symmetric
// RatioBalanced groups.
//
// In-place strategies (InPlaceIfPossible / InPlaceOnly) skip CheckRatio
// entirely. For in-place the drain is paired 1:1 with a same-pod return
// (mark-not-ready -> patch -> mark-ready), so the net capacity change is
// ~0; running the gate would project a -1 loss that misrepresents that
// transient as permanent and indefinitely over-blocks the smaller
// Component of a ratio pair (e.g. the decoder in a 4:2 engine:decoder
// pair with 25% tolerance deadlocks because 4:1 = 4.0 sits above the
// [1.5, 2.5] band). CheckUnavailability (called below for non-surge
// strategies, which includes both in-place variants) already governs
// concurrent disruption via the MaxUnavailable budget, so RatioBalanced
// safety is preserved without the structural deadlock. The
// RatioGateBypassed event + metric fire so operators see why the gate
// did not run.
//
// Reads authoritative InferenceReplica status via the provided context and
// reader. recorder may be nil (the event is best-effort observability).
// defaults carries the operator-configured group fill-ins (e.g. the ratio
// tolerance default) so the gates see the same resolved groups as the
// coordination reconciler.
func EvaluateUpdateGate(
	ctx context.Context,
	reads client.Reader,
	isvc *v1beta1.InferenceService,
	component v1beta1.ComponentType,
	recorder record.EventRecorder,
	defaults GroupDefaults,
	strategy workloadtypes.UpdateStrategyType,
	inFlightSurge, inFlightUnavail int32,
) (allowed bool, denyReason string) {
	// The gate context is resolved once so the shared prelude (ResolveGroups +
	// MembershipFor + nil-isvc / no-coord short-circuit) runs once instead of
	// four times -- see the coordination.GateContext docs.
	gateCtx := ResolveGateContextWithDefaults(ctx, reads, isvc, component, defaults)

	inPlace := strategy == workloadtypes.UpdateStrategyInPlaceIfPossible ||
		strategy == workloadtypes.UpdateStrategyInPlaceOnly
	if !inPlace {
		// Strategy-aware ratio projection. SurgeThenDrain adds a Ready pod
		// (+1) BEFORE draining the old, so the transient is a capacity GAIN,
		// not a -1 loss. Projecting -1 for a surge roll mis-models it and
		// deadlocks symmetric RatioBalanced groups (both members denied at
		// the start, neither can go first). Drain-first strategies (recreate)
		// keep the honest -1.
		projDelta := int32(-1)
		if strategy == workloadtypes.UpdateStrategySurgeThenDrain || strategy == "" {
			projDelta = 1
		}
		if ok, reason := gateCtx.CheckRatio(inFlightSurge, inFlightUnavail, projDelta); !ok {
			return false, reason
		}
	} else {
		RecordRatioGateBypassed(string(component), string(strategy))
		if recorder != nil && isvc != nil {
			recorder.Eventf(isvc, corev1.EventTypeNormal, EventReasonRatioGateBypassed,
				"RatioGateBypassed: Component=%s, Strategy=%s. RatioBalanced is bypassed for in-place strategies because the drain is paired 1:1 with a same-pod return. Per-Component pacing (CoordinationPacing.PerComponent) still governs concurrent disruption.",
				component, strategy)
		}
	}

	if strategy == workloadtypes.UpdateStrategySurgeThenDrain || strategy == "" {
		if ok, reason := gateCtx.CheckSurge(inFlightSurge); !ok {
			return false, reason
		}
	} else {
		if ok, reason := gateCtx.CheckUnavailability(inFlightUnavail); !ok {
			return false, reason
		}
	}

	if ok, reason := gateCtx.CheckSequential(); !ok {
		return false, reason
	}
	return true, ""
}
