package core

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/controller/v1beta1/training/runtime"
)

type Registry map[string]func(ctx context.Context, client client.Client, indexer client.FieldIndexer) (runtime.Runtime, error)

func NewRuntimeRegistry() Registry {
	return Registry{
		TrainingRuntimeGroupKind:        NewTrainingRuntime,
		ClusterTrainingRuntimeGroupKind: NewClusterTrainingRuntime,
	}
}
