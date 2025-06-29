package github

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	pkgstorage "github.com/sgl-project/ome/pkg/storage"
)

// Mock HTTP transport for testing
type mockTransport struct {
	responses map[string]*http.Response
	requests  []http.Request
	mu        sync.Mutex
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		responses: make(map[string]*http.Response),
	}
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Store request for verification
	m.requests = append(m.requests, *req)

	// Create a key for the request
	key := fmt.Sprintf("%s %s", req.Method, req.URL.Path)

	// Check if we have a mock response
	if resp, exists := m.responses[key]; exists {
		return resp, nil
	}

	// Default 404 response
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
		Header:     make(http.Header),
	}, nil
}

func (m *mockTransport) setResponse(method, path string, statusCode int, body interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := fmt.Sprintf("%s %s", method, path)

	var bodyReader io.ReadCloser
	if body != nil {
		switch v := body.(type) {
		case string:
			bodyReader = io.NopCloser(strings.NewReader(v))
		case []byte:
			bodyReader = io.NopCloser(bytes.NewReader(v))
		default:
			data, _ := json.Marshal(body)
			bodyReader = io.NopCloser(bytes.NewReader(data))
		}
	} else {
		bodyReader = io.NopCloser(strings.NewReader(""))
	}

	m.responses[key] = &http.Response{
		StatusCode: statusCode,
		Body:       bodyReader,
		Header:     make(http.Header),
	}
}

// Mock GitHub credentials
type mockComprehensiveGitHubCredentials struct {
	provider   auth.Provider
	authType   auth.AuthType
	token      string
	httpClient *http.Client
}

func (m *mockComprehensiveGitHubCredentials) Provider() auth.Provider {
	return m.provider
}

func (m *mockComprehensiveGitHubCredentials) Type() auth.AuthType {
	return m.authType
}

func (m *mockComprehensiveGitHubCredentials) Token(ctx context.Context) (string, error) {
	if m.token == "" {
		return "", errors.New("no token available")
	}
	return m.token, nil
}

func (m *mockComprehensiveGitHubCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	if m.token != "" {
		req.Header.Set("Authorization", "Bearer "+m.token)
	}
	return nil
}

func (m *mockComprehensiveGitHubCredentials) Refresh(ctx context.Context) error {
	return nil
}

func (m *mockComprehensiveGitHubCredentials) IsExpired() bool {
	return false
}

func (m *mockComprehensiveGitHubCredentials) GetHTTPClient() *http.Client {
	return m.httpClient
}

// Helper function to create test storage with mock transport
func createTestStorage(t *testing.T) (*GitHubLFSStorage, *mockTransport) {
	mockTrans := newMockTransport()
	httpClient := &http.Client{Transport: mockTrans}

	creds := &mockComprehensiveGitHubCredentials{
		provider:   auth.ProviderGitHub,
		authType:   auth.GitHubPersonalAccessToken,
		token:      "test-token",
		httpClient: httpClient,
	}

	cfg := &Config{
		Owner:       "testowner",
		Repo:        "testrepo",
		APIEndpoint: "https://api.github.com",
		ChunkSize:   1024,
	}

	logger := logging.NewNopLogger()

	storage, err := New(context.Background(), cfg, creds, logger)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	return storage, mockTrans
}

// Test New function
func TestNew(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewNopLogger()

	tests := []struct {
		name        string
		cfg         *Config
		credentials auth.Credentials
		expectError bool
		errorMsg    string
	}{
		{
			name: "Invalid credentials type",
			cfg:  nil,
			credentials: &mockComprehensiveCredentials{
				provider: auth.ProviderAWS,
			},
			expectError: true,
			errorMsg:    "invalid credentials type",
		},
		{
			name: "Invalid with nil config (missing owner)",
			cfg:  nil,
			credentials: &mockComprehensiveGitHubCredentials{
				provider:   auth.ProviderGitHub,
				httpClient: &http.Client{},
			},
			expectError: true,
			errorMsg:    "owner is required",
		},
		{
			name: "Valid with custom config",
			cfg: &Config{
				Owner:       "myowner",
				Repo:        "myrepo",
				APIEndpoint: "https://github.example.com",
				ChunkSize:   50 * 1024 * 1024,
			},
			credentials: &mockComprehensiveGitHubCredentials{
				provider:   auth.ProviderGitHub,
				httpClient: &http.Client{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := New(ctx, tt.cfg, tt.credentials, logger)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if storage == nil {
					t.Error("Expected storage instance but got nil")
				} else {
					// Verify config defaults were applied
					if tt.cfg == nil {
						if storage.config.APIEndpoint != "https://api.github.com" {
							t.Errorf("Expected default APIEndpoint, got %s", storage.config.APIEndpoint)
						}
						if storage.config.ChunkSize != 100*1024*1024 {
							t.Errorf("Expected default ChunkSize 100MB, got %d", storage.config.ChunkSize)
						}
					}
				}
			}
		})
	}
}

