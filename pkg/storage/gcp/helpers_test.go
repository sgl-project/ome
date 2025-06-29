package gcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/sgl-project/ome/pkg/logging"
	pkgstorage "github.com/sgl-project/ome/pkg/storage"
)

func TestMultipartDownload(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockGCSClient()

	// Create test data
	testData := make([]byte, 100*1024*1024) // 100MB
	for i := range testData {
		testData[i] = byte(i % 256)
	}

	// Pre-populate test object
	bucket := mockClient.Bucket("test-bucket").(*mockBucket)
	bucket.objects["test-object"] = &mockObject{
		data:        testData,
		contentType: "application/octet-stream",
	}

	s := &GCSStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		source      pkgstorage.ObjectURI
		target      string
		objectSize  int64
		opts        pkgstorage.DownloadOptions
		expectError bool
	}{
		{
			name: "Successful multipart download",
			source: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGCP,
				BucketName: "test-bucket",
				ObjectName: "test-object",
			},
			target:     "/tmp/test-multipart-download",
			objectSize: int64(len(testData)),
			opts: pkgstorage.DownloadOptions{
				ChunkSizeInMB: 10,
				Threads:       4,
			},
		},
		{
			name: "Single part download",
			source: pkgstorage.ObjectURI{
				Provider:   pkgstorage.ProviderGCP,
				BucketName: "test-bucket",
				ObjectName: "test-object",
			},
			target:     "/tmp/test-single-part",
			objectSize: int64(len(testData)),
			opts: pkgstorage.DownloadOptions{
				ChunkSizeInMB: 200, // Larger than file
				Threads:       1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.multipartDownload(ctx, tt.source, tt.target, tt.objectSize, &tt.opts)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify downloaded file
			if !tt.expectError {
				data, err := os.ReadFile(tt.target)
				if err != nil {
					t.Fatalf("Failed to read downloaded file: %v", err)
				}
				if !bytes.Equal(data, testData) {
					t.Error("Downloaded data mismatch")
				}
			}

			// Clean up
			os.Remove(tt.target)
		})
	}
}

func TestDownloadPart(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockGCSClient()

	// Create test data
	testData := []byte("This is test data for part download")

	// Pre-populate test object
	bucket := mockClient.Bucket("test-bucket").(*mockBucket)
	bucket.objects["test-object"] = &mockObject{
		data:        testData,
		contentType: "text/plain",
	}

	s := &GCSStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	tests := []struct {
		name        string
		part        PrepareDownloadPart
		expectError bool
		expectData  []byte
	}{
		{
			name: "Download full object",
			part: PrepareDownloadPart{
				PartNum:   1,
				Bucket:    "test-bucket",
				Object:    "test-object",
				Offset:    0,
				Size:      int64(len(testData)),
				ByteRange: "",
			},
			expectError: false,
			expectData:  testData,
		},
		{
			name: "Download partial object",
			part: PrepareDownloadPart{
				PartNum:   2,
				Bucket:    "test-bucket",
				Object:    "test-object",
				Offset:    5,
				Size:      10,
				ByteRange: "",
			},
			expectError: false,
			expectData:  testData[5:15],
		},
		{
			name: "Download non-existent object",
			part: PrepareDownloadPart{
				PartNum:   3,
				Bucket:    "test-bucket",
				Object:    "non-existent",
				Offset:    0,
				Size:      10,
				ByteRange: "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temp file for testing
			tempFile := fmt.Sprintf("/tmp/test-part-%d", tt.part.PartNum)
			result := s.downloadPartToFile(ctx, tt.part, tempFile)

			if tt.expectError && result.Err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && result.Err != nil {
				t.Errorf("Unexpected error: %v", result.Err)
			}

			// Verify downloaded data
			if !tt.expectError {
				data, err := os.ReadFile(tempFile)
				if err != nil {
					t.Fatalf("Failed to read downloaded file: %v", err)
				}
				if !bytes.Equal(data, tt.expectData) {
					t.Errorf("Downloaded data mismatch: expected %q, got %q", tt.expectData, data)
				}
			}

			// Clean up
			os.Remove(tempFile)
		})
	}
}

func TestCombineParts(t *testing.T) {
	// Create temporary part files
	part1Data := []byte("Part 1 data")
	part2Data := []byte("Part 2 data")
	part3Data := []byte("Part 3 data")

	part1File := "/tmp/test-combine-part-1"
	part2File := "/tmp/test-combine-part-2"
	part3File := "/tmp/test-combine-part-3"
	targetFile := "/tmp/test-combine-target"

	// Write part files
	if err := os.WriteFile(part1File, part1Data, 0644); err != nil {
		t.Fatalf("Failed to write part 1: %v", err)
	}
	if err := os.WriteFile(part2File, part2Data, 0644); err != nil {
		t.Fatalf("Failed to write part 2: %v", err)
	}
	if err := os.WriteFile(part3File, part3Data, 0644); err != nil {
		t.Fatalf("Failed to write part 3: %v", err)
	}

	// Clean up all files at the end
	defer func() {
		os.Remove(part1File)
		os.Remove(part2File)
		os.Remove(part3File)
		os.Remove(targetFile)
	}()

	s := &GCSStorage{
		logger: logging.NewNopLogger(),
	}

	parts := []DownloadedPart{
		{PartNum: 1, TempFilePath: part1File},
		{PartNum: 2, TempFilePath: part2File},
		{PartNum: 3, TempFilePath: part3File},
	}

	err := s.combineParts(parts, targetFile)
	if err != nil {
		t.Fatalf("Failed to combine parts: %v", err)
	}

	// Verify combined file
	combinedData, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read combined file: %v", err)
	}

	expectedData := append(append(part1Data, part2Data...), part3Data...)
	if !bytes.Equal(combinedData, expectedData) {
		t.Errorf("Combined data mismatch: expected %q, got %q", expectedData, combinedData)
	}
}

