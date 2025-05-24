package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DacLifecycleState is a string enumeration type for the state of the Dedicated AI Cluster.
// +kubebuilder:validation:Enum=ACTIVE;CREATING;DELETING;FAILED;UPDATING
type DacLifecycleState string

// LifecycleState Enum
const (
	ACTIVE   DacLifecycleState = "ACTIVE"
	CREATING DacLifecycleState = "CREATING"
	DELETING DacLifecycleState = "DELETING"
	FAILED   DacLifecycleState = "FAILED"
	UPDATING DacLifecycleState = "UPDATING"
)

// DedicatedAIClusterSpec defines the desired state of DedicatedAICluster
// +k8s:openapi-gen=true
type DedicatedAIClusterSpec struct {

	// DedicatedAIClusterProfileName is the name of the DedicatedAIClusterProfile to use for this DedicatedAICluster.
	// +optional
	Profile string `json:"profile,omitempty"`

	// Count is the number of resources in the DAC
	// +optional
	Count int `json:"count"`

	// The resource requirements of the DAC, get from spec.type + spec.shape
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,8,opt,name=resources"`

	// The GPU shape affinity of DAC, get from spec.type + spec.shape
	// +optional
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

	// The compartment ID to use for the DAC
	// +optional
	CompartmentID string `json:"compartmentID,omitempty"`

	// CapacityReservation ID that used to create this DedicatedAICluster.
	// +optional
	CapacityReservationId string `json:"capacityReservationId,omitempty"`
}

// DedicatedAIClusterStatus defines the observed state of DedicatedAICluster
// +k8s:openapi-gen=true
type DedicatedAIClusterStatus struct {
	//The available number of GPU for allocation
	AvailableGpu int `json:"availableGpu,omitempty"`

	//The number of GPU already allocated
	AllocatedGpu int `json:"allocatedGpu,omitempty"`

	// DacLifecycleState indicates the current phase of the Dedicated AI Cluster (e.g., "active", "creating", "Failed" etc.).
	DacLifecycleState DacLifecycleState `json:"dacLifecycleState,omitempty"`

	// Conditions reflects the current state of the cluster.
	// +listType=atomic
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// A message describing the current state in more detail that can provide actionable information.
	LifecycleDetail string `json:"lifecycleDetail,omitempty"`
}

// DedicatedAICluster is the Schema for the dedicatedaiclusters API
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="Disabled",type="boolean",JSONPath=".spec.disabled"
// +kubebuilder:printcolumn:name="Count",type="integer",JSONPath=".spec.count"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.dacLifecycleState"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type DedicatedAICluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DedicatedAIClusterSpec   `json:"spec,omitempty"`
	Status DedicatedAIClusterStatus `json:"status,omitempty"`
}

// DedicatedAIClusterList contains a list of DedicatedAICluster
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type DedicatedAIClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DedicatedAICluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DedicatedAICluster{}, &DedicatedAIClusterList{})
	SchemeBuilder.Register(&DedicatedAIClusterProfile{}, &DedicatedAIClusterProfileList{})
}
