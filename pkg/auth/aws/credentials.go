package aws

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// AWSCredentials implements auth.Credentials for AWS
type AWSCredentials struct {
	credProvider aws.CredentialsProvider
	authType     auth.AuthType
	region       string
	logger       logging.Interface
	cachedCreds  *aws.Credentials
	cacheExpiry  time.Time
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
	service := "s3" // Default to S3, can be extended based on URL

	// Sign the request
	err = signer.SignHTTP(ctx, *creds, req, "", service, c.region, time.Now())
	if err != nil {
		return fmt.Errorf("failed to sign request: %w", err)
	}

	return nil
}

// Refresh refreshes the credentials
func (c *AWSCredentials) Refresh(ctx context.Context) error {
	// Clear cache to force refresh
	c.cachedCreds = nil
	c.cacheExpiry = time.Time{}

	// Try to get new credentials
	_, err := c.getCredentials(ctx)
	return err
}

// IsExpired checks if the credentials are expired
func (c *AWSCredentials) IsExpired() bool {
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
	// Check cache
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

// AccessKeyConfig represents AWS access key configuration
type AccessKeyConfig struct {
	AccessKeyID     string `mapstructure:"access_key_id" json:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key" json:"secret_access_key"`
	SessionToken    string `mapstructure:"session_token" json:"session_token,omitempty"`
}

// Validate validates the access key configuration
func (c *AccessKeyConfig) Validate() error {
	if c.AccessKeyID == "" {
		return fmt.Errorf("access_key_id is required")
	}
	if c.SecretAccessKey == "" {
		return fmt.Errorf("secret_access_key is required")
	}
	return nil
}

// AssumeRoleConfig represents AWS assume role configuration
type AssumeRoleConfig struct {
	RoleARN         string            `mapstructure:"role_arn" json:"role_arn"`
	RoleSessionName string            `mapstructure:"role_session_name" json:"role_session_name,omitempty"`
	ExternalID      string            `mapstructure:"external_id" json:"external_id,omitempty"`
	Duration        time.Duration     `mapstructure:"duration" json:"duration,omitempty"`
	Tags            map[string]string `mapstructure:"tags" json:"tags,omitempty"`
}

// Validate validates the assume role configuration
func (c *AssumeRoleConfig) Validate() error {
	if c.RoleARN == "" {
		return fmt.Errorf("role_arn is required")
	}
	return nil
}

// createStaticCredentialsProvider creates a static credentials provider
func createStaticCredentialsProvider(config AccessKeyConfig) aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider(
		config.AccessKeyID,
		config.SecretAccessKey,
		config.SessionToken,
	)
}

// WebIdentityConfig represents AWS web identity configuration
type WebIdentityConfig struct {
	RoleARN         string `mapstructure:"role_arn" json:"role_arn"`
	TokenFile       string `mapstructure:"token_file" json:"token_file"`
	RoleSessionName string `mapstructure:"role_session_name" json:"role_session_name,omitempty"`
}

// Validate validates the web identity configuration
func (c *WebIdentityConfig) Validate() error {
	if c.RoleARN == "" {
		return fmt.Errorf("role_arn is required for web identity")
	}
	if c.TokenFile == "" {
		return fmt.Errorf("token_file is required for web identity")
	}
	return nil
}
