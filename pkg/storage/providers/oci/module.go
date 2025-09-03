package oci

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// ProvideOCIStorage creates an OCI storage provider using viper configuration
// This is the fx provider function specifically for OCI storage
func ProvideOCIStorage(v *viper.Viper, logger logging.Interface) (storage.Storage, error) {
	// Extract OCI-specific configuration from viper
	config := storage.Config{
		Provider:  storage.ProviderOCI,
		Region:    v.GetString("oci.region"),
		Bucket:    v.GetString("oci.bucket"),
		Namespace: v.GetString("oci.namespace"),
		Endpoint:  v.GetString("oci.endpoint"),
	}

	// Handle auth configuration
	authType := v.GetString("oci.auth.type")
	if authType == "" {
		authType = "instance_principal" // Default to instance principal
	}

	config.AuthConfig = &storage.AuthConfig{
		Type:  authType,
		Extra: v.GetStringMap("oci.auth.extra"),
	}

	// Validate required fields
	if config.Bucket == "" {
		return nil, fmt.Errorf("OCI bucket not configured")
	}

	// Create the OCI provider
	ctx := context.Background()
	provider, err := NewOCIProvider(ctx, config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI storage provider: %w", err)
	}

	logger.WithField("provider", "oci").
		WithField("bucket", config.Bucket).
		WithField("namespace", config.Namespace).
		WithField("region", config.Region).
		Info("OCI storage provider initialized")

	return provider, nil
}

// OCIStorageModule is an fx module that provides OCI storage
var OCIStorageModule = fx.Provide(
	ProvideOCIStorage,
)

// OCIConfig represents OCI-specific configuration
type OCIConfig struct {
	Region    string
	Bucket    string
	Namespace string
	Endpoint  string
	AuthType  string
}

// ProvideOCIStorageWithConfig creates an OCI storage provider with explicit config
// This is useful for testing or when configuration comes from sources other than viper
func ProvideOCIStorageWithConfig(config OCIConfig) func(logging.Interface) (storage.Storage, error) {
	return func(logger logging.Interface) (storage.Storage, error) {
		storageConfig := storage.Config{
			Provider:  storage.ProviderOCI,
			Region:    config.Region,
			Bucket:    config.Bucket,
			Namespace: config.Namespace,
			Endpoint:  config.Endpoint,
		}

		if config.AuthType != "" {
			storageConfig.AuthConfig = &storage.AuthConfig{
				Type: config.AuthType,
			}
		}

		ctx := context.Background()
		provider, err := NewOCIProvider(ctx, storageConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create OCI storage provider: %w", err)
		}

		return provider, nil
	}
}

// OCIStorageParams defines the fx input struct for components that need OCI storage
type OCIStorageParams struct {
	fx.In

	Storage storage.Storage
	Logger  logging.Interface
}
