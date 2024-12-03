package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CapacityReservationLifecycleState is a string enumeration type for the state of the Capacity Reservation.
// +kubebuilder:validation:Enum=Active;Creating;Deleting;Failed;Updating
type CapacityReservationLifecycleState string

// LifecycleState Enum
const (
	CapacityReservationActive   CapacityReservationLifecycleState = "Active"
	CapacityReservationCreating CapacityReservationLifecycleState = "Creating"
	CapacityReservationDeleting CapacityReservationLifecycleState = "Deleting"
	CapacityReservationFailed   CapacityReservationLifecycleState = "Failed"
	CapacityReservationUpdating CapacityReservationLifecycleState = "Updating"
)

// CapacityReservationConditionType is a string enumeration type for the condition type of the Capacity Reservation.
// +kubebuilder:validation:Enum=Ready;ResourcesProvisioned;DACAssociationsHealthy;WorkloadsHealthy
type CapacityReservationConditionType string

// ConditionType Enum
const (
	CapacityReservationReady CapacityReservationConditionType = "Ready"
	ResourcesProvisioned     CapacityReservationConditionType = "ResourcesProvisioned"
	DACAssociationsHealthy   CapacityReservationConditionType = "DACAssociationsHealthy"
	WorkloadsHealthy         CapacityReservationConditionType = "WorkloadsHealthy"
)

// CapacityReservationSpec defines the desired state of Capacity Reservation.
// +k8s:openapi-gen=true
type CapacityReservationSpec struct {
	// The resource requirements of the Capacity Reservation.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty" protobuf:"bytes,8,opt,name=resources"`

	// The compartment ID to use for the Capacity Reservation.
	// +optional
	CompartmentID string `json:"compartmentID,omitempty"`

	// PriorityClassName is the priority class assigned to workloads associated to the Capacity Reservation.
	// +optional
	PriorityClassName string `json:"priorityClassName,omitempty"`

	// AllowBorrowing defines if this capacity reservation can borrow resources from others.
	// +optional
	AllowBorrowing bool `json:"allowBorrowing,omitempty"`

	// Affinity defines node affinity for scheduling workloads.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// NodeSelector defines the node labels for scheduling workloads.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations for taints to allow workloads scheduling on specific nodes.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// CapacityReservationStatus defines the observed status of CapacityReservation.
// +k8s:openapi-gen=true
type CapacityReservationStatus struct {
	// Capacity represents the total resources available in this capacity reservation.
	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty" protobuf:"bytes,2,rep,name=capacity"`

	// Allocatable represents the resources that are available for scheduling.
	// +optional
	Allocatable corev1.ResourceList `json:"allocatable,omitempty" protobuf:"bytes,2,rep,name=allocatable"`

	// Usages of associations
	// An association can be a DAC or a Workload
	// +optional
	AssociationUsages []AssociationUsage `json:"associationUsages,omitempty" protobuf:"bytes,2,rep,name=associationUsages"`

	// Conditions represents health and operational states.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []CapacityReservationCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=conditions"`

	// CapacityReservationLifecycleState indicates the current phase of the CapacityReservation (e.g., "active", "creating", "Failed" etc.).
	CapacityReservationLifecycleState CapacityReservationLifecycleState `json:"capacityReservationLifecycleState,omitempty" protobuf:"capacityReservationLifecycleState"`

	// A message describing the current state in more detail that can provide actionable information.
	// +optional
	LifecycleDetail string `json:"lifecycleDetail,omitempty" protobuf:"bytes,2,name=lifecycleDetail"`
}

// AssociationUsage defines the usage of the association.
// +k8s:openapi-gen=true
type AssociationUsage struct {
	// Name of the association.
	// +required
	Name string `json:"name"`

	// Usage of the association.
	// +required
	Usage corev1.ResourceList `json:"usage" protobuf:"bytes,2,name=usage"`

	// BorrowedQuota of the association.
	// +required
	BorrowedQuota corev1.ResourceList `json:"borrowedQuota" protobuf:"bytes,2,name=borrowedQuota"`
}

// CapacityReservationCondition defines health and operational status of the capacity reservation.
// +k8s:openapi-gen=true
type CapacityReservationCondition struct {
	// Type of condition.
	// +required
	Type CapacityReservationConditionType `json:"type"`

	// Status of the condition.
	// +required
	Status corev1.ConditionStatus `json:"status"`

	// LastTransitionTime is the timestamp when the condition last changed.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason for the condition's last transition.
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message indicating details about the condition.
	// +optional
	Message string `json:"message,omitempty"`
}

// CapacityReservation is the Schema for the capacityReservations API
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type CapacityReservation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CapacityReservationSpec   `json:"spec,omitempty"`
	Status CapacityReservationStatus `json:"status,omitempty"`
}

// CapacityReservationList contains a list of CapacityReservation.
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CapacityReservationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CapacityReservation `json:"items"`
}

// ClusterCapacityReservation is the Schema for the capacityReservations API
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope="Cluster"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ClusterCapacityReservation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CapacityReservationSpec   `json:"spec,omitempty"`
	Status CapacityReservationStatus `json:"status,omitempty"`
}

// ClusterCapacityReservationList contains a list of ClusterCapacityReservation.
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterCapacityReservationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterCapacityReservation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CapacityReservation{}, &CapacityReservationList{})
	SchemeBuilder.Register(&ClusterCapacityReservation{}, &ClusterCapacityReservationList{})
}
