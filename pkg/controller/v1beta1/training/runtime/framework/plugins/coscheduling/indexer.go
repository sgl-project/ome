package coscheduling

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
)

var (
	TrainingRuntimeContainerRuntimeClassKey        = ".trainingRuntimeSpec.jobSetTemplateSpec.replicatedJobs.podTemplateSpec.runtimeClassName"
	ClusterTrainingRuntimeContainerRuntimeClassKey = ".clusterTrainingRuntimeSpec.jobSetTemplateSpec.replicatedJobs.podTemplateSpec.runtimeClassName"
)

func IndexTrainingRuntimeContainerRuntimeClass(obj client.Object) []string {
	runtime, ok := obj.(*omev1beta1.TrainingRuntime)
	if !ok {
		return nil
	}
	var runtimeClasses []string
	for _, rJob := range runtime.Spec.Template.Spec.ReplicatedJobs {
		if rJob.Template.Spec.Template.Spec.RuntimeClassName != nil {
			runtimeClasses = append(runtimeClasses, *rJob.Template.Spec.Template.Spec.RuntimeClassName)
		}
	}
	return runtimeClasses
}

func IndexClusterTrainingRuntimeContainerRuntimeClass(obj client.Object) []string {
	clRuntime, ok := obj.(*omev1beta1.ClusterTrainingRuntime)
	if !ok {
		return nil
	}
	var runtimeClasses []string
	for _, rJob := range clRuntime.Spec.Template.Spec.ReplicatedJobs {
		if rJob.Template.Spec.Template.Spec.RuntimeClassName != nil {
			runtimeClasses = append(runtimeClasses, *rJob.Template.Spec.Template.Spec.RuntimeClassName)
		}
	}
	return runtimeClasses
}
