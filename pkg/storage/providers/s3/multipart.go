package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sgl-project/ome/pkg/storage"
)

// MultipartUpload represents an active multipart upload
type MultipartUpload struct {
	UploadID string
	Key      string
	Parts    []types.CompletedPart
	mu       sync.Mutex
}

// InitiateMultipartUpload starts a new multipart upload
func (p *S3Provider) InitiateMultipartUpload(ctx context.Context, key string, contentType string, metadata map[string]string) (*MultipartUpload, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(p.bucket),
		Key:    aws.String(key),
	}

	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	if len(metadata) > 0 {
		input.Metadata = ConvertMetadataToS3(metadata)
	}

	result, err := p.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	return &MultipartUpload{
		UploadID: *result.UploadId,
		Key:      key,
		Parts:    make([]types.CompletedPart, 0),
	}, nil
}

// UploadPart uploads a single part in a multipart upload
func (p *S3Provider) UploadPart(ctx context.Context, upload *MultipartUpload, partNumber int32, reader io.Reader, size int64) (*types.CompletedPart, error) {
	// Read the part data
	data := make([]byte, size)
	n, err := io.ReadFull(reader, data)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, fmt.Errorf("failed to read part data: %w", err)
	}
	data = data[:n]

	input := &s3.UploadPartInput{
		Bucket:     aws.String(p.bucket),
		Key:        aws.String(upload.Key),
		UploadId:   aws.String(upload.UploadID),
		PartNumber: aws.Int32(partNumber),
		Body:       bytes.NewReader(data),
	}

	result, err := p.client.UploadPart(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to upload part %d: %w", partNumber, err)
	}

	completedPart := types.CompletedPart{
		ETag:       result.ETag,
		PartNumber: aws.Int32(partNumber),
	}

	// Add to the parts list (thread-safe)
	upload.mu.Lock()
	upload.Parts = append(upload.Parts, completedPart)
	upload.mu.Unlock()

	return &completedPart, nil
}

// CompleteMultipartUpload completes a multipart upload
func (p *S3Provider) CompleteMultipartUpload(ctx context.Context, upload *MultipartUpload) (*s3.CompleteMultipartUploadOutput, error) {
	// Sort parts by part number
	sortCompletedParts(upload.Parts)

	input := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(p.bucket),
		Key:      aws.String(upload.Key),
		UploadId: aws.String(upload.UploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: upload.Parts,
		},
	}

	result, err := p.client.CompleteMultipartUpload(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return result, nil
}

// AbortMultipartUpload cancels a multipart upload
func (p *S3Provider) AbortMultipartUpload(ctx context.Context, upload *MultipartUpload) error {
	input := &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(p.bucket),
		Key:      aws.String(upload.Key),
		UploadId: aws.String(upload.UploadID),
	}

	_, err := p.client.AbortMultipartUpload(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to abort multipart upload: %w", err)
	}

	return nil
}

// ListMultipartUploads lists all active multipart uploads with pagination support
func (p *S3Provider) ListMultipartUploads(ctx context.Context, prefix string) ([]*MultipartUpload, error) {
	var uploads []*MultipartUpload
	var keyMarker *string
	var uploadIDMarker *string

	for {
		input := &s3.ListMultipartUploadsInput{
			Bucket: aws.String(p.bucket),
		}

		if prefix != "" {
			input.Prefix = aws.String(prefix)
		}

		if keyMarker != nil {
			input.KeyMarker = keyMarker
		}

		if uploadIDMarker != nil {
			input.UploadIdMarker = uploadIDMarker
		}

		result, err := p.client.ListMultipartUploads(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list multipart uploads: %w", err)
		}

		for _, upload := range result.Uploads {
			uploads = append(uploads, &MultipartUpload{
				UploadID: *upload.UploadId,
				Key:      *upload.Key,
			})
		}

		if !aws.ToBool(result.IsTruncated) {
			break
		}

		keyMarker = result.NextKeyMarker
		uploadIDMarker = result.NextUploadIdMarker
	}

	return uploads, nil
}

// ListParts lists the parts that have been uploaded for a multipart upload
func (p *S3Provider) ListParts(ctx context.Context, upload *MultipartUpload) ([]types.Part, error) {
	var parts []types.Part
	var nextPartNumberMarker *string

	for {
		input := &s3.ListPartsInput{
			Bucket:   aws.String(p.bucket),
			Key:      aws.String(upload.Key),
			UploadId: aws.String(upload.UploadID),
		}

		if nextPartNumberMarker != nil {
			input.PartNumberMarker = nextPartNumberMarker
		}

		result, err := p.client.ListParts(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list parts: %w", err)
		}

		parts = append(parts, result.Parts...)

		if !aws.ToBool(result.IsTruncated) {
			break
		}

		nextPartNumberMarker = result.NextPartNumberMarker
	}

	return parts, nil
}

