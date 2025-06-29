package aws

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sgl-project/ome/pkg/auth"
	authaws "github.com/sgl-project/ome/pkg/auth/aws"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

func TestS3Storage_Provider(t *testing.T) {
	s := &S3Storage{
		logger: logging.NewNopLogger(),
	}

	if provider := s.Provider(); provider != storage.ProviderAWS {
		t.Errorf("Expected provider %s, got %s", storage.ProviderAWS, provider)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.PartSize != 5*1024*1024 {
		t.Errorf("Expected PartSize 5MB, got %d", cfg.PartSize)
	}

	if cfg.Concurrency != 10 {
		t.Errorf("Expected Concurrency 10, got %d", cfg.Concurrency)
	}
}

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
				provider: auth.ProviderOCI,
			},
			expectError: true,
			errorMsg:    "invalid credentials type",
		},
		{
			name: "Valid with nil config",
			cfg:  nil,
			credentials: &testAWSCredentials{
				credProvider: credentials.NewStaticCredentialsProvider("test", "test", ""),
				region:       "us-east-1",
			},
			expectError: false,
		},
		{
			name: "Valid with custom config",
			cfg: &Config{
				Region:      "us-west-2",
				PartSize:    10 * 1024 * 1024,
				Concurrency: 20,
			},
			credentials: &testAWSCredentials{
				credProvider: credentials.NewStaticCredentialsProvider("test", "test", ""),
				region:       "",
			},
			expectError: false,
		},
		{
			name: "Valid with endpoint config",
			cfg: &Config{
				Endpoint:       "http://localhost:9000",
				ForcePathStyle: true,
				DisableSSL:     true,
			},
			credentials: &testAWSCredentials{
				credProvider: credentials.NewStaticCredentialsProvider("test", "test", ""),
				region:       "us-east-1",
			},
			expectError: false,
		},
		{
			name: "Config with zero values gets defaults",
			cfg: &Config{
				Region: "eu-west-1",
				// PartSize and Concurrency are zero
			},
			credentials: &testAWSCredentials{
				credProvider: credentials.NewStaticCredentialsProvider("test", "test", ""),
				region:       "",
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
					if tt.cfg == nil || tt.cfg.PartSize == 0 {
						if storage.config.PartSize != 5*1024*1024 {
							t.Errorf("Expected default PartSize 5MB, got %d", storage.config.PartSize)
						}
					}
					if tt.cfg == nil || tt.cfg.Concurrency == 0 {
						if storage.config.Concurrency != 10 {
							t.Errorf("Expected default Concurrency 10, got %d", storage.config.Concurrency)
						}
					}
				}
			}
		})
	}
}

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "NoSuchKey error",
			err:      &mockError{message: "NoSuchKey"},
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

// Mock implementations for testing

type mockCredentials struct {
	provider auth.Provider
}

func (m *mockCredentials) Provider() auth.Provider {
	return m.provider
}

func (m *mockCredentials) Type() auth.AuthType {
	return auth.AWSAccessKey
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

// testAWSCredentials is a mock AWS credentials for testing New
type testAWSCredentials struct {
	*authaws.AWSCredentials
	credProvider aws.CredentialsProvider
	region       string
}

func (t *testAWSCredentials) GetCredentialsProvider() aws.CredentialsProvider {
	return t.credProvider
}

func (t *testAWSCredentials) GetRegion() string {
	return t.region
}

func (t *testAWSCredentials) Provider() auth.Provider {
	return auth.ProviderAWS
}

func (t *testAWSCredentials) Type() auth.AuthType {
	return auth.AWSAccessKey
}

func (t *testAWSCredentials) Token(ctx context.Context) (string, error) {
	return "test-token", nil
}

func (t *testAWSCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	return nil
}

func (t *testAWSCredentials) Refresh(ctx context.Context) error {
	return nil
}

func (t *testAWSCredentials) IsExpired() bool {
	return false
}

type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}

// Mock AWS credentials for testing
type mockAWSCredentials struct {
	*authaws.AWSCredentials
}

func (m *mockAWSCredentials) GetCredentialsProvider() aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider("test-key", "test-secret", "")
}

