package v1beta1

import (
	"k8s.io/apimachinery/pkg/util/intstr"
)

// CohereTrainingJobSpec defines configuration shared across all cohere runtimes
type CohereTrainingJobSpec struct {
	// TrainingJobSpec defines the base job spec
	TrainingJobSpec `json:",inline"`

	// ReplicaSpecs contains maps from `ReplicaType` to `ReplicaSpec` that
	// specify the Training replicas to run.
	ReplicaSpecs map[ReplicaType]*ReplicaSpec `json:"replicaSpecs,omitempty"`
}

// Hyperparameter struct
type Hyperparameter struct {
	Key   string             `json:"key"`
	Value intstr.IntOrString `json:"value"`
}
