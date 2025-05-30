package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	knservingv1 "knative.dev/serving/pkg/apis/serving/v1"
)

// InferenceServiceStatus defines the observed state of InferenceService
type InferenceServiceStatus struct {
	// Conditions for the InferenceService <br/>
	// - EngineRouteReady: engine route readiness condition; <br/>
	// - DecoderRouteReady: decoder route readiness condition; <br/>
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
	EngineComponent    ComponentType = "engine"
	DecoderComponent   ComponentType = "decoder"
)

// ConditionType represents a Service condition value
const (
	// EngineRouteReady is set when engine route is ready
	EngineRouteReady apis.ConditionType = "EngineRouteReady"
	// DecoderRouteReady is set when decoder route is ready
	DecoderRouteReady apis.ConditionType = "DecoderRouteReady"
	// PredictorRouteReady is set when network configuration has completed.
	PredictorRouteReady apis.ConditionType = "PredictorRouteReady"
	// EngineConfigurationReady is set when engine pods are ready.
	EngineConfigurationReady apis.ConditionType = "EngineConfigurationReady"
	// DecoderConfigurationReady is set when decoder pods are ready.
	DecoderConfigurationReady apis.ConditionType = "DecoderConfigurationReady"
	// PredictorConfigurationReady is set when predictor pods are ready.
	PredictorConfigurationReady apis.ConditionType = "PredictorConfigurationReady"
	// EngineReady is set when engine pods are ready.
	EngineReady apis.ConditionType = "EngineReady"
	// DecoderReady is set when decoder pods are ready.
	DecoderReady apis.ConditionType = "DecoderReady"
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

// InferenceService component conditions
// The overall Ready condition is managed by the conditionSet which only requires IngressReady
// Component-specific ready conditions (PredictorReady, EngineReady, DecoderReady) are managed separately
var conditionSet = apis.NewLivingConditionSet(
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

// SetCondition sets a condition on the status using the conditionSet
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
