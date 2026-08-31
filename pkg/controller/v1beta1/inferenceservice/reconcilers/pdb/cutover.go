package pdb

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// OMENativeCutoverReady live-reads the expected InferenceReplica before
// allowing its mode-specific PDB selector to become active.
func OMENativeCutoverReady(
	ctx context.Context,
	reader client.Reader,
	expected *v1beta1.InferenceReplica,
) (bool, error) {
	if expected == nil || reader == nil {
		return false, nil
	}
	live := &v1beta1.InferenceReplica{}
	if err := reader.Get(ctx, client.ObjectKeyFromObject(expected), live); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if !sameTarget(expected.UID, expected.Generation, live.UID, live.Generation) {
		return false, nil
	}
	return omeNativeStateReady(live), nil
}

// RawDeploymentCutoverReady live-reads the expected Deployment before
// allowing its mode-specific PDB selector to become active.
func RawDeploymentCutoverReady(
	ctx context.Context,
	reader client.Reader,
	expected *appsv1.Deployment,
) (bool, error) {
	if expected == nil || reader == nil {
		return false, nil
	}
	live := &appsv1.Deployment{}
	if err := reader.Get(ctx, client.ObjectKeyFromObject(expected), live); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if !sameTarget(expected.UID, expected.Generation, live.UID, live.Generation) {
		return false, nil
	}
	return rawDeploymentStateReady(live), nil
}

func sameTarget(expectedUID types.UID, expectedGeneration int64, liveUID types.UID, liveGeneration int64) bool {
	return expectedUID == liveUID && expectedGeneration == liveGeneration
}

func omeNativeStateReady(ir *v1beta1.InferenceReplica) bool {
	if ir == nil || ir.DeletionTimestamp != nil || ir.Spec.Replicas == nil || *ir.Spec.Replicas < 0 {
		return false
	}
	if ir.Status.ObservedGeneration < ir.Generation {
		return false
	}
	desired := *ir.Spec.Replicas
	return ir.Status.ReadyReplicas >= desired && ir.Status.AvailableReplicas >= desired
}

func rawDeploymentStateReady(deployment *appsv1.Deployment) bool {
	if deployment == nil || deployment.DeletionTimestamp != nil || deployment.Spec.Replicas == nil || *deployment.Spec.Replicas < 0 {
		return false
	}
	if deployment.Status.ObservedGeneration < deployment.Generation {
		return false
	}
	desired := *deployment.Spec.Replicas
	return deployment.Status.ReadyReplicas >= desired && deployment.Status.AvailableReplicas >= desired
}
