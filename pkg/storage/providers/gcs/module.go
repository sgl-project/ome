package gcs

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// ProvideGCSStorage creates a GCS storage provider using viper configuration
// This is the fx provider function specifically for GCS storage
func ProvideGCSStorage(v *viper.Viper, logger logging.Interface) (storage.Storage, error) {
	// Extract GCS-specific configuration from viper
	config := storage.Config{
		Provider: storage.ProviderGCS,
		Bucket:   v.GetString("gcs.bucket"),
		Region:   v.GetString("gcs.location"), // GCS uses location instead of region
		Endpoint: v.GetString("gcs.endpoint"), // For emulator support
	}

	// Add project ID to extra config
	projectID := v.GetString("gcs.project_id")
	if projectID != "" {
		if config.Extra == nil {
			config.Extra = make(map[string]interface{})
		}
		config.Extra["project_id"] = projectID
	}

	// Handle auth configuration
	authType := v.GetString("gcs.auth.type")
	if authType == "" {
		authType = "application_default" // Default to Application Default Credentials
	}

	config.AuthConfig = &storage.AuthConfig{
		Type:  authType,
		Extra: v.GetStringMap("gcs.auth.extra"),
	}

	// Add service account file if provided
	if serviceAccountFile := v.GetString("gcs.auth.service_account_file"); serviceAccountFile != "" {
		if config.AuthConfig.Extra == nil {
			config.AuthConfig.Extra = make(map[string]interface{})
		}
		config.AuthConfig.Extra["service_account_file"] = serviceAccountFile
	}

	// Add GCS-specific options to extra config
	if v.IsSet("gcs.options.storage_class") {
		if config.Extra == nil {
			config.Extra = make(map[string]interface{})
		}
		config.Extra["storage_class"] = v.GetString("gcs.options.storage_class")
	}

	if v.IsSet("gcs.options.uniform_bucket_access") {
		if config.Extra == nil {
			config.Extra = make(map[string]interface{})
		}
		config.Extra["uniform_bucket_access"] = v.GetBool("gcs.options.uniform_bucket_access")
	}

	if v.IsSet("gcs.options.enable_versioning") {
		if config.Extra == nil {
			config.Extra = make(map[string]interface{})
		}
		config.Extra["enable_versioning"] = v.GetBool("gcs.options.enable_versioning")
	}

	// Validate required fields
	if config.Bucket == "" {
		return nil, fmt.Errorf("GCS bucket not configured")
	}

	// Create the GCS provider
	ctx := context.Background()
	provider, err := NewGCSProvider(ctx, config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS storage provider: %w", err)
	}

	logger.WithField("provider", "gcs").
		WithField("bucket", config.Bucket).
		WithField("project", projectID).
		WithField("location", config.Region).
		Info("GCS storage provider initialized")

	return provider, nil
}

// GCSStorageModule is an fx module that provides GCS storage
var GCSStorageModule = fx.Provide(
	ProvideGCSStorage,
)

// GCSConfig represents GCS-specific configuration
type GCSConfig struct {
	ProjectID           string
	Bucket              string
	Location            string
	Endpoint            string // For emulator support
	AuthType            string
	ServiceAccountFile  string
	StorageClass        string
	UniformBucketAccess bool
	EnableVersioning    bool
}

// ProvideGCSStorageWithConfig creates a GCS storage provider with explicit config
// This is useful for testing or when configuration comes from sources other than viper
func ProvideGCSStorageWithConfig(config GCSConfig) func(logging.Interface) (storage.Storage, error) {
	return func(logger logging.Interface) (storage.Storage, error) {
		storageConfig := storage.Config{
			Provider: storage.ProviderGCS,
			Bucket:   config.Bucket,
			Region:   config.Location,
			Endpoint: config.Endpoint,
		}

		// Add project ID to extra config
		if config.ProjectID != "" {
			storageConfig.Extra = map[string]interface{}{
				"project_id": config.ProjectID,
			}
		}

		// Add GCS-specific options
		if config.StorageClass != "" {
			if storageConfig.Extra == nil {
				storageConfig.Extra = make(map[string]interface{})
			}
			storageConfig.Extra["storage_class"] = config.StorageClass
		}

		if config.UniformBucketAccess {
			if storageConfig.Extra == nil {
				storageConfig.Extra = make(map[string]interface{})
			}
			storageConfig.Extra["uniform_bucket_access"] = config.UniformBucketAccess
		}

		if config.EnableVersioning {
			if storageConfig.Extra == nil {
				storageConfig.Extra = make(map[string]interface{})
			}
			storageConfig.Extra["enable_versioning"] = config.EnableVersioning
		}

		// Configure authentication
		if config.AuthType != "" {
			storageConfig.AuthConfig = &storage.AuthConfig{
				Type: config.AuthType,
			}

			if config.ServiceAccountFile != "" {
				storageConfig.AuthConfig.Extra = map[string]interface{}{
					"service_account_file": config.ServiceAccountFile,
				}
			}
		}

		ctx := context.Background()
		provider, err := NewGCSProvider(ctx, storageConfig, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create GCS storage provider: %w", err)
		}

		return provider, nil
	}
}

// GCSStorageParams defines the fx input struct for components that need GCS storage
type GCSStorageParams struct {
	fx.In

	Storage storage.Storage
	Logger  logging.Interface
}

// GCSStorageResult defines the fx output struct for the GCS storage provider
type GCSStorageResult struct {
	fx.Out

	Storage storage.Storage
}
