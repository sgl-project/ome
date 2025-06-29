package aws

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// S3Storage implements storage.Storage for AWS S3
type S3Storage struct {
	client      s3Client
	downloader  *manager.Downloader
	uploader    *manager.Uploader
	credentials auth.Credentials
	logger      logging.Interface
	config      *Config
}

// Config represents S3 storage configuration
type Config struct {
	Region                  string `json:"region"`
	Endpoint                string `json:"endpoint"`
	ForcePathStyle          bool   `json:"force_path_style"`
	DisableSSL              bool   `json:"disable_ssl"`
	PartSize                int64  `json:"part_size"`
	Concurrency             int    `json:"concurrency"`
	DisableContentMD5       bool   `json:"disable_content_md5"`
	DisableComputeChecksums bool   `json:"disable_compute_checksums"`
}

// DefaultConfig returns default S3 storage configuration
func DefaultConfig() *Config {
	return &Config{
		PartSize:    5 * 1024 * 1024, // 5MB
		Concurrency: 10,
	}
}

// New creates a new S3 storage instance
func New(ctx context.Context, cfg *Config, credentials auth.Credentials, logger logging.Interface) (*S3Storage, error) {
	// Ensure we have AWS credentials
	awsCreds, ok := credentials.(awsCredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type: expected AWS credentials")
	}

	// Apply defaults
	if cfg == nil {
		cfg = DefaultConfig()
	} else {
		defaultConfig := DefaultConfig()
		if cfg.PartSize == 0 {
			cfg.PartSize = defaultConfig.PartSize
		}
		if cfg.Concurrency == 0 {
			cfg.Concurrency = defaultConfig.Concurrency
		}
	}

	// Create AWS config
	awsConfig, err := createAWSConfig(ctx, cfg, awsCreds)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	})

	// Create downloader and uploader
	downloader := manager.NewDownloader(client, func(d *manager.Downloader) {
		d.PartSize = cfg.PartSize
		d.Concurrency = cfg.Concurrency
	})

	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = cfg.PartSize
		u.Concurrency = cfg.Concurrency
		u.LeavePartsOnError = false
	})

	return &S3Storage{
		client:      client,
		downloader:  downloader,
		uploader:    uploader,
		credentials: credentials,
		logger:      logger,
		config:      cfg,
	}, nil
}

// Provider returns the storage provider type
func (s *S3Storage) Provider() storage.Provider {
	return storage.ProviderAWS
}

// Download retrieves the object and writes it to the target path
func (s *S3Storage) Download(ctx context.Context, source storage.ObjectURI, target string, opts ...storage.DownloadOption) error {
	// Apply download options
	downloadOpts := storage.DefaultDownloadOptions()
	for _, opt := range opts {
		if err := opt(&downloadOpts); err != nil {
			return err
		}
	}

	// Compute actual target path based on download options
	actualTarget := target
	if downloadOpts.StripPrefix || downloadOpts.UseBaseNameOnly || downloadOpts.JoinWithTailOverlap {
		targetDir := filepath.Dir(target)
		actualTarget = storage.ComputeLocalPath(targetDir, source.ObjectName, downloadOpts)
	}

	// Check if we should skip existing valid files
	if !downloadOpts.DisableOverride {
		if exists, _ := storage.FileExists(actualTarget); exists {
			// Get object metadata for validation
			headResp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
				Bucket: aws.String(source.BucketName),
				Key:    aws.String(source.ObjectName),
			})
			if err == nil {
				// Convert to storage.Metadata
				metadata := storage.Metadata{
					ObjectInfo: storage.ObjectInfo{
						Name: source.ObjectName,
						Size: *headResp.ContentLength,
					},
				}
				if headResp.ETag != nil {
					metadata.ETag = *headResp.ETag
				}
				// S3 returns MD5 in ETag for non-multipart uploads
				if headResp.ETag != nil && !strings.Contains(*headResp.ETag, "-") {
					// Remove quotes from ETag
					metadata.ContentMD5 = strings.Trim(*headResp.ETag, "\"")
				}

				if valid, _ := storage.IsLocalFileValid(actualTarget, metadata); valid {
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

	// Create file
	file, err := os.Create(actualTarget)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Configure download
	input := &s3.GetObjectInput{
		Bucket: aws.String(source.BucketName),
		Key:    aws.String(source.ObjectName),
	}

	// Download based on options
	if downloadOpts.ForceStandard || (downloadOpts.ForceMultipart == false && downloadOpts.SizeThresholdInMB > 0) {
		// Check object size first
		headResp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: input.Bucket,
			Key:    input.Key,
		})
		if err != nil {
			return fmt.Errorf("failed to get object info: %w", err)
		}

		if downloadOpts.ForceStandard || *headResp.ContentLength <= int64(downloadOpts.SizeThresholdInMB)*1024*1024 {
			// Use standard download
			resp, err := s.client.GetObject(ctx, input)
			if err != nil {
				return fmt.Errorf("failed to get object: %w", err)
			}
			defer resp.Body.Close()

			_, err = io.Copy(file, resp.Body)
			return err
		}
	}

	// Use multipart download
	downloader := manager.NewDownloader(s.client, func(d *manager.Downloader) {
		if downloadOpts.ChunkSizeInMB > 0 {
			d.PartSize = int64(downloadOpts.ChunkSizeInMB) * 1024 * 1024
		}
		if downloadOpts.Threads > 0 {
			d.Concurrency = downloadOpts.Threads
		}
	})

	_, err = downloader.Download(ctx, file, input)
	return err
}

