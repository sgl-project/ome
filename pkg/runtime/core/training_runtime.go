package core

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework/core"
	frameworkcore "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework/core"
	trainingindexer "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework/indexer"
	fwkplugins "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework/plugins"
	"context"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"
)

type TrainingRuntime struct {
	client    client.Client
	Log       logr.Logger
	framework *core.Framework
}

var _ runtime.Runtime = (*TrainingRuntime)(nil)

var trainingRuntimeFactory *TrainingRuntime

var TrainingRuntimeName = "TrainingRuntime"

func (r *TrainingRuntime) TerminalCondition(ctx context.Context, trainJob *v1beta1.TrainingJob) (*metav1.Condition, error) {
	return r.framework.RunTerminalConditionPlugins(ctx, trainJob)
}

func NewTrainingRuntime(ctx context.Context, c client.Client, indexer client.FieldIndexer) (runtime.Runtime, error) {
	if err := indexer.IndexField(ctx, &v1beta1.TrainingJob{}, trainingindexer.TrainJobRuntimeRefKey, trainingindexer.IndexTrainJobTrainingRuntime); err != nil {
		return nil, errors.Wrapf(err, "Error setting index on TrainingRuntime for TrainJob")
	}

	fwk, err := frameworkcore.New(ctx, c, fwkplugins.NewRegistry(), indexer)
	if err != nil {
		return nil, err
	}
	trainingRuntimeFactory = &TrainingRuntime{
		framework: fwk,
		client:    c,
	}
	return trainingRuntimeFactory, nil
}

func (r *TrainingRuntime) NewObjects(ctx context.Context, trainJob *v1beta1.TrainingJob) ([]client.Object, error) {
	var trainingRuntime v1beta1.TrainingRuntime
	err := r.client.Get(ctx, client.ObjectKey{Namespace: trainJob.Namespace, Name: *trainJob.Spec.Trainer.Runtime}, &trainingRuntime)
	if err != nil {
		r.Log.Error(err, "Error getting TrainingRuntime", "namespace")
		return nil, errors.Wrapf(err, "TrainingRuntime specified in TrainJob is not found")
	}
	return r.buildObjects(ctx, trainJob, trainingRuntime.Spec.Template, trainingRuntime.Spec.MLPolicy, trainingRuntime.Spec.PodGroupPolicy)
}

func (r *TrainingRuntime) buildObjects(ctx context.Context, trainJob *v1beta1.TrainingJob, jobSetTemplateSpec v1beta1.JobSetTemplateSpec, mlPolicy *v1beta1.MLPolicy, podGroupPolicy *v1beta1.PodGroupPolicy) ([]client.Object, error) {
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
		// Every ReplicatedJob has only 1 replica by default.
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

func (r *TrainingRuntime) EventHandlerRegistrars() []runtime.ReconcilerBuilder {
	var builders []runtime.ReconcilerBuilder
	for _, ex := range r.framework.WatchExtensionPlugins() {
		builders = append(builders, ex.ReconcilerBuilders()...)
	}
	return builders
}
