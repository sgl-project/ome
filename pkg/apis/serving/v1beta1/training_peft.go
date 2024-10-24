package v1beta1

var (
	_ TrainingJobImplementation = &PeftTrainingJobSpec{}
)

type PeftTrainingJobSpec struct {
	// TrainingJobSpec defines the base job spec
	TrainingJobSpec `json:",inline"`

	// ReplicaSpecs contains maps from `ReplicaType` to `ReplicaSpec` that
	// specify the Training replicas to run.
	ReplicaSpecs map[ReplicaType]*ReplicaSpec `json:"peftFineTuningReplicaSpecs,omitempty"`
}

func (peft *PeftTrainingJobSpec) GetLauncherReplicaSpec() *ReplicaSpec {
	launcherSpec, exist := peft.ReplicaSpecs[PeftFinetuningReplicaTypeLauncher]
	if !exist {
		return nil
	}
	return launcherSpec
}
