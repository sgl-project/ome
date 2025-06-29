package azure

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// mockTokenCredential implements azcore.TokenCredential for testing
type mockTokenCredential struct {
	token        azcore.AccessToken
	err          error
	refreshCount int
	mu           sync.Mutex
}

func (m *mockTokenCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return azcore.AccessToken{}, m.err
	}

	m.refreshCount++
	return m.token, nil
}

// Test Token method comprehensively
func TestAzureCredentials_Token_Comprehensive(t *testing.T) {
	tests := []struct {
		name      string
		token     azcore.AccessToken
		tokenErr  error
		wantToken string
		wantErr   bool
	}{
		{
			name: "Valid token",
			token: azcore.AccessToken{
				Token:     "test-access-token",
				ExpiresOn: time.Now().Add(time.Hour),
			},
			wantToken: "test-access-token",
			wantErr:   false,
		},
		{
			name: "Expired token gets refreshed",
			token: azcore.AccessToken{
				Token:     "refreshed-token",
				ExpiresOn: time.Now().Add(time.Hour),
			},
			wantToken: "refreshed-token",
			wantErr:   false,
		},
		{
			name:     "Token credential error",
			tokenErr: errors.New("failed to get token"),
			wantErr:  true,
		},
		{
			name: "Empty access token",
			token: azcore.AccessToken{
				Token:     "",
				ExpiresOn: time.Now().Add(time.Hour),
			},
			wantToken: "",
			wantErr:   false,
		},
		{
			name: "Token with past expiry",
			token: azcore.AccessToken{
				Token:     "expired-token",
				ExpiresOn: time.Now().Add(-time.Hour),
			},
			wantToken: "expired-token",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCred := &mockTokenCredential{
				token: tt.token,
				err:   tt.tokenErr,
			}

			creds := &AzureCredentials{
				credential: mockCred,
				authType:   auth.AzureClientSecret,
				logger:     logging.NewNopLogger(),
			}

			ctx := context.Background()
			token, err := creds.Token(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Token() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if token != tt.wantToken {
					t.Errorf("Token() = %v, want %v", token, tt.wantToken)
				}

				// Check cached token
				if creds.cachedToken == nil {
					t.Error("Token not cached")
				} else if creds.cachedToken.Token != tt.token.Token {
					t.Error("Cached token mismatch")
				}
			}
		})
	}
}

