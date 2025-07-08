package v1beta1

import (
	"github.com/sgl-project/ome/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:openapi-gen=true
type SupportedModelFormat struct {
	// TODO this field is being used as model format name, and this is not correct, we should deprecate this and use Name from ModelFormat
	// Name of the model
	// +optional
	Name string `json:"name"`
	// ModelFormat of the model, e.g., "PyTorch", "TensorFlow", "ONNX", "SafeTensors"
	// +required
	ModelFormat *ModelFormat `json:"modelFormat"`
	// +optional
	// DEPRECATED: This field is deprecated and will be removed in future releases.
	// +optional
	ModelType *string `json:"modelType,omitempty"`
	// Version of the model format.
	// Used in validating that a runtime supports a predictor.
	// It Can be "major", "major.minor" or "major.minor.patch".
	// +optional
	Version *string `json:"version,omitempty"`
	// ModelFramework of the model, e.g., "PyTorch", "TensorFlow", "ONNX", "Transformers"
	// +required
	ModelFramework *ModelFrameworkSpec `json:"modelFramework,omitempty"`
	// ModelArchitecture of the model, e.g., "LlamaForCausalLM", "GemmaForCausalLM", "MixtralForCausalLM"
	// +optional
	ModelArchitecture *string `json:"modelArchitecture,omitempty"`

	// Quantization of the model, e.g., "fp8", "fbgemm_fp8", "int4"
	// +optional
	Quantization *ModelQuantization `json:"quantization,omitempty"`

	// Set to true to allow the ServingRuntime to be used for automatic model placement if
	// this model format is specified with no explicit runtime.
	// +optional
	AutoSelect *bool `json:"autoSelect,omitempty"`

	// +kubebuilder:validation:Minimum=1

	// Priority of this serving runtime for auto selection.
	// This is used to select the serving runtime if more than one serving runtime supports the same model format.
	// The value should be greater than zero.  The higher the value, the higher the priority.
	// Priority is not considered if AutoSelect is either false or not specified.
	// Priority can be overridden by specifying the runtime in the InferenceService.
	// +optional
	Priority *int32 `json:"priority,omitempty"`

	// Operator for the selector with supported values: "Equal", "GreaterThan"
	// This is used to select the serving runtime based on the modelFormat version, modelFramework version
	// +optional
	Operator *RuntimeSelectorOperator `json:"operator,omitempty"`
}

type RuntimeSelectorOperator string

const (
	RuntimeSelectorOpEqual       RuntimeSelectorOperator = "Equal"
	RuntimeSelectorOpGreaterThan RuntimeSelectorOperator = "GreaterThan"
)

// +k8s:openapi-gen=true
type ServingRuntimePodSpec struct {
	// List of containers belonging to the pod.
	// Containers cannot currently be added or removed.
	// Cannot be updated.
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	// +optional
	Containers []corev1.Container `json:"containers" patchStrategy:"merge" patchMergeKey:"name"`

	// List of volumes that can be mounted by containers belonging to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes
	// +optional
	// +patchMergeKey=name
	// +listType=map
	// +listMapKey=name
	// +patchStrategy=merge,retainKeys
	Volumes []corev1.Volume `json:"volumes,omitempty" patchStrategy:"merge,retainKeys" patchMergeKey:"name" protobuf:"bytes,1,rep,name=volumes"`

	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// If specified, the pod's scheduling constraints
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// If specified, the pod's tolerations.
	// +listType=atomic
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Labels that will be add to the pod.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations that will be add to the pod.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=name
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,15,rep,name=imagePullSecrets"`

	// If specified, the pod will be dispatched by specified scheduler.
	// If not specified, the pod will be dispatched by default scheduler.
	// +optional
	SchedulerName string `json:"schedulerName,omitempty" protobuf:"bytes,19,opt,name=schedulerName"`

	// Use the host's ipc namespace.
	// Optional: Default to false.
	// +k8s:conversion-gen=false
	// +optional
	HostIPC bool `json:"hostIPC,omitempty" protobuf:"varint,13,opt,name=hostIPC"`

	// Set DNS policy for the pod.
	// Defaults to "ClusterFirst".
	// Valid values are 'ClusterFirstWithHostNet', 'ClusterFirst', 'Default' or 'None'.
	// DNS parameters given in DNSConfig will be merged with the policy selected with DNSPolicy.
	// To have DNS options set along with hostNetwork, you have to specify DNS policy
	// explicitly to 'ClusterFirstWithHostNet'.
	// +optional
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty" protobuf:"bytes,6,opt,name=dnsPolicy,casttype=DNSPolicy"`

	// Host networking requested for this pod. Use the host's network namespace.
	// If this option is set, the ports that will be used must be specified.
	// Default to false.
	// +k8s:conversion-gen=false
	// +optional
	HostNetwork bool `json:"hostNetwork,omitempty" protobuf:"varint,11,opt,name=hostNetwork"`
}

// ServingRuntimeSpec defines the desired state of ServingRuntime. This spec is currently provisional
// and are subject to change as details regarding single-model serving and multi-model serving
// are hammered out.
// +k8s:openapi-gen=true
type ServingRuntimeSpec struct {
	// Model formats and version supported by this runtime
	// +listType=atomic
	SupportedModelFormats []SupportedModelFormat `json:"supportedModelFormats,omitempty"`

	// ModelSizeRange is the range of model sizes supported by this runtime
	// +optional
	ModelSizeRange *ModelSizeRangeSpec `json:"modelSizeRange,omitempty"`

	// Set to true to disable use of this runtime
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// Router configuration for this runtime
	// +optional
	RouterConfig *RouterSpec `json:"routerConfig,omitempty"`

	// Engine configuration for this runtime
	// +optional
	EngineConfig *EngineSpec `json:"engineConfig,omitempty"`
	// Decoder configuration for this runtime
	// +optional
	DecoderConfig *DecoderSpec `json:"decoderConfig,omitempty"`

	// Supported protocol versions (i.e. openAI or cohere or openInference-v1 or openInference-v2)
	// +optional
	// +listType=atomic
	ProtocolVersions []constants.InferenceServiceProtocol `json:"protocolVersions,omitempty"`

	// PodSpec for the serving runtime
	ServingRuntimePodSpec `json:",inline"`

	// WorkerPodSpec for the serving runtime, this is used for multi-node serving without Ray Cluster
	// +optional
	WorkerPodSpec *WorkerPodSpec `json:"workers,omitempty"`
}

type WorkerPodSpec struct {
	// Size of the worker, this is the number of pods in the worker.
	// +immutable
	// +optional
	Size *int `json:"size"`

	// PodSpec for the worker
	// +optional
	ServingRuntimePodSpec `json:",inline"`
}

// ModelSizeRangeSpec defines the range of model sizes supported by this runtime
// +k8s:openapi-gen=true
type ModelSizeRangeSpec struct {
	// Minimum size of the model in bytes
	// +optional
	Min *string `json:"min,omitempty"`
	// Maximum size of the model in bytes
	// +optional
	Max *string `json:"max,omitempty"`
}

// ServingRuntimeStatus defines the observed state of ServingRuntime
// +k8s:openapi-gen=true
type ServingRuntimeStatus struct {
}

// ServingRuntime is the Schema for the servingruntimes API
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="ModelFormat",type=string,JSONPath=".spec.supportedModelFormats[*].modelFormat.name"
// +kubebuilder:printcolumn:name="ModelFramework",type=string,JSONPath=".spec.supportedModelFormats[*].modelFramework.name"
// +kubebuilder:printcolumn:name="ModelFrameworkVersion",type=string,JSONPath=".spec.supportedModelFormats[*].modelFramework.version"
// +kubebuilder:printcolumn:name="ModelArchitecture",type="string",JSONPath=".spec.supportedModelFormats[*].modelArchitecture"
// +kubebuilder:printcolumn:name="ModelSizeMin",type="string",JSONPath=".spec.modelSizeRange.min"
// +kubebuilder:printcolumn:name="ModelSizeMax",type="string",JSONPath=".spec.modelSizeRange.max"
// +kubebuilder:printcolumn:name="Images",type="string",JSONPath=".spec.containers[*].image"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ServingRuntime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServingRuntimeSpec   `json:"spec,omitempty"`
	Status ServingRuntimeStatus `json:"status,omitempty"`
}