// Test Download functionality
func TestGitHubLFSStorage_Download(t *testing.T) {
	ctx := context.Background()
	storage, mockTrans := createTestStorage(t)

	oid := "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"

	tests := []struct {
		name         string
		uri          pkgstorage.ObjectURI
		setupMocks   func()
		expectError  bool
		errorMsg     string
		expectedData string
	}{
		{
			name: "Success",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			setupMocks: func() {
				// Mock batch API response
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 11,
							Actions: map[string]LFSAction{
								"download": {
									Href: "https://lfs.github.com/download/test",
								},
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)

				// Mock download response
				mockTrans.setResponse("GET", "/download/test", 200, "test content")
			},
			expectError:  false,
			expectedData: "test content",
		},
		{
			name: "Batch API error",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			setupMocks: func() {
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 401, "Unauthorized")
			},
			expectError: true,
			errorMsg:    "batch request failed",
		},
		{
			name: "Object not found",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			setupMocks: func() {
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 11,
							Error: &LFSError{
								Code:    404,
								Message: "Object not found",
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)
			},
			expectError: true,
			errorMsg:    "Object not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock transport
			mockTrans.responses = make(map[string]*http.Response)
			mockTrans.requests = nil

			if tt.setupMocks != nil {
				tt.setupMocks()
			}

			// Create temp file for download
			tmpFile := "/tmp/test-download"
			defer os.Remove(tmpFile)

			err := storage.Download(ctx, tt.uri, tmpFile)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify downloaded content
				if tt.expectedData != "" {
					data, err := os.ReadFile(tmpFile)
					if err != nil {
						t.Errorf("Failed to read downloaded file: %v", err)
					} else if string(data) != tt.expectedData {
						t.Errorf("Expected data %q, got %q", tt.expectedData, string(data))
					}
				}
			}
		})
	}
}

// Test Upload functionality
func TestGitHubLFSStorage_Upload(t *testing.T) {
	ctx := context.Background()
	storage, mockTrans := createTestStorage(t)

	tests := []struct {
		name        string
		uri         pkgstorage.ObjectURI
		fileContent string
		setupMocks  func(oid string)
		expectError bool
		errorMsg    string
	}{
		{
			name: "Success",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			fileContent: "test content",
			setupMocks: func(oid string) {
				// Mock batch API response
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 12,
							Actions: map[string]LFSAction{
								"upload": {
									Href: "https://lfs.github.com/upload/test",
									Header: map[string]string{
										"Authorization": "Bearer upload-token",
									},
								},
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)

				// Mock upload response
				mockTrans.setResponse("PUT", "/upload/test", 200, "")
			},
			expectError: false,
		},
		{
			name: "File not found",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			fileContent: "",
			expectError: true,
			errorMsg:    "failed to open file",
		},
		{
			name: "Upload failure",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			fileContent: "test content",
			setupMocks: func(oid string) {
				// Mock batch API response
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 12,
							Actions: map[string]LFSAction{
								"upload": {
									Href: "https://lfs.github.com/upload/test",
								},
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)

				// Mock upload failure
				mockTrans.setResponse("PUT", "/upload/test", 500, "Server Error")
			},
			expectError: true,
			errorMsg:    "upload failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock transport
			mockTrans.responses = make(map[string]*http.Response)
			mockTrans.requests = nil

			// Create temp file for upload
			var tmpFile string
			if tt.fileContent != "" {
				tmpFile = "/tmp/test-upload"
				if err := os.WriteFile(tmpFile, []byte(tt.fileContent), 0644); err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				defer os.Remove(tmpFile)

				// Calculate OID for setup
				if tt.setupMocks != nil {
					oid := testCalculateOIDFromBytes([]byte(tt.fileContent))
					tt.setupMocks(oid)
				}
			} else {
				tmpFile = "/non/existent/file"
			}

			err := storage.Upload(ctx, tmpFile, tt.uri)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// Test Get functionality
func TestGitHubLFSStorage_Get(t *testing.T) {
	ctx := context.Background()
	storage, mockTrans := createTestStorage(t)

	oid := "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"

	tests := []struct {
		name         string
		uri          pkgstorage.ObjectURI
		setupMocks   func()
		expectError  bool
		errorMsg     string
		expectedData string
	}{
		{
			name: "Success",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			setupMocks: func() {
				// Mock batch API response
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 11,
							Actions: map[string]LFSAction{
								"download": {
									Href: "https://lfs.github.com/download/test",
								},
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)

				// Mock download response
				mockTrans.setResponse("GET", "/download/test", 200, "test content")
			},
			expectError:  false,
			expectedData: "test content",
		},
		{
			name: "Download URL error",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			setupMocks: func() {
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 401, "Unauthorized")
			},
			expectError: true,
			errorMsg:    "failed to get download URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock transport
			mockTrans.responses = make(map[string]*http.Response)
			mockTrans.requests = nil

			if tt.setupMocks != nil {
				tt.setupMocks()
			}

			reader, err := storage.Get(ctx, tt.uri)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if reader != nil {
					defer reader.Close()

					// Read and verify content
					data, err := io.ReadAll(reader)
					if err != nil {
						t.Errorf("Failed to read data: %v", err)
					} else if string(data) != tt.expectedData {
						t.Errorf("Expected data %q, got %q", tt.expectedData, string(data))
					}
				}
			}
		})
	}
}

