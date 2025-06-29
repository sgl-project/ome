package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/sgl-project/ome/pkg/logging"
)

// DefaultFactory is the default auth factory implementation
type DefaultFactory struct {
	mu        sync.RWMutex
	providers map[Provider]ProviderFactory
	logger    logging.Interface
}

// ProviderFactory creates credentials for a specific provider
type ProviderFactory interface {
	Create(ctx context.Context, config Config) (Credentials, error)
	SupportedAuthTypes() []AuthType
}

// NewDefaultFactory creates a new default factory
func NewDefaultFactory(logger logging.Interface) *DefaultFactory {
	f := &DefaultFactory{
		providers: make(map[Provider]ProviderFactory),
		logger:    logger,
	}

	// Providers should be registered externally to avoid import cycles
	// Example:
	// factory.RegisterProvider(auth.ProviderOCI, oci.NewFactory(logger))

	return f
}

// RegisterProvider registers a provider factory
func (f *DefaultFactory) RegisterProvider(provider Provider, factory ProviderFactory) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[provider] = factory
}

// Create creates credentials for the given provider and config
func (f *DefaultFactory) Create(ctx context.Context, config Config) (Credentials, error) {
	f.mu.RLock()
	factory, exists := f.providers[config.Provider]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}

	f.logger.WithField("provider", config.Provider).WithField("auth_type", config.AuthType).Info("Creating credentials")

	// Try primary config
	creds, err := factory.Create(ctx, config)
	if err == nil {
		return creds, nil
	}

	// If primary fails and fallback is configured, try fallback
	if config.Fallback != nil {
		f.logger.WithError(err).Warn("Primary auth failed, trying fallback")
		return f.Create(ctx, *config.Fallback)
	}

	return nil, err
}

// SupportedProviders returns list of supported providers
func (f *DefaultFactory) SupportedProviders() []Provider {
	f.mu.RLock()
	defer f.mu.RUnlock()

	providers := make([]Provider, 0, len(f.providers))
	for p := range f.providers {
		providers = append(providers, p)
	}
	return providers
}

// SupportedAuthTypes returns supported auth types for a provider
func (f *DefaultFactory) SupportedAuthTypes(provider Provider) []AuthType {
	f.mu.RLock()
	factory, exists := f.providers[provider]
	f.mu.RUnlock()

	if !exists {
		return nil
	}

	return factory.SupportedAuthTypes()
}

// defaultFactory is the global default factory instance
var (
	defaultFactory Factory
	defaultOnce    sync.Once
)

// GetDefaultFactory returns the global default factory
func GetDefaultFactory() Factory {
	defaultOnce.Do(func() {
		defaultFactory = NewDefaultFactory(logging.NewNopLogger())
	})
	return defaultFactory
}

// SetDefaultFactory sets the global default factory
func SetDefaultFactory(factory Factory) {
	defaultFactory = factory
}
