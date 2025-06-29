package azure

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/sgl-project/ome/pkg/auth"
	authazure "github.com/sgl-project/ome/pkg/auth/azure"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// AzureStorage implements storage.Storage for Azure Blob Storage
type AzureStorage struct {
	client      azureClient
	credentials auth.Credentials
	logger      logging.Interface
	config      *Config
}

// Config represents Azure Blob Storage configuration
type Config struct {
	AccountName           string `json:"account_name"`
	AccountKey            string `json:"account_key"`
	SASToken              string `json:"sas_token"`
	Endpoint              string `json:"endpoint"`
	UseDevelopmentStorage bool   `json:"use_development_storage"`
	BlockSize             int64  `json:"block_size"`
	Concurrency           int    `json:"concurrency"`
}

// DefaultConfig returns default Azure storage configuration
func DefaultConfig() *Config {
	return &Config{
		BlockSize:   4 * 1024 * 1024, // 4MB
		Concurrency: 5,
	}
}

// New creates a new Azure storage instance
func New(ctx context.Context, cfg *Config, credentials auth.Credentials, logger logging.Interface) (*AzureStorage, error) {
	// Ensure we have Azure credentials
	azureCreds, ok := credentials.(*authazure.AzureCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type: expected Azure credentials")
	}

	// Apply defaults
	if cfg == nil {
		cfg = DefaultConfig()
	} else {
		defaultConfig := DefaultConfig()
		if cfg.BlockSize == 0 {
			cfg.BlockSize = defaultConfig.BlockSize
		}
		if cfg.Concurrency == 0 {
			cfg.Concurrency = defaultConfig.Concurrency
		}
	}

	// Create Azure client
	client, err := createAzureClient(ctx, cfg, azureCreds)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure client: %w", err)
	}

	return &AzureStorage{
		client:      client,
		credentials: credentials,
		logger:      logger,
		config:      cfg,
	}, nil
}

// NewWithClient creates a new Azure storage instance with a custom client (for testing)
func NewWithClient(client azureClient, cfg *Config, credentials auth.Credentials, logger logging.Interface) *AzureStorage {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &AzureStorage{
		client:      client,
		credentials: credentials,
		logger:      logger,
		config:      cfg,
	}
}

// Provider returns the storage provider type
func (s *AzureStorage) Provider() storage.Provider {
	return storage.ProviderAzure
}

// Download retrieves the object and writes it to the target path
func (s *AzureStorage) Download(ctx context.Context, source storage.ObjectURI, target string, opts ...storage.DownloadOption) error {
	// Apply download options
	downloadOpts := storage.DefaultDownloadOptions()
	for _, opt := range opts {
		if err := opt(&downloadOpts); err != nil {
			return err
		}
	}

	// Create directory if needed
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Check if we should use multipart download
	var useMultipart bool
	var objectSize int64

	if !downloadOpts.ForceStandard {
		// Get blob properties
		blobClient := s.client.ServiceClient().NewContainerClient(source.BucketName).NewBlobClient(source.ObjectName)
		props, err := blobClient.GetProperties(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to get blob properties: %w", err)
		}

		if props.ContentLength != nil {
			objectSize = *props.ContentLength
		}

		// Determine if we should use multipart
		if downloadOpts.ForceMultipart {
			useMultipart = true
		} else if objectSize > int64(downloadOpts.SizeThresholdInMB)*1024*1024 {
			useMultipart = true
		}
	}

	if useMultipart && objectSize > 0 {
		// Use advanced multipart download with validation
		return s.multipartDownload(ctx, source, target, objectSize, &downloadOpts)
	}

	// Use standard download for small files
	blobClient := s.client.ServiceClient().NewContainerClient(source.BucketName).NewBlobClient(source.ObjectName)

	// Create file
	file, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Download options
	downloadOptions := &azblob.DownloadFileOptions{
		BlockSize:   s.config.BlockSize,
		Concurrency: uint16(s.config.Concurrency),
	}

	if downloadOpts.ChunkSizeInMB > 0 {
		downloadOptions.BlockSize = int64(downloadOpts.ChunkSizeInMB) * 1024 * 1024
	}
	if downloadOpts.Threads > 0 {
		downloadOptions.Concurrency = uint16(downloadOpts.Threads)
	}

	// Download file
	_, err = blobClient.DownloadFile(ctx, file, downloadOptions)
	if err != nil {
		return fmt.Errorf("failed to download blob: %w", err)
	}

	return nil
}

// Upload stores the file at source path as the target object
func (s *AzureStorage) Upload(ctx context.Context, source string, target storage.ObjectURI, opts ...storage.UploadOption) error {
	// Open file
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	return s.Put(ctx, target, file, info.Size(), opts...)
}

// Get retrieves an object and returns a reader
func (s *AzureStorage) Get(ctx context.Context, uri storage.ObjectURI) (io.ReadCloser, error) {
	blobClient := s.client.ServiceClient().NewContainerClient(uri.BucketName).NewBlobClient(uri.ObjectName)

	resp, err := blobClient.DownloadStream(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob: %w", err)
	}

	return resp.Body, nil
}

