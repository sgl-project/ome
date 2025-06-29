package gcp

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"cloud.google.com/go/storage"
	pkgstorage "github.com/sgl-project/ome/pkg/storage"
)

// PrepareDownloadPart represents a part to download
type PrepareDownloadPart struct {
	Bucket    string
	Object    string
	ByteRange string
	Offset    int64
	PartNum   int
	Size      int64
}

// DownloadedPart represents a downloaded part
type DownloadedPart struct {
	PartNum      int
	Offset       int64
	Size         int64
	TempFilePath string
	Err          error
	MD5          string
}

// downloadPartToFile downloads a part directly to a file
func (s *GCSStorage) downloadPartToFile(ctx context.Context, part PrepareDownloadPart, filePath string) DownloadedPart {
	result := DownloadedPart{
		PartNum:      part.PartNum,
		Offset:       part.Offset,
		Size:         part.Size,
		TempFilePath: filePath,
	}

	// Create directory if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.Err = fmt.Errorf("failed to create directory: %w", err)
		return result
	}

	// Get object handle
	bucket := s.client.Bucket(part.Bucket)
	object := bucket.Object(part.Object)

	// Create reader with range
	reader, err := object.NewRangeReader(ctx, part.Offset, part.Size)
	if err != nil {
		result.Err = fmt.Errorf("failed to create range reader: %w", err)
		return result
	}
	defer reader.Close()

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		result.Err = fmt.Errorf("failed to create file: %w", err)
		return result
	}
	defer file.Close()

	// Copy data
	written, err := io.Copy(file, reader)
	if err != nil {
		result.Err = fmt.Errorf("failed to write file: %w", err)
		return result
	}

	if written != part.Size {
		result.Err = fmt.Errorf("size mismatch: expected %d, got %d", part.Size, written)
		return result
	}

	return result
}

// multipartDownload performs a parallel multipart download
func (s *GCSStorage) multipartDownload(ctx context.Context, source pkgstorage.ObjectURI, target string, size int64, opts *pkgstorage.DownloadOptions) error {
	// Calculate parts
	chunkSize := int64(opts.ChunkSizeInMB) * 1024 * 1024
	numParts := (size + chunkSize - 1) / chunkSize

	// Prepare download parts
	parts := make([]PrepareDownloadPart, numParts)
	for i := int64(0); i < numParts; i++ {
		start := i * chunkSize
		end := start + chunkSize - 1
		if end >= size {
			end = size - 1
		}

		parts[i] = PrepareDownloadPart{
			Bucket:    source.BucketName,
			Object:    source.ObjectName,
			ByteRange: fmt.Sprintf("bytes=%d-%d", start, end),
			Offset:    start,
			PartNum:   int(i + 1),
			Size:      end - start + 1,
		}
	}

	// Download parts concurrently
	downloadedParts := make([]DownloadedPart, len(parts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, opts.Threads)

	for i, part := range parts {
		wg.Add(1)
		go func(idx int, p PrepareDownloadPart) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			downloadedParts[idx] = s.downloadPart(ctx, p)
		}(i, part)
	}

	wg.Wait()

	// Check for errors
	for _, dp := range downloadedParts {
		if dp.Err != nil {
			// Clean up temp files
			for _, p := range downloadedParts {
				if p.TempFilePath != "" {
					os.Remove(p.TempFilePath)
				}
			}
			return fmt.Errorf("failed to download part %d: %w", dp.PartNum, dp.Err)
		}
	}

	// Combine parts
	err := s.combineParts(downloadedParts, target)

	// Clean up temp files
	for _, p := range downloadedParts {
		if p.TempFilePath != "" {
			os.Remove(p.TempFilePath)
		}
	}

	if err != nil {
		return err
	}

	// Validate MD5 if requested
	if opts.ValidateMD5 {
		// Get object attributes for MD5
		attrs, err := s.client.Bucket(source.BucketName).Object(source.ObjectName).Attrs(ctx)
		if err != nil {
			return fmt.Errorf("failed to get attributes for MD5 validation: %w", err)
		}

		if attrs.MD5 != nil {
			// GCS returns MD5 as byte array, convert to base64
			expectedMD5 := base64.StdEncoding.EncodeToString(attrs.MD5)

			valid, err := pkgstorage.ValidateFileMD5(target, expectedMD5)
			if err != nil {
				return fmt.Errorf("MD5 validation error: %w", err)
			}
			if !valid {
				os.Remove(target) // Remove invalid file
				return fmt.Errorf("MD5 validation failed for %s", source.ObjectName)
			}
		}
	}

	return nil
}

