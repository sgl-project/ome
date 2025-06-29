package github

import (
	"context"
	"fmt"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// Factory creates GitHub LFS storage instances
type Factory struct {
	logger logging.Interface
}

// NewFactory creates a new GitHub LFS storage factory
func NewFactory(logger logging.Interface) *Factory {
	return &Factory{
		logger: logger,
	}
}

// Create creates a GitHub LFS storage instance
func (f *Factory) Create(ctx context.Context, config interface{}, credentials auth.Credentials) (storage.Storage, error) {
	// Type assert config
	githubConfig, ok := config.(*Config)
	if !ok {
		// Try to convert from generic map
		if mapConfig, ok := config.(map[string]interface{}); ok {
			githubConfig = &Config{}
			if owner, ok := mapConfig["owner"].(string); ok {
				githubConfig.Owner = owner
			}
			if repo, ok := mapConfig["repo"].(string); ok {
				githubConfig.Repo = repo
			}
			if apiEndpoint, ok := mapConfig["api_endpoint"].(string); ok {
				githubConfig.APIEndpoint = apiEndpoint
			}
			if lfsEndpoint, ok := mapConfig["lfs_endpoint"].(string); ok {
				githubConfig.LFSEndpoint = lfsEndpoint
			}
			if chunkSize, ok := mapConfig["chunk_size"].(int64); ok {
				githubConfig.ChunkSize = chunkSize
			}
		} else {
			return nil, fmt.Errorf("invalid config type: expected *Config or map[string]interface{}")
		}
	}

	// Validate credentials provider
	if credentials.Provider() != auth.ProviderGitHub {
		return nil, fmt.Errorf("invalid credentials provider: expected %s, got %s", auth.ProviderGitHub, credentials.Provider())
	}

	return New(ctx, githubConfig, credentials, f.logger)
}
