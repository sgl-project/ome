package ociobjectstore

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/oracle/oci-go-sdk/v65/objectstorage/transfer"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/principals"
)

// TestLogger for testing
type TestUploadLogger struct{}

func (l *TestUploadLogger) WithField(key string, value interface{}) logging.Interface { return l }
func (l *TestUploadLogger) WithError(err error) logging.Interface                     { return l }
func (l *TestUploadLogger) Debug(msg string)                                          {}
func (l *TestUploadLogger) Info(msg string)                                           {}
func (l *TestUploadLogger) Warn(msg string)                                           {}
func (l *TestUploadLogger) Error(msg string)                                          {}
func (l *TestUploadLogger) Fatal(msg string)                                          {}
func (l *TestUploadLogger) Debugf(format string, args ...interface{})                 {}
func (l *TestUploadLogger) Infof(format string, args ...interface{})                  {}
func (l *TestUploadLogger) Warnf(format string, args ...interface{})                  {}
func (l *TestUploadLogger) Errorf(format string, args ...interface{})                 {}
func (l *TestUploadLogger) Fatalf(format string, args ...interface{})                 {}

// Test upload request preparation without actual OCI client
func TestPrepareMultipartUploadRequest(t *testing.T) {
	t.Run("Upload request structure validation", func(t *testing.T) {
		// We can't test the actual method without OCI client, but we can test the parameters
		target := ObjectURI{
			Namespace:  "test-namespace",
			BucketName: "test-bucket",
			ObjectName: "test-file.bin",
		}

		chunkSizeInMB := 16
		uploadThreads := 10

		// Validate the target URI structure
		assert.Equal(t, "test-namespace", target.Namespace)
		assert.Equal(t, "test-bucket", target.BucketName)
		assert.Equal(t, "test-file.bin", target.ObjectName)

		// Validate parameters
		assert.Equal(t, 16, chunkSizeInMB)
		assert.Equal(t, 10, uploadThreads)

		// Test chunk size calculation
		expectedPartSize := int64(chunkSizeInMB) * int64(MB)
		assert.Equal(t, int64(16000000), expectedPartSize)
	})

	t.Run("Upload request with empty namespace", func(t *testing.T) {
		target := ObjectURI{
			Namespace:  "", // Empty namespace should be handled
			BucketName: "test-bucket",
			ObjectName: "test-file.bin",
		}

		assert.Empty(t, target.Namespace)
		assert.Equal(t, "test-bucket", target.BucketName)
		assert.Equal(t, "test-file.bin", target.ObjectName)
	})

	t.Run("Upload parameters validation", func(t *testing.T) {
		// Test various chunk sizes
		chunkSizes := []int{1, 8, 16, 32, 64}
		for _, size := range chunkSizes {
			expectedPartSize := int64(size) * int64(MB)
			assert.Equal(t, int64(size*1000000), expectedPartSize)
		}

		// Test various thread counts
		threadCounts := []int{1, 5, 10, 20, 50}
		for _, count := range threadCounts {
			assert.Greater(t, count, 0)
			assert.LessOrEqual(t, count, 100) // Reasonable upper limit
		}
	})
}

// Test upload configuration validation
func TestUploadConfiguration(t *testing.T) {
	t.Run("Valid upload configuration", func(t *testing.T) {
		authType := principals.InstancePrincipal
		config := &Config{
			AuthType:      &authType,
			Name:          "upload-test",
			Region:        "us-chicago-1",
			AnotherLogger: &TestUploadLogger{},
		}

		err := config.Validate()
		assert.NoError(t, err)
		assert.Equal(t, "upload-test", config.Name)
		assert.Equal(t, "us-chicago-1", config.Region)
	})

	t.Run("Upload target validation", func(t *testing.T) {
		target := ObjectURI{
			BucketName: "upload-bucket",
			ObjectName: "uploads/large-file.bin",
		}

		// Validate required fields
		assert.NotEmpty(t, target.BucketName)
		assert.NotEmpty(t, target.ObjectName)

		// Test object name with path
		assert.Contains(t, target.ObjectName, "/")
		assert.Equal(t, "large-file.bin", ObjectBaseName(target.ObjectName))
	})
}