// mockS3ClientFull implements all S3 operations for comprehensive testing
type mockS3ClientFull struct {

	// Control behavior
	failPutObject    bool
	failGetObject    bool
	failDeleteObject bool
	failListObjects  bool
	failHeadObject   bool
	failCopyObject   bool

	// Track calls
	putObjectCalled    bool
	getObjectCalled    bool
	deleteObjectCalled bool
	listObjectsCalled  bool
	headObjectCalled   bool
	copyObjectCalled   bool

	// Mock data
	objects map[string]mockObject
}

type mockObject struct {
	data         []byte
	contentType  string
	storageClass string
	metadata     map[string]string
	size         int64
}

func (m *mockS3ClientFull) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	m.putObjectCalled = true

	if m.failPutObject {
		return nil, &types.NoSuchBucket{Message: aws.String("bucket not found")}
	}

	// Read data
	data, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}

	// Store object
	key := aws.ToString(params.Key)
	m.objects[key] = mockObject{
		data:         data,
		contentType:  aws.ToString(params.ContentType),
		storageClass: string(params.StorageClass),
		metadata:     params.Metadata,
		size:         int64(len(data)),
	}

	return &s3.PutObjectOutput{
		ETag: aws.String("mock-etag"),
	}, nil
}

func (m *mockS3ClientFull) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	m.getObjectCalled = true

	if m.failGetObject {
		return nil, &types.NoSuchKey{Message: aws.String("key not found")}
	}

	key := aws.ToString(params.Key)
	obj, exists := m.objects[key]
	if !exists {
		return nil, &types.NoSuchKey{Message: aws.String("key not found")}
	}

	// Handle range requests
	var data []byte
	if params.Range != nil {
		// Parse range header (simplified)
		data = obj.data // In real implementation, would parse range
	} else {
		data = obj.data
	}

	return &s3.GetObjectOutput{
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String(obj.contentType),
		StorageClass:  types.StorageClass(obj.storageClass),
		Metadata:      obj.metadata,
		ETag:          aws.String("mock-etag"),
	}, nil
}

func (m *mockS3ClientFull) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	m.deleteObjectCalled = true

	if m.failDeleteObject {
		return nil, &types.NoSuchBucket{Message: aws.String("bucket not found")}
	}

	key := aws.ToString(params.Key)
	delete(m.objects, key)

	return &s3.DeleteObjectOutput{}, nil
}

func (m *mockS3ClientFull) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	m.listObjectsCalled = true

	if m.failListObjects {
		return nil, &types.NoSuchBucket{Message: aws.String("bucket not found")}
	}

	var objects []types.Object
	prefix := aws.ToString(params.Prefix)

	for key, obj := range m.objects {
		if prefix == "" || len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			objects = append(objects, types.Object{
				Key:          aws.String(key),
				Size:         aws.Int64(obj.size),
				StorageClass: types.ObjectStorageClass(obj.storageClass),
				ETag:         aws.String("mock-etag"),
			})
		}
	}

	return &s3.ListObjectsV2Output{
		Contents:    objects,
		IsTruncated: aws.Bool(false),
		KeyCount:    aws.Int32(int32(len(objects))),
		MaxKeys:     params.MaxKeys,
		Name:        params.Bucket,
		Prefix:      params.Prefix,
	}, nil
}

func (m *mockS3ClientFull) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	m.headObjectCalled = true

	if m.failHeadObject {
		return nil, &types.NoSuchBucket{Message: aws.String("bucket error")}
	}

	key := aws.ToString(params.Key)
	obj, exists := m.objects[key]
	if !exists {
		return nil, &types.NotFound{Message: aws.String("not found")}
	}

	return &s3.HeadObjectOutput{
		ContentLength: aws.Int64(obj.size),
		ContentType:   aws.String(obj.contentType),
		StorageClass:  types.StorageClass(obj.storageClass),
		Metadata:      obj.metadata,
		ETag:          aws.String("mock-etag"),
	}, nil
}

