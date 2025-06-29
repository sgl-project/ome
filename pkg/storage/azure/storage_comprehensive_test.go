package azure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/sgl-project/ome/pkg/auth"
	authazure "github.com/sgl-project/ome/pkg/auth/azure"
	"github.com/sgl-project/ome/pkg/logging"
	pkgstorage "github.com/sgl-project/ome/pkg/storage"
)

// Mock Azure client implementation
type mockAzureClient struct {
	containers   map[string]*mockContainer
	mu           sync.Mutex
	failUpload   bool
	failDownload bool
	failDelete   bool
}

func newMockAzureClient() *mockAzureClient {
	return &mockAzureClient{
		containers: make(map[string]*mockContainer),
	}
}

func (m *mockAzureClient) UploadStream(ctx context.Context, containerName, blobName string, body io.Reader, options *azblob.UploadStreamOptions) (azblob.UploadStreamResponse, error) {
	if m.failUpload {
		return azblob.UploadStreamResponse{}, errors.New("upload failed")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	container, exists := m.containers[containerName]
	if !exists {
		container = &mockContainer{
			name:  containerName,
			blobs: make(map[string]*mockBlob),
		}
		m.containers[containerName] = container
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return azblob.UploadStreamResponse{}, err
	}

	container.mu.Lock()
	container.blobs[blobName] = &mockBlob{
		name:       blobName,
		data:       data,
		properties: map[string]string{},
		created:    time.Now(),
		modified:   time.Now(),
	}
	container.mu.Unlock()

	return azblob.UploadStreamResponse{}, nil
}

func (m *mockAzureClient) UploadFile(ctx context.Context, containerName, blobName string, file io.Reader, options *azblob.UploadFileOptions) (azblob.UploadFileResponse, error) {
	_, err := m.UploadStream(ctx, containerName, blobName, file, nil)
	return azblob.UploadFileResponse{}, err
}

func (m *mockAzureClient) DownloadStream(ctx context.Context, containerName, blobName string, options *azblob.DownloadStreamOptions) (azblob.DownloadStreamResponse, error) {
	if m.failDownload {
		return azblob.DownloadStreamResponse{}, errors.New("download failed")
	}

	m.mu.Lock()
	container, exists := m.containers[containerName]
	m.mu.Unlock()

	if !exists {
		return azblob.DownloadStreamResponse{}, fmt.Errorf("container not found")
	}

	container.mu.Lock()
	_, existsBlob := container.blobs[blobName]
	container.mu.Unlock()

	if !existsBlob {
		return azblob.DownloadStreamResponse{}, &mockError{message: "BlobNotFound"}
	}

	// Return an empty response - the actual body is handled differently in real SDK
	var resp azblob.DownloadStreamResponse
	// In a real implementation, we would set the body reader
	// For tests, we'll just return an empty response
	return resp, nil
}

func (m *mockAzureClient) DownloadFile(ctx context.Context, containerName, blobName, fileName string, options *azblob.DownloadFileOptions) (int64, error) {
	// Check if download should fail
	if m.failDownload {
		return 0, errors.New("download failed")
	}

	// Check container exists
	m.mu.Lock()
	container, exists := m.containers[containerName]
	m.mu.Unlock()

	if !exists {
		return 0, fmt.Errorf("container not found")
	}

	// Check blob exists
	container.mu.Lock()
	blob, exists := container.blobs[blobName]
	container.mu.Unlock()

	if !exists {
		return 0, &mockError{message: "BlobNotFound"}
	}

	// Write data to file
	err := os.WriteFile(fileName, blob.data, 0644)
	if err != nil {
		return 0, err
	}

	return int64(len(blob.data)), nil
}

func (m *mockAzureClient) DeleteBlob(ctx context.Context, containerName, blobName string, options *blob.DeleteOptions) (blob.DeleteResponse, error) {
	if m.failDelete {
		return blob.DeleteResponse{}, errors.New("delete failed")
	}

	m.mu.Lock()
	container, exists := m.containers[containerName]
	m.mu.Unlock()

	if !exists {
		return blob.DeleteResponse{}, fmt.Errorf("container not found")
	}

	container.mu.Lock()
	defer container.mu.Unlock()

	if _, exists := container.blobs[blobName]; !exists {
		return blob.DeleteResponse{}, &mockError{message: "BlobNotFound"}
	}

	delete(container.blobs, blobName)
	return blob.DeleteResponse{}, nil
}

func (m *mockAzureClient) ServiceClient() azServiceClient {
	return &mockServiceClient{
		containers: m.containers,
		client:     m,
	}
}

// Mock container
type mockContainer struct {
	name  string
	blobs map[string]*mockBlob
	mu    sync.Mutex
}

// Mock blob
type mockBlob struct {
	name       string
	data       []byte
	properties map[string]string
	created    time.Time
	modified   time.Time
	blockIDs   []string
	blocks     map[string][]byte
}

// Test Azure credentials mock
type testAzureCredentials struct {
	*authazure.AzureCredentials
	tokenCred azcore.TokenCredential
}

func (t *testAzureCredentials) GetTokenCredential() azcore.TokenCredential {
	return t.tokenCred
}

func (t *testAzureCredentials) Provider() auth.Provider {
	return auth.ProviderAzure
}

func (t *testAzureCredentials) Type() auth.AuthType {
	return auth.AzureClientSecret
}

func (t *testAzureCredentials) Token(ctx context.Context) (string, error) {
	return "test-token", nil
}

func (t *testAzureCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	return nil
}

func (t *testAzureCredentials) Refresh(ctx context.Context) error {
	return nil
}

func (t *testAzureCredentials) IsExpired() bool {
	return false
}

// Comprehensive tests

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
			credentials: &mockCredentials{
				provider: auth.ProviderGCP,
			},
			expectError: true,
			errorMsg:    "invalid credentials type",
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
						if storage.config.BlockSize != 4*1024*1024 {
							t.Errorf("Expected default BlockSize 4MB, got %d", storage.config.BlockSize)
						}
						if storage.config.Concurrency != 5 {
							t.Errorf("Expected default Concurrency 5, got %d", storage.config.Concurrency)
						}
					}
				}
			}
		})
	}
}

