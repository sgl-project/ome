package plugins

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/framework"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/framework/plugins/coscheduling"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/framework/plugins/jobset"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/framework/plugins/mpi"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/framework/plugins/plainml"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime/framework/plugins/torch"
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
