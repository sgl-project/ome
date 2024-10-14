package v1beta1

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TrainingJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TrainingJobSpec   `json:"spec,omitempty"`
	Status TrainingJobStatus `json:"status,omitempty"`
}

// TrainingJobSpec defines the base job spec which various training job specs implement.
// It defines the desired state of a training job
type TrainingJobSpec struct {
	// +required Specific ClusterBaseModel/BaseModel name to use for hosting the model.
	BaseModel *string `json:"baseModel,omitempty"`

	// Hyperparameters for cohere fine-tuning
	Hyperparameters []Hyperparameter `json:"hyperparameters,omitempty"`

	// Data for training and validation
	Datasets map[constants.DatasetType]*Storage `json:"datasetsSpecs,omitempty"`

	// OutputLocation: define the location where training output stores. Checkpointing etc.
	OutputLocation Storage `json:"outputLocation,omitempty"`

	// The compartment ID to use for the training job
	// +optional
	CompartmentID string `json:"compartmentID,omitempty"`
}

type TrainingJobStatus struct {
	// JobReplicaStatus contains maps from `ReplicaType` to `ReplicaStatus` that specify
	//  the replica current status condition
	JobReplicaStatus map[ReplicaType]*ReplicaStatus `json:"jobReplicaStatus,omitempty"`

	// Conditions is an array of current observed job conditions.
	Conditions []JobCondition `json:"conditions,omitempty"`

	// Represents any details about the training job
	Details string `json:"details,omitempty"`

	// Represents time when the training job is acknowledged by the controller.
	// It is not guaranteed to be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Represents time when the training job is completed. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Represents last time when the job was reconciled. It is not guaranteed to
	// be set in happens-before order across separate operations.
	// It is represented in RFC3339 form and is in UTC.
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}
