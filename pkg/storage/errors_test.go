package storage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError(t *testing.T) {
	t.Run("error with path", func(t *testing.T) {
		err := NewError("upload", "/path/to/file", "s3", ErrAccessDenied)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage s3")
		assert.Contains(t, err.Error(), "upload failed")
		assert.Contains(t, err.Error(), "/path/to/file")
		assert.Contains(t, err.Error(), "access denied")
	})

	t.Run("error without path", func(t *testing.T) {
		err := NewError("list", "", "azure", ErrTimeout)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "storage azure")
		assert.Contains(t, err.Error(), "list failed")
		assert.Contains(t, err.Error(), "operation timed out")
		assert.NotContains(t, err.Error(), "for ")
	})

	t.Run("error unwrap", func(t *testing.T) {
		baseErr := errors.New("base error")
		err := &Error{
			Op:       "test",
			Path:     "/test",
			Provider: "gcs",
			Err:      baseErr,
		}
		assert.Equal(t, baseErr, err.Unwrap())
	})

	t.Run("error is", func(t *testing.T) {
		err := NewError("delete", "/file", "oci", ErrNotFound)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.False(t, errors.Is(err, ErrAlreadyExists))
	})
}

func TestErrorCheckers(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		checker func(error) bool
		want    bool
	}{
		{
			name:    "IsNotFound with ErrNotFound",
			err:     ErrNotFound,
			checker: IsNotFound,
			want:    true,
		},
		{
			name:    "IsNotFound with wrapped ErrNotFound",
			err:     NewError("get", "/file", "s3", ErrNotFound),
			checker: IsNotFound,
			want:    true,
		},
		{
			name:    "IsNotFound with different error",
			err:     ErrAccessDenied,
			checker: IsNotFound,
			want:    false,
		},
		{
			name:    "IsAlreadyExists",
			err:     NewError("create", "/file", "azure", ErrAlreadyExists),
			checker: IsAlreadyExists,
			want:    true,
		},
		{
			name:    "IsAccessDenied",
			err:     NewError("upload", "/file", "gcs", ErrAccessDenied),
			checker: IsAccessDenied,
			want:    true,
		},
		{
			name:    "IsInvalidPath",
			err:     ErrInvalidPath,
			checker: IsInvalidPath,
			want:    true,
		},
		{
			name:    "IsNotSupported",
			err:     ErrNotSupported,
			checker: IsNotSupported,
			want:    true,
		},
		{
			name:    "IsTimeout",
			err:     ErrTimeout,
			checker: IsTimeout,
			want:    true,
		},
		{
			name:    "IsChecksumMismatch",
			err:     ErrChecksumMismatch,
			checker: IsChecksumMismatch,
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.checker(tt.err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestRetryableError(t *testing.T) {
	t.Run("create and check retryable", func(t *testing.T) {
		baseErr := errors.New("temporary error")
		err := NewRetryableError(baseErr)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "retryable error")
		assert.Contains(t, err.Error(), "temporary error")
		assert.True(t, IsRetryable(err))
	})

	t.Run("non-retryable error", func(t *testing.T) {
		err := errors.New("permanent error")
		assert.False(t, IsRetryable(err))
	})

	t.Run("unwrap retryable error", func(t *testing.T) {
		baseErr := errors.New("base error")
		retryErr := &RetryableError{Err: baseErr}
		assert.Equal(t, baseErr, retryErr.Unwrap())
	})

	t.Run("nested retryable error", func(t *testing.T) {
		baseErr := ErrTimeout
		retryErr := NewRetryableError(baseErr)
		wrappedErr := NewError("operation", "/path", "s3", retryErr)

		assert.True(t, IsRetryable(wrappedErr))
		assert.True(t, IsTimeout(baseErr))
	})
}
