package oci

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"

	"github.com/sgl-project/ome/pkg/auth"
	ociauth "github.com/sgl-project/ome/pkg/auth/oci" // Register OCI auth provider
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

const (
	// Default thresholds for parallel operations
	defaultParallelDownloadThresholdMB = 100 // 100MB - matching old implementation
	defaultMultipartUploadThresholdMB  = 100 // 100MB
	defaultPartSizeMB                  = 128 // 128MB
	defaultConcurrency                 = 16  // 16 threads - matching old implementation

	// OCI limits
	maxParts    = 10000
	minPartSize = 5 * 1024 * 1024 // 5MB
)

// OCIProvider implements the Storage interface for OCI Object Storage
type OCIProvider struct {
	logger      logging.Interface
	credentials auth.Credentials
	client      *objectstorage.ObjectStorageClient
	namespace   string
	bucket      string // default bucket from config
	region      string
}

// Ensure OCIProvider implements the Storage interface
var _ storage.Storage = (*OCIProvider)(nil)

// NewOCIProvider creates a new OCI storage provider
func NewOCIProvider(ctx context.Context, config storage.Config, logger logging.Interface) (storage.Storage, error) {
	if config.AuthConfig == nil {
		return nil, fmt.Errorf("auth configuration is required for OCI storage")
	}

	// Create auth configuration
	authConfig := auth.Config{
		Provider: auth.ProviderOCI,
		AuthType: getAuthType(config.AuthConfig),
		Region:   config.Region,
		Extra:    config.AuthConfig.Extra,
	}

	// Create credentials using the auth factory
	authFactory := auth.GetDefaultFactory()
	credentials, err := authFactory.Create(ctx, authConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI credentials: %w", err)
	}

	// Get OCI configuration provider from credentials
	ociCreds, ok := credentials.(*ociauth.OCICredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type: expected OCI credentials")
	}
	configProvider := ociCreds.GetConfigurationProvider()

	// Create OCI Object Storage client
	client, err := createObjectStorageClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI Object Storage client: %w", err)
	}

	// Set region if specified
	if config.Region != "" {
		client.SetRegion(config.Region)
	}

	// Get namespace if not provided
	namespace := config.Namespace
	if namespace == "" {
		namespaceResponse, err := client.GetNamespace(ctx, objectstorage.GetNamespaceRequest{})
		if err != nil {
			return nil, fmt.Errorf("failed to get OCI namespace: %w", err)
		}
		namespace = *namespaceResponse.Value
	}

	return &OCIProvider{
		logger:      logger,
		credentials: credentials,
		client:      client,
		namespace:   namespace,
		bucket:      config.Bucket,
		region:      config.Region,
	}, nil
}

// Provider returns the storage provider type
func (p *OCIProvider) Provider() storage.Provider {
	return storage.ProviderOCI
}

// Download retrieves the object from OCI and saves it to the local file system
func (p *OCIProvider) Download(ctx context.Context, source string, target string, opts ...storage.DownloadOption) error {
	options := storage.BuildDownloadOptions(opts...)

	// Parse source URI
	sourceURI, err := parseOCIURI(source, p.namespace, p.bucket)
	if err != nil {
		return storage.NewError("download", source, "oci", err)
	}

	// Check if we should skip download for valid local copy
	if options.SkipIfValid && !options.ForceRedownload {
		valid, err := p.isLocalCopyValid(ctx, sourceURI, target)
		if err != nil {
			p.logger.WithField("error", err).Warn("Failed to validate local copy, proceeding with download")
		} else if valid {
			p.logger.WithField("target", target).Info("Skipping download, valid local copy exists")
			if options.Progress != nil {
				// Report immediate completion
				headResponse, _ := p.client.HeadObject(ctx, objectstorage.HeadObjectRequest{
					NamespaceName: &sourceURI.Namespace,
					BucketName:    &sourceURI.Bucket,
					ObjectName:    &sourceURI.Object,
				})
				if headResponse.ContentLength != nil {
					options.Progress.Update(*headResponse.ContentLength, *headResponse.ContentLength)
				}
				options.Progress.Done()
			}
			return nil
		}
	}

	// Report progress if configured
	if options.Progress != nil {
		defer options.Progress.Done()
	}

	// Get object metadata to determine size
	headResponse, err := p.client.HeadObject(ctx, objectstorage.HeadObjectRequest{
		NamespaceName: &sourceURI.Namespace,
		BucketName:    &sourceURI.Bucket,
		ObjectName:    &sourceURI.Object,
	})
	if err != nil {
		if options.Progress != nil {
			options.Progress.Error(err)
		}
		return storage.NewError("download", source, "oci", err)
	}

	contentLength := headResponse.ContentLength
	if contentLength == nil {
		return storage.NewError("download", source, "oci", fmt.Errorf("unable to determine object size"))
	}

	// Determine download strategy
	if shouldUseParallelDownload(*contentLength, options) {
		return p.parallelDownload(ctx, sourceURI, target, *contentLength, options)
	}

	return p.simpleDownload(ctx, sourceURI, target, *contentLength, options)
}

