package storage

import (
	"context"
	"fmt"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/sgl-project/ome/pkg/logging"
)

// ProvideStorageFactory creates a storage factory with proper logging
// This is the fx provider function for the storage factory
func ProvideStorageFactory(logger logging.Interface) *DefaultFactory {
	// Initialize the global factory with the logger
	InitGlobalFactory(logger)
	return GetGlobalFactory()
}

// ProvideStorage creates a storage instance based on configuration
// This is the fx provider function for creating storage instances
func ProvideStorage(v *viper.Viper, factory *DefaultFactory, logger logging.Interface) (Storage, error) {
	// Extract storage configuration from viper
	config := Config{
		Provider:  Provider(v.GetString("storage.provider")),
		Region:    v.GetString("storage.region"),
		Bucket:    v.GetString("storage.bucket"),
		Endpoint:  v.GetString("storage.endpoint"),
		Namespace: v.GetString("storage.namespace"),
	}

	// Handle auth configuration if present
	if v.IsSet("storage.auth") {
		config.AuthConfig = &AuthConfig{
			Type:  v.GetString("storage.auth.type"),
			Extra: v.GetStringMap("storage.auth.extra"),
		}
	}

	// Validate provider is set
	if config.Provider == "" {
		return nil, fmt.Errorf("storage provider not configured")
	}

	// Create the storage instance
	ctx := context.Background()
	storage, err := factory.CreateStorage(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage provider %s: %w", config.Provider, err)
	}

	logger.WithField("provider", config.Provider).
		WithField("bucket", config.Bucket).
		Info("Storage provider initialized")

	return storage, nil
}

// StorageFactoryModule provides the storage factory as an fx module
var StorageFactoryModule = fx.Provide(
	ProvideStorageFactory,
)

// StorageModule provides both the factory and storage instance as an fx module
var StorageModule = fx.Options(
	StorageFactoryModule,
	fx.Provide(ProvideStorage),
)

// StorageParams defines the fx input struct for components that need storage
type StorageParams struct {
	fx.In

	Storage Storage
	Factory *DefaultFactory `optional:"true"`
	Logger  logging.Interface
}

// MultiStorageParams allows injection of multiple storage instances
type MultiStorageParams struct {
	fx.In

	Storages []Storage `group:"storages"`
	Factory  *DefaultFactory
	Logger   logging.Interface
}

// ProvideStorageWithConfig creates a storage instance with a specific config
// This can be used to create multiple storage instances with different configs
func ProvideStorageWithConfig(config Config) func(*DefaultFactory, logging.Interface) (Storage, error) {
	return func(factory *DefaultFactory, logger logging.Interface) (Storage, error) {
		ctx := context.Background()
		storage, err := factory.CreateStorage(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create storage provider %s: %w", config.Provider, err)
		}

		logger.WithField("provider", config.Provider).
			WithField("bucket", config.Bucket).
			Info("Storage provider initialized from config")

		return storage, nil
	}
}
