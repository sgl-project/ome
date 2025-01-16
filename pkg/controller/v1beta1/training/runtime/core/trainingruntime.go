package core

import (
	"context"
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime"
	fwkcore "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/framework/core"
	fwkplugins "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/framework/plugins"
	idxer "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/indexer"
)

var (
	errorNotFoundSpecifiedTrainingRuntime = errors.New("TrainingRuntime specified in TrainJob is not found")
)

type TrainingRuntime struct {
	framework *fwkcore.Framework
	client    client.Client
}

var TrainingRuntimeGroupKind = schema.GroupKind{
	Group: omev1beta1.SchemeGroupVersion.Group,
	Kind:  omev1beta1.TrainingRuntimeKind,
}.String()

var _ runtime.Runtime = (*TrainingRuntime)(nil)

var trainingRuntimeFactory *TrainingRuntime

func NewTrainingRuntime(ctx context.Context, c client.Client, indexer client.FieldIndexer) (runtime.Runtime, error) {
	if err := indexer.IndexField(ctx, &omev1beta1.TrainingJob{}, idxer.TrainJobRuntimeRefKey, idxer.IndexTrainJobTrainingRuntime); err != nil {
		return nil, fmt.Errorf("setting index on TrainingRuntime for TrainJob: %w", err)
	}
	if err := indexer.IndexField(ctx, &omev1beta1.TrainingJob{}, idxer.TrainJobClusterRuntimeRefKey, idxer.IndexTrainJobClusterTrainingRuntime); err != nil {
		return nil, fmt.Errorf("setting index on ClusterTrainingRuntime for TrainJob: %w", err)
	}
	fwk, err := fwkcore.New(ctx, c, fwkplugins.NewRegistry(), indexer)
	if err != nil {
		return nil, err
	}
	trainingRuntimeFactory = &TrainingRuntime{
		framework: fwk,
		client:    c,
	}
	return trainingRuntimeFactory, nil
}

func (r *TrainingRuntime) NewObjects(ctx context.Context, trainJob *omev1beta1.TrainingJob) ([]client.Object, error) {
	var trainingRuntime omev1beta1.TrainingRuntime
	err := r.client.Get(ctx, client.ObjectKey{Namespace: trainJob.Namespace, Name: trainJob.Spec.RuntimeRef.Name}, &trainingRuntime)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errorNotFoundSpecifiedTrainingRuntime, err)
	}
	return r.buildObjects(ctx, trainJob, trainingRuntime.Spec.Template, trainingRuntime.Spec.MLPolicy, trainingRuntime.Spec.PodGroupPolicy)
}

func (r *TrainingRuntime) buildObjects(
	ctx context.Context, trainJob *omev1beta1.TrainingJob, jobSetTemplateSpec omev1beta1.JobSetTemplateSpec, mlPolicy *omev1beta1.MLPolicy, podGroupPolicy *omev1beta1.PodGroupPolicy,
) ([]client.Object, error) {
	propagationLabels := jobSetTemplateSpec.Labels
	if propagationLabels == nil && trainJob.Spec.Labels != nil {
		propagationLabels = make(map[string]string, len(trainJob.Spec.Labels))
	}
	for k, v := range trainJob.Spec.Labels {
		// The JobSetTemplateSpec labels are overridden by the TrainJob Labels (.spec.labels).
		propagationLabels[k] = v
	}
	propagationAnnotations := jobSetTemplateSpec.Annotations
	if propagationAnnotations == nil && trainJob.Spec.Annotations != nil {
		propagationAnnotations = make(map[string]string, len(trainJob.Spec.Annotations))
	}
	for k, v := range trainJob.Spec.Annotations {
		// The JobSetTemplateSpec annotations are overridden by the TrainJob Annotations (.spec.annotations).
		propagationAnnotations[k] = v
	}
	opts := []runtime.InfoOption{
		runtime.WithLabels(propagationLabels),
		runtime.WithAnnotations(propagationAnnotations),
		runtime.WithMLPolicy(mlPolicy),
		runtime.WithPodGroupPolicy(podGroupPolicy),
	}

	for _, rJob := range jobSetTemplateSpec.Spec.ReplicatedJobs {
		// By default every ReplicatedJob has only 1 replica.
		opts = append(opts, runtime.WithPodSpecReplicas(rJob.Name, 1, rJob.Template.Spec.Template.Spec))
	}

	info := runtime.NewInfo(opts...)

	if err := r.framework.RunEnforceMLPolicyPlugins(info, trainJob); err != nil {
		return nil, err
	}

	if err := r.framework.RunEnforcePodGroupPolicyPlugins(info, trainJob); err != nil {
		return nil, err
	}

	jobSetTemplate := jobsetv1alpha2.JobSet{
		Spec: jobSetTemplateSpec.Spec,
	}

	return r.framework.RunComponentBuilderPlugins(ctx, jobSetTemplate.DeepCopy(), info, trainJob)
}

func (r *TrainingRuntime) TerminalCondition(ctx context.Context, trainJob *omev1beta1.TrainingJob) (*metav1.Condition, error) {
	return r.framework.RunTerminalConditionPlugins(ctx, trainJob)
}

func (r *TrainingRuntime) EventHandlerRegistrars() []runtime.ReconcilerBuilder {
	var builders []runtime.ReconcilerBuilder
	for _, ex := range r.framework.WatchExtensionPlugins() {
		builders = append(builders, ex.ReconcilerBuilders()...)
	}
	return builders
}

func (r *TrainingRuntime) ValidateObjects(ctx context.Context, old, new *omev1beta1.TrainingJob) (admission.Warnings, field.ErrorList) {
	if err := r.client.Get(ctx, client.ObjectKey{
		Namespace: old.Namespace,
		Name:      old.Spec.RuntimeRef.Name,
	}, &omev1beta1.TrainingRuntime{}); err != nil {
		return nil, field.ErrorList{
			field.Invalid(field.NewPath("spec", "runtimeRef"), old.Spec.RuntimeRef,
				fmt.Sprintf("%v: specified trainingRuntime must be created before the TrainJob is created", err)),
		}
	}
	return r.framework.RunCustomValidationPlugins(old, new)
}
