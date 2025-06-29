package storage

import (
	"context"
	"fmt"
	"sync"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// DefaultFactory is the default storage factory implementation
type DefaultFactory struct {
	mu          sync.RWMutex
	providers   map[Provider]ProviderStorageFactory
	authFactory auth.Factory
	logger      logging.Interface
}

// ProviderStorageFactory creates storage instances for a specific provider
type ProviderStorageFactory interface {
	Create(ctx context.Context, config interface{}, credentials auth.Credentials) (Storage, error)
}

// NewDefaultFactory creates a new default factory
func NewDefaultFactory(authFactory auth.Factory, logger logging.Interface) *DefaultFactory {
	f := &DefaultFactory{
		providers:   make(map[Provider]ProviderStorageFactory),
		authFactory: authFactory,
		logger:      logger,
	}

	// Providers should be registered externally to avoid import cycles
	// Example:
	// factory.RegisterProvider(storage.ProviderOCI, oci.NewFactory(logger))

	return f
}

// RegisterProvider registers a storage provider factory
func (f *DefaultFactory) RegisterProvider(provider Provider, factory ProviderStorageFactory) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[provider] = factory
}

// Create creates a storage instance for the given provider
func (f *DefaultFactory) Create(ctx context.Context, provider Provider, config interface{}) (Storage, error) {
	f.mu.RLock()
	factory, exists := f.providers[provider]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unsupported storage provider: %s", provider)
	}

	// Extract auth config from storage config
	authConfig, err := extractAuthConfig(provider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to extract auth config: %w", err)
	}

	// Create credentials
	credentials, err := f.authFactory.Create(ctx, authConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create credentials: %w", err)
	}

	f.logger.WithField("provider", provider).Info("Creating storage instance")

	// Create storage instance
	return factory.Create(ctx, config, credentials)
}

// extractAuthConfig extracts auth configuration from storage config
func extractAuthConfig(provider Provider, config interface{}) (auth.Config, error) {
	// This is a simplified version - in practice, each provider's config
	// would have its own structure with auth configuration embedded
	type AuthConfigExtractor interface {
		GetAuthConfig() auth.Config
	}

	if extractor, ok := config.(AuthConfigExtractor); ok {
		return extractor.GetAuthConfig(), nil
	}

	// Default auth config
	return auth.Config{
		Provider: auth.Provider(provider),
	}, nil
}

// StorageConfig represents base configuration for storage
type StorageConfig struct {
	Provider   Provider               `json:"provider" validate:"required"`
	Region     string                 `json:"region,omitempty"`
	AuthConfig auth.Config            `json:"auth" validate:"required"`
	Extra      map[string]interface{} `json:"extra,omitempty"`
}

// GetAuthConfig returns the auth configuration
func (c StorageConfig) GetAuthConfig() auth.Config {
	return c.AuthConfig
}

// defaultFactory is the global default factory instance
var (
	defaultFactory StorageFactory
	defaultOnce    sync.Once
)

// GetDefaultFactory returns the global default factory
func GetDefaultFactory() StorageFactory {
	defaultOnce.Do(func() {
		authFactory := auth.GetDefaultFactory()
		defaultFactory = NewDefaultFactory(authFactory, logging.NewNopLogger())
	})
	return defaultFactory
}

// SetDefaultFactory sets the global default factory
func SetDefaultFactory(factory StorageFactory) {
	defaultFactory = factory
}
