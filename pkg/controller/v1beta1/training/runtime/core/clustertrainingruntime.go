package core

import (
	"context"
	"errors"
	"fmt"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	errorNotFoundSpecifiedClusterTrainingRuntime = errors.New("not found ClusterTrainingRuntime specified in TrainJob")
)

type ClusterTrainingRuntime struct {
	*TrainingRuntime
}

var _ runtime.Runtime = (*ClusterTrainingRuntime)(nil)

var ClusterTrainingRuntimeGroupKind = schema.GroupKind{
	Group: omev1beta1.SchemeGroupVersion.Group,
	Kind:  omev1beta1.ClusterTrainingRuntimeKind,
}.String()

func NewClusterTrainingRuntime(context.Context, client.Client, client.FieldIndexer) (runtime.Runtime, error) {
	return &ClusterTrainingRuntime{
		TrainingRuntime: trainingRuntimeFactory,
	}, nil
}

func (r *ClusterTrainingRuntime) NewObjects(ctx context.Context, trainJob *omev1beta1.TrainingJob, vendor *string) ([]client.Object, error) {
	clTrainingRuntime := &omev1beta1.ClusterTrainingRuntime{}
	if err := r.client.Get(ctx, client.ObjectKey{Name: trainJob.Spec.RuntimeRef.Name}, clTrainingRuntime); err != nil {
		return nil, fmt.Errorf("%w: %w", errorNotFoundSpecifiedClusterTrainingRuntime, err)
	}
	return r.buildObjects(ctx, trainJob, clTrainingRuntime.Spec.Template, clTrainingRuntime.Spec.MLPolicy, clTrainingRuntime.Spec.PodGroupPolicy, vendor)
}

func (r *ClusterTrainingRuntime) TerminalCondition(ctx context.Context, trainJob *omev1beta1.TrainingJob) (*metav1.Condition, error) {
	return r.TrainingRuntime.TerminalCondition(ctx, trainJob)
}

func (r *ClusterTrainingRuntime) EventHandlerRegistrars() []runtime.ReconcilerBuilder {
	return nil
}

func (r *ClusterTrainingRuntime) ValidateObjects(ctx context.Context, old, new *omev1beta1.TrainingJob) (admission.Warnings, field.ErrorList) {
	if err := r.client.Get(ctx, client.ObjectKey{
		Namespace: old.Namespace,
		Name:      old.Spec.RuntimeRef.Name,
	}, &omev1beta1.ClusterTrainingRuntime{}); err != nil {
		return nil, field.ErrorList{
			field.Invalid(field.NewPath("spec", "RuntimeRef"), old.Spec.RuntimeRef,
				fmt.Sprintf("%v: specified clusterTrainingRuntime must be created before the TrainJob is created", err)),
		}
	}
	return r.framework.RunCustomValidationPlugins(old, new)
}
