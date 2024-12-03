package v1beta1

import (
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
	FrameworkType TrainingFrameworkType `json:"framework"`

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

func (tf *TrainingFramework) IsRuntimeSpecified() bool {
	return tf.Runtimes != nil && len(tf.Runtimes) > 0
}

func (tf *TrainingFramework) GetMostRecentSupportedRuntime(cl client.Client, namespace string) (*TrainingRuntimeSpec, error) {
	// List all namespace-scoped runtimes.
	runtimes := &TrainingRuntimeList{}
	if err := cl.List(context.TODO(), runtimes, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	// Sort namespace-scoped runtimes by created timestamp desc and name asc.
	sortTrainingRuntimeList(runtimes)

	// List all cluster-scoped runtimes.
	clusterRuntimes := &ClusterTrainingRuntimeList{}
	if err := cl.List(context.TODO(), clusterRuntimes); err != nil {
		return nil, err
	}
	// Sort cluster-scoped runtimes by created timestamp desc and name asc.
	sortClusterTrainingRuntimeList(clusterRuntimes)

	for i := range runtimes.Items {
		rt := &runtimes.Items[i]
		if !rt.Spec.IsDisabled() && rt.Spec.IsTrainingFrameworkSupported(*tf) {
			if isLauncherType(*rt.Spec.ReplicaType) {
				return &rt.Spec, nil
			}
		}
	}

	for i := range clusterRuntimes.Items {
		crt := &clusterRuntimes.Items[i]
		if !crt.Spec.IsDisabled() && crt.Spec.IsTrainingFrameworkSupported(*tf) {
			if isLauncherType(*crt.Spec.ReplicaType) {
				return &crt.Spec, nil
			}
		}
	}

	return nil, nil
}
