package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"go.uber.org/zap/zaptest"
	"golang.org/x/oauth2"
)

func TestGCPCredentials_Provider(t *testing.T) {
	creds := &GCPCredentials{
		authType:  auth.GCPServiceAccount,
		projectID: "test-project",
		logger:    logging.ForZap(zaptest.NewLogger(t)),
	}

	if provider := creds.Provider(); provider != auth.ProviderGCP {
		t.Errorf("Expected provider %s, got %s", auth.ProviderGCP, provider)
	}
}

func TestGCPCredentials_Type(t *testing.T) {
	tests := []struct {
		name     string
		authType auth.AuthType
	}{
		{
			name:     "Service Account",
			authType: auth.GCPServiceAccount,
		},
		{
			name:     "Workload Identity",
			authType: auth.GCPWorkloadIdentity,
		},
		{
			name:     "Default",
			authType: auth.GCPDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &GCPCredentials{
				authType: tt.authType,
			}
			if typ := creds.Type(); typ != tt.authType {
				t.Errorf("Expected type %s, got %s", tt.authType, typ)
			}
		})
	}
}

func TestGCPCredentials_GetProjectID(t *testing.T) {
	creds := &GCPCredentials{
		projectID: "my-gcp-project",
	}

	if projectID := creds.GetProjectID(); projectID != "my-gcp-project" {
		t.Errorf("Expected project ID my-gcp-project, got %s", projectID)
	}
}

func TestGCPCredentials_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		creds    *GCPCredentials
		expected bool
	}{
		{
			name:     "No cached token",
			creds:    &GCPCredentials{},
			expected: true,
		},
		{
			name: "With expired token",
			creds: &GCPCredentials{
				cachedToken: &oauth2.Token{
					AccessToken: "expired-token",
					Expiry:      time.Now().Add(-1 * time.Hour), // Expired
				},
			},
			expected: true,
		},
		{
			name: "With valid token",
			creds: &GCPCredentials{
				cachedToken: &oauth2.Token{
					AccessToken: "valid-token",
					Expiry:      time.Now().Add(1 * time.Hour),
				},
			},
			expected: false,
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

func TestServiceAccountConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    ServiceAccountConfig
		wantError bool
	}{
		{
			name: "Valid config",
			config: ServiceAccountConfig{
				Type:        "service_account",
				ProjectID:   "test-project",
				PrivateKey:  "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
				ClientEmail: "test@test-project.iam.gserviceaccount.com",
			},
			wantError: false,
		},
		{
			name: "Wrong type",
			config: ServiceAccountConfig{
				Type:        "wrong_type",
				ProjectID:   "test-project",
				PrivateKey:  "key",
				ClientEmail: "test@test.com",
			},
			wantError: true,
		},
		{
			name: "Missing project ID",
			config: ServiceAccountConfig{
				Type:        "service_account",
				PrivateKey:  "key",
				ClientEmail: "test@test.com",
			},
			wantError: true,
		},
		{
			name: "Missing private key",
			config: ServiceAccountConfig{
				Type:        "service_account",
				ProjectID:   "test-project",
				ClientEmail: "test@test.com",
			},
			wantError: true,
		},
		{
			name: "Missing client email",
			config: ServiceAccountConfig{
				Type:       "service_account",
				ProjectID:  "test-project",
				PrivateKey: "key",
			},
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

func TestServiceAccountConfig_ToJSON(t *testing.T) {
	config := ServiceAccountConfig{
		Type:                    "service_account",
		ProjectID:               "test-project",
		PrivateKeyID:            "key-id",
		PrivateKey:              "private-key",
		ClientEmail:             "test@test-project.iam.gserviceaccount.com",
		ClientID:                "client-id",
		AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
		TokenURI:                "https://oauth2.googleapis.com/token",
		AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
		ClientX509CertURL:       "https://www.googleapis.com/robot/v1/metadata/x509/test%40test-project.iam.gserviceaccount.com",
	}

	jsonData, err := config.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("Expected non-empty JSON data")
	}

	// Verify it's valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Errorf("Invalid JSON output: %v", err)
	}
}

func TestWorkloadIdentityConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    WorkloadIdentityConfig
		wantError bool
	}{
		{
			name: "Valid minimal config",
			config: WorkloadIdentityConfig{
				ProjectID: "test-project",
			},
			wantError: false,
		},
		{
			name: "Valid GKE workload identity config",
			config: WorkloadIdentityConfig{
				ProjectID:                "test-project",
				ServiceAccount:           "my-sa@test-project.iam.gserviceaccount.com",
				KubernetesServiceAccount: "default/my-ksa",
			},
			wantError: false,
		},
		{
			name: "Valid full GKE config",
			config: WorkloadIdentityConfig{
				ProjectID:                "test-project",
				ServiceAccount:           "my-sa@test-project.iam.gserviceaccount.com",
				KubernetesServiceAccount: "default/my-ksa",
				ClusterName:              "my-cluster",
				ClusterLocation:          "us-central1",
			},
			wantError: false,
		},
		{
			name: "Missing project ID",
			config: WorkloadIdentityConfig{
				ServiceAccount: "my-sa@test-project.iam.gserviceaccount.com",
			},
			wantError: true,
		},
		{
			name:      "Empty config",
			config:    WorkloadIdentityConfig{},
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

// mockTokenSource implements oauth2.TokenSource for testing
type mockTokenSource struct {
	token     *oauth2.Token
	err       error
	calls     int
	tokenFunc func() (*oauth2.Token, error) // Allow overriding Token method
}

func (m *mockTokenSource) Token() (*oauth2.Token, error) {
	m.calls++
	if m.tokenFunc != nil {
		return m.tokenFunc()
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.token, nil
}

func TestGCPCredentials_Token(t *testing.T) {
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
				Expiry:      time.Now().Add(1 * time.Hour),
			},
			wantToken: "test-access-token",
			wantErr:   false,
		},
		{
			name:      "Token error",
			tokenErr:  errors.New("token retrieval failed"),
			wantToken: "",
			wantErr:   true,
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
				projectID:   "test-project",
				logger:      logging.ForZap(zaptest.NewLogger(t)),
			}

			token, err := creds.Token(context.Background())
			if (err != nil) != tt.wantErr {
				t.Errorf("Token() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if token != tt.wantToken {
				t.Errorf("Token() = %v, want %v", token, tt.wantToken)
			}

			// Verify token was cached
			if err == nil {
				creds.mu.RLock()
				if creds.cachedToken != tt.token {
					t.Error("Token was not cached")
				}
				creds.mu.RUnlock()
			}
		})
	}
}

