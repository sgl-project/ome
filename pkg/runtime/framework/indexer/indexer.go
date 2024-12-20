package indexer

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	TrainJobRuntimeRefKey        = ".spec.runtimeRef.kind=trainingRuntime"
	TrainJobClusterRuntimeRefKey = ".spec.runtimeRef.kind=clusterTrainingRuntime"
)

func IndexTrainJobTrainingRuntime(obj client.Object) []string {
	trainJob, ok := obj.(*v1beta1.TrainingJob)
	if !ok {
		return nil
	}

	return []string{*trainJob.Spec.Trainer.Runtime}
}
