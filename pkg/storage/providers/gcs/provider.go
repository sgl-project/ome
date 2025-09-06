package gcs

import (
	"context"
	"fmt"
	"io"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	storageTypes "github.com/sgl-project/ome/pkg/storage"
)

const (
	// Default thresholds and settings for GCS operations
	defaultConcurrency                 = 16                // Higher than S3 due to GCS performance
	defaultParallelDownloadThresholdMB = 100               // 100MB threshold
	parallelThreshold                  = 100 * 1024 * 1024 // 100MB in bytes
	bufferSize                         = 1024 * 1024       // 1MB buffer
)

// GCSProvider implements the Storage interface for Google Cloud Storage
type GCSProvider struct {
	client      *storage.Client // GCS client
	bucket      string
	projectID   string
	location    string // GCS location (region)
	region      string // Alias for location (for presigned URLs)
	logger      logging.Interface
	bufferPool  *sync.Pool
	credentials auth.Credentials

	// activeUploads tracks ongoing composite uploads for this provider instance
	activeUploadsLock sync.RWMutex
	activeUploads     map[string]*compositeUpload
}

// Provider is an alias for GCSProvider for consistency with Phase 2 files
type Provider = GCSProvider

// Ensure GCSProvider implements the Storage interface
var _ storageTypes.Storage = (*GCSProvider)(nil)

// NewGCSProvider creates a new GCS storage provider
func NewGCSProvider(ctx context.Context, config storageTypes.Config, logger logging.Interface) (storageTypes.Storage, error) {
	if config.Provider != storageTypes.ProviderGCS {
		return nil, fmt.Errorf("invalid provider: expected %s, got %s", storageTypes.ProviderGCS, config.Provider)
	}

	// Validate required configuration
	if config.Bucket == "" {
		return nil, fmt.Errorf("GCS bucket is required")
	}

	// Extract project ID from config
	projectID := ""
	if config.Extra != nil {
		if pid, ok := config.Extra["project_id"].(string); ok {
			projectID = pid
		}
	}

	// Create auth configuration
	authConfig := auth.Config{
		Provider: auth.ProviderGCP,
		AuthType: getAuthType(config.AuthConfig),
		Extra:    config.AuthConfig.Extra,
	}

	// Add project ID if provided
	if projectID != "" && authConfig.Extra == nil {
		authConfig.Extra = make(map[string]interface{})
		authConfig.Extra["project_id"] = projectID
	}

	// Create credentials using the auth factory
	authFactory := auth.GetDefaultFactory()
	credentials, err := authFactory.Create(ctx, authConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP credentials: %w", err)
	}

	// Initialize buffer pool for efficient memory usage
	bufferPool := &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, bufferSize)
			return &buf
		},
	}

	// Create GCS client
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	provider := &GCSProvider{
		client:        client,
		bucket:        config.Bucket,
		projectID:     projectID,
		location:      config.Region, // GCS uses region as location
		region:        config.Region, // Alias for presigned URLs
		logger:        logger,
		bufferPool:    bufferPool,
		credentials:   credentials,
		activeUploads: make(map[string]*compositeUpload),
	}

	logger.WithField("provider", "gcs").
		WithField("bucket", config.Bucket).
		WithField("project", projectID).
		WithField("location", config.Region).
		Info("GCS storage provider initialized")

	return provider, nil
}

// getAuthType determines the auth type from configuration
func getAuthType(authConfig *storageTypes.AuthConfig) auth.AuthType {
	if authConfig == nil || authConfig.Type == "" {
		return auth.GCPApplicationDefault
	}

	switch authConfig.Type {
	case "service_account":
		return auth.GCPServiceAccount
	case "application_default", "default":
		return auth.GCPApplicationDefault
	default:
		return auth.GCPApplicationDefault
	}
}

// Provider returns the storage provider type
func (p *GCSProvider) Provider() storageTypes.Provider {
	return storageTypes.ProviderGCS
}

// Download downloads an object from GCS to a local file
func (p *GCSProvider) Download(ctx context.Context, source string, target string, opts ...storageTypes.DownloadOption) error {
	return fmt.Errorf("GCS Download not implemented yet")
}

// Upload uploads a local file to GCS
func (p *GCSProvider) Upload(ctx context.Context, source string, target string, opts ...storageTypes.UploadOption) error {
	return fmt.Errorf("GCS Upload not implemented yet")
}

// Get retrieves an object from GCS as a reader
func (p *GCSProvider) Get(ctx context.Context, uri string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("GCS Get not implemented yet")
}

// Put uploads data to GCS
func (p *GCSProvider) Put(ctx context.Context, uri string, reader io.Reader, size int64, opts ...storageTypes.UploadOption) error {
	return fmt.Errorf("GCS Put not implemented yet")
}

// Delete removes an object from GCS
func (p *GCSProvider) Delete(ctx context.Context, uri string) error {
	return fmt.Errorf("GCS Delete not implemented yet")
}

// Exists checks if an object exists in GCS
func (p *GCSProvider) Exists(ctx context.Context, uri string) (bool, error) {
	return false, fmt.Errorf("GCS Exists not implemented yet")
}

// List lists objects in GCS with the given prefix
func (p *GCSProvider) List(ctx context.Context, uri string, opts ...storageTypes.ListOption) ([]storageTypes.ObjectInfo, error) {
	return nil, fmt.Errorf("GCS List not implemented yet")
}

// Stat retrieves metadata for an object
func (p *GCSProvider) Stat(ctx context.Context, uri string) (*storageTypes.Metadata, error) {
	return nil, fmt.Errorf("GCS Stat not implemented yet")
}

// Copy performs a server-side copy within GCS
func (p *GCSProvider) Copy(ctx context.Context, source string, target string) error {
	return fmt.Errorf("GCS Copy not implemented yet")
}
