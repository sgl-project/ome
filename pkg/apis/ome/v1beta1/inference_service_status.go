package v1beta1

import (
	"reflect"

	"github.com/sgl-project/sgl-ome/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
	lwsspec "sigs.k8s.io/lws/api/leaderworkerset/v1"
)

// InferenceServiceStatus defines the observed state of InferenceService
type InferenceServiceStatus struct {
	// Conditions for the InferenceService <br/>
	// - PredictorReady: predictor readiness condition; <br/>
	// - RoutesReady (serverless mode only): aggregated routing condition, i.e. endpoint readiness condition; <br/>
	// - LatestDeploymentReady (serverless mode only): aggregated configuration condition, i.e. latest deployment readiness condition; <br/>
	// - Ready: aggregated condition; <br/>
	duckv1.Status `json:",inline"`
	// Addressable endpoint for the InferenceService
	// +optional
	Address *duckv1.Addressable `json:"address,omitempty"`
	// URL holds the url that will distribute traffic over the provided traffic targets.
	// It generally has the form http[s]://{route-name}.{route-namespace}.{cluster-level-suffix}
	// +optional
	URL *apis.URL `json:"url,omitempty"`
	// Statuses for the components of the InferenceService
	Components map[ComponentType]ComponentStatusSpec `json:"components,omitempty"`
	// Model related statuses
	ModelStatus ModelStatus `json:"modelStatus,omitempty"`
}

// ComponentStatusSpec describes the state of the component
type ComponentStatusSpec struct {
	// Latest revision name that is in ready state
	// +optional
	LatestReadyRevision string `json:"latestReadyRevision,omitempty"`
	// Latest revision name that is created
	// +optional
	LatestCreatedRevision string `json:"latestCreatedRevision,omitempty"`
	// Previous revision name that is rolled out with 100 percent traffic
	// +optional
	PreviousRolledoutRevision string `json:"previousRolledoutRevision,omitempty"`
	// Latest revision name that is rolled out with 100 percent traffic
	// +optional
	LatestRolledoutRevision string `json:"latestRolledoutRevision,omitempty"`
	// Traffic holds the configured traffic distribution for latest ready revision and previous rolled out revision.
	// +optional
	// +listType=atomic
	Traffic []knservingv1.TrafficTarget `json:"traffic,omitempty"`
	// URL holds the primary url that will distribute traffic over the provided traffic targets.
	// This will be one the REST or gRPC endpoints that are available.
	// It generally has the form http[s]://{route-name}.{route-namespace}.{cluster-level-suffix}
	// +optional
	URL *apis.URL `json:"url,omitempty"`
	// REST endpoint of the component if available.
	// +optional
	RestURL *apis.URL `json:"restURL,omitempty"`
	// Addressable endpoint for the InferenceService
	// +optional
	Address *duckv1.Addressable `json:"address,omitempty"`
}

// ComponentType contains the different types of components of the service
type ComponentType string

// PredictorComponent ComponentType Enum
const (
	PredictorComponent ComponentType = "predictor"
)

// ConditionType represents a Service condition value
const (
	// PredictorRouteReady is set when network configuration has completed.
	PredictorRouteReady apis.ConditionType = "PredictorRouteReady"
	// PredictorConfigurationReady is set when predictor pods are ready.
	PredictorConfigurationReady apis.ConditionType = "PredictorConfigurationReady"
	// PredictorReady is set when predictor has reported readiness.
	PredictorReady apis.ConditionType = "PredictorReady"
	// IngressReady is set when Ingress is created
	IngressReady apis.ConditionType = "IngressReady"
	// RoutesReady is set when underlying routes for all components have reported readiness.
	RoutesReady apis.ConditionType = "RoutesReady"
	// LatestDeploymentReady is set when underlying configurations for all components have reported readiness.
	LatestDeploymentReady apis.ConditionType = "LatestDeploymentReady"
)

type ModelStatus struct {
	// Whether the available predictor endpoints reflect the current Spec or is in transition
	// +kubebuilder:default=UpToDate
	TransitionStatus TransitionStatus `json:"transitionStatus"`

	// State information of the predictor's model.
	// +optional
	ModelRevisionStates *ModelRevisionStates `json:"modelRevisionStates,omitempty"`

	// Details of last failure, when load of target model is failed or blocked.
	// +optional
	LastFailureInfo *FailureInfo `json:"lastFailureInfo,omitempty"`

	// Model copy information of the predictor's model.
	// +optional
	ModelCopies *ModelCopies `json:"modelCopies,omitempty"`
}

