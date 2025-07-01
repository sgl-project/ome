package oci

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/oracle/oci-go-sdk/v65/common"
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

	// Compute actual target path based on download options
	actualTarget := target
	if downloadOpts.StripPrefix || downloadOpts.UseBaseNameOnly || downloadOpts.JoinWithTailOverlap {
		targetDir := filepath.Dir(target)
		actualTarget = storage.ComputeLocalPath(targetDir, source.ObjectName, downloadOpts)
	}

	// Check if we should skip existing valid files
	if !downloadOpts.DisableOverride {
		if exists, _ := storage.FileExists(actualTarget); exists {
			// Convert ObjectInfo to Metadata for validation
			metadata := storage.Metadata{
				ObjectInfo: *info,
			}
			if valid, _ := storage.IsLocalFileValid(actualTarget, metadata); valid {
				return nil // Skip download, file is already valid
			}
		}
	}

	return writeToFile(actualTarget, reader)
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
	// Ensure namespace
	if uri.Namespace == "" {
		if s.namespace == nil {
			ns, err := s.getNamespace(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get namespace: %w", err)
			}
			s.namespace = ns
		}
		uri.Namespace = *s.namespace
	}

	req := objectstorage.ListObjectsRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
		Fields:        common.String("name,size,md5"),
	}

	// Use prefix from options if provided, otherwise use URI prefix
	if opts.Prefix != "" {
		req.Prefix = &opts.Prefix
	} else if uri.Prefix != "" {
		req.Prefix = &uri.Prefix
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

	var objects []storage.ObjectInfo
	page := 0

	for {
		resp, err := s.client.ListObjects(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects at page %d: %w", page, err)
		}

		for i, obj := range resp.Objects {
			// Skip objects with nil required fields
			if obj.Name == nil || obj.Size == nil {
				logger := s.logger.
					WithField("page", page).
					WithField("index", i).
					WithField("has_name", obj.Name != nil).
					WithField("has_size", obj.Size != nil)

				if obj.Name != nil {
					logger = logger.WithField("name", *obj.Name)
				}
				if obj.Md5 != nil {
					logger = logger.WithField("has_md5", true)
				}

				logger.Debug("Skipping object with nil required field")
				continue
			}

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

		// Check if there are more results
		if resp.NextStartWith == nil {
			break
		}

		// Update request for next page
		req.Start = resp.NextStartWith
		page++

		// If MaxKeys is set and we've already fetched enough objects, break
		if opts.MaxKeys > 0 && len(objects) >= opts.MaxKeys {
			// Trim to exact MaxKeys count
			objects = objects[:opts.MaxKeys]
			break
		}
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
	if resp.OpcMeta != nil {
		info.Metadata = resp.OpcMeta
	}

	return info, nil
}

// Stat retrieves metadata about an object (alias for GetObjectInfo)
func (s *OCIStorage) Stat(ctx context.Context, uri storage.ObjectURI) (*storage.Metadata, error) {
	// First get the basic object info
	info, err := s.GetObjectInfo(ctx, uri)
	if err != nil {
		return nil, err
	}

	// Get additional metadata via HeadObject
	req := objectstorage.HeadObjectRequest{
		NamespaceName: &uri.Namespace,
		BucketName:    &uri.BucketName,
		ObjectName:    &uri.ObjectName,
	}

	resp, err := s.client.HeadObject(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Create Metadata struct with all fields
	metadata := &storage.Metadata{
		ObjectInfo: *info,
	}

	// Add additional metadata fields
	if resp.CacheControl != nil {
		metadata.CacheControl = *resp.CacheControl
	}
	// OCI doesn't have Expires in HeadObject response
	if resp.VersionId != nil {
		metadata.VersionID = *resp.VersionId
	}
	if resp.ContentMd5 != nil {
		metadata.ContentMD5 = *resp.ContentMd5
	}
	// Check if it's a multipart upload by looking for multipart MD5
	if resp.OpcMultipartMd5 != nil && *resp.OpcMultipartMd5 != "" {
		metadata.IsMultipart = true
		// For multipart uploads, check if actual MD5 is stored in metadata
		if resp.OpcMeta != nil {
			if md5, ok := resp.OpcMeta["md5"]; ok && md5 != "" {
				metadata.ContentMD5 = md5
			}
		}
		// OCI doesn't expose parts count in HeadObject response
	}

	// Collect additional headers
	metadata.Headers = make(map[string]string)
	if resp.ContentEncoding != nil {
		metadata.Headers["Content-Encoding"] = *resp.ContentEncoding
	}
	if resp.ContentLanguage != nil {
		metadata.Headers["Content-Language"] = *resp.ContentLanguage
	}
	if resp.ContentDisposition != nil {
		metadata.Headers["Content-Disposition"] = *resp.ContentDisposition
	}
	// ArchivalState is an enum, not a pointer
	if resp.ArchivalState != "" {
		metadata.Headers["archival-state"] = string(resp.ArchivalState)
	}
	// StorageTier is also an enum
	if resp.StorageTier != "" {
		metadata.Headers["storage-tier"] = string(resp.StorageTier)
	}

	return metadata, nil
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

	// Add metadata including MD5 if provided
	if uploadOpts.Metadata != nil && len(uploadOpts.Metadata) > 0 {
		req.CreateMultipartUploadDetails.Metadata = uploadOpts.Metadata
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
