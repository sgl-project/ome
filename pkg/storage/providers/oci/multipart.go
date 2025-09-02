package oci

import (
	"context"
	"io"
	"os"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/oracle/oci-go-sdk/v65/objectstorage/transfer"

	"github.com/sgl-project/ome/pkg/storage"
)

// Ensure OCIProvider implements MultipartCapable
var _ storage.MultipartCapable = (*OCIProvider)(nil)

// InitiateMultipartUpload starts a new multipart upload
func (p *OCIProvider) InitiateMultipartUpload(ctx context.Context, uri string, opts ...storage.UploadOption) (string, error) {
	options := storage.BuildUploadOptions(opts...)
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return "", storage.NewError("initiate_multipart", uri, "oci", err)
	}

	metadata := convertMetadataToOCI(options.Metadata)
	details := objectstorage.CreateMultipartUploadDetails{
		Object:      &ociURI.Object,
		ContentType: &options.ContentType,
		Metadata:    metadata,
	}

	if options.StorageClass != "" {
		storageTier := objectstorage.StorageTierEnum(options.StorageClass)
		details.StorageTier = storageTier
	}

	request := objectstorage.CreateMultipartUploadRequest{
		NamespaceName:                &ociURI.Namespace,
		BucketName:                   &ociURI.Bucket,
		CreateMultipartUploadDetails: details,
	}

	response, err := p.client.CreateMultipartUpload(ctx, request)
	if err != nil {
		return "", storage.NewError("initiate_multipart", uri, "oci", err)
	}

	return *response.UploadId, nil
}

// UploadPart uploads a single part in a multipart upload
func (p *OCIProvider) UploadPart(ctx context.Context, uri string, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return "", storage.NewError("upload_part", uri, "oci", err)
	}

	request := objectstorage.UploadPartRequest{
		NamespaceName:  &ociURI.Namespace,
		BucketName:     &ociURI.Bucket,
		ObjectName:     &ociURI.Object,
		UploadId:       &uploadID,
		UploadPartNum:  &partNumber,
		UploadPartBody: io.NopCloser(reader),
		ContentLength:  &size,
	}

	response, err := p.client.UploadPart(ctx, request)
	if err != nil {
		return "", storage.NewError("upload_part", uri, "oci", err)
	}

	return *response.ETag, nil
}

// CompleteMultipartUpload finishes a multipart upload
func (p *OCIProvider) CompleteMultipartUpload(ctx context.Context, uri string, uploadID string, parts []storage.Part) error {
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return storage.NewError("complete_multipart", uri, "oci", err)
	}

	// Convert parts to OCI format
	ociParts := make([]objectstorage.CommitMultipartUploadPartDetails, len(parts))
	for i, part := range parts {
		ociParts[i] = objectstorage.CommitMultipartUploadPartDetails{
			PartNum: &part.PartNumber,
			Etag:    &part.ETag,
		}
	}

	request := objectstorage.CommitMultipartUploadRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
		ObjectName:    &ociURI.Object,
		UploadId:      &uploadID,
		CommitMultipartUploadDetails: objectstorage.CommitMultipartUploadDetails{
			PartsToCommit: ociParts,
		},
	}

	_, err = p.client.CommitMultipartUpload(ctx, request)
	if err != nil {
		return storage.NewError("complete_multipart", uri, "oci", err)
	}

	return nil
}

// AbortMultipartUpload cancels a multipart upload
func (p *OCIProvider) AbortMultipartUpload(ctx context.Context, uri string, uploadID string) error {
	ociURI, err := parseOCIURI(uri, p.namespace, p.bucket)
	if err != nil {
		return storage.NewError("abort_multipart", uri, "oci", err)
	}

	request := objectstorage.AbortMultipartUploadRequest{
		NamespaceName: &ociURI.Namespace,
		BucketName:    &ociURI.Bucket,
		ObjectName:    &ociURI.Object,
		UploadId:      &uploadID,
	}

	_, err = p.client.AbortMultipartUpload(ctx, request)
	if err != nil {
		return storage.NewError("abort_multipart", uri, "oci", err)
	}

	return nil
}