// downloadPart downloads a single part
func (s *GCSStorage) downloadPart(ctx context.Context, part PrepareDownloadPart) DownloadedPart {
	result := DownloadedPart{
		PartNum: part.PartNum,
		Offset:  part.Offset,
		Size:    part.Size,
	}

	// Create temp file
	tempFile, err := os.CreateTemp("", fmt.Sprintf("gcs-download-part-%d-*", part.PartNum))
	if err != nil {
		result.Err = fmt.Errorf("failed to create temp file: %w", err)
		return result
	}
	result.TempFilePath = tempFile.Name()
	defer tempFile.Close()

	// Get object handle
	bucket := s.client.Bucket(part.Bucket)
	object := bucket.Object(part.Object)

	// Create reader with range
	reader, err := object.NewRangeReader(ctx, part.Offset, part.Size)
	if err != nil {
		result.Err = fmt.Errorf("failed to create range reader: %w", err)
		return result
	}
	defer reader.Close()

	// Create MD5 hasher
	hasher := md5.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	// Copy data
	written, err := io.Copy(multiWriter, reader)
	if err != nil {
		result.Err = fmt.Errorf("failed to write part: %w", err)
		return result
	}

	if written != part.Size {
		result.Err = fmt.Errorf("size mismatch: expected %d, got %d", part.Size, written)
		return result
	}

	result.MD5 = base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	return result
}

// combineParts combines downloaded parts into the final file
func (s *GCSStorage) combineParts(parts []DownloadedPart, targetPath string) error {
	// Create target file
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer targetFile.Close()

	// Copy parts in order
	for _, part := range parts {
		partFile, err := os.Open(part.TempFilePath)
		if err != nil {
			return fmt.Errorf("failed to open part file: %w", err)
		}

		_, err = io.Copy(targetFile, partFile)
		partFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy part %d: %w", part.PartNum, err)
		}
	}

	return nil
}

// isNotFoundError checks if the error is a not found error
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return err == storage.ErrObjectNotExist
}

// getObjectSize gets the size of an object
func (s *GCSStorage) getObjectSize(ctx context.Context, bucket, object string) (int64, error) {
	bucketHandle := s.client.Bucket(bucket)
	objectHandle := bucketHandle.Object(object)

	attrs, err := objectHandle.Attrs(ctx)
	if err != nil {
		return 0, err
	}

	return attrs.Size, nil
}

// MultipartUploadInfo stores information about an ongoing multipart upload
type MultipartUploadInfo struct {
	UploadID string
	Parts    map[int]string // partNumber -> temporary object name
	mu       sync.Mutex
}

// Global storage for multipart uploads (in production, use a persistent store)
var (
	multipartUploads = make(map[string]*MultipartUploadInfo)
	uploadsMu        sync.Mutex
)

// createMultipartUpload creates a new multipart upload
func (s *GCSStorage) createMultipartUpload(uploadID string) *MultipartUploadInfo {
	uploadsMu.Lock()
	defer uploadsMu.Unlock()

	info := &MultipartUploadInfo{
		UploadID: uploadID,
		Parts:    make(map[int]string),
	}
	multipartUploads[uploadID] = info
	return info
}

// getMultipartUpload retrieves multipart upload info
func (s *GCSStorage) getMultipartUpload(uploadID string) (*MultipartUploadInfo, error) {
	uploadsMu.Lock()
	defer uploadsMu.Unlock()

	info, ok := multipartUploads[uploadID]
	if !ok {
		return nil, fmt.Errorf("upload ID not found: %s", uploadID)
	}
	return info, nil
}

// deleteMultipartUpload removes multipart upload info
func (s *GCSStorage) deleteMultipartUpload(uploadID string) {
	uploadsMu.Lock()
	defer uploadsMu.Unlock()

	delete(multipartUploads, uploadID)
}

// cleanupParts removes temporary part files
func (s *GCSStorage) cleanupParts(parts []DownloadedPart) {
	for _, part := range parts {
		if err := os.Remove(part.TempFilePath); err != nil && !os.IsNotExist(err) {
			s.logger.WithError(err).Debug("failed to remove temp file")
		}
	}
}