func TestCleanupParts(t *testing.T) {
	// Create temporary part files
	part1File := "/tmp/test-cleanup-part-1"
	part2File := "/tmp/test-cleanup-part-2"

	if err := os.WriteFile(part1File, []byte("data"), 0644); err != nil {
		t.Fatalf("Failed to write part 1: %v", err)
	}
	if err := os.WriteFile(part2File, []byte("data"), 0644); err != nil {
		t.Fatalf("Failed to write part 2: %v", err)
	}

	s := &GCSStorage{
		logger: logging.NewNopLogger(),
	}

	parts := []DownloadedPart{
		{PartNum: 1, TempFilePath: part1File},
		{PartNum: 2, TempFilePath: part2File},
		{PartNum: 3, TempFilePath: "/tmp/non-existent-file"}, // This shouldn't cause error
	}

	s.cleanupParts(parts)

	// Verify files are deleted
	if _, err := os.Stat(part1File); !os.IsNotExist(err) {
		t.Error("Part 1 file should have been deleted")
	}
	if _, err := os.Stat(part2File); !os.IsNotExist(err) {
		t.Error("Part 2 file should have been deleted")
	}
}

func TestGetMultipartUpload(t *testing.T) {
	s := &GCSStorage{
		logger: logging.NewNopLogger(),
	}

	// Create a multipart upload
	uploadID := "test-upload-id"
	s.createMultipartUpload(uploadID)

	// Test get existing upload
	retrieved, err := s.getMultipartUpload(uploadID)
	if err != nil {
		t.Errorf("Failed to get existing upload: %v", err)
	}
	if retrieved.UploadID != uploadID {
		t.Errorf("Expected upload ID %s, got %s", uploadID, retrieved.UploadID)
	}

	// Test get non-existent upload
	_, err = s.getMultipartUpload("non-existent-id")
	if err == nil {
		t.Error("Expected error for non-existent upload ID")
	}

	// Test delete upload
	s.deleteMultipartUpload(uploadID)
	_, err = s.getMultipartUpload(uploadID)
	if err == nil {
		t.Error("Expected error after deleting upload")
	}
}

func TestMultipartUploadConcurrency(t *testing.T) {
	s := &GCSStorage{
		logger: logging.NewNopLogger(),
	}

	uploadID := "concurrent-upload-id"
	s.createMultipartUpload(uploadID)

	info, _ := s.getMultipartUpload(uploadID)

	// Simulate concurrent part uploads
	var wg sync.WaitGroup
	numParts := 10

	for i := 1; i <= numParts; i++ {
		wg.Add(1)
		go func(partNum int) {
			defer wg.Done()

			info.mu.Lock()
			info.Parts[partNum] = fmt.Sprintf("temp-object-%d", partNum)
			info.mu.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify all parts were recorded
	if len(info.Parts) != numParts {
		t.Errorf("Expected %d parts, got %d", numParts, len(info.Parts))
	}

	for i := 1; i <= numParts; i++ {
		if _, ok := info.Parts[i]; !ok {
			t.Errorf("Part %d missing", i)
		}
	}
}

// Mock reader that can simulate errors
type errorReader struct {
	data      []byte
	readIndex int
	errorAt   int
}

func (r *errorReader) Read(p []byte) (n int, err error) {
	if r.readIndex >= r.errorAt && r.errorAt >= 0 {
		return 0, errors.New("simulated read error")
	}

	remaining := len(r.data) - r.readIndex
	if remaining == 0 {
		return 0, io.EOF
	}

	n = copy(p, r.data[r.readIndex:])
	r.readIndex += n
	return n, nil
}

func TestDownloadPartWithError(t *testing.T) {
	ctx := context.Background()
	mockClient := newMockGCSClient()

	// Create mock object that will fail
	bucket := mockClient.Bucket("test-bucket").(*mockBucket)
	bucket.objects["error-object"] = &mockObject{
		data: []byte("test data"),
	}

	s := &GCSStorage{
		client: mockClient,
		logger: logging.NewNopLogger(),
		config: DefaultConfig(),
	}

	// Create a part that will cause directory creation to fail
	part := PrepareDownloadPart{
		PartNum:   1,
		Bucket:    "test-bucket",
		Object:    "error-object",
		Offset:    0,
		Size:      10,
		ByteRange: "",
	}

	result := s.downloadPartToFile(ctx, part, "/invalid\x00path/file")
	if result.Err == nil {
		t.Error("Expected error for invalid file path")
	}
}
