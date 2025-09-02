package storage

import (
	"errors"
	"fmt"
)

// Common storage errors
var (
	// ErrNotFound indicates the requested object was not found
	ErrNotFound = errors.New("storage: object not found")

	// ErrAlreadyExists indicates the object already exists
	ErrAlreadyExists = errors.New("storage: object already exists")

	// ErrAccessDenied indicates access was denied
	ErrAccessDenied = errors.New("storage: access denied")

	// ErrInvalidPath indicates an invalid path was provided
	ErrInvalidPath = errors.New("storage: invalid path")

	// ErrInvalidConfig indicates invalid configuration
	ErrInvalidConfig = errors.New("storage: invalid configuration")

	// ErrNotSupported indicates the operation is not supported
	ErrNotSupported = errors.New("storage: operation not supported")

	// ErrTimeout indicates the operation timed out
	ErrTimeout = errors.New("storage: operation timed out")

	// ErrQuotaExceeded indicates storage quota was exceeded
	ErrQuotaExceeded = errors.New("storage: quota exceeded")

	// ErrChecksumMismatch indicates checksum verification failed
	ErrChecksumMismatch = errors.New("storage: checksum mismatch")

	// ErrPartialContent indicates only partial content was retrieved
	ErrPartialContent = errors.New("storage: partial content")
)

// Error represents a storage error with additional context
type Error struct {
	Op       string // Operation that failed
	Path     string // Path involved in the operation
	Provider string // Storage provider type
	Err      error  // Underlying error
}

// Error returns the string representation of the error
func (e *Error) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("storage %s: %s failed for %s: %v", e.Provider, e.Op, e.Path, e.Err)
	}
	return fmt.Sprintf("storage %s: %s failed: %v", e.Provider, e.Op, e.Err)
}

// Unwrap returns the underlying error
func (e *Error) Unwrap() error {
	return e.Err
}

// Is checks if the error matches the target error
func (e *Error) Is(target error) bool {
	return errors.Is(e.Err, target)
}

// NewError creates a new storage error
func NewError(op string, path string, provider string, err error) error {
	return &Error{
		Op:       op,
		Path:     path,
		Provider: provider,
		Err:      err,
	}
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if an error is an already exists error
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsAccessDenied checks if an error is an access denied error
func IsAccessDenied(err error) bool {
	return errors.Is(err, ErrAccessDenied)
}

// IsInvalidPath checks if an error is an invalid path error
func IsInvalidPath(err error) bool {
	return errors.Is(err, ErrInvalidPath)
}

// IsNotSupported checks if an error is a not supported error
func IsNotSupported(err error) bool {
	return errors.Is(err, ErrNotSupported)
}

// IsTimeout checks if an error is a timeout error
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// IsChecksumMismatch checks if an error is a checksum mismatch error
func IsChecksumMismatch(err error) bool {
	return errors.Is(err, ErrChecksumMismatch)
}

// RetryableError wraps an error to indicate it can be retried
type RetryableError struct {
	Err error
}

// Error returns the string representation of the retryable error
func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %v", e.Err)
}

// Unwrap returns the underlying error
func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var retryable *RetryableError
	return errors.As(err, &retryable)
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error) error {
	return &RetryableError{Err: err}
}
