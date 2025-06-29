package gcp

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	pkgstorage "github.com/sgl-project/ome/pkg/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCSStorage implements storage.Storage for Google Cloud Storage
type GCSStorage struct {
	client      gcsClient
	credentials auth.Credentials
	logger      logging.Interface
	config      *Config
}

// Config represents GCS storage configuration
type Config struct {
	ProjectID                string `json:"project_id"`
	Location                 string `json:"location"`
	StorageClass             string `json:"storage_class"`
	UniformBucketLevelAccess bool   `json:"uniform_bucket_level_access"`
	ChunkSize                int    `json:"chunk_size"` // in MB
	EnableCRC32C             bool   `json:"enable_crc32c"`
}

// DefaultConfig returns default GCS storage configuration
func DefaultConfig() *Config {
	return &Config{
		StorageClass: "STANDARD",
		ChunkSize:    16, // 16MB chunks
		EnableCRC32C: true,
	}
}

// New creates a new GCS storage instance
func New(ctx context.Context, cfg *Config, credentials auth.Credentials, logger logging.Interface) (*GCSStorage, error) {
	// Ensure we have GCP credentials
	gcpCreds, ok := credentials.(gcpCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type: expected GCP credentials")
	}

	// Apply defaults
	if cfg == nil {
		cfg = DefaultConfig()
	} else {
		defaultConfig := DefaultConfig()
		if cfg.StorageClass == "" {
			cfg.StorageClass = defaultConfig.StorageClass
		}
		if cfg.ChunkSize == 0 {
			cfg.ChunkSize = defaultConfig.ChunkSize
		}
	}

	// Get token source from credentials
	tokenSource := gcpCreds.GetTokenSource()

	// Create client options
	var clientOpts []option.ClientOption
	if tokenSource != nil {
		clientOpts = append(clientOpts, option.WithTokenSource(tokenSource))
	}

	// Create storage client
	client, err := storage.NewClient(ctx, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCS client: %w", err)
	}

	return &GCSStorage{
		client:      &clientWrapper{client},
		credentials: credentials,
		logger:      logger,
		config:      cfg,
	}, nil
}

// Provider returns the storage provider type
func (s *GCSStorage) Provider() pkgstorage.Provider {
	return pkgstorage.ProviderGCP
}

// Download retrieves the object and writes it to the target path
func (s *GCSStorage) Download(ctx context.Context, source pkgstorage.ObjectURI, target string, opts ...pkgstorage.DownloadOption) error {
	// Apply download options
	downloadOpts := pkgstorage.DefaultDownloadOptions()
	for _, opt := range opts {
		if err := opt(&downloadOpts); err != nil {
			return err
		}
	}

	// Compute actual target path based on download options
	actualTarget := target
	if downloadOpts.StripPrefix || downloadOpts.UseBaseNameOnly || downloadOpts.JoinWithTailOverlap {
		targetDir := filepath.Dir(target)
		actualTarget = pkgstorage.ComputeLocalPath(targetDir, source.ObjectName, downloadOpts)
	}

	// Check if we should skip existing valid files
	if !downloadOpts.DisableOverride {
		if exists, _ := pkgstorage.FileExists(actualTarget); exists {
			// Get object attributes for validation
			attrs, err := s.client.Bucket(source.BucketName).Object(source.ObjectName).Attrs(ctx)
			if err == nil {
				// Convert to storage.Metadata
				metadata := pkgstorage.Metadata{
					ObjectInfo: pkgstorage.ObjectInfo{
						Name: source.ObjectName,
						Size: attrs.Size,
					},
				}
				if attrs.Etag != "" {
					metadata.ETag = attrs.Etag
				}
				if attrs.MD5 != nil {
					// GCS returns MD5 as byte array, convert to base64
					metadata.ContentMD5 = base64.StdEncoding.EncodeToString(attrs.MD5)
				}

				if valid, _ := pkgstorage.IsLocalFileValid(actualTarget, metadata); valid {
					return nil // Skip download, file is already valid
				}
			}
		}
	}

	// Create directory if needed
	dir := filepath.Dir(actualTarget)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Check if we should use multipart download
	var useMultipart bool
	var objectSize int64

	if !downloadOpts.ForceStandard {
		// Get object size
		attrs, err := s.client.Bucket(source.BucketName).Object(source.ObjectName).Attrs(ctx)
		if err != nil {
			return fmt.Errorf("failed to get object info: %w", err)
		}
		objectSize = attrs.Size

		// Determine if we should use multipart
		if downloadOpts.ForceMultipart {
			useMultipart = true
		} else if objectSize > int64(downloadOpts.SizeThresholdInMB)*1024*1024 {
			useMultipart = true
		}
	}

	if useMultipart {
		// Use multipart download for large files
		return s.multipartDownload(ctx, source, actualTarget, objectSize, &downloadOpts)
	}

	// Use standard download for small files
	// Get object handle
	bucket := s.client.Bucket(source.BucketName)
	object := bucket.Object(source.ObjectName)

	// Create reader
	reader, err := object.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create reader: %w", err)
	}
	defer reader.Close()

	// Create file
	file, err := os.Create(actualTarget)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	return nil
}

