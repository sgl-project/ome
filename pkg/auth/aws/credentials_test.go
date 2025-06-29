package aws

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// mockCredentialsProvider implements aws.CredentialsProvider for testing
type mockCredentialsProvider struct {
	creds       aws.Credentials
	retrieveErr error
}

func (m *mockCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	if m.retrieveErr != nil {
		return aws.Credentials{}, m.retrieveErr
	}
	return m.creds, nil
}

func TestAWSCredentials_Provider(t *testing.T) {
	creds := &AWSCredentials{
		authType: auth.AWSAccessKey,
		region:   "us-east-1",
		logger:   logging.NewNopLogger(),
	}

	if provider := creds.Provider(); provider != auth.ProviderAWS {
		t.Errorf("Expected provider %s, got %s", auth.ProviderAWS, provider)
	}
}

func TestAWSCredentials_Type(t *testing.T) {
	tests := []struct {
		name     string
		authType auth.AuthType
	}{
		{
			name:     "Access Key",
			authType: auth.AWSAccessKey,
		},
		{
			name:     "Assume Role",
			authType: auth.AWSAssumeRole,
		},
		{
			name:     "Instance Profile",
			authType: auth.AWSInstanceProfile,
		},
		{
			name:     "Web Identity",
			authType: auth.AWSWebIdentity,
		},
		{
			name:     "Default",
			authType: auth.AWSDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &AWSCredentials{
				authType: tt.authType,
			}
			if typ := creds.Type(); typ != tt.authType {
				t.Errorf("Expected type %s, got %s", tt.authType, typ)
			}
		})
	}
}

func TestAWSCredentials_GetRegion(t *testing.T) {
	creds := &AWSCredentials{
		region: "us-west-2",
	}

	if region := creds.GetRegion(); region != "us-west-2" {
		t.Errorf("Expected region us-west-2, got %s", region)
	}
}

