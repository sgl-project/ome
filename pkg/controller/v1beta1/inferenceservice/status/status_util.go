package status

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	lwsspec "sigs.k8s.io/lws/api/leaderworkerset/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

const containerStartupFailureRestartThreshold int32 = 3

// Helper functions for StatusReconciler

// initializeComponentStatus ensures component status is properly initialized
func (sr *StatusReconciler) initializeComponentStatus(status *v1beta1.InferenceServiceStatus, component v1beta1.ComponentType) v1beta1.ComponentStatusSpec {
	if len(status.Components) == 0 {
		status.Components = make(map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec)
	}
	statusSpec, ok := status.Components[component]
	if !ok {
		statusSpec = v1beta1.ComponentStatusSpec{}
	}
	return statusSpec
}

// getFirstPod safely returns the first pod from a pod list
func (sr *StatusReconciler) getFirstPod(podList *v1.PodList) (*v1.Pod, error) {
	if podList == nil || len(podList.Items) == 0 {
		return nil, fmt.Errorf("pod list is empty")
	}
	return &podList.Items[0], nil
}

// getFirstDeployment safely returns the first deployment from a deployment slice
func (sr *StatusReconciler) getFirstDeployment(deployments []*appsv1.Deployment) (*appsv1.Deployment, error) {
	if len(deployments) == 0 {
		return nil, fmt.Errorf("deployment list is empty")
	}
	return deployments[0], nil
}

// getDeploymentCondition extracts condition from deployment
func (sr *StatusReconciler) getDeploymentCondition(deployment *appsv1.Deployment, conditionType appsv1.DeploymentConditionType) *apis.Condition {
	condition := apis.Condition{}
	for _, con := range deployment.Status.Conditions {
		if con.Type == conditionType {
			condition.Type = apis.ConditionType(conditionType)
			condition.Status = con.Status
			condition.Message = con.Message
			condition.LastTransitionTime = apis.VolatileTime{
				Inner: con.LastTransitionTime,
			}
			condition.Reason = con.Reason
			break
		}
	}
	return &condition
}

func (sr *StatusReconciler) isDeploymentRolloutComplete(deployment *appsv1.Deployment) bool {
	if deployment == nil {
		return false
	}

	desiredReplicas := deployment.Status.Replicas
	if deployment.Spec.Replicas != nil {
		desiredReplicas = *deployment.Spec.Replicas
	}
	if desiredReplicas == 0 {
		return false
	}

	return deployment.Status.ObservedGeneration >= deployment.Generation &&
		deployment.Status.UpdatedReplicas == desiredReplicas &&
		deployment.Status.Replicas == desiredReplicas &&
		deployment.Status.AvailableReplicas == desiredReplicas
}

// getLWSConditions extracts condition from LeaderWorkerSet
func (sr *StatusReconciler) getLWSConditions(lws *lwsspec.LeaderWorkerSet, conditionType lwsspec.LeaderWorkerSetConditionType) *apis.Condition {
	condition := apis.Condition{}
	for _, con := range lws.Status.Conditions {
		if lwsspec.LeaderWorkerSetConditionType(con.Type) == conditionType {
			condition.Type = apis.ConditionType(conditionType)
			condition.Status = v1.ConditionStatus(con.Status)
			condition.Message = con.Message
			condition.LastTransitionTime = apis.VolatileTime{
				Inner: con.LastTransitionTime,
			}
			condition.Reason = con.Reason
			break
		}
	}
	return &condition
}

