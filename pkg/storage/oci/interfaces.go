package oci

import (
	"context"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// ObjectStorageClientInterface defines the interface for OCI Object Storage operations
// This allows for easy mocking in unit tests
type ObjectStorageClientInterface interface {
	GetNamespace(ctx context.Context, req objectstorage.GetNamespaceRequest) (objectstorage.GetNamespaceResponse, error)
	GetObject(ctx context.Context, req objectstorage.GetObjectRequest) (objectstorage.GetObjectResponse, error)
	PutObject(ctx context.Context, req objectstorage.PutObjectRequest) (objectstorage.PutObjectResponse, error)
	DeleteObject(ctx context.Context, req objectstorage.DeleteObjectRequest) (objectstorage.DeleteObjectResponse, error)
	HeadObject(ctx context.Context, req objectstorage.HeadObjectRequest) (objectstorage.HeadObjectResponse, error)
	ListObjects(ctx context.Context, req objectstorage.ListObjectsRequest) (objectstorage.ListObjectsResponse, error)
	CopyObject(ctx context.Context, req objectstorage.CopyObjectRequest) (objectstorage.CopyObjectResponse, error)
	CreateMultipartUpload(ctx context.Context, req objectstorage.CreateMultipartUploadRequest) (objectstorage.CreateMultipartUploadResponse, error)
	UploadPart(ctx context.Context, req objectstorage.UploadPartRequest) (objectstorage.UploadPartResponse, error)
	CommitMultipartUpload(ctx context.Context, req objectstorage.CommitMultipartUploadRequest) (objectstorage.CommitMultipartUploadResponse, error)
	AbortMultipartUpload(ctx context.Context, req objectstorage.AbortMultipartUploadRequest) (objectstorage.AbortMultipartUploadResponse, error)
}

// Ensure the OCI SDK client implements our interface
var _ ObjectStorageClientInterface = (*objectstorage.ObjectStorageClient)(nil)
