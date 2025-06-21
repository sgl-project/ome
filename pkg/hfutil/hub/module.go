package hub

import (
	"context"
	"fmt"

	"github.com/sgl-project/ome/pkg/logging"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// HubClientParams represents the parameters that can be injected into the Hub client
type HubClientParams struct {
	fx.In

	// Logger for hub operations
	Logger logging.Interface `name:"hub_logger"`
	// Alternative logger option
	AnotherLogger logging.Interface `name:"another_log" optional:"true"`
}

// HubClient represents the enhanced Hub client with dependency injection support
type HubClient struct {
	config *HubConfig
	logger logging.Interface
}

// NewHubClient creates a new Hub client with the provided configuration
func NewHubClient(config *HubConfig) (*HubClient, error) {
	if err := config.ValidateConfig(); err != nil {
		return nil, fmt.Errorf("invalid hub config: %w", err)
	}

	return &HubClient{
		config: config,
		logger: config.Logger,
	}, nil
}

// Download downloads a single file using the configured client
func (c *HubClient) Download(ctx context.Context, repoID, filename string, opts ...DownloadOption) (string, error) {
	config := c.config.ToDownloadConfig()
	config.RepoID = repoID
	config.Filename = filename

	// Apply additional options
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return "", fmt.Errorf("failed to apply download option: %w", err)
		}
	}

	// Create progress manager and log the operation
	progressManager := c.config.CreateProgressManager()
	if progressManager != nil {
		progressManager.LogDownloadStart(repoID, filename, 0) // Size unknown at this point
	}

	// Add hub config to context for progress reporting
	ctx = context.WithValue(ctx, HubConfigKey, c.config)

	result, err := HfHubDownload(ctx, config)

	if err != nil && progressManager != nil {
		progressManager.LogError("single_download", repoID, err)
	}

	return result, err
}

// SnapshotDownload downloads all files in a repository
func (c *HubClient) SnapshotDownload(ctx context.Context, repoID, localDir string, opts ...DownloadOption) (string, error) {
	config := c.config.ToDownloadConfig()
	config.RepoID = repoID
	config.LocalDir = localDir

	// Apply additional options
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return "", fmt.Errorf("failed to apply download option: %w", err)
		}
	}

	// Add hub config to context for progress reporting
	ctx = context.WithValue(ctx, HubConfigKey, c.config)

	result, err := SnapshotDownload(ctx, config)

	if err != nil {
		progressManager := c.config.CreateProgressManager()
		if progressManager != nil {
			progressManager.LogError("snapshot_download", repoID, err)
		}
	}

	return result, err
}

// ListFiles lists all files in a repository
func (c *HubClient) ListFiles(ctx context.Context, repoID string, opts ...DownloadOption) ([]RepoFile, error) {
	config := c.config.ToDownloadConfig()
	config.RepoID = repoID

	// Apply additional options
	for _, opt := range opts {
		if err := opt(config); err != nil {
			return nil, fmt.Errorf("failed to apply download option: %w", err)
		}
	}

	// Create progress manager for logging
	progressManager := c.config.CreateProgressManager()
	if progressManager != nil && progressManager.enableDetailedLogs {
		progressManager.logger.
			WithField("repo_id", repoID).
			WithField("operation", "list_files").
			Info("Starting repository file listing")
	}

	files, err := ListRepoFiles(ctx, config)

	if err != nil && progressManager != nil {
		progressManager.LogError("list_files", repoID, err)
	} else if progressManager != nil {
		progressManager.LogRepoListing(repoID, len(files))
	}

	return files, err
}

// GetConfig returns the client configuration
func (c *HubClient) GetConfig() *HubConfig {
	return c.config
}

// DownloadOption represents an option for download operations
type DownloadOption func(*DownloadConfig) error

// WithRevision sets the revision for the download
func WithRevision(revision string) DownloadOption {
	return func(config *DownloadConfig) error {
		config.Revision = revision
		return nil
	}
}

// WithSubfolder sets the subfolder for the download
func WithSubfolder(subfolder string) DownloadOption {
	return func(config *DownloadConfig) error {
		config.Subfolder = subfolder
		return nil
	}
}

// WithRepoType sets the repository type
func WithRepoType(repoType string) DownloadOption {
	return func(config *DownloadConfig) error {
		config.RepoType = repoType
		return nil
	}
}

// WithForceDownload enables force download mode
func WithForceDownload(force bool) DownloadOption {
	return func(config *DownloadConfig) error {
		config.ForceDownload = force
		return nil
	}
}

// WithLocalOnly enables local files only mode for downloads
func WithLocalOnly(localOnly bool) DownloadOption {
	return func(config *DownloadConfig) error {
		config.LocalFilesOnly = localOnly
		return nil
	}
}

// WithPatterns sets allow and ignore patterns for filtering
func WithPatterns(allowPatterns, ignorePatterns []string) DownloadOption {
	return func(config *DownloadConfig) error {
		config.AllowPatterns = allowPatterns
		config.IgnorePatterns = ignorePatterns
		return nil
	}
}

// Module provides the fx module for dependency injection
var Module = fx.Provide(
	func(v *viper.Viper, params HubClientParams) (*HubClient, error) {
		// Use the provided logger or fall back to AnotherLogger
		var logger logging.Interface
		if params.Logger != nil {
			logger = params.Logger
		} else if params.AnotherLogger != nil {
			logger = params.AnotherLogger
		}

		config, err := NewHubConfig(
			WithViper(v),
			WithLogger(logger),
		)
		if err != nil {
			return nil, fmt.Errorf("error creating hub config: %+v", err)
		}

		return NewHubClient(config)
	})
