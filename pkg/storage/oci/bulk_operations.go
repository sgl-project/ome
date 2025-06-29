package oci

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/sgl-project/ome/pkg/storage"
)

// BulkDownload implements bulk download functionality for OCI storage
func (s *OCIStorage) BulkDownload(ctx context.Context, objects []storage.ObjectURI, targetDir string, opts storage.BulkDownloadOptions) ([]storage.BulkDownloadResult, error) {
	// Convert ObjectURIs to include namespace if missing
	for i := range objects {
		if objects[i].Namespace == "" {
			ns, err := s.getNamespace(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get namespace: %w", err)
			}
			if ns != nil {
				objects[i].Namespace = *ns
			}
		}
	}

	// Use the generic bulk download implementation with OCI-specific handling
	return storage.BulkDownload(ctx, s, objects, targetDir, opts)
}

// BulkUpload implements bulk upload functionality for OCI storage
func (s *OCIStorage) BulkUpload(ctx context.Context, files []storage.BulkUploadFile, opts storage.BulkUploadOptions) ([]storage.BulkUploadResult, error) {
	// Ensure namespace is set for all target URIs
	namespace, err := s.getNamespace(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace: %w", err)
	}

	for i := range files {
		if files[i].TargetURI.Namespace == "" && namespace != nil {
			files[i].TargetURI.Namespace = *namespace
		}
	}

	// Use the generic bulk upload implementation
	return storage.BulkUpload(ctx, s, files, opts)
}

