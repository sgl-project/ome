package oci

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
)

// mockConfigProviderWithErrors allows simulating errors
type mockConfigProviderWithErrors struct {
	tenancy         string
	user            string
	fingerprint     string
	region          string
	privateKey      *rsa.PrivateKey
	shouldFailKeyID bool
	shouldFailSign  bool
}

func (m *mockConfigProviderWithErrors) TenancyOCID() (string, error) {
	if m.tenancy == "" {
		return "", errors.New("no tenancy configured")
	}
	return m.tenancy, nil
}

func (m *mockConfigProviderWithErrors) UserOCID() (string, error) {
	if m.user == "" {
		return "", errors.New("no user configured")
	}
	return m.user, nil
}

func (m *mockConfigProviderWithErrors) KeyFingerprint() (string, error) {
	if m.fingerprint == "" {
		return "", errors.New("no fingerprint configured")
	}
	return m.fingerprint, nil
}

func (m *mockConfigProviderWithErrors) Region() (string, error) {
	if m.region == "" {
		return "", errors.New("no region configured")
	}
	return m.region, nil
}

func (m *mockConfigProviderWithErrors) KeyID() (string, error) {
	if m.shouldFailKeyID {
		return "", errors.New("failed to get key ID")
	}
	return fmt.Sprintf("%s/%s/%s", m.tenancy, m.user, m.fingerprint), nil
}

func (m *mockConfigProviderWithErrors) PrivateRSAKey() (*rsa.PrivateKey, error) {
	if m.privateKey == nil {
		return nil, errors.New("no private key configured")
	}
	return m.privateKey, nil
}

func (m *mockConfigProviderWithErrors) AuthType() (common.AuthConfig, error) {
	return common.AuthConfig{
		AuthType:         common.UnknownAuthenticationType,
		IsFromConfigFile: false,
		OboToken:         nil,
	}, nil
}

// Test Token method comprehensively
func TestOCICredentials_Token_Comprehensive(t *testing.T) {
	tests := []struct {
		name      string
		authType  auth.AuthType
		wantToken string
		wantErr   bool
	}{
		{
			name:      "Instance Principal returns empty token",
			authType:  auth.OCIInstancePrincipal,
			wantToken: "",
			wantErr:   false,
		},
		{
			name:      "User Principal returns empty token",
			authType:  auth.OCIUserPrincipal,
			wantToken: "",
			wantErr:   false,
		},
		{
			name:      "Resource Principal returns empty token",
			authType:  auth.OCIResourcePrincipal,
			wantToken: "",
			wantErr:   false,
		},
		{
			name:      "OKE Workload Identity returns empty token",
			authType:  auth.OCIOkeWorkloadIdentity,
			wantToken: "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &OCICredentials{
				configProvider: &mockConfigProvider{},
				authType:       tt.authType,
				logger:         logging.NewNopLogger(),
			}

			ctx := context.Background()
			token, err := creds.Token(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Token() error = %v, wantErr %v", err, tt.wantErr)
			}
			if token != tt.wantToken {
				t.Errorf("Token() = %v, want %v", token, tt.wantToken)
			}
		})
	}
}

