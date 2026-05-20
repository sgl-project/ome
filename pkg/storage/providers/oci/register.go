package oci

import (
	"context"

	"sigs.k8s.io/ome/pkg/logging"
	"sigs.k8s.io/ome/pkg/storage"
)

func init() {
	// Register OCI provider with the global factory
	// This will be called when the package is imported
	storage.MustRegister(storage.ProviderOCI, func(ctx context.Context, config storage.Config, logger logging.Interface) (storage.Storage, error) {
		return NewOCIProvider(ctx, config, logger)
	})
}
