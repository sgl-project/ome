package storage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUploadOptions(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		opts := DefaultUploadOptions()
		assert.Equal(t, "application/octet-stream", opts.ContentType)
		assert.NotNil(t, opts.Metadata)
		assert.Equal(t, 5, opts.Concurrency)
		assert.Equal(t, int64(5*1024*1024), opts.PartSize)
	})

	t.Run("with content type", func(t *testing.T) {
		opts := BuildUploadOptions(WithContentType("text/plain"))
		assert.Equal(t, "text/plain", opts.ContentType)
	})

	t.Run("with metadata", func(t *testing.T) {
		metadata := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}
		opts := BuildUploadOptions(WithMetadata(metadata))
		assert.Equal(t, metadata, opts.Metadata)
	})

	t.Run("with progress", func(t *testing.T) {
		progress := &SimpleProgressReporter{}
		opts := BuildUploadOptions(WithUploadProgress(progress))
		assert.Equal(t, progress, opts.Progress)
	})

	t.Run("with part size", func(t *testing.T) {
		opts := BuildUploadOptions(WithPartSize(10 * 1024 * 1024))
		assert.Equal(t, int64(10*1024*1024), opts.PartSize)
	})

	t.Run("with concurrency", func(t *testing.T) {
		opts := BuildUploadOptions(WithUploadConcurrency(10))
		assert.Equal(t, 10, opts.Concurrency)
	})

	t.Run("with storage class", func(t *testing.T) {
		opts := BuildUploadOptions(WithStorageClass("GLACIER"))
		assert.Equal(t, "GLACIER", opts.StorageClass)
	})

	t.Run("multiple options", func(t *testing.T) {
		metadata := map[string]string{"tag": "test"}
		progress := &SimpleProgressReporter{}
		opts := BuildUploadOptions(
			WithContentType("image/jpeg"),
			WithMetadata(metadata),
			WithUploadProgress(progress),
			WithPartSize(1024),
			WithUploadConcurrency(3),
			WithStorageClass("STANDARD_IA"),
		)
		assert.Equal(t, "image/jpeg", opts.ContentType)
		assert.Equal(t, metadata, opts.Metadata)
		assert.Equal(t, progress, opts.Progress)
		assert.Equal(t, int64(1024), opts.PartSize)
		assert.Equal(t, 3, opts.Concurrency)
		assert.Equal(t, "STANDARD_IA", opts.StorageClass)
	})
}

func TestDownloadOptions(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		opts := DefaultDownloadOptions()
		assert.Nil(t, opts.Range)
		assert.Nil(t, opts.Progress)
		assert.Equal(t, 5, opts.Concurrency)
		assert.Empty(t, opts.VerifyETag)
	})

	t.Run("with range", func(t *testing.T) {
		opts := BuildDownloadOptions(WithRange(100, 200))
		assert.NotNil(t, opts.Range)
		assert.Equal(t, int64(100), opts.Range.Start)
		assert.Equal(t, int64(200), opts.Range.End)
	})

	t.Run("with progress", func(t *testing.T) {
		progress := &SimpleProgressReporter{}
		opts := BuildDownloadOptions(WithDownloadProgress(progress))
		assert.Equal(t, progress, opts.Progress)
	})

	t.Run("with concurrency", func(t *testing.T) {
		opts := BuildDownloadOptions(WithDownloadConcurrency(8))
		assert.Equal(t, 8, opts.Concurrency)
	})

	t.Run("with etag verification", func(t *testing.T) {
		opts := BuildDownloadOptions(WithETagVerification("abc123"))
		assert.Equal(t, "abc123", opts.VerifyETag)
	})

	t.Run("multiple options", func(t *testing.T) {
		progress := &SimpleProgressReporter{}
		opts := BuildDownloadOptions(
			WithRange(0, 1024),
			WithDownloadProgress(progress),
			WithDownloadConcurrency(2),
			WithETagVerification("etag123"),
		)
		assert.NotNil(t, opts.Range)
		assert.Equal(t, int64(0), opts.Range.Start)
		assert.Equal(t, int64(1024), opts.Range.End)
		assert.Equal(t, progress, opts.Progress)
		assert.Equal(t, 2, opts.Concurrency)
		assert.Equal(t, "etag123", opts.VerifyETag)
	})
}

