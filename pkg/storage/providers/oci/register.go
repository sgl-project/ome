package oci

import (
	"context"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

func init() {
	// Register OCI provider with the global factory
	// This will be called when the package is imported
	storage.MustRegister(storage.ProviderOCI, func(ctx context.Context, config storage.Config, logger logging.Interface) (storage.Storage, error) {
		return NewOCIProvider(ctx, config, logger)
	})
}
