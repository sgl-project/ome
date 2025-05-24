package framework

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
)

type Plugin interface {
	Name() string
}

type WatchExtensionPlugin interface {
	Plugin
	ReconcilerBuilders() []runtime.ReconcilerBuilder
}

type EnforcePodGroupPolicyPlugin interface {
	Plugin
	EnforcePodGroupPolicy(info *runtime.Info, trainJob *omev1beta1.TrainingJob) error
}

type EnforceMLPolicyPlugin interface {
	Plugin
	EnforceMLPolicy(info *runtime.Info, trainJob *omev1beta1.TrainingJob) error
}

type CustomValidationPlugin interface {
	Plugin
	Validate(oldObj, newObj *omev1beta1.TrainingJob) (admission.Warnings, field.ErrorList)
}

type ComponentBuilderPlugin interface {
	Plugin
	Build(ctx context.Context, runtimeJobTemplate client.Object, info *runtime.Info, trainJob *omev1beta1.TrainingJob) (client.Object, error)
}

type TerminalConditionPlugin interface {
	Plugin
	TerminalCondition(ctx context.Context, trainJob *omev1beta1.TrainingJob) (*metav1.Condition, error)
}
