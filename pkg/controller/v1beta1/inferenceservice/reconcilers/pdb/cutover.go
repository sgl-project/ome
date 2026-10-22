package pdb

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// OMENativeCutoverReady reports whether the InferenceReplica the caller
// expects is live, unchanged and serving every pod it asks for, so the PDB
// selector of its mode may become active. A missing object is not ready.
func OMENativeCutoverReady(ctx context.Context, reader client.Reader, expected *v1beta1.InferenceReplica) (bool, error) {
	if expected == nil || reader == nil {
		return false, nil
	}
	live := &v1beta1.InferenceReplica{}
	if err := reader.Get(ctx, client.ObjectKeyFromObject(expected), live); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return cutoverReady(expected, live, omeNativeStateReady), nil
}

// RawDeploymentCutoverReady is the same check for a Deployment.
func RawDeploymentCutoverReady(ctx context.Context, reader client.Reader, expected *appsv1.Deployment) (bool, error) {
	if expected == nil || reader == nil {
		return false, nil
	}
	live := &appsv1.Deployment{}
	if err := reader.Get(ctx, client.ObjectKeyFromObject(expected), live); err != nil {
		return false, client.IgnoreNotFound(err)
	}
	return cutoverReady(expected, live, rawDeploymentStateReady), nil
}

// cutoverReady reports whether live is the object the caller expected, the
// same UID at the same generation, and serving by its kind's reading. Any
// other live object is a replacement, or carries a spec the caller has not
// seen, and is not ready.
func cutoverReady[T client.Object](expected, live T, serving func(T) bool) bool {
	if expected.GetUID() != live.GetUID() || expected.GetGeneration() != live.GetGeneration() {
		return false
	}
	return serving(live)
}

// workloadCounts is what the cut-over reads of one workload, the same for an
// InferenceReplica and a Deployment: whether it is being deleted, the pod
// count it asks for, and the ready and available counts its status reports
// at the generation it last observed.
type workloadCounts struct {
	deleting           bool
	desired            *int32
	generation         int64
	observedGeneration int64
	ready              int32
	available          int32
}

// servesDesired reports whether the workload serves every pod it asks for:
// it is not being deleted, its status is at its generation, and both the
// ready and the available count reach the desired count. A workload with no
// desired count, or a negative one, does not serve.
func (c workloadCounts) servesDesired() bool {
	if c.deleting || c.desired == nil || *c.desired < 0 || c.observedGeneration < c.generation {
		return false
	}
	return c.ready >= *c.desired && c.available >= *c.desired
}

// omeNativeStateReady reports whether an InferenceReplica serves every pod
// it asks for.
func omeNativeStateReady(ir *v1beta1.InferenceReplica) bool {
	if ir == nil {
		return false
	}
	return workloadCounts{
		deleting:           ir.DeletionTimestamp != nil,
		desired:            ir.Spec.Replicas,
		generation:         ir.Generation,
		observedGeneration: ir.Status.ObservedGeneration,
		ready:              ir.Status.ReadyReplicas,
		available:          ir.Status.AvailableReplicas,
	}.servesDesired()
}

// rawDeploymentStateReady reports whether a Deployment serves every pod it
// asks for.
func rawDeploymentStateReady(deployment *appsv1.Deployment) bool {
	if deployment == nil {
		return false
	}
	return workloadCounts{
		deleting:           deployment.DeletionTimestamp != nil,
		desired:            deployment.Spec.Replicas,
		generation:         deployment.Generation,
		observedGeneration: deployment.Status.ObservedGeneration,
		ready:              deployment.Status.ReadyReplicas,
		available:          deployment.Status.AvailableReplicas,
	}.servesDesired()
}
