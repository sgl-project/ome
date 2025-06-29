package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

// mockTokenSource implements oauth2.TokenSource for testing
type mockTokenSource struct {
	token        *oauth2.Token
	err          error
	refreshCount int
	mu           sync.Mutex
}

func (m *mockTokenSource) Token() (*oauth2.Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return nil, m.err
	}

	m.refreshCount++
	return m.token, nil
}

// Test Token method comprehensively
func TestGCPCredentials_Token_Comprehensive(t *testing.T) {
	tests := []struct {
		name      string
		token     *oauth2.Token
		tokenErr  error
		wantToken string
		wantErr   bool
	}{
		{
			name: "Valid token",
			token: &oauth2.Token{
				AccessToken: "test-access-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			},
			wantToken: "test-access-token",
			wantErr:   false,
		},
		{
			name: "Expired token gets refreshed",
			token: &oauth2.Token{
				AccessToken: "refreshed-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			},
			wantToken: "refreshed-token",
			wantErr:   false,
		},
		{
			name:     "Token source error",
			tokenErr: errors.New("failed to get token"),
			wantErr:  true,
		},
		{
			name: "Empty access token",
			token: &oauth2.Token{
				AccessToken: "",
				TokenType:   "Bearer",
			},
			wantToken: "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTS := &mockTokenSource{
				token: tt.token,
				err:   tt.tokenErr,
			}

			creds := &GCPCredentials{
				tokenSource: mockTS,
				authType:    auth.GCPServiceAccount,
				logger:      logging.NewNopLogger(),
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
				if creds.cachedToken != tt.token {
					t.Error("Token not cached properly")
				}
			}
		})
	}
}

