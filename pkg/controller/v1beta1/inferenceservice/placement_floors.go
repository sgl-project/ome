package inferenceservice

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"knative.dev/pkg/apis"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/placement/protocol"
)

// A placement mark without an execution policy means the hub has not granted
// authority yet. The service says so in its status and keeps its local replica
// policy until the policy arrives.
const (
	placementExecutionPendingReason  = "ExecutionPolicyPending"
	placementExecutionPendingEvent   = "PlacementExecutionPending"
	placementExecutionPendingMessage = "placement marked this service but has not written its execution policy yet; the service keeps reconciling as a local service until the policy arrives"
)

// notePlacementExecutionPending publishes the pending policy and warns once per
// transition; the pass then continues as a local service. Only a write that
// cannot land ends the pass: a conflict requeues, any other failure surfaces.
func (r *InferenceServiceReconciler) notePlacementExecutionPending(ctx context.Context, service *v1beta1.InferenceService) (ctrl.Result, error) {
	changed, err := r.writePlacementCondition(ctx, service, apis.Condition{
		Type: v1beta1.PlacementReplicaFloorsReady, Status: corev1.ConditionUnknown,
		Reason: placementExecutionPendingReason, Message: placementExecutionPendingMessage,
	})
	if apierrors.IsConflict(err) {
		return ctrl.Result{Requeue: true}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	if changed && r.Recorder != nil {
		r.Recorder.Event(service, corev1.EventTypeWarning, placementExecutionPendingEvent, placementExecutionPendingMessage)
	}
	return ctrl.Result{}, nil
}

func checkPlacementReplicaFloors(service *v1beta1.InferenceService, engine *v1beta1.EngineSpec, decoder *v1beta1.DecoderSpec, router *v1beta1.RouterSpec) error {
	policy, err := protocol.FromDerived(service)
	if err != nil {
		return err
	}
	if policy == nil || len(policy.ReplicaFloors) == 0 {
		return nil
	}
	return protocol.CheckReplicaFloors(policy.ReplicaFloors, engine, decoder, router)
}

func clearPlacementReplicaFloorsHold(service *v1beta1.InferenceService) {
	if service.Status.GetCondition(v1beta1.PlacementReplicaFloorsReady) != nil {
		placementBackendConditions.Manage(&service.Status).SetCondition(apis.Condition{
			Type: v1beta1.PlacementReplicaFloorsReady, Status: corev1.ConditionTrue, Reason: "ReplicaFloorsAccepted",
		})
	}
}
