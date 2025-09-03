package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/sgl-project/ome/pkg/auth"
	awsauth "github.com/sgl-project/ome/pkg/auth/aws"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

const (
	// S3 specific constants
	defaultConcurrency                 = 10  // AWS SDK default
	defaultPartSize                    = 5   // 5MB - S3 minimum part size
	defaultParallelDownloadThresholdMB = 100 // 100MB threshold for parallel downloads
	maxRetries                         = 3
	httpTimeout                        = 10 * time.Minute
	maxIdleConns                       = 100
	bufferSize                         = 1024 * 1024 // 1MB buffer
)

// S3Provider implements the Storage interface for AWS S3
type S3Provider struct {
	client         *s3.Client
	bucket         string
	region         string
	endpoint       string // For S3-compatible services
	uploader       *manager.Uploader
	downloader     *manager.Downloader
	logger         logging.Interface
	bufferPool     *sync.Pool
	forcePathStyle bool // For S3-compatible services
}

// NewS3Provider creates a new S3 storage provider
func NewS3Provider(ctx context.Context, config storage.Config, logger logging.Interface) (storage.Storage, error) {
	if config.Provider != storage.ProviderS3 {
		return nil, fmt.Errorf("invalid provider: expected %s, got %s", storage.ProviderS3, config.Provider)
	}

	// Validate required configuration
	if config.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket is required")
	}

	// Initialize the S3 client
	client, err := initializeS3Client(ctx, config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Create uploader and downloader with appropriate options
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = defaultPartSize * 1024 * 1024
		u.Concurrency = defaultConcurrency
	})

	downloader := manager.NewDownloader(client, func(d *manager.Downloader) {
		d.PartSize = defaultPartSize * 1024 * 1024
		d.Concurrency = defaultConcurrency
	})

	// Initialize buffer pool for efficient memory usage
	bufferPool := &sync.Pool{
		New: func() interface{} {
			buf := make([]byte, bufferSize)
			return &buf
		},
	}

	provider := &S3Provider{
		client:     client,
		bucket:     config.Bucket,
		region:     config.Region,
		endpoint:   config.Endpoint,
		uploader:   uploader,
		downloader: downloader,
		logger:     logger,
		bufferPool: bufferPool,
	}

	// Check if we need path-style addressing (for S3-compatible services)
	if config.Endpoint != "" && !strings.Contains(config.Endpoint, "amazonaws.com") {
		provider.forcePathStyle = true
	}

	logger.WithField("provider", "s3").
		WithField("bucket", config.Bucket).
		WithField("region", config.Region).
		Info("S3 storage provider initialized")

	return provider, nil
}

// initializeS3Client creates and configures the S3 client
func initializeS3Client(ctx context.Context, config storage.Config, logger logging.Interface) (*s3.Client, error) {
	// Build AWS configuration options
	configOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithHTTPClient(&http.Client{
			Timeout: httpTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        maxIdleConns,
				MaxIdleConnsPerHost: maxIdleConns,
				IdleConnTimeout:     90 * time.Second,
			},
		}),
	}

	// Set region if specified
	if config.Region != "" {
		configOpts = append(configOpts, awsconfig.WithRegion(config.Region))
	}

	// Handle custom endpoint for S3-compatible services
	if config.Endpoint != "" {
		configOpts = append(configOpts, awsconfig.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{
						URL:               config.Endpoint,
						HostnameImmutable: true,
					}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			}),
		))
	}

	// Handle authentication if configured
	if config.AuthConfig != nil {
		creds, err := createAWSCredentials(ctx, config.AuthConfig, config.Region, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create AWS credentials: %w", err)
		}
		configOpts = append(configOpts, awsconfig.WithCredentialsProvider(creds))
	}

	// Load AWS configuration
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Configure retry logic
	awsCfg.RetryMode = aws.RetryModeStandard
	awsCfg.RetryMaxAttempts = maxRetries

	// Create S3 client with options
	clientOpts := []func(*s3.Options){
		func(o *s3.Options) {
			o.UsePathStyle = config.Endpoint != "" && !strings.Contains(config.Endpoint, "amazonaws.com")
		},
	}

	client := s3.NewFromConfig(awsCfg, clientOpts...)

	return client, nil
}

