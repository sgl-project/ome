package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	// TrainingJobRef references the training job that produced this weight
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

// FineTunedWeight describes cluster-scoped weights trained from a base
// model, such as a LoRA adapter, that an InferenceService serves on top of
// that model.
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
	SchemeBuilder.Register(&FineTunedWeight{}, &FineTunedWeightList{})
}
