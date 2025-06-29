package oci

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/ome/pkg/auth"
	authoci "github.com/sgl-project/ome/pkg/auth/oci"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// OCIStorage implements storage.Storage for OCI Object Storage
type OCIStorage struct {
	client        *objectstorage.ObjectStorageClient
	credentials   auth.Credentials
	logger        logging.Interface
	config        *Config
	compartmentID string
	namespace     *string
}

// New creates a new OCI storage instance
func New(ctx context.Context, config *Config, credentials auth.Credentials, logger logging.Interface) (*OCIStorage, error) {
	// Ensure we have OCI credentials
	ociCreds, ok := credentials.(*authoci.OCICredentials)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type: expected OCI credentials")
	}

	// Get the OCI configuration provider
	configProvider := ociCreds.GetConfigurationProvider()

	// Apply defaults
	if config == nil {
		config = DefaultConfig()
	} else {
		// Apply defaults for missing values
		defaultConfig := DefaultConfig()
		if config.HTTPTimeout == 0 {
			config.HTTPTimeout = defaultConfig.HTTPTimeout
		}
		if config.MaxIdleConns == 0 {
			config.MaxIdleConns = defaultConfig.MaxIdleConns
		}
		if config.MaxIdleConnsPerHost == 0 {
			config.MaxIdleConnsPerHost = defaultConfig.MaxIdleConnsPerHost
		}
		if config.MaxConnsPerHost == 0 {
			config.MaxConnsPerHost = defaultConfig.MaxConnsPerHost
		}
	}

	// Create OCI object storage client
	client, err := createObjectStorageClient(configProvider, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create OCI client: %w", err)
	}

	storage := &OCIStorage{
		client:        client,
		credentials:   credentials,
		logger:        logger,
		config:        config,
		compartmentID: config.CompartmentID,
	}

	// Get namespace
	namespace, err := storage.getNamespace(ctx)
	if err != nil {
		logger.WithError(err).Warn("Failed to get namespace")
	} else {
		storage.namespace = namespace
	}

	return storage, nil
}

// Provider returns the storage provider type
func (s *OCIStorage) Provider() storage.Provider {
	return storage.ProviderOCI
}

// Download retrieves the object and writes it to the target path
func (s *OCIStorage) Download(ctx context.Context, source storage.ObjectURI, target string, opts ...storage.DownloadOption) error {
	// Apply download options
	downloadOpts := storage.DefaultDownloadOptions()
	for _, opt := range opts {
		if err := opt(&downloadOpts); err != nil {
			return err
		}
	}

	// Ensure namespace
	if source.Namespace == "" {
		if s.namespace == nil {
			ns, err := s.getNamespace(ctx)
			if err != nil {
				return fmt.Errorf("failed to get namespace: %w", err)
			}
			s.namespace = ns
		}
		source.Namespace = *s.namespace
	}

	// Get object info
	info, err := s.GetObjectInfo(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to get object info: %w", err)
	}

	// Decide whether to use multipart download
	useMultipart := false
	if downloadOpts.ForceMultipart {
		useMultipart = true
	} else if !downloadOpts.ForceStandard && info.Size > int64(downloadOpts.SizeThresholdInMB)*1024*1024 {
		useMultipart = true
	}

	if useMultipart {
		return s.multipartDownload(ctx, source, target, info.Size, &downloadOpts)
	}

	// Simple download
	reader, err := s.Get(ctx, source)
	if err != nil {
		return err
	}
	defer reader.Close()

	return writeToFile(target, reader)
}

// Upload stores the file at source path as the target object
func (s *OCIStorage) Upload(ctx context.Context, source string, target storage.ObjectURI, opts ...storage.UploadOption) error {
	// Open source file
	file, err := openFile(source)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Ensure namespace
	if target.Namespace == "" {
		if s.namespace == nil {
			ns, err := s.getNamespace(ctx)
			if err != nil {
				return fmt.Errorf("failed to get namespace: %w", err)
			}
			s.namespace = ns
		}
		target.Namespace = *s.namespace
	}

	return s.Put(ctx, target, file, info.Size(), opts...)
}

// Get retrieves an object and returns a reader
func (s *OCIStorage) Get(ctx context.Context, uri storage.ObjectURI) (io.ReadCloser, error) {
	req := objectstorage.GetObjectRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
		ObjectName:    &uri.ObjectName,
	}

	resp, err := s.client.GetObject(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	return resp.Content, nil
}

// Put stores data from reader as an object
func (s *OCIStorage) Put(ctx context.Context, uri storage.ObjectURI, reader io.Reader, size int64, opts ...storage.UploadOption) error {
	uploadOpts := storage.DefaultUploadOptions()
	for _, opt := range opts {
		if err := opt(&uploadOpts); err != nil {
			return err
		}
	}

	req := objectstorage.PutObjectRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
		ObjectName:    &uri.ObjectName,
		PutObjectBody: ioutil.NopCloser(reader),
		ContentLength: &size,
	}

	if uploadOpts.ContentType != "" {
		req.ContentType = &uploadOpts.ContentType
	}

	if uploadOpts.StorageClass != "" {
		storageTier := objectstorage.PutObjectStorageTierEnum(uploadOpts.StorageClass)
		req.StorageTier = storageTier
	}

	_, err := s.client.PutObject(ctx, req)
	return err
}

// Delete removes an object
func (s *OCIStorage) Delete(ctx context.Context, uri storage.ObjectURI) error {
	req := objectstorage.DeleteObjectRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
		ObjectName:    &uri.ObjectName,
	}

	_, err := s.client.DeleteObject(ctx, req)
	return err
}