// Upload stores the file at source path as the target object
func (s *GCSStorage) Upload(ctx context.Context, source string, target pkgstorage.ObjectURI, opts ...pkgstorage.UploadOption) error {
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
func (s *GCSStorage) Get(ctx context.Context, uri pkgstorage.ObjectURI) (io.ReadCloser, error) {
	bucket := s.client.Bucket(uri.BucketName)
	object := bucket.Object(uri.ObjectName)

	reader, err := object.NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	return reader, nil
}

// Put stores data from reader as an object
func (s *GCSStorage) Put(ctx context.Context, uri pkgstorage.ObjectURI, reader io.Reader, size int64, opts ...pkgstorage.UploadOption) error {
	uploadOpts := pkgstorage.DefaultUploadOptions()
	for _, opt := range opts {
		if err := opt(&uploadOpts); err != nil {
			return err
		}
	}

	// Get object handle
	bucket := s.client.Bucket(uri.BucketName)
	object := bucket.Object(uri.ObjectName)

	// Create writer
	gcsWriter := object.NewWriter(ctx)

	// We need to cast to access GCS-specific properties
	if writer, ok := gcsWriter.(*writerWrapper); ok {
		// Set options
		if uploadOpts.ContentType != "" {
			writer.Writer.ContentType = uploadOpts.ContentType
		}

		if uploadOpts.StorageClass != "" {
			writer.Writer.StorageClass = uploadOpts.StorageClass
		} else if s.config.StorageClass != "" {
			writer.Writer.StorageClass = s.config.StorageClass
		}

		if uploadOpts.Metadata != nil {
			writer.Writer.Metadata = uploadOpts.Metadata
		}

		// Set chunk size if specified
		if uploadOpts.ChunkSizeInMB > 0 {
			writer.Writer.ChunkSize = uploadOpts.ChunkSizeInMB * 1024 * 1024
		} else if s.config.ChunkSize > 0 {
			writer.Writer.ChunkSize = s.config.ChunkSize * 1024 * 1024
		}

		// Disable CRC32C if requested
		if !s.config.EnableCRC32C {
			writer.Writer.SendCRC32C = false
		}
	}

	// Copy data
	if _, err := io.Copy(gcsWriter, reader); err != nil {
		gcsWriter.Close()
		return fmt.Errorf("failed to write object: %w", err)
	}

	// Close writer to finalize upload
	if err := gcsWriter.Close(); err != nil {
		return fmt.Errorf("failed to finalize upload: %w", err)
	}

	return nil
}

// Delete removes an object
func (s *GCSStorage) Delete(ctx context.Context, uri pkgstorage.ObjectURI) error {
	bucket := s.client.Bucket(uri.BucketName)
	object := bucket.Object(uri.ObjectName)

	if err := object.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Exists checks if an object exists
func (s *GCSStorage) Exists(ctx context.Context, uri pkgstorage.ObjectURI) (bool, error) {
	bucket := s.client.Bucket(uri.BucketName)
	object := bucket.Object(uri.ObjectName)

	_, err := object.Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// List returns a list of objects matching the criteria
func (s *GCSStorage) List(ctx context.Context, uri pkgstorage.ObjectURI, opts pkgstorage.ListOptions) ([]pkgstorage.ObjectInfo, error) {
	bucket := s.client.Bucket(uri.BucketName)

	query := &storage.Query{}
	if opts.Prefix != "" {
		query.Prefix = opts.Prefix
	} else if uri.Prefix != "" {
		query.Prefix = uri.Prefix
	}

	if opts.Delimiter != "" {
		query.Delimiter = opts.Delimiter
	}

	var objects []pkgstorage.ObjectInfo
	it := bucket.Objects(ctx, query)
	count := 0

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		// Skip if we've reached MaxKeys
		if opts.MaxKeys > 0 && count >= opts.MaxKeys {
			break
		}

		// Skip if before StartAfter
		if opts.StartAfter != "" && attrs.Name <= opts.StartAfter {
			continue
		}

		info := pkgstorage.ObjectInfo{
			Name: attrs.Name,
			Size: attrs.Size,
		}

		if !attrs.Updated.IsZero() {
			info.LastModified = attrs.Updated.Format(time.RFC3339)
		}

		if attrs.Etag != "" {
			info.ETag = attrs.Etag
		}

		if attrs.ContentType != "" {
			info.ContentType = attrs.ContentType
		}

		if attrs.StorageClass != "" {
			info.StorageClass = attrs.StorageClass
		}

		objects = append(objects, info)
		count++
	}

	return objects, nil
}

// GetObjectInfo retrieves metadata about an object
func (s *GCSStorage) GetObjectInfo(ctx context.Context, uri pkgstorage.ObjectURI) (*pkgstorage.ObjectInfo, error) {
	bucket := s.client.Bucket(uri.BucketName)
	object := bucket.Object(uri.ObjectName)

	attrs, err := object.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	info := &pkgstorage.ObjectInfo{
		Name: attrs.Name,
		Size: attrs.Size,
	}

	if !attrs.Updated.IsZero() {
		info.LastModified = attrs.Updated.Format(time.RFC3339)
	}

	if attrs.Etag != "" {
		info.ETag = attrs.Etag
	}

	if attrs.ContentType != "" {
		info.ContentType = attrs.ContentType
	}

	if attrs.StorageClass != "" {
		info.StorageClass = attrs.StorageClass
	}

	if attrs.Metadata != nil {
		info.Metadata = attrs.Metadata
	}

	return info, nil
}

// Stat retrieves metadata about an object (alias for GetObjectInfo)
func (s *GCSStorage) Stat(ctx context.Context, uri pkgstorage.ObjectURI) (*pkgstorage.Metadata, error) {
	// First get the basic object info
	info, err := s.GetObjectInfo(ctx, uri)
	if err != nil {
		return nil, err
	}

	// Get additional metadata via Attrs
	bucket := s.client.Bucket(uri.BucketName)
	object := bucket.Object(uri.ObjectName)
	attrs, err := object.Attrs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Create Metadata struct with all fields
	metadata := &pkgstorage.Metadata{
		ObjectInfo: *info,
	}

	// Add additional metadata fields
	if attrs.CacheControl != "" {
		metadata.CacheControl = attrs.CacheControl
	}
	// GCS doesn't have a direct Expires field, but we can check custom metadata
	if attrs.Metadata != nil {
		if expires, ok := attrs.Metadata["expires"]; ok {
			metadata.Expires = expires
		}
	}
	// GCS uses generation numbers instead of version IDs
	if attrs.Generation > 0 {
		metadata.VersionID = fmt.Sprintf("%d", attrs.Generation)
	}
	if attrs.MD5 != nil && len(attrs.MD5) > 0 {
		metadata.ContentMD5 = base64.StdEncoding.EncodeToString(attrs.MD5)
	}

	// GCS doesn't directly expose multipart info
	// We'll leave IsMultipart as false and Parts as 0

	// Collect additional headers
	metadata.Headers = make(map[string]string)
	if attrs.ContentEncoding != "" {
		metadata.Headers["Content-Encoding"] = attrs.ContentEncoding
	}
	if attrs.ContentLanguage != "" {
		metadata.Headers["Content-Language"] = attrs.ContentLanguage
	}
	if attrs.ContentDisposition != "" {
		metadata.Headers["Content-Disposition"] = attrs.ContentDisposition
	}
	if attrs.Metageneration > 0 {
		metadata.Headers["x-goog-metageneration"] = fmt.Sprintf("%d", attrs.Metageneration)
	}

	return metadata, nil
}

// Copy copies an object within GCS
func (s *GCSStorage) Copy(ctx context.Context, source, target pkgstorage.ObjectURI) error {
	srcBucket := s.client.Bucket(source.BucketName)
	srcObject := srcBucket.Object(source.ObjectName)

	dstBucket := s.client.Bucket(target.BucketName)
	dstObject := dstBucket.Object(target.ObjectName)

	copier := dstObject.CopierFrom(srcObject)
	if _, err := copier.Run(ctx); err != nil {
		return fmt.Errorf("failed to copy object: %w", err)
	}

	return nil
}

// GCS doesn't have native multipart upload API like S3/OCI
// These methods provide compatibility but use standard upload internally

func (s *GCSStorage) InitiateMultipartUpload(ctx context.Context, uri pkgstorage.ObjectURI, opts ...pkgstorage.UploadOption) (string, error) {
	// Generate a unique upload ID
	uploadID := fmt.Sprintf("gcs-upload-%s-%s-%d", uri.BucketName, uri.ObjectName, time.Now().UnixNano())

	// Create multipart upload tracking
	s.createMultipartUpload(uploadID)

	return uploadID, nil
}

func (s *GCSStorage) UploadPart(ctx context.Context, uri pkgstorage.ObjectURI, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	// Get upload info
	uploadInfo, err := s.getMultipartUpload(uploadID)
	if err != nil {
		return "", err
	}

	// Create temporary object name for this part
	tempObjectName := fmt.Sprintf(".multipart/%s/part-%d", uploadID, partNumber)

	// Upload the part as a temporary object
	bucket := s.client.Bucket(uri.BucketName)
	object := bucket.Object(tempObjectName)
	writer := object.NewWriter(ctx)

	// Calculate MD5 while copying
	hasher := md5.New()
	multiReader := io.TeeReader(reader, hasher)

	// Copy data
	if _, err := io.Copy(writer, multiReader); err != nil {
		writer.Close()
		return "", fmt.Errorf("failed to write part: %w", err)
	}

	// Close writer to finalize upload
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to finalize part upload: %w", err)
	}

	// Store part info
	uploadInfo.mu.Lock()
	uploadInfo.Parts[partNumber] = tempObjectName
	uploadInfo.mu.Unlock()

	// Return MD5 as ETag
	return base64.StdEncoding.EncodeToString(hasher.Sum(nil)), nil
}

func (s *GCSStorage) CompleteMultipartUpload(ctx context.Context, uri pkgstorage.ObjectURI, uploadID string, parts []pkgstorage.CompletedPart) error {
	// Get upload info
	uploadInfo, err := s.getMultipartUpload(uploadID)
	if err != nil {
		return err
	}

	// Sort parts by part number
	var sourceObjects []*storage.ObjectHandle
	bucket := s.client.Bucket(uri.BucketName)

	// For composition, we need concrete storage.ObjectHandle types
	// We'll create them directly instead of using the interface
	concreteBucket, ok := s.client.(*clientWrapper)
	if !ok {
		return fmt.Errorf("failed to get concrete client")
	}
	concreteStorageBucket := concreteBucket.Client.Bucket(uri.BucketName)

	for _, part := range parts {
		tempObjectName, ok := uploadInfo.Parts[part.PartNumber]
		if !ok {
			return fmt.Errorf("part %d not found", part.PartNumber)
		}
		sourceObjects = append(sourceObjects, concreteStorageBucket.Object(tempObjectName))
	}

	// Compose the final object from parts
	dst := concreteStorageBucket.Object(uri.ObjectName)
	composer := dst.ComposerFrom(sourceObjects...)

	// Run the composition
	if _, err := composer.Run(ctx); err != nil {
		return fmt.Errorf("failed to compose object: %w", err)
	}

	// Clean up temporary objects
	for _, tempObjectName := range uploadInfo.Parts {
		if err := bucket.Object(tempObjectName).Delete(ctx); err != nil {
			s.logger.WithError(err).Debug("failed to delete temporary part")
		}
	}

	// Remove upload info
	s.deleteMultipartUpload(uploadID)

	return nil
}

func (s *GCSStorage) AbortMultipartUpload(ctx context.Context, uri pkgstorage.ObjectURI, uploadID string) error {
	// Get upload info
	uploadInfo, err := s.getMultipartUpload(uploadID)
	if err != nil {
		// Already aborted or doesn't exist
		return nil
	}

	// Clean up temporary objects
	bucket := s.client.Bucket(uri.BucketName)
	for _, tempObjectName := range uploadInfo.Parts {
		if err := bucket.Object(tempObjectName).Delete(ctx); err != nil {
			s.logger.WithError(err).Debug("failed to delete temporary part")
		}
	}

	// Remove upload info
	s.deleteMultipartUpload(uploadID)

	return nil
}

// Close closes the storage client
func (s *GCSStorage) Close() error {
	return s.client.Close()
}