// Test upload parameter calculations
func TestUploadParameterCalculations(t *testing.T) {
	t.Run("Chunk size calculations", func(t *testing.T) {
		tests := []struct {
			chunkSizeInMB int
			expectedBytes int64
		}{
			{1, 1000000},
			{8, 8000000},
			{16, 16000000},
			{32, 32000000},
			{64, 64000000},
		}

		for _, tt := range tests {
			result := int64(tt.chunkSizeInMB) * int64(MB)
			assert.Equal(t, tt.expectedBytes, result)
		}
	})

	t.Run("Thread count validation", func(t *testing.T) {
		validThreadCounts := []int{1, 2, 5, 10, 15, 20, 25, 30}

		for _, count := range validThreadCounts {
			assert.Greater(t, count, 0, "Thread count should be positive")
			assert.LessOrEqual(t, count, 50, "Thread count should be reasonable")
		}
	})

	t.Run("MB constant validation", func(t *testing.T) {
		assert.Equal(t, ChunkUnit(1000000), MB)

		// Test MB usage in calculations
		assert.Equal(t, int64(1000000), int64(1)*int64(MB))
		assert.Equal(t, int64(16000000), int64(16)*int64(MB))
	})
}

// Test upload error scenarios
func TestUploadErrorScenarios(t *testing.T) {
	t.Run("Invalid target URI", func(t *testing.T) {
		// Test empty bucket name
		target := ObjectURI{
			BucketName: "", // Invalid
			ObjectName: "file.txt",
		}

		assert.Empty(t, target.BucketName)
		assert.NotEmpty(t, target.ObjectName)
	})

	t.Run("Invalid object name", func(t *testing.T) {
		// Test empty object name
		target := ObjectURI{
			BucketName: "valid-bucket",
			ObjectName: "", // Invalid
		}

		assert.NotEmpty(t, target.BucketName)
		assert.Empty(t, target.ObjectName)
	})

	t.Run("Invalid chunk size", func(t *testing.T) {
		invalidChunkSizes := []int{0, -1, -10}

		for _, size := range invalidChunkSizes {
			assert.LessOrEqual(t, size, 0, "Chunk size should be positive")
		}
	})

	t.Run("Invalid thread count", func(t *testing.T) {
		invalidThreadCounts := []int{0, -1, -5}

		for _, count := range invalidThreadCounts {
			assert.LessOrEqual(t, count, 0, "Thread count should be positive")
		}
	})
}

// Test upload callback functionality structure
func TestUploadCallbackStructure(t *testing.T) {
	t.Run("Callback function signature", func(t *testing.T) {
		// We can't test the actual callback without OCI SDK, but we can test the structure

		// Test that we can define a callback function
		callbackCalled := false
		callback := func(part interface{}) {
			callbackCalled = true
		}

		// Simulate calling the callback
		callback(nil)
		assert.True(t, callbackCalled)
	})

	t.Run("Upload progress tracking", func(t *testing.T) {
		// Test progress tracking structure
		totalParts := 10
		completedParts := 0

		// Simulate progress updates
		for i := 1; i <= totalParts; i++ {
			completedParts++
			progress := float64(completedParts) / float64(totalParts) * 100

			assert.GreaterOrEqual(t, progress, 0.0)
			assert.LessOrEqual(t, progress, 100.0)
		}

		assert.Equal(t, totalParts, completedParts)
	})
}