type ModelRevisionStates struct {
	// High level state string: Pending, Standby, Loading, Loaded, FailedToLoad
	// +kubebuilder:default=Pending
	ActiveModelState ModelState `json:"activeModelState"`
	// +kubebuilder:default=""
	TargetModelState ModelState `json:"targetModelState,omitempty"`
}

type ModelCopies struct {
	// How many copies of this predictor's models failed to load recently
	// +kubebuilder:default=0
	FailedCopies int `json:"failedCopies"`
	// Total number copies of this predictor's models that are currently loaded
	// +optional
	TotalCopies int `json:"totalCopies,omitempty"`
}

// TransitionStatus enum
// +kubebuilder:validation:Enum="";UpToDate;InProgress;BlockedByFailedLoad;InvalidSpec
type TransitionStatus string

// TransitionStatus Enum values
const (
	// Predictor is up-to-date (reflects current spec)
	UpToDate TransitionStatus = "UpToDate"
	// Waiting for target model to reach state of active model
	InProgress TransitionStatus = "InProgress"
	// Target model failed to load
	BlockedByFailedLoad TransitionStatus = "BlockedByFailedLoad"
	// Target predictor spec failed validation
	InvalidSpec TransitionStatus = "InvalidSpec"
)

// ModelState enum
// +kubebuilder:validation:Enum="";Pending;Standby;Loading;Loaded;FailedToLoad
type ModelState string

// ModelState Enum values
const (
	// Model is not yet registered
	Pending ModelState = "Pending"
	// Model is available but not loaded (will load when used)
	Standby ModelState = "Standby"
	// Model is loading
	Loading ModelState = "Loading"
	// At least one copy of the model is loaded
	Loaded ModelState = "Loaded"
	// All copies of the model failed to load
	FailedToLoad ModelState = "FailedToLoad"
)

// FailureReason enum
// +kubebuilder:validation:Enum=BaseModelNotReady;BaseModelNotFound;ModelLoadFailed;RuntimeUnhealthy;RuntimeDisabled;NoSupportingRuntime;RuntimeNotRecognized;InvalidPredictorSpec
type FailureReason string

// FailureReason enum values
const (
	// ModelLoadFailed The model failed to load within a ServingRuntime container
	ModelLoadFailed FailureReason = "ModelLoadFailed"
	// RuntimeUnhealthy Corresponding ServingRuntime containers failed to start or are unhealthy
	RuntimeUnhealthy FailureReason = "RuntimeUnhealthy"
	// RuntimeDisabled The ServingRuntime is disabled
	RuntimeDisabled FailureReason = "RuntimeDisabled"
	// NoSupportingRuntime There are no ServingRuntime which support the specified model type
	NoSupportingRuntime FailureReason = "NoSupportingRuntime"
	// RuntimeNotRecognized There is no ServingRuntime defined with the specified runtime name
	RuntimeNotRecognized FailureReason = "RuntimeNotRecognized"
	// InvalidPredictorSpec The current Predictor Spec is invalid or unsupported
	InvalidPredictorSpec FailureReason = "InvalidPredictorSpec"
	// BaseModelNotFound base model is not found either from the cluster level or from the specified namespace
	BaseModelNotFound FailureReason = "BaseModelNotFound"
	// BaseModelNotReady base model is not ready
	BaseModelNotReady FailureReason = "BaseModelNotReady"
	// FineTunedWeightsNotFound not found
	FineTunedWeightsNotFound FailureReason = "FineTunedWeightsNotFound"
	// BaseModelDisabled base model is disabled
	BaseModelDisabled FailureReason = "BaseModelDisabled"
	// FineTunedWeightsDisabled the fine-tuned weights are disabled
	FineTunedWeightsDisabled FailureReason = "FineTunedWeightsDisabled"
	// BaseModelDeprecated base model is deprecated
	BaseModelDeprecated FailureReason = "BaseModelDeprecated"
	// FineTunedWeightsDeprecated the fine-tuned weights are deprecated
	FineTunedWeightsDeprecated FailureReason = "FineTunedWeightsDeprecated"
	// FineTuneWeightLoadFailed fine-tuned weights load failed
	FineTuneWeightLoadFailed FailureReason = "FineTuneWeightLoadFailed"
)

