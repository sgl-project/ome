package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ModelFormat struct {
	// Name of the format in which the model is stored, e.g., "ONNX", "TensorFlow SavedModel", "PyTorch", "SafeTensors"
	// +required
	Name string `json:"name"`
	// Version of the model format.
	// Used in validating that a runtime supports a predictor.
	// It Can be "major", "major.minor" or "major.minor.patch".
	// +optional
	Version *string `json:"version,omitempty"`
	// Operator for the selector with supported values: "Equal", "GreaterThan"
	// This is used to select the serving runtime based on the modelFormat version
	// +optional
	// +kubebuilder:default=Equal
	Operator *RuntimeSelectorOperator `json:"operator,omitempty"`
	// Weight of the model format in the runtime selector, used to prioritize modelFormat
	// +optional
	// +kubebuilder:default=1
	Weight int64 `json:"weight,omitempty"`
}

type ModelFrameworkSpec struct {
	// Name of the library in which the model is stored, e.g., "ONNXRuntime", "TensorFlow", "PyTorch", "Transformer", "TensorRTLLM"
	// +required
	Name string `json:"name"`
	// Version of the library.
	// Used in validating that a runtime supports a predictor.
	// It Can be "major", "major.minor" or "major.minor.patch".
	// +optional
	Version *string `json:"version,omitempty"`
	// Operator for the selector with supported values: "Equal", "GreaterThan"
	// This is used to select the serving runtime based on the modelFramework version
	// +optional
	// +kubebuilder:default=Equal
	Operator *RuntimeSelectorOperator `json:"operator,omitempty"`
	// Weight of the framework in the runtime selector, used to prioritize modelFramework
	// +optional
	// +kubebuilder:default=1
	Weight int64 `json:"weight,omitempty"`
}

type RuntimeSelectorOperator string

const (
	RuntimeSelectorOpEqual              RuntimeSelectorOperator = "Equal"
	RuntimeSelectorOpGreaterThan        RuntimeSelectorOperator = "GreaterThan"
	RuntimeSelectorOpGreaterThanOrEqual RuntimeSelectorOperator = "GreaterThanOrEqual"
)

