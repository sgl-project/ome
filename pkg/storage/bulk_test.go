package storage_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/sgl-project/ome/pkg/storage"
)

// mockStorage implements Storage interface for testing
type mockStorage struct {
	objects      map[string][]byte
	downloadFail map[string]bool
	uploadFail   map[string]bool
}

func newMockStorage() *mockStorage {
	return &mockStorage{
		objects:      make(map[string][]byte),
		downloadFail: make(map[string]bool),
		uploadFail:   make(map[string]bool),
	}
}

func (m *mockStorage) Provider() storage.Provider {
	return "mock"
}

func (m *mockStorage) Get(ctx context.Context, uri storage.ObjectURI) (io.ReadCloser, error) {
	key := fmt.Sprintf("%s/%s", uri.BucketName, uri.ObjectName)

	if m.downloadFail[key] {
		return nil, fmt.Errorf("download failed for %s", key)
	}

	data, exists := m.objects[key]
	if !exists {
		return nil, fmt.Errorf("object not found: %s", key)
	}

	return io.NopCloser(strings.NewReader(string(data))), nil
}

func (m *mockStorage) Put(ctx context.Context, uri storage.ObjectURI, reader io.Reader, size int64, opts ...storage.UploadOption) error {
	key := fmt.Sprintf("%s/%s", uri.BucketName, uri.ObjectName)

	if m.uploadFail[key] {
		return fmt.Errorf("upload failed for %s", key)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	m.objects[key] = data
	return nil
}

func (m *mockStorage) Stat(ctx context.Context, uri storage.ObjectURI) (*storage.Metadata, error) {
	key := fmt.Sprintf("%s/%s", uri.BucketName, uri.ObjectName)

	data, exists := m.objects[key]
	if !exists {
		return nil, fmt.Errorf("object not found: %s", key)
	}

	return &storage.Metadata{
		ObjectInfo: storage.ObjectInfo{
			Name: uri.ObjectName,
			Size: int64(len(data)),
		},
	}, nil
}

func (m *mockStorage) Download(ctx context.Context, source storage.ObjectURI, target string, opts ...storage.DownloadOption) error {
	reader, err := m.Get(ctx, source)
	if err != nil {
		return err
	}
	defer reader.Close()

	// For testing, we just read the data
	_, err = io.ReadAll(reader)
	return err
}

func (m *mockStorage) Upload(ctx context.Context, source string, target storage.ObjectURI, opts ...storage.UploadOption) error {
	// Check if upload should fail
	key := fmt.Sprintf("%s/%s", target.BucketName, target.ObjectName)
	if m.uploadFail[key] {
		return fmt.Errorf("upload failed for %s", key)
	}

	// For testing, we just put some dummy data
	m.objects[key] = []byte("test data from " + source)
	return nil
}

func (m *mockStorage) Delete(ctx context.Context, uri storage.ObjectURI) error {
	key := fmt.Sprintf("%s/%s", uri.BucketName, uri.ObjectName)
	delete(m.objects, key)
	return nil
}

func (m *mockStorage) Exists(ctx context.Context, uri storage.ObjectURI) (bool, error) {
	key := fmt.Sprintf("%s/%s", uri.BucketName, uri.ObjectName)
	_, exists := m.objects[key]
	return exists, nil
}

func (m *mockStorage) List(ctx context.Context, uri storage.ObjectURI, opts storage.ListOptions) ([]storage.ObjectInfo, error) {
	var results []storage.ObjectInfo
	prefix := fmt.Sprintf("%s/", uri.BucketName)

	for key, data := range m.objects {
		if strings.HasPrefix(key, prefix) {
			name := strings.TrimPrefix(key, prefix)
			if opts.Prefix == "" || strings.HasPrefix(name, opts.Prefix) {
				results = append(results, storage.ObjectInfo{
					Name: name,
					Size: int64(len(data)),
				})
			}
		}
	}

	return results, nil
}

func (m *mockStorage) GetObjectInfo(ctx context.Context, uri storage.ObjectURI) (*storage.ObjectInfo, error) {
	key := fmt.Sprintf("%s/%s", uri.BucketName, uri.ObjectName)

	data, exists := m.objects[key]
	if !exists {
		return nil, fmt.Errorf("object not found: %s", key)
	}

	return &storage.ObjectInfo{
		Name: uri.ObjectName,
		Size: int64(len(data)),
	}, nil
}

func (m *mockStorage) Copy(ctx context.Context, source, target storage.ObjectURI) error {
	sourceKey := fmt.Sprintf("%s/%s", source.BucketName, source.ObjectName)
	targetKey := fmt.Sprintf("%s/%s", target.BucketName, target.ObjectName)

	data, exists := m.objects[sourceKey]
	if !exists {
		return fmt.Errorf("source object not found: %s", sourceKey)
	}

	m.objects[targetKey] = data
	return nil
}

func TestBulkDownload(t *testing.T) {
	ctx := context.Background()
	mock := newMockStorage()

	// Setup test objects
	objects := []storage.ObjectURI{
		{BucketName: "test-bucket", ObjectName: "file1.txt"},
		{BucketName: "test-bucket", ObjectName: "file2.txt"},
		{BucketName: "test-bucket", ObjectName: "file3.txt"},
	}

	for i, obj := range objects {
		key := fmt.Sprintf("%s/%s", obj.BucketName, obj.ObjectName)
		mock.objects[key] = []byte(fmt.Sprintf("content %d", i))
	}

	// Test successful bulk download
	t.Run("Success", func(t *testing.T) {
		opts := storage.DefaultBulkDownloadOptions()
		opts.Concurrency = 2

		results, err := storage.BulkDownload(ctx, mock, objects, "/tmp/test", opts)
		if err != nil {
			t.Fatalf("BulkDownload failed: %v", err)
		}

		if len(results) != len(objects) {
			t.Errorf("Expected %d results, got %d", len(objects), len(results))
		}

		for _, result := range results {
			if result.Error != nil {
				t.Errorf("Download failed for %s: %v", result.URI.ObjectName, result.Error)
			}
			if result.Size == 0 {
				t.Errorf("Zero size for %s", result.URI.ObjectName)
			}
		}
	})

	// Test with failures
	t.Run("WithFailures", func(t *testing.T) {
		mock.downloadFail["test-bucket/file2.txt"] = true

		opts := storage.DefaultBulkDownloadOptions()
		opts.Concurrency = 2
		opts.RetryConfig.MaxRetries = 1
		opts.RetryConfig.InitialDelay = 1 * time.Millisecond

		results, err := storage.BulkDownload(ctx, mock, objects, "/tmp/test", opts)
		if err != nil {
			t.Fatalf("BulkDownload failed: %v", err)
		}

		// Check that file2.txt failed
		failedCount := 0
		for _, result := range results {
			if result.Error != nil {
				failedCount++
				if result.URI.ObjectName != "file2.txt" {
					t.Errorf("Unexpected failure for %s", result.URI.ObjectName)
				}
			}
		}

		if failedCount != 1 {
			t.Errorf("Expected 1 failure, got %d", failedCount)
		}

		// Clean up
		delete(mock.downloadFail, "test-bucket/file2.txt")
	})

	// Test with progress callback
	t.Run("WithProgress", func(t *testing.T) {
		progressCalls := 0
		opts := storage.DefaultBulkDownloadOptions()
		opts.ProgressCallback = func(completed, total int, current *storage.BulkDownloadResult) {
			progressCalls++
			if completed > total {
				t.Errorf("Completed %d > Total %d", completed, total)
			}
		}

		_, err := storage.BulkDownload(ctx, mock, objects, "/tmp/test", opts)
		if err != nil {
			t.Fatalf("BulkDownload failed: %v", err)
		}

		if progressCalls != len(objects) {
			t.Errorf("Expected %d progress calls, got %d", len(objects), progressCalls)
		}
	})
}

func TestBulkUpload(t *testing.T) {
	// Skip this test for now as it requires actual files
	t.Skip("BulkUpload test requires actual file system operations")

	ctx := context.Background()
	mock := newMockStorage()

	// Setup test files
	files := []storage.BulkUploadFile{
		{
			SourcePath: "/tmp/file1.txt",
			TargetURI:  storage.ObjectURI{BucketName: "test-bucket", ObjectName: "uploaded/file1.txt"},
		},
		{
			SourcePath: "/tmp/file2.txt",
			TargetURI:  storage.ObjectURI{BucketName: "test-bucket", ObjectName: "uploaded/file2.txt"},
		},
	}

	// Test successful bulk upload
	t.Run("Success", func(t *testing.T) {
		opts := storage.DefaultBulkUploadOptions()
		opts.Concurrency = 2

		results, err := storage.BulkUpload(ctx, mock, files, opts)
		if err != nil {
			t.Fatalf("BulkUpload failed: %v", err)
		}

		if len(results) != len(files) {
			t.Errorf("Expected %d results, got %d", len(files), len(results))
		}

		// Verify uploads succeeded
		for _, file := range files {
			key := fmt.Sprintf("%s/%s", file.TargetURI.BucketName, file.TargetURI.ObjectName)
			if _, exists := mock.objects[key]; !exists {
				t.Errorf("Object %s was not uploaded", key)
			}
		}
	})

	// Test with failures
	t.Run("WithFailures", func(t *testing.T) {
		mock.uploadFail["test-bucket/uploaded/file2.txt"] = true

		opts := storage.DefaultBulkUploadOptions()
		opts.RetryConfig.MaxRetries = 1
		opts.RetryConfig.InitialDelay = 1 * time.Millisecond

		results, err := storage.BulkUpload(ctx, mock, files, opts)
		if err != nil {
			t.Fatalf("BulkUpload failed: %v", err)
		}

		// Check that file2.txt failed
		failedCount := 0
		for _, result := range results {
			if result.Error != nil {
				failedCount++
				if result.URI.ObjectName != "uploaded/file2.txt" {
					t.Errorf("Unexpected failure for %s", result.URI.ObjectName)
				}
			}
		}

		if failedCount != 1 {
			t.Errorf("Expected 1 failure, got %d", failedCount)
		}

		// Clean up
		delete(mock.uploadFail, "test-bucket/uploaded/file2.txt")
	})
}

func TestConcurrency(t *testing.T) {
	ctx := context.Background()
	mock := newMockStorage()

	// Create many objects to test concurrency
	numObjects := 20
	objects := make([]storage.ObjectURI, numObjects)
	for i := 0; i < numObjects; i++ {
		obj := storage.ObjectURI{
			BucketName: "test-bucket",
			ObjectName: fmt.Sprintf("file%d.txt", i),
		}
		objects[i] = obj
		key := fmt.Sprintf("%s/%s", obj.BucketName, obj.ObjectName)
		mock.objects[key] = []byte(fmt.Sprintf("content %d", i))
	}

	// Test different concurrency levels
	concurrencyLevels := []int{1, 4, 10}

	for _, concurrency := range concurrencyLevels {
		t.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(t *testing.T) {
			start := time.Now()

			opts := storage.DefaultBulkDownloadOptions()
			opts.Concurrency = concurrency

			results, err := storage.BulkDownload(ctx, mock, objects, "/tmp/test", opts)
			if err != nil {
				t.Fatalf("BulkDownload failed: %v", err)
			}

			duration := time.Since(start)
			t.Logf("Downloaded %d objects with concurrency %d in %v",
				len(results), concurrency, duration)

			// Verify all objects were downloaded
			if len(results) != numObjects {
				t.Errorf("Expected %d results, got %d", numObjects, len(results))
			}

			for _, result := range results {
				if result.Error != nil {
					t.Errorf("Download failed for %s: %v",
						result.URI.ObjectName, result.Error)
				}
			}
		})
	}
}
