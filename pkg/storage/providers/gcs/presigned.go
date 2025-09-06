package gcs

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"cloud.google.com/go/storage"
)

// PresignedURLOptions configures presigned URL generation
type PresignedURLOptions struct {
	Method      string            // HTTP method (GET, PUT, POST, DELETE)
	Expires     time.Duration     // URL expiration duration
	ContentType string            // Content-Type for PUT requests
	Headers     map[string]string // Additional headers to sign
}

// GetPresignedURL generates a presigned URL for temporary access to an object
func (p *Provider) GetPresignedURL(ctx context.Context, uri string, expiry time.Duration) (string, error) {
	bucket, objectName, err := parseGCSURI(uri)
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %w", err)
	}

	// Default to GET method
	return p.GeneratePresignedURL(ctx, bucket, objectName, &PresignedURLOptions{
		Method:  http.MethodGet,
		Expires: expiry,
	})
}

// GeneratePresignedURL generates a presigned URL with custom options
func (p *Provider) GeneratePresignedURL(ctx context.Context, bucketName, objectName string, opts *PresignedURLOptions) (string, error) {
	if opts == nil {
		opts = &PresignedURLOptions{
			Method:  http.MethodGet,
			Expires: 15 * time.Minute,
		}
	}

	// Default expiry if not specified
	if opts.Expires == 0 {
		opts.Expires = 15 * time.Minute
	}

	// Validate expiry duration (GCS max is 7 days)
	maxExpiry := 7 * 24 * time.Hour
	if opts.Expires > maxExpiry {
		return "", fmt.Errorf("expiry duration exceeds maximum of 7 days")
	}

	// Prepare signing options
	signOpts := &storage.SignedURLOptions{
		Method:  opts.Method,
		Expires: time.Now().Add(opts.Expires),
	}

	// Add content type if specified (for PUT requests)
	if opts.ContentType != "" && opts.Method == http.MethodPut {
		signOpts.ContentType = opts.ContentType
	}

	// Add custom headers if provided
	if len(opts.Headers) > 0 {
		signOpts.Headers = make([]string, 0, len(opts.Headers))
		for key, value := range opts.Headers {
			signOpts.Headers = append(signOpts.Headers, fmt.Sprintf("%s:%s", key, value))
		}
	}

	// Generate the signed URL
	signedURL, err := storage.SignedURL(bucketName, objectName, signOpts)
	if err != nil {
		return "", fmt.Errorf("failed to generate signed URL: %w", err)
	}

	p.logger.WithField("bucket", bucketName).
		WithField("object", objectName).
		WithField("method", opts.Method).
		WithField("expires", opts.Expires).
		Debug("Generated presigned URL")

	return signedURL, nil
}

// GetPresignedDownloadURL generates a presigned URL for downloading an object
func (p *Provider) GetPresignedDownloadURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	return p.GeneratePresignedURL(ctx, bucketName, objectName, &PresignedURLOptions{
		Method:  http.MethodGet,
		Expires: expiry,
	})
}

// GetPresignedUploadURL generates a presigned URL for uploading an object
func (p *Provider) GetPresignedUploadURL(ctx context.Context, bucketName, objectName string, expiry time.Duration, contentType string) (string, error) {
	return p.GeneratePresignedURL(ctx, bucketName, objectName, &PresignedURLOptions{
		Method:      http.MethodPut,
		Expires:     expiry,
		ContentType: contentType,
	})
}