// Exists checks if an object exists
func (s *OCIStorage) Exists(ctx context.Context, uri storage.ObjectURI) (bool, error) {
	req := objectstorage.HeadObjectRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
		ObjectName:    &uri.ObjectName,
	}

	_, err := s.client.HeadObject(ctx, req)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// List returns a list of objects matching the criteria
func (s *OCIStorage) List(ctx context.Context, uri storage.ObjectURI, opts storage.ListOptions) ([]storage.ObjectInfo, error) {
	req := objectstorage.ListObjectsRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
	}

	if opts.Prefix != "" {
		req.Prefix = &opts.Prefix
	}
	if opts.Delimiter != "" {
		req.Delimiter = &opts.Delimiter
	}
	if opts.StartAfter != "" {
		req.Start = &opts.StartAfter
	}
	if opts.MaxKeys > 0 {
		limit := opts.MaxKeys
		req.Limit = &limit
	}

	resp, err := s.client.ListObjects(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	var objects []storage.ObjectInfo
	for _, obj := range resp.Objects {
		info := storage.ObjectInfo{
			Name: *obj.Name,
			Size: *obj.Size,
		}
		if obj.TimeCreated != nil {
			info.LastModified = obj.TimeCreated.String()
		}
		if obj.Etag != nil {
			info.ETag = *obj.Etag
		}
		objects = append(objects, info)
	}

	return objects, nil
}

// GetObjectInfo retrieves metadata about an object
func (s *OCIStorage) GetObjectInfo(ctx context.Context, uri storage.ObjectURI) (*storage.ObjectInfo, error) {
	req := objectstorage.HeadObjectRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
		ObjectName:    &uri.ObjectName,
	}

	resp, err := s.client.HeadObject(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	info := &storage.ObjectInfo{
		Name: uri.ObjectName,
	}

	if resp.ContentLength != nil {
		info.Size = *resp.ContentLength
	}
	if resp.LastModified != nil {
		info.LastModified = resp.LastModified.String()
	}
	if resp.ETag != nil {
		info.ETag = *resp.ETag
	}
	if resp.ContentType != nil {
		info.ContentType = *resp.ContentType
	}

	return info, nil
}

// Copy copies an object within the same storage system
func (s *OCIStorage) Copy(ctx context.Context, source, target storage.ObjectURI) error {

	req := objectstorage.CopyObjectRequest{
		NamespaceName: &source.Namespace,
		BucketName:    &source.BucketName,
		CopyObjectDetails: objectstorage.CopyObjectDetails{
			SourceObjectName:      &source.ObjectName,
			DestinationRegion:     &target.Region,
			DestinationNamespace:  &target.Namespace,
			DestinationBucket:     &target.BucketName,
			DestinationObjectName: &target.ObjectName,
		},
	}

	_, err := s.client.CopyObject(ctx, req)
	return err
}

// Multipart operations

// InitiateMultipartUpload starts a multipart upload
func (s *OCIStorage) InitiateMultipartUpload(ctx context.Context, uri storage.ObjectURI, opts ...storage.UploadOption) (string, error) {
	uploadOpts := storage.DefaultUploadOptions()
	for _, opt := range opts {
		if err := opt(&uploadOpts); err != nil {
			return "", err
		}
	}

	req := objectstorage.CreateMultipartUploadRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
		CreateMultipartUploadDetails: objectstorage.CreateMultipartUploadDetails{
			Object: &uri.ObjectName,
		},
	}

	if uploadOpts.ContentType != "" {
		req.CreateMultipartUploadDetails.ContentType = &uploadOpts.ContentType
	}

	if uploadOpts.StorageClass != "" {
		storageTier := objectstorage.StorageTierEnum(uploadOpts.StorageClass)
		req.CreateMultipartUploadDetails.StorageTier = storageTier
	}

	resp, err := s.client.CreateMultipartUpload(ctx, req)
	if err != nil {
		return "", err
	}

	return *resp.UploadId, nil
}

// UploadPart uploads a part of a multipart upload
func (s *OCIStorage) UploadPart(ctx context.Context, uri storage.ObjectURI, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	req := objectstorage.UploadPartRequest{
		NamespaceName:  &uri.Namespace,
		BucketName:     &uri.BucketName,
		ObjectName:     &uri.ObjectName,
		UploadId:       &uploadID,
		UploadPartNum:  &partNumber,
		UploadPartBody: ioutil.NopCloser(reader),
		ContentLength:  &size,
	}

	resp, err := s.client.UploadPart(ctx, req)
	if err != nil {
		return "", err
	}

	return *resp.ETag, nil
}

// CompleteMultipartUpload completes a multipart upload
func (s *OCIStorage) CompleteMultipartUpload(ctx context.Context, uri storage.ObjectURI, uploadID string, parts []storage.CompletedPart) error {
	var commitParts []objectstorage.CommitMultipartUploadPartDetails
	for _, part := range parts {
		commitParts = append(commitParts, objectstorage.CommitMultipartUploadPartDetails{
			PartNum: &part.PartNumber,
			Etag:    &part.ETag,
		})
	}

	req := objectstorage.CommitMultipartUploadRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
		ObjectName:    &uri.ObjectName,
		UploadId:      &uploadID,
		CommitMultipartUploadDetails: objectstorage.CommitMultipartUploadDetails{
			PartsToCommit: commitParts,
		},
	}

	_, err := s.client.CommitMultipartUpload(ctx, req)
	return err
}

// AbortMultipartUpload cancels a multipart upload
func (s *OCIStorage) AbortMultipartUpload(ctx context.Context, uri storage.ObjectURI, uploadID string) error {
	req := objectstorage.AbortMultipartUploadRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
		ObjectName:    &uri.ObjectName,
		UploadId:      &uploadID,
	}

	_, err := s.client.AbortMultipartUpload(ctx, req)
	return err
}