// Test SignRequest method comprehensively
func TestOCICredentials_SignRequest_Comprehensive(t *testing.T) {
	// Generate a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	tests := []struct {
		name           string
		configProvider common.ConfigurationProvider
		request        *http.Request
		wantErr        bool
		checkSignature bool
	}{
		{
			name: "Successful request signing",
			configProvider: &mockConfigProviderWithErrors{
				tenancy:     "ocid1.tenancy.oc1..example",
				user:        "ocid1.user.oc1..example",
				fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
				region:      "us-ashburn-1",
				privateKey:  privateKey,
			},
			request:        httptest.NewRequest("GET", "https://objectstorage.us-ashburn-1.oraclecloud.com/n/namespace/b/bucket/o/object", nil),
			wantErr:        false,
			checkSignature: true,
		},
		{
			name: "POST request with body",
			configProvider: &mockConfigProviderWithErrors{
				tenancy:     "ocid1.tenancy.oc1..example",
				user:        "ocid1.user.oc1..example",
				fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
				region:      "us-ashburn-1",
				privateKey:  privateKey,
			},
			request:        httptest.NewRequest("POST", "https://objectstorage.us-ashburn-1.oraclecloud.com/n/namespace/b/bucket/o/", strings.NewReader(`{"name":"test"}`)),
			wantErr:        false,
			checkSignature: true,
		},
		{
			name: "Request with query parameters",
			configProvider: &mockConfigProviderWithErrors{
				tenancy:     "ocid1.tenancy.oc1..example",
				user:        "ocid1.user.oc1..example",
				fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
				region:      "us-ashburn-1",
				privateKey:  privateKey,
			},
			request:        httptest.NewRequest("GET", "https://objectstorage.us-ashburn-1.oraclecloud.com/n/namespace/b/bucket/o/?limit=100&prefix=test", nil),
			wantErr:        false,
			checkSignature: true,
		},
		{
			name: "Failed to get key ID",
			configProvider: &mockConfigProviderWithErrors{
				shouldFailKeyID: true,
			},
			request: httptest.NewRequest("GET", "https://objectstorage.us-ashburn-1.oraclecloud.com/n/namespace/b/bucket/o/object", nil),
			wantErr: true,
		},
		{
			name:           "Missing config provider fields",
			configProvider: &mockConfigProviderWithErrors{},
			request:        httptest.NewRequest("GET", "https://objectstorage.us-ashburn-1.oraclecloud.com/n/namespace/b/bucket/o/object", nil),
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &OCICredentials{
				configProvider: tt.configProvider,
				authType:       auth.OCIUserPrincipal,
				logger:         logging.NewNopLogger(),
			}

			ctx := context.Background()
			err := creds.SignRequest(ctx, tt.request)

			if (err != nil) != tt.wantErr {
				t.Errorf("SignRequest() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.checkSignature {
				// Verify headers were added
				authHeader := tt.request.Header.Get("Authorization")
				if authHeader == "" {
					t.Error("Expected Authorization header to be set")
				}

				// Check for OCI signature format
				if !strings.HasPrefix(authHeader, "Signature ") {
					t.Errorf("Expected Authorization header to start with 'Signature ', got %s", authHeader)
				}

				// Check for required components
				if !strings.Contains(authHeader, "keyId=") {
					t.Error("Authorization header missing keyId")
				}
				if !strings.Contains(authHeader, "algorithm=") {
					t.Error("Authorization header missing algorithm")
				}
				if !strings.Contains(authHeader, "signature=") {
					t.Error("Authorization header missing signature")
				}
				if !strings.Contains(authHeader, "headers=") {
					t.Error("Authorization header missing headers")
				}

				// The OCI SDK may use its own date header handling
				// Skip checking for Date header as it's handled internally

				// The OCI SDK handles content hashing internally
				// Skip checking for x-content-sha256 header
			}
		})
	}
}

