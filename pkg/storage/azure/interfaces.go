package azure

import (
	"context"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blockblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/sgl-project/ome/pkg/auth"
)

// azureCredentials defines the interface for Azure-specific credentials methods
type azureCredentials interface {
	auth.Credentials
	GetTokenCredential() azcore.TokenCredential
}

// azureClient defines the interface for Azure Blob client operations
type azureClient interface {
	UploadStream(ctx context.Context, containerName, blobName string, body io.Reader, options *azblob.UploadStreamOptions) (azblob.UploadStreamResponse, error)
	UploadFile(ctx context.Context, containerName, blobName string, file io.Reader, options *azblob.UploadFileOptions) (azblob.UploadFileResponse, error)
	DownloadStream(ctx context.Context, containerName, blobName string, options *azblob.DownloadStreamOptions) (azblob.DownloadStreamResponse, error)
	DownloadFile(ctx context.Context, containerName, blobName, file string, options *azblob.DownloadFileOptions) (int64, error)
	DeleteBlob(ctx context.Context, containerName, blobName string, options *blob.DeleteOptions) (blob.DeleteResponse, error)
	ServiceClient() azServiceClient
}

// containerClient defines the interface for container operations
type containerClient interface {
	NewBlockBlobClient(blobName string) *blockblob.Client
}

// blockBlobClient defines the interface for block blob operations
type blockBlobClient interface {
	UploadStream(ctx context.Context, body io.Reader, options *blockblob.UploadStreamOptions) (blockblob.UploadStreamResponse, error)
	UploadFile(ctx context.Context, file io.Reader, options *blockblob.UploadFileOptions) (blockblob.UploadFileResponse, error)
	DownloadStream(ctx context.Context, options *blob.DownloadStreamOptions) (blob.DownloadStreamResponse, error)
	DownloadFile(ctx context.Context, file *os.File, options *blob.DownloadFileOptions) (int64, error)
	Delete(ctx context.Context, options *blob.DeleteOptions) (blob.DeleteResponse, error)
	GetProperties(ctx context.Context, options *blob.GetPropertiesOptions) (blob.GetPropertiesResponse, error)
	StageBlock(ctx context.Context, base64BlockID string, body io.ReadSeekCloser, options *blockblob.StageBlockOptions) (blockblob.StageBlockResponse, error)
	CommitBlockList(ctx context.Context, base64BlockIDs []string, options *blockblob.CommitBlockListOptions) (blockblob.CommitBlockListResponse, error)
	GetBlockList(ctx context.Context, listType blockblob.BlockListType, options *blockblob.GetBlockListOptions) (blockblob.GetBlockListResponse, error)
}

// azServiceClient defines the interface for service client operations
type azServiceClient interface {
	NewContainerClient(containerName string) azContainerClient
}

// azContainerClient defines the interface for container client operations
type azContainerClient interface {
	NewBlobClient(blobName string) azBlobClient
	NewBlockBlobClient(blobName string) azBlockBlobClient
	NewListBlobsFlatPager(options *container.ListBlobsFlatOptions) *runtime.Pager[container.ListBlobsFlatResponse]
}

// azBlobClient defines the interface for blob client operations
type azBlobClient interface {
	GetProperties(ctx context.Context, options *blob.GetPropertiesOptions) (blob.GetPropertiesResponse, error)
	DownloadStream(ctx context.Context, options *blob.DownloadStreamOptions) (blob.DownloadStreamResponse, error)
	DownloadFile(ctx context.Context, file *os.File, options *blob.DownloadFileOptions) (int64, error)
	Delete(ctx context.Context, options *blob.DeleteOptions) (blob.DeleteResponse, error)
	StartCopyFromURL(ctx context.Context, copySource string, options *blob.StartCopyFromURLOptions) (blob.StartCopyFromURLResponse, error)
}

// azBlockBlobClient defines the interface for block blob client operations
type azBlockBlobClient interface {
	Upload(ctx context.Context, body io.ReadSeekCloser, options *blockblob.UploadOptions) (blockblob.UploadResponse, error)
	StageBlock(ctx context.Context, base64BlockID string, body io.ReadSeekCloser, options *blockblob.StageBlockOptions) (blockblob.StageBlockResponse, error)
	CommitBlockList(ctx context.Context, base64BlockIDs []string, options *blockblob.CommitBlockListOptions) (blockblob.CommitBlockListResponse, error)
	GetBlockList(ctx context.Context, listType blockblob.BlockListType, options *blockblob.GetBlockListOptions) (blockblob.GetBlockListResponse, error)
}