// Upload sends a local file to OCI Object Storage
func (p *OCIProvider) Upload(ctx context.Context, source string, target string, opts ...storage.UploadOption) error {
	options := storage.BuildUploadOptions(opts...)

	// Parse target URI
	targetURI, err := parseOCIURI(target, p.namespace, p.bucket)
	if err != nil {
		return storage.NewError("upload", target, "oci", err)
	}

	// Get file info
	fileInfo, err := os.Stat(source)
	if err != nil {
		return storage.NewError("upload", source, "oci", err)
	}

	// Report progress if configured
	if options.Progress != nil {
		defer options.Progress.Done()
	}

	// Determine upload strategy
	if shouldUseMultipartUpload(fileInfo.Size(), options) {
		return p.multipartFileUpload(ctx, source, targetURI, fileInfo.Size(), options)
	}

	return p.simpleUpload(ctx, source, targetURI, fileInfo.Size(), options)
}

// Get retrieves an object as a stream
func (p *OCIProvider) Get(ctx context.Context, uri string) (io.ReadCloser, error) {
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return nil, storage.NewError("get", uri, "oci", err)
	}

	request := objectstorage.GetObjectRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
		ObjectName:    &ociURI.Object,
	}

	response, err := p.client.GetObject(ctx, request)
	if err != nil {
		return nil, storage.NewError("get", uri, "oci", err)
	}

	return response.Content, nil
}

// Put uploads a stream to OCI Object Storage
func (p *OCIProvider) Put(ctx context.Context, uri string, reader io.Reader, size int64, opts ...storage.UploadOption) error {
	options := storage.BuildUploadOptions(opts...)
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return storage.NewError("put", uri, "oci", err)
	}

	// Report progress if configured
	if options.Progress != nil {
		defer options.Progress.Done()
	}

	// For large streams, use multipart upload
	if shouldUseMultipartUpload(size, options) {
		return p.multipartStreamUpload(ctx, ociURI, reader, size, options)
	}

	// Simple put for small objects
	metadata := convertMetadataToOCI(options.Metadata)
	request := objectstorage.PutObjectRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
		ObjectName:    &ociURI.Object,
		PutObjectBody: io.NopCloser(reader),
		ContentLength: &size,
		ContentType:   &options.ContentType,
		OpcMeta:       metadata,
	}

	if options.StorageClass != "" {
		request.StorageTier = objectstorage.PutObjectStorageTierEnum(options.StorageClass)
	}

	_, err = p.client.PutObject(ctx, request)
	if err != nil {
		if options.Progress != nil {
			options.Progress.Error(err)
		}
		return storage.NewError("put", uri, "oci", err)
	}

	if options.Progress != nil {
		options.Progress.Update(size, size)
	}

	return nil
}

// Delete removes an object from OCI Object Storage
func (p *OCIProvider) Delete(ctx context.Context, uri string) error {
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return storage.NewError("delete", uri, "oci", err)
	}

	request := objectstorage.DeleteObjectRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
		ObjectName:    &ociURI.Object,
	}

	_, err = p.client.DeleteObject(ctx, request)
	if err != nil {
		return storage.NewError("delete", uri, "oci", err)
	}

	return nil
}

// Exists checks if an object exists in OCI Object Storage
func (p *OCIProvider) Exists(ctx context.Context, uri string) (bool, error) {
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return false, storage.NewError("exists", uri, "oci", err)
	}

	request := objectstorage.HeadObjectRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
		ObjectName:    &ociURI.Object,
	}

	_, err = p.client.HeadObject(ctx, request)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, storage.NewError("exists", uri, "oci", err)
	}

	return true, nil
}

