package ops

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// recordNormal emits a Normal K8s Event against target. nil-safe: when
// the recorder isn't wired (tests that construct workload.Deps{}
// directly) OR the target is nil, this is a no-op so callers don't have
// to plumb a recorder. Reason is typed as workload.EventReason — call
// sites pass the workload-owned constant rather than a raw string so a
// typo or drift between reasons becomes a compile-time error.
func recordNormal(rec record.EventRecorder, target client.Object, reason workload.EventReason, messageFmt string, args ...any) {
	if rec == nil || target == nil {
		return
	}
	rec.Eventf(target, corev1.EventTypeNormal, string(reason), messageFmt, args...)
}

// recordWarning emits a Warning K8s Event against target. Same nil-safe
// semantics as recordNormal. Reason is workload-typed for the same
// drift-resistance rationale.
func recordWarning(rec record.EventRecorder, target client.Object, reason workload.EventReason, messageFmt string, args ...any) {
	if rec == nil || target == nil {
		return
	}
	rec.Eventf(target, corev1.EventTypeWarning, string(reason), messageFmt, args...)
}

// eventTarget returns the object emitted events should be stamped
// against — input.EventTarget when set, falling back to OwnerObject.
// Matches the nil-OK semantics documented on ReconcileInput.EventTarget
// so workload code never branches on EventTarget==nil itself.
func eventTarget(input workload.ReconcileInput) client.Object {
	if input.EventTarget != nil {
		return input.EventTarget
	}
	return input.OwnerObject
}

// instanceKey formats "component=engine instance=2" used in event
// messages. Keeps the format consistent across every emitter so
// operators can grep on a single shape.
func instanceKey(component workload.ComponentType, idx int32) string {
	return fmt.Sprintf("component=%s instance=%d", component, idx)
}
