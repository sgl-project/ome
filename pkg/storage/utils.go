package storage

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	utilstorage "github.com/sgl-project/ome/pkg/utils/storage"
)

// NormalizePath normalizes a storage path
func NormalizePath(p string) string {
	// Remove leading slash
	p = strings.TrimPrefix(p, "/")
	// Clean the path
	p = path.Clean(p)
	// Remove trailing slash unless it's root
	if p != "/" {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

// JoinPath joins storage path components
func JoinPath(parts ...string) string {
	joined := path.Join(parts...)
	return NormalizePath(joined)
}

// SplitPath splits a storage path into bucket and key
func SplitPath(p string) (bucket, key string) {
	p = NormalizePath(p)
	parts := strings.SplitN(p, "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

// IsDirectory checks if a path represents a directory (ends with /)
func IsDirectory(p string) bool {
	return strings.HasSuffix(p, "/")
}

// CalculateETag calculates an ETag for data
func CalculateETag(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// CalculateSHA256 calculates SHA256 hash for data
func CalculateSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// CopyWithProgress copies from source to destination with progress reporting
func CopyWithProgress(ctx context.Context, dst io.Writer, src io.Reader, size int64, progress ProgressReporter) (int64, error) {
	if progress != nil {
		defer progress.Done()
	}

	buf := make([]byte, 32*1024) // 32KB buffer
	var written int64

	for {
		select {
		case <-ctx.Done():
			if progress != nil {
				progress.Error(ctx.Err())
			}
			return written, ctx.Err()
		default:
		}

		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
				if progress != nil {
					progress.Update(written, size)
				}
			}
			if ew != nil {
				if progress != nil {
					progress.Error(ew)
				}
				return written, ew
			}
			if nr != nw {
				err := io.ErrShortWrite
				if progress != nil {
					progress.Error(err)
				}
				return written, err
			}
		}
		if er != nil {
			if er != io.EOF {
				if progress != nil {
					progress.Error(er)
				}
				return written, er
			}
			break
		}
	}

	return written, nil
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Second,
		MaxDelay:    30 * time.Second,
		Multiplier:  2.0,
	}
}

// Retry executes a function with exponential backoff retry
func Retry(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error
	delay := config.BaseDelay

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return err
		}

		// Don't retry if context is cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Don't sleep after last attempt
		if attempt == config.MaxAttempts {
			break
		}

		// Sleep with exponential backoff
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}

		// Calculate next delay
		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return fmt.Errorf("operation failed after %d attempts: %w", config.MaxAttempts, lastErr)
}

// GetStorageTypeFromURI determines the storage type from a URI using the existing utils package
func GetStorageTypeFromURI(uri string) (Type, error) {
	storageType, err := utilstorage.GetStorageType(uri)
	if err != nil {
		return "", err
	}
	return Type(storageType), nil
}

// SimpleProgressReporter is a simple progress reporter implementation
type SimpleProgressReporter struct {
	onUpdate func(bytesTransferred, totalBytes int64)
	onDone   func()
	onError  func(err error)
}

// NewSimpleProgressReporter creates a new simple progress reporter
func NewSimpleProgressReporter(
	onUpdate func(bytesTransferred, totalBytes int64),
	onDone func(),
	onError func(err error),
) *SimpleProgressReporter {
	return &SimpleProgressReporter{
		onUpdate: onUpdate,
		onDone:   onDone,
		onError:  onError,
	}
}

// Update reports progress update
func (r *SimpleProgressReporter) Update(bytesTransferred, totalBytes int64) {
	if r.onUpdate != nil {
		r.onUpdate(bytesTransferred, totalBytes)
	}
}

// Done reports completion
func (r *SimpleProgressReporter) Done() {
	if r.onDone != nil {
		r.onDone()
	}
}

// Error reports an error
func (r *SimpleProgressReporter) Error(err error) {
	if r.onError != nil {
		r.onError(err)
	}
}
