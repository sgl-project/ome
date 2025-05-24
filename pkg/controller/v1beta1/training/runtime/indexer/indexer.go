package indexer

import (
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
)

const (
	TrainJobRuntimeRefKey        = ".spec.runtimeRef.kind=trainingRuntime"
	TrainJobClusterRuntimeRefKey = ".spec.runtimeRef.kind=clusterTrainingRuntime"
)

func IndexTrainJobTrainingRuntime(obj client.Object) []string {
	trainJob, ok := obj.(*omev1beta1.TrainingJob)
	if !ok {
		return nil
	}
	if ptr.Deref(trainJob.Spec.RuntimeRef.APIGroup, "") == omev1beta1.SchemeGroupVersion.Group &&
		ptr.Deref(trainJob.Spec.RuntimeRef.Kind, "") == omev1beta1.TrainingRuntimeKind {
		return []string{trainJob.Spec.RuntimeRef.Name}
	}
	return nil
}

func IndexTrainJobClusterTrainingRuntime(obj client.Object) []string {
	trainJob, ok := obj.(*omev1beta1.TrainingJob)
	if !ok {
		return nil
	}
	if ptr.Deref(trainJob.Spec.RuntimeRef.APIGroup, "") == omev1beta1.SchemeGroupVersion.Group &&
		ptr.Deref(trainJob.Spec.RuntimeRef.Kind, "") == omev1beta1.ClusterTrainingRuntimeKind {
		return []string{trainJob.Spec.RuntimeRef.Name}
	}
	return nil
}
