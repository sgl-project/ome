package core

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sgl-project/sgl-ome/pkg/controller/v1beta1/training/runtime"
)

// +kubebuilder:rbac:groups=ome.io,resources=trainingruntimes,verbs=get;list;watch
// +kubebuilder:rbac:groups=ome.io,resources=clustertrainingruntimes,verbs=get;list;watch

func New(ctx context.Context, client client.Client, indexer client.FieldIndexer) (map[string]runtime.Runtime, error) {
	registry := NewRuntimeRegistry()
	runtimes := make(map[string]runtime.Runtime, len(registry))
	for name, factory := range registry {
		r, err := factory(ctx, client, indexer)
		if err != nil {
			return nil, fmt.Errorf("initializing runtime %q: %w", name, err)
		}
		runtimes[name] = r
	}
	return runtimes, nil
}
