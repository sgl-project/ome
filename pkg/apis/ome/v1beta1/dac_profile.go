package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DedicatedAIClusterProfileSpec defines the desired state of DedicatedAIClusterProfile
// +k8s:openapi-gen=true
type DedicatedAIClusterProfileSpec struct {
	// Set to true to disable use of this profile.
	// +optional
	Disabled *bool `json:"disabled,omitempty"`

	// Count is the number of units in the DAC
	// +optional
	Count int `json:"count"`

	// The resource requirements of the DAC, get from spec.type + spec.shape
	// +required
	Resources corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,8,opt,name=resources"`

	// The GPU shape affinity of DAC, get from spec.type + spec.shape
	// +required
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations specifies the tolerations for scheduling the resources on tainted nodes.
	// +listType=atomic
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// NodeSelector specifies node selectors for scheduling the resources on specific nodes.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// PriorityClassName is the priority class assigned to workloads in this Dedicated AI Cluster.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
}

// DedicatedAIClusterProfileStatus defines the observed state of DedicatedAIClusterProfile
// +k8s:openapi-gen=true
type DedicatedAIClusterProfileStatus struct {
}

// DedicatedAIClusterProfile is the Schema for the dedicatedaiclusterprofiles API
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="Count",type="integer",JSONPath=".spec.count"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type DedicatedAIClusterProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DedicatedAIClusterProfileSpec   `json:"spec,omitempty"`
	Status DedicatedAIClusterProfileStatus `json:"status,omitempty"`
}

// DedicatedAIClusterProfileList contains a list of DedicatedAIClusterProfile
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type DedicatedAIClusterProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DedicatedAIClusterProfile `json:"items"`
}
