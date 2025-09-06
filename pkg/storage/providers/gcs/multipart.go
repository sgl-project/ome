package gcs

import (
	"context"
	"fmt"
	"io"
	"sort"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/google/uuid"
	storageTypes "github.com/sgl-project/ome/pkg/storage"
)

// Ensure Provider implements MultipartCapable
var _ storageTypes.MultipartCapable = (*Provider)(nil)

// compositeUpload represents a GCS composite upload operation
type compositeUpload struct {
	bucket      string
	finalObject string
	parts       []compositePart
	mutex       sync.Mutex
}

// compositePart represents a part of a composite upload
type compositePart struct {
	partNumber int
	objectName string
	etag       string
	size       int64
}

// InitiateMultipartUpload starts a new composite upload
// GCS doesn't have true multipart uploads like S3, but we can simulate with composite objects
func (p *Provider) InitiateMultipartUpload(ctx context.Context, uri string, opts ...storageTypes.UploadOption) (string, error) {
	// options := storageTypes.BuildUploadOptions(opts...)  // Currently unused

	bucket, objectName, err := parseGCSURI(uri)
	if err != nil {
		return "", storageTypes.NewError("initiate_multipart", uri, "gcs", err)
	}

	// Generate a unique upload ID using UUID
	uploadID := uuid.New().String()

	// Store the upload information
	upload := &compositeUpload{
		bucket:      bucket,
		finalObject: objectName,
		parts:       make([]compositePart, 0),
	}

	p.activeUploadsLock.Lock()
	p.activeUploads[uploadID] = upload
	p.activeUploadsLock.Unlock()

	p.logger.WithField("uploadID", uploadID).
		WithField("bucket", bucket).
		WithField("object", objectName).
		Debug("Initiated composite upload")

	return uploadID, nil
}

// UploadPart uploads a single part of a multipart upload
func (p *Provider) UploadPart(ctx context.Context, uri string, uploadID string, partNumber int, data io.Reader, _ int64) (string, error) {
	p.activeUploadsLock.RLock()
	upload, exists := p.activeUploads[uploadID]
	p.activeUploadsLock.RUnlock()

	if !exists {
		return "", storageTypes.NewError("upload_part", uploadID, "gcs", fmt.Errorf("upload ID not found"))
	}

	// Create a temporary object name for this part
	partObjectName := fmt.Sprintf("%s.part%d", upload.finalObject, partNumber)

	// Upload the part as a separate object
	obj := p.client.Bucket(upload.bucket).Object(partObjectName)
	writer := obj.NewWriter(ctx)

	size, err := io.Copy(writer, data)
	if err != nil {
		writer.Close()
		return "", storageTypes.NewError("upload_part", uploadID, "gcs", err)
	}

	if err := writer.Close(); err != nil {
		return "", storageTypes.NewError("upload_part", uploadID, "gcs", err)
	}

	// Get the object attributes for the ETag
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return "", storageTypes.NewError("upload_part", uploadID, "gcs", err)
	}

	// Store the part information
	part := compositePart{
		partNumber: partNumber,
		objectName: partObjectName,
		etag:       attrs.Etag,
		size:       size,
	}

	upload.mutex.Lock()
	upload.parts = append(upload.parts, part)
	upload.mutex.Unlock()

	p.logger.WithField("uploadID", uploadID).
		WithField("partNumber", partNumber).
		WithField("size", size).
		WithField("etag", attrs.Etag).
		Debug("Uploaded part")

	return attrs.Etag, nil
}