func TestNewWithClient(t *testing.T) {
	logger := logging.NewNopLogger()
	mockClient := newMockAzureClient()

	tests := []struct {
		name        string
		cfg         *Config
		credentials auth.Credentials
	}{
		{
			name: "With nil config",
			cfg:  nil,
			credentials: &testAzureCredentials{
				AzureCredentials: &authazure.AzureCredentials{},
			},
		},
		{
			name: "With custom config",
			cfg: &Config{
				AccountName: "testaccount",
				BlockSize:   8 * 1024 * 1024,
				Concurrency: 10,
			},
			credentials: &testAzureCredentials{
				AzureCredentials: &authazure.AzureCredentials{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := NewWithClient(mockClient, tt.cfg, tt.credentials, logger)

			if storage == nil {
				t.Error("Expected storage instance but got nil")
			}

			// Verify config defaults were applied
			if tt.cfg == nil {
				if storage.config.BlockSize != 4*1024*1024 {
					t.Errorf("Expected default BlockSize 4MB, got %d", storage.config.BlockSize)
				}
				if storage.config.Concurrency != 5 {
					t.Errorf("Expected default Concurrency 5, got %d", storage.config.Concurrency)
				}
			} else {
				if storage.config.AccountName != tt.cfg.AccountName {
					t.Errorf("Expected AccountName %s, got %s", tt.cfg.AccountName, storage.config.AccountName)
				}
			}
		})
	}
}

func TestAzureStorage_Upload(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockAzureClient()

	s := &AzureStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	uri := pkgstorage.ObjectURI{
		Provider:   pkgstorage.ProviderAzure,
		BucketName: "test-container",
		ObjectName: "test-blob",
	}

	tests := []struct {
		name        string
		data        []byte
		opts        []pkgstorage.UploadOption
		failUpload  bool
		expectError bool
	}{
		{
			name: "Success with options",
			data: []byte("test data"),
			opts: []pkgstorage.UploadOption{
				pkgstorage.WithContentType("text/plain"),
				pkgstorage.WithMetadata(map[string]string{"key": "value"}),
			},
			expectError: false,
		},
		{
			name:        "Success without options",
			data:        []byte("test data"),
			expectError: false,
		},
		{
			name:        "Upload failure",
			data:        []byte("test data"),
			failUpload:  true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile := "/tmp/test-upload"
			if err := os.WriteFile(tmpFile, tt.data, 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile)

			mockClient.failUpload = tt.failUpload

			err := s.Upload(ctx, tmpFile, uri, tt.opts...)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify stored blob
			if !tt.expectError {
				mockClient.mu.Lock()
				container := mockClient.containers["test-container"]
				mockClient.mu.Unlock()

				if container == nil {
					t.Error("Container not found")
				} else {
					container.mu.Lock()
					blob := container.blobs["test-blob"]
					container.mu.Unlock()

					if blob == nil {
						t.Error("Blob not stored")
					} else if !bytes.Equal(blob.data, tt.data) {
						t.Error("Stored data mismatch")
					}
				}
			}
		})
	}
}