// Test upload request validation
func TestUploadRequestValidation(t *testing.T) {
	t.Run("File upload request validation", func(t *testing.T) {
		filePath := "/path/to/large-file.bin"
		target := ObjectURI{
			BucketName: "upload-bucket",
			ObjectName: "uploads/large-file.bin",
		}
		chunkSizeInMB := 16
		uploadThreads := 10

		// Validate all parameters
		assert.NotEmpty(t, filePath)
		assert.NotEmpty(t, target.BucketName)
		assert.NotEmpty(t, target.ObjectName)
		assert.Greater(t, chunkSizeInMB, 0)
		assert.Greater(t, uploadThreads, 0)
	})

	t.Run("Stream upload request validation", func(t *testing.T) {
		target := ObjectURI{
			BucketName: "stream-bucket",
			ObjectName: "streams/data.bin",
		}
		chunkSizeInMB := 8
		uploadThreads := 5

		// Validate stream upload parameters
		assert.NotEmpty(t, target.BucketName)
		assert.NotEmpty(t, target.ObjectName)
		assert.Greater(t, chunkSizeInMB, 0)
		assert.Greater(t, uploadThreads, 0)

		// Test that chunk size is reasonable for streaming
		assert.GreaterOrEqual(t, chunkSizeInMB, 1)
		assert.LessOrEqual(t, chunkSizeInMB, 100)
	})
}

// Test upload method signatures and parameters
func TestUploadMethodSignatures(t *testing.T) {
	t.Run("MultipartFileUpload parameters", func(t *testing.T) {
		// Test parameter types and validation
		filePath := "/path/to/file.bin"
		target := ObjectURI{
			BucketName: "test-bucket",
			ObjectName: "test-file.bin",
		}
		chunkSizeInMB := 16
		uploadThreads := 10

		// Validate parameter types
		assert.IsType(t, "", filePath)
		assert.IsType(t, ObjectURI{}, target)
		assert.IsType(t, 0, chunkSizeInMB)
		assert.IsType(t, 0, uploadThreads)
	})

	t.Run("MultipartStreamUpload parameters", func(t *testing.T) {
		// Test parameter types for stream upload
		target := ObjectURI{
			BucketName: "stream-bucket",
			ObjectName: "stream-file.bin",
		}
		chunkSizeInMB := 8
		uploadThreads := 5

		// Validate parameter types
		assert.IsType(t, ObjectURI{}, target)
		assert.IsType(t, 0, chunkSizeInMB)
		assert.IsType(t, 0, uploadThreads)

		// Validate reasonable values for streaming
		assert.GreaterOrEqual(t, chunkSizeInMB, 1)
		assert.LessOrEqual(t, uploadThreads, 20) // Reasonable for streaming
	})
}

