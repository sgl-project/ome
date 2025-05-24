package plugins

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/coscheduling"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/jobset"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/mpi"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/plainml"
	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime/framework/plugins/torch"
)

type Registry map[string]func(ctx context.Context, client client.Client, indexer client.FieldIndexer) (framework.Plugin, error)

func NewRegistry() Registry {
	return Registry{
		coscheduling.Name: coscheduling.New,
		mpi.Name:          mpi.New,
		plainml.Name:      plainml.New,
		torch.Name:        torch.New,
		jobset.Name:       jobset.New,
	}
}
