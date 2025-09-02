package storage

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/to/file", "path/to/file"},
		{"path/to/file", "path/to/file"},
		{"path//to///file", "path/to/file"},
		{"/path/to/file/", "path/to/file"},
		{"", "."},
		{"/", "."},
		{"./path", "path"},
		{"../path", "../path"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		parts    []string
		expected string
	}{
		{[]string{"path", "to", "file"}, "path/to/file"},
		{[]string{"/path", "to", "file"}, "path/to/file"},
		{[]string{"path/", "/to/", "/file"}, "path/to/file"},
		{[]string{"", "path", "file"}, "path/file"},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := JoinPath(tt.parts...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		input          string
		expectedBucket string
		expectedKey    string
	}{
		{"bucket/path/to/file", "bucket", "path/to/file"},
		{"bucket", "bucket", ""},
		{"/bucket/path", "bucket", "path"},
		{"", ".", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			bucket, key := SplitPath(tt.input)
			assert.Equal(t, tt.expectedBucket, bucket)
			assert.Equal(t, tt.expectedKey, key)
		})
	}
}

func TestIsDirectory(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"path/to/dir/", true},
		{"path/to/file", false},
		{"/", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsDirectory(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateETag(t *testing.T) {
	data := []byte("test data")
	etag := CalculateETag(data)
	assert.NotEmpty(t, etag)
	assert.Len(t, etag, 32) // MD5 hex string length

	// Same data should produce same ETag
	etag2 := CalculateETag(data)
	assert.Equal(t, etag, etag2)

	// Different data should produce different ETag
	etag3 := CalculateETag([]byte("different data"))
	assert.NotEqual(t, etag, etag3)
}

func TestCalculateSHA256(t *testing.T) {
	data := []byte("test data")
	hash := CalculateSHA256(data)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 64) // SHA256 hex string length

	// Same data should produce same hash
	hash2 := CalculateSHA256(data)
	assert.Equal(t, hash, hash2)

	// Different data should produce different hash
	hash3 := CalculateSHA256([]byte("different data"))
	assert.NotEqual(t, hash, hash3)
}

func TestCopyWithProgress(t *testing.T) {
	t.Run("successful copy", func(t *testing.T) {
		src := bytes.NewReader([]byte("test data for copying"))
		dst := &bytes.Buffer{}

		var progressUpdates []int64
		var doneCalled bool
		progress := NewSimpleProgressReporter(
			func(transferred, total int64) {
				progressUpdates = append(progressUpdates, transferred)
			},
			func() {
				doneCalled = true
			},
			nil,
		)

		n, err := CopyWithProgress(context.Background(), dst, src, 21, progress)
		assert.NoError(t, err)
		assert.Equal(t, int64(21), n)
		assert.Equal(t, "test data for copying", dst.String())
		assert.True(t, doneCalled)
		assert.NotEmpty(t, progressUpdates)
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		src := bytes.NewReader([]byte("test data"))
		dst := &bytes.Buffer{}

		_, err := CopyWithProgress(ctx, dst, src, 9, nil)
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
	})

	t.Run("write error", func(t *testing.T) {
		src := bytes.NewReader([]byte("test"))
		dst := &errorWriter{err: errors.New("write failed")}

		var errorCalled bool
		progress := NewSimpleProgressReporter(
			nil,
			nil,
			func(err error) {
				errorCalled = true
			},
		)

		_, err := CopyWithProgress(context.Background(), dst, src, 4, progress)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "write failed")
		assert.True(t, errorCalled)
	})
}

// errorWriter is a writer that always returns an error
type errorWriter struct {
	err error
}

func (w *errorWriter) Write(p []byte) (int, error) {
	return 0, w.err
}

func TestRetry(t *testing.T) {
	t.Run("successful on first attempt", func(t *testing.T) {
		attempts := 0
		config := DefaultRetryConfig()
		err := Retry(context.Background(), config, func() error {
			attempts++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("successful after retries", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   1 * time.Millisecond,
			MaxDelay:    10 * time.Millisecond,
			Multiplier:  2.0,
		}
		err := Retry(context.Background(), config, func() error {
			attempts++
			if attempts < 3 {
				return NewRetryableError(errors.New("temporary error"))
			}
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 3, attempts)
	})

	t.Run("non-retryable error", func(t *testing.T) {
		attempts := 0
		config := DefaultRetryConfig()
		expectedErr := errors.New("permanent error")
		err := Retry(context.Background(), config, func() error {
			attempts++
			return expectedErr
		})
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, 1, attempts)
	})

	t.Run("max attempts exceeded", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxAttempts: 2,
			BaseDelay:   1 * time.Millisecond,
			MaxDelay:    10 * time.Millisecond,
			Multiplier:  2.0,
		}
		err := Retry(context.Background(), config, func() error {
			attempts++
			return NewRetryableError(errors.New("always fails"))
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "operation failed after 2 attempts")
		assert.Equal(t, 2, attempts)
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		attempts := 0
		config := DefaultRetryConfig()
		err := Retry(ctx, config, func() error {
			attempts++
			return NewRetryableError(errors.New("error"))
		})
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
		assert.Equal(t, 1, attempts)
	})
}

func TestGetStorageTypeFromURI(t *testing.T) {
	tests := []struct {
		uri          string
		expectedType Type
		expectError  bool
	}{
		{
			uri:          "s3://bucket/path/to/object",
			expectedType: TypeS3,
		},
		{
			uri:          "az://container/blob/path",
			expectedType: TypeAzure,
		},
		{
			uri:          "gs://bucket/object/path",
			expectedType: TypeGCS,
		},
		{
			uri:          "oci://namespace/bucket/object",
			expectedType: TypeOCI,
		},
		{
			uri:          "pvc://my-pvc/path/to/file",
			expectedType: TypePVC,
		},
		{
			uri:          "local:///absolute/path",
			expectedType: TypeLocal,
		},
		{
			uri:         "invalid://scheme",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			storageType, err := GetStorageTypeFromURI(tt.uri)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedType, storageType)
			}
		})
	}
}

func TestSimpleProgressReporter(t *testing.T) {
	var updateCalled, doneCalled, errorCalled bool
	var lastTransferred, lastTotal int64
	var lastError error

	reporter := NewSimpleProgressReporter(
		func(transferred, total int64) {
			updateCalled = true
			lastTransferred = transferred
			lastTotal = total
		},
		func() {
			doneCalled = true
		},
		func(err error) {
			errorCalled = true
			lastError = err
		},
	)

	// Test Update
	reporter.Update(100, 1000)
	assert.True(t, updateCalled)
	assert.Equal(t, int64(100), lastTransferred)
	assert.Equal(t, int64(1000), lastTotal)

	// Test Done
	reporter.Done()
	assert.True(t, doneCalled)

	// Test Error
	expectedErr := errors.New("test error")
	reporter.Error(expectedErr)
	assert.True(t, errorCalled)
	assert.Equal(t, expectedErr, lastError)

	// Test with nil callbacks
	nilReporter := NewSimpleProgressReporter(nil, nil, nil)
	assert.NotPanics(t, func() {
		nilReporter.Update(0, 0)
		nilReporter.Done()
		nilReporter.Error(errors.New("error"))
	})
}
