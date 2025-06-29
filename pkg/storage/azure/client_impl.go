package azure

import (
	"context"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

// azureClientImpl is the concrete implementation of azureClient interface
type azureClientImpl struct {
	client *azblob.Client
}

func (c *azureClientImpl) UploadStream(ctx context.Context, containerName, blobName string, body io.Reader, options *azblob.UploadStreamOptions) (azblob.UploadStreamResponse, error) {
	containerClient := c.client.ServiceClient().NewContainerClient(containerName)
	blockBlobClient := containerClient.NewBlockBlobClient(blobName)

	// Convert options to blockblob options
	var uploadOptions *blockblob.UploadStreamOptions
	if options != nil {
		uploadOptions = &blockblob.UploadStreamOptions{
			BlockSize:   options.BlockSize,
			Concurrency: options.Concurrency,
		}
		if options.HTTPHeaders != nil {
			uploadOptions.HTTPHeaders = options.HTTPHeaders
		}
		if options.Metadata != nil {
			uploadOptions.Metadata = options.Metadata
		}
		if options.AccessTier != nil {
			uploadOptions.AccessTier = options.AccessTier
		}
	}

	resp, err := blockBlobClient.UploadStream(ctx, body, uploadOptions)
	if err != nil {
		return azblob.UploadStreamResponse{}, err
	}

	// Convert response
	return azblob.UploadStreamResponse{
		ClientRequestID:     resp.ClientRequestID,
		ContentMD5:          resp.ContentMD5,
		Date:                resp.Date,
		ETag:                resp.ETag,
		EncryptionKeySHA256: resp.EncryptionKeySHA256,
		EncryptionScope:     resp.EncryptionScope,
		IsServerEncrypted:   resp.IsServerEncrypted,
		LastModified:        resp.LastModified,
		RequestID:           resp.RequestID,
		Version:             resp.Version,
		VersionID:           resp.VersionID,
	}, nil
}

func (c *azureClientImpl) UploadFile(ctx context.Context, containerName, blobName string, file io.Reader, options *azblob.UploadFileOptions) (azblob.UploadFileResponse, error) {
	containerClient := c.client.ServiceClient().NewContainerClient(containerName)
	blockBlobClient := containerClient.NewBlockBlobClient(blobName)

	// For simplicity, we'll use UploadStream
	// In production, this would handle file-specific operations
	uploadOptions := &blockblob.UploadStreamOptions{}
	if options != nil {
		uploadOptions.BlockSize = options.BlockSize
		uploadOptions.Concurrency = int(options.Concurrency)
		if options.HTTPHeaders != nil {
			uploadOptions.HTTPHeaders = options.HTTPHeaders
		}
		if options.Metadata != nil {
			uploadOptions.Metadata = options.Metadata
		}
		if options.AccessTier != nil {
			uploadOptions.AccessTier = options.AccessTier
		}
	}

	resp, err := blockBlobClient.UploadStream(ctx, file, uploadOptions)
	if err != nil {
		return azblob.UploadFileResponse{}, err
	}

	// Convert response
	return azblob.UploadFileResponse{
		ClientRequestID:     resp.ClientRequestID,
		ContentMD5:          resp.ContentMD5,
		Date:                resp.Date,
		ETag:                resp.ETag,
		EncryptionKeySHA256: resp.EncryptionKeySHA256,
		EncryptionScope:     resp.EncryptionScope,
		IsServerEncrypted:   resp.IsServerEncrypted,
		LastModified:        resp.LastModified,
		RequestID:           resp.RequestID,
		Version:             resp.Version,
		VersionID:           resp.VersionID,
	}, nil
}

func (c *azureClientImpl) DownloadStream(ctx context.Context, containerName, blobName string, options *azblob.DownloadStreamOptions) (azblob.DownloadStreamResponse, error) {
	containerClient := c.client.ServiceClient().NewContainerClient(containerName)
	blobClient := containerClient.NewBlobClient(blobName)

	// Convert options
	var downloadOptions *blob.DownloadStreamOptions
	if options != nil {
		downloadOptions = &blob.DownloadStreamOptions{
			Range:              options.Range,
			RangeGetContentMD5: options.RangeGetContentMD5,
			AccessConditions:   options.AccessConditions,
			CPKInfo:            options.CPKInfo,
			CPKScopeInfo:       options.CPKScopeInfo,
		}
	}

	resp, err := blobClient.DownloadStream(ctx, downloadOptions)
	if err != nil {
		return azblob.DownloadStreamResponse{}, err
	}

	// Return the response as-is since it's already the correct type
	return resp, nil
}

func (c *azureClientImpl) DownloadFile(ctx context.Context, containerName, blobName, fileName string, options *azblob.DownloadFileOptions) (int64, error) {
	containerClient := c.client.ServiceClient().NewContainerClient(containerName)
	blobClient := containerClient.NewBlobClient(blobName)

	// Convert options
	var downloadOptions *blob.DownloadFileOptions
	if options != nil {
		downloadOptions = &blob.DownloadFileOptions{
			BlockSize:                  options.BlockSize,
			Concurrency:                options.Concurrency,
			AccessConditions:           options.AccessConditions,
			CPKInfo:                    options.CPKInfo,
			CPKScopeInfo:               options.CPKScopeInfo,
			Progress:                   options.Progress,
			RetryReaderOptionsPerBlock: options.RetryReaderOptionsPerBlock,
		}
	}

	// Open file for writing
	file, err := os.Create(fileName)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	return blobClient.DownloadFile(ctx, file, downloadOptions)
}

func (c *azureClientImpl) DeleteBlob(ctx context.Context, containerName, blobName string, options *blob.DeleteOptions) (blob.DeleteResponse, error) {
	containerClient := c.client.ServiceClient().NewContainerClient(containerName)
	blobClient := containerClient.NewBlobClient(blobName)

	return blobClient.Delete(ctx, options)
}

func (c *azureClientImpl) ServiceClient() azServiceClient {
	return &azServiceClientImpl{client: c.client}
}

// azServiceClientImpl implements azServiceClient interface
type azServiceClientImpl struct {
	client *azblob.Client
}

func (s *azServiceClientImpl) NewContainerClient(containerName string) azContainerClient {
	return &azContainerClientImpl{client: s.client.ServiceClient().NewContainerClient(containerName)}
}

// azContainerClientImpl implements azContainerClient interface
type azContainerClientImpl struct {
	client *container.Client
}

func (c *azContainerClientImpl) NewBlobClient(blobName string) azBlobClient {
	return &azBlobClientImpl{client: c.client.NewBlobClient(blobName)}
}

func (c *azContainerClientImpl) NewBlockBlobClient(blobName string) azBlockBlobClient {
	return &azBlockBlobClientImpl{client: c.client.NewBlockBlobClient(blobName)}
}

func (c *azContainerClientImpl) NewListBlobsFlatPager(options *container.ListBlobsFlatOptions) *runtime.Pager[container.ListBlobsFlatResponse] {
	return c.client.NewListBlobsFlatPager(options)
}

// azBlobClientImpl implements azBlobClient interface
type azBlobClientImpl struct {
	client *blob.Client
}

func (b *azBlobClientImpl) GetProperties(ctx context.Context, options *blob.GetPropertiesOptions) (blob.GetPropertiesResponse, error) {
	return b.client.GetProperties(ctx, options)
}

func (b *azBlobClientImpl) DownloadStream(ctx context.Context, options *blob.DownloadStreamOptions) (blob.DownloadStreamResponse, error) {
	return b.client.DownloadStream(ctx, options)
}

func (b *azBlobClientImpl) DownloadFile(ctx context.Context, file *os.File, options *blob.DownloadFileOptions) (int64, error) {
	return b.client.DownloadFile(ctx, file, options)
}

func (b *azBlobClientImpl) Delete(ctx context.Context, options *blob.DeleteOptions) (blob.DeleteResponse, error) {
	return b.client.Delete(ctx, options)
}

func (b *azBlobClientImpl) StartCopyFromURL(ctx context.Context, copySource string, options *blob.StartCopyFromURLOptions) (blob.StartCopyFromURLResponse, error) {
	return b.client.StartCopyFromURL(ctx, copySource, options)
}

// azBlockBlobClientImpl implements azBlockBlobClient interface
type azBlockBlobClientImpl struct {
	client *blockblob.Client
}

func (b *azBlockBlobClientImpl) Upload(ctx context.Context, body io.ReadSeekCloser, options *blockblob.UploadOptions) (blockblob.UploadResponse, error) {
	return b.client.Upload(ctx, body, options)
}

func (b *azBlockBlobClientImpl) StageBlock(ctx context.Context, base64BlockID string, body io.ReadSeekCloser, options *blockblob.StageBlockOptions) (blockblob.StageBlockResponse, error) {
	return b.client.StageBlock(ctx, base64BlockID, body, options)
}

func (b *azBlockBlobClientImpl) CommitBlockList(ctx context.Context, base64BlockIDs []string, options *blockblob.CommitBlockListOptions) (blockblob.CommitBlockListResponse, error) {
	return b.client.CommitBlockList(ctx, base64BlockIDs, options)
}

func (b *azBlockBlobClientImpl) GetBlockList(ctx context.Context, listType blockblob.BlockListType, options *blockblob.GetBlockListOptions) (blockblob.GetBlockListResponse, error) {
	return b.client.GetBlockList(ctx, listType, options)
}
