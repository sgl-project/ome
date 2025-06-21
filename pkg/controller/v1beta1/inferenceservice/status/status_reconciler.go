package status

import (
	"reflect"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	lwsspec "sigs.k8s.io/lws/api/leaderworkerset/v1"
)

// Constants for magic numbers and string literals
const (
	FullTrafficPercent           = 100
	RoutesReadyCondition         = "RoutesReady"
	ConfigurationsReadyCondition = "ConfigurationsReady"
)

// StatusReconciler handles all status-related operations for InferenceService
type StatusReconciler struct{}

// NewStatusReconciler creates a new StatusReconciler instance
func NewStatusReconciler() *StatusReconciler {
	return &StatusReconciler{}
}

// PropagateRawStatus propagates status from raw Kubernetes deployment
func (sm *StatusReconciler) PropagateRawStatus(
	status *v1beta1.InferenceServiceStatus,
	component v1beta1.ComponentType,
	deployment *appsv1.Deployment,
	url *apis.URL) {

	statusSpec := sm.initializeComponentStatus(status, component)

	statusSpec.LatestCreatedRevision = deployment.GetObjectMeta().GetAnnotations()["deployment.kubernetes.io/revision"]
	condition := sm.getDeploymentCondition(deployment, appsv1.DeploymentAvailable)
	if condition != nil && condition.Status == v1.ConditionTrue {
		statusSpec.URL = url
	}
	readyCondition := sm.getReadyConditionsMap()[component]
	sm.setCondition(status, readyCondition, condition)
	status.Components[component] = statusSpec
	status.ObservedGeneration = deployment.Status.ObservedGeneration
}

// PropagateMultiNodeStatus propagates status from LeaderWorkerSet
func (sm *StatusReconciler) PropagateMultiNodeStatus(
	status *v1beta1.InferenceServiceStatus,
	component v1beta1.ComponentType,
	lws *lwsspec.LeaderWorkerSet,
	url *apis.URL) {

	statusSpec := sm.initializeComponentStatus(status, component)

	statusSpec.LatestCreatedRevision = lws.GetObjectMeta().GetAnnotations()["resourceVersion"]
	condition := sm.getLWSConditions(lws, lwsspec.LeaderWorkerSetAvailable)
	if condition != nil && condition.Status == v1.ConditionTrue {
		statusSpec.URL = url
	}
	readyCondition := sm.getReadyConditionsMap()[component]
	sm.setCondition(status, readyCondition, condition)
	status.Components[component] = statusSpec
	status.ObservedGeneration = lws.Generation
}

// PropagateMultiNodeRayVLLMStatus propagates status from multiple deployments
func (sm *StatusReconciler) PropagateMultiNodeRayVLLMStatus(
	status *v1beta1.InferenceServiceStatus,
	component v1beta1.ComponentType,
	deployments []*appsv1.Deployment,
	url *apis.URL) {

	statusSpec := sm.initializeComponentStatus(status, component)

	firstDeployment, err := sm.getFirstDeployment(deployments)
	if err != nil {
		// Handle error case gracefully - set a default state
		sm.setCondition(status, sm.getReadyConditionsMap()[component], &apis.Condition{
			Type:    sm.getReadyConditionsMap()[component],
			Status:  v1.ConditionFalse,
			Reason:  "NoDeployments",
			Message: "No deployments available",
		})
		return
	}

	statusSpec.LatestCreatedRevision = firstDeployment.GetObjectMeta().GetAnnotations()["deployment.kubernetes.io/revision"]

	condition := sm.getMultiDeploymentCondition(deployments, appsv1.DeploymentAvailable)
	if condition != nil && condition.Status == v1.ConditionTrue {
		statusSpec.URL = url
	}
	readyCondition := sm.getReadyConditionsMap()[component]
	sm.setCondition(status, readyCondition, condition)
	status.Components[component] = statusSpec
	status.ObservedGeneration = firstDeployment.Status.ObservedGeneration
}