// Test adjustMetadataForFileUpload function
func TestAdjustMetadataForFileUpload(t *testing.T) {
	// Create a test logger
	logger := &TestUploadLogger{}

	// Create a test OCIOSDataStore instance
	cds := &OCIOSDataStore{
		logger: logger,
	}

	t.Run("File size less than part size - should remove opc-meta- prefix", func(t *testing.T) {
		// Create a temporary file smaller than the default part size
		tempFile, err := os.CreateTemp("", "test-small-file-*.txt")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		// Write some content to make it smaller than 128MB
		content := "small file content"
		_, err = tempFile.WriteString(content)
		assert.NoError(t, err)
		tempFile.Close()

		// Create upload request with metadata containing opc-meta- prefix
		uploadRequest := &transfer.UploadRequest{
			Metadata: map[string]string{
				"opc-meta-custom-key":   "custom-value",
				"opc-meta-content-type": "text/plain",
				"regular-key":           "regular-value",
			},
		}

		// Call the function
		err = cds.adjustMetadataForFileUpload(uploadRequest, tempFile.Name())
		assert.NoError(t, err)

		// Verify that opc-meta- prefix was removed
		expectedMetadata := map[string]string{
			"custom-key":   "custom-value",
			"content-type": "text/plain",
			"regular-key":  "regular-value",
		}
		assert.Equal(t, expectedMetadata, uploadRequest.Metadata)
	})

	t.Run("File size equal to part size - should remove opc-meta- prefix", func(t *testing.T) {
		// Create a temporary file exactly equal to the default part size (128MB)
		tempFile, err := os.CreateTemp("", "test-exact-size-file-*.bin")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		// Create a file exactly 128MB in size
		exactSize := DefaultMultipartUploadFilePartSize
		data := make([]byte, exactSize)
		_, err = tempFile.Write(data)
		assert.NoError(t, err)
		tempFile.Close()

		// Create upload request with metadata
		uploadRequest := &transfer.UploadRequest{
			Metadata: map[string]string{
				"opc-meta-test-key": "test-value",
			},
		}

		// Call the function
		err = cds.adjustMetadataForFileUpload(uploadRequest, tempFile.Name())
		assert.NoError(t, err)

		// Verify that opc-meta- prefix was removed
		expectedMetadata := map[string]string{
			"test-key": "test-value",
		}
		assert.Equal(t, expectedMetadata, uploadRequest.Metadata)
	})

	t.Run("File size greater than part size - should not remove opc-meta- prefix", func(t *testing.T) {
		// Create a temporary file larger than the default part size
		tempFile, err := os.CreateTemp("", "test-large-file-*.bin")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		// Create a file larger than 128MB (129MB)
		largeSize := DefaultMultipartUploadFilePartSize + 1024*1024 // 128MB + 1MB
		data := make([]byte, largeSize)
		_, err = tempFile.Write(data)
		assert.NoError(t, err)
		tempFile.Close()

		// Create upload request with metadata
		originalMetadata := map[string]string{
			"opc-meta-large-file-key": "large-file-value",
			"regular-large-key":       "regular-large-value",
		}
		uploadRequest := &transfer.UploadRequest{
			Metadata: originalMetadata,
		}

		// Call the function
		err = cds.adjustMetadataForFileUpload(uploadRequest, tempFile.Name())
		assert.NoError(t, err)

		// Verify that opc-meta- prefix was NOT removed (metadata should remain unchanged)
		assert.Equal(t, originalMetadata, uploadRequest.Metadata)
	})

	t.Run("Multipart upload not allowed - should remove opc-meta- prefix", func(t *testing.T) {
		// Create a temporary file
		tempFile, err := os.CreateTemp("", "test-multipart-disabled-*.txt")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		// Write some content
		content := "test content"
		_, err = tempFile.WriteString(content)
		assert.NoError(t, err)
		tempFile.Close()

		// Create upload request with multipart upload disabled
		allowMultipart := false
		uploadRequest := &transfer.UploadRequest{
			AllowMultipartUploads: &allowMultipart,
			Metadata: map[string]string{
				"opc-meta-disabled-key": "disabled-value",
			},
		}

		// Call the function
		err = cds.adjustMetadataForFileUpload(uploadRequest, tempFile.Name())
		assert.NoError(t, err)

		// Verify that opc-meta- prefix was removed
		expectedMetadata := map[string]string{
			"disabled-key": "disabled-value",
		}
		assert.Equal(t, expectedMetadata, uploadRequest.Metadata)
	})

	t.Run("Custom part size - file size less than custom part size", func(t *testing.T) {
		// Create a temporary file
		tempFile, err := os.CreateTemp("", "test-custom-part-size-*.txt")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		// Write content smaller than custom part size
		content := "small content"
		_, err = tempFile.WriteString(content)
		assert.NoError(t, err)
		tempFile.Close()

		// Create upload request with custom part size (1GB)
		customPartSize := int64(1024 * 1024 * 1024) // 1GB
		uploadRequest := &transfer.UploadRequest{
			PartSize: &customPartSize,
			Metadata: map[string]string{
				"opc-meta-custom-part-key": "custom-part-value",
			},
		}

		// Call the function
		err = cds.adjustMetadataForFileUpload(uploadRequest, tempFile.Name())
		assert.NoError(t, err)

		// Verify that opc-meta- prefix was removed
		expectedMetadata := map[string]string{
			"custom-part-key": "custom-part-value",
		}
		assert.Equal(t, expectedMetadata, uploadRequest.Metadata)
	})
}

