package v1beta1

import (
	v1 "k8s.io/api/core/v1"
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

func (peft *PeftTrainingJobSpec) GetLauncherContainer() *v1.Container {
	launcherSpec := peft.GetLauncherReplicaSpec()
	if launcherSpec == nil || len(launcherSpec.Template.Spec.Containers) == 0 {
		return nil
	}
	return &launcherSpec.Template.Spec.Containers[0]
}