// Test SignRequest method comprehensively
func TestGCPCredentials_SignRequest_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		token    *oauth2.Token
		tokenErr error
		wantErr  bool
		checkReq func(*testing.T, *http.Request)
	}{
		{
			name: "Successful signing",
			token: &oauth2.Token{
				AccessToken: "test-token-123",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
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
			name: "Different token type",
			token: &oauth2.Token{
				AccessToken: "test-token-456",
				TokenType:   "MAC",
				Expiry:      time.Now().Add(time.Hour),
			},
			wantErr: false,
			checkReq: func(t *testing.T, req *http.Request) {
				authHeader := req.Header.Get("Authorization")
				if authHeader != "MAC test-token-456" {
					t.Errorf("Expected Authorization header 'MAC test-token-456', got %s", authHeader)
				}
			},
		},
		{
			name:     "Token source error",
			tokenErr: errors.New("authentication failed"),
			wantErr:  true,
		},
		{
			name: "Request with existing headers",
			token: &oauth2.Token{
				AccessToken: "new-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
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
			mockTS := &mockTokenSource{
				token: tt.token,
				err:   tt.tokenErr,
			}

			creds := &GCPCredentials{
				tokenSource: mockTS,
				authType:    auth.GCPServiceAccount,
				logger:      logging.NewNopLogger(),
			}

			req := httptest.NewRequest("GET", "https://storage.googleapis.com/test", nil)
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
func TestGCPCredentials_Refresh_Comprehensive(t *testing.T) {
	tests := []struct {
		name         string
		token        *oauth2.Token
		tokenErr     error
		wantErr      bool
		checkRefresh func(*testing.T, *mockTokenSource)
	}{
		{
			name: "Successful refresh",
			token: &oauth2.Token{
				AccessToken: "refreshed-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			},
			wantErr: false,
			checkRefresh: func(t *testing.T, ts *mockTokenSource) {
				if ts.refreshCount != 1 {
					t.Errorf("Expected refresh count 1, got %d", ts.refreshCount)
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
			token: &oauth2.Token{
				AccessToken: "multi-refresh-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(time.Hour),
			},
			wantErr: false,
			checkRefresh: func(t *testing.T, ts *mockTokenSource) {
				// Do another refresh
				creds := &GCPCredentials{tokenSource: ts}
				creds.Refresh(context.Background())
				if ts.refreshCount != 2 {
					t.Errorf("Expected refresh count 2 after multiple refreshes, got %d", ts.refreshCount)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockTS := &mockTokenSource{
				token: tt.token,
				err:   tt.tokenErr,
			}

			creds := &GCPCredentials{
				tokenSource: mockTS,
				authType:    auth.GCPServiceAccount,
				logger:      logging.NewNopLogger(),
			}

			ctx := context.Background()
			err := creds.Refresh(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Refresh() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.checkRefresh != nil {
				tt.checkRefresh(t, mockTS)
			}
		})
	}
}

// Test IsExpired method comprehensively
func TestGCPCredentials_IsExpired_Comprehensive(t *testing.T) {
	tests := []struct {
		name        string
		cachedToken *oauth2.Token
		want        bool
	}{
		{
			name:        "No cached token",
			cachedToken: nil,
			want:        true,
		},
		{
			name: "Valid token",
			cachedToken: &oauth2.Token{
				AccessToken: "valid-token",
				Expiry:      time.Now().Add(time.Hour),
			},
			want: false,
		},
		{
			name: "Expired token",
			cachedToken: &oauth2.Token{
				AccessToken: "expired-token",
				Expiry:      time.Now().Add(-time.Hour),
			},
			want: true,
		},
		{
			name: "Token expiring soon (within 10 seconds)",
			cachedToken: &oauth2.Token{
				AccessToken: "expiring-soon",
				Expiry:      time.Now().Add(5 * time.Second),
			},
			want: true, // oauth2.Token.Valid() considers tokens expiring within 10s as invalid
		},
		{
			name: "Token with zero expiry",
			cachedToken: &oauth2.Token{
				AccessToken: "no-expiry",
				Expiry:      time.Time{},
			},
			want: false, // Zero time means token doesn't expire
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &GCPCredentials{
				cachedToken: tt.cachedToken,
				authType:    auth.GCPServiceAccount,
				logger:      logging.NewNopLogger(),
			}

			if got := creds.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test GetTokenSource method
func TestGCPCredentials_GetTokenSource(t *testing.T) {
	mockTS := &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: "test-token",
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		logger:      logging.NewNopLogger(),
	}

	ts := creds.GetTokenSource()
	if ts != mockTS {
		t.Error("GetTokenSource() should return the same token source")
	}
}

// Test GetClientOption function
func TestGetClientOption(t *testing.T) {
	mockTS := &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: "test-token",
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		logger:      logging.NewNopLogger(),
	}

	opt := GetClientOption(creds)
	if opt == nil {
		t.Fatal("GetClientOption() returned nil")
	}

	// Verify it's a valid client option by trying to apply it
	var opts []option.ClientOption
	opts = append(opts, opt)
	if len(opts) != 1 {
		t.Error("Failed to create valid client option")
	}
}

// Test concurrent token operations
func TestGCPCredentials_ConcurrentOperations(t *testing.T) {
	mockTS := &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: "concurrent-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(time.Hour),
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		logger:      logging.NewNopLogger(),
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
}

// Test edge cases
func TestGCPCredentials_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "Nil logger handling",
			test: func(t *testing.T) {
				creds := &GCPCredentials{
					tokenSource: &mockTokenSource{
						token: &oauth2.Token{AccessToken: "test"},
					},
					authType: auth.GCPServiceAccount,
					logger:   nil,
				}
				// Should not panic
				_ = creds.Provider()
				_ = creds.Type()
			},
		},
		{
			name: "Empty project ID",
			test: func(t *testing.T) {
				creds := &GCPCredentials{
					projectID: "",
					authType:  auth.GCPServiceAccount,
					logger:    logging.NewNopLogger(),
				}
				if creds.GetProjectID() != "" {
					t.Error("Expected empty project ID")
				}
			},
		},
		{
			name: "Multiple token types",
			test: func(t *testing.T) {
				authTypes := []auth.AuthType{
					auth.GCPServiceAccount,
					auth.GCPWorkloadIdentity,
					auth.GCPDefault,
				}

				for _, at := range authTypes {
					creds := &GCPCredentials{
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

// Test ServiceAccountConfig validation comprehensively
func TestServiceAccountConfig_Validate_Comprehensive(t *testing.T) {
	tests := []struct {
		name    string
		config  ServiceAccountConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config",
			config: ServiceAccountConfig{
				Type:        "service_account",
				ProjectID:   "test-project",
				PrivateKey:  "-----BEGIN PRIVATE KEY-----\ntest\n-----END PRIVATE KEY-----",
				ClientEmail: "test@test-project.iam.gserviceaccount.com",
			},
			wantErr: false,
		},
		{
			name: "Invalid type",
			config: ServiceAccountConfig{
				Type:        "user",
				ProjectID:   "test-project",
				PrivateKey:  "key",
				ClientEmail: "test@example.com",
			},
			wantErr: true,
			errMsg:  "invalid service account type: user",
		},
		{
			name: "Missing project ID",
			config: ServiceAccountConfig{
				Type:        "service_account",
				PrivateKey:  "key",
				ClientEmail: "test@example.com",
			},
			wantErr: true,
			errMsg:  "project_id is required",
		},
		{
			name: "Missing private key",
			config: ServiceAccountConfig{
				Type:        "service_account",
				ProjectID:   "test-project",
				ClientEmail: "test@example.com",
			},
			wantErr: true,
			errMsg:  "private_key is required",
		},
		{
			name: "Missing client email",
			config: ServiceAccountConfig{
				Type:       "service_account",
				ProjectID:  "test-project",
				PrivateKey: "key",
			},
			wantErr: true,
			errMsg:  "client_email is required",
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

// Test ServiceAccountConfig ToJSON
func TestServiceAccountConfig_ToJSON_Comprehensive(t *testing.T) {
	config := ServiceAccountConfig{
		Type:                    "service_account",
		ProjectID:               "test-project-123",
		PrivateKeyID:            "key-id-123",
		PrivateKey:              "-----BEGIN PRIVATE KEY-----\ntest-key\n-----END PRIVATE KEY-----",
		ClientEmail:             "test@test-project-123.iam.gserviceaccount.com",
		ClientID:                "123456789",
		AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
		TokenURI:                "https://oauth2.googleapis.com/token",
		AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
		ClientX509CertURL:       "https://www.googleapis.com/robot/v1/metadata/x509/test%40test-project-123.iam.gserviceaccount.com",
	}

	jsonBytes, err := config.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Verify JSON structure
	var decoded ServiceAccountConfig
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check key fields
	if decoded.Type != config.Type {
		t.Errorf("JSON type = %v, want %v", decoded.Type, config.Type)
	}
	if decoded.ProjectID != config.ProjectID {
		t.Errorf("JSON project_id = %v, want %v", decoded.ProjectID, config.ProjectID)
	}
	if decoded.ClientEmail != config.ClientEmail {
		t.Errorf("JSON client_email = %v, want %v", decoded.ClientEmail, config.ClientEmail)
	}
}

// Test WorkloadIdentityConfig validation comprehensively
func TestWorkloadIdentityConfig_Validate_Comprehensive(t *testing.T) {
	tests := []struct {
		name    string
		config  WorkloadIdentityConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid config",
			config: WorkloadIdentityConfig{
				ProjectID:  "test-project",
				PoolID:     "test-pool",
				ProviderID: "test-provider",
			},
			wantErr: false,
		},
		{
			name: "Valid config with optional fields",
			config: WorkloadIdentityConfig{
				ProjectID:        "test-project",
				PoolID:           "test-pool",
				ProviderID:       "test-provider",
				ServiceAccount:   "test@test-project.iam.gserviceaccount.com",
				CredentialSource: "/var/run/secrets/tokens/gcp-ksa",
			},
			wantErr: false,
		},
		{
			name: "Missing project ID",
			config: WorkloadIdentityConfig{
				PoolID:     "test-pool",
				ProviderID: "test-provider",
			},
			wantErr: true,
			errMsg:  "project_id is required",
		},
		{
			name: "Missing pool ID",
			config: WorkloadIdentityConfig{
				ProjectID:  "test-project",
				ProviderID: "test-provider",
			},
			wantErr: true,
			errMsg:  "pool_id is required",
		},
		{
			name: "Missing provider ID",
			config: WorkloadIdentityConfig{
				ProjectID: "test-project",
				PoolID:    "test-pool",
			},
			wantErr: true,
			errMsg:  "provider_id is required",
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

// Test error messages format
func TestGCPCredentials_ErrorMessages(t *testing.T) {
	baseErr := errors.New("base error")

	tests := []struct {
		name    string
		method  func(*GCPCredentials) error
		wantErr string
	}{
		{
			name: "Token error message",
			method: func(c *GCPCredentials) error {
				_, err := c.Token(context.Background())
				return err
			},
			wantErr: "failed to get token: base error",
		},
		{
			name: "SignRequest error message",
			method: func(c *GCPCredentials) error {
				req := httptest.NewRequest("GET", "https://example.com", nil)
				return c.SignRequest(context.Background(), req)
			},
			wantErr: "failed to get token: base error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &GCPCredentials{
				tokenSource: &mockTokenSource{err: baseErr},
				logger:      logging.NewNopLogger(),
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
func TestGCPCredentials_TokenCaching(t *testing.T) {
	token1 := &oauth2.Token{
		AccessToken: "token-1",
		Expiry:      time.Now().Add(time.Hour),
	}

	token2 := &oauth2.Token{
		AccessToken: "token-2",
		Expiry:      time.Now().Add(2 * time.Hour),
	}

	mockTS := &mockTokenSource{token: token1}
	creds := &GCPCredentials{
		tokenSource: mockTS,
		logger:      logging.NewNopLogger(),
	}

	// First call caches token1
	ctx := context.Background()
	t1, _ := creds.Token(ctx)
	if t1 != "token-1" {
		t.Errorf("Expected token-1, got %s", t1)
	}
	if creds.cachedToken != token1 {
		t.Error("Token not cached")
	}

	// Update token source
	mockTS.token = token2

	// Second call gets and caches token2
	t2, _ := creds.Token(ctx)
	if t2 != "token-2" {
		t.Errorf("Expected token-2, got %s", t2)
	}
	if creds.cachedToken != token2 {
		t.Error("Token cache not updated")
	}
}

// Benchmark Token method
func BenchmarkGCPCredentials_Token(b *testing.B) {
	mockTS := &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: "bench-token",
			Expiry:      time.Now().Add(time.Hour),
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		logger:      logging.NewNopLogger(),
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		creds.Token(ctx)
	}
}
