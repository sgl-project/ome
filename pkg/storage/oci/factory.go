package oci

import (
	"context"
	"fmt"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// Factory creates OCI storage instances
type Factory struct {
	logger logging.Interface
}

// NewFactory creates a new OCI storage factory
func NewFactory(logger logging.Interface) *Factory {
	return &Factory{
		logger: logger,
	}
}

// Create creates an OCI storage instance
func (f *Factory) Create(ctx context.Context, config interface{}, credentials auth.Credentials) (storage.Storage, error) {
	// Type assert config
	ociConfig, ok := config.(*Config)
	if !ok {
		// Try to convert from generic map
		if mapConfig, ok := config.(map[string]interface{}); ok {
			ociConfig = &Config{}
			if compartmentID, ok := mapConfig["compartment_id"].(string); ok {
				ociConfig.CompartmentID = compartmentID
			}
			if region, ok := mapConfig["region"].(string); ok {
				ociConfig.Region = region
			}
			if enableOboToken, ok := mapConfig["enable_obo_token"].(bool); ok {
				ociConfig.EnableOboToken = enableOboToken
			}
			if oboToken, ok := mapConfig["obo_token"].(string); ok {
				ociConfig.OboToken = oboToken
			}
			// Note: auth is now handled separately through credentials parameter
		} else {
			return nil, fmt.Errorf("invalid config type: expected *Config or map[string]interface{}")
		}
	}

	// Validate credentials provider
	if credentials.Provider() != auth.ProviderOCI {
		return nil, fmt.Errorf("invalid credentials provider: expected %s, got %s", auth.ProviderOCI, credentials.Provider())
	}

	return New(ctx, ociConfig, credentials, f.logger)
}
