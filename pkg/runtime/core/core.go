package core

import (
	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/runtime"
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