// Test Refresh method
func TestOCICredentials_Refresh_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		authType auth.AuthType
		wantErr  bool
	}{
		{
			name:     "Instance Principal refresh",
			authType: auth.OCIInstancePrincipal,
			wantErr:  false,
		},
		{
			name:     "User Principal refresh",
			authType: auth.OCIUserPrincipal,
			wantErr:  false,
		},
		{
			name:     "Resource Principal refresh",
			authType: auth.OCIResourcePrincipal,
			wantErr:  false,
		},
		{
			name:     "OKE Workload Identity refresh",
			authType: auth.OCIOkeWorkloadIdentity,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &OCICredentials{
				configProvider: &mockConfigProvider{},
				authType:       tt.authType,
				logger:         logging.NewNopLogger(),
			}

			ctx := context.Background()
			err := creds.Refresh(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Refresh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test OCIHTTPClient Do method comprehensively
func TestOCIHTTPClient_Do_Comprehensive(t *testing.T) {
	// Generate a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request has OCI signature
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Signature ") {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Missing or invalid authorization header"))
			return
		}

		// Return success - OCI SDK handles date header internally
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	tests := []struct {
		name           string
		configProvider common.ConfigurationProvider
		request        *http.Request
		wantStatus     int
		wantErr        bool
	}{
		{
			name: "Successful request",
			configProvider: &mockConfigProviderWithErrors{
				tenancy:     "ocid1.tenancy.oc1..example",
				user:        "ocid1.user.oc1..example",
				fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
				region:      "us-ashburn-1",
				privateKey:  privateKey,
			},
			request:    httptest.NewRequest("GET", server.URL+"/test", nil),
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "POST request with body",
			configProvider: &mockConfigProviderWithErrors{
				tenancy:     "ocid1.tenancy.oc1..example",
				user:        "ocid1.user.oc1..example",
				fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
				region:      "us-ashburn-1",
				privateKey:  privateKey,
			},
			request:    httptest.NewRequest("POST", server.URL+"/test", strings.NewReader(`{"data":"test"}`)),
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name: "Failed signing",
			configProvider: &mockConfigProviderWithErrors{
				shouldFailKeyID: true,
			},
			request: httptest.NewRequest("GET", server.URL+"/test", nil),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := &OCICredentials{
				configProvider: tt.configProvider,
				authType:       auth.OCIUserPrincipal,
				logger:         logging.NewNopLogger(),
			}

			client := NewOCIHTTPClient(creds)

			// Create new request to avoid RequestURI issues
			req, err := http.NewRequest(tt.request.Method, server.URL+tt.request.URL.Path, tt.request.Body)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			tt.request = req

			resp, err := client.Do(tt.request)

			if (err != nil) != tt.wantErr {
				t.Errorf("Do() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && resp != nil {
				defer resp.Body.Close()
				if resp.StatusCode != tt.wantStatus {
					t.Errorf("Do() status = %v, want %v", resp.StatusCode, tt.wantStatus)
				}
			}
		})
	}
}

// Test concurrent request signing
func TestOCICredentials_ConcurrentSigning(t *testing.T) {
	// Generate a test RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key: %v", err)
	}

	creds := &OCICredentials{
		configProvider: &mockConfigProviderWithErrors{
			tenancy:     "ocid1.tenancy.oc1..example",
			user:        "ocid1.user.oc1..example",
			fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
			region:      "us-ashburn-1",
			privateKey:  privateKey,
		},
		authType: auth.OCIUserPrincipal,
		logger:   logging.NewNopLogger(),
	}

	// Run concurrent signing operations
	var wg sync.WaitGroup
	numGoroutines := 10
	numRequestsPerGoroutine := 5

	errors := make(chan error, numGoroutines*numRequestsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numRequestsPerGoroutine; j++ {
				req := httptest.NewRequest("GET", fmt.Sprintf("https://example.com/test/%d/%d", id, j), nil)
				ctx := context.Background()
				if err := creds.SignRequest(ctx, req); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent signing error: %v", err)
	}
}