func TestAzureStorage_Download(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockAzureClient()

	// Pre-populate test data
	container := &mockContainer{
		name:  "test-container",
		blobs: make(map[string]*mockBlob),
	}
	container.blobs["existing-blob"] = &mockBlob{
		name: "existing-blob",
		data: []byte("test content"),
	}
	mockClient.containers["test-container"] = container

	s := &AzureStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name         string
		uri          pkgstorage.ObjectURI
		target       string
		failDownload bool
		expectError  bool
	}{
		{
			name: "Success",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderAzure,
				BucketName: "test-container",
				ObjectName: "existing-blob",
			},
			target:      "/tmp/test-download",
			expectError: false,
		},
		{
			name: "Blob not found",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderAzure,
				BucketName: "test-container",
				ObjectName: "non-existing",
			},
			target:      "/tmp/test-download",
			expectError: true,
		},
		{
			name: "Download failure",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderAzure,
				BucketName: "test-container",
				ObjectName: "existing-blob",
			},
			target:       "/tmp/test-download",
			failDownload: true,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.failDownload = tt.failDownload

			err := s.Download(ctx, tt.uri, tt.target)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Clean up
			os.Remove(tt.target)
		})
	}
}

func TestAzureStorage_Delete(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockAzureClient()

	// Pre-populate test data
	container := &mockContainer{
		name:  "test-container",
		blobs: make(map[string]*mockBlob),
	}
	container.blobs["existing-blob"] = &mockBlob{
		name: "existing-blob",
		data: []byte("test content"),
	}
	mockClient.containers["test-container"] = container

	s := &AzureStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		uri         pkgstorage.ObjectURI
		failDelete  bool
		expectError bool
	}{
		{
			name: "Success",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderAzure,
				BucketName: "test-container",
				ObjectName: "existing-blob",
			},
			expectError: false,
		},
		{
			name: "Blob not found",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderAzure,
				BucketName: "test-container",
				ObjectName: "non-existing",
			},
			expectError: true,
		},
		{
			name: "Delete failure",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderAzure,
				BucketName: "test-container",
				ObjectName: "existing-blob",
			},
			failDelete:  true,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Re-create blob if needed
			if tt.name == "Delete failure" {
				container.mu.Lock()
				container.blobs["existing-blob"] = &mockBlob{
					name: "existing-blob",
					data: []byte("test content"),
				}
				container.mu.Unlock()
			}

			mockClient.failDelete = tt.failDelete

			err := s.Delete(ctx, tt.uri)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify deletion
			if !tt.expectError && !tt.failDelete {
				container.mu.Lock()
				_, exists := container.blobs[tt.uri.ObjectName]
				container.mu.Unlock()

				if exists {
					t.Error("Blob not deleted")
				}
			}
		})
	}
}

func TestAzureStorage_Put(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockAzureClient()

	s := &AzureStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	uri := pkgstorage.ObjectURI{
		Provider:   pkgstorage.ProviderAzure,
		BucketName: "test-container",
		ObjectName: "test-blob",
	}

	tests := []struct {
		name        string
		data        []byte
		size        int64
		opts        []pkgstorage.UploadOption
		expectError bool
	}{
		{
			name: "Success with options",
			data: []byte("test data"),
			size: 9,
			opts: []pkgstorage.UploadOption{
				pkgstorage.WithContentType("text/plain"),
				pkgstorage.WithMetadata(map[string]string{"key": "value"}),
			},
			expectError: false,
		},
		{
			name:        "Success without options",
			data:        []byte("test data"),
			size:        9,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bytes.NewReader(tt.data)

			err := s.Put(ctx, uri, reader, tt.size, tt.opts...)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify stored blob
			if !tt.expectError {
				mockClient.mu.Lock()
				container := mockClient.containers["test-container"]
				mockClient.mu.Unlock()

				if container == nil {
					t.Error("Container not found")
				} else {
					container.mu.Lock()
					blob := container.blobs["test-blob"]
					container.mu.Unlock()

					if blob == nil {
						t.Error("Blob not stored")
					} else if !bytes.Equal(blob.data, tt.data) {
						t.Error("Stored data mismatch")
					}
				}
			}
		})
	}
}

