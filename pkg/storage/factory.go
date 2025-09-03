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

	// Only log if logger is available
	if f.logger != nil {
		f.logger.WithField("provider", provider).Debug("Registered storage provider")
	}
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

	// Use a discard logger if none is set
	logger := f.logger
	if logger == nil {
		logger = logging.Discard()
	}

	// Create the provider
	storage, err := factory(ctx, config, logger)
	if err != nil {
		return nil, NewError("create", "", string(config.Provider), err)
	}

	// Log if we have a logger
	if f.logger != nil {
		f.logger.WithField("provider", config.Provider).
			WithField("region", config.Region).
			WithField("bucket", config.Bucket).
			Info("Created storage provider")
	}

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
		// OCI can auto-detect namespace but requires bucket
		// Namespace can be empty - the provider will auto-detect it
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

// InitGlobalFactory initializes the global factory with a logger
// Call this at application startup if you want logging
func InitGlobalFactory(logger logging.Interface) {
	globalFactoryOnce.Do(func() {
		globalFactory = NewFactory(logger)
	})
	// If factory was already created, update its logger
	if globalFactory != nil && logger != nil {
		globalFactory.mu.Lock()
		globalFactory.logger = logger
		globalFactory.mu.Unlock()
	}
}

// GetGlobalFactory returns the global storage factory instance
// If not initialized with InitGlobalFactory, it creates one without a logger
func GetGlobalFactory() *DefaultFactory {
	globalFactoryOnce.Do(func() {
		// Create with a discard logger by default
		// This will be replaced when InitGlobalFactory is called with a proper logger
		globalFactory = NewFactory(logging.Discard())
	})
	return globalFactory
}

// MustRegister registers a storage provider factory and panics on error
// This is designed to be called from init() functions
func MustRegister(provider Provider, factory StorageFactory) {
	if err := GetGlobalFactory().Register(provider, factory); err != nil {
		panic(fmt.Sprintf("failed to register storage provider %s: %v", provider, err))
	}
}
