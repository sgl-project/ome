package auth

import (
	"context"
	"fmt"
	"sync"

	"github.com/sgl-project/ome/pkg/logging"
	"go.uber.org/zap"
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

// maxFallbackDepth is the maximum number of fallback attempts allowed
const maxFallbackDepth = 10

// Create creates credentials for the given provider and config
func (f *DefaultFactory) Create(ctx context.Context, config Config) (Credentials, error) {
	return f.createWithDepth(ctx, config, 0)
}

// createWithDepth creates credentials with fallback depth tracking
func (f *DefaultFactory) createWithDepth(ctx context.Context, config Config, depth int) (Credentials, error) {
	// Check recursion depth limit
	if depth >= maxFallbackDepth {
		return nil, fmt.Errorf("maximum fallback depth (%d) exceeded - possible circular dependency in auth configuration", maxFallbackDepth)
	}

	f.mu.RLock()
	factory, exists := f.providers[config.Provider]
	f.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unsupported provider: %s", config.Provider)
	}

	f.logger.WithField("provider", config.Provider).WithField("auth_type", config.AuthType).WithField("depth", depth).Info("Creating credentials")

	// Try primary config
	creds, err := factory.Create(ctx, config)
	if err == nil {
		return creds, nil
	}

	// If primary fails and fallback is configured, try fallback
	if config.Fallback != nil {
		f.logger.WithError(err).WithField("fallback_depth", depth+1).Warn("Primary auth failed, trying fallback")
		return f.createWithDepth(ctx, *config.Fallback, depth+1)
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
	defaultMu      sync.RWMutex
)

// GetDefaultFactory returns the global default factory. It is thread-safe.
func GetDefaultFactory() Factory {
	defaultMu.RLock()
	if defaultFactory != nil {
		defer defaultMu.RUnlock()
		return defaultFactory
	}
	defaultMu.RUnlock()

	defaultMu.Lock()
	defer defaultMu.Unlock()

	// Double-check after acquiring write lock
	if defaultFactory == nil {
		// Create a simple zap production logger
		logger, err := zap.NewProduction()
		if err != nil {
			panic(fmt.Sprintf("failed to create default logger for auth factory: %v", err))
		}
		defaultFactory = NewDefaultFactory(logging.ForZap(logger))
	}
	return defaultFactory
}

// SetDefaultFactory sets the global default factory. It is thread-safe.
func SetDefaultFactory(factory Factory) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultFactory = factory
}
