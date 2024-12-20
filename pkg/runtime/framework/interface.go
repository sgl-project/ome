package framework

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime"
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
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
	EnforcePodGroupPolicy(info *runtime.Info, trainJob *v1beta1.TrainingJob) error
}

type EnforceMLPolicyPlugin interface {
	Plugin
	EnforceMLPolicy(info *runtime.Info, trainJob *v1beta1.TrainingJob) error
}

type CustomValidationPlugin interface {
	Plugin
	Validate(oldObj, newObj *v1beta1.TrainingJob) (admission.Warnings, field.ErrorList)
}

type ComponentBuilderPlugin interface {
	Plugin
	Build(ctx context.Context, runtimeJobTemplate client.Object, info *runtime.Info, trainJob *v1beta1.TrainingJob) (client.Object, error)
}

type TerminalConditionPlugin interface {
	Plugin
	TerminalCondition(ctx context.Context, trainJob *v1beta1.TrainingJob) (*metav1.Condition, error)
}