func TestGCPCredentials_SignRequest(t *testing.T) {
	tests := []struct {
		name     string
		token    *oauth2.Token
		tokenErr error
		wantErr  bool
	}{
		{
			name: "Valid token",
			token: &oauth2.Token{
				AccessToken: "test-access-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			},
			wantErr: false,
		},
		{
			name:     "Token error",
			tokenErr: errors.New("token retrieval failed"),
			wantErr:  true,
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
				projectID:   "test-project",
				logger:      logging.ForZap(zaptest.NewLogger(t)),
			}

			req, _ := http.NewRequest("GET", "https://storage.googleapis.com/test", nil)
			err := creds.SignRequest(context.Background(), req)

			if (err != nil) != tt.wantErr {
				t.Errorf("SignRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				authHeader := req.Header.Get("Authorization")
				expectedHeader := "Bearer " + tt.token.AccessToken
				if authHeader != expectedHeader {
					t.Errorf("Authorization header = %v, want %v", authHeader, expectedHeader)
				}
			}
		})
	}
}

func TestGCPCredentials_Refresh(t *testing.T) {
	mockTS := &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(1 * time.Hour),
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		projectID:   "test-project",
		logger:      logging.ForZap(zaptest.NewLogger(t)),
	}

	// Cache a token first
	_, _ = creds.Token(context.Background())

	// Reset mock calls counter
	mockTS.calls = 0

	// Refresh should force a new token fetch
	err := creds.Refresh(context.Background())
	if err != nil {
		t.Errorf("Refresh() error = %v", err)
	}

	if mockTS.calls != 1 {
		t.Errorf("Expected 1 token fetch after refresh, got %d", mockTS.calls)
	}
}

