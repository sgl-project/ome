package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InferenceServiceSpec is the top level type for this resource
type InferenceServiceSpec struct {
	// Predictor defines the model serving spec
	// It specifies how the model should be deployed and served, handling inference requests.
	// Deprecated: Predictor is deprecated and will be removed in a future release. Please use Engine and Model fields instead.
	// +optional
	Predictor PredictorSpec `json:"predictor"`

	// Engine defines the serving engine spec
	// This provides detailed container and pod specifications for model serving.
	// It allows defining the model runner (container spec), as well as complete pod specifications
	// including init containers, sidecar containers, and other pod-level configurations.
	// Engine can also be configured for multi-node deployments using leader and worker specifications.
	// +optional
	Engine *EngineSpec `json:"engine,omitempty"`

	// Decoder defines the decoder spec
	// This is specifically used for PD (Prefill-Decode) disaggregated serving deployments.
	// Similar to Engine in structure, it allows for container and pod specifications,
	// but is only utilized when implementing the disaggregated serving pattern
	// to separate the prefill and decode phases of inference.
	// +optional
	Decoder *DecoderSpec `json:"decoder,omitempty"`

	// Model defines the model to be used for inference, referencing either a BaseModel or a custom model.
	// This allows models to be managed independently of the serving configuration.
	// +optional
	Model *ModelRef `json:"model,omitempty"`

	// Runtime defines the serving runtime environment that will be used to execute the model.
	// It is an inference service spec template that determines how the service should be deployed.
	// Runtime is optional - if not defined, the operator will automatically select the best runtime
	// based on the model's size, architecture, format, quantization, and framework.
	// +optional
	Runtime *ServingRuntimeRef `json:"runtime,omitempty"`

	// Router defines the router spec
	// +optional
	Router *RouterSpec `json:"router,omitempty"`

	// KedaConfig defines the autoscaling configuration for KEDA
	// Provides settings for event-driven autoscaling using KEDA (Kubernetes Event-driven Autoscaling),
	// allowing the service to scale based on custom metrics or event sources.
	KedaConfig *KedaConfig `json:"kedaConfig,omitempty"`
}

// EngineSpec defines the configuration for the Engine component (can be used for both single-node and multi-node deployments)
// Provides a comprehensive specification for deploying model serving containers and pods.
// It allows for complete Kubernetes pod configuration including main containers,
// init containers, sidecars, volumes, and other pod-level settings.
// For distributed deployments, it supports leader-worker architecture configuration.
type EngineSpec struct {
	// This spec provides a full PodSpec for the engine component
	// Allows complete customization of the Kubernetes Pod configuration including
	// containers, volumes, security contexts, affinity rules, and other pod settings.
	// +optional
	PodSpec `json:",inline"`

	// ComponentExtensionSpec defines deployment configuration like min/max replicas, scaling metrics, etc.
	// Controls scaling behavior and resource allocation for the engine component.
	ComponentExtensionSpec `json:",inline"`

	// Runner container override for customizing the engine container
	// This is essentially a container spec that can override the default container
	// Defines the main model runner container configuration, including image,
	// resource requests/limits, environment variables, and command.
	// +optional
	Runner *RunnerSpec `json:"runner,omitempty"`

	// Leader node configuration (only used for MultiNode deployment)
	// Defines the pod and container spec for the leader node that coordinates
	// distributed inference in multi-node deployments.
	// +optional
	Leader *LeaderSpec `json:"leader,omitempty"`

	// Worker nodes configuration (only used for MultiNode deployment)
	// Defines the pod and container spec for worker nodes that perform
	// distributed processing tasks as directed by the leader.
	// +optional
	Worker *WorkerSpec `json:"worker,omitempty"`
}

// DecoderSpec defines the configuration for the Decoder component (token generation in PD-disaggregated deployment)
// Used specifically for prefill-decode disaggregated deployments to handle the token generation phase.
// Similar to EngineSpec in structure, it allows for detailed pod and container configuration,
// but is specifically used for the decode phase when separating prefill and decode processes.
type DecoderSpec struct {
	// This spec provides a full PodSpec for the decoder component
	// Allows complete customization of the Kubernetes Pod configuration including
	// containers, volumes, security contexts, affinity rules, and other pod settings.
	// +optional
	PodSpec `json:",inline"`

	// ComponentExtensionSpec defines deployment configuration like min/max replicas, scaling metrics, etc.
	// Controls scaling behavior and resource allocation for the decoder component.
	ComponentExtensionSpec `json:",inline"`

	// Runner container override for customizing the main container
	// This is essentially a container spec that can override the default container
	// Defines the main decoder container configuration, including image,
	// resource requests/limits, environment variables, and command.
	// +optional
	Runner *RunnerSpec `json:"runner,omitempty"`

	// Leader node configuration (only used for MultiNode deployment)
	// Defines the pod and container spec for the leader node that coordinates
	// distributed token generation in multi-node deployments.
	// +optional
	Leader *LeaderSpec `json:"leader,omitempty"`

	// Worker nodes configuration (only used for MultiNode deployment)
	// Defines the pod and container spec for worker nodes that perform
	// distributed token generation tasks as directed by the leader.
	// +optional
	Worker *WorkerSpec `json:"worker,omitempty"`
}