// Test Put functionality
func TestGitHubLFSStorage_Put(t *testing.T) {
	ctx := context.Background()
	storage, mockTrans := createTestStorage(t)

	tests := []struct {
		name        string
		uri         pkgstorage.ObjectURI
		data        []byte
		setupMocks  func(oid string)
		expectError bool
		errorMsg    string
	}{
		{
			name: "Success",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			data: []byte("test content"),
			setupMocks: func(oid string) {
				// Mock batch API response
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 12,
							Actions: map[string]LFSAction{
								"upload": {
									Href: "https://lfs.github.com/upload/test",
								},
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)

				// Mock upload response
				mockTrans.setResponse("PUT", "/upload/test", 200, "")
			},
			expectError: false,
		},
		{
			name: "Upload URL error",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			data: []byte("test content"),
			setupMocks: func(oid string) {
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 403, "Forbidden")
			},
			expectError: true,
			errorMsg:    "failed to get upload URL",
		},
		{
			name: "Empty data",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			data: []byte{},
			setupMocks: func(oid string) {
				// Mock batch API response
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 0,
							Actions: map[string]LFSAction{
								"upload": {
									Href: "https://lfs.github.com/upload/test",
								},
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)

				// Mock upload response
				mockTrans.setResponse("PUT", "/upload/test", 200, "")
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock transport
			mockTrans.responses = make(map[string]*http.Response)
			mockTrans.requests = nil

			if tt.setupMocks != nil {
				oid := testCalculateOIDFromBytes(tt.data)
				tt.setupMocks(oid)
			}

			reader := bytes.NewReader(tt.data)
			err := storage.Put(ctx, tt.uri, reader, int64(len(tt.data)))

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// Test Exists functionality
func TestGitHubLFSStorage_Exists(t *testing.T) {
	ctx := context.Background()
	storage, mockTrans := createTestStorage(t)

	oid := "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"

	tests := []struct {
		name        string
		uri         pkgstorage.ObjectURI
		setupMocks  func()
		expectExist bool
		expectError bool
		errorMsg    string
	}{
		{
			name: "Object exists",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			setupMocks: func() {
				// Mock batch API response - object exists
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 11,
							Actions: map[string]LFSAction{
								"download": {
									Href: "https://lfs.github.com/download/test",
								},
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)
			},
			expectExist: true,
			expectError: false,
		},
		{
			name: "Object not found",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			setupMocks: func() {
				// Mock batch API response - object not found
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 11,
							Error: &LFSError{
								Code:    404,
								Message: "not found",
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)
			},
			expectExist: false,
			expectError: false,
		},
		{
			name: "API error",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			setupMocks: func() {
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 500, "Server Error")
			},
			expectExist: false,
			expectError: true,
			errorMsg:    "batch request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock transport
			mockTrans.responses = make(map[string]*http.Response)
			mockTrans.requests = nil

			if tt.setupMocks != nil {
				tt.setupMocks()
			}

			exists, err := storage.Exists(ctx, tt.uri)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if exists != tt.expectExist {
					t.Errorf("Expected exists=%v, got %v", tt.expectExist, exists)
				}
			}
		})
	}
}

