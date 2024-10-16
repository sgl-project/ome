package v1beta1

// CohereTrainingJobSpec defines configuration shared across all cohere runtimes
type CohereTrainingJobSpec struct {
	// TrainingJobSpec defines the base job spec
	TrainingJobSpec `json:",inline"`

	// ReplicaSpecs contains maps from `ReplicaType` to `ReplicaSpec` that
	// specify the Training replicas to run.
	ReplicaSpecs map[ReplicaType]*ReplicaSpec `json:"replicaSpecs,omitempty"`
}
