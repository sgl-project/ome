package runtime

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	omev1beta1 "github.com/sgl-project/sgl-ome/pkg/apis/ome/v1beta1"
)

type ReconcilerBuilder func(*builder.Builder, client.Client, cache.Cache) *builder.Builder

type Runtime interface {
	NewObjects(ctx context.Context, trainJob *omev1beta1.TrainingJob, vendor *string) ([]client.Object, error)
	TerminalCondition(ctx context.Context, trainJob *omev1beta1.TrainingJob) (*metav1.Condition, error)
	EventHandlerRegistrars() []ReconcilerBuilder
	ValidateObjects(ctx context.Context, old, new *omev1beta1.TrainingJob) (admission.Warnings, field.ErrorList)
}