func TestAzureStorage_Get(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockAzureClient()

	// Pre-populate test data
	container := &mockContainer{
		name:  "test-container",
		blobs: make(map[string]*mockBlob),
	}
	container.blobs["existing-blob"] = &mockBlob{
		name: "existing-blob",
		data: []byte("test content"),
	}
	mockClient.containers["test-container"] = container

	s := &AzureStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		uri         pkgstorage.ObjectURI
		expectError bool
		expectData  string
	}{
		{
			name: "Success",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderAzure,
				BucketName: "test-container",
				ObjectName: "existing-blob",
			},
			expectError: false,
			expectData:  "test content",
		},
		{
			name: "Blob not found",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderAzure,
				BucketName: "test-container",
				ObjectName: "non-existing",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := s.Get(ctx, tt.uri)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && reader != nil {
				defer reader.Close()
				data, err := io.ReadAll(reader)
				if err != nil {
					t.Errorf("Failed to read data: %v", err)
				}
				if string(data) != tt.expectData {
					t.Errorf("Expected data %q, got %q", tt.expectData, string(data))
				}
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Additional mock implementations for comprehensive testing

// mockServiceClient implements the ServiceClient interface
type mockServiceClient struct {
	containers map[string]*mockContainer
	client     *mockAzureClient
}

func (m *mockServiceClient) NewContainerClient(name string) azContainerClient {
	return &mockContainerClientImpl{
		name:       name,
		containers: m.containers,
		client:     m.client,
	}
}

// mockContainerClientImpl implements azContainerClient interface
type mockContainerClientImpl struct {
	name       string
	containers map[string]*mockContainer
	client     *mockAzureClient
}

func (m *mockContainerClientImpl) NewBlobClient(blobName string) azBlobClient {
	return &mockBlobClientImpl{
		containerName: m.name,
		blobName:      blobName,
		containers:    m.containers,
		client:        m.client,
	}
}

func (m *mockContainerClientImpl) NewBlockBlobClient(blobName string) azBlockBlobClient {
	return &mockBlockBlobClientImpl{
		containerName: m.name,
		blobName:      blobName,
		containers:    m.containers,
		client:        m.client,
	}
}

func (m *mockContainerClientImpl) NewListBlobsFlatPager(options *container.ListBlobsFlatOptions) *runtime.Pager[container.ListBlobsFlatResponse] {
	// Return nil for tests - actual list testing would implement this
	return nil
}

// mockBlobClientImpl implements azBlobClient interface
type mockBlobClientImpl struct {
	containerName string
	blobName      string
	containers    map[string]*mockContainer
	client        *mockAzureClient
}

func (m *mockBlobClientImpl) GetProperties(ctx context.Context, options *blob.GetPropertiesOptions) (blob.GetPropertiesResponse, error) {
	container, exists := m.containers[m.containerName]
	if !exists {
		var emptyResp blob.GetPropertiesResponse
		return emptyResp, &mockError{message: "ContainerNotFound"}
	}

	container.mu.Lock()
	mockBlob, exists := container.blobs[m.blobName]
	container.mu.Unlock()

	if !exists {
		var emptyResp blob.GetPropertiesResponse
		return emptyResp, &mockError{message: "BlobNotFound"}
	}

	contentLength := int64(len(mockBlob.data))
	contentType := "application/octet-stream"
	if ct, ok := mockBlob.properties["Content-Type"]; ok {
		contentType = ct
	}

	// Create and return a GetPropertiesResponse
	var resp blob.GetPropertiesResponse
	resp.ContentLength = &contentLength
	resp.ContentType = &contentType
	resp.LastModified = &mockBlob.modified

	return resp, nil
}

func (m *mockBlobClientImpl) DownloadStream(ctx context.Context, options *blob.DownloadStreamOptions) (blob.DownloadStreamResponse, error) {
	resp, err := m.client.DownloadStream(ctx, m.containerName, m.blobName, nil)
	return blob.DownloadStreamResponse(resp), err
}

func (m *mockBlobClientImpl) DownloadFile(ctx context.Context, file *os.File, options *blob.DownloadFileOptions) (int64, error) {
	// Delegate to the mock client's DownloadFile
	return m.client.DownloadFile(ctx, m.containerName, m.blobName, file.Name(), nil)
}

func (m *mockBlobClientImpl) Delete(ctx context.Context, options *blob.DeleteOptions) (blob.DeleteResponse, error) {
	return m.client.DeleteBlob(ctx, m.containerName, m.blobName, options)
}

func (m *mockBlobClientImpl) StartCopyFromURL(ctx context.Context, copySource string, options *blob.StartCopyFromURLOptions) (blob.StartCopyFromURLResponse, error) {
	// Not implemented for tests
	return blob.StartCopyFromURLResponse{}, fmt.Errorf("not implemented")
}

// mockBlockBlobClientImpl implements azBlockBlobClient interface
type mockBlockBlobClientImpl struct {
	containerName string
	blobName      string
	containers    map[string]*mockContainer
	client        *mockAzureClient
}

func (m *mockBlockBlobClientImpl) Upload(ctx context.Context, body io.ReadSeekCloser, options *blockblob.UploadOptions) (blockblob.UploadResponse, error) {
	// Check if upload should fail
	if m.client.failUpload {
		return blockblob.UploadResponse{}, errors.New("upload failed")
	}

	// Read all data
	data, err := io.ReadAll(body)
	if err != nil {
		return blockblob.UploadResponse{}, err
	}

	// Store in mock
	m.client.mu.Lock()
	container, exists := m.containers[m.containerName]
	if !exists {
		container = &mockContainer{
			name:  m.containerName,
			blobs: make(map[string]*mockBlob),
		}
		m.containers[m.containerName] = container
	}
	m.client.mu.Unlock()

	container.mu.Lock()
	container.blobs[m.blobName] = &mockBlob{
		name:       m.blobName,
		data:       data,
		properties: make(map[string]string),
		created:    time.Now(),
		modified:   time.Now(),
	}
	container.mu.Unlock()

	return blockblob.UploadResponse{}, nil
}

func (m *mockBlockBlobClientImpl) StageBlock(ctx context.Context, base64BlockID string, body io.ReadSeekCloser, options *blockblob.StageBlockOptions) (blockblob.StageBlockResponse, error) {
	// Not implemented for tests
	return blockblob.StageBlockResponse{}, fmt.Errorf("not implemented")
}

func (m *mockBlockBlobClientImpl) CommitBlockList(ctx context.Context, base64BlockIDs []string, options *blockblob.CommitBlockListOptions) (blockblob.CommitBlockListResponse, error) {
	// Not implemented for tests
	return blockblob.CommitBlockListResponse{}, fmt.Errorf("not implemented")
}

func (m *mockBlockBlobClientImpl) GetBlockList(ctx context.Context, listType blockblob.BlockListType, options *blockblob.GetBlockListOptions) (blockblob.GetBlockListResponse, error) {
	// Not implemented for tests
	return blockblob.GetBlockListResponse{}, fmt.Errorf("not implemented")
}

// Test Exists functionality
func TestAzureStorage_Exists(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockAzureClient()

	// Pre-populate test data
	container := &mockContainer{
		name:  "test-container",
		blobs: make(map[string]*mockBlob),
	}
	container.blobs["existing-blob"] = &mockBlob{
		name: "existing-blob",
		data: []byte("test content"),
	}
	mockClient.containers["test-container"] = container

	s := &AzureStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		uri         pkgstorage.ObjectURI
		expectExist bool
		expectError bool
	}{
		{
			name: "Existing blob",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderAzure,
				BucketName: "test-container",
				ObjectName: "existing-blob",
			},
			expectExist: true,
			expectError: false,
		},
		{
			name: "Non-existing blob",
			uri: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderAzure,
				BucketName: "test-container",
				ObjectName: "non-existing",
			},
			expectExist: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := s.Exists(ctx, tt.uri)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if exists != tt.expectExist {
				t.Errorf("Expected exists=%v, got %v", tt.expectExist, exists)
			}
		})
	}
}

