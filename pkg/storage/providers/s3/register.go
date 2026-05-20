package s3

import (
	"context"

	"sigs.k8s.io/ome/pkg/logging"
	"sigs.k8s.io/ome/pkg/storage"
)

func init() {
	// Register S3 provider with the global factory
	// This will be called when the package is imported
	storage.MustRegister(storage.ProviderS3, func(ctx context.Context, config storage.Config, logger logging.Interface) (storage.Storage, error) {
		return NewS3Provider(ctx, config, logger)
	})
}