// getMultiDeploymentCondition checks conditions across multiple deployments
func (sr *StatusReconciler) getMultiDeploymentCondition(deployments []*appsv1.Deployment, conditionType appsv1.DeploymentConditionType) *apis.Condition {
	condition := apis.Condition{}
	allDeploymentsAvailable := true

	if len(deployments) == 0 {
		return &apis.Condition{
			Type:    apis.ConditionType(conditionType),
			Status:  v1.ConditionFalse,
			Reason:  "NoDeployments",
			Message: "No deployments available",
		}
	}

	for _, d := range deployments {
		if d.Status.Conditions == nil {
			allDeploymentsAvailable = false
			break
		}
		for _, con := range d.Status.Conditions {
			if con.Type == conditionType && con.Status == v1.ConditionFalse {
				allDeploymentsAvailable = false
				break
			}
		}
	}

	if allDeploymentsAvailable {
		// Safely access the first deployment's conditions
		firstDeployment := deployments[0]
		if len(firstDeployment.Status.Conditions) > 0 {
			condition.Type = apis.ConditionType(conditionType)
			condition.Status = v1.ConditionTrue
			condition.Message = firstDeployment.Status.Conditions[0].Message
			condition.LastTransitionTime = apis.VolatileTime{
				Inner: firstDeployment.Status.Conditions[0].LastTransitionTime,
			}
			condition.Reason = firstDeployment.Status.Conditions[0].Reason
		} else {
			// Fallback if no conditions exist
			condition.Type = apis.ConditionType(conditionType)
			condition.Status = v1.ConditionTrue
			condition.Message = "All deployments available"
			condition.Reason = "Available"
		}
	}

	return &condition
}

// setCondition sets a condition on the status
func (sr *StatusReconciler) setCondition(status *v1beta1.InferenceServiceStatus, conditionType apis.ConditionType, condition *apis.Condition) {
	if condition != nil {
		status.SetCondition(conditionType, condition)
	}
}

// InitializeComponentCondition initializes a component ready condition if it doesn't exist
// This is used for MultiNode deployments to ensure conditions are visible from the start
func (sr *StatusReconciler) InitializeComponentCondition(status *v1beta1.InferenceServiceStatus, component v1beta1.ComponentType) {
	readyCondition := sr.getReadyConditionsMap()[component]

	// Only initialize if the condition doesn't exist yet
	if !status.IsConditionReady(readyCondition) && !status.IsConditionUnknown(readyCondition) {
		condition := &apis.Condition{
			Type:    readyCondition,
			Status:  v1.ConditionFalse,
			Reason:  "Initializing",
			Message: fmt.Sprintf("%s component initializing", component),
		}
		sr.setCondition(status, readyCondition, condition)
	}
}

// getReadyConditionsMap returns the mapping of component types to ready conditions
func (sr *StatusReconciler) getReadyConditionsMap() map[v1beta1.ComponentType]apis.ConditionType {
	return map[v1beta1.ComponentType]apis.ConditionType{
		v1beta1.RouterComponent:  v1beta1.RouterReady,
		v1beta1.EngineComponent:  v1beta1.EngineReady,
		v1beta1.DecoderComponent: v1beta1.DecoderReady,
	}
}

