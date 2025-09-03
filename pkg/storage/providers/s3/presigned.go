package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// GeneratePresignedGetURL generates a presigned URL for downloading an object
func (p *S3Provider) GeneratePresignedGetURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	// Create a presign client
	presignClient := s3.NewPresignClient(p.client)

	// Create the request
	request := &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}

	// Generate the presigned URL
	presignedReq, err := presignClient.PresignGetObject(ctx, request, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned GET URL: %w", err)
	}

	return presignedReq.URL, nil
}

// GeneratePresignedPutURL generates a presigned URL for uploading an object
func (p *S3Provider) GeneratePresignedPutURL(ctx context.Context, key string, expiry time.Duration, contentType string) (string, error) {
	// Create a presign client
	presignClient := s3.NewPresignClient(p.client)

	// Create the request
	request := &s3.PutObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}

	// Set content type if provided
	if contentType != "" {
		request.ContentType = aws.String(contentType)
	}

	// Generate the presigned URL
	presignedReq, err := presignClient.PresignPutObject(ctx, request, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned PUT URL: %w", err)
	}

	return presignedReq.URL, nil
}

// GeneratePresignedDeleteURL generates a presigned URL for deleting an object
func (p *S3Provider) GeneratePresignedDeleteURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	// Create a presign client
	presignClient := s3.NewPresignClient(p.client)

	// Create the request
	request := &s3.DeleteObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}

	// Generate the presigned URL
	presignedReq, err := presignClient.PresignDeleteObject(ctx, request, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned DELETE URL: %w", err)
	}

	return presignedReq.URL, nil
}

// GeneratePresignedPostURL generates presigned POST data for browser-based uploads
// Note: AWS SDK v2 doesn't have direct POST policy support like v1
// This would need custom implementation or use of v1 SDK for POST policies
func (p *S3Provider) GeneratePresignedPostURL(ctx context.Context, key string, expiry time.Duration, conditions []interface{}) (map[string]string, error) {
	// AWS SDK v2 doesn't have direct POST policy support
	// Return an error for now
	return nil, fmt.Errorf("presigned POST not yet implemented in AWS SDK v2")
}

// PresignedURLOptions contains options for presigned URL generation
type PresignedURLOptions struct {
	Expiry                     time.Duration
	ContentType                string
	Metadata                   map[string]string
	ResponseContentDisposition string
	ResponseContentType        string
}

// GeneratePresignedURL generates a presigned URL with custom options
func (p *S3Provider) GeneratePresignedURL(ctx context.Context, operation string, key string, options PresignedURLOptions) (string, error) {
	// Default expiry to 1 hour if not specified
	if options.Expiry == 0 {
		options.Expiry = time.Hour
	}

	switch operation {
	case "GET", "get":
		return p.generatePresignedGetWithOptions(ctx, key, options)
	case "PUT", "put":
		return p.generatePresignedPutWithOptions(ctx, key, options)
	case "DELETE", "delete":
		return p.GeneratePresignedDeleteURL(ctx, key, options.Expiry)
	default:
		return "", fmt.Errorf("unsupported operation: %s", operation)
	}
}

// generatePresignedGetWithOptions generates a presigned GET URL with custom options
func (p *S3Provider) generatePresignedGetWithOptions(ctx context.Context, key string, options PresignedURLOptions) (string, error) {
	// Create a presign client
	presignClient := s3.NewPresignClient(p.client)

	// Create the request
	request := &s3.GetObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}

	// Set response content disposition if provided
	if options.ResponseContentDisposition != "" {
		request.ResponseContentDisposition = aws.String(options.ResponseContentDisposition)
	}

	// Set response content type if provided
	if options.ResponseContentType != "" {
		request.ResponseContentType = aws.String(options.ResponseContentType)
	}

	// Generate the presigned URL
	presignedReq, err := presignClient.PresignGetObject(ctx, request, func(opts *s3.PresignOptions) {
		opts.Expires = options.Expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned GET URL: %w", err)
	}

	return presignedReq.URL, nil
}

// generatePresignedPutWithOptions generates a presigned PUT URL with custom options
func (p *S3Provider) generatePresignedPutWithOptions(ctx context.Context, key string, options PresignedURLOptions) (string, error) {
	// Create a presign client
	presignClient := s3.NewPresignClient(p.client)

	// Create the request
	request := &s3.PutObjectInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}

	// Set content type if provided
	if options.ContentType != "" {
		request.ContentType = aws.String(options.ContentType)
	}

	// Set metadata if provided
	if len(options.Metadata) > 0 {
		request.Metadata = options.Metadata
	}

	// Generate the presigned URL
	presignedReq, err := presignClient.PresignPutObject(ctx, request, func(opts *s3.PresignOptions) {
		opts.Expires = options.Expiry
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned PUT URL: %w", err)
	}

	return presignedReq.URL, nil
}

// GetPresignedURLForDownload generates a presigned URL for downloading (convenience method)
func (p *S3Provider) GetPresignedURLForDownload(ctx context.Context, key string) (string, error) {
	return p.GeneratePresignedGetURL(ctx, key, time.Hour)
}

// GetPresignedURLForUpload generates a presigned URL for uploading (convenience method)
func (p *S3Provider) GetPresignedURLForUpload(ctx context.Context, key string) (string, error) {
	return p.GeneratePresignedPutURL(ctx, key, time.Hour, "")
}