// LeaderSpec defines the configuration for a leader node in a multi-node component
// The leader node coordinates the activities of worker nodes in distributed inference or
// token generation setups, handling task distribution and result aggregation.
type LeaderSpec struct {
	// Pod specification for the leader node
	// This overrides the main PodSpec when specified
	// Allows customization of the Kubernetes Pod configuration specifically for the leader node.
	// +optional
	PodSpec `json:",inline"`

	// Runner container override for customizing the main container
	// This is essentially a container spec that can override the default container
	// Provides fine-grained control over the container that executes the leader node's coordination logic.
	// +optional
	Runner *RunnerSpec `json:"runner,omitempty"`
}

// WorkerSpec defines the configuration for worker nodes in a multi-node component
// Worker nodes perform the distributed processing tasks assigned by the leader node,
// enabling horizontal scaling for compute-intensive workloads.
type WorkerSpec struct {
	// PodSpec for the worker
	// Allows customization of the Kubernetes Pod configuration specifically for worker nodes.
	// +optional
	PodSpec `json:",inline"`

	// Size of the worker, this is the number of pods in the worker.
	// Controls how many worker pod instances will be deployed for horizontal scaling.
	// +optional
	Size *int `json:"size,omitempty"`

	// Runner container override for customizing the main container
	// This is essentially a container spec that can override the default container
	// Provides fine-grained control over the container that executes the worker node's processing logic.
	// +optional
	Runner *RunnerSpec `json:"runner,omitempty"`
}

// RouterSpec defines the configuration for the Router component, which handles request routing
type RouterSpec struct {
	// PodSpec defines the container configuration for the router
	PodSpec `json:",inline"`

	// ComponentExtensionSpec defines deployment configuration like min/max replicas, scaling metrics, etc.
	ComponentExtensionSpec `json:",inline"`

	// This is essentially a container spec that can override the default container
	// +optional
	Runner *RunnerSpec `json:"runner,omitempty"`

	// Additional configuration parameters for the runner
	// This can include framework-specific settings
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// RunnerSpec defines container configuration plus additional config settings
// The Runner is the primary container that executes the model serving or token generation logic.
type RunnerSpec struct {
	// Container spec for the runner
	// Provides complete Kubernetes container configuration for the primary execution container.
	// +optional
	v1.Container `json:",inline"`
}

type ModelRef struct {
	// Name of the model being referenced
	// Identifies the specific model to be used for inference.
	Name string `json:"name"`

	// Kind of the model being referenced
	// Defaults to ClusterBaseModel
	// Specifies the Kubernetes resource kind of the referenced model.
	// +kubebuilder:default="ClusterBaseModel"
	Kind *string `json:"kind,omitempty"`

	// APIGroup of the resource being referenced
	// Defaults to `ome.io`
	// Specifies the Kubernetes API group of the referenced model.
	// +kubebuilder:default="ome.io"
	APIGroup *string `json:"apiGroup,omitempty"`

	// Optional FineTunedWeights references
	// References to fine-tuned weights that should be applied to the base model.
	// +optional
	// +listType=atomic
	FineTunedWeights []string `json:"fineTunedWeights,omitempty"`
}

type ServingRuntimeRef struct {
	// Name of the runtime being referenced
	// Identifies the specific runtime environment to be used for model execution.
	Name string `json:"name"`

	// Kind of the runtime being referenced
	// Defaults to ClusterServingRuntime
	// Specifies the Kubernetes resource kind of the referenced runtime.
	// ClusterServingRuntime is a cluster-wide runtime, while ServingRuntime is namespace-scoped.
	// +kubebuilder:default="ClusterServingRuntime"
	Kind *string `json:"kind,omitempty"`

	// APIGroup of the resource being referenced
	// Defaults to `ome.io`
	// Specifies the Kubernetes API group of the referenced runtime.
	// +kubebuilder:default="ome.io"
	APIGroup *string `json:"apiGroup,omitempty"`
}

// LoggerType controls the scope of log publishing
// Determines which parts of the request-response cycle are logged.
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
// Configures how request and response payloads are logged for auditing and debugging.
type LoggerSpec struct {
	// URL to send logging events
	// The endpoint where log data will be sent for external processing or storage.
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
// +kubebuilder:printcolumn:name="BaseModel",type="string",JSONPath=".spec.model.name"
// +kubebuilder:printcolumn:name="Runtime",type="string",JSONPath=".spec.runtime.name"
// +kubebuilder:printcolumn:name="Prev",type="integer",JSONPath=".status.components.engine.traffic[?(@.tag=='prev')].percent"
// +kubebuilder:printcolumn:name="Latest",type="integer",JSONPath=".status.components.engine.traffic[?(@.latestRevision==true)].percent"
// +kubebuilder:printcolumn:name="PrevRolledoutRevision",type="string",JSONPath=".status.components.engine.traffic[?(@.tag=='prev')].revisionName"
// +kubebuilder:printcolumn:name="LatestReadyRevision",type="string",JSONPath=".status.components.engine.traffic[?(@.latestRevision==true)].revisionName"
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