// UploadPartCopy copies data from an existing object as a part
func (p *S3Provider) UploadPartCopy(ctx context.Context, upload *MultipartUpload, partNumber int32, sourceKey string, startByte, endByte int64) (*types.CompletedPart, error) {
	copySource := fmt.Sprintf("%s/%s", p.bucket, sourceKey)
	copyRange := fmt.Sprintf("bytes=%d-%d", startByte, endByte)

	input := &s3.UploadPartCopyInput{
		Bucket:          aws.String(p.bucket),
		Key:             aws.String(upload.Key),
		UploadId:        aws.String(upload.UploadID),
		PartNumber:      aws.Int32(partNumber),
		CopySource:      aws.String(copySource),
		CopySourceRange: aws.String(copyRange),
	}

	result, err := p.client.UploadPartCopy(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to upload part copy %d: %w", partNumber, err)
	}

	completedPart := types.CompletedPart{
		ETag:       result.CopyPartResult.ETag,
		PartNumber: aws.Int32(partNumber),
	}

	// Add to the parts list (thread-safe)
	upload.mu.Lock()
	upload.Parts = append(upload.Parts, completedPart)
	upload.mu.Unlock()

	return &completedPart, nil
}

// Helper function to sort completed parts by part number
func sortCompletedParts(parts []types.CompletedPart) {
	// Use sort.Slice for efficient sorting, as multipart uploads can have up to 10,000 parts
	sort.Slice(parts, func(i, j int) bool {
		return *parts[i].PartNumber < *parts[j].PartNumber
	})
}

// UploadLargeFile uploads a large file using multipart upload with automatic part management
// NOTE: This function is currently unused. The S3 provider uses the SDK's manager.Uploader instead.
// WARNING: This implementation has a critical race condition - multiple goroutines read from a shared io.Reader.
// If this function is to be used, it should be refactored to either:
// 1. Use io.ReaderAt for concurrent reads from different offsets
// 2. Read sequentially into buffers first, then upload in parallel
// 3. Protect the reader with a mutex for sequential access
func (p *S3Provider) UploadLargeFile(ctx context.Context, key string, reader io.Reader, size int64, options storage.UploadOptions) error {
	// Calculate part size and count
	partSize := options.PartSize
	if partSize == 0 {
		partSize = defaultPartSize * 1024 * 1024 // 5MB default
	}

	partCount := (size + partSize - 1) / partSize

	// Initiate multipart upload
	upload, err := p.InitiateMultipartUpload(ctx, key, options.ContentType, options.Metadata)
	if err != nil {
		return err
	}

	// Ensure we abort the upload if something goes wrong
	defer func() {
		if err != nil {
			_ = p.AbortMultipartUpload(ctx, upload)
		}
	}()

	// Upload parts concurrently
	concurrency := options.Concurrency
	if concurrency == 0 {
		concurrency = defaultConcurrency
	}

	var wg sync.WaitGroup
	errChan := make(chan error, partCount)
	semaphore := make(chan struct{}, concurrency)

	for i := int64(0); i < partCount; i++ {
		wg.Add(1)
		go func(partNum int64) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Calculate part size
			start := partNum * partSize
			end := start + partSize
			if end > size {
				end = size
			}
			currentPartSize := end - start

			// Read part data
			partData := make([]byte, currentPartSize)
			n, err := io.ReadFull(reader, partData)
			if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
				errChan <- fmt.Errorf("failed to read part %d: %w", partNum+1, err)
				return
			}
			partData = partData[:n]

			// Upload part
			_, err = p.UploadPart(ctx, upload, int32(partNum+1), bytes.NewReader(partData), int64(n))
			if err != nil {
				errChan <- err
				return
			}

			// Report progress
			if options.Progress != nil {
				options.Progress.Update(int64(n), size)
			}
		}(i)
	}

	// Wait for all uploads to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for uploadErr := range errChan {
		if uploadErr != nil {
			err = uploadErr
			return fmt.Errorf("multipart upload failed: %w", err)
		}
	}

	// Complete the multipart upload
	_, err = p.CompleteMultipartUpload(ctx, upload)
	if err != nil {
		return err
	}

	// Report completion
	if options.Progress != nil {
		options.Progress.Done()
	}

	return nil
}
