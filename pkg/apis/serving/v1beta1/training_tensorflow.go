package v1beta1

// TensorFlowTrainingJobSpec is a desired state description of the TFJob.
type TensorFlowTrainingJobSpec struct {
	// TrainingJobSpec defines the base job spec
	TrainingJobSpec `json:",inline"`

	// RunPolicy encapsulates various runtime policies of the distributed training
	// job, for example how to clean up resources and how long the job can stay
	// active.
	//+kubebuilder:validation:Optional
	RunPolicy RunPolicy `json:"runPolicy"`

	// SuccessPolicy defines the policy to mark the TFJob as succeeded.
	// Default to "", using the default rules.
	// +optional
	SuccessPolicy *SuccessPolicy `json:"successPolicy,omitempty"`

	// A map of TFReplicaType (type) to ReplicaSpec (value). Specifies the TF cluster configuration.
	// For example,
	//   {
	//     "PS": ReplicaSpec,
	//     "Worker": ReplicaSpec,
	//   }
	ReplicaSpecs map[ReplicaType]*ReplicaSpec `json:"tfReplicaSpecs"`

	// A switch to enable dynamic worker
	EnableDynamicWorker bool `json:"enableDynamicWorker,omitempty"`
}

// SuccessPolicy is the success policy.
type SuccessPolicy string

const (
	SuccessPolicyDefault    SuccessPolicy = ""
	SuccessPolicyAllWorkers SuccessPolicy = "AllWorkers"
)