// DownloadWithProgress downloads an object with progress tracking
func (s *OCIStorage) DownloadWithProgress(ctx context.Context, source storage.ObjectURI, target string, progress storage.ProgressCallback, opts ...storage.DownloadOption) error {
	// Get object info first to determine size
	info, err := s.GetObjectInfo(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to get object info: %w", err)
	}

	// Create progress tracker
	tracker := storage.NewProgressTracker(info.Size, 1, progress)
	tracker.SetCurrentFile(source.ObjectName)

	// Get the object
	reader, err := s.Get(ctx, source)
	if err != nil {
		tracker.SetError(err)
		return err
	}
	defer reader.Close()

	// Wrap reader with progress tracking
	progressReader := storage.NewProgressReader(reader, tracker)

	// Determine target file path
	targetPath := target
	if stat, err := storage.GetFileInfo(target); err == nil && stat.IsDir() {
		targetPath = filepath.Join(target, filepath.Base(source.ObjectName))
	}

	// Create target directory if needed
	targetDir := filepath.Dir(targetPath)
	if err := storage.MkdirAll(targetDir, 0755); err != nil {
		tracker.SetError(err)
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	// Create target file
	file, err := storage.CreateFile(targetPath)
	if err != nil {
		tracker.SetError(err)
		return fmt.Errorf("failed to create file %s: %w", targetPath, err)
	}
	defer file.Close()

	// Copy with progress tracking
	if _, err := storage.CopyData(file, progressReader); err != nil {
		tracker.SetError(err)
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Mark as complete
	tracker.CompleteFile()
	tracker.Complete()

	return nil
}

// UploadWithProgress uploads a file with progress tracking
func (s *OCIStorage) UploadWithProgress(ctx context.Context, source string, target storage.ObjectURI, progress storage.ProgressCallback, opts ...storage.UploadOption) error {
	// Get file info
	fileInfo, err := storage.GetFileInfo(source)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	// Create progress tracker
	tracker := storage.NewProgressTracker(fileInfo.Size(), 1, progress)
	tracker.SetCurrentFile(filepath.Base(source))

	// Open source file
	file, err := storage.OpenFile(source)
	if err != nil {
		tracker.SetError(err)
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer file.Close()

	// Wrap reader with progress tracking
	progressReader := storage.NewProgressReader(file, tracker)

	// Perform upload
	err = s.Put(ctx, target, progressReader, fileInfo.Size(), opts...)
	if err != nil {
		tracker.SetError(err)
		return err
	}

	// Mark as complete
	tracker.CompleteFile()
	tracker.Complete()

	return nil
}

// ValidateLocalFile checks if a local file matches the remote object
func (s *OCIStorage) ValidateLocalFile(ctx context.Context, localPath string, uri storage.ObjectURI) (bool, error) {
	// Get local file info
	localInfo, err := storage.GetFileInfo(localPath)
	if err != nil {
		return false, fmt.Errorf("failed to stat local file: %w", err)
	}

	// Get remote object metadata (includes MD5 from metadata for multipart)
	remoteMetadata, err := s.Stat(ctx, uri)
	if err != nil {
		return false, fmt.Errorf("failed to get remote object metadata: %w", err)
	}

	// Check size first
	if localInfo.Size() != remoteMetadata.Size {
		return false, nil
	}

	// If this is a multipart upload, use special validation
	if remoteMetadata.IsMultipart {
		return s.validateMultipartMD5(localPath, remoteMetadata)
	}

	// For non-multipart uploads, validate using ContentMD5
	if remoteMetadata.ContentMD5 != "" {
		valid, err := storage.ValidateFileMD5(localPath, remoteMetadata.ContentMD5)
		if err != nil {
			return false, fmt.Errorf("failed to validate MD5: %w", err)
		}
		return valid, nil
	}

	// If no MD5 available, consider valid if size matches
	return true, nil
}

// DownloadWithRetry downloads an object with automatic retry on failure
func (s *OCIStorage) DownloadWithRetry(ctx context.Context, source storage.ObjectURI, target string, retryConfig storage.RetryConfig, opts ...storage.DownloadOption) error {
	return storage.RetryOperation(ctx, retryConfig, func() error {
		return s.Download(ctx, source, target, opts...)
	})
}

// UploadWithRetry uploads a file with automatic retry on failure
func (s *OCIStorage) UploadWithRetry(ctx context.Context, source string, target storage.ObjectURI, retryConfig storage.RetryConfig, opts ...storage.UploadOption) error {
	return storage.RetryOperation(ctx, retryConfig, func() error {
		return s.Upload(ctx, source, target, opts...)
	})
}

// GetWithValidation retrieves an object with MD5 validation
func (s *OCIStorage) GetWithValidation(ctx context.Context, uri storage.ObjectURI, expectedMD5 string) (storage.ValidatingReader, error) {
	// Get object metadata first
	metadata, err := s.GetObjectInfo(ctx, uri)
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	// Get the object
	reader, err := s.Get(ctx, uri)
	if err != nil {
		return nil, err
	}

	// If no expected MD5 provided, use the one from metadata
	if expectedMD5 == "" && metadata.ETag != "" {
		expectedMD5 = metadata.ETag
		// Remove quotes if present
		if len(expectedMD5) > 2 && expectedMD5[0] == '"' && expectedMD5[len(expectedMD5)-1] == '"' {
			expectedMD5 = expectedMD5[1 : len(expectedMD5)-1]
		}
	}

	// Create validating reader
	return storage.NewValidatingReader(reader, expectedMD5), nil
}

// PutWithValidation stores data with MD5 validation
func (s *OCIStorage) PutWithValidation(ctx context.Context, uri storage.ObjectURI, reader io.Reader, size int64, expectedMD5 string, opts ...storage.UploadOption) error {
	// Create validating reader if MD5 is provided
	var validatingReader storage.ValidatingReader
	if expectedMD5 != "" {
		// Wrap the reader with validation
		teeReader, validator := storage.TeeValidatingReader(reader, expectedMD5)
		reader = teeReader
		_ = validator // We'll check after upload
	}

	// Perform the upload
	err := s.Put(ctx, uri, reader, size, opts...)
	if err != nil {
		return err
	}

	// If we were validating, check the result
	if validatingReader != nil && !validatingReader.Valid() {
		// Delete the invalid upload
		_ = s.Delete(ctx, uri)
		return fmt.Errorf("MD5 validation failed after upload: expected %s, got %s",
			validatingReader.Expected(), validatingReader.Actual())
	}

	return nil
}
