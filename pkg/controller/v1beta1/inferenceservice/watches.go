package inferenceservice

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// isvcReconcileTriggerPredicate gates the controller's For(&InferenceService{})
// watch so the reconciler reacts to spec and metadata changes but NOT to its
// own status writes.
//
// Why this exists: the autoscaler-status mirror copies the live HPA's
// conditions onto status.components.<comp>.autoscaler.conditions. When a
// Component's HPA can't read a metric (e.g. a CPU-utilization HPA on pods with
// no CPU request) the HPA controller perpetually rewrites a
// ScalingActive=False / FailedGetResourceMetric condition. Even after we
// normalize the rotating pod name out of the message, any status write the
// reconciler makes still bumps resourceVersion — and an unfiltered For() watch
// re-enqueues on every such self-write, producing a reconcile + status-write
// storm.
//
// We deliberately do NOT use predicate.GenerationChangedPredicate: OME drives
// rollouts (canary promote/rollback, etc.) through annotations that do not bump
// .metadata.generation. Dropping those would break rollouts. Instead we enqueue
// when generation changed OR labels/annotations changed, and drop updates that
// only touched status / resourceVersion / managedFields.
//
// Create / Delete / Generic always pass — they are not status-churn sources and
// the reconciler must see object lifecycle transitions.
func isvcReconcileTriggerPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return true
			}
			return metaOrSpecChanged(e.ObjectOld, e.ObjectNew)
		},
	}
}

// ownedStatusIgnoringPredicate gates an Owns() watch so the reconciler reacts
// to spec/generation and metadata changes on the owned object but ignores
// status-only churn.
//
// This is used for the HPA (and KEDA ScaledObject) Owns() watches. The HPA
// controller continuously rewrites the HPA's .status.conditions when it can't
// fetch a metric; without this filter every such rewrite re-enqueues the owning
// ISVC. We still react to HPA spec changes (e.g. min/max replicas) and metadata
// changes via the generation-or-metadata comparison below.
//
// Create / Delete / Generic always pass.
func ownedStatusIgnoringPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return true
			}
			return metaOrSpecChanged(e.ObjectOld, e.ObjectNew)
		},
	}
}

// metaOrSpecChanged reports whether an old→new object transition touched
// .metadata.generation (a proxy for spec changes on a CRD/built-in with a
// status subresource), .metadata.deletionTimestamp, labels, or annotations.
// Status-only writes leave all of these untouched (generation only bumps on
// spec changes; the apiserver does not advance generation for status
// subresource writes), so they return false and are dropped.
//
// The deletionTimestamp check is load-bearing: deleting an object that
// carries a finalizer surfaces as an UPDATE event (deletionTimestamp set),
// not a Delete event — the Delete event only fires when the object is
// actually removed, after every finalizer drops. Without the check the
// reconciler never learns the deletion started and finalizer processing
// stalls until some unrelated event happens to wake it.
func metaOrSpecChanged(oldObj, newObj client.Object) bool {
	if oldObj.GetGeneration() != newObj.GetGeneration() {
		return true
	}
	if (oldObj.GetDeletionTimestamp() == nil) != (newObj.GetDeletionTimestamp() == nil) {
		return true
	}
	if !reflect.DeepEqual(oldObj.GetLabels(), newObj.GetLabels()) {
		return true
	}
	if !reflect.DeepEqual(oldObj.GetAnnotations(), newObj.GetAnnotations()) {
		return true
	}
	return false
}
