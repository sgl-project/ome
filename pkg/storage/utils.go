package storage

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"path/filepath"
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

// ComputeTargetFilePath computes the target file path for a given object key and download options.
// It handles path manipulation based on the options provided.
func ComputeTargetFilePath(objectKey string, targetDir string, opts DownloadOptions) string {
	if opts.UseBaseNameOnly {
		// Use only the base name (file name) of the object
		return filepath.Join(targetDir, ObjectBaseName(objectKey))
	}

	if opts.StripPrefix && opts.PrefixToStrip != "" {
		// Strip the specified prefix from the object key
		stripped := TrimObjectPrefix(objectKey, opts.PrefixToStrip)
		return filepath.Join(targetDir, stripped)
	}

	if opts.JoinWithTailOverlap {
		// Join with tail overlap
		return JoinWithTailOverlap(targetDir, objectKey)
	}

	// Default: join target directory with full object key
	return filepath.Join(targetDir, objectKey)
}

// ObjectBaseName returns only the file name from a given object path.
// For example, given "bucket/folder/file.txt", it returns "file.txt".
// If the input path does not contain "/", the original string is returned.
func ObjectBaseName(objectPath string) string {
	if !strings.Contains(objectPath, "/") {
		return objectPath
	}

	values := strings.Split(objectPath, "/")
	return values[len(values)-1] // Return the last segment (file name)
}

// TrimObjectPrefix removes a given prefix from the object path if it exists.
// For example, given objectPath "bucket/folder/file.txt" and prefix "bucket/",
// it returns "folder/file.txt".
func TrimObjectPrefix(objectPath string, prefix string) string {
	if len(prefix) == 0 {
		return objectPath
	}

	return strings.TrimPrefix(objectPath, prefix)
}

// JoinWithTailOverlap combines directoryPath and objectPath with overlap.
// This function finds the longest overlap between the directory suffix and object prefix,
// then combines them without duplication.
func JoinWithTailOverlap(directoryPath, objectPath string) string {
	// Clean the directory path using OS-specific separators
	cleanDir := filepath.Clean(directoryPath)
	// For object paths, always use forward slashes
	cleanObj := path.Clean(objectPath)

	// Determine if the directory path was absolute
	isAbsolute := filepath.IsAbs(directoryPath)

	// Split paths for comparison - use OS separator for directory, forward slash for object
	dirParts := strings.Split(cleanDir, string(filepath.Separator))
	objParts := strings.Split(cleanObj, "/")

	// Remove empty parts from splitting
	dirParts = removeEmptyStrings(dirParts)
	objParts = removeEmptyStrings(objParts)

	// Find the longest overlap between dirParts suffix and objParts prefix
	for l := minInt(len(dirParts), len(objParts)); l > 0; l-- {
		if slicesEqual(dirParts[len(dirParts)-l:], objParts[:l]) {
			combined := append(dirParts, objParts[l:]...)
			result := filepath.Join(combined...)
			// Preserve absolute path nature
			if isAbsolute && !filepath.IsAbs(result) {
				result = string(filepath.Separator) + result
			}
			return result
		}
	}

	// No overlap found - just combine the paths
	result := filepath.Join(append(dirParts, objParts...)...)
	// Preserve absolute path nature
	if isAbsolute && !filepath.IsAbs(result) {
		result = string(filepath.Separator) + result
	}
	return result
}

// ShouldExclude checks if an object key matches any of the exclude patterns.
// Patterns can use glob syntax (e.g., "*.tmp", "temp/*", "*/backup/*")
func ShouldExclude(objectKey string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, objectKey)
		if err != nil {
			// Invalid pattern, skip it
			continue
		}
		if matched {
			return true
		}

		// Also check against the base name
		matched, err = filepath.Match(pattern, ObjectBaseName(objectKey))
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}

	return false
}

// slicesEqual checks if two slices are equal.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// minInt returns the minimum of two integers.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// removeEmptyStrings removes empty strings from a slice
func removeEmptyStrings(s []string) []string {
	var result []string
	for _, str := range s {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}