// multipartFileUpload performs a multipart upload from a file
func (p *OCIProvider) multipartFileUpload(ctx context.Context, source string, target *ociURI, size int64, options storage.UploadOptions) error {
	// Open source file
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	// Determine part size
	partSize := options.PartSize
	if partSize == 0 {
		partSize = calculateOptimalPartSize(size)
	}

	// Determine concurrency
	concurrency := options.Concurrency
	if concurrency == 0 {
		concurrency = defaultConcurrency
	}

	// Use OCI SDK's transfer manager for efficient multipart upload
	uploadRequest := transfer.UploadRequest{
		NamespaceName:                       &target.Namespace,
		BucketName:                          &target.Bucket,
		ObjectName:                          &target.Object,
		PartSize:                            common.Int64(partSize),
		NumberOfGoroutines:                  common.Int(concurrency),
		ObjectStorageClient:                 p.client,
		EnableMultipartChecksumVerification: common.Bool(true),
		Metadata:                            convertMetadataToOCI(options.Metadata),
	}

	if options.StorageClass != "" {
		storageTier := objectstorage.PutObjectStorageTierEnum(options.StorageClass)
		uploadRequest.StorageTier = storageTier
	}

	// Set up progress callback if configured
	if options.Progress != nil {
		var totalUploaded int64
		uploadRequest.CallBack = func(multiPartUploadPart transfer.MultiPartUploadPart) {
			if multiPartUploadPart.Err == nil {
				totalUploaded += multiPartUploadPart.Size
				options.Progress.Update(totalUploaded, size)
				p.logger.WithField("part", multiPartUploadPart.PartNum).
					WithField("size", multiPartUploadPart.Size).
					Debug("Uploaded part")
			} else {
				options.Progress.Error(multiPartUploadPart.Err)
			}
		}
	}

	// Create upload manager and perform upload
	uploadManager := transfer.NewUploadManager()
	req := transfer.UploadFileRequest{
		UploadRequest: uploadRequest,
		FilePath:      source,
	}
	response, err := uploadManager.UploadFile(ctx, req)
	if err != nil {
		if options.Progress != nil {
			options.Progress.Error(err)
		}
		return err
	}

	p.logger.WithField("etag", response.SinglepartUploadResponse.ETag).
		WithField("object", target.Object).
		Info("Multipart upload completed")

	return nil
}

// multipartStreamUpload performs a multipart upload from a stream
func (p *OCIProvider) multipartStreamUpload(ctx context.Context, target *ociURI, reader io.Reader, size int64, options storage.UploadOptions) error {
	// Determine part size
	partSize := options.PartSize
	if partSize == 0 {
		partSize = calculateOptimalPartSize(size)
	}

	// Determine concurrency
	concurrency := options.Concurrency
	if concurrency == 0 {
		concurrency = defaultConcurrency
	}

	// Use OCI SDK's transfer manager for efficient multipart upload
	uploadRequest := transfer.UploadRequest{
		NamespaceName:                       &target.Namespace,
		BucketName:                          &target.Bucket,
		ObjectName:                          &target.Object,
		PartSize:                            common.Int64(partSize),
		NumberOfGoroutines:                  common.Int(concurrency),
		ObjectStorageClient:                 p.client,
		EnableMultipartChecksumVerification: common.Bool(true),
		Metadata:                            convertMetadataToOCI(options.Metadata),
	}

	if options.StorageClass != "" {
		storageTier := objectstorage.PutObjectStorageTierEnum(options.StorageClass)
		uploadRequest.StorageTier = storageTier
	}

	// Wrap reader to ensure it implements io.ReadCloser
	var streamReader io.ReadCloser
	if rc, ok := reader.(io.ReadCloser); ok {
		streamReader = rc
	} else {
		streamReader = io.NopCloser(reader)
	}

	// Set up progress callback if configured
	if options.Progress != nil {
		var totalUploaded int64
		uploadRequest.CallBack = func(multiPartUploadPart transfer.MultiPartUploadPart) {
			if multiPartUploadPart.Err == nil {
				totalUploaded += multiPartUploadPart.Size
				options.Progress.Update(totalUploaded, size)
			} else {
				options.Progress.Error(multiPartUploadPart.Err)
			}
		}
	}

	// Create upload manager and perform upload
	uploadManager := transfer.NewUploadManager()
	req := transfer.UploadStreamRequest{
		UploadRequest: uploadRequest,
		StreamReader:  streamReader,
	}
	response, err := uploadManager.UploadStream(ctx, req)
	if err != nil {
		if options.Progress != nil {
			options.Progress.Error(err)
		}
		return err
	}

	if response.MultipartUploadResponse != nil {
		p.logger.WithField("etag", response.MultipartUploadResponse.ETag).
			WithField("object", target.Object).
			Info("Multipart stream upload completed")
	} else if response.SinglepartUploadResponse != nil {
		p.logger.WithField("etag", response.SinglepartUploadResponse.ETag).
			WithField("object", target.Object).
			Info("Single-part stream upload completed")
	}

	return nil
}

// calculateOptimalPartSize determines the best part size for multipart upload
func calculateOptimalPartSize(fileSize int64) int64 {
	// OCI has a maximum of 10,000 parts
	calculatedSize := fileSize / int64(maxParts-1000) // Leave some buffer

	// Ensure minimum part size
	if calculatedSize < minPartSize {
		return minPartSize
	}

	// Use default if reasonable
	defaultSize := int64(defaultPartSizeMB * 1024 * 1024)
	if calculatedSize < defaultSize {
		return defaultSize
	}

	// Round up to nearest MB
	return ((calculatedSize / (1024 * 1024)) + 1) * 1024 * 1024
}
