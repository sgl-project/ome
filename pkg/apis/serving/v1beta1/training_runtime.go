package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// TrainingRuntime is the Schema for the TrainingRuntimes API
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="TrainingFrameworks",type="string",JSONPath=".spec.supportedTrainingFrameworks[*].name"
// +kubebuilder:printcolumn:name="TrainingReplicaType",type="string",JSONPath=".spec.replicaType"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type TrainingRuntime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrainingRuntimeSpec   `json:"spec,omitempty"`
	Status TrainingRuntimeStatus `json:"status,omitempty"`
}

// TrainingRuntimeSpec defines the desired state of TrainingRuntime
// +k8s:openapi-gen=true
type TrainingRuntimeSpec struct {
	// Training Framework and version supported by this runtime
	// Example: MPI, TensorFlow, CohereFinetuning
	SupportedTrainingFrameworks []TrainingFramework `json:"supportedTrainingFrameworks,omitempty"`

	// Set to true to disable use of this runtime
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// The pod replica template spec
	*ReplicaSpec `json:",inline"`

	// The target runtime ReplicaType to specify
	// Example: Launcher, Worker
	ReplicaType *ReplicaType `json:"replicaType,omitempty"`

	// Labels that will be added to the runtime spec.
	// More info: http://kubernetes.io/docs/user-guide/labels
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations that will be added to the runtime spec.
	// More info: http://kubernetes.io/docs/user-guide/annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// The compartment ID to use for the training runtime
	// +optional
	CompartmentID string `json:"compartmentID,omitempty"`
}

// TrainingRuntimeStatus defines the observed state of TrainingRuntime
// +k8s:openapi-gen=true
type TrainingRuntimeStatus struct {
	// RuntimeReplicaStatus contains maps from `ReplicaType` to `ReplicaStatus` that specify
	//  the replica current status condition
	RuntimeReplicaStatus map[ReplicaType]*ReplicaStatus `json:"runtimeReplicaStatus,omitempty"`

	// Represents any details about the training runtime
	Details string `json:"details,omitempty"`
}

// TrainingRuntimeList contains a list of TrainingRuntime
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type TrainingRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TrainingRuntime `json:"items"`
}

// ClusterTrainingRuntime is the Schema for the TrainingRuntimes API in cluster scope
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="TrainingFrameworks",type="string",JSONPath=".spec.supportedTrainingFrameworks[*].name"
// +kubebuilder:printcolumn:name="TrainingReplicaType",type="string",JSONPath=".spec.replicaType"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterTrainingRuntime struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrainingRuntimeSpec   `json:"spec,omitempty"`
	Status TrainingRuntimeStatus `json:"status,omitempty"`
}

// ClusterTrainingRuntimeList contains a list of ClusterTrainingRuntime
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type ClusterTrainingRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterTrainingRuntime `json:"items"`
}
