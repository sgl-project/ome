package status

import (
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
)

// DeriveLifecycleState converts detailed InferenceService status into a high-level state.
func DeriveLifecycleState(
	isvc *v1beta1.InferenceService,
	previousState v1beta1.InferenceServiceLifecycleState,
) v1beta1.InferenceServiceLifecycleState {
	if isvc == nil {
		return v1beta1.InferenceServiceLifecycleStateCreating
	}

	if !isvc.GetDeletionTimestamp().IsZero() {
		return v1beta1.InferenceServiceLifecycleStateDeleting
	}

	readyCondition := isvc.Status.GetCondition(apis.ConditionReady)
	if lifecycleTransitionFailed(isvc.Status.ModelStatus.TransitionStatus) ||
		hasFailure(&isvc.Status) {
		return v1beta1.InferenceServiceLifecycleStateFailed
	}

	if isvc.Status.IsReady() {
		return v1beta1.InferenceServiceLifecycleStateReady
	}

	if lifecycleProgressing(&isvc.Status, readyCondition) {
		if lifecyclePreviouslyEstablished(previousState) {
			return v1beta1.InferenceServiceLifecycleStateUpdating
		}
		return v1beta1.InferenceServiceLifecycleStateCreating
	}

	if readyCondition != nil &&
		readyCondition.Status == v1.ConditionFalse &&
		lifecyclePreviouslyEstablished(previousState) {
		return v1beta1.InferenceServiceLifecycleStateFailed
	}

	return v1beta1.InferenceServiceLifecycleStateCreating
}

func lifecycleTransitionFailed(transitionStatus v1beta1.TransitionStatus) bool {
	return transitionStatus == v1beta1.InvalidSpec || transitionStatus == v1beta1.BlockedByFailedLoad
}

func hasFailure(status *v1beta1.InferenceServiceStatus) bool {
	return status != nil && status.ModelStatus.LastFailureInfo != nil
}

func lifecycleProgressing(status *v1beta1.InferenceServiceStatus, readyCondition *apis.Condition) bool {
	if status.ModelStatus.TransitionStatus == v1beta1.InProgress {
		return true
	}
	if readyCondition != nil && readyCondition.Status == v1.ConditionUnknown {
		return true
	}
	for _, componentStatus := range status.Components {
		if componentStatus.LatestCreatedRevision != "" &&
			componentStatus.LatestReadyRevision != "" &&
			componentStatus.LatestCreatedRevision != componentStatus.LatestReadyRevision {
			return true
		}
	}
	return false
}

func lifecyclePreviouslyEstablished(previousState v1beta1.InferenceServiceLifecycleState) bool {
	switch previousState {
	case v1beta1.InferenceServiceLifecycleStateReady,
		v1beta1.InferenceServiceLifecycleStateUpdating,
		v1beta1.InferenceServiceLifecycleStateFailed:
		return true
	default:
		return false
	}
}
