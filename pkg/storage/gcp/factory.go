package gcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// Factory creates GCP storage instances
type Factory struct {
	logger logging.Interface
}

// NewFactory creates a new GCP storage factory
func NewFactory(logger logging.Interface) *Factory {
	return &Factory{
		logger: logger,
	}
}

// Create creates a GCP storage instance
func (f *Factory) Create(ctx context.Context, config interface{}, credentials auth.Credentials) (storage.Storage, error) {
	// Type assert config
	var gcsConfig *Config

	switch v := config.(type) {
	case *Config:
		gcsConfig = v
	case []byte:
		gcsConfig = &Config{}
		if err := json.Unmarshal(v, gcsConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON config: %w", err)
		}
	case json.RawMessage:
		gcsConfig = &Config{}
		if err := json.Unmarshal(v, gcsConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON config: %w", err)
		}
	case map[string]interface{}:
		gcsConfig = &Config{}
		if projectID, ok := v["project_id"].(string); ok {
			gcsConfig.ProjectID = projectID
		}
		if location, ok := v["location"].(string); ok {
			gcsConfig.Location = location
		}
		if storageClass, ok := v["storage_class"].(string); ok {
			gcsConfig.StorageClass = storageClass
		}
		if uniformBucketLevelAccess, ok := v["uniform_bucket_level_access"].(bool); ok {
			gcsConfig.UniformBucketLevelAccess = uniformBucketLevelAccess
		}
		if chunkSize, ok := v["chunk_size"].(int); ok {
			gcsConfig.ChunkSize = chunkSize
		}
		if enableCRC32C, ok := v["enable_crc32c"].(bool); ok {
			gcsConfig.EnableCRC32C = enableCRC32C
		}
	case nil:
		// Allow nil config
		gcsConfig = nil
	default:
		return nil, fmt.Errorf("invalid config type: expected *Config, []byte, json.RawMessage, or map[string]interface{}")
	}

	// Validate credentials provider
	if credentials.Provider() != auth.ProviderGCP {
		return nil, fmt.Errorf("invalid credentials provider: expected %s, got %s", auth.ProviderGCP, credentials.Provider())
	}

	return New(ctx, gcsConfig, credentials, f.logger)
}