func TestAWSCredentials_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		creds    *AWSCredentials
		expected bool
	}{
		{
			name:     "No cached credentials",
			creds:    &AWSCredentials{},
			expected: true,
		},
		{
			name: "Valid credentials",
			creds: &AWSCredentials{
				cachedCreds: &aws.Credentials{
					AccessKeyID:     "test",
					SecretAccessKey: "test",
				},
				cacheExpiry: time.Now().Add(1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "Expired credentials",
			creds: &AWSCredentials{
				cachedCreds: &aws.Credentials{
					AccessKeyID:     "test",
					SecretAccessKey: "test",
				},
				cacheExpiry: time.Now().Add(-1 * time.Hour),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.creds.IsExpired()
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestAWSCredentials_SignRequest(t *testing.T) {
	creds := &AWSCredentials{
		credProvider: createStaticCredentialsProvider(AccessKeyConfig{
			AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
			SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		}),
		region: "us-east-1",
		logger: logging.NewNopLogger(),
	}

	req, _ := http.NewRequest("GET", "https://s3.amazonaws.com/test-bucket/test-key", nil)
	ctx := context.Background()

	err := creds.SignRequest(ctx, req)
	if err != nil {
		t.Errorf("Failed to sign request: %v", err)
	}

	// Check that Authorization header was added
	if req.Header.Get("Authorization") == "" {
		t.Error("Expected Authorization header to be set")
	}
}

func TestAWSCredentials_SignRequest_Error(t *testing.T) {
	ctx := context.Background()
	mockProvider := &mockCredentialsProvider{
		retrieveErr: errors.New("test error"),
	}

	creds := &AWSCredentials{
		credProvider: mockProvider,
		region:       "us-west-2",
		logger:       logging.NewNopLogger(),
	}

	req, _ := http.NewRequest("GET", "https://s3.amazonaws.com/test-bucket/test-object", nil)

	err := creds.SignRequest(ctx, req)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestAWSCredentials_Token(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		provider  *mockCredentialsProvider
		wantToken string
		wantError bool
	}{
		{
			name: "Valid credentials",
			provider: &mockCredentialsProvider{
				creds: aws.Credentials{
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
				},
			},
			wantToken: "test-key",
			wantError: false,
		},
		{
			name: "Error retrieving credentials",
			provider: &mockCredentialsProvider{
				retrieveErr: errors.New("retrieve error"),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &AWSCredentials{
				credProvider: tt.provider,
				logger:       logging.NewNopLogger(),
			}

			token, err := creds.Token(ctx)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if token != tt.wantToken {
					t.Errorf("Expected token %s, got %s", tt.wantToken, token)
				}
			}
		})
	}
}

func TestAWSCredentials_Refresh(t *testing.T) {
	ctx := context.Background()

	mockProvider := &mockCredentialsProvider{
		creds: aws.Credentials{
			AccessKeyID:     "new-key",
			SecretAccessKey: "new-secret",
		},
	}

	creds := &AWSCredentials{
		credProvider: mockProvider,
		logger:       logging.NewNopLogger(),
		cachedCreds: &aws.Credentials{
			AccessKeyID: "old-key",
		},
		cacheExpiry: time.Now().Add(1 * time.Hour),
	}

	err := creds.Refresh(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// After refresh, getCredentials is called which sets new cache
	// So we should check that the cached creds are the new ones
	if creds.cachedCreds == nil {
		t.Error("Expected cached credentials to be set after refresh")
	} else if creds.cachedCreds.AccessKeyID != "new-key" {
		t.Errorf("Expected new credentials to be cached, got %s", creds.cachedCreds.AccessKeyID)
	}
}

func TestAWSCredentials_Refresh_Error(t *testing.T) {
	ctx := context.Background()

	mockProvider := &mockCredentialsProvider{
		retrieveErr: errors.New("refresh error"),
	}

	creds := &AWSCredentials{
		credProvider: mockProvider,
		logger:       logging.NewNopLogger(),
	}

	err := creds.Refresh(ctx)
	if err == nil {
		t.Error("Expected error but got none")
	}
}

func TestAWSCredentials_GetCredentialsProvider(t *testing.T) {
	mockProvider := &mockCredentialsProvider{}
	creds := &AWSCredentials{
		credProvider: mockProvider,
	}

	provider := creds.GetCredentialsProvider()
	if provider != mockProvider {
		t.Error("Expected to get the same credentials provider")
	}
}

func TestAWSCredentials_getCredentials_Caching(t *testing.T) {
	ctx := context.Background()
	callCount := 0

	// Create a provider that counts calls
	countingProvider := aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
		callCount++
		return aws.Credentials{
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
			Expires:         time.Now().Add(2 * time.Hour),
		}, nil
	})

	creds := &AWSCredentials{
		credProvider: countingProvider,
		logger:       logging.NewNopLogger(),
	}

	// First call should retrieve credentials
	_, err := creds.getCredentials(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call to provider, got %d", callCount)
	}

	// Second call should use cache
	_, err = creds.getCredentials(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 call to provider (cached), got %d", callCount)
	}
}

func TestAWSCredentials_getCredentials_NoExpiry(t *testing.T) {
	ctx := context.Background()

	provider := &mockCredentialsProvider{
		creds: aws.Credentials{
			AccessKeyID:     "test-key",
			SecretAccessKey: "test-secret",
			// No Expires field - should cache for 1 hour
		},
	}

	creds := &AWSCredentials{
		credProvider: provider,
		logger:       logging.NewNopLogger(),
	}

	_, err := creds.getCredentials(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that cache expiry was set to ~1 hour from now
	expectedExpiry := time.Now().Add(1 * time.Hour)
	if creds.cacheExpiry.Before(expectedExpiry.Add(-5*time.Minute)) ||
		creds.cacheExpiry.After(expectedExpiry.Add(5*time.Minute)) {
		t.Errorf("Expected cache expiry around %v, got %v", expectedExpiry, creds.cacheExpiry)
	}
}

func TestAccessKeyConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    AccessKeyConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: AccessKeyConfig{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			wantError: false,
		},
		{
			name: "Valid config with session token",
			config: AccessKeyConfig{
				AccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				SessionToken:    "FwoGZXIvYXdzEBYaDExampleSessionToken",
			},
			wantError: false,
		},
		{
			name: "Missing access key ID",
			config: AccessKeyConfig{
				SecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			},
			wantError: true,
		},
		{
			name: "Missing secret access key",
			config: AccessKeyConfig{
				AccessKeyID: "AKIAIOSFODNN7EXAMPLE",
			},
			wantError: true,
		},
		{
			name:      "Empty config",
			config:    AccessKeyConfig{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestAssumeRoleConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    AssumeRoleConfig
		wantError bool
	}{
		{
			name: "Valid minimal config",
			config: AssumeRoleConfig{
				RoleARN: "arn:aws:iam::123456789012:role/MyRole",
			},
			wantError: false,
		},
		{
			name: "Valid full config",
			config: AssumeRoleConfig{
				RoleARN:         "arn:aws:iam::123456789012:role/MyRole",
				RoleSessionName: "my-session",
				ExternalID:      "external-123",
				Duration:        30 * time.Minute,
			},
			wantError: false,
		},
		{
			name: "Missing role ARN",
			config: AssumeRoleConfig{
				RoleSessionName: "my-session",
			},
			wantError: true,
		},
		{
			name:      "Empty config",
			config:    AssumeRoleConfig{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestWebIdentityConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    WebIdentityConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: WebIdentityConfig{
				RoleARN:   "arn:aws:iam::123456789012:role/TestRole",
				TokenFile: "/tmp/token",
			},
			wantError: false,
		},
		{
			name: "Valid config with session name",
			config: WebIdentityConfig{
				RoleARN:         "arn:aws:iam::123456789012:role/TestRole",
				TokenFile:       "/tmp/token",
				RoleSessionName: "test-session",
			},
			wantError: false,
		},
		{
			name: "Missing role ARN",
			config: WebIdentityConfig{
				TokenFile: "/tmp/token",
			},
			wantError: true,
		},
		{
			name: "Missing token file",
			config: WebIdentityConfig{
				RoleARN: "arn:aws:iam::123456789012:role/TestRole",
			},
			wantError: true,
		},
		{
			name:      "Empty config",
			config:    WebIdentityConfig{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestCreateStaticCredentialsProvider(t *testing.T) {
	config := AccessKeyConfig{
		AccessKeyID:     "test-key",
		SecretAccessKey: "test-secret",
		SessionToken:    "test-token",
	}

	provider := createStaticCredentialsProvider(config)

	// Retrieve credentials
	ctx := context.Background()
	creds, err := provider.Retrieve(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if creds.AccessKeyID != config.AccessKeyID {
		t.Errorf("Expected access key ID %s, got %s", config.AccessKeyID, creds.AccessKeyID)
	}
	if creds.SecretAccessKey != config.SecretAccessKey {
		t.Errorf("Expected secret access key %s, got %s", config.SecretAccessKey, creds.SecretAccessKey)
	}
	if creds.SessionToken != config.SessionToken {
		t.Errorf("Expected session token %s, got %s", config.SessionToken, creds.SessionToken)
	}
}