func TestListOptions(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		opts := DefaultListOptions()
		assert.Equal(t, 1000, opts.MaxResults)
		assert.Empty(t, opts.Delimiter)
		assert.False(t, opts.Recursive)
		assert.False(t, opts.IncludeHidden)
		assert.Empty(t, opts.StartAfter)
	})

	t.Run("with max results", func(t *testing.T) {
		opts := BuildListOptions(WithMaxResults(50))
		assert.Equal(t, 50, opts.MaxResults)
	})

	t.Run("with delimiter", func(t *testing.T) {
		opts := BuildListOptions(WithDelimiter("/"))
		assert.Equal(t, "/", opts.Delimiter)
	})

	t.Run("with recursive", func(t *testing.T) {
		opts := BuildListOptions(WithRecursive(true))
		assert.True(t, opts.Recursive)
	})

	t.Run("with include hidden", func(t *testing.T) {
		opts := BuildListOptions(WithIncludeHidden(true))
		assert.True(t, opts.IncludeHidden)
	})

	t.Run("with start after", func(t *testing.T) {
		opts := BuildListOptions(WithStartAfter("marker123"))
		assert.Equal(t, "marker123", opts.StartAfter)
	})

	t.Run("multiple options", func(t *testing.T) {
		opts := BuildListOptions(
			WithMaxResults(100),
			WithDelimiter("/"),
			WithRecursive(true),
			WithIncludeHidden(true),
			WithStartAfter("start-marker"),
		)
		assert.Equal(t, 100, opts.MaxResults)
		assert.Equal(t, "/", opts.Delimiter)
		assert.True(t, opts.Recursive)
		assert.True(t, opts.IncludeHidden)
		assert.Equal(t, "start-marker", opts.StartAfter)
	})
}

func TestBulkOptions(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		opts := DefaultBulkOptions()
		assert.Equal(t, 5, opts.Concurrency)
		assert.True(t, opts.ContinueOnError)
		assert.Equal(t, 3, opts.RetryAttempts)
		assert.NotZero(t, opts.RetryDelay)
	})

	t.Run("with concurrency", func(t *testing.T) {
		opts := BuildBulkOptions(WithBulkConcurrency(10))
		assert.Equal(t, 10, opts.Concurrency)
	})

	t.Run("with progress", func(t *testing.T) {
		progress := &SimpleProgressReporter{}
		opts := BuildBulkOptions(WithBulkProgress(progress))
		assert.Equal(t, progress, opts.Progress)
	})

	t.Run("with continue on error", func(t *testing.T) {
		opts := BuildBulkOptions(WithContinueOnError(false))
		assert.False(t, opts.ContinueOnError)
	})

	t.Run("with retry attempts", func(t *testing.T) {
		opts := BuildBulkOptions(WithRetryAttempts(5))
		assert.Equal(t, 5, opts.RetryAttempts)
	})

	t.Run("with retry delay", func(t *testing.T) {
		delay := 5 * time.Second
		opts := BuildBulkOptions(WithRetryDelay(delay))
		assert.Equal(t, delay, opts.RetryDelay)
	})

	t.Run("multiple options", func(t *testing.T) {
		progress := &SimpleProgressReporter{}
		delay := 2 * time.Second
		opts := BuildBulkOptions(
			WithBulkConcurrency(8),
			WithBulkProgress(progress),
			WithContinueOnError(false),
			WithRetryAttempts(2),
			WithRetryDelay(delay),
		)
		assert.Equal(t, 8, opts.Concurrency)
		assert.Equal(t, progress, opts.Progress)
		assert.False(t, opts.ContinueOnError)
		assert.Equal(t, 2, opts.RetryAttempts)
		assert.Equal(t, delay, opts.RetryDelay)
	})
}

func TestBuildOptionsIdempotent(t *testing.T) {
	// Test that building options doesn't modify the defaults
	defaultUpload := DefaultUploadOptions()
	BuildUploadOptions(WithContentType("custom"))
	assert.Equal(t, "application/octet-stream", defaultUpload.ContentType)

	defaultDownload := DefaultDownloadOptions()
	BuildDownloadOptions(WithDownloadConcurrency(10))
	assert.Equal(t, 5, defaultDownload.Concurrency)

	defaultList := DefaultListOptions()
	BuildListOptions(WithMaxResults(10))
	assert.Equal(t, 1000, defaultList.MaxResults)

	defaultBulk := DefaultBulkOptions()
	BuildBulkOptions(WithBulkConcurrency(20))
	assert.Equal(t, 5, defaultBulk.Concurrency)
}