// Test List functionality
func TestAzureStorage_List(t *testing.T) {
	// Skip for now - List requires complex pager mock
	t.Skip("List test skipped - requires full mock implementation")
}

// Test GetObjectInfo functionality
func TestAzureStorage_GetObjectInfo(t *testing.T) {
	// Skip for now - GetObjectInfo requires proper mock implementation
	t.Skip("GetObjectInfo test skipped - requires full mock implementation")
}

// Test Copy functionality
func TestAzureStorage_Copy(t *testing.T) {
	// Skip for now - Copy requires proper mock implementation
	t.Skip("Copy test skipped - requires full mock implementation")
}

// Test multipart operations
func TestAzureStorage_MultipartOperations(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockAzureClient()

	s := &AzureStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	uri := pkgstorage.ObjectURI{
		Provider:   pkgstorage.ProviderAzure,
		BucketName: "test-container",
		ObjectName: "test-multipart",
	}

	t.Run("InitiateMultipartUpload", func(t *testing.T) {
		uploadID, err := s.InitiateMultipartUpload(ctx, uri)
		if err != nil {
			t.Errorf("Failed to initiate multipart upload: %v", err)
		}
		if uploadID == "" {
			t.Error("Expected non-empty upload ID")
		}
		if !strings.Contains(uploadID, "azure-upload") {
			t.Error("Upload ID should contain 'azure-upload' prefix")
		}
	})

	t.Run("UploadPart", func(t *testing.T) {
		// Skip for now - UploadPart requires proper mock implementation
		t.Skip("UploadPart test skipped - requires full mock implementation")
	})

	t.Run("CompleteMultipartUpload", func(t *testing.T) {
		// Skip for now - CompleteMultipartUpload requires proper mock implementation
		t.Skip("CompleteMultipartUpload test skipped - requires full mock implementation")
	})

	t.Run("AbortMultipartUpload", func(t *testing.T) {
		uploadID := "azure-upload-test-123"

		err := s.AbortMultipartUpload(ctx, uri, uploadID)
		if err != nil {
			t.Errorf("Failed to abort multipart upload: %v", err)
		}
	})
}

