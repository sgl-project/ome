package storage

import (
	"context"
	"fmt"
	"sync"

	"github.com/sgl-project/ome/pkg/logging"
)

// StorageFactory is a factory function that creates a storage provider
type StorageFactory func(ctx context.Context, config Config, logger logging.Interface) (Storage, error)

// DefaultFactory is the default storage factory implementation
type DefaultFactory struct {
	logger    logging.Interface
	providers map[Provider]StorageFactory
	mu        sync.RWMutex
}

// NewFactory creates a new storage factory
func NewFactory(logger logging.Interface) *DefaultFactory {
	return &DefaultFactory{
		logger:    logger,
		providers: make(map[Provider]StorageFactory),
	}
}

// Register registers a storage provider factory
func (f *DefaultFactory) Register(provider Provider, factory StorageFactory) error {
	// Check if it's a known storage provider
	if provider == "" {
		return fmt.Errorf("invalid storage provider: empty")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.providers[provider]; exists {
		return fmt.Errorf("storage provider %s already registered", provider)
	}

	f.providers[provider] = factory
	f.logger.WithField("provider", provider).Debug("Registered storage provider")
	return nil
}

// CreateStorage creates a storage provider based on configuration
func (f *DefaultFactory) CreateStorage(ctx context.Context, config Config) (Storage, error) {
	if config.Provider == "" {
		return nil, NewError("create", "", string(config.Provider), ErrInvalidConfig)
	}

	f.mu.RLock()
	factory, exists := f.providers[config.Provider]
	f.mu.RUnlock()

	if !exists {
		return nil, NewError("create", "", string(config.Provider),
			fmt.Errorf("storage provider %s not registered", config.Provider))
	}

	// Validate common configuration
	if err := f.validateConfig(config); err != nil {
		return nil, NewError("create", "", string(config.Provider), err)
	}

	// Create the provider
	storage, err := factory(ctx, config, f.logger)
	if err != nil {
		return nil, NewError("create", "", string(config.Provider), err)
	}

	f.logger.WithField("provider", config.Provider).
		WithField("region", config.Region).
		WithField("bucket", config.Bucket).
		Info("Created storage provider")

	return storage, nil
}

// SupportedProviders returns the list of supported storage providers
func (f *DefaultFactory) SupportedProviders() []Provider {
	f.mu.RLock()
	defer f.mu.RUnlock()

	providers := make([]Provider, 0, len(f.providers))
	for p := range f.providers {
		providers = append(providers, p)
	}
	return providers
}

// validateConfig validates common configuration parameters
func (f *DefaultFactory) validateConfig(config Config) error {
	// Validate based on storage provider
	switch config.Provider {
	case ProviderS3, ProviderAzure, ProviderGCS:
		// Cloud storage requires bucket
		if config.Bucket == "" {
			return fmt.Errorf("bucket is required for %s storage", config.Provider)
		}
		// Validate auth config for cloud storage
		if config.AuthConfig == nil {
			return fmt.Errorf("auth configuration is required for %s storage", config.Provider)
		}
	case ProviderOCI:
		// OCI requires namespace and bucket
		if config.Namespace == "" {
			return fmt.Errorf("namespace is required for OCI storage")
		}
		if config.Bucket == "" {
			return fmt.Errorf("bucket is required for OCI storage")
		}
		if config.AuthConfig == nil {
			return fmt.Errorf("auth configuration is required for OCI storage")
		}
	case ProviderPVC:
		// PVC requires specific configuration in Extra
		if config.Extra == nil {
			return fmt.Errorf("PVC configuration is required in Extra field")
		}
	case ProviderLocal:
		// Local storage requires base path in Extra
		if config.Extra == nil || config.Extra["base_path"] == nil {
			return fmt.Errorf("base_path is required for local storage")
		}
	}

	return nil
}

// GlobalFactory is the global storage factory instance
var globalFactory *DefaultFactory
var globalFactoryOnce sync.Once

// GetGlobalFactory returns the global storage factory instance
func GetGlobalFactory(logger logging.Interface) *DefaultFactory {
	globalFactoryOnce.Do(func() {
		globalFactory = NewFactory(logger)
	})
	return globalFactory
}

// MustRegister registers a storage provider factory and panics on error
func MustRegister(provider Provider, factory StorageFactory, logger logging.Interface) {
	if err := GetGlobalFactory(logger).Register(provider, factory); err != nil {
		panic(fmt.Sprintf("failed to register storage provider %s: %v", provider, err))
	}
}
