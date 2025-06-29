package azure

import (
	"context"
	"fmt"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// Factory creates Azure storage instances
type Factory struct {
	logger logging.Interface
}

// NewFactory creates a new Azure storage factory
func NewFactory(logger logging.Interface) *Factory {
	return &Factory{
		logger: logger,
	}
}

// Create creates an Azure storage instance
func (f *Factory) Create(ctx context.Context, config interface{}, credentials auth.Credentials) (storage.Storage, error) {
	// Type assert config
	azureConfig, ok := config.(*Config)
	if !ok {
		// Try to convert from generic map
		if mapConfig, ok := config.(map[string]interface{}); ok {
			azureConfig = &Config{}
			if accountName, ok := mapConfig["account_name"].(string); ok {
				azureConfig.AccountName = accountName
			}
			if accountKey, ok := mapConfig["account_key"].(string); ok {
				azureConfig.AccountKey = accountKey
			}
			if sasToken, ok := mapConfig["sas_token"].(string); ok {
				azureConfig.SASToken = sasToken
			}
			if endpoint, ok := mapConfig["endpoint"].(string); ok {
				azureConfig.Endpoint = endpoint
			}
			if useDevelopmentStorage, ok := mapConfig["use_development_storage"].(bool); ok {
				azureConfig.UseDevelopmentStorage = useDevelopmentStorage
			}
			if blockSize, ok := mapConfig["block_size"].(int64); ok {
				azureConfig.BlockSize = blockSize
			}
			if concurrency, ok := mapConfig["concurrency"].(int); ok {
				azureConfig.Concurrency = concurrency
			}
		} else {
			return nil, fmt.Errorf("invalid config type: expected *Config or map[string]interface{}")
		}
	}

	// Validate credentials provider
	if credentials.Provider() != auth.ProviderAzure {
		return nil, fmt.Errorf("invalid credentials provider: expected %s, got %s", auth.ProviderAzure, credentials.Provider())
	}

	return New(ctx, azureConfig, credentials, f.logger)
}
