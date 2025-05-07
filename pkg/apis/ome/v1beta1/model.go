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
}

type ModelFrameworkSpec struct {
	// Name of the library in which the model is stored, e.g., "ONNX", "TensorFlow", "PyTorch", "Transformer", "TensorRTLLM"
	// +required
	Name string `json:"name"`
	// Version of the library.
	// Used in validating that a runtime supports a predictor.
	// It Can be "major", "major.minor" or "major.minor.patch".
	// +optional
	Version *string `json:"version,omitempty"`
}

type StorageSpec struct {
	// The path to the model where it will be downloaded.
	// Default is /mnt/models/vendor/model-name
	// +optional
	Path *string `json:"path,omitempty"`
	// The path to the model schema file in the storage.
	// +optional
	SchemaPath *string `json:"schemaPath,omitempty"`
	// Parameters to override the default storage credentials and config.
	// +optional
	Parameters *map[string]string `json:"parameters,omitempty"`
	// The Storage Key in the secret for this model.
	// +optional
	StorageKey *string `json:"key,omitempty"`
	// The path to the model object in storage.
	// Supported storage types:
	// - OCI object storage (e.g., oci://n/{namespace}/b/{bucket}/o/{object_path})
	// - PVC storage (e.g., pvc://{pvc-name}/{sub-path})
	// - Vendor storage (e.g., vendor://{vendor-name}/{resource-type}/{resource-path})
	// +required
	StorageUri *string `json:"storageUri,omitempty"`
	// NodeSelector is a selector which must be true for the model to fit on a node.
	// Selector which must match a node's labels for the model to be downloaded on that node.
	// +optional
	// +mapType=atomic
	NodeSelector map[string]string `json:"nodeSelector,omitempty" protobuf:"bytes,7,rep,name=nodeSelector"`
	// Describes node affinity rules for the model download.
	// +optional
	NodeAffinity *v1.NodeAffinity `json:"nodeAffinity,omitempty" protobuf:"bytes,1,opt,name=nodeAffinity"`
}

// BaseModelSpec defines the desired state of BaseModel
type BaseModelSpec struct {
	// +optional
	ModelFormat ModelFormat `json:"modelFormat"`

	// +optional
	// DEPRECATED: This field is deprecated and will be removed in future releases.
	// +optional
	ModelType *string `json:"modelType,omitempty"`

	// ModelFramework of the model, e.g., "ONNX", "TensorFlow", "PyTorch", "Transformer", "TensorRTLLM"
	// +optional
	ModelFramework *ModelFrameworkSpec `json:"modelFramework,omitempty"`

	// ModelArchitecture of the model, e.g., "LlamaForCausalLM", "GemmaForCausalLM", "MixtralForCausalLM"
	// +optional
	ModelArchitecture *string `json:"modelArchitecture,omitempty"`

	// Quantization of the model, e.g., "fp8", "fbgemm_fp8", "int4"
	// +optional
	Quantization *ModelQuantization `json:"quantization,omitempty"`

	// ModelParameterSize is the size of the model parameters, e.g., "175B"
	// +optional
	ModelParameterSize *string `json:"modelParameterSize,omitempty"`

	// ModelCapabilities of the model, e.g., "TEXT_GENERATION", "TEXT_SUMMARIZATION", "TEXT_EMBEDDINGS"
	// +listType=atomic
	// +optional
	ModelCapabilities []string `json:"modelCapabilities,omitempty"`

	// Configuration of the model, stored as generic JSON for flexibility.
	// +optional
	ModelConfiguration runtime.RawExtension `json:"modelConfiguration,omitempty"`

	// TensorRT-LLM specific configuration, stored as generic JSON for flexibility.
	// +optional
	TensorRTLLMConfiguration runtime.RawExtension `json:"tensorRTLLMConfiguration,omitempty"`

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

	// DeprecationTime is the time the model was deprecated
	// +optional
	DeprecationTime metav1.Time `json:"deprecationTime,omitempty"`

	// LongTermSupported indicates if the model is long term supported
	// +optional
	IsLongTermSupported *bool `json:"isLongTermSupported,omitempty"`

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