// Test edge cases and error handling
func TestAzureStorage_EdgeCases(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockAzureClient()

	s := &AzureStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	t.Run("Upload empty file", func(t *testing.T) {
		tmpFile := "/tmp/empty-file"
		if err := os.WriteFile(tmpFile, []byte{}, 0644); err != nil {
			t.Fatalf("Failed to create empty file: %v", err)
		}
		defer os.Remove(tmpFile)

		uri := pkgstorage.ObjectURI{
			Provider:   pkgstorage.ProviderAzure,
			BucketName: "test-container",
			ObjectName: "empty-blob",
		}

		err := s.Upload(ctx, tmpFile, uri)
		if err != nil {
			t.Errorf("Failed to upload empty file: %v", err)
		}
	})

	t.Run("Upload non-existent file", func(t *testing.T) {
		uri := pkgstorage.ObjectURI{
			Provider:   pkgstorage.ProviderAzure,
			BucketName: "test-container",
			ObjectName: "test-blob",
		}

		err := s.Upload(ctx, "/non/existent/file", uri)
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})

	t.Run("Download to invalid path", func(t *testing.T) {
		// Pre-populate test data
		container := &mockContainer{
			name:  "test-container",
			blobs: make(map[string]*mockBlob),
		}
		container.blobs["test-blob"] = &mockBlob{
			name: "test-blob",
			data: []byte("test content"),
		}
		mockClient.containers["test-container"] = container

		uri := pkgstorage.ObjectURI{
			Provider:   pkgstorage.ProviderAzure,
			BucketName: "test-container",
			ObjectName: "test-blob",
		}

		err := s.Download(ctx, uri, "/root/cannot/write/here")
		if err == nil {
			t.Error("Expected error for invalid download path")
		}
	})

	t.Run("Put with nil reader", func(t *testing.T) {
		uri := pkgstorage.ObjectURI{
			Provider:   pkgstorage.ProviderAzure,
			BucketName: "test-container",
			ObjectName: "test-blob",
		}

		err := s.Put(ctx, uri, nil, 0)
		if err == nil {
			t.Error("Expected error for nil reader")
		}
	})
}
