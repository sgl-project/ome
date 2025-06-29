package azure

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

func TestAzureStorage_Provider(t *testing.T) {
	s := &AzureStorage{
		logger: logging.NewNopLogger(),
	}

	if provider := s.Provider(); provider != storage.ProviderAzure {
		t.Errorf("Expected provider %s, got %s", storage.ProviderAzure, provider)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.BlockSize != 4*1024*1024 {
		t.Errorf("Expected BlockSize 4MB, got %d", cfg.BlockSize)
	}

	if cfg.Concurrency != 5 {
		t.Errorf("Expected Concurrency 5, got %d", cfg.Concurrency)
	}
}

func TestNew_InvalidCredentials(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewNopLogger()

	// Create non-Azure credentials
	mockCreds := &mockCredentials{
		provider: auth.ProviderGCP,
	}

	_, err := New(ctx, nil, mockCreds, logger)
	if err == nil {
		t.Error("Expected error for invalid credentials type")
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "BlobNotFound error",
			err:      &mockError{message: "BlobNotFound"},
			expected: true,
		},
		{
			name:     "NotFound error",
			err:      &mockError{message: "NotFound"},
			expected: true,
		},
		{
			name:     "404 error",
			err:      &mockError{message: "404"},
			expected: true,
		},
		{
			name:     "Other error",
			err:      &mockError{message: "AccessDenied"},
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBase64Encode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"000001", "MDAwMDAx"},
		{"test block", "dGVzdCBibG9jaw=="},
		{"hello/world", "aGVsbG8vd29ybGQ="},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := base64Encode(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestNopCloserSeeker(t *testing.T) {
	data := []byte("test data")
	reader := bytes.NewReader(data)
	ncs := &nopCloserSeeker{Reader: reader}

	// Test reading
	buf := make([]byte, 4)
	n, err := ncs.Read(buf)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if n != 4 {
		t.Errorf("Expected 4 bytes read, got %d", n)
	}
	if string(buf) != "test" {
		t.Errorf("Expected 'test', got %s", string(buf))
	}

	// Test seeking
	pos, err := ncs.Seek(0, 0)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if pos != 0 {
		t.Errorf("Expected position 0, got %d", pos)
	}

	// Test closing
	err = ncs.Close()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestMultipartUploadOperations(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewNopLogger()
	s := &AzureStorage{
		logger: logger,
		config: DefaultConfig(),
	}

	uri := storage.ObjectURI{
		Provider:   storage.ProviderAzure,
		BucketName: "test-container",
		ObjectName: "test-object",
	}

	// Test InitiateMultipartUpload
	uploadID, err := s.InitiateMultipartUpload(ctx, uri)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if uploadID == "" {
		t.Error("Expected non-empty upload ID")
	}

	// Test AbortMultipartUpload (should always succeed for Azure)
	err = s.AbortMultipartUpload(ctx, uri, uploadID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestCalculateMD5(t *testing.T) {
	testData := []byte("test data")
	expected := "63M6AMDJ0zbmVpGjerVCkw==" // base64 encoded MD5

	result := calculateMD5(testData)
	if result != expected {
		t.Errorf("Expected MD5 %s, got %s", expected, result)
	}
}

// Mock implementations for testing

type mockCredentials struct {
	provider auth.Provider
}

func (m *mockCredentials) Provider() auth.Provider {
	return m.provider
}

func (m *mockCredentials) Type() auth.AuthType {
	return auth.AzureClientSecret
}

func (m *mockCredentials) Token(ctx context.Context) (string, error) {
	return "mock-token", nil
}

func (m *mockCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	return nil
}

func (m *mockCredentials) Refresh(ctx context.Context) error {
	return nil
}

func (m *mockCredentials) IsExpired() bool {
	return false
}

type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}