// createAWSCredentials creates AWS credentials based on auth configuration
func createAWSCredentials(ctx context.Context, authConfig *storage.AuthConfig, region string, logger logging.Interface) (aws.CredentialsProvider, error) {
	// Map storage auth type to AWS auth type
	var authType auth.AuthType
	switch authConfig.Type {
	case "access_key":
		authType = auth.AWSAccessKey
	case "assume_role":
		authType = auth.AWSAssumeRole
	case "instance_profile":
		authType = auth.AWSInstanceProfile
	case "web_identity":
		authType = auth.AWSWebIdentity
	case "ecs_task_role":
		authType = auth.AWSECSTaskRole
	case "process":
		authType = auth.AWSProcess
	case "default":
		authType = auth.AWSDefault
	default:
		// Default to AWS default credential chain
		authType = auth.AWSDefault
	}

	// Create auth configuration
	authCfg := auth.Config{
		Provider: auth.ProviderAWS,
		AuthType: authType,
		Region:   region,
		Extra:    authConfig.Extra,
	}

	// Create AWS credentials factory
	factory := awsauth.NewFactory(logger)

	// Create credentials
	creds, err := factory.Create(ctx, authCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS credentials: %w", err)
	}

	// Get the credentials provider
	awsCreds, ok := creds.(*awsauth.AWSCredentials)
	if !ok {
		return nil, fmt.Errorf("unexpected credentials type")
	}

	return awsCreds.GetCredentialsProvider(), nil
}

// Provider returns the provider type
func (p *S3Provider) Provider() storage.Provider {
	return storage.ProviderS3
}

// Download downloads a file from S3 to local filesystem
func (p *S3Provider) Download(ctx context.Context, source string, target string, opts ...storage.DownloadOption) error {
	// Parse S3 URI if needed
	key := source
	if strings.HasPrefix(source, "s3://") {
		_, parsedKey, err := parseS3URI(source)
		if err != nil {
			return err
		}
		key = parsedKey
	}

	// Build download options
	options := storage.BuildDownloadOptions(opts...)

	// Get object metadata first
	metadata, err := p.Stat(ctx, key)
	if err != nil {
		return err
	}

	// Check if we should use parallel download
	if metadata.Size > defaultParallelDownloadThresholdMB*1024*1024 && options.Concurrency > 0 {
		return p.downloadParallel(ctx, key, target, metadata.Size, options)
	}

	// Simple download for small files
	return p.downloadSimple(ctx, key, target)
}

// downloadSimple performs a simple download
func (p *S3Provider) downloadSimple(ctx context.Context, key string, target string) error {
	// Get the object
	reader, err := p.Get(ctx, key)
	if err != nil {
		return err
	}
	defer reader.Close()

	// Create the target file
	file, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer file.Close()

	// Copy the content
	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return nil
}

// downloadParallel performs parallel download (to be implemented)
func (p *S3Provider) downloadParallel(ctx context.Context, key string, target string, size int64, options storage.DownloadOptions) error {
	// This will be implemented in parallel.go
	return fmt.Errorf("parallel download not yet implemented")
}

// Upload uploads a file from local filesystem to S3
func (p *S3Provider) Upload(ctx context.Context, source string, target string, opts ...storage.UploadOption) error {
	// Parse S3 URI if needed
	key := target
	if strings.HasPrefix(target, "s3://") {
		_, parsedKey, err := parseS3URI(target)
		if err != nil {
			return err
		}
		key = parsedKey
	}

	// Build upload options
	options := storage.BuildUploadOptions(opts...)

	// Open the source file
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Upload the file
	return p.PutWithOptions(ctx, key, file, fileInfo.Size(), options)
}

