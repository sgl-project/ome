package status

import (
	"time"

	v1 "k8s.io/api/core/v1"
	knapis "knative.dev/pkg/apis"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/obsmetrics"
)

// IsReadyTrue reports whether a status carries an aggregate Ready condition
// set to True.
func IsReadyTrue(status v1beta1.InferenceServiceStatus) bool {
	cond := status.GetCondition(knapis.ConditionReady)
	return cond != nil && cond.Status == v1.ConditionTrue
}

// ObserveTimeToReady records end-to-end deployment latency for one
// not-Ready-to-Ready transition of an InferenceService.
//
// It must be called only after the status write that carried the transition
// has committed, and prevReady must be the aggregate Ready condition from the
// object that write replaced — read from the apiserver, not from an informer
// cache. Both conditions matter: the ISVC's readiness is written by two
// independent paths (the reconciler's own status flush and the IR projector
// stamping a component-ready condition), so a stale baseline makes one path
// attribute a transition the other actually caused.
//
// The start anchor is the instant the ISVC last left Ready, which is that
// prior condition's LastTransitionTime. Before the condition has ever been
// stamped, the ISVC's own creation is the only anchor available.
//
// deploy_kind is read off the prior condition's status: an ISVC that has not
// yet converged reports Unknown, whereas one being rolled out reports False.
// A first deploy that briefly reports False before coming up is therefore
// counted as an update.
func ObserveTimeToReady(isvc *v1beta1.InferenceService, prevReady *knapis.Condition) {
	if isvc == nil {
		return
	}
	start := isvc.CreationTimestamp.Time
	kind := obsmetrics.DeployKindCreate
	if prevReady != nil && !prevReady.LastTransitionTime.Inner.IsZero() {
		start = prevReady.LastTransitionTime.Inner.Time
		if prevReady.Status == v1.ConditionFalse {
			kind = obsmetrics.DeployKindUpdate
		}
	}
	if start.IsZero() {
		return
	}
	obsmetrics.RecordISVCTimeToReady(isvc.Namespace, isvc.Name, kind, time.Since(start).Seconds())
}