func (m *mockS3ClientFull) CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	m.copyObjectCalled = true

	if m.failCopyObject {
		return nil, &types.NoSuchBucket{Message: aws.String("bucket not found")}
	}

	// Parse copy source - format is "bucket/key"
	copySource := aws.ToString(params.CopySource)
	parts := strings.SplitN(copySource, "/", 2)
	if len(parts) != 2 {
		return nil, &types.NoSuchKey{Message: aws.String("invalid copy source")}
	}
	sourceKey := parts[1]

	// Find source object
	srcObj, exists := m.objects[sourceKey]
	if !exists {
		return nil, &types.NoSuchKey{Message: aws.String("source not found")}
	}

	// Copy object
	destKey := aws.ToString(params.Key)
	m.objects[destKey] = mockObject{
		data:         srcObj.data,
		contentType:  aws.ToString(params.ContentType),
		storageClass: string(params.StorageClass),
		metadata:     params.Metadata,
		size:         srcObj.size,
	}

	return &s3.CopyObjectOutput{
		CopyObjectResult: &types.CopyObjectResult{
			ETag: aws.String("mock-etag"),
		},
	}, nil
}

func (m *mockS3ClientFull) CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	return &s3.CreateMultipartUploadOutput{
		UploadId: aws.String("test-upload-id"),
	}, nil
}

func (m *mockS3ClientFull) UploadPart(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	return &s3.UploadPartOutput{
		ETag: aws.String("test-etag"),
	}, nil
}

func (m *mockS3ClientFull) CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	return &s3.CompleteMultipartUploadOutput{
		ETag: aws.String("final-etag"),
	}, nil
}

func (m *mockS3ClientFull) AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	return &s3.AbortMultipartUploadOutput{}, nil
}

func TestS3Storage_Upload(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3ClientFull{
		objects: make(map[string]mockObject),
	}

	s := &S3Storage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	uri := storage.ObjectURI{
		Provider:   storage.ProviderAWS,
		BucketName: "test-bucket",
		ObjectName: "test-object",
	}

	tests := []struct {
		name        string
		data        []byte
		opts        []storage.UploadOption
		setupMock   func()
		expectError bool
	}{
		{
			name: "Success with options",
			data: []byte("test data"),
			opts: []storage.UploadOption{
				storage.WithContentType("text/plain"),
				storage.WithStorageClass("GLACIER"),
				storage.WithMetadata(map[string]string{"key": "value"}),
			},
			setupMock: func() {
				mockClient.failPutObject = false
			},
			expectError: false,
		},
		{
			name: "Success without options",
			data: []byte("test data"),
			setupMock: func() {
				mockClient.failPutObject = false
			},
			expectError: false,
		},
		{
			name: "Failure",
			data: []byte("test data"),
			setupMock: func() {
				mockClient.failPutObject = true
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.putObjectCalled = false
			tt.setupMock()

			// Create a temporary file with the test data
			tmpFile := "/tmp/test-upload"
			if err := os.WriteFile(tmpFile, tt.data, 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile)

			err := s.Upload(ctx, tmpFile, uri, tt.opts...)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !mockClient.putObjectCalled {
				t.Error("Expected putObject to be called")
			}

			// Verify stored object
			if !tt.expectError {
				obj, exists := mockClient.objects["test-object"]
				if !exists {
					t.Error("Object not stored")
				}
				if !bytes.Equal(obj.data, tt.data) {
					t.Error("Stored data mismatch")
				}
			}
		})
	}
}