type StorageSpec struct {
	// Path is the absolute path where the model will be downloaded and stored on the node.
	// +optional
	Path *string `json:"path,omitempty"`

	// SchemaPath is the path to the model schema or configuration file within the storage system.
	// This can be used to validate the model or customize how it's loaded.
	// +optional
	SchemaPath *string `json:"schemaPath,omitempty"`

	// Parameters contain key-value pairs to override default storage credentials or configuration.
	// These values are typically used to configure access to object storage or mount options.
	// +optional
	Parameters *map[string]string `json:"parameters,omitempty"`

	// StorageKey is the name of the key in a Kubernetes Secret used to authenticate access to the model storage.
	// This key will be used to fetch credentials during model download or access.
	// +optional
	StorageKey *string `json:"key,omitempty"`

	// StorageUri specifies the source URI of the model in a supported storage backend.
	// Supported formats:
	// - OCI Object Storage:   oci://n/{namespace}/b/{bucket}/o/{object_path}
	// - Persistent Volume:    pvc://{pvc-name}/{sub-path}
	// - Vendor-specific:      vendor://{vendor-name}/{resource-type}/{resource-path}
	// This field is required.
	// +required
	StorageUri *string `json:"storageUri,omitempty"`

	// NodeSelector defines a set of key-value label pairs that must be present on a node
	// for the model to be scheduled and downloaded onto that node.
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`

	// NodeAffinity describes the node affinity rules that further constrain which nodes
	// are eligible to download and store this model, based on advanced scheduling policies.
	// +optional
	NodeAffinity *v1.NodeAffinity `json:"nodeAffinity,omitempty" protobuf:"bytes,1,opt,name=nodeAffinity"`
}

// BaseModelSpec defines the desired state of BaseModel
type BaseModelSpec struct {
	// +optional
	ModelFormat ModelFormat `json:"modelFormat"`

	// ModelType defines the architecture family of the model (e.g., "bert", "gpt2", "llama").
	// This value typically corresponds to the "model_type" field in a Hugging Face model's config.json.
	// It is used to identify the transformer architecture and inform runtime selection and tokenizer behavior.
	// +optional
	ModelType *string `json:"modelType,omitempty"`

	// ModelFramework specifies the underlying framework used by the model,
	// such as "ONNX", "TensorFlow", "PyTorch", "Transformer", or "TensorRTLLM".
	// This value helps determine the appropriate runtime for model serving.
	// +optional
	ModelFramework *ModelFrameworkSpec `json:"modelFramework,omitempty"`

	// ModelArchitecture specifies the concrete model implementation or head,
	// such as "LlamaForCausalLM", "GemmaForCausalLM", or "MixtralForCausalLM".
	// This is often derived from the "architectures" field in Hugging Face config.json.
	// +optional
	ModelArchitecture *string `json:"modelArchitecture,omitempty"`

	// Quantization defines the quantization scheme applied to the model weights,
	// such as "fp8", "fbgemm_fp8", or "int4". This influences runtime compatibility and performance.
	// +optional
	Quantization *ModelQuantization `json:"quantization,omitempty"`

	// ModelParameterSize indicates the total number of parameters in the model,
	// expressed in human-readable form such as "7B", "13B", or "175B".
	// This can be used for scheduling or runtime selection.
	// +optional
	ModelParameterSize *string `json:"modelParameterSize,omitempty"`

	// ModelCapabilities of the model, e.g., "TEXT_GENERATION", "TEXT_SUMMARIZATION", "TEXT_EMBEDDINGS"
	// +listType=atomic
	// +optional
	ModelCapabilities []string `json:"modelCapabilities,omitempty"`

	// Configuration of the model, stored as generic JSON for flexibility.
	// +optional
	ModelConfiguration runtime.RawExtension `json:"modelConfiguration,omitempty"`

	// Storage configuration for the model
	// +required
	Storage *StorageSpec `json:"storage,omitempty"`

	// ModelExtension is the common extension of the model
	ModelExtensionSpec `json:",inline"`

	// +optional Serving mode of the model, e.g., ["On-demand", "Dedicated"]
	// +listType=atomic
	ServingMode []string `json:"servingMode,omitempty"`

	// +optional
	// MaxTokens is the maximum number of tokens that can be processed by the model
	MaxTokens *int32 `json:"maxTokens,omitempty"`

	// Additional metadata for the model
	// +optional
	AdditionalMetadata map[string]string `json:"additionalMetadata,omitempty"`
}

type ModelExtensionSpec struct {
	// DisplayName is the user-friendly name of the model
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// +optional
	Version *string `json:"version,omitempty"`

	// Whether the model is enabled or not
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// Vendor of the model, e.g., "NVIDIA", "Meta", "HuggingFace"
	// +optional
	Vendor *string `json:"vendor,omitempty"`

	// CompartmentID is the compartment ID of the model
	// +optional
	CompartmentID *string `json:"compartmentID,omitempty"`
}

// ServingMode enum
// +kubebuilder:validation:Enum=On-demand;Dedicated
type ServingMode string

const (
	// OnDemand Model Serving Mode
	OnDemand = "On-demand"
	// Dedicated Model Serving Mode
	Dedicated = "Dedicated"
)

type ModelQuantization string

const (
	ModelQuantizationFP8       ModelQuantization = "fp8"
	ModelQuantizationFbgemmFP8 ModelQuantization = "fbgemm_fp8"
	ModelQuantizationINT4      ModelQuantization = "int4"
)

// ModelCapability enum
// +kubebuilder:validation:Enum=TEXT_GENERATION;TEXT_SUMMARIZATION;TEXT_EMBEDDINGS;TEXT_RERANK;CHAT
type ModelCapability string

const (
	ModelCapabilityTextGeneration    ModelCapability = "TEXT_GENERATION"
	ModelCapabilityTextSummarization ModelCapability = "TEXT_SUMMARIZATION"
	ModelCapabilityTextEmbeddings    ModelCapability = "TEXT_EMBEDDINGS"
	ModelCapabilityTextRerank        ModelCapability = "TEXT_RERANK"
	ModelCapabilityChat              ModelCapability = "CHAT"
	ModelCapabilityVision            ModelCapability = "VISION"
	ModelCapabilityUnknown           ModelCapability = ""
)

// ModelWeightStatus enum
// +kubebuilder:validation:Enum=Deprecated;Experiment;Public;Internal
type ModelWeightStatus string

const (
	Deprecated = "Deprecated"
	Experiment = "Experiment"
	Public     = "Public"
	Internal   = "Internal"
)

// FineTunedWeightSpec defines the desired state of FineTunedWeight
type FineTunedWeightSpec struct {
	// Reference to the base model that this weight is fine-tuned from
	// +required
	BaseModelRef ObjectReference `json:"baseModelRef,omitempty"`

	// ModelType of the fine-tuned weight, e.g., "Distillation", "Adapter", "Tfew"
	// +required
	ModelType *string `json:"modelType,omitempty"` // e.g., "LoRA", "Adapter", "Distillation"

	// HyperParameters used for fine-tuning, stored as generic JSON for flexibility
	// +required
	HyperParameters runtime.RawExtension `json:"hyperParameters,omitempty"`

	// ModelExtension is the common extension of the model
	ModelExtensionSpec `json:",inline"`

	// Configuration of the fine-tuned weight, stored as generic JSON for flexibility
	// +optional
	Configuration runtime.RawExtension `json:"configuration,omitempty"`

	// Storage configuration for the fine-tuned weight
	// +required
	Storage *StorageSpec `json:"storage,omitempty"`

	// TrainingJobID is the ID of the training job that produced this weight
	// +optional
	TrainingJobRef ObjectReference `json:"trainingJobRef,omitempty"`
}

// ObjectReference contains enough information to let you inspect or modify the referred object.
type ObjectReference struct {
	// Name of the referenced object
	// +required
	Name *string `json:"name,omitempty"`

	// Namespace of the referenced object
	Namespace *string `json:"namespace,omitempty"`
}

// LifeCycleState enum
// +kubebuilder:validation:Enum=Creating;Importing;In_Transit;In_Training;Ready;Failed
type LifeCycleState string

const (
	LifeCycleStateCreating   LifeCycleState = "Creating"
	LifeCycleStateImporting  LifeCycleState = "Importing"
	LifeCycleStateInTransit  LifeCycleState = "In_Transit"
	LifeCycleStateInTraining LifeCycleState = "In_Training"
	LifeCycleStateReady      LifeCycleState = "Ready"
	LifeCycleStateFailed     LifeCycleState = "Failed"
)

const (
	LifeCycleDetailImporting  string = "Creates Import Job"
	LifeCycleDetailInTransit  string = "Model is in transit"
	LifeCycleDetailInTraining string = "Model is in training"
	LifeCycleDetailReady      string = "Model is ready to use"
	LifeCycleDetailFailed     string = "Associated JobRun Failed"
)

// ModelStatusSpec defines the observed state of Model weight
type ModelStatusSpec struct {
	// LifeCycle is an enum of Deprecated, Experiment, Public, Internal
	LifeCycle *string `json:"lifecycle,omitempty"`

	// Status of the model weight
	State LifeCycleState `json:"state"`

	// +listType=atomic
	NodesReady []string `json:"nodesReady,omitempty"`

	// +listType=atomic
	NodesFailed []string `json:"nodesFailed,omitempty"`
}

// BaseModel is the Schema for the basemodels API
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Vendor",type="string",JSONPath=".spec.vendor"
// +kubebuilder:printcolumn:name="Framework",type=string,JSONPath=".spec.modelFramework.name"
// +kubebuilder:printcolumn:name="FrameworkVersion",type=string,JSONPath=".spec.modelFramework.version"
// +kubebuilder:printcolumn:name="ModelFormat",type="string",JSONPath=".spec.modelFormat.name"
// +kubebuilder:printcolumn:name="Architecture",type="string",JSONPath=".spec.modelArchitecture"
// +kubebuilder:printcolumn:name="Capabilities",type="string",JSONPath=".spec.modelCapabilities[*]"
// +kubebuilder:printcolumn:name="Size",type="string",JSONPath=".spec.modelParameterSize"
// +kubebuilder:printcolumn:name="CompartmentID",type="string",JSONPath=".spec.compartmentID"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type BaseModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BaseModelSpec   `json:"spec,omitempty"`
	Status ModelStatusSpec `json:"status,omitempty"`
}

// ClusterBaseModel is the Schema for the basemodels API
// +k8s:openapi-gen=true
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Vendor",type="string",JSONPath=".spec.vendor"
// +kubebuilder:printcolumn:name="Framework",type=string,JSONPath=".spec.modelFramework.name"
// +kubebuilder:printcolumn:name="FrameworkVersion",type=string,JSONPath=".spec.modelFramework.version"
// +kubebuilder:printcolumn:name="ModelFormat",type="string",JSONPath=".spec.modelFormat.name"
// +kubebuilder:printcolumn:name="Architecture",type="string",JSONPath=".spec.modelArchitecture"
// +kubebuilder:printcolumn:name="Capabilities",type="string",JSONPath=".spec.modelCapabilities[*]"
// +kubebuilder:printcolumn:name="Size",type="string",JSONPath=".spec.modelParameterSize"
// +kubebuilder:printcolumn:name="CompartmentID",type="string",JSONPath=".spec.compartmentID"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterBaseModel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BaseModelSpec   `json:"spec,omitempty"`
	Status ModelStatusSpec `json:"status,omitempty"`
}

// BaseModelList contains a list of BaseModel
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type BaseModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BaseModel `json:"items"`
}

// ClusterBaseModelList contains a list of ClusterBaseModel
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type ClusterBaseModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterBaseModel `json:"items"`
}

// FineTunedWeight is the Schema for the finetunedweights API
// +k8s:openapi-gen=true
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version"
// +kubebuilder:printcolumn:name="Vendor",type="string",JSONPath=".spec.vendor"
// +kubebuilder:printcolumn:name="CompartmentID",type="string",JSONPath=".spec.compartmentID"
// +kubebuilder:printcolumn:name="ModelType",type="string",JSONPath=".spec.modelType"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.state"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type FineTunedWeight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FineTunedWeightSpec `json:"spec,omitempty"`
	Status ModelStatusSpec     `json:"status,omitempty"`
}

// FineTunedWeightList contains a list of FineTunedWeight
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type FineTunedWeightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FineTunedWeight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BaseModel{}, &BaseModelList{})
	SchemeBuilder.Register(&FineTunedWeight{}, &FineTunedWeightList{})
	SchemeBuilder.Register(&ClusterBaseModel{}, &ClusterBaseModelList{})
}