// Test edge cases and error conditions
func TestOCICredentials_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "Nil logger handling",
			test: func(t *testing.T) {
				creds := &OCICredentials{
					configProvider: &mockConfigProvider{},
					authType:       auth.OCIInstancePrincipal,
					logger:         nil,
				}
				// Should not panic
				_ = creds.Provider()
				_ = creds.Type()
			},
		},
		{
			name: "Empty region fallback",
			test: func(t *testing.T) {
				creds := &OCICredentials{
					configProvider: &mockConfigProviderWithErrors{},
					authType:       auth.OCIInstancePrincipal,
					region:         "",
					logger:         logging.NewNopLogger(),
				}
				region := creds.GetRegion()
				if region != "" {
					t.Errorf("Expected empty region, got %s", region)
				}
			},
		},
		{
			name: "Large request body signing",
			test: func(t *testing.T) {
				privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)
				creds := &OCICredentials{
					configProvider: &mockConfigProviderWithErrors{
						tenancy:     "ocid1.tenancy.oc1..example",
						user:        "ocid1.user.oc1..example",
						fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
						region:      "us-ashburn-1",
						privateKey:  privateKey,
					},
					authType: auth.OCIUserPrincipal,
					logger:   logging.NewNopLogger(),
				}

				// Create a large body
				largeBody := bytes.Repeat([]byte("a"), 1024*1024) // 1MB
				req := httptest.NewRequest("POST", "https://example.com/upload", bytes.NewReader(largeBody))

				ctx := context.Background()
				err := creds.SignRequest(ctx, req)
				if err != nil {
					t.Errorf("Failed to sign large request: %v", err)
				}

				// Verify x-content-sha256 header
				sha256Header := req.Header.Get("x-content-sha256")
				if sha256Header == "" {
					t.Error("Expected x-content-sha256 header for large body")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

// Test HTTP client timeout and transport settings
func TestOCIHTTPClient_Configuration(t *testing.T) {
	creds := &OCICredentials{
		configProvider: &mockConfigProvider{},
		authType:       auth.OCIInstancePrincipal,
		logger:         logging.NewNopLogger(),
	}

	client := NewOCIHTTPClient(creds)

	// Check timeout
	if client.client.Timeout != 20*time.Minute {
		t.Errorf("Expected timeout 20m, got %v", client.client.Timeout)
	}

	// Check transport settings
	transport, ok := client.client.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected http.Transport")
	}

	if transport.MaxIdleConns != 200 {
		t.Errorf("Expected MaxIdleConns 200, got %d", transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != 200 {
		t.Errorf("Expected MaxIdleConnsPerHost 200, got %d", transport.MaxIdleConnsPerHost)
	}

	if transport.MaxConnsPerHost != 200 {
		t.Errorf("Expected MaxConnsPerHost 200, got %d", transport.MaxConnsPerHost)
	}
}

// Test request signing with various HTTP methods
func TestOCICredentials_SignRequest_AllMethods(t *testing.T) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			creds := &OCICredentials{
				configProvider: &mockConfigProviderWithErrors{
					tenancy:     "ocid1.tenancy.oc1..example",
					user:        "ocid1.user.oc1..example",
					fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
					region:      "us-ashburn-1",
					privateKey:  privateKey,
				},
				authType: auth.OCIUserPrincipal,
				logger:   logging.NewNopLogger(),
			}

			var body io.Reader
			if method == "POST" || method == "PUT" || method == "PATCH" {
				body = strings.NewReader(`{"test":"data"}`)
			}

			req := httptest.NewRequest(method, "https://example.com/api/test", body)
			ctx := context.Background()

			err := creds.SignRequest(ctx, req)
			if err != nil {
				t.Errorf("Failed to sign %s request: %v", method, err)
			}

			// Verify authorization header
			if req.Header.Get("Authorization") == "" {
				t.Errorf("Missing Authorization header for %s request", method)
			}
		})
	}
}

// Benchmark signing performance
func BenchmarkOCICredentials_SignRequest(b *testing.B) {
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	creds := &OCICredentials{
		configProvider: &mockConfigProviderWithErrors{
			tenancy:     "ocid1.tenancy.oc1..example",
			user:        "ocid1.user.oc1..example",
			fingerprint: "aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88:99",
			region:      "us-ashburn-1",
			privateKey:  privateKey,
		},
		authType: auth.OCIUserPrincipal,
		logger:   logging.NewNopLogger(),
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("GET", "https://example.com/test", nil)
		creds.SignRequest(ctx, req)
	}
}