func TestS3Storage_Download(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3ClientFull{
		objects: map[string]mockObject{
			"existing-object": {
				data:        []byte("test content"),
				contentType: "text/plain",
				size:        12,
			},
		},
	}

	s := &S3Storage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		uri         storage.ObjectURI
		target      string
		opts        *storage.DownloadOptions
		setupMock   func()
		expectError bool
	}{
		{
			name: "Success",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "existing-object",
			},
			target: "/tmp/test-download",
			setupMock: func() {
				mockClient.failGetObject = false
				mockClient.failHeadObject = false
			},
			expectError: false,
		},
		{
			name: "Object not found",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "non-existing",
			},
			target: "/tmp/test-download",
			setupMock: func() {
				mockClient.failGetObject = false
				mockClient.failHeadObject = false
			},
			expectError: true,
		},
		{
			name: "S3 error",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "existing-object",
			},
			target: "/tmp/test-download",
			setupMock: func() {
				mockClient.failGetObject = true
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.getObjectCalled = false
			mockClient.headObjectCalled = false
			tt.setupMock()

			err := s.Download(ctx, tt.uri, tt.target)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestS3Storage_Delete(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3ClientFull{
		objects: map[string]mockObject{
			"existing-object": {
				data: []byte("test content"),
				size: 12,
			},
		},
	}

	s := &S3Storage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		uri         storage.ObjectURI
		setupMock   func()
		expectError bool
	}{
		{
			name: "Success",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "existing-object",
			},
			setupMock: func() {
				mockClient.failDeleteObject = false
			},
			expectError: false,
		},
		{
			name: "Failure",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "test-object",
			},
			setupMock: func() {
				mockClient.failDeleteObject = true
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.deleteObjectCalled = false
			tt.setupMock()

			err := s.Delete(ctx, tt.uri)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !mockClient.deleteObjectCalled {
				t.Error("Expected deleteObject to be called")
			}

			// Verify deletion
			if !tt.expectError {
				_, exists := mockClient.objects[tt.uri.ObjectName]
				if exists {
					t.Error("Object not deleted")
				}
			}
		})
	}
}

func TestS3Storage_List(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3ClientFull{
		objects: map[string]mockObject{
			"prefix/object1": {size: 100},
			"prefix/object2": {size: 200},
			"other/object3":  {size: 300},
		},
	}

	s := &S3Storage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		bucketName  string
		prefix      string
		setupMock   func()
		expectError bool
		expectCount int
	}{
		{
			name:       "List with prefix",
			bucketName: "test-bucket",
			prefix:     "prefix/",
			setupMock: func() {
				mockClient.failListObjects = false
			},
			expectError: false,
			expectCount: 2,
		},
		{
			name:       "List all",
			bucketName: "test-bucket",
			prefix:     "",
			setupMock: func() {
				mockClient.failListObjects = false
			},
			expectError: false,
			expectCount: 3,
		},
		{
			name:       "Failure",
			bucketName: "test-bucket",
			prefix:     "",
			setupMock: func() {
				mockClient.failListObjects = true
			},
			expectError: true,
			expectCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.listObjectsCalled = false
			tt.setupMock()

			uri := storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: tt.bucketName,
			}
			listOpts := storage.ListOptions{
				Prefix: tt.prefix,
			}
			objects, err := s.List(ctx, uri, listOpts)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !mockClient.listObjectsCalled {
				t.Error("Expected listObjects to be called")
			}

			if len(objects) != tt.expectCount {
				t.Errorf("Expected %d objects, got %d", tt.expectCount, len(objects))
			}
		})
	}
}

func TestS3Storage_Copy(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3ClientFull{
		objects: map[string]mockObject{
			"source-object": {
				data:        []byte("source content"),
				contentType: "text/plain",
				size:        14,
			},
		},
	}

	s := &S3Storage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		source      storage.ObjectURI
		target      storage.ObjectURI
		setupMock   func()
		expectError bool
	}{
		{
			name: "Success",
			source: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "source-object",
			},
			target: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "target-object",
			},
			setupMock: func() {
				mockClient.failCopyObject = false
			},
			expectError: false,
		},
		{
			name: "Source not found",
			source: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "non-existing",
			},
			target: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "target-object",
			},
			setupMock: func() {
				mockClient.failCopyObject = false
			},
			expectError: true,
		},
		{
			name: "Copy failure",
			source: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "source-object",
			},
			target: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "target-object",
			},
			setupMock: func() {
				mockClient.failCopyObject = true
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.copyObjectCalled = false
			tt.setupMock()

			err := s.Copy(ctx, tt.source, tt.target)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !mockClient.copyObjectCalled {
				t.Error("Expected copyObject to be called")
			}

			// Verify copy
			if !tt.expectError {
				targetObj, exists := mockClient.objects[tt.target.ObjectName]
				if !exists {
					t.Error("Target object not created")
				}
				sourceObj := mockClient.objects[tt.source.ObjectName]
				if !bytes.Equal(targetObj.data, sourceObj.data) {
					t.Error("Copied data mismatch")
				}
			}
		})
	}
}