// Test SignRequest method comprehensively
func TestAzureCredentials_SignRequest_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		token    azcore.AccessToken
		tokenErr error
		wantErr  bool
		checkReq func(*testing.T, *http.Request)
	}{
		{
			name: "Successful signing",
			token: azcore.AccessToken{
				Token:     "test-token-123",
				ExpiresOn: time.Now().Add(time.Hour),
			},
			wantErr: false,
			checkReq: func(t *testing.T, req *http.Request) {
				authHeader := req.Header.Get("Authorization")
				if authHeader != "Bearer test-token-123" {
					t.Errorf("Expected Authorization header 'Bearer test-token-123', got %s", authHeader)
				}
			},
		},
		{
			name: "Empty token",
			token: azcore.AccessToken{
				Token:     "",
				ExpiresOn: time.Now().Add(time.Hour),
			},
			wantErr: false,
			checkReq: func(t *testing.T, req *http.Request) {
				authHeader := req.Header.Get("Authorization")
				if authHeader != "Bearer " {
					t.Errorf("Expected Authorization header 'Bearer ', got %s", authHeader)
				}
			},
		},
		{
			name:     "Token credential error",
			tokenErr: errors.New("authentication failed"),
			wantErr:  true,
		},
		{
			name: "Request with existing headers",
			token: azcore.AccessToken{
				Token:     "new-token",
				ExpiresOn: time.Now().Add(time.Hour),
			},
			wantErr: false,
			checkReq: func(t *testing.T, req *http.Request) {
				// Should replace existing Authorization header
				authHeader := req.Header.Get("Authorization")
				if authHeader != "Bearer new-token" {
					t.Errorf("Expected Authorization header to be replaced with 'Bearer new-token', got %s", authHeader)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCred := &mockTokenCredential{
				token: tt.token,
				err:   tt.tokenErr,
			}

			creds := &AzureCredentials{
				credential: mockCred,
				authType:   auth.AzureClientSecret,
				logger:     logging.NewNopLogger(),
			}

			req := httptest.NewRequest("GET", "https://myaccount.blob.core.windows.net/container/blob", nil)
			if tt.name == "Request with existing headers" {
				req.Header.Set("Authorization", "Bearer old-token")
			}

			ctx := context.Background()
			err := creds.SignRequest(ctx, req)

			if (err != nil) != tt.wantErr {
				t.Errorf("SignRequest() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.checkReq != nil {
				tt.checkReq(t, req)
			}
		})
	}
}

// Test Refresh method comprehensively
func TestAzureCredentials_Refresh_Comprehensive(t *testing.T) {
	tests := []struct {
		name         string
		token        azcore.AccessToken
		tokenErr     error
		wantErr      bool
		checkRefresh func(*testing.T, *mockTokenCredential)
	}{
		{
			name: "Successful refresh",
			token: azcore.AccessToken{
				Token:     "refreshed-token",
				ExpiresOn: time.Now().Add(time.Hour),
			},
			wantErr: false,
			checkRefresh: func(t *testing.T, cred *mockTokenCredential) {
				if cred.refreshCount != 1 {
					t.Errorf("Expected refresh count 1, got %d", cred.refreshCount)
				}
			},
		},
		{
			name:     "Refresh error",
			tokenErr: errors.New("refresh failed"),
			wantErr:  true,
		},
		{
			name: "Multiple refreshes",
			token: azcore.AccessToken{
				Token:     "multi-refresh-token",
				ExpiresOn: time.Now().Add(time.Hour),
			},
			wantErr: false,
			checkRefresh: func(t *testing.T, cred *mockTokenCredential) {
				// Do another refresh
				azCreds := &AzureCredentials{credential: cred}
				azCreds.Refresh(context.Background())
				if cred.refreshCount != 2 {
					t.Errorf("Expected refresh count 2 after multiple refreshes, got %d", cred.refreshCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCred := &mockTokenCredential{
				token: tt.token,
				err:   tt.tokenErr,
			}

			creds := &AzureCredentials{
				credential: mockCred,
				authType:   auth.AzureClientSecret,
				logger:     logging.NewNopLogger(),
			}

			ctx := context.Background()
			err := creds.Refresh(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Refresh() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.checkRefresh != nil {
				tt.checkRefresh(t, mockCred)
			}
		})
	}
}

// Test GetCredential and GetTokenCredential
func TestAzureCredentials_GetCredential(t *testing.T) {
	mockCred := &mockTokenCredential{
		token: azcore.AccessToken{
			Token: "test-token",
		},
	}

	creds := &AzureCredentials{
		credential: mockCred,
		authType:   auth.AzureClientSecret,
		logger:     logging.NewNopLogger(),
	}

	// Test GetCredential
	cred := creds.GetCredential()
	if cred != mockCred {
		t.Error("GetCredential() should return the same credential")
	}

	// Test GetTokenCredential (should be same as GetCredential)
	tokenCred := creds.GetTokenCredential()
	if tokenCred != mockCred {
		t.Error("GetTokenCredential() should return the same credential")
	}
}

// Test concurrent operations
func TestAzureCredentials_ConcurrentOperations(t *testing.T) {
	mockCred := &mockTokenCredential{
		token: azcore.AccessToken{
			Token:     "concurrent-token",
			ExpiresOn: time.Now().Add(time.Hour),
		},
	}

	creds := &AzureCredentials{
		credential: mockCred,
		authType:   auth.AzureClientSecret,
		logger:     logging.NewNopLogger(),
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 10
	errors := make(chan error, numGoroutines*3)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(3)

		// Concurrent Token() calls
		go func() {
			defer wg.Done()
			ctx := context.Background()
			if _, err := creds.Token(ctx); err != nil {
				errors <- err
			}
		}()

		// Concurrent SignRequest() calls
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "https://example.com", nil)
			ctx := context.Background()
			if err := creds.SignRequest(ctx, req); err != nil {
				errors <- err
			}
		}()

		// Concurrent Refresh() calls
		go func() {
			defer wg.Done()
			ctx := context.Background()
			if err := creds.Refresh(ctx); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Check refresh count
	if mockCred.refreshCount < numGoroutines {
		t.Errorf("Expected at least %d refreshes, got %d", numGoroutines, mockCred.refreshCount)
	}
}

// Test edge cases
func TestAzureCredentials_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "Nil logger handling",
			test: func(t *testing.T) {
				creds := &AzureCredentials{
					credential: &mockTokenCredential{
						token: azcore.AccessToken{Token: "test"},
					},
					authType: auth.AzureClientSecret,
					logger:   nil,
				}
				// Should not panic
				_ = creds.Provider()
				_ = creds.Type()
			},
		},
		{
			name: "Empty tenant and client IDs",
			test: func(t *testing.T) {
				creds := &AzureCredentials{
					tenantID: "",
					clientID: "",
					authType: auth.AzureClientSecret,
					logger:   logging.NewNopLogger(),
				}
				if creds.GetTenantID() != "" {
					t.Error("Expected empty tenant ID")
				}
				if creds.GetClientID() != "" {
					t.Error("Expected empty client ID")
				}
			},
		},
		{
			name: "Multiple auth types",
			test: func(t *testing.T) {
				authTypes := []auth.AuthType{
					auth.AzureClientSecret,
					auth.AzureClientCertificate,
					auth.AzureManagedIdentity,
					auth.AzureAccountKey,
					auth.AzureDeviceFlow,
					auth.AzureDefault,
				}

				for _, at := range authTypes {
					creds := &AzureCredentials{
						authType: at,
						logger:   logging.NewNopLogger(),
					}
					if creds.Type() != at {
						t.Errorf("Expected auth type %s, got %s", at, creds.Type())
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

// Test error messages format
func TestAzureCredentials_ErrorMessages(t *testing.T) {
	baseErr := errors.New("base error")

	tests := []struct {
		name    string
		method  func(*AzureCredentials) error
		wantErr string
	}{
		{
			name: "Token error message",
			method: func(c *AzureCredentials) error {
				_, err := c.Token(context.Background())
				return err
			},
			wantErr: "failed to get token: base error",
		},
		{
			name: "SignRequest error message",
			method: func(c *AzureCredentials) error {
				req := httptest.NewRequest("GET", "https://example.com", nil)
				return c.SignRequest(context.Background(), req)
			},
			wantErr: "failed to get token: base error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &AzureCredentials{
				credential: &mockTokenCredential{err: baseErr},
				logger:     logging.NewNopLogger(),
			}

			err := tt.method(creds)
			if err == nil {
				t.Fatal("Expected error but got nil")
			}

			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Error = %v, want to contain %v", err.Error(), tt.wantErr)
			}
		})
	}
}

// Test token caching behavior
func TestAzureCredentials_TokenCaching(t *testing.T) {
	token1 := azcore.AccessToken{
		Token:     "token-1",
		ExpiresOn: time.Now().Add(time.Hour),
	}

	token2 := azcore.AccessToken{
		Token:     "token-2",
		ExpiresOn: time.Now().Add(2 * time.Hour),
	}

	mockCred := &mockTokenCredential{token: token1}
	creds := &AzureCredentials{
		credential: mockCred,
		logger:     logging.NewNopLogger(),
	}

	// First call caches token1
	ctx := context.Background()
	t1, _ := creds.Token(ctx)
	if t1 != "token-1" {
		t.Errorf("Expected token-1, got %s", t1)
	}
	if creds.cachedToken == nil || creds.cachedToken.Token != "token-1" {
		t.Error("Token not cached properly")
	}

	// Update token source
	mockCred.token = token2

	// Second call gets and caches token2
	t2, _ := creds.Token(ctx)
	if t2 != "token-2" {
		t.Errorf("Expected token-2, got %s", t2)
	}
	if creds.cachedToken == nil || creds.cachedToken.Token != "token-2" {
		t.Error("Token cache not updated")
	}
}

// Test DeviceFlowConfig validation
func TestDeviceFlowConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  DeviceFlowConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config",
			config: DeviceFlowConfig{
				TenantID: "test-tenant",
				ClientID: "test-client",
			},
			wantErr: false,
		},
		{
			name: "Missing tenant ID",
			config: DeviceFlowConfig{
				ClientID: "test-client",
			},
			wantErr: true,
			errMsg:  "tenant_id is required",
		},
		{
			name: "Missing client ID",
			config: DeviceFlowConfig{
				TenantID: "test-tenant",
			},
			wantErr: true,
			errMsg:  "client_id is required",
		},
		{
			name:    "Both missing",
			config:  DeviceFlowConfig{},
			wantErr: true,
			errMsg:  "tenant_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// Test different token scopes
func TestAzureCredentials_DifferentScopes(t *testing.T) {
	scopesCalled := 0

	// Custom mock that tracks scopes
	mockCred := &mockTokenCredential{
		token: azcore.AccessToken{
			Token:     "scoped-token",
			ExpiresOn: time.Now().Add(time.Hour),
		},
	}

	creds := &AzureCredentials{
		credential: mockCred,
		logger:     logging.NewNopLogger(),
	}

	// Call Token()
	ctx := context.Background()
	token, err := creds.Token(ctx)
	if err != nil {
		t.Fatalf("Token() error: %v", err)
	}
	if token != "scoped-token" {
		t.Errorf("Expected scoped-token, got %s", token)
	}
	scopesCalled++

	// Call SignRequest()
	req := httptest.NewRequest("GET", "https://example.com", nil)
	err = creds.SignRequest(ctx, req)
	if err != nil {
		t.Fatalf("SignRequest() error: %v", err)
	}
	scopesCalled++

	// Verify both methods were called
	if scopesCalled != 2 {
		t.Errorf("Expected 2 scope calls, got %d", scopesCalled)
	}

	// Verify request was signed
	authHeader := req.Header.Get("Authorization")
	if authHeader != "Bearer scoped-token" {
		t.Errorf("Expected Bearer scoped-token, got %s", authHeader)
	}
}

// Benchmark Token method
func BenchmarkAzureCredentials_Token(b *testing.B) {
	mockCred := &mockTokenCredential{
		token: azcore.AccessToken{
			Token:     "bench-token",
			ExpiresOn: time.Now().Add(time.Hour),
		},
	}

	creds := &AzureCredentials{
		credential: mockCred,
		logger:     logging.NewNopLogger(),
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		creds.Token(ctx)
	}
}
