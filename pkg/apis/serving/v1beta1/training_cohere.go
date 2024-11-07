package v1beta1

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	v1 "k8s.io/api/core/v1"
	"strings"
)

// CohereTrainingJobSpec defines configuration shared across all cohere runtimes
type CohereTrainingJobSpec struct {
	// TrainingJobSpec defines the base job spec
	TrainingJobSpec `json:",inline"`

	// ReplicaSpecs contains maps from `ReplicaType` to `ReplicaSpec` that
	// specify the Training replicas to run.
	ReplicaSpecs map[ReplicaType]*ReplicaSpec `json:"replicaSpecs,omitempty"`
}

func IsCohereFTRuntime(runtimeName string) bool {
	return strings.HasPrefix(runtimeName, string(constants.CohereTrainingRuntimePrefix))
}

func IsCohereFTCommandRRuntime(runtimeName string) bool {
	return strings.HasPrefix(runtimeName, constants.CohereCommandRFTRuntimePrefix)
}

func (ctjs *CohereTrainingJobSpec) GetLauncherReplicaSpec() *ReplicaSpec {
	launcherSpec, exist := ctjs.ReplicaSpecs[CohereLauncher]
	if !exist {
		return nil
	}
	return launcherSpec
}

func (ctjs *CohereTrainingJobSpec) GetLauncherContainer() *v1.Container {
	launcherSpec := ctjs.GetLauncherReplicaSpec()
	if launcherSpec == nil || len(launcherSpec.Template.Spec.Containers) == 0 {
		return nil
	}
	return &launcherSpec.Template.Spec.Containers[0]
}