func TestS3Storage_Get(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3ClientFull{
		objects: map[string]mockObject{
			"existing-object": {
				data:        []byte("test content"),
				contentType: "text/plain",
				size:        12,
			},
		},
	}

	s := &S3Storage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		uri         storage.ObjectURI
		setupMock   func()
		expectError bool
		expectData  string
	}{
		{
			name: "Success",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "existing-object",
			},
			setupMock: func() {
				mockClient.failGetObject = false
			},
			expectError: false,
			expectData:  "test content",
		},
		{
			name: "Object not found",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "non-existing",
			},
			setupMock: func() {
				mockClient.failGetObject = false
			},
			expectError: true,
		},
		{
			name: "S3 error",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "existing-object",
			},
			setupMock: func() {
				mockClient.failGetObject = true
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.getObjectCalled = false
			tt.setupMock()

			reader, err := s.Get(ctx, tt.uri)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !mockClient.getObjectCalled {
				t.Error("Expected getObject to be called")
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

func TestS3Storage_Exists(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3ClientFull{
		objects: map[string]mockObject{
			"existing-object": {
				data: []byte("test content"),
				size: 12,
			},
		},
	}

	s := &S3Storage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name         string
		uri          storage.ObjectURI
		setupMock    func()
		expectExists bool
		expectError  bool
	}{
		{
			name: "Object exists",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "existing-object",
			},
			setupMock: func() {
				mockClient.failHeadObject = false
			},
			expectExists: true,
			expectError:  false,
		},
		{
			name: "Object not found",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "non-existing",
			},
			setupMock: func() {
				mockClient.failHeadObject = false
			},
			expectExists: false,
			expectError:  false,
		},
		{
			name: "S3 error",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "existing-object",
			},
			setupMock: func() {
				mockClient.failHeadObject = true
			},
			expectExists: false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.headObjectCalled = false
			tt.setupMock()

			exists, err := s.Exists(ctx, tt.uri)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !mockClient.headObjectCalled {
				t.Error("Expected headObject to be called")
			}

			if exists != tt.expectExists {
				t.Errorf("Expected exists=%v, got %v", tt.expectExists, exists)
			}
		})
	}
}

func TestS3Storage_GetObjectInfo(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3ClientFull{
		objects: map[string]mockObject{
			"existing-object": {
				data:         []byte("test content"),
				contentType:  "text/plain",
				storageClass: "STANDARD",
				metadata:     map[string]string{"key": "value"},
				size:         12,
			},
		},
	}

	s := &S3Storage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		uri         storage.ObjectURI
		setupMock   func()
		expectError bool
	}{
		{
			name: "Success",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "existing-object",
			},
			setupMock: func() {
				mockClient.failHeadObject = false
			},
			expectError: false,
		},
		{
			name: "Object not found",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "non-existing",
			},
			setupMock: func() {
				mockClient.failHeadObject = false
			},
			expectError: true,
		},
		{
			name: "S3 error",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: "test-bucket",
				ObjectName: "existing-object",
			},
			setupMock: func() {
				mockClient.failHeadObject = true
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.headObjectCalled = false
			tt.setupMock()

			info, err := s.GetObjectInfo(ctx, tt.uri)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !mockClient.headObjectCalled {
				t.Error("Expected headObject to be called")
			}

			if !tt.expectError && info != nil {
				if info.Name != tt.uri.ObjectName {
					t.Errorf("Expected name %s, got %s", tt.uri.ObjectName, info.Name)
				}
				if info.Size != 12 {
					t.Errorf("Expected size 12, got %d", info.Size)
				}
				if info.ContentType != "text/plain" {
					t.Errorf("Expected content type text/plain, got %s", info.ContentType)
				}
				if info.StorageClass != "STANDARD" {
					t.Errorf("Expected storage class STANDARD, got %s", info.StorageClass)
				}
			}
		})
	}
}
