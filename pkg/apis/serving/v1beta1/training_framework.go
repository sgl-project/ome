package v1beta1

type TrainingFrameworkType string

const (
	CohereFinetune         TrainingFrameworkType = "CohereFinetune"
	CohereCommandRFinetune TrainingFrameworkType = "CohereCommandRFinetune"
	MPI                    TrainingFrameworkType = "MPI"
	TensorFlow             TrainingFrameworkType = "TensorFlow"
	Peft                   TrainingFrameworkType = "Peft"
	Deepspeed              TrainingFrameworkType = "Deepspeed"
	Accelerate             TrainingFrameworkType = "Accelerate"
	Pytorch                TrainingFrameworkType = "Pytorch"
)

type TrainingFramework struct {
	// Name of the training framework.
	// +required
	Framework TrainingFrameworkType `json:"framework"`

	// Version of training framework.
	// Used in validating that a trainer is supported by a runtime.
	// Can be "major", "major.minor" or "major.minor.patch".
	// +optional
	Version *string `json:"version,omitempty"`

	// Used training runtime against `ReplicaType`
	// of current training framework.
	// +optional
	Runtimes map[ReplicaType]*string `json:"runtimes,omitempty"`
}
