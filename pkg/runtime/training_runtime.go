package runtime

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/apis/ome/v1beta1"
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TrainingRuntime struct {
	client client.Client
}

var _ Runtime = (*TrainingRuntime)(nil)

func NewTrainingRuntime(ctx context.Context, c client.Client, indexer client.FieldIndexer) (TrainingRuntime, error) {
	// Todo: Construct runtime resource
	return TrainingRuntime{}, nil
}

func (r *TrainingRuntime) NewObjects(ctx context.Context, trainJob *v1beta1.TrainingJob) ([]client.Object, error) {
	// Todo: Implement NewObjects logic
	return []client.Object{}, nil
}
