package runtime

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcilerBuilder func(*builder.Builder, client.Client) *builder.Builder

type Runtime interface {
	NewObjects(ctx context.Context, trainJob *v1beta1.TrainingJob) ([]client.Object, error)

	TerminalCondition(ctx context.Context, trainJob *v1beta1.TrainingJob) (*metav1.Condition, error)
	EventHandlerRegistrars() []ReconcilerBuilder
}