// Put stores data from reader as an object
func (s *AzureStorage) Put(ctx context.Context, uri storage.ObjectURI, reader io.Reader, size int64, opts ...storage.UploadOption) error {
	if reader == nil {
		return fmt.Errorf("reader cannot be nil")
	}

	uploadOpts := storage.DefaultUploadOptions()
	for _, opt := range opts {
		if err := opt(&uploadOpts); err != nil {
			return err
		}
	}

	// Get blob client
	blobClient := s.client.ServiceClient().NewContainerClient(uri.BucketName).NewBlockBlobClient(uri.ObjectName)

	// Set upload options
	uploadOptions := &blockblob.UploadOptions{}

	// Set HTTP headers
	httpHeaders := &blob.HTTPHeaders{}
	if uploadOpts.ContentType != "" {
		httpHeaders.BlobContentType = &uploadOpts.ContentType
	}
	uploadOptions.HTTPHeaders = httpHeaders

	// Set metadata
	if uploadOpts.Metadata != nil {
		// Convert map[string]string to map[string]*string
		metadata := make(map[string]*string)
		for k, v := range uploadOpts.Metadata {
			value := v
			metadata[k] = &value
		}
		uploadOptions.Metadata = metadata
	}

	// Set access tier if storage class is specified
	if uploadOpts.StorageClass != "" {
		tier := blob.AccessTier(uploadOpts.StorageClass)
		uploadOptions.Tier = &tier
	}

	// Upload blob
	// Need to provide an io.ReadSeekCloser
	var body io.ReadSeekCloser
	if rsc, ok := reader.(io.ReadSeekCloser); ok {
		body = rsc
	} else {
		// For non-seekable readers, we need to buffer the content
		data, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("failed to read data: %w", err)
		}
		body = &nopCloserSeeker{Reader: bytes.NewReader(data)}
	}

	_, err := blobClient.Upload(ctx, body, uploadOptions)
	if err != nil {
		return fmt.Errorf("failed to upload blob: %w", err)
	}

	return nil
}

// Delete removes an object
func (s *AzureStorage) Delete(ctx context.Context, uri storage.ObjectURI) error {
	blobClient := s.client.ServiceClient().NewContainerClient(uri.BucketName).NewBlobClient(uri.ObjectName)

	_, err := blobClient.Delete(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to delete blob: %w", err)
	}

	return nil
}

// Exists checks if an object exists
func (s *AzureStorage) Exists(ctx context.Context, uri storage.ObjectURI) (bool, error) {
	blobClient := s.client.ServiceClient().NewContainerClient(uri.BucketName).NewBlobClient(uri.ObjectName)

	_, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// List returns a list of objects matching the criteria
func (s *AzureStorage) List(ctx context.Context, uri storage.ObjectURI, opts storage.ListOptions) ([]storage.ObjectInfo, error) {
	containerClient := s.client.ServiceClient().NewContainerClient(uri.BucketName)

	// Set list options
	listOptions := &container.ListBlobsFlatOptions{}

	prefix := opts.Prefix
	if prefix == "" && uri.Prefix != "" {
		prefix = uri.Prefix
	}
	if prefix != "" {
		listOptions.Prefix = &prefix
	}

	if opts.MaxKeys > 0 {
		maxResults := int32(opts.MaxKeys)
		listOptions.MaxResults = &maxResults
	}

	var objects []storage.ObjectInfo
	count := 0

	pager := containerClient.NewListBlobsFlatPager(listOptions)
	for pager.More() {
		resp, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list blobs: %w", err)
		}

		for _, item := range resp.Segment.BlobItems {
			// Skip if before StartAfter
			if opts.StartAfter != "" && *item.Name <= opts.StartAfter {
				continue
			}

			// Skip if we've reached MaxKeys
			if opts.MaxKeys > 0 && count >= opts.MaxKeys {
				break
			}

			info := storage.ObjectInfo{
				Name: *item.Name,
			}

			if item.Properties != nil {
				if item.Properties.ContentLength != nil {
					info.Size = *item.Properties.ContentLength
				}
				if item.Properties.LastModified != nil {
					info.LastModified = item.Properties.LastModified.Format(time.RFC3339)
				}
				if item.Properties.ETag != nil {
					info.ETag = string(*item.Properties.ETag)
				}
				if item.Properties.ContentType != nil {
					info.ContentType = *item.Properties.ContentType
				}
				if item.Properties.AccessTier != nil {
					info.StorageClass = string(*item.Properties.AccessTier)
				}
			}

			objects = append(objects, info)
			count++
		}

		// If we've reached MaxKeys, stop
		if opts.MaxKeys > 0 && count >= opts.MaxKeys {
			break
		}
	}

	return objects, nil
}

