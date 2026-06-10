package jobset

import (
	"context"
	"fmt"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/utils"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	jobsetv1alpha2 "sigs.k8s.io/jobset/api/jobset/v1alpha2"

	omev1beta1 "bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/constants"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/framework"
)

type JobSet struct {
	client     client.Client
	restMapper meta.RESTMapper
	scheme     *apiruntime.Scheme
	logger     logr.Logger
}

var _ framework.WatchExtensionPlugin = (*JobSet)(nil)
var _ framework.ComponentBuilderPlugin = (*JobSet)(nil)
var _ framework.TerminalConditionPlugin = (*JobSet)(nil)

const Name = constants.JobSetKind

// +kubebuilder:rbac:groups=jobset.x-k8s.io,resources=jobsets,verbs=get;list;watch;create

func New(ctx context.Context, c client.Client, _ client.FieldIndexer) (framework.Plugin, error) {
	return &JobSet{
		client:     c,
		restMapper: c.RESTMapper(),
		scheme:     c.Scheme(),
		logger:     ctrl.LoggerFrom(ctx).WithValues("pluginName", constants.JobSetKind),
	}, nil
}

func (j *JobSet) Name() string {
	return Name
}

func (j *JobSet) Build(ctx context.Context, runtimeJobTemplate client.Object, info *runtime.Info, trainJob *omev1beta1.TrainingJob) (client.Object, error) {
	if runtimeJobTemplate == nil || info == nil || trainJob == nil {
		return nil, fmt.Errorf("runtime info or object is missing")
	}

	raw, ok := runtimeJobTemplate.(*jobsetv1alpha2.JobSet)
	if !ok {
		return nil, nil
	}

	// TODO: Add support for the PodSpecOverride for other pod specs.
	// TODO: Refactor the builder with wrappers for PodSpec.
	jobSet := NewBuilder(client.ObjectKeyFromObject(trainJob), omev1beta1.JobSetTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      info.Labels,
			Annotations: info.Annotations,
		},
		Spec: raw.Spec,
	}).
		Trainer(info, trainJob).
		PodLabels(info.PodLabels).
		// Todo: We only support single job training (Trainer Job) for now. Support multi-job in the future.
		Initializer(trainJob).
		Suspend(trainJob.Spec.Suspend).
		Build()

	// Set label for integration with Kueue
	if kueueEnabled, ok := trainJob.Annotations[constants.KueueEnabledLabelKey]; ok && kueueEnabled == "true" {
		jobSet.Labels[constants.KueueQueueLabelKey] = trainJob.Namespace
		jobSet.Labels[constants.KueueWorkloadPriorityClassLabelKey] = constants.DedicatedAiClusterPreemptionWorkloadPriorityClass
	}

	if err := ctrlutil.SetControllerReference(trainJob, jobSet, j.scheme); err != nil {
		return nil, err
	}

	return jobSet, nil
}

func (j *JobSet) TerminalCondition(ctx context.Context, trainJob *omev1beta1.TrainingJob) (*metav1.Condition, error) {
	jobSet := &jobsetv1alpha2.JobSet{}
	jobSetName := utils.GetShortTrainJobName(trainJob.Name)
	if err := j.client.Get(ctx, client.ObjectKey{Name: jobSetName, Namespace: trainJob.Namespace}, jobSet); err != nil {
		return nil, err
	}
	if completed := meta.FindStatusCondition(jobSet.Status.Conditions, string(jobsetv1alpha2.JobSetCompleted)); completed != nil && completed.Status == metav1.ConditionTrue {
		completed.Type = omev1beta1.TrainJobComplete
		return completed, nil
	}
	if failed := meta.FindStatusCondition(jobSet.Status.Conditions, string(jobsetv1alpha2.JobSetFailed)); failed != nil && failed.Status == metav1.ConditionTrue {
		failed.Type = omev1beta1.TrainJobFailed
		return failed, nil
	}
	return nil, nil
}

func (j *JobSet) ReconcilerBuilders() []runtime.ReconcilerBuilder {
	if _, err := j.restMapper.RESTMapping(
		schema.GroupKind{Group: jobsetv1alpha2.GroupVersion.Group, Kind: constants.JobSetKind},
		jobsetv1alpha2.SchemeGroupVersion.Version,
	); err != nil {
		// TODO (tenzen-y): After we provide the Configuration API, we should return errors based on the enabled plugins.
		j.logger.Error(err, "JobSet CRDs must be installed in advance")
	}
	return []runtime.ReconcilerBuilder{
		func(b *builder.Builder, c client.Client, cache cache.Cache) *builder.Builder {
			return b.Owns(&jobsetv1alpha2.JobSet{})
		},
	}
}