func TestGCPCredentials_Refresh_Error(t *testing.T) {
	mockTS := &mockTokenSource{
		err: errors.New("refresh failed"),
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		projectID:   "test-project",
		logger:      logging.ForZap(zaptest.NewLogger(t)),
	}

	err := creds.Refresh(context.Background())
	if err == nil {
		t.Error("Expected error from refresh")
	}
}

func TestGCPCredentials_IsExpired_ThreadSafe(t *testing.T) {
	creds := &GCPCredentials{
		tokenSource: &mockTokenSource{
			token: &oauth2.Token{
				AccessToken: "test",
				Expiry:      time.Now().Add(1 * time.Hour),
			},
		},
		authType:  auth.GCPServiceAccount,
		projectID: "test-project",
		logger:    logging.ForZap(zaptest.NewLogger(t)),
	}

	// Test concurrent access
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = creds.IsExpired()
			_, _ = creds.Token(context.Background())
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestGCPCredentials_GetTokenSource(t *testing.T) {
	mockTS := &mockTokenSource{}
	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		projectID:   "test-project",
		logger:      logging.ForZap(zaptest.NewLogger(t)),
	}

	ts := creds.GetTokenSource()
	if ts != mockTS {
		t.Error("GetTokenSource() did not return the expected token source")
	}
}

func TestGetClientOption(t *testing.T) {
	mockTS := &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: "test-access-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(1 * time.Hour),
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		projectID:   "test-project",
		logger:      logging.ForZap(zaptest.NewLogger(t)),
	}

	opt := GetClientOption(creds)
	if opt == nil {
		t.Error("GetClientOption() returned nil")
	}
}

func TestGCPCredentials_NilTokenSource(t *testing.T) {
	creds := &GCPCredentials{
		authType:  auth.GCPServiceAccount,
		projectID: "test-project",
		logger:    logging.ForZap(zaptest.NewLogger(t)),
		// tokenSource is nil
	}

	// Test Token() with nil tokenSource
	_, err := creds.Token(context.Background())
	if err == nil || err.Error() != "token source is not initialized" {
		t.Errorf("Expected 'token source is not initialized' error, got %v", err)
	}

	// Test SignRequest() with nil tokenSource
	req, _ := http.NewRequest("GET", "https://example.com", nil)
	err = creds.SignRequest(context.Background(), req)
	if err == nil || err.Error() != "token source is not initialized" {
		t.Errorf("Expected 'token source is not initialized' error, got %v", err)
	}

	// Test Refresh() with nil tokenSource
	err = creds.Refresh(context.Background())
	if err == nil || err.Error() != "token source is not initialized" {
		t.Errorf("Expected 'token source is not initialized' error, got %v", err)
	}
}