// Test GetObjectInfo functionality
func TestGitHubLFSStorage_GetObjectInfo(t *testing.T) {
	ctx := context.Background()
	storage, mockTrans := createTestStorage(t)

	oid := "2c26b46b68ffc68ff99b453c1d30413413422d706483bfa0f98a5e886266e7ae"

	tests := []struct {
		name        string
		uri         pkgstorage.ObjectURI
		setupMocks  func()
		expectError bool
		errorMsg    string
		expectSize  int64
	}{
		{
			name: "Success",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			setupMocks: func() {
				// Mock batch API response
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 1234,
							Actions: map[string]LFSAction{
								"download": {
									Href: "https://lfs.github.com/download/test",
								},
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)
			},
			expectError: false,
			expectSize:  1234,
		},
		{
			name: "Object not found",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGitHub,
				BucketName: "testowner/testrepo",
				ObjectName: "test.txt",
			},
			setupMocks: func() {
				// Mock batch API response - object not found
				batchResp := &LFSBatchResponse{
					Transfer: "basic",
					Objects: []LFSBatchResponseObject{
						{
							OID:  oid,
							Size: 0,
							Error: &LFSError{
								Code:    404,
								Message: "not found",
							},
						},
					},
				}
				mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, batchResp)
			},
			expectError: true,
			errorMsg:    "LFS error: not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mock transport
			mockTrans.responses = make(map[string]*http.Response)
			mockTrans.requests = nil

			if tt.setupMocks != nil {
				tt.setupMocks()
			}

			info, err := storage.GetObjectInfo(ctx, tt.uri)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if info == nil {
					t.Error("Expected info but got nil")
				} else {
					if info.Size != tt.expectSize {
						t.Errorf("Expected size %d, got %d", tt.expectSize, info.Size)
					}
					// GitHub LFS returns OID as the name
					if info.Name != oid {
						t.Errorf("Expected name (OID) %s, got %s", oid, info.Name)
					}
				}
			}
		})
	}
}

// Test unsupported operations
func TestGitHubLFSStorage_UnsupportedOperations(t *testing.T) {
	ctx := context.Background()
	storage, _ := createTestStorage(t)

	uri := pkgstorage.ObjectURI{
		Provider:   pkgstorage.ProviderGitHub,
		BucketName: "testowner/testrepo",
		ObjectName: "test.txt",
	}

	// Test Delete
	err := storage.Delete(ctx, uri)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("Expected 'not supported' error for Delete, got: %v", err)
	}

	// Test List
	_, err = storage.List(ctx, uri, pkgstorage.ListOptions{})
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("Expected 'not supported' error for List, got: %v", err)
	}

	// Test Copy
	err = storage.Copy(ctx, uri, uri)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("Expected 'not supported' error for Copy, got: %v", err)
	}

	// Test InitiateMultipartUpload
	_, err = storage.InitiateMultipartUpload(ctx, uri)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("Expected 'not supported' error for InitiateMultipartUpload, got: %v", err)
	}

	// Test UploadPart
	_, err = storage.UploadPart(ctx, uri, "uploadID", 1, nil, 0)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("Expected 'not supported' error for UploadPart, got: %v", err)
	}

	// Test CompleteMultipartUpload
	err = storage.CompleteMultipartUpload(ctx, uri, "uploadID", nil)
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("Expected 'not supported' error for CompleteMultipartUpload, got: %v", err)
	}

	// Test AbortMultipartUpload
	err = storage.AbortMultipartUpload(ctx, uri, "uploadID")
	if err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Errorf("Expected 'not supported' error for AbortMultipartUpload, got: %v", err)
	}
}

