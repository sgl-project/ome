package azure

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/sgl-project/ome/pkg/storage"
)

// PrepareDownloadPart represents a part to download
type PrepareDownloadPart struct {
	Container string
	Blob      string
	Offset    int64
	Count     int64
	PartNum   int
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

// multipartDownload performs a parallel multipart download with validation
func (s *AzureStorage) multipartDownload(ctx context.Context, source storage.ObjectURI, target string, size int64, opts *storage.DownloadOptions) error {
	// Calculate parts
	chunkSize := int64(opts.ChunkSizeInMB) * 1024 * 1024
	numParts := (size + chunkSize - 1) / chunkSize

	// Prepare download parts
	parts := make([]PrepareDownloadPart, numParts)
	for i := int64(0); i < numParts; i++ {
		start := i * chunkSize
		count := chunkSize
		if start+count > size {
			count = size - start
		}

		parts[i] = PrepareDownloadPart{
			Container: source.BucketName,
			Blob:      source.ObjectName,
			Offset:    start,
			Count:     count,
			PartNum:   int(i + 1),
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
func (s *AzureStorage) downloadPart(ctx context.Context, part PrepareDownloadPart) DownloadedPart {
	result := DownloadedPart{
		PartNum: part.PartNum,
		Offset:  part.Offset,
		Size:    part.Count,
	}

	// Create temp file
	tempFile, err := os.CreateTemp("", fmt.Sprintf("azure-download-part-%d-*", part.PartNum))
	if err != nil {
		result.Err = fmt.Errorf("failed to create temp file: %w", err)
		return result
	}
	result.TempFilePath = tempFile.Name()
	defer tempFile.Close()

	// Get blob client
	blobClient := s.client.ServiceClient().NewContainerClient(part.Container).NewBlobClient(part.Blob)

	// Download range
	downloadOptions := &blob.DownloadStreamOptions{
		Range: blob.HTTPRange{
			Offset: part.Offset,
			Count:  part.Count,
		},
	}

	resp, err := blobClient.DownloadStream(ctx, downloadOptions)
	if err != nil {
		result.Err = fmt.Errorf("failed to download stream: %w", err)
		return result
	}
	defer resp.Body.Close()

	// Create MD5 hasher
	hasher := md5.New()
	multiWriter := io.MultiWriter(tempFile, hasher)

	// Copy data
	written, err := io.Copy(multiWriter, resp.Body)
	if err != nil {
		result.Err = fmt.Errorf("failed to write part: %w", err)
		return result
	}

	if written != part.Count {
		result.Err = fmt.Errorf("size mismatch: expected %d, got %d", part.Count, written)
		return result
	}

	result.MD5 = base64.StdEncoding.EncodeToString(hasher.Sum(nil))
	return result
}

// combineParts combines downloaded parts into the final file
func (s *AzureStorage) combineParts(parts []DownloadedPart, targetPath string) error {
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

// calculateMD5 calculates MD5 hash of data
func calculateMD5(data []byte) string {
	hash := md5.Sum(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}

// getBlockList gets the list of blocks for a blob
func (s *AzureStorage) getBlockList(ctx context.Context, container, blobName string) ([]string, error) {
	blockBlobClient := s.client.ServiceClient().NewContainerClient(container).NewBlockBlobClient(blobName)

	resp, err := blockBlobClient.GetBlockList(ctx, blockblob.BlockListTypeCommitted, nil)
	if err != nil {
		return nil, err
	}

	var blockIDs []string
	for _, block := range resp.BlockList.CommittedBlocks {
		if block.Name != nil {
			blockIDs = append(blockIDs, *block.Name)
		}
	}

	return blockIDs, nil
}