// List lists objects in the bucket with optional prefix
func (p *S3Provider) List(ctx context.Context, prefix string, opts ...storage.ListOption) ([]storage.ObjectInfo, error) {
	// Parse S3 URI if needed
	if strings.HasPrefix(prefix, "s3://") {
		_, parsedKey, err := parseS3URI(prefix)
		if err != nil {
			return nil, err
		}
		prefix = parsedKey
	}

	// Build list options
	options := storage.BuildListOptions(opts...)

	var objects []storage.ObjectInfo

	input := &s3.ListObjectsV2Input{
		Bucket:     aws.String(p.bucket),
		Prefix:     aws.String(prefix),
		MaxKeys:    aws.Int32(int32(options.MaxResults)),
		Delimiter:  aws.String(options.Delimiter),
		StartAfter: aws.String(options.StartAfter),
	}

	paginator := s3.NewListObjectsV2Paginator(p.client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, p.wrapError(err, "failed to list objects")
		}

		for _, obj := range page.Contents {
			// Key is always required in S3
			if obj.Key == nil {
				continue
			}

			info := storage.ObjectInfo{
				Name:         *obj.Key,
				Size:         aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified),
				ContentType:  "", // Will be populated if needed via HeadObject
			}

			// ETag might be nil in some cases
			if obj.ETag != nil {
				info.ETag = strings.Trim(*obj.ETag, "\"")
			}

			objects = append(objects, info)
		}

		// Handle common prefixes (directories)
		for _, prefix := range page.CommonPrefixes {
			if prefix.Prefix != nil {
				objects = append(objects, storage.ObjectInfo{
					Name:  *prefix.Prefix,
					IsDir: true,
				})
			}
		}
	}

	return objects, nil
}

// Get retrieves an object from S3
func (p *S3Provider) Get(ctx context.Context, uri string) (io.ReadCloser, error) {
	// Parse S3 URI if needed
	key := uri
	if strings.HasPrefix(uri, "s3://") {
		_, parsedKey, err := parseS3URI(uri)
		if err != nil {
			return nil, err
		}
		key = parsedKey
	}

	result, err := p.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, p.wrapError(err, "failed to get object")
	}

	return result.Body, nil
}

// Put uploads an object to S3
func (p *S3Provider) Put(ctx context.Context, uri string, reader io.Reader, size int64, opts ...storage.UploadOption) error {
	// Parse S3 URI if needed
	key := uri
	if strings.HasPrefix(uri, "s3://") {
		_, parsedKey, err := parseS3URI(uri)
		if err != nil {
			return err
		}
		key = parsedKey
	}

	// Build upload options
	options := storage.BuildUploadOptions(opts...)

	return p.PutWithOptions(ctx, key, reader, size, options)
}

// PutWithOptions uploads an object with upload options
func (p *S3Provider) PutWithOptions(ctx context.Context, key string, reader io.Reader, size int64, options storage.UploadOptions) error {
	// Set content type if provided
	contentType := options.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// For large files, use the upload manager (multipart)
	// For small files, use direct PutObject
	if size > defaultParallelDownloadThresholdMB*1024*1024 {
		return p.putMultipart(ctx, key, reader, contentType, options.Metadata)
	}

	return p.putDirect(ctx, key, reader, size, contentType, options.Metadata)
}

// putDirect uploads small objects directly using PutObject
func (p *S3Provider) putDirect(ctx context.Context, key string, reader io.Reader, size int64, contentType string, metadata map[string]string) error {
	// Read the content into memory for small files
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	input := &s3.PutObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	}

	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	if len(metadata) > 0 {
		input.Metadata = ConvertMetadataToS3(metadata)
	}

	_, err = p.client.PutObject(ctx, input)
	if err != nil {
		return p.wrapError(err, "failed to put object")
	}

	return nil
}

