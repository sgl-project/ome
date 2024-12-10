package runtime

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"context"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ReconcilerBuilder func(*builder.Builder, client.Client) *builder.Builder

type Runtime interface {
	NewObjects(ctx context.Context, trainJob *v1beta1.TrainingJob) ([]client.Object, error)
}
