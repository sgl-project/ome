package aws

import (
	"context"
	"fmt"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// Factory creates AWS S3 storage instances
type Factory struct {
	logger logging.Interface
}

// NewFactory creates a new AWS S3 storage factory
func NewFactory(logger logging.Interface) *Factory {
	return &Factory{
		logger: logger,
	}
}

// Create creates an AWS S3 storage instance
func (f *Factory) Create(ctx context.Context, config interface{}, credentials auth.Credentials) (storage.Storage, error) {
	// Handle nil config
	if config == nil {
		return New(ctx, nil, credentials, f.logger)
	}

	// Type assert config
	s3Config, ok := config.(*Config)
	if !ok {
		// Try to convert from generic map
		if mapConfig, ok := config.(map[string]interface{}); ok {
			s3Config = &Config{}
			if region, ok := mapConfig["region"].(string); ok {
				s3Config.Region = region
			}
			if endpoint, ok := mapConfig["endpoint"].(string); ok {
				s3Config.Endpoint = endpoint
			}
			if forcePathStyle, ok := mapConfig["force_path_style"].(bool); ok {
				s3Config.ForcePathStyle = forcePathStyle
			}
			if disableSSL, ok := mapConfig["disable_ssl"].(bool); ok {
				s3Config.DisableSSL = disableSSL
			}
			if partSize, ok := mapConfig["part_size"].(int64); ok {
				s3Config.PartSize = partSize
			}
			if concurrency, ok := mapConfig["concurrency"].(int); ok {
				s3Config.Concurrency = concurrency
			}
		} else {
			return nil, fmt.Errorf("invalid config type: expected *Config or map[string]interface{}")
		}
	}

	// Validate credentials provider
	if credentials.Provider() != auth.ProviderAWS {
		return nil, fmt.Errorf("invalid credentials provider: expected %s, got %s", auth.ProviderAWS, credentials.Provider())
	}

	return New(ctx, s3Config, credentials, f.logger)
}
