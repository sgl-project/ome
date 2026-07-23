package status

import (
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	lwsspec "sigs.k8s.io/lws/api/leaderworkerset/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// StatusReconciler handles all status-related operations for InferenceService
type StatusReconciler struct{}

// NewStatusReconciler creates a new StatusReconciler instance
func NewStatusReconciler() *StatusReconciler {
	return &StatusReconciler{}
}

// PropagateRawStatus propagates status from raw Kubernetes deployment
func (sr *StatusReconciler) PropagateRawStatus(
	status *v1beta1.InferenceServiceStatus,
	component v1beta1.ComponentType,
	deployment *appsv1.Deployment,
	url *apis.URL) {

	statusSpec := sr.initializeComponentStatus(status, component)

	latestRevision := deployment.GetObjectMeta().GetAnnotations()["deployment.kubernetes.io/revision"]
	statusSpec.LatestCreatedRevision = latestRevision
	if latestRevision != "" && sr.isDeploymentRolloutComplete(deployment) {
		statusSpec.LatestReadyRevision = latestRevision
	}
	condition := sr.getDeploymentCondition(deployment, appsv1.DeploymentAvailable)
	if condition != nil && condition.Status == v1.ConditionTrue {
		statusSpec.URL = url
	}
	readyCondition := sr.getReadyConditionsMap()[component]
	sr.setCondition(status, readyCondition, condition)
	status.Components[component] = statusSpec
	status.ObservedGeneration = deployment.Status.ObservedGeneration
}

// PropagateMultiNodeStatus propagates status from LeaderWorkerSet
func (sr *StatusReconciler) PropagateMultiNodeStatus(
	status *v1beta1.InferenceServiceStatus,
	component v1beta1.ComponentType,
	lws *lwsspec.LeaderWorkerSet,
	url *apis.URL) {

	statusSpec := sr.initializeComponentStatus(status, component)

	statusSpec.LatestCreatedRevision = lws.GetObjectMeta().GetAnnotations()["resourceVersion"]
	lwsCondition := sr.getLWSConditions(lws, lwsspec.LeaderWorkerSetAvailable)
	if lwsCondition != nil && lwsCondition.Status == v1.ConditionTrue {
		statusSpec.URL = url
	}

	readyCondition := sr.getReadyConditionsMap()[component]

	// Create a new condition with the correct component ready condition type
	// instead of using the LWS condition type directly
	if lwsCondition != nil {
		componentCondition := &apis.Condition{
			Type:               readyCondition,
			Status:             lwsCondition.Status,
			Message:            lwsCondition.Message,
			Reason:             lwsCondition.Reason,
			LastTransitionTime: lwsCondition.LastTransitionTime,
		}
		sr.setCondition(status, readyCondition, componentCondition)
	}

	status.Components[component] = statusSpec
	status.ObservedGeneration = lws.Generation
}

// PropagateMultiNodeRayVLLMStatus propagates status from multiple deployments
func (sr *StatusReconciler) PropagateMultiNodeRayVLLMStatus(
	status *v1beta1.InferenceServiceStatus,
	component v1beta1.ComponentType,
	deployments []*appsv1.Deployment,
	url *apis.URL) {

	statusSpec := sr.initializeComponentStatus(status, component)

	firstDeployment, err := sr.getFirstDeployment(deployments)
	if err != nil {
		// Handle error case gracefully - set a default state
		sr.setCondition(status, sr.getReadyConditionsMap()[component], &apis.Condition{
			Type:    sr.getReadyConditionsMap()[component],
			Status:  v1.ConditionFalse,
			Reason:  "NoDeployments",
			Message: "No deployments available",
		})
		return
	}

	statusSpec.LatestCreatedRevision = firstDeployment.GetObjectMeta().GetAnnotations()["deployment.kubernetes.io/revision"]

	condition := sr.getMultiDeploymentCondition(deployments, appsv1.DeploymentAvailable)
	if condition != nil && condition.Status == v1.ConditionTrue {
		statusSpec.URL = url
	}
	readyCondition := sr.getReadyConditionsMap()[component]
	sr.setCondition(status, readyCondition, condition)
	status.Components[component] = statusSpec
	status.ObservedGeneration = firstDeployment.Status.ObservedGeneration
}

// PropagateModelStatus propagates model status from pod information
func (sr *StatusReconciler) PropagateModelStatus(
	status *v1beta1.InferenceServiceStatus,
	statusSpec v1beta1.ComponentStatusSpec,
	podList *v1.PodList,
	rawDeployment bool,
	reportContainerStartupFailure bool) {

	// Check at least one pod is running for the latest revision of inferenceservice
	totalCopies := len(podList.Items)
	if totalCopies == 0 {
		sr.UpdateModelRevisionStates(status, v1beta1.Pending, totalCopies, nil)
		return
	}

	// Use helper function to safely get the first pod
	firstPod, err := sr.getFirstPod(podList)
	if err != nil {
		sr.UpdateModelRevisionStates(status, v1beta1.Pending, totalCopies, nil)
		return
	}

	// Update model state to 'Loaded' when the current ready status still matches the latest rollout revision.
	if status.IsReady() && statusSpec.LatestCreatedRevision == statusSpec.LatestReadyRevision {
		sr.UpdateModelRevisionStates(status, v1beta1.Loaded, totalCopies, nil)
		return
	}

	// Check container statuses
	currentModelRevisionName := statusSpec.LatestCreatedRevision
	reportContainerStartupFailure = reportContainerStartupFailure &&
		sr.shouldReportContainerStartupFailure(status, statusSpec, rawDeployment)
	sr.checkContainerStatuses(status, firstPod, totalCopies, currentModelRevisionName, reportContainerStartupFailure)
}

func (sr *StatusReconciler) shouldReportContainerStartupFailure(
	status *v1beta1.InferenceServiceStatus,
	statusSpec v1beta1.ComponentStatusSpec,
	rawDeployment bool) bool {
	if status.ModelStatus.ModelRevisionStates == nil ||
		status.ModelStatus.ModelRevisionStates.ActiveModelState != v1beta1.Loaded {
		return true
	}

	// Raw deployments do not expose a revision-level "latest revision became ready" signal.
	// Once a raw deployment has previously loaded, a later crash during rollout could happen
	// either before first ready or after the new pod was already serving. Keep the existing
	// ModelLoadFailed classification in that case rather than guessing.
	if rawDeployment {
		return false
	}

	// After a model is already loaded, only treat crash loops as startup failures when the
	// component status clearly shows a new rollout is in progress.
	if statusSpec.LatestCreatedRevision == "" || statusSpec.LatestReadyRevision == "" {
		return false
	}
	return statusSpec.LatestCreatedRevision != statusSpec.LatestReadyRevision
}

// UpdateModelRevisionStates updates the model revision states
func (sr *StatusReconciler) UpdateModelRevisionStates(
	status *v1beta1.InferenceServiceStatus,
	modelState v1beta1.ModelState,
	totalCopies int,
	info *v1beta1.FailureInfo) {

	if status.ModelStatus.ModelRevisionStates == nil {
		status.ModelStatus.ModelRevisionStates = &v1beta1.ModelRevisionStates{TargetModelState: modelState}
	} else {
		status.ModelStatus.ModelRevisionStates.TargetModelState = modelState
	}

	// Update transition status, failure info based on new model state
	switch modelState {
	case v1beta1.Pending, v1beta1.Loading:
		status.ModelStatus.TransitionStatus = v1beta1.InProgress
	case v1beta1.Loaded:
		status.ModelStatus.TransitionStatus = v1beta1.UpToDate
		status.ModelStatus.ModelCopies = &v1beta1.ModelCopies{TotalCopies: totalCopies}
		status.ModelStatus.ModelRevisionStates.ActiveModelState = v1beta1.Loaded
	case v1beta1.FailedToLoad:
		status.ModelStatus.TransitionStatus = v1beta1.BlockedByFailedLoad
	}

	if info != nil {
		sr.SetModelFailureInfo(status, info)
	}
}

// UpdateModelTransitionStatus updates the model transition status
func (sr *StatusReconciler) UpdateModelTransitionStatus(
	status *v1beta1.InferenceServiceStatus,
	transitionStatus v1beta1.TransitionStatus,
	info *v1beta1.FailureInfo) {

	status.ModelStatus.TransitionStatus = transitionStatus

	// Update model state to 'FailedToLoad' in case of invalid spec provided
	if status.ModelStatus.TransitionStatus == v1beta1.InvalidSpec {
		if status.ModelStatus.ModelRevisionStates == nil {
			status.ModelStatus.ModelRevisionStates = &v1beta1.ModelRevisionStates{TargetModelState: v1beta1.FailedToLoad}
		} else {
			status.ModelStatus.ModelRevisionStates.TargetModelState = v1beta1.FailedToLoad
		}
	}

	if info != nil {
		sr.SetModelFailureInfo(status, info)
	}
}

// SetModelFailureInfo sets the model failure information
func (sr *StatusReconciler) SetModelFailureInfo(status *v1beta1.InferenceServiceStatus, info *v1beta1.FailureInfo) bool {
	if reflect.DeepEqual(info, status.ModelStatus.LastFailureInfo) {
		return false
	}
	status.ModelStatus.LastFailureInfo = info
	return true
}