// Test helper functions
func TestHelperFunctions(t *testing.T) {
	// Test calculateOID
	testData := []byte("hello world")
	expectedOID := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

	oid := testCalculateOIDFromBytes(testData)
	if oid != expectedOID {
		t.Errorf("Expected OID %s, got %s", expectedOID, oid)
	}

}

// Test LFS batch request functionality
func TestLFSBatchRequest(t *testing.T) {
	ctx := context.Background()
	storage, mockTrans := createTestStorage(t)

	// Test successful batch request
	mockResp := &LFSBatchResponse{
		Transfer: "basic",
		Objects: []LFSBatchResponseObject{
			{
				OID:  "test-oid",
				Size: 100,
			},
		},
	}
	mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, mockResp)

	req := &LFSBatchRequest{
		Operation: "download",
		Transfers: []string{"basic"},
		Objects: []LFSObject{
			{OID: "test-oid", Size: 100},
		},
	}

	resp, err := storage.lfsBatchRequest(ctx, req)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp == nil {
		t.Error("Expected response but got nil")
	}

	// Verify request was made with correct headers
	if len(mockTrans.requests) == 0 {
		t.Error("Expected at least one request")
	} else {
		lastReq := mockTrans.requests[len(mockTrans.requests)-1]
		if lastReq.Header.Get("Content-Type") != "application/vnd.git-lfs+json" {
			t.Errorf("Expected Content-Type header, got %s", lastReq.Header.Get("Content-Type"))
		}
		if lastReq.Header.Get("Accept") != "application/vnd.git-lfs+json" {
			t.Errorf("Expected Accept header, got %s", lastReq.Header.Get("Accept"))
		}
	}
}

// Test edge cases
func TestGitHubLFSStorage_EdgeCases(t *testing.T) {
	ctx := context.Background()
	storage, mockTrans := createTestStorage(t)

	// Test with missing bucket name
	uri := pkgstorage.ObjectURI{
		Provider:   pkgstorage.ProviderGitHub,
		BucketName: "",
		ObjectName: "test.txt",
	}

	err := storage.Download(ctx, uri, "/tmp/test")
	if err == nil {
		t.Error("Expected error for missing bucket name")
	}

	// Test with invalid bucket name format
	uri.BucketName = "invalid-format"
	err = storage.Download(ctx, uri, "/tmp/test")
	if err == nil {
		t.Error("Expected error for invalid bucket name format")
	}

	// Test with empty object name
	uri = pkgstorage.ObjectURI{
		Provider:   pkgstorage.ProviderGitHub,
		BucketName: "owner/repo",
		ObjectName: "",
	}

	err = storage.Download(ctx, uri, "/tmp/test")
	if err == nil {
		t.Error("Expected error for empty object name")
	}

	// Test with very large file size
	oid := testCalculateOIDFromBytes([]byte("large"))
	mockTrans.setResponse("POST", "/testowner/testrepo.git/info/lfs/objects/batch", 200, &LFSBatchResponse{
		Transfer: "basic",
		Objects: []LFSBatchResponseObject{
			{
				OID:  oid,
				Size: 10 * 1024 * 1024 * 1024, // 10GB
				Actions: map[string]LFSAction{
					"download": {
						Href: "https://lfs.github.com/download/large",
					},
				},
			},
		},
	})

	uri = pkgstorage.ObjectURI{
		Provider:   pkgstorage.ProviderGitHub,
		BucketName: "testowner/testrepo",
		ObjectName: "large.bin",
	}

	_, err = storage.GetObjectInfo(ctx, uri)
	if err != nil {
		t.Errorf("Unexpected error for large file: %v", err)
	}
}

// Mock credentials for testing non-GitHub credentials
type mockComprehensiveCredentials struct {
	provider auth.Provider
}

func (m *mockComprehensiveCredentials) Provider() auth.Provider {
	return m.provider
}

func (m *mockComprehensiveCredentials) Type() auth.AuthType {
	return auth.GitHubPersonalAccessToken
}

func (m *mockComprehensiveCredentials) Token(ctx context.Context) (string, error) {
	return "mock-token", nil
}

func (m *mockComprehensiveCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	return nil
}

func (m *mockComprehensiveCredentials) Refresh(ctx context.Context) error {
	return nil
}

func (m *mockComprehensiveCredentials) IsExpired() bool {
	return false
}

// Helper to calculate OID from bytes
func testCalculateOIDFromBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// Helper to calculate OID from file path
func calculateOIDFromPath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