// checkContainerStatuses checks the status of containers in a pod
func (sr *StatusReconciler) checkContainerStatuses(
	status *v1beta1.InferenceServiceStatus,
	firstPod *v1.Pod,
	totalCopies int,
	currentModelRevisionName string,
	reportContainerStartupFailure bool) {
	// Update model state to 'Loading' if storage initializer is running.
	// If the storage initializer is terminated due to error, update model
	// state to 'ModelLoadFailed' with failure info.
	for _, cs := range firstPod.Status.InitContainerStatuses {
		if cs.Name == constants.StorageInitializerContainerName {
			switch {
			case cs.State.Running != nil:
				sr.UpdateModelRevisionStates(status, v1beta1.Loading, totalCopies, nil)
				return
			case cs.State.Terminated != nil && cs.State.Terminated.Reason == constants.StateReasonError:
				message, exitCode, _ := sr.safeGetTerminationMessage(cs)
				sr.UpdateModelRevisionStates(status, v1beta1.FailedToLoad, totalCopies, &v1beta1.FailureInfo{
					Reason:   v1beta1.ModelLoadFailed,
					Message:  message,
					ExitCode: exitCode,
				})
				return
			case cs.State.Waiting != nil && cs.State.Waiting.Reason == constants.StateReasonCrashLoopBackOff:
				message, exitCode, hasTermination := sr.safeGetTerminationMessage(cs)
				if hasTermination {
					sr.UpdateModelRevisionStates(status, v1beta1.FailedToLoad, totalCopies, &v1beta1.FailureInfo{
						Reason:   v1beta1.ModelLoadFailed,
						Message:  message,
						ExitCode: exitCode,
					})
				} else {
					sr.UpdateModelRevisionStates(status, v1beta1.Pending, totalCopies, nil)
				}
				return
			}
		}
	}

	// If the ome container is terminated due to error, update model
	// state to 'ModelLoadFailed' with failure info.
	for _, cs := range firstPod.Status.ContainerStatuses {
		if cs.Name == constants.MainContainerName {
			switch {
			case cs.State.Terminated != nil && cs.State.Terminated.Reason == constants.StateReasonError:
				message, exitCode, _ := sr.safeGetTerminationMessage(cs)
				sr.UpdateModelRevisionStates(status, v1beta1.FailedToLoad, totalCopies, &v1beta1.FailureInfo{
					Reason:   v1beta1.ModelLoadFailed,
					Message:  message,
					ExitCode: exitCode,
				})
			case cs.State.Waiting != nil && cs.State.Waiting.Reason == constants.StateReasonCrashLoopBackOff:
				var startupFailureInfo *v1beta1.FailureInfo
				if reportContainerStartupFailure {
					startupFailureInfo = sr.getCrashLoopBackOffStartupFailure(firstPod, cs, currentModelRevisionName)
				}

				message, exitCode, hasTermination := sr.safeGetTerminationMessage(cs)
				if hasTermination {
					failureInfo := &v1beta1.FailureInfo{
						Reason:   v1beta1.ModelLoadFailed,
						Message:  message,
						ExitCode: exitCode,
					}
					if startupFailureInfo != nil {
						failureInfo.Reason = startupFailureInfo.Reason
						failureInfo.Location = startupFailureInfo.Location
						failureInfo.ModelRevisionName = startupFailureInfo.ModelRevisionName
						if failureInfo.Message == "" {
							failureInfo.Message = startupFailureInfo.Message
						}
						if failureInfo.ExitCode == 0 {
							failureInfo.ExitCode = startupFailureInfo.ExitCode
						}
					}
					sr.UpdateModelRevisionStates(status, v1beta1.FailedToLoad, totalCopies, failureInfo)
				} else if startupFailureInfo != nil {
					sr.UpdateModelRevisionStates(status, v1beta1.FailedToLoad, totalCopies, startupFailureInfo)
				} else {
					sr.UpdateModelRevisionStates(status, v1beta1.Pending, totalCopies, nil)
				}
			default:
				sr.UpdateModelRevisionStates(status, v1beta1.Pending, totalCopies, nil)
			}
		}
	}
}

func (sr *StatusReconciler) getCrashLoopBackOffStartupFailure(
	pod *v1.Pod,
	cs v1.ContainerStatus,
	currentModelRevisionName string) *v1beta1.FailureInfo {
	if cs.State.Waiting == nil || cs.State.Waiting.Reason != constants.StateReasonCrashLoopBackOff {
		return nil
	}
	if cs.Ready || cs.RestartCount <= containerStartupFailureRestartThreshold {
		return nil
	}

	failureInfo := &v1beta1.FailureInfo{
		Location: fmt.Sprintf("%s/%s", pod.Name, cs.Name),
		Reason:   v1beta1.ContainerStartupFailed,
	}
	message, exitCode, hasTermination := sr.safeGetTerminationMessage(cs)
	if hasTermination {
		failureInfo.Message = message
		failureInfo.ExitCode = exitCode
	} else {
		failureInfo.Message = fmt.Sprintf("Container %q in pod %q never became ready: %s after %d restarts",
			cs.Name, pod.Name, cs.State.Waiting.Reason, cs.RestartCount)
		if cs.State.Waiting.Message != "" {
			failureInfo.Message = fmt.Sprintf("%s: %s", failureInfo.Message, cs.State.Waiting.Message)
		}
	}
	if currentModelRevisionName != "" {
		failureInfo.ModelRevisionName = currentModelRevisionName
	}

	return failureInfo
}

// safeGetTerminationMessage safely extracts termination message from container status
func (sr *StatusReconciler) safeGetTerminationMessage(cs v1.ContainerStatus) (message string, exitCode int32, hasTermination bool) {
	if cs.State.Terminated != nil {
		return cs.State.Terminated.Message, cs.State.Terminated.ExitCode, true
	}
	if cs.State.Waiting != nil && cs.State.Waiting.Reason == constants.StateReasonCrashLoopBackOff {
		if cs.LastTerminationState.Terminated != nil {
			return cs.LastTerminationState.Terminated.Message, cs.LastTerminationState.Terminated.ExitCode, true
		}
	}
	return "", 0, false
}