type FailureInfo struct {
	// Name of component to which the failure relates (usually Pod name)
	//+optional
	Location string `json:"location,omitempty"`
	// High level class of failure
	//+optional
	Reason FailureReason `json:"reason,omitempty"`
	// Detailed error message
	//+optional
	Message string `json:"message,omitempty"`
	// Internal Revision/ID of model, tied to specific Spec contents
	//+optional
	ModelRevisionName string `json:"modelRevisionName,omitempty"`
	// Time failure occurred or was discovered
	//+optional
	Time *metav1.Time `json:"time,omitempty"`
	// Exit status from the last termination of the container
	//+optional
	ExitCode int32 `json:"exitCode,omitempty"`
}

var readyConditionsMap = map[ComponentType]apis.ConditionType{
	PredictorComponent: PredictorReady,
}

var routeConditionsMap = map[ComponentType]apis.ConditionType{
	PredictorComponent: PredictorRouteReady,
}

var configurationConditionsMap = map[ComponentType]apis.ConditionType{
	PredictorComponent: PredictorConfigurationReady,
}

var conditionsMapIndex = map[apis.ConditionType]map[ComponentType]apis.ConditionType{
	RoutesReady:           routeConditionsMap,
	LatestDeploymentReady: configurationConditionsMap,
}

// InferenceService Ready condition is depending on predictor and route readiness condition
var conditionSet = apis.NewLivingConditionSet(
	PredictorReady,
	IngressReady,
)

var _ apis.ConditionsAccessor = (*InferenceServiceStatus)(nil)

func (ss *InferenceServiceStatus) InitializeConditions() {
	conditionSet.Manage(ss).InitializeConditions()
}

// IsReady returns the overall readiness for the inference service.
func (ss *InferenceServiceStatus) IsReady() bool {
	return conditionSet.Manage(ss).IsHappy()
}

// GetCondition returns the condition by name.
func (ss *InferenceServiceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return conditionSet.Manage(ss).GetCondition(t)
}

// IsConditionReady returns the readiness for a given condition
func (ss *InferenceServiceStatus) IsConditionReady(t apis.ConditionType) bool {
	condition := conditionSet.Manage(ss).GetCondition(t)
	return condition != nil && condition.Status == v1.ConditionTrue
}

// IsConditionFalse returns if a given condition is False
func (ss *InferenceServiceStatus) IsConditionFalse(t apis.ConditionType) bool {
	condition := conditionSet.Manage(ss).GetCondition(t)
	return condition != nil && condition.Status == v1.ConditionFalse
}

// IsConditionUnknown returns if a given condition is Unknown
func (ss *InferenceServiceStatus) IsConditionUnknown(t apis.ConditionType) bool {
	condition := conditionSet.Manage(ss).GetCondition(t)
	return condition == nil || condition.Status == v1.ConditionUnknown
}

func (ss *InferenceServiceStatus) PropagateRawStatus(
	component ComponentType,
	deployment *appsv1.Deployment,
	url *apis.URL) {
	if len(ss.Components) == 0 {
		ss.Components = make(map[ComponentType]ComponentStatusSpec)
	}
	statusSpec, ok := ss.Components[component]
	if !ok {
		ss.Components[component] = ComponentStatusSpec{}
	}

	statusSpec.LatestCreatedRevision = deployment.GetObjectMeta().GetAnnotations()["deployment.kubernetes.io/revision"]
	condition := getDeploymentCondition(deployment, appsv1.DeploymentAvailable)
	if condition != nil && condition.Status == v1.ConditionTrue {
		statusSpec.URL = url
	}
	readyCondition := readyConditionsMap[component]
	ss.SetCondition(readyCondition, condition)
	ss.Components[component] = statusSpec
	ss.ObservedGeneration = deployment.Status.ObservedGeneration
}

func (ss *InferenceServiceStatus) PropagateMultiNodeStatus(component ComponentType, lws *lwsspec.LeaderWorkerSet, url *apis.URL) {
	if len(ss.Components) == 0 {
		ss.Components = make(map[ComponentType]ComponentStatusSpec)
	}
	statusSpec, ok := ss.Components[component]
	if !ok {
		ss.Components[component] = ComponentStatusSpec{}
	}

	statusSpec.LatestCreatedRevision = lws.GetObjectMeta().GetAnnotations()["resourceVersion"]
	condition := getLWSConditions(lws, lwsspec.LeaderWorkerSetAvailable)
	if condition != nil && condition.Status == v1.ConditionTrue {
		statusSpec.URL = url
	}
	readyCondition := readyConditionsMap[component]
	ss.SetCondition(readyCondition, condition)
	ss.Components[component] = statusSpec
	ss.ObservedGeneration = lws.Generation
}