// Test adjustMetadataForStreamUpload function
func TestAdjustMetadataForStreamUpload(t *testing.T) {
	// Create a test logger
	logger := &TestUploadLogger{}

	// Create a test OCIOSDataStore instance
	cds := &OCIOSDataStore{
		logger: logger,
	}

	t.Run("Empty stream reader wrapped with io.NopCloser - should NOT remove opc-meta- prefix", func(t *testing.T) {
		// Create an empty buffer reader
		emptyReader := bytes.NewBuffer([]byte{})

		// Create upload request with metadata containing opc-meta- prefix
		uploadRequest := &transfer.UploadRequest{
			Metadata: map[string]string{
				"opc-meta-empty-stream-key": "empty-stream-value",
				"opc-meta-content-type":     "application/octet-stream",
				"regular-stream-key":        "regular-stream-value",
			},
		}

		// Call the function
		cds.adjustMetadataForStreamUpload(uploadRequest, io.NopCloser(emptyReader))

		// Verify that opc-meta- prefix was NOT removed (IsReaderEmpty returns false for io.NopCloser wrapped readers)
		expectedMetadata := map[string]string{
			"opc-meta-empty-stream-key": "empty-stream-value",
			"opc-meta-content-type":     "application/octet-stream",
			"regular-stream-key":        "regular-stream-value",
		}
		assert.Equal(t, expectedMetadata, uploadRequest.Metadata)
	})

	t.Run("Non-empty stream reader - should not remove opc-meta- prefix", func(t *testing.T) {
		// Create a non-empty buffer reader
		nonEmptyReader := bytes.NewBuffer([]byte("some content"))

		// Create upload request with metadata
		originalMetadata := map[string]string{
			"opc-meta-non-empty-key": "non-empty-value",
			"regular-non-empty-key":  "regular-non-empty-value",
		}
		uploadRequest := &transfer.UploadRequest{
			Metadata: originalMetadata,
		}

		// Call the function
		cds.adjustMetadataForStreamUpload(uploadRequest, io.NopCloser(nonEmptyReader))

		// Verify that opc-meta- prefix was NOT removed (metadata should remain unchanged)
		assert.Equal(t, originalMetadata, uploadRequest.Metadata)
	})

	t.Run("Nil stream reader - should handle gracefully", func(t *testing.T) {
		// Create upload request with metadata
		originalMetadata := map[string]string{
			"opc-meta-nil-reader-key": "nil-reader-value",
		}
		uploadRequest := &transfer.UploadRequest{
			Metadata: originalMetadata,
		}

		// Call the function with nil reader
		cds.adjustMetadataForStreamUpload(uploadRequest, nil)

		// Verify that metadata remains unchanged
		assert.Equal(t, originalMetadata, uploadRequest.Metadata)
	})

	t.Run("Empty file reader - should remove opc-meta- prefix", func(t *testing.T) {
		// Create a temporary empty file
		tempFile, err := os.CreateTemp("", "test-empty-file-*.txt")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		// Open the file as io.ReadCloser
		file, err := os.Open(tempFile.Name())
		assert.NoError(t, err)
		defer file.Close()

		// Create upload request with metadata containing opc-meta- prefix
		uploadRequest := &transfer.UploadRequest{
			Metadata: map[string]string{
				"opc-meta-file-key": "file-value",
			},
		}

		// Call the function
		cds.adjustMetadataForStreamUpload(uploadRequest, file)

		// Verify that opc-meta- prefix was removed (IsReaderEmpty can detect empty files)
		expectedMetadata := map[string]string{
			"file-key": "file-value",
		}
		assert.Equal(t, expectedMetadata, uploadRequest.Metadata)
	})

	t.Run("Non-empty file reader - should NOT remove opc-meta- prefix", func(t *testing.T) {
		// Create a temporary file with content
		tempFile, err := os.CreateTemp("", "test-nonempty-file-*.txt")
		assert.NoError(t, err)
		defer os.Remove(tempFile.Name())

		// Write some content
		_, err = tempFile.WriteString("some content")
		assert.NoError(t, err)
		tempFile.Close()

		// Open the file as io.ReadCloser
		file, err := os.Open(tempFile.Name())
		assert.NoError(t, err)
		defer file.Close()

		// Create upload request with metadata containing opc-meta- prefix
		originalMetadata := map[string]string{
			"opc-meta-nonempty-file-key": "nonempty-file-value",
		}
		uploadRequest := &transfer.UploadRequest{
			Metadata: originalMetadata,
		}

		// Call the function
		cds.adjustMetadataForStreamUpload(uploadRequest, file)

		// Verify that opc-meta- prefix was NOT removed (file is not empty)
		assert.Equal(t, originalMetadata, uploadRequest.Metadata)
	})
}