// ServingRuntimeList contains a list of ServingRuntime
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type ServingRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ServingRuntime `json:"items"`
}

// ClusterServingRuntime is the Schema for the servingruntimes API
// +k8s:openapi-gen=true
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="ModelFormat",type=string,JSONPath=".spec.supportedModelFormats[*].modelFormat.name"
// +kubebuilder:printcolumn:name="ModelFramework",type=string,JSONPath=".spec.supportedModelFormats[*].modelFramework.name"
// +kubebuilder:printcolumn:name="ModelFrameworkVersion",type=string,JSONPath=".spec.supportedModelFormats[*].modelFramework.version"
// +kubebuilder:printcolumn:name="ModelArchitecture",type="string",JSONPath=".spec.supportedModelFormats[*].modelArchitecture"
// +kubebuilder:printcolumn:name="ModelSizeMin",type="string",JSONPath=".spec.modelSizeRange.min"
// +kubebuilder:printcolumn:name="ModelSizeMax",type="string",JSONPath=".spec.modelSizeRange.max"
// +kubebuilder:printcolumn:name="Images",type="string",JSONPath=".spec.containers[*].image"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterServingRuntime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServingRuntimeSpec   `json:"spec,omitempty"`
	Status ServingRuntimeStatus `json:"status,omitempty"`
}

// ClusterServingRuntimeList contains a list of ServingRuntime
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type ClusterServingRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterServingRuntime `json:"items"`
}

// SupportedRuntime is the schema for supported runtime result of automatic selection
type SupportedRuntime struct {
	Name string
	Spec ServingRuntimeSpec
}

func (srSpec *ServingRuntimeSpec) IsDisabled() bool {
	return srSpec.Disabled != nil && *srSpec.Disabled
}

func (srSpec *ServingRuntimeSpec) IsProtocolVersionSupported(modelProtocolVersion constants.InferenceServiceProtocol) bool {
	if len(modelProtocolVersion) == 0 || srSpec.ProtocolVersions == nil || len(srSpec.ProtocolVersions) == 0 {
		return true
	}
	for _, srProtocolVersion := range srSpec.ProtocolVersions {
		if srProtocolVersion == modelProtocolVersion {
			return true
		}
	}
	return false
}

// GetPriority returns the priority of the specified model. It returns nil if priority is not set or the model is not found.
func (srSpec *ServingRuntimeSpec) GetPriority(modelName string) *int32 {
	for _, model := range srSpec.SupportedModelFormats {
		if model.Name == modelName {
			return model.Priority
		}
	}
	return nil
}

func (m *SupportedModelFormat) IsAutoSelectEnabled() bool {
	return m.AutoSelect != nil && *m.AutoSelect
}
