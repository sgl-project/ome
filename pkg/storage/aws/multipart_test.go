package aws

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// mockS3Client implements a mock S3 client for testing
type mockS3Client struct {

	// Control behavior
	failCreateMultipart bool
	failUploadPart      bool
	failComplete        bool
	failAbort           bool

	// Track calls
	createMultipartCalled bool
	uploadPartCalled      bool
	completeCalled        bool
	abortCalled           bool

	// Stored data
	uploadID string
	parts    map[int32][]byte
}

func (m *mockS3Client) CreateMultipartUpload(ctx context.Context, params *s3.CreateMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CreateMultipartUploadOutput, error) {
	m.createMultipartCalled = true

	if m.failCreateMultipart {
		return nil, &types.NoSuchBucket{Message: aws.String("bucket not found")}
	}

	m.uploadID = "test-upload-id"
	if m.parts == nil {
		m.parts = make(map[int32][]byte)
	}

	return &s3.CreateMultipartUploadOutput{
		UploadId: aws.String(m.uploadID),
		Bucket:   params.Bucket,
		Key:      params.Key,
	}, nil
}

func (m *mockS3Client) UploadPart(ctx context.Context, params *s3.UploadPartInput, optFns ...func(*s3.Options)) (*s3.UploadPartOutput, error) {
	m.uploadPartCalled = true

	if m.failUploadPart {
		return nil, &types.NoSuchUpload{Message: aws.String("upload not found")}
	}

	// Read the part data
	data, err := io.ReadAll(params.Body)
	if err != nil {
		return nil, err
	}

	if m.parts == nil {
		m.parts = make(map[int32][]byte)
	}
	m.parts[*params.PartNumber] = data

	etag := aws.String("etag-" + string(*params.PartNumber))
	return &s3.UploadPartOutput{
		ETag: etag,
	}, nil
}

func (m *mockS3Client) CompleteMultipartUpload(ctx context.Context, params *s3.CompleteMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.CompleteMultipartUploadOutput, error) {
	m.completeCalled = true

	if m.failComplete {
		return nil, &types.NoSuchUpload{Message: aws.String("upload not found")}
	}

	return &s3.CompleteMultipartUploadOutput{
		Bucket: params.Bucket,
		Key:    params.Key,
		ETag:   aws.String("final-etag"),
	}, nil
}

func (m *mockS3Client) AbortMultipartUpload(ctx context.Context, params *s3.AbortMultipartUploadInput, optFns ...func(*s3.Options)) (*s3.AbortMultipartUploadOutput, error) {
	m.abortCalled = true

	if m.failAbort {
		return nil, &types.NoSuchUpload{Message: aws.String("upload not found")}
	}

	return &s3.AbortMultipartUploadOutput{}, nil
}

// Implement remaining s3Client methods for mock
func (m *mockS3Client) PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	return nil, nil
}

func (m *mockS3Client) GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return nil, nil
}

func (m *mockS3Client) DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return nil, nil
}

func (m *mockS3Client) ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return nil, nil
}

func (m *mockS3Client) HeadObject(ctx context.Context, params *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	return nil, nil
}

func (m *mockS3Client) CopyObject(ctx context.Context, params *s3.CopyObjectInput, optFns ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	return nil, nil
}

func TestS3Storage_InitiateMultipartUpload(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3Client{}

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
		name         string
		opts         []storage.UploadOption
		setupMock    func()
		expectError  bool
		expectCalled bool
	}{
		{
			name: "Success",
			opts: []storage.UploadOption{
				storage.WithContentType("text/plain"),
				storage.WithStorageClass("GLACIER"),
			},
			setupMock: func() {
				mockClient.failCreateMultipart = false
			},
			expectError:  false,
			expectCalled: true,
		},
		{
			name: "Failure",
			setupMock: func() {
				mockClient.failCreateMultipart = true
			},
			expectError:  true,
			expectCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.createMultipartCalled = false
			tt.setupMock()

			uploadID, err := s.InitiateMultipartUpload(ctx, uri, tt.opts...)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectCalled != mockClient.createMultipartCalled {
				t.Errorf("Expected createMultipartCalled=%v, got %v", tt.expectCalled, mockClient.createMultipartCalled)
			}

			if !tt.expectError && uploadID == "" {
				t.Error("Expected non-empty upload ID")
			}
		})
	}
}

func TestS3Storage_UploadPart(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3Client{}

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
		uploadID    string
		partNumber  int
		data        []byte
		setupMock   func()
		expectError bool
		expectETag  bool
	}{
		{
			name:       "Success",
			uploadID:   "test-upload-id",
			partNumber: 1,
			data:       []byte("test data"),
			setupMock: func() {
				mockClient.failUploadPart = false
			},
			expectError: false,
			expectETag:  true,
		},
		{
			name:       "Failure",
			uploadID:   "test-upload-id",
			partNumber: 1,
			data:       []byte("test data"),
			setupMock: func() {
				mockClient.failUploadPart = true
			},
			expectError: true,
			expectETag:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.uploadPartCalled = false
			tt.setupMock()

			reader := bytes.NewReader(tt.data)
			etag, err := s.UploadPart(ctx, uri, tt.uploadID, tt.partNumber, reader, int64(len(tt.data)))

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !mockClient.uploadPartCalled {
				t.Error("Expected uploadPart to be called")
			}

			if tt.expectETag && etag == "" {
				t.Error("Expected non-empty ETag")
			}
		})
	}
}

func TestS3Storage_CompleteMultipartUpload(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3Client{}

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
		uploadID    string
		parts       []storage.CompletedPart
		setupMock   func()
		expectError bool
	}{
		{
			name:     "Success",
			uploadID: "test-upload-id",
			parts: []storage.CompletedPart{
				{PartNumber: 1, ETag: "etag-1"},
				{PartNumber: 2, ETag: "etag-2"},
			},
			setupMock: func() {
				mockClient.failComplete = false
			},
			expectError: false,
		},
		{
			name:     "Failure",
			uploadID: "test-upload-id",
			parts: []storage.CompletedPart{
				{PartNumber: 1, ETag: "etag-1"},
			},
			setupMock: func() {
				mockClient.failComplete = true
			},
			expectError: true,
		},
		{
			name:     "Empty parts",
			uploadID: "test-upload-id",
			parts:    []storage.CompletedPart{},
			setupMock: func() {
				mockClient.failComplete = false
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.completeCalled = false
			tt.setupMock()

			err := s.CompleteMultipartUpload(ctx, uri, tt.uploadID, tt.parts)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !mockClient.completeCalled {
				t.Error("Expected complete to be called")
			}
		})
	}
}

func TestS3Storage_AbortMultipartUpload(t *testing.T) {
	ctx := context.Background()
	mockClient := &mockS3Client{}

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
		uploadID    string
		setupMock   func()
		expectError bool
	}{
		{
			name:     "Success",
			uploadID: "test-upload-id",
			setupMock: func() {
				mockClient.failAbort = false
			},
			expectError: false,
		},
		{
			name:     "Failure",
			uploadID: "test-upload-id",
			setupMock: func() {
				mockClient.failAbort = true
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient.abortCalled = false
			tt.setupMock()

			err := s.AbortMultipartUpload(ctx, uri, tt.uploadID)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !mockClient.abortCalled {
				t.Error("Expected abort to be called")
			}
		})
	}
}
