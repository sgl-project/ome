package plugins

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework"
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime/framework/plugins/jobset"
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Registry map[string]func(ctx context.Context, client client.Client, indexer client.FieldIndexer) (framework.Plugin, error)

func NewRegistry() Registry {
	// Todo: Register more plugins (MPI, torch etc.)
	return Registry{
		jobset.Name: jobset.New,
	}
}
