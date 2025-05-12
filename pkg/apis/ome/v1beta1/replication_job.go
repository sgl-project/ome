package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplicationJobPhase defines possible states of a ReplicationJob
type ReplicationJobPhase string

const (
	ReplicationJobCompleted ReplicationJobPhase = "Completed"
	ReplicationJobRunning   ReplicationJobPhase = "Running"
	ReplicationJobPending   ReplicationJobPhase = "Pending"
	ReplicationJobSuspended ReplicationJobPhase = "Suspended"
	ReplicationJobFailed    ReplicationJobPhase = "Failed"
)

// ReplicationJobSpec defines the desired state of ReplicationJob.
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
}

// ReplicationJobStatus represents the current state of the ReplicationJob.
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
// +k8s:openapi-gen=true
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type ReplicationJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicationJobSpec   `json:"spec,omitempty"`
	Status ReplicationJobStatus `json:"status,omitempty"`
}

// ReplicationJobList contains a list of ReplicationJob
// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
type ReplicationJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ReplicationJob `json:"items"`
}