func TestGCPCredentials_TokenCaching(t *testing.T) {
	callCount := 0
	testToken := &oauth2.Token{
		AccessToken: "test-token",
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(1 * time.Hour),
	}

	mockTS := &mockTokenSource{
		tokenFunc: func() (*oauth2.Token, error) {
			callCount++
			return testToken, nil
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		projectID:   "test-project",
		logger:      logging.ForZap(zaptest.NewLogger(t)),
	}

	// First call should fetch token
	token1, err := creds.Token(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Expected 1 token fetch, got %d", callCount)
	}

	// Check that token was cached
	creds.mu.RLock()
	cachedToken := creds.cachedToken
	creds.mu.RUnlock()

	if cachedToken == nil {
		t.Error("Token was not cached")
	}
	if cachedToken.AccessToken != token1 {
		t.Error("Cached token doesn't match returned token")
	}
}

func TestGCPCredentials_SignRequest_SetsAuthHeader(t *testing.T) {
	mockTS := &mockTokenSource{
		token: &oauth2.Token{
			AccessToken: "test-bearer-token",
			TokenType:   "Bearer",
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		projectID:   "test-project",
		logger:      logging.ForZap(zaptest.NewLogger(t)),
	}

	req, _ := http.NewRequest("GET", "https://example.com/api", nil)

	err := creds.SignRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	authHeader := req.Header.Get("Authorization")
	expectedHeader := "Bearer test-bearer-token"
	if authHeader != expectedHeader {
		t.Errorf("Expected auth header %q, got %q", expectedHeader, authHeader)
	}
}

func TestGCPCredentials_ConcurrentTokenAccess(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	mockTS := &mockTokenSource{
		tokenFunc: func() (*oauth2.Token, error) {
			mu.Lock()
			callCount++
			mu.Unlock()

			// Simulate some processing time
			time.Sleep(10 * time.Millisecond)

			return &oauth2.Token{
				AccessToken: "concurrent-token",
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		projectID:   "test-project",
		logger:      logging.ForZap(zaptest.NewLogger(t)),
	}

	// Launch multiple goroutines to access token concurrently
	numGoroutines := 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			token, err := creds.Token(context.Background())
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if token != "concurrent-token" {
				t.Errorf("Expected token 'concurrent-token', got %s", token)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// All goroutines should have gotten the token
	mu.Lock()
	if callCount != numGoroutines {
		t.Errorf("Expected %d token fetches, got %d", numGoroutines, callCount)
	}
	mu.Unlock()
}

func TestServiceAccountConfig_ToJSON_Error(t *testing.T) {
	// Test a config that would cause JSON marshaling to fail
	// Since ServiceAccountConfig only has string fields, it's hard to make it fail
	// But we can test the error path exists
	config := &ServiceAccountConfig{
		Type:        "service_account",
		ProjectID:   "test-project",
		PrivateKey:  "test-key",
		ClientEmail: "test@test.com",
	}

	jsonData, err := config.ToJSON()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify the JSON is valid
	var parsed ServiceAccountConfig
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Errorf("Failed to unmarshal JSON: %v", err)
	}
}

func TestGCPCredentials_RefreshUpdatesCache(t *testing.T) {
	refreshCount := 0
	mockTS := &mockTokenSource{
		tokenFunc: func() (*oauth2.Token, error) {
			refreshCount++
			return &oauth2.Token{
				AccessToken: fmt.Sprintf("refreshed-token-%d", refreshCount),
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		projectID:   "test-project",
		logger:      logging.ForZap(zaptest.NewLogger(t)),
	}

	// Get initial token
	token1, _ := creds.Token(context.Background())
	if token1 != "refreshed-token-1" {
		t.Errorf("Expected 'refreshed-token-1', got %s", token1)
	}

	// Refresh should get a new token
	err := creds.Refresh(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check cached token was updated
	creds.mu.RLock()
	cachedToken := creds.cachedToken
	creds.mu.RUnlock()

	if cachedToken.AccessToken != "refreshed-token-2" {
		t.Errorf("Expected cached token to be 'refreshed-token-2', got %s", cachedToken.AccessToken)
	}
}

func TestGCPCredentials_SignRequestUpdatesCache(t *testing.T) {
	signCount := 0
	mockTS := &mockTokenSource{
		tokenFunc: func() (*oauth2.Token, error) {
			signCount++
			return &oauth2.Token{
				AccessToken: fmt.Sprintf("sign-token-%d", signCount),
				TokenType:   "Bearer",
				Expiry:      time.Now().Add(1 * time.Hour),
			}, nil
		},
	}

	creds := &GCPCredentials{
		tokenSource: mockTS,
		authType:    auth.GCPServiceAccount,
		projectID:   "test-project",
		logger:      logging.ForZap(zaptest.NewLogger(t)),
	}

	req, _ := http.NewRequest("GET", "https://example.com", nil)

	// SignRequest should cache the token
	err := creds.SignRequest(context.Background(), req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check that token was cached
	creds.mu.RLock()
	cachedToken := creds.cachedToken
	creds.mu.RUnlock()

	if cachedToken == nil {
		t.Error("Token was not cached by SignRequest")
	}
	if cachedToken.AccessToken != "sign-token-1" {
		t.Errorf("Expected cached token to be 'sign-token-1', got %s", cachedToken.AccessToken)
	}
}
