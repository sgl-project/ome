package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InferenceServiceSpec is the top level type for this resource
type InferenceServiceSpec struct {
	// Predictor defines the model serving spec
	// +required
	Predictor PredictorSpec `json:"predictor"`

	// Engine defines the serving engine spec
	// This will be used to replace the predictor
	// +optional
	Engine *EngineSpec `json:"engine,omitempty"`

	// Decoder defines the decoder spec
	// This will be used as decode server for prefill-decode disaggregated deployment
	// +optional
	Decoder *DecoderSpec `json:"decoder,omitempty"`

	// Model specification moved out of predictor for flexibility
	// +optional
	Model *ModelRef `json:"model,omitempty"`

	// Runtime specification moved out of predictor for flexibility
	// +optional
	Runtime *RuntimeRef `json:"runtime,omitempty"`

	// The compartment ID to use for the inference service
	// +optional
	CompartmentID *string `json:"compartmentID,omitempty"`

	// KedaConfig defines the autoscaling configuration for KEDA
	KedaConfig *KedaConfig `json:"kedaConfig,omitempty"`
}

// EngineSpec defines the configuration for the Engine component (can be used for both single-node and multi-node deployments)
type EngineSpec struct {
	// This spec provides a full PodSpec for the engine component
	// +optional
	PodSpec `json:",inline"`

	// ComponentExtensionSpec defines deployment configuration like min/max replicas, scaling metrics, etc.
	ComponentExtensionSpec `json:",inline"`

	// Runner container override for customizing the engine container
	// This is essentially a container spec that can override the default container
	// +optional
	Runner *RunnerSpec `json:"runner,omitempty"`

	// Leader node configuration (only used for MultiNode deployment)
	// +optional
	Leader *LeaderSpec `json:"leader,omitempty"`

	// Worker nodes configuration (only used for MultiNode deployment)
	// +optional
	Worker *WorkerSpec `json:"worker,omitempty"`
}

// DecoderSpec defines the configuration for the Decoder component (token generation in PD-disaggregated deployment)
type DecoderSpec struct {
	// This spec provides a full PodSpec for the decoder component
	// +optional
	PodSpec `json:",inline"`

	// ComponentExtensionSpec defines deployment configuration like min/max replicas, scaling metrics, etc.
	ComponentExtensionSpec `json:",inline"`

	// Runner container override for customizing the main container
	// This is essentially a container spec that can override the default container
	// +optional
	Runner *RunnerSpec `json:"runner,omitempty"`

	// Leader node configuration (only used for MultiNode deployment)
	// +optional
	Leader *LeaderSpec `json:"leader,omitempty"`

	// Worker nodes configuration (only used for MultiNode deployment)
	// +optional
	Worker *WorkerSpec `json:"worker,omitempty"`
}

// LeaderSpec defines the configuration for a leader node in a multi-node component
type LeaderSpec struct {
	// Pod specification for the leader node
	// This overrides the main PodSpec when specified
	// +optional
	PodSpec `json:",inline"`

	// Runner container override for customizing the main container
	// This is essentially a container spec that can override the default container
	// +optional
	Runner *RunnerSpec `json:"runner,omitempty"`
}

// WorkerSpec defines the configuration for worker nodes in a multi-node component
type WorkerSpec struct {
	// PodSpec for the worker
	// +optional
	PodSpec `json:",inline"`

	// Size of the worker, this is the number of pods in the worker.
	// +optional
	Size *int `json:"size,omitempty"`

	// Runner container override for customizing the main container
	// This is essentially a container spec that can override the default container
	// +optional
	Runner *RunnerSpec `json:"runner,omitempty"`
}

// RunnerSpec defines container configuration plus additional config settings
type RunnerSpec struct {
	// Container spec for the runner
	// +optional
	v1.Container `json:",inline"`
}

type ModelRef struct {
	// Name of the model being referenced
	Name string `json:"name"`

	// Kind of the model being referenced
	// Defaults to ClusterBaseModel
	// +kubebuilder:default="ClusterBaseModel"
	Kind *string `json:"kind,omitempty"`

	// APIGroup of the resource being referenced
	// Defaults to `ome.io`
	// +kubebuilder:default="ome.io"
	APIGroup *string `json:"apiGroup,omitempty"`

	// Optional FineTunedWeights references
	// +optional
	FineTunedWeights []string `json:"fineTunedWeights,omitempty"`
}

type ServingRuntimeRef struct {
	// Name of the runtime being referenced
	Name string `json:"name"`

	// Kind of the runtime being referenced
	// Defaults to ClusterServingRuntime
	// +kubebuilder:default="ClusterServingRuntime"
	Kind *string `json:"kind,omitempty"`

	// APIGroup of the resource being referenced
	// Defaults to `ome.io`
	// +kubebuilder:default="ome.io"
	APIGroup *string `json:"apiGroup,omitempty"`
}

// LoggerType controls the scope of log publishing
// +kubebuilder:validation:Enum=all;request;response
type LoggerType string

// LoggerType Enum
const (
	// Logger mode to log both request and response
	LogAll LoggerType = "all"
	// Logger mode to log only request
	LogRequest LoggerType = "request"
	// Logger mode to log only response
	LogResponse LoggerType = "response"
)

// LoggerSpec specifies optional payload logging available for all components
type LoggerSpec struct {
	// URL to send logging events
	// +optional
	URL *string `json:"url,omitempty"`
	// Specifies the scope of the loggers. <br />
	// Valid values are: <br />
	// - "all" (default): log both request and response; <br />
	// - "request": log only request; <br />
	// - "response": log only response <br />
	// +optional
	Mode LoggerType `json:"mode,omitempty"`
}

// InferenceService is the Schema for the InferenceServices API
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="URL",type="string",JSONPath=".status.url"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="BaseModel",type="string",JSONPath=".spec.predictor.model.baseModel"
// +kubebuilder:printcolumn:name="Runtime",type="string",JSONPath=".spec.predictor.model.runtime"
// +kubebuilder:printcolumn:name="Prev",type="integer",JSONPath=".status.components.predictor.traffic[?(@.tag=='prev')].percent"
// +kubebuilder:printcolumn:name="Latest",type="integer",JSONPath=".status.components.predictor.traffic[?(@.latestRevision==true)].percent"
// +kubebuilder:printcolumn:name="PrevRolledoutRevision",type="string",JSONPath=".status.components.predictor.traffic[?(@.tag=='prev')].revisionName"
// +kubebuilder:printcolumn:name="LatestReadyRevision",type="string",JSONPath=".status.components.predictor.traffic[?(@.latestRevision==true)].revisionName"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:path=inferenceservices,shortName=isvc
// +kubebuilder:storageversion
type InferenceService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec InferenceServiceSpec `json:"spec,omitempty"`

	// +kubebuilder:pruning:PreserveUnknownFields
	Status InferenceServiceStatus `json:"status,omitempty"`
}

// InferenceServiceList contains a list of Service
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type InferenceServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// +listType=set
	Items []InferenceService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InferenceService{}, &InferenceServiceList{})
	SchemeBuilder.Register(&ServingRuntime{}, &ServingRuntimeList{})
	SchemeBuilder.Register(&ClusterServingRuntime{}, &ClusterServingRuntimeList{})
}