// PropagateStatus propagates status from Knative Service
func (sm *StatusReconciler) PropagateStatus(
	status *v1beta1.InferenceServiceStatus,
	component v1beta1.ComponentType,
	serviceStatus *knservingv1.ServiceStatus) {

	statusSpec := sm.initializeComponentStatus(status, component)

	statusSpec.LatestCreatedRevision = serviceStatus.LatestCreatedRevisionName
	revisionTraffic := map[string]int64{}
	for _, traffic := range serviceStatus.Traffic {
		if traffic.Percent != nil {
			revisionTraffic[traffic.RevisionName] += *traffic.Percent
		}
	}

	// Handle traffic routing logic
	sm.handleTrafficRouting(&statusSpec, serviceStatus, revisionTraffic)

	if serviceStatus.LatestReadyRevisionName != statusSpec.LatestReadyRevision {
		statusSpec.LatestReadyRevision = serviceStatus.LatestReadyRevisionName
	}

	// Propagate conditions
	sm.propagateServiceConditions(status, component, serviceStatus, &statusSpec)

	status.Components[component] = statusSpec
	status.ObservedGeneration = serviceStatus.ObservedGeneration
}

// PropagateModelStatus propagates model status from pod information
func (sm *StatusReconciler) PropagateModelStatus(
	status *v1beta1.InferenceServiceStatus,
	statusSpec v1beta1.ComponentStatusSpec,
	podList *v1.PodList,
	rawDeployment bool) {

	// Check at least one pod is running for the latest revision of inferenceservice
	totalCopies := len(podList.Items)
	if totalCopies == 0 {
		sm.UpdateModelRevisionStates(status, v1beta1.Pending, totalCopies, nil)
		return
	}

	// Use helper function to safely get the first pod
	firstPod, err := sm.getFirstPod(podList)
	if err != nil {
		sm.UpdateModelRevisionStates(status, v1beta1.Pending, totalCopies, nil)
		return
	}

	// Update model state to 'Loaded' if inferenceservice status is ready.
	if status.IsReady() {
		if rawDeployment {
			sm.UpdateModelRevisionStates(status, v1beta1.Loaded, totalCopies, nil)
			return
		} else if statusSpec.LatestCreatedRevision == statusSpec.LatestReadyRevision {
			sm.UpdateModelRevisionStates(status, v1beta1.Loaded, totalCopies, nil)
			return
		}
	}

	// Check container statuses
	sm.checkContainerStatuses(status, firstPod, totalCopies)
}

// UpdateModelRevisionStates updates the model revision states
func (sm *StatusReconciler) UpdateModelRevisionStates(
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
		sm.SetModelFailureInfo(status, info)
	}
}

// UpdateModelTransitionStatus updates the model transition status
func (sm *StatusReconciler) UpdateModelTransitionStatus(
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
		sm.SetModelFailureInfo(status, info)
	}
}

// SetModelFailureInfo sets the model failure information
func (sm *StatusReconciler) SetModelFailureInfo(status *v1beta1.InferenceServiceStatus, info *v1beta1.FailureInfo) bool {
	if reflect.DeepEqual(info, status.ModelStatus.LastFailureInfo) {
		return false
	}
	status.ModelStatus.LastFailureInfo = info
	return true
}

// PropagateCrossComponentStatus aggregates conditions across components
func (sm *StatusReconciler) PropagateCrossComponentStatus(
	status *v1beta1.InferenceServiceStatus,
	componentList []v1beta1.ComponentType,
	conditionType apis.ConditionType) {

	conditionsMap, ok := sm.getConditionsMapIndex()[conditionType]
	if !ok {
		return
	}

	crossComponentCondition := &apis.Condition{
		Type:   conditionType,
		Status: v1.ConditionTrue,
	}

	for _, component := range componentList {
		if !status.IsConditionReady(conditionsMap[component]) {
			crossComponentCondition.Status = v1.ConditionFalse
			if status.IsConditionUnknown(conditionsMap[component]) {
				crossComponentCondition.Status = v1.ConditionUnknown
			}
			crossComponentCondition.Reason = string(conditionsMap[component]) + " not ready"
		}
	}

	sm.setCondition(status, conditionType, crossComponentCondition)
}