// GeneratePostPolicy generates a POST policy for browser-based uploads
// This is useful for direct browser uploads to GCS
func (p *Provider) GeneratePostPolicy(ctx context.Context, bucketName string, conditions map[string]interface{}, expiry time.Duration) (*PostPolicyResult, error) {
	if expiry == 0 {
		expiry = 1 * time.Hour
	}

	// Create expiration time
	expiration := time.Now().Add(expiry)

	// Build policy conditions
	policyConditions := []storage.PostPolicyV4Condition{}

	// Add bucket condition
	policyConditions = append(policyConditions, storage.ConditionContentLengthRange(0, 1024*1024*100)) // 100MB max

	// Add custom conditions
	objectKey := ""
	for key, value := range conditions {
		switch key {
		case "key":
			if v, ok := value.(string); ok {
				objectKey = v
				policyConditions = append(policyConditions, storage.ConditionStartsWith("key", objectKey))
			}
		case "content-type":
			// Content type condition - using starts-with condition
			if v, ok := value.(string); ok {
				policyConditions = append(policyConditions, storage.ConditionStartsWith("content-type", v))
			}
		case "content-length-range":
			if rangeVals, ok := value.([]int64); ok && len(rangeVals) == 2 {
				policyConditions = append(policyConditions, storage.ConditionContentLengthRange(uint64(rangeVals[0]), uint64(rangeVals[1])))
			}
		}
	}

	// Ensure we have an object key
	if objectKey == "" {
		return nil, fmt.Errorf("object key is required in conditions")
	}

	// Generate the signed POST policy
	policy, err := storage.GenerateSignedPostPolicyV4(
		bucketName,
		objectKey,
		&storage.PostPolicyV4Options{
			Expires:    expiration,
			Conditions: policyConditions,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate POST policy: %w", err)
	}

	result := &PostPolicyResult{
		URL:    policy.URL,
		Fields: policy.Fields,
		Expiry: expiration,
	}

	p.logger.WithField("bucket", bucketName).
		WithField("expiry", expiry).
		Debug("Generated POST policy")

	return result, nil
}

// PostPolicyResult contains the result of generating a POST policy
type PostPolicyResult struct {
	URL    string            // The URL to POST to
	Fields map[string]string // Form fields to include in the POST
	Expiry time.Time         // When the policy expires
}

// ValidatePresignedURL validates that a presigned URL is still valid
func (p *Provider) ValidatePresignedURL(ctx context.Context, signedURL string) error {
	// Parse the URL
	parsedURL, err := url.Parse(signedURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check if it's a GCS URL
	if parsedURL.Host != "storage.googleapis.com" && !isGCSHost(parsedURL.Host) {
		return fmt.Errorf("not a valid GCS URL")
	}

	// Try to get the expiry from the URL parameters
	query := parsedURL.Query()
	if expiresStr := query.Get("Expires"); expiresStr != "" {
		// Parse Unix timestamp
		var expires int64
		if _, err := fmt.Sscanf(expiresStr, "%d", &expires); err == nil {
			expiryTime := time.Unix(expires, 0)
			if time.Now().After(expiryTime) {
				return fmt.Errorf("URL has expired")
			}
		}
	}

	// Make a HEAD request to validate the URL
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, signedURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to validate URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("URL is not valid or has expired")
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("URL validation failed with status: %s", resp.Status)
	}

	return nil
}

// isGCSHost checks if the hostname is a valid GCS host
func isGCSHost(host string) bool {
	// Check for various GCS host patterns
	validPatterns := []string{
		"storage.googleapis.com",
		".storage.googleapis.com",
		"storage.cloud.google.com",
		".googleusercontent.com",
	}

	for _, pattern := range validPatterns {
		if host == pattern || (len(pattern) > 0 && pattern[0] == '.' && len(host) > len(pattern) && host[len(host)-len(pattern):] == pattern) {
			return true
		}
	}

	return false
}

// RevokePresignedURL revokes a presigned URL (if supported)
// Note: GCS doesn't support direct revocation of signed URLs, but we can
// implement application-level revocation tracking if needed
func (p *Provider) RevokePresignedURL(ctx context.Context, signedURL string) error {
	p.logger.Warn("GCS does not support direct revocation of signed URLs")
	// In a production system, you might implement an application-level
	// revocation list that's checked when URLs are used
	return fmt.Errorf("presigned URL revocation not supported by GCS")
}
