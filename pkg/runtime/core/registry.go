package core

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime"
	"context"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Registry map[string]func(ctx context.Context, client client.Client, indexer client.FieldIndexer) (runtime.Runtime, error)

func NewRuntimeRegistry() Registry {
	return Registry{
		TrainingRuntimeName: NewTrainingRuntime,
	}
}
