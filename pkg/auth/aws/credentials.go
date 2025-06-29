package aws

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// AWSCredentials implements auth.Credentials for AWS
type AWSCredentials struct {
	credProvider aws.CredentialsProvider
	authType     auth.AuthType
	region       string
	logger       logging.Interface

	// Mutex protects cached credentials
	mu          sync.RWMutex
	cachedCreds *aws.Credentials
	cacheExpiry time.Time
}

// Provider returns the provider type
func (c *AWSCredentials) Provider() auth.Provider {
	return auth.ProviderAWS
}

// Type returns the authentication type
func (c *AWSCredentials) Type() auth.AuthType {
	return c.authType
}

// Token retrieves the AWS credentials as a token string
func (c *AWSCredentials) Token(ctx context.Context) (string, error) {
	creds, err := c.getCredentials(ctx)
	if err != nil {
		return "", err
	}

	// Return formatted token (access key for identification)
	return creds.AccessKeyID, nil
}

// SignRequest signs an HTTP request with AWS v4 signature
func (c *AWSCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	creds, err := c.getCredentials(ctx)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	// Create a signer
	signer := v4.NewSigner()

	// Determine service from host
	service := extractServiceFromHost(req.Host)

	// Calculate payload hash (empty for GET requests, unsigned for others)
	payloadHash := "UNSIGNED-PAYLOAD"
	if req.Method == http.MethodGet || req.Method == http.MethodHead {
		payloadHash = ""
	}

	// Sign the request
	err = signer.SignHTTP(ctx, *creds, req, payloadHash, service, c.region, time.Now())
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	return nil
}

// Refresh refreshes the credentials
func (c *AWSCredentials) Refresh(ctx context.Context) error {
	// Clear cache to force refresh
	c.mu.Lock()
	c.cachedCreds = nil
	c.cacheExpiry = time.Time{}
	c.mu.Unlock()

	// Try to get new credentials
	_, err := c.getCredentials(ctx)
	return err
}

// IsExpired checks if the credentials are expired
func (c *AWSCredentials) IsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.cachedCreds == nil {
		return true
	}
	return time.Now().After(c.cacheExpiry)
}

// GetRegion returns the AWS region
func (c *AWSCredentials) GetRegion() string {
	return c.region
}

// GetCredentialsProvider returns the underlying AWS credentials provider
func (c *AWSCredentials) GetCredentialsProvider() aws.CredentialsProvider {
	return c.credProvider
}

// getCredentials retrieves and caches AWS credentials
func (c *AWSCredentials) getCredentials(ctx context.Context) (*aws.Credentials, error) {
	// Check cache with read lock
	c.mu.RLock()
	if c.cachedCreds != nil && time.Now().Before(c.cacheExpiry) {
		creds := *c.cachedCreds
		c.mu.RUnlock()
		return &creds, nil
	}
	c.mu.RUnlock()

	// Need to refresh - acquire write lock
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.cachedCreds != nil && time.Now().Before(c.cacheExpiry) {
		return c.cachedCreds, nil
	}

	// Retrieve new credentials
	creds, err := c.credProvider.Retrieve(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve credentials: %w", err)
	}

	// Cache credentials
	c.cachedCreds = &creds
	if creds.Expires.IsZero() {
		// If no expiry, cache for 1 hour
		c.cacheExpiry = time.Now().Add(1 * time.Hour)
	} else {
		// Cache until 5 minutes before expiry
		c.cacheExpiry = creds.Expires.Add(-5 * time.Minute)
	}

	return &creds, nil
}

// extractServiceFromHost extracts the AWS service name from the host
func extractServiceFromHost(host string) string {
	// Remove port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Extract service from standard AWS domain pattern
	// Examples: s3.amazonaws.com, dynamodb.us-east-1.amazonaws.com
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		// Check for service.region.amazonaws.com pattern
		if len(parts) >= 3 && parts[len(parts)-2] == "amazonaws" {
			return parts[0]
		}
		// Check for service.amazonaws.com pattern
		if parts[1] == "amazonaws" {
			return parts[0]
		}
	}

	// Default to s3 for unknown patterns
	return "s3"
}
