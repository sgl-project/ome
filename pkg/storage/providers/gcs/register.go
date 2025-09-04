package gcs

import (
	"context"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

func init() {
	// Register GCS provider with the global factory
	// This will be called when the package is imported
	storage.MustRegister(storage.ProviderGCS, func(ctx context.Context, config storage.Config, logger logging.Interface) (storage.Storage, error) {
		return NewGCSProvider(ctx, config, logger)
	})
}