// CompleteMultipartUpload completes a multipart upload by composing the parts
func (p *Provider) CompleteMultipartUpload(ctx context.Context, uri string, uploadID string, parts []storageTypes.Part) error {
	p.activeUploadsLock.Lock()
	upload, exists := p.activeUploads[uploadID]
	delete(p.activeUploads, uploadID)
	p.activeUploadsLock.Unlock()

	if !exists {
		return storageTypes.NewError("complete_multipart", uploadID, "gcs", fmt.Errorf("upload ID not found"))
	}

	// Sort parts by part number
	sort.Slice(upload.parts, func(i, j int) bool {
		return upload.parts[i].partNumber < upload.parts[j].partNumber
	})

	// Prepare source objects for composition
	var sources []*storage.ObjectHandle
	for _, part := range upload.parts {
		if part.objectName != "" {
			sources = append(sources, p.client.Bucket(upload.bucket).Object(part.objectName))
		}
	}

	if len(sources) == 0 {
		return storageTypes.NewError("complete_multipart", uploadID, "gcs", fmt.Errorf("no parts to compose"))
	}

	// GCS has a limit of 32 components per compose operation
	finalObject := p.client.Bucket(upload.bucket).Object(upload.finalObject)

	if len(sources) <= 32 {
		// Single compose operation
		composer := finalObject.ComposerFrom(sources...)
		if _, err := composer.Run(ctx); err != nil {
			return storageTypes.NewError("complete_multipart", uploadID, "gcs", err)
		}
	} else {
		// Need to do multiple compose operations
		if err := p.composeInBatches(ctx, upload.bucket, upload.finalObject, sources); err != nil {
			return storageTypes.NewError("complete_multipart", uploadID, "gcs", err)
		}
	}

	// Clean up part objects
	for _, part := range upload.parts {
		if part.objectName != "" {
			if err := p.client.Bucket(upload.bucket).Object(part.objectName).Delete(ctx); err != nil {
				p.logger.WithError(err).WithField("part", part.objectName).Warn("Failed to delete part object")
			}
		}
	}

	p.logger.WithField("uploadID", uploadID).Debug("Completed composite upload")
	return nil
}

// AbortMultipartUpload cancels a multipart upload and cleans up parts
func (p *Provider) AbortMultipartUpload(ctx context.Context, uri string, uploadID string) error {
	p.activeUploadsLock.Lock()
	upload, exists := p.activeUploads[uploadID]
	delete(p.activeUploads, uploadID)
	p.activeUploadsLock.Unlock()

	if !exists {
		return storageTypes.NewError("abort_multipart", uploadID, "gcs", fmt.Errorf("upload ID not found"))
	}

	// Clean up all part objects
	for _, part := range upload.parts {
		if part.objectName != "" {
			if err := p.client.Bucket(upload.bucket).Object(part.objectName).Delete(ctx); err != nil {
				p.logger.WithError(err).WithField("part", part.objectName).Warn("Failed to delete part object during abort")
			}
		}
	}

	p.logger.WithField("uploadID", uploadID).Debug("Aborted composite upload")
	return nil
}

// ListParts lists the parts that have been uploaded for a multipart upload
func (p *Provider) ListParts(ctx context.Context, uploadID string) ([]storageTypes.Part, error) {
	p.activeUploadsLock.RLock()
	upload, exists := p.activeUploads[uploadID]
	p.activeUploadsLock.RUnlock()

	if !exists {
		return nil, storageTypes.NewError("list_parts", uploadID, "gcs", fmt.Errorf("upload ID not found"))
	}

	upload.mutex.Lock()
	defer upload.mutex.Unlock()

	parts := make([]storageTypes.Part, 0, len(upload.parts))
	for _, part := range upload.parts {
		parts = append(parts, storageTypes.Part{
			PartNumber: part.partNumber,
			ETag:       part.etag,
			Size:       part.size,
		})
	}

	return parts, nil
}

// composeInBatches handles composition when there are more than 32 parts
func (p *Provider) composeInBatches(ctx context.Context, bucket, finalObject string, sources []*storage.ObjectHandle) error {
	// Compose in batches of 32
	batchSize := 32
	tempObjects := make([]*storage.ObjectHandle, 0)

	for i := 0; i < len(sources); i += batchSize {
		end := i + batchSize
		if end > len(sources) {
			end = len(sources)
		}

		batch := sources[i:end]

		// Create temporary object for this batch
		tempName := fmt.Sprintf("%s.temp%d", finalObject, i/batchSize)
		tempObj := p.client.Bucket(bucket).Object(tempName)

		composer := tempObj.ComposerFrom(batch...)
		if _, err := composer.Run(ctx); err != nil {
			// Clean up any temporary objects we created
			for _, temp := range tempObjects {
				temp.Delete(ctx)
			}
			return fmt.Errorf("failed to compose batch: %w", err)
		}

		tempObjects = append(tempObjects, tempObj)
	}

	// Now compose all temporary objects into the final object
	finalObj := p.client.Bucket(bucket).Object(finalObject)
	composer := finalObj.ComposerFrom(tempObjects...)
	if _, err := composer.Run(ctx); err != nil {
		return fmt.Errorf("failed to compose final object: %w", err)
	}

	// Clean up temporary objects
	for _, temp := range tempObjects {
		if err := temp.Delete(ctx); err != nil {
			p.logger.WithError(err).Warn("Failed to delete temporary compose object")
		}
	}

	return nil
}