// GetObjectInfo retrieves metadata about an object
func (s *AzureStorage) GetObjectInfo(ctx context.Context, uri storage.ObjectURI) (*storage.ObjectInfo, error) {
	blobClient := s.client.ServiceClient().NewContainerClient(uri.BucketName).NewBlobClient(uri.ObjectName)

	props, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob properties: %w", err)
	}

	info := &storage.ObjectInfo{
		Name: uri.ObjectName,
	}

	if props.ContentLength != nil {
		info.Size = *props.ContentLength
	}
	if props.LastModified != nil {
		info.LastModified = props.LastModified.Format(time.RFC3339)
	}
	if props.ETag != nil {
		info.ETag = string(*props.ETag)
	}
	if props.ContentType != nil {
		info.ContentType = *props.ContentType
	}
	if props.AccessTier != nil {
		info.StorageClass = string(*props.AccessTier)
	}

	return info, nil
}

// Copy copies an object within Azure
func (s *AzureStorage) Copy(ctx context.Context, source, target storage.ObjectURI) error {
	// Get source URL
	// Extract account name from extra fields or use the same account
	accountName := s.config.AccountName
	if source.Extra != nil {
		if name, ok := source.Extra["account_name"].(string); ok {
			accountName = name
		}
	}
	sourceURL := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s",
		accountName, source.BucketName, source.ObjectName)

	// Get target blob client
	targetClient := s.client.ServiceClient().NewContainerClient(target.BucketName).NewBlobClient(target.ObjectName)

	// Start copy operation
	_, err := targetClient.StartCopyFromURL(ctx, sourceURL, nil)
	if err != nil {
		return fmt.Errorf("failed to copy blob: %w", err)
	}

	return nil
}

// Azure Blob Storage doesn't have native multipart API like S3
// We implement compatibility using block blobs

func (s *AzureStorage) InitiateMultipartUpload(ctx context.Context, uri storage.ObjectURI, opts ...storage.UploadOption) (string, error) {
	// Azure doesn't require initiating multipart uploads
	// Return a dummy upload ID
	return fmt.Sprintf("azure-upload-%s-%s-%d", uri.BucketName, uri.ObjectName, time.Now().Unix()), nil
}

func (s *AzureStorage) UploadPart(ctx context.Context, uri storage.ObjectURI, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	// Get block blob client
	blockBlobClient := s.client.ServiceClient().NewContainerClient(uri.BucketName).NewBlockBlobClient(uri.ObjectName)

	// Generate block ID (must be base64 encoded and same length)
	blockID := fmt.Sprintf("%06d", partNumber)
	encodedBlockID := base64Encode(blockID)

	// Stage block
	// Need to provide an io.ReadSeekCloser
	var body io.ReadSeekCloser
	if rsc, ok := reader.(io.ReadSeekCloser); ok {
		body = rsc
	} else {
		// For non-seekable readers, we need to buffer the content
		data, err := io.ReadAll(reader)
		if err != nil {
			return "", fmt.Errorf("failed to read data: %w", err)
		}
		body = &nopCloserSeeker{Reader: bytes.NewReader(data)}
	}

	_, err := blockBlobClient.StageBlock(ctx, encodedBlockID, body, nil)
	if err != nil {
		return "", fmt.Errorf("failed to stage block: %w", err)
	}

	return encodedBlockID, nil
}

func (s *AzureStorage) CompleteMultipartUpload(ctx context.Context, uri storage.ObjectURI, uploadID string, parts []storage.CompletedPart) error {
	// Get block blob client
	blockBlobClient := s.client.ServiceClient().NewContainerClient(uri.BucketName).NewBlockBlobClient(uri.ObjectName)

	// Create block list
	var blockList []string
	for _, part := range parts {
		blockList = append(blockList, part.ETag) // ETag contains the block ID
	}

	// Commit blocks
	_, err := blockBlobClient.CommitBlockList(ctx, blockList, nil)
	if err != nil {
		return fmt.Errorf("failed to commit block list: %w", err)
	}

	return nil
}

func (s *AzureStorage) AbortMultipartUpload(ctx context.Context, uri storage.ObjectURI, uploadID string) error {
	// Azure doesn't have an explicit abort operation
	// Uncommitted blocks are automatically garbage collected
	return nil
}

// Helper functions

func createAzureClient(ctx context.Context, cfg *Config, creds *authazure.AzureCredentials) (azureClient, error) {
	var serviceURL string
	if cfg.UseDevelopmentStorage {
		serviceURL = "http://127.0.0.1:10000/devstoreaccount1"
	} else if cfg.Endpoint != "" {
		serviceURL = cfg.Endpoint
	} else {
		serviceURL = fmt.Sprintf("https://%s.blob.core.windows.net/", cfg.AccountName)
	}

	// Get credential from our auth package
	azureCredential := creds.GetCredential()

	// Create client with credential
	client, err := azblob.NewClient(serviceURL, azureCredential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create blob client: %w", err)
	}

	// Wrap the client in our implementation
	return &azureClientImpl{client: client}, nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "BlobNotFound") ||
		strings.Contains(err.Error(), "404") ||
		strings.Contains(err.Error(), "NotFound")
}

func base64Encode(s string) string {
	return url.QueryEscape(s)
}

// nopCloserSeeker wraps a bytes.Reader to implement io.ReadSeekCloser
type nopCloserSeeker struct {
	*bytes.Reader
}

func (ncs *nopCloserSeeker) Close() error {
	return nil
}