// putMultipart uploads large objects using multipart upload
func (p *S3Provider) putMultipart(ctx context.Context, key string, reader io.Reader, contentType string, metadata map[string]string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
		Body:   reader,
	}

	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	if len(metadata) > 0 {
		input.Metadata = ConvertMetadataToS3(metadata)
	}

	_, err := p.uploader.Upload(ctx, input)
	if err != nil {
		return p.wrapError(err, "failed to upload object")
	}

	return nil
}

// Delete removes an object from S3
func (p *S3Provider) Delete(ctx context.Context, uri string) error {
	// Parse S3 URI if needed
	key := uri
	if strings.HasPrefix(uri, "s3://") {
		_, parsedKey, err := parseS3URI(uri)
		if err != nil {
			return err
		}
		key = parsedKey
	}

	_, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return p.wrapError(err, "failed to delete object")
	}

	return nil
}

// Exists checks if an object exists in S3
func (p *S3Provider) Exists(ctx context.Context, uri string) (bool, error) {
	// Parse S3 URI if needed
	key := uri
	if strings.HasPrefix(uri, "s3://") {
		_, parsedKey, err := parseS3URI(uri)
		if err != nil {
			return false, err
		}
		key = parsedKey
	}

	_, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound" {
				return false, nil
			}
		}
		return false, p.wrapError(err, "failed to check object existence")
	}

	return true, nil
}

// Stat retrieves object metadata from S3
func (p *S3Provider) Stat(ctx context.Context, uri string) (*storage.Metadata, error) {
	// Parse S3 URI if needed
	key := uri
	if strings.HasPrefix(uri, "s3://") {
		_, parsedKey, err := parseS3URI(uri)
		if err != nil {
			return nil, err
		}
		key = parsedKey
	}

	result, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, p.wrapError(err, "failed to get object metadata")
	}

	metadata := &storage.Metadata{
		Name:         key,
		Size:         aws.ToInt64(result.ContentLength),
		ContentType:  aws.ToString(result.ContentType),
		LastModified: aws.ToTime(result.LastModified),
		Metadata:     make(map[string]string),
	}

	// Copy custom metadata
	for k, v := range result.Metadata {
		metadata.Metadata[k] = v
	}

	// ETag
	if result.ETag != nil {
		metadata.ETag = strings.Trim(*result.ETag, "\"")
	}

	// Storage class
	if result.StorageClass != "" {
		metadata.StorageClass = string(result.StorageClass)
	}

	return metadata, nil
}

// Copy copies an object within S3
func (p *S3Provider) Copy(ctx context.Context, source, target string) error {
	// Parse S3 URIs if needed
	sourceKey := source
	if strings.HasPrefix(source, "s3://") {
		_, parsedKey, err := parseS3URI(source)
		if err != nil {
			return err
		}
		sourceKey = parsedKey
	}

	destKey := target
	if strings.HasPrefix(target, "s3://") {
		_, parsedKey, err := parseS3URI(target)
		if err != nil {
			return err
		}
		destKey = parsedKey
	}

	copySource := fmt.Sprintf("%s/%s", p.bucket, sourceKey)

	_, err := p.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(p.bucket),
		Key:        aws.String(destKey),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		return p.wrapError(err, "failed to copy object")
	}

	return nil
}

// wrapError wraps S3 errors with additional context
func (p *S3Provider) wrapError(err error, msg string) error {
	if err == nil {
		return nil
	}

	// Check for smithy API errors
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound":
			return storage.ErrNotFound
		case "NoSuchBucket":
			return fmt.Errorf("%s: bucket not found: %w", msg, err)
		case "AccessDenied":
			return fmt.Errorf("%s: access denied: %w", msg, err)
		default:
			return fmt.Errorf("%s: %s: %w", msg, apiErr.ErrorCode(), err)
		}
	}

	return fmt.Errorf("%s: %w", msg, err)
}
