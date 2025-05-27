package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplicationJobPhase defines possible states of a ReplicationJob
// +kubebuilder:validation:Enum=Completed;Running;Pending;Suspended;Failed
type ReplicationJobPhase string

const (
	ReplicationJobCompleted ReplicationJobPhase = "Completed"
	ReplicationJobRunning   ReplicationJobPhase = "Running"
	ReplicationJobPending   ReplicationJobPhase = "Pending"
	ReplicationJobSuspended ReplicationJobPhase = "Suspended"
	ReplicationJobFailed    ReplicationJobPhase = "Failed"
)

// ReplicationJobSpec defines the desired state of ReplicationJob.
// +k8s:openapi-gen=true
type ReplicationJobSpec struct {
	// Source specifies the data source for the replication job.
	// +required
	Source *StorageSpec `json:"source"`

	// Destination specifies the data destination for the replication job.
	// +required
	Destination *StorageSpec `json:"destination"`

	// The compartment ID to use for the replication job.
	// +optional
	CompartmentID string `json:"compartmentID,omitempty"`

	// ContainerOverride defines the custom container configuration used for the replication job.
	// +optional
	ContainerOverride *v1.Container `json:"containerOverride,omitempty"`
}

// ReplicationJobStatus represents the current state of the ReplicationJob.
// +k8s:openapi-gen=true
type ReplicationJobStatus struct {
	// Status represents the overall phase of the replication job.
	Status ReplicationJobPhase `json:"status,omitempty"`

	// Conditions is an array of current observed job conditions.
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// RetryCount represents the number of retries the replication job has performed.
	RetryCount int `json:"retryCount,omitempty"`

	// StartTime is the time when the replication job was acknowledged by the controller.
	// This field is updated best-effort and may not strictly reflect real-time operation order.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is the time when the replication job completed.
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// LastReconcileTime is the most recent time the job was reconciled.
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// Message is a human-readable message indicating details about the job status.
	Message string `json:"message,omitempty"`
}

// ReplicationJob is the Schema for the replicationjobs API
// +kubebuilder:subresource:status
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type ReplicationJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicationJobSpec   `json:"spec,omitempty"`
	Status ReplicationJobStatus `json:"status,omitempty"`
}

// ReplicationJobList contains a list of ReplicationJob
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ReplicationJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReplicationJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ReplicationJob{}, &ReplicationJobList{})
}