func getLWSConditions(lws *lwsspec.LeaderWorkerSet, conditionType lwsspec.LeaderWorkerSetConditionType) *apis.Condition {
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

func (ss *InferenceServiceStatus) PropagateMultiNodeRayVLLMStatus(
	component ComponentType,
	deployment []*appsv1.Deployment,
	url *apis.URL) {
	if len(ss.Components) == 0 {
		ss.Components = make(map[ComponentType]ComponentStatusSpec)
	}
	statusSpec, ok := ss.Components[component]
	if !ok {
		ss.Components[component] = ComponentStatusSpec{}
	}

	statusSpec.LatestCreatedRevision = deployment[0].GetObjectMeta().GetAnnotations()["deployment.kubernetes.io/revision"]

	condition := getMultiDeploymentCondition(deployment, appsv1.DeploymentAvailable)
	if condition != nil && condition.Status == v1.ConditionTrue {
		statusSpec.URL = url
	}
	readyCondition := readyConditionsMap[component]
	ss.SetCondition(readyCondition, condition)
	ss.Components[component] = statusSpec
	ss.ObservedGeneration = deployment[0].Status.ObservedGeneration
}

func getMultiDeploymentCondition(deployment []*appsv1.Deployment, conditionType appsv1.DeploymentConditionType) *apis.Condition {
	condition := apis.Condition{}
	allDeploymentsAvailable := true
	for _, d := range deployment {
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
		condition.Type = apis.ConditionType(conditionType)
		condition.Status = v1.ConditionTrue
		condition.Message = deployment[0].Status.Conditions[0].Message
		condition.LastTransitionTime = apis.VolatileTime{
			Inner: deployment[0].Status.Conditions[0].LastTransitionTime,
		}
		condition.Reason = deployment[0].Status.Conditions[0].Reason
	}

	return &condition
}

func getDeploymentCondition(deployment *appsv1.Deployment, conditionType appsv1.DeploymentConditionType) *apis.Condition {
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

// PropagateCrossComponentStatus aggregates the RoutesReady or ConfigurationsReady condition across all available components
// and propagates the RoutesReady or LatestDeploymentReady status accordingly.
func (ss *InferenceServiceStatus) PropagateCrossComponentStatus(componentList []ComponentType, conditionType apis.ConditionType) {
	conditionsMap, ok := conditionsMapIndex[conditionType]
	if !ok {
		return
	}
	crossComponentCondition := &apis.Condition{
		Type:   conditionType,
		Status: v1.ConditionTrue,
	}
	for _, component := range componentList {
		if !ss.IsConditionReady(conditionsMap[component]) {
			crossComponentCondition.Status = v1.ConditionFalse
			if ss.IsConditionUnknown(conditionsMap[component]) { // include check for nil condition
				crossComponentCondition.Status = v1.ConditionUnknown
			}
			crossComponentCondition.Reason = string(conditionsMap[component]) + " not ready"
		}
	}
	ss.SetCondition(conditionType, crossComponentCondition)
}

func (ss *InferenceServiceStatus) PropagateStatus(component ComponentType, serviceStatus *knservingv1.ServiceStatus) {
	if len(ss.Components) == 0 {
		ss.Components = make(map[ComponentType]ComponentStatusSpec)
	}
	statusSpec, ok := ss.Components[component]
	if !ok {
		ss.Components[component] = ComponentStatusSpec{}
	}
	statusSpec.LatestCreatedRevision = serviceStatus.LatestCreatedRevisionName
	revisionTraffic := map[string]int64{}
	for _, traffic := range serviceStatus.Traffic {
		if traffic.Percent != nil {
			revisionTraffic[traffic.RevisionName] += *traffic.Percent
		}
	}
	for _, traffic := range serviceStatus.Traffic {
		if traffic.RevisionName == serviceStatus.LatestReadyRevisionName && traffic.LatestRevision != nil &&
			*traffic.LatestRevision {
			if statusSpec.LatestRolledoutRevision != serviceStatus.LatestReadyRevisionName {
				if traffic.Percent != nil && *traffic.Percent == 100 {
					// track the last revision that's fully rolled out
					statusSpec.PreviousRolledoutRevision = statusSpec.LatestRolledoutRevision
					statusSpec.LatestRolledoutRevision = serviceStatus.LatestReadyRevisionName
				}
			} else {
				// This is to handle case when the latest ready revision is rolled out with 100% and then rolled back
				// so here we need to rollback the LatestRolledoutRevision to PreviousRolledoutRevision
				if serviceStatus.LatestReadyRevisionName == serviceStatus.LatestCreatedRevisionName {
					if traffic.Percent != nil && *traffic.Percent < 100 {
						// check the possibility that the traffic is split over the same revision
						if val, ok := revisionTraffic[traffic.RevisionName]; ok {
							if val == 100 && statusSpec.PreviousRolledoutRevision != "" {
								statusSpec.LatestRolledoutRevision = statusSpec.PreviousRolledoutRevision
							}
						}
					}
				}
			}
		}
	}

	if serviceStatus.LatestReadyRevisionName != statusSpec.LatestReadyRevision {
		statusSpec.LatestReadyRevision = serviceStatus.LatestReadyRevisionName
	}
	// propagate overall ready condition for each component
	readyCondition := serviceStatus.GetCondition(knservingv1.ServiceConditionReady)
	if readyCondition != nil && readyCondition.Status == v1.ConditionTrue {
		if serviceStatus.Address != nil {
			statusSpec.Address = serviceStatus.Address
		}
		if serviceStatus.URL != nil {
			statusSpec.URL = serviceStatus.URL
		}
	}
	readyConditionType := readyConditionsMap[component]
	ss.SetCondition(readyConditionType, readyCondition)
	// propagate route condition for each component
	routeCondition := serviceStatus.GetCondition("RoutesReady")
	routeConditionType := routeConditionsMap[component]
	ss.SetCondition(routeConditionType, routeCondition)
	// propagate configuration condition for each component
	configurationCondition := serviceStatus.GetCondition("ConfigurationsReady")
	configurationConditionType := configurationConditionsMap[component]
	// propagate traffic status for each component
	statusSpec.Traffic = serviceStatus.Traffic
	ss.SetCondition(configurationConditionType, configurationCondition)

	ss.Components[component] = statusSpec
	ss.ObservedGeneration = serviceStatus.ObservedGeneration
}

func (ss *InferenceServiceStatus) SetCondition(conditionType apis.ConditionType, condition *apis.Condition) {
	switch {
	case condition == nil:
	case condition.Status == v1.ConditionUnknown:
		conditionSet.Manage(ss).MarkUnknown(conditionType, condition.Reason, condition.Message)
	case condition.Status == v1.ConditionTrue:
		conditionSet.Manage(ss).MarkTrue(conditionType)
	case condition.Status == v1.ConditionFalse:
		conditionSet.Manage(ss).MarkFalse(conditionType, condition.Reason, condition.Message)
	}
}

func (ss *InferenceServiceStatus) ClearCondition(conditionType apis.ConditionType) {
	if conditionSet.Manage(ss).GetCondition(conditionType) != nil {
		if err := conditionSet.Manage(ss).ClearCondition(conditionType); err != nil {
			return
		}
	}
}

func (ss *InferenceServiceStatus) UpdateModelRevisionStates(modelState ModelState, totalCopies int, info *FailureInfo) {
	if ss.ModelStatus.ModelRevisionStates == nil {
		ss.ModelStatus.ModelRevisionStates = &ModelRevisionStates{TargetModelState: modelState}
	} else {
		ss.ModelStatus.ModelRevisionStates.TargetModelState = modelState
	}
	// Update transition status, failure info based on new model state
	switch modelState {
	case Pending, Loading:
		ss.ModelStatus.TransitionStatus = InProgress
	case Loaded:
		ss.ModelStatus.TransitionStatus = UpToDate
		ss.ModelStatus.ModelCopies = &ModelCopies{TotalCopies: totalCopies}
		ss.ModelStatus.ModelRevisionStates.ActiveModelState = Loaded
	case FailedToLoad:
		ss.ModelStatus.TransitionStatus = BlockedByFailedLoad
	}
	if info != nil {
		ss.SetModelFailureInfo(info)
	}
}

func (ss *InferenceServiceStatus) UpdateModelTransitionStatus(status TransitionStatus, info *FailureInfo) {
	ss.ModelStatus.TransitionStatus = status
	// Update model state to 'FailedToLoad' in case of invalid spec provided
	if ss.ModelStatus.TransitionStatus == InvalidSpec {
		if ss.ModelStatus.ModelRevisionStates == nil {
			ss.ModelStatus.ModelRevisionStates = &ModelRevisionStates{TargetModelState: FailedToLoad}
		} else {
			ss.ModelStatus.ModelRevisionStates.TargetModelState = FailedToLoad
		}
	}
	if info != nil {
		ss.SetModelFailureInfo(info)
	}
}

func (ss *InferenceServiceStatus) SetModelFailureInfo(info *FailureInfo) bool {
	if reflect.DeepEqual(info, ss.ModelStatus.LastFailureInfo) {
		return false
	}
	ss.ModelStatus.LastFailureInfo = info
	return true
}

func (ss *InferenceServiceStatus) PropagateModelStatus(statusSpec ComponentStatusSpec, podList *v1.PodList, rawDeployment bool) {
	// Check at least one pod is running for the latest revision of inferenceservice
	totalCopies := len(podList.Items)
	if totalCopies == 0 {
		ss.UpdateModelRevisionStates(Pending, totalCopies, nil)
		return
	}
	// Update model state to 'Loaded' if inferenceservice status is ready.
	// For serverless deployment, the latest created revision and the latest ready revision should be equal
	if ss.IsReady() {
		if rawDeployment {
			ss.UpdateModelRevisionStates(Loaded, totalCopies, nil)
			return
		} else if statusSpec.LatestCreatedRevision == statusSpec.LatestReadyRevision {
			ss.UpdateModelRevisionStates(Loaded, totalCopies, nil)
			return
		}
	}
	// Update model state to 'Loading' if storage initializer is running.
	// If the storage initializer is terminated due to error or crashloopbackoff, update model
	// state to 'ModelLoadFailed' with failure info.
	for _, cs := range podList.Items[0].Status.InitContainerStatuses {
		if cs.Name == constants.StorageInitializerContainerName {
			switch {
			case cs.State.Running != nil:
				ss.UpdateModelRevisionStates(Loading, totalCopies, nil)
				return
			case cs.State.Terminated != nil && cs.State.Terminated.Reason == constants.StateReasonError:
				ss.UpdateModelRevisionStates(FailedToLoad, totalCopies, &FailureInfo{
					Reason:   ModelLoadFailed,
					Message:  cs.State.Terminated.Message,
					ExitCode: cs.State.Terminated.ExitCode,
				})
				return
			case cs.State.Waiting != nil && cs.State.Waiting.Reason == constants.StateReasonCrashLoopBackOff:
				ss.UpdateModelRevisionStates(FailedToLoad, totalCopies, &FailureInfo{
					Reason:   ModelLoadFailed,
					Message:  cs.LastTerminationState.Terminated.Message,
					ExitCode: cs.LastTerminationState.Terminated.ExitCode,
				})
				return
			}
		}
	}
	// If the ome container is terminated due to error or crashloopbackoff, update model
	// state to 'ModelLoadFailed' with failure info.
	for _, cs := range podList.Items[0].Status.ContainerStatuses {
		if cs.Name == constants.MainContainerName {
			switch {
			case cs.State.Terminated != nil && cs.State.Terminated.Reason == constants.StateReasonError:
				ss.UpdateModelRevisionStates(FailedToLoad, totalCopies, &FailureInfo{
					Reason:   ModelLoadFailed,
					Message:  cs.State.Terminated.Message,
					ExitCode: cs.State.Terminated.ExitCode,
				})
			case cs.State.Waiting != nil && cs.State.Waiting.Reason == constants.StateReasonCrashLoopBackOff:
				ss.UpdateModelRevisionStates(FailedToLoad, totalCopies, &FailureInfo{
					Reason:   ModelLoadFailed,
					Message:  cs.LastTerminationState.Terminated.Message,
					ExitCode: cs.LastTerminationState.Terminated.ExitCode,
				})
			default:
				ss.UpdateModelRevisionStates(Pending, totalCopies, nil)
			}
		}
	}
}
