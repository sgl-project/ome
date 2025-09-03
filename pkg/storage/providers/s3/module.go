package s3

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// ProvideS3Storage creates an S3 storage provider using viper configuration
// This is the fx provider function specifically for S3 storage
func ProvideS3Storage(v *viper.Viper, logger logging.Interface) (storage.Storage, error) {
	// Extract S3-specific configuration from viper
	config := storage.Config{
		Provider: storage.ProviderS3,
		Region:   v.GetString("s3.region"),
		Bucket:   v.GetString("s3.bucket"),
		Endpoint: v.GetString("s3.endpoint"),
	}

	// Handle auth configuration
	authType := v.GetString("s3.auth.type")
	if authType == "" {
		authType = "default" // Default to AWS SDK credential chain
	}

	config.AuthConfig = &storage.AuthConfig{
		Type:  authType,
		Extra: v.GetStringMap("s3.auth.extra"),
	}

	// Validate required fields
	if config.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket not configured")
	}

	// Create the S3 provider
	ctx := context.Background()
	provider, err := NewS3Provider(ctx, config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 storage provider: %w", err)
	}

	logger.WithField("provider", "s3").
		WithField("bucket", config.Bucket).
		WithField("region", config.Region).
		Info("S3 storage provider initialized")

	return provider, nil
}

// S3StorageModule is an fx module that provides S3 storage
var S3StorageModule = fx.Provide(
	ProvideS3Storage,
)

// S3Config represents S3-specific configuration
type S3Config struct {
	Region         string
	Bucket         string
	Endpoint       string
	AuthType       string
	ForcePathStyle bool
	DisableSSL     bool
	UseAccelerate  bool
}

// ProvideS3StorageWithConfig creates an S3 storage provider with explicit config
// This is useful for testing or when configuration comes from sources other than viper
func ProvideS3StorageWithConfig(config S3Config) func(logging.Interface) (storage.Storage, error) {
	return func(logger logging.Interface) (storage.Storage, error) {
		storageConfig := storage.Config{
			Provider: storage.ProviderS3,
			Region:   config.Region,
			Bucket:   config.Bucket,
			Endpoint: config.Endpoint,
		}

		if config.AuthType != "" {
			storageConfig.AuthConfig = &storage.AuthConfig{
				Type: config.AuthType,
			}
		}

		ctx := context.Background()
		provider, err := NewS3Provider(ctx, storageConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create S3 storage provider: %w", err)
		}

		return provider, nil
	}
}

// S3StorageParams defines the fx input struct for components that need S3 storage
type S3StorageParams struct {
	fx.In

	Storage storage.Storage
	Logger  logging.Interface
}