// Upload stores the file at source path as the target object
func (s *S3Storage) Upload(ctx context.Context, source string, target storage.ObjectURI, opts ...storage.UploadOption) error {
	// Apply upload options
	uploadOpts := storage.DefaultUploadOptions()
	for _, opt := range opts {
		if err := opt(&uploadOpts); err != nil {
			return err
		}
	}

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
func (s *S3Storage) Get(ctx context.Context, uri storage.ObjectURI) (io.ReadCloser, error) {
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(uri.BucketName),
		Key:    aws.String(uri.ObjectName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	return resp.Body, nil
}

// Put stores data from reader as an object
func (s *S3Storage) Put(ctx context.Context, uri storage.ObjectURI, reader io.Reader, size int64, opts ...storage.UploadOption) error {
	uploadOpts := storage.DefaultUploadOptions()
	for _, opt := range opts {
		if err := opt(&uploadOpts); err != nil {
			return err
		}
	}

	// Configure upload
	input := &s3.PutObjectInput{
		Bucket: aws.String(uri.BucketName),
		Key:    aws.String(uri.ObjectName),
		Body:   reader,
	}

	if uploadOpts.ContentType != "" {
		input.ContentType = aws.String(uploadOpts.ContentType)
	}

	if uploadOpts.StorageClass != "" {
		input.StorageClass = types.StorageClass(uploadOpts.StorageClass)
	}

	if uploadOpts.Metadata != nil {
		input.Metadata = uploadOpts.Metadata
	}

	// Use uploader for large files or multipart
	if size > s.config.PartSize && s.uploader != nil {
		_, err := s.uploader.Upload(ctx, input)
		return err
	}

	// Use simple put for small files
	_, err := s.client.PutObject(ctx, input)
	return err
}

// Delete removes an object
func (s *S3Storage) Delete(ctx context.Context, uri storage.ObjectURI) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(uri.BucketName),
		Key:    aws.String(uri.ObjectName),
	})
	return err
}

// Exists checks if an object exists
func (s *S3Storage) Exists(ctx context.Context, uri storage.ObjectURI) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(uri.BucketName),
		Key:    aws.String(uri.ObjectName),
	})
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// List returns a list of objects matching the criteria
func (s *S3Storage) List(ctx context.Context, uri storage.ObjectURI, opts storage.ListOptions) ([]storage.ObjectInfo, error) {
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(uri.BucketName),
	}

	if opts.Prefix != "" {
		input.Prefix = aws.String(opts.Prefix)
	} else if uri.Prefix != "" {
		input.Prefix = aws.String(uri.Prefix)
	}

	if opts.Delimiter != "" {
		input.Delimiter = aws.String(opts.Delimiter)
	}

	if opts.StartAfter != "" {
		input.StartAfter = aws.String(opts.StartAfter)
	}

	if opts.MaxKeys > 0 {
		input.MaxKeys = aws.Int32(int32(opts.MaxKeys))
	}

	var objects []storage.ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(s.client, input)

	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range resp.Contents {
			info := storage.ObjectInfo{
				Name: *obj.Key,
				Size: *obj.Size,
			}
			if obj.LastModified != nil {
				info.LastModified = obj.LastModified.Format("2006-01-02T15:04:05Z")
			}
			if obj.ETag != nil {
				info.ETag = strings.Trim(*obj.ETag, "\"")
			}
			if obj.StorageClass != "" {
				info.StorageClass = string(obj.StorageClass)
			}
			objects = append(objects, info)
		}

		// If we've reached MaxKeys, stop
		if opts.MaxKeys > 0 && len(objects) >= opts.MaxKeys {
			break
		}
	}

	return objects, nil
}

