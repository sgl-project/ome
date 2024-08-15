package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
type DedicatedAIClusterSpec struct {
	// Count is the number of resources in the DAC
	// +optional
	Count int `json:"count"`

	// The resource requirements of the DAC, get from spec.type + spec.shape
	// +required
	Resources corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,8,opt,name=resources"`

	// The GPU shape affinity of DAC, get from spec.type + spec.shape
	// +required
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Tolerations specifies the tolerations for scheduling the resources on tainted nodes.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// NodeSelector specifies node selectors for scheduling the resources on specific nodes.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// PriorityClassName is the priority class assigned to workloads in this Dedicated AI Cluster.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`
}

// DedicatedAIClusterStatus defines the observed state of DedicatedAICluster
type DedicatedAIClusterStatus struct {
	//The available number of GPU for allocation
	AvailableGpu int `json:"availableGpu,omitempty"`

	//The number of GPU already allocated
	AllocatedGpu int `json:"allocatedGpu,omitempty"`

	// DacLifecycleState indicates the current phase of the Dedicated AI Cluster (e.g., "active", "creating", "Failed" etc.).
	DacLifecycleState DacLifecycleState `json:"dacLifecycleState,omitempty"`

	// Conditions reflects the current state of the cluster.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// A message describing the current state in more detail that can provide actionable information.
	LifecycleDetail string `json:"lifecycleDetail,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DedicatedAICluster is the Schema for the dedicatedaiclusters API
type DedicatedAICluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DedicatedAIClusterSpec   `json:"spec,omitempty"`
	Status DedicatedAIClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DedicatedAIClusterList contains a list of DedicatedAICluster
type DedicatedAIClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DedicatedAICluster `json:"items"`
}