// List returns objects in a bucket with optional prefix
func (p *OCIProvider) List(ctx context.Context, uri string, opts ...storage.ListOption) ([]storage.ObjectInfo, error) {
	options := storage.BuildListOptions(opts...)
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return nil, storage.NewError("list", uri, "oci", err)
	}

	request := objectstorage.ListObjectsRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
	}

	// Set prefix from the object part of the URI
	if ociURI.Object != "" {
		request.Prefix = &ociURI.Object
	}

	// Apply list options
	if options.MaxResults > 0 {
		limit := options.MaxResults
		request.Limit = &limit
	}

	if options.Delimiter != "" {
		request.Delimiter = &options.Delimiter
	}

	if options.StartAfter != "" {
		request.Start = &options.StartAfter
	}

	var objects []storage.ObjectInfo
	for {
		response, err := p.client.ListObjects(ctx, request)
		if err != nil {
			return nil, storage.NewError("list", uri, "oci", err)
		}

		// Log response details
		p.logger.WithField("object_count", len(response.Objects)).Debug("Got ListObjects response")

		for _, obj := range response.Objects {
			// Name is absolutely required
			if obj.Name == nil {
				continue
			}

			info := storage.ObjectInfo{
				Name: *obj.Name,
			}

			// Size might be nil for directories/prefixes - default to 0
			if obj.Size != nil {
				info.Size = *obj.Size
			} else {
				info.Size = 0
			}

			// Optional fields - only set if present
			if obj.TimeCreated != nil {
				info.LastModified = obj.TimeCreated.Time
			}
			if obj.Etag != nil {
				info.ETag = *obj.Etag
			}
			// Note: OCI ListObjects doesn't return ContentType, leave it empty
			// ContentType would need to be fetched via GetObjectMetadata if needed
			objects = append(objects, info)
		}

		// Check if we need to continue pagination
		if response.NextStartWith == nil || *response.NextStartWith == "" {
			break
		}

		// Check if we've reached the max results
		if options.MaxResults > 0 && len(objects) >= options.MaxResults {
			objects = objects[:options.MaxResults]
			break
		}

		request.Start = response.NextStartWith
	}

	return objects, nil
}

// Stat returns metadata for an object
func (p *OCIProvider) Stat(ctx context.Context, uri string) (*storage.Metadata, error) {
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return nil, storage.NewError("stat", uri, "oci", err)
	}

	request := objectstorage.HeadObjectRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
		ObjectName:    &ociURI.Object,
	}

	response, err := p.client.HeadObject(ctx, request)
	if err != nil {
		return nil, storage.NewError("stat", uri, "oci", err)
	}

	metadata := &storage.Metadata{
		Name:         ociURI.Object,
		Size:         *response.ContentLength,
		ContentType:  *response.ContentType,
		ETag:         *response.ETag,
		LastModified: response.LastModified.Time,
		Metadata:     convertMetadataFromOCI(response.OpcMeta),
	}

	if response.StorageTier != "" {
		metadata.StorageClass = string(response.StorageTier)
	}

	return metadata, nil
}

// Copy copies an object within OCI Object Storage
func (p *OCIProvider) Copy(ctx context.Context, source string, target string) error {
	sourceURI, err := parseOCIURI(source, p.namespace, p.bucket)
	if err != nil {
		return storage.NewError("copy", source, "oci", err)
	}

	targetURI, err := parseOCIURI(target, p.namespace, p.bucket)
	if err != nil {
		return storage.NewError("copy", target, "oci", err)
	}

	// Use the copy object API
	copySource := fmt.Sprintf("/%s/%s/%s", sourceURI.Namespace, sourceURI.Bucket, sourceURI.Object)
	request := objectstorage.CopyObjectRequest{
		NamespaceName: &targetURI.Namespace,
		BucketName:    &targetURI.Bucket,
		CopyObjectDetails: objectstorage.CopyObjectDetails{
			SourceObjectName:      &copySource,
			DestinationRegion:     &p.region,
			DestinationNamespace:  &targetURI.Namespace,
			DestinationBucket:     &targetURI.Bucket,
			DestinationObjectName: &targetURI.Object,
		},
	}

	_, err = p.client.CopyObject(ctx, request)
	if err != nil {
		return storage.NewError("copy", source, "oci", err)
	}

	return nil
}

// Helper functions

func getAuthType(authConfig *storage.AuthConfig) auth.AuthType {
	if authConfig.Type != "" {
		return auth.AuthType(authConfig.Type)
	}
	// Default to instance principal
	return auth.OCIInstancePrincipal
}