// GetObjectInfo retrieves metadata about an object
func (s *S3Storage) GetObjectInfo(ctx context.Context, uri storage.ObjectURI) (*storage.ObjectInfo, error) {
	resp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(uri.BucketName),
		Key:    aws.String(uri.ObjectName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	info := &storage.ObjectInfo{
		Name: uri.ObjectName,
	}

	if resp.ContentLength != nil {
		info.Size = *resp.ContentLength
	}
	if resp.LastModified != nil {
		info.LastModified = resp.LastModified.Format("2006-01-02T15:04:05Z")
	}
	if resp.ETag != nil {
		info.ETag = strings.Trim(*resp.ETag, "\"")
	}
	if resp.ContentType != nil {
		info.ContentType = *resp.ContentType
	}
	if resp.StorageClass != "" {
		info.StorageClass = string(resp.StorageClass)
	}
	if resp.Metadata != nil {
		info.Metadata = resp.Metadata
	}

	return info, nil
}

// Stat retrieves metadata about an object (alias for GetObjectInfo)
func (s *S3Storage) Stat(ctx context.Context, uri storage.ObjectURI) (*storage.Metadata, error) {
	// First get the basic object info
	info, err := s.GetObjectInfo(ctx, uri)
	if err != nil {
		return nil, err
	}

	// Get additional metadata via HeadObject
	resp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(uri.BucketName),
		Key:    aws.String(uri.ObjectName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Create Metadata struct with all fields
	metadata := &storage.Metadata{
		ObjectInfo: *info,
	}

	// Add additional metadata fields
	if resp.CacheControl != nil {
		metadata.CacheControl = *resp.CacheControl
	}
	if resp.Expires != nil {
		metadata.Expires = resp.Expires.Format("2006-01-02T15:04:05Z")
	}
	if resp.VersionId != nil {
		metadata.VersionID = *resp.VersionId
	}
	if resp.PartsCount != nil && *resp.PartsCount > 1 {
		metadata.IsMultipart = true
		metadata.Parts = int(*resp.PartsCount)
	}

	// ContentMD5 is typically in the ETag for non-multipart objects
	if !metadata.IsMultipart && metadata.ETag != "" {
		metadata.ContentMD5 = metadata.ETag
	}

	// Collect additional headers
	metadata.Headers = make(map[string]string)
	if resp.ServerSideEncryption != "" {
		metadata.Headers["x-amz-server-side-encryption"] = string(resp.ServerSideEncryption)
	}
	if resp.WebsiteRedirectLocation != nil {
		metadata.Headers["x-amz-website-redirect-location"] = *resp.WebsiteRedirectLocation
	}

	return metadata, nil
}

// Copy copies an object within S3
func (s *S3Storage) Copy(ctx context.Context, source, target storage.ObjectURI) error {
	copySource := fmt.Sprintf("%s/%s", source.BucketName, source.ObjectName)

	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(target.BucketName),
		Key:        aws.String(target.ObjectName),
		CopySource: aws.String(copySource),
	})
	return err
}

// Multipart operations for S3Storage
func (s *S3Storage) InitiateMultipartUpload(ctx context.Context, uri storage.ObjectURI, opts ...storage.UploadOption) (string, error) {
	uploadOpts := storage.DefaultUploadOptions()
	for _, opt := range opts {
		if err := opt(&uploadOpts); err != nil {
			return "", err
		}
	}

	input := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(uri.BucketName),
		Key:    aws.String(uri.ObjectName),
	}

	if uploadOpts.ContentType != "" {
		input.ContentType = aws.String(uploadOpts.ContentType)
	}
	if uploadOpts.StorageClass != "" {
		input.StorageClass = types.StorageClass(uploadOpts.StorageClass)
	}
	if uploadOpts.Metadata != nil {
		input.Metadata = uploadOpts.Metadata
	}

	resp, err := s.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", err
	}

	return *resp.UploadId, nil
}

func (s *S3Storage) UploadPart(ctx context.Context, uri storage.ObjectURI, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	resp, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(uri.BucketName),
		Key:        aws.String(uri.ObjectName),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(int32(partNumber)),
		Body:       reader,
	})
	if err != nil {
		return "", err
	}

	return strings.Trim(*resp.ETag, "\""), nil
}

func (s *S3Storage) CompleteMultipartUpload(ctx context.Context, uri storage.ObjectURI, uploadID string, parts []storage.CompletedPart) error {
	var completedParts []types.CompletedPart
	for _, part := range parts {
		completedParts = append(completedParts, types.CompletedPart{
			PartNumber: aws.Int32(int32(part.PartNumber)),
			ETag:       aws.String(part.ETag),
		})
	}

	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(uri.BucketName),
		Key:      aws.String(uri.ObjectName),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	return err
}

func (s *S3Storage) AbortMultipartUpload(ctx context.Context, uri storage.ObjectURI, uploadID string) error {
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(uri.BucketName),
		Key:      aws.String(uri.ObjectName),
		UploadId: aws.String(uploadID),
	})
	return err
}

// Helper functions

func createAWSConfig(ctx context.Context, cfg *Config, creds awsCredentials) (aws.Config, error) {
	// Get credentials provider from our auth package
	credProvider := creds.GetCredentialsProvider()

	// Load config with our credentials
	awsConfig, err := config.LoadDefaultConfig(ctx,
		config.WithCredentialsProvider(credProvider),
	)
	if err != nil {
		return aws.Config{}, err
	}

	// Apply region if specified
	if cfg.Region != "" {
		awsConfig.Region = cfg.Region
	} else if region := creds.GetRegion(); region != "" {
		awsConfig.Region = region
	}

	return awsConfig, nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "NoSuchKey") ||
		strings.Contains(err.Error(), "NotFound") ||
		strings.Contains(err.Error(), "404")
}
