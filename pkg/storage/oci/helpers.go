package oci

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	authoci "github.com/sgl-project/ome/pkg/auth/oci"
	"github.com/sgl-project/ome/pkg/storage"
)

// getNamespace retrieves the OCI namespace
func (s *OCIStorage) getNamespace(ctx context.Context) (*string, error) {
	// Check if we have compartment ID
	compartmentID := s.compartmentID
	if compartmentID == "" {
		// Try to get tenancy from credentials
		if tenancy, err := s.getTenancy(); err == nil && tenancy != "" {
			compartmentID = tenancy
		}
	}

	if compartmentID == "" {
		return nil, fmt.Errorf("compartment ID not specified and could not determine from credentials")
	}

	req := objectstorage.GetNamespaceRequest{
		CompartmentId: &compartmentID,
	}

	resp, err := s.client.GetNamespace(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp.Value, nil
}

// getTenancy tries to get tenancy from credentials
func (s *OCIStorage) getTenancy() (string, error) {
	// Try to get OCI credentials
	ociCreds, ok := s.credentials.(*authoci.OCICredentials)
	if !ok {
		return "", fmt.Errorf("not OCI credentials")
	}

	configProvider := ociCreds.GetConfigurationProvider()
	if configProvider == nil {
		return "", fmt.Errorf("no configuration provider")
	}

	return configProvider.TenancyOCID()
}

// writeToFile writes data from reader to a file
func writeToFile(path string, reader io.Reader) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// openFile opens a file for reading
func openFile(path string) (*os.File, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}

// multipartDownload performs a multipart download
func (s *OCIStorage) multipartDownload(ctx context.Context, source storage.ObjectURI, target string, size int64, opts *storage.DownloadOptions) error {
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
			Namespace: source.Namespace,
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

	return err
}

// downloadPart downloads a single part
func (s *OCIStorage) downloadPart(ctx context.Context, part PrepareDownloadPart) DownloadedPart {
	result := DownloadedPart{
		PartNum: part.PartNum,
		Offset:  part.Offset,
		Size:    part.Size,
	}

	// Create temp file
	tempFile, err := os.CreateTemp("", fmt.Sprintf("oci-download-part-%d-*", part.PartNum))
	if err != nil {
		result.Err = fmt.Errorf("failed to create temp file: %w", err)
		return result
	}
	result.TempFilePath = tempFile.Name()
	defer tempFile.Close()

	// Download part
	req := objectstorage.GetObjectRequest{
		NamespaceName: &part.Namespace,
		BucketName:    &part.Bucket,
		ObjectName:    &part.Object,
		Range:         &part.ByteRange,
	}

	resp, err := s.client.GetObject(ctx, req)
	if err != nil {
		result.Err = fmt.Errorf("failed to get object part: %w", err)
		return result
	}
	defer resp.Content.Close()

	// Write to temp file
	written, err := io.Copy(tempFile, resp.Content)
	if err != nil {
		result.Err = fmt.Errorf("failed to write part: %w", err)
		return result
	}

	if written != part.Size {
		result.Err = fmt.Errorf("size mismatch: expected %d, got %d", part.Size, written)
		return result
	}

	return result
}

// combineParts combines downloaded parts into final file
func (s *OCIStorage) combineParts(parts []DownloadedPart, target string) error {
	// Create directory if needed
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create target file
	targetFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer targetFile.Close()

	// Combine parts in order
	for _, part := range parts {
		partFile, err := os.Open(part.TempFilePath)
		if err != nil {
			return fmt.Errorf("failed to open part file: %w", err)
		}

		_, err = io.Copy(targetFile, partFile)
		partFile.Close()
		if err != nil {
			return fmt.Errorf("failed to copy part: %w", err)
		}
	}

	return nil
}

// isNotFoundError checks if error is a not found error
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check for OCI service error
	if serviceErr, ok := err.(common.ServiceError); ok {
		return serviceErr.GetHTTPStatusCode() == 404
	}

	// Check error message
	return strings.Contains(err.Error(), "NotFound") ||
		strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "404")
}