func createObjectStorageClient(configProvider common.ConfigurationProvider, config storage.Config) (*objectstorage.ObjectStorageClient, error) {
	common.EnableInstanceMetadataServiceLookup()

	client, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, err
	}

	// Configure HTTP client for better performance
	client.BaseClient.HTTPClient = &http.Client{
		Timeout: 20 * time.Minute,
		Transport: &http.Transport{
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 200,
			MaxConnsPerHost:     200,
		},
	}

	return &client, nil
}

func shouldUseParallelDownload(size int64, options storage.DownloadOptions) bool {
	threshold := int64(defaultParallelDownloadThresholdMB * 1024 * 1024)
	// Allow override via options if we add a ForceSimple option later
	return size >= threshold && options.Concurrency > 0
}

func shouldUseMultipartUpload(size int64, options storage.UploadOptions) bool {
	threshold := int64(defaultMultipartUploadThresholdMB * 1024 * 1024)
	return size > threshold && options.PartSize > 0
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	// Check for OCI 404 error
	return strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "NotFound")
}

// simpleDownload performs a simple single-threaded download
func (p *OCIProvider) simpleDownload(ctx context.Context, source *ociURI, target string, size int64, options storage.DownloadOptions) error {
	// Ensure target directory exists
	targetDir := filepath.Dir(target)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Get the object
	request := objectstorage.GetObjectRequest{
		NamespaceName: &source.Namespace,
		BucketName:    &source.Bucket,
		ObjectName:    &source.Object,
	}

	// Apply range if specified
	if options.Range != nil {
		rangeHeader := fmt.Sprintf("bytes=%d-%d", options.Range.Start, options.Range.End)
		request.Range = &rangeHeader
	}

	response, err := p.client.GetObject(ctx, request)
	if err != nil {
		return err
	}
	defer response.Content.Close()

	// Create target file
	file, err := os.Create(target)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy with progress reporting
	if options.Progress != nil {
		written, err := storage.CopyWithProgress(ctx, file, response.Content, size, options.Progress)
		if err != nil {
			return err
		}
		p.logger.WithField("bytes", written).Debug("Downloaded object")
	} else {
		// Use buffer pool for efficient copying
		buf := BufferPool.Get().([]byte)
		_, err = io.CopyBuffer(file, response.Content, buf)
		BufferPool.Put(buf)
		if err != nil {
			return err
		}
	}

	// Verify ETag if specified
	if options.VerifyETag != "" && response.ETag != nil {
		if *response.ETag != options.VerifyETag {
			return fmt.Errorf("ETag mismatch: expected %s, got %s", options.VerifyETag, *response.ETag)
		}
	}

	// Verify MD5 integrity
	if err := p.verifyMD5(ctx, source, target); err != nil {
		// MD5 mismatch is critical - remove the corrupted file
		os.Remove(target)
		return fmt.Errorf("MD5 verification failed: %w", err)
	}

	return nil
}

// progressReader wraps an io.ReadCloser to report progress
type progressReader struct {
	reader   io.ReadCloser
	size     int64
	read     int64
	progress storage.ProgressReporter
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.read += int64(n)
		pr.progress.Update(pr.read, pr.size)
	}
	return n, err
}

func (pr *progressReader) Close() error {
	return pr.reader.Close()
}

// simpleUpload performs a simple single-part upload
func (p *OCIProvider) simpleUpload(ctx context.Context, source string, target *ociURI, size int64, options storage.UploadOptions) error {
	// Open source file
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create put request
	metadata := convertMetadataToOCI(options.Metadata)
	request := objectstorage.PutObjectRequest{
		NamespaceName: &target.Namespace,
		BucketName:    &target.Bucket,
		ObjectName:    &target.Object,
		PutObjectBody: file,
		ContentLength: &size,
		ContentType:   &options.ContentType,
		OpcMeta:       metadata,
	}

	if options.StorageClass != "" {
		request.StorageTier = objectstorage.PutObjectStorageTierEnum(options.StorageClass)
	}

	// Upload with progress if configured
	if options.Progress != nil {
		// Wrap the file reader with progress reporting
		progressReader := &progressReader{
			reader:   file,
			size:     size,
			progress: options.Progress,
		}
		request.PutObjectBody = progressReader
	}

	_, err = p.client.PutObject(ctx, request)
	if err != nil {
		if options.Progress != nil {
			options.Progress.Error(err)
		}
		return err
	}

	return nil
}
