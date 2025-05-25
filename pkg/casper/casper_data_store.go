// Package casper provides a data store abstraction backed by Oracle Object Storage.
// It supports object uploads, downloads (including multipart), metadata inspection,
// and local integrity validation via MD5 checksum comparison.
package casper

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/sgl-ome/pkg/logging"
)

/*
 * CasperDataStore used to perform data store operations with Object Storage(Casper)
 */

// CasperDataStore performs data store operations against Oracle Object Storage ("Casper").
// It provides file upload, download, listing, and validation methods.
type CasperDataStore struct {
	logger logging.Interface
	Config *Config
	Client *objectstorage.ObjectStorageClient `validate:"required"`
}

// DownloadOptions defines parameters to control SmartDownload behavior.
type DownloadOptions struct {
	SizeThresholdInMB   int      // threshold above which multipart download is used
	ChunkSizeInMB       int      // multipart chunk size
	Threads             int      // number of concurrent download threads
	StripPrefix         bool     // if true, removes object prefix/folder structure
	ForceStandard       bool     // if true, force standard download regardless of size
	ForceMultipart      bool     // if true, force multipart download regardless of size
	DisableOverride     bool     // if true, do not re-download if the local copy is valid
	ExcludePatterns     []string // object names to exclude
	JoinWithTailOverlap bool     // if true, join with tail overlap
}

const (
	maxRetries         = 3
	retryDelay         = 2 * time.Second
	defaultThresholdMB = 100
)

func DefaultDownloadOptions() DownloadOptions {
	return DownloadOptions{
		SizeThresholdInMB:   defaultThresholdMB,
		ChunkSizeInMB:       8,
		Threads:             100,
		StripPrefix:         false,
		ForceStandard:       false,
		ForceMultipart:      false,
		DisableOverride:     true,
		ExcludePatterns:     []string{},
		JoinWithTailOverlap: false,
	}
}

// NewCasperDataStore initializes a CasperDataStore using the given configuration and environment.
// It validates the config, creates an OCI config provider, and initializes the Object Storage client.
func NewCasperDataStore(config *Config) (*CasperDataStore, error) {
	if config == nil {
		return nil, fmt.Errorf("casper config is nil")
	}
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("casper config is invalid: %+v", err)
	}

	configProvider, err := getConfigProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to get config provider: %+v", err)
	}

	client, err := NewObjectStorageClient(configProvider, config)
	if err != nil {
		return nil, err
	}

	return &CasperDataStore{
		logger: config.AnotherLogger,
		Config: config,
		Client: client,
	}, nil
}

// SetRegion updates the configured region for both the client and config object.
func (cds *CasperDataStore) SetRegion(region string) {
	cds.Config.Region = region
	cds.Client.SetRegion(region)
}

func applyDownloadDefaults(opts *DownloadOptions) DownloadOptions {
	defaults := DefaultDownloadOptions()

	if opts == nil {
		return defaults
	}

	merged := *opts

	if merged.SizeThresholdInMB == 0 {
		merged.SizeThresholdInMB = defaults.SizeThresholdInMB
	}
	if merged.ChunkSizeInMB == 0 {
		merged.ChunkSizeInMB = defaults.ChunkSizeInMB
	}
	if merged.Threads == 0 {
		merged.Threads = defaults.Threads
	}
	// bools are false by default; skip
	// slices: if nil, just leave them as is unless you want a default pattern

	return merged
}

// BulkDownload uses SmartDownload for each object with concurrency and retry logic.
func (cds *CasperDataStore) BulkDownload(objects []ObjectURI, targetDir string, opts DownloadOptions, concurrency int) error {
	if len(objects) == 0 {
		return nil
	}

	downloadOpts := applyDownloadDefaults(&opts)

	jobs := make(chan ObjectURI, len(objects))
	errs := make(chan error, len(objects))
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for object := range jobs {
				var err error
				for attempt := 1; attempt <= maxRetries; attempt++ {
					// Compute the intended target file path
					var targetFilePath string
					if downloadOpts.StripPrefix {
						targetFilePath = filepath.Join(targetDir, ExtractPureObjectName(object.ObjectName))
					} else if downloadOpts.JoinWithTailOverlap {
						targetFilePath = JoinWithTailOverlap(targetDir, object.ObjectName)
					} else {
						targetFilePath = filepath.Join(targetDir, object.ObjectName)
					}
					if downloadOpts.DisableOverride {
						valid, errCheck := cds.IsLocalCopyValid(object, targetFilePath)
						if errCheck != nil {
							cds.logger.Warnf("[Worker %d] Failed to check if local copy is valid for %s: %v", workerID, object.ObjectName, errCheck)
						} else if valid {
							cds.logger.Infof("[Worker %d] Skipping download for %s: valid local copy exists at %s", workerID, object.ObjectName, targetFilePath)
							break
						}
					}
					err = cds.SmartDownload(object, targetDir, downloadOpts)
					if err == nil {
						cds.logger.Infof("[Worker %d] Successfully downloaded and validated %s", workerID, object.ObjectName)
						break
					}
					cds.logger.Warnf("[Worker %d] Retry %d for %s after error: %v", workerID, attempt, object.ObjectName, err)
					time.Sleep(retryDelay)
				}
				if err != nil {
					errs <- fmt.Errorf("failed to smart download %s: %w", object.ObjectName, err)
				}
			}
		}(i)
	}

	for _, obj := range objects {
		jobs <- obj
	}
	close(jobs)
	wg.Wait()
	close(errs)

	for err := range errs {
		return err
	}

	cds.logger.Infof("All smart downloads completed.")
	return nil
}

// SmartDownload chooses between standard and multipart download based on object size and DownloadOptions.
func (cds *CasperDataStore) SmartDownload(source ObjectURI, target string, opts DownloadOptions) error {
	downloadOpts := applyDownloadDefaults(&opts)

	source.Prefix = source.ObjectName

	// Exclude if object name matches any exclude pattern
	for _, pat := range opts.ExcludePatterns {
		if strings.Contains(source.ObjectName, pat) {
			cds.logger.Infof("Skipping download for %s: matches exclude pattern %q", source.ObjectName, pat)
			return nil
		}
	}

	// Compute the intended target file path
	var targetFilePath string
	if downloadOpts.StripPrefix {
		targetFilePath = filepath.Join(target, ExtractPureObjectName(source.ObjectName))
	} else if downloadOpts.JoinWithTailOverlap {
		targetFilePath = JoinWithTailOverlap(target, source.ObjectName)
	} else {
		targetFilePath = filepath.Join(target, source.ObjectName)
	}

	if opts.DisableOverride {
		valid, err := cds.IsLocalCopyValid(source, targetFilePath)
		if err != nil {
			return fmt.Errorf("failed to check if local copy is valid: %w", err)
		}
		if valid {
			cds.logger.Infof("Skipping download for %s: valid local copy exists at %s", source.ObjectName, targetFilePath)
			return nil
		}
	}

	objects, err := cds.ListObjects(source)
	if err != nil {
		return fmt.Errorf("failed to list objects for %s: %w", source.ObjectName, err)
	}
	if len(objects) == 0 {
		return fmt.Errorf("object %s not found in bucket %s", source.ObjectName, source.BucketName)
	}

	object := objects[0]

	if downloadOpts.ForceStandard {
		cds.logger.Infof("SmartDownload forced standard download for %s", source.ObjectName)
		return cds.Download(source, target, downloadOpts)
	}

	if downloadOpts.ForceMultipart || (object.Size != nil && *object.Size >= int64(downloadOpts.SizeThresholdInMB)*1024*1024) {
		cds.logger.Infof("SmartDownload using multipart for %s, size: %d", source.ObjectName, *object.Size)
		return cds.MultipartDownload(source, target, downloadOpts)
	}

	cds.logger.Infof("SmartDownload using standard download for %s", source.ObjectName)
	return cds.Download(source, target, downloadOpts)
}

func (cds *CasperDataStore) DownloadBasedOnObjectSize(source ObjectURI, target string, excludePrefix bool, sizeThresholdInMB int, downloadingChunkSize int, downloadingThread int) error {
	source.Prefix = source.ObjectName

	downloadOpts := DefaultDownloadOptions()
	downloadOpts.StripPrefix = excludePrefix
	downloadOpts.SizeThresholdInMB = sizeThresholdInMB
	downloadOpts.ChunkSizeInMB = downloadingChunkSize
	downloadOpts.Threads = downloadingThread

	objectSummary, err := cds.ListObjects(source)
	if err != nil {
		return fmt.Errorf("failed to do object list: %+v", err)
	}

	if len(objectSummary) == 0 {
		return fmt.Errorf("object %s not found in object storage bucket: %s, in namespace: %s", source.ObjectName, source.BucketName, source.Namespace)
	}

	object := objectSummary[0]

	if object.Size == nil {
		cds.logger.Infof("Regular download %s \n", source.ObjectName)
		err = cds.Download(source, target, downloadOpts)
	} else if *(object.Size) < (int64(sizeThresholdInMB) * int64(MB)) {
		cds.logger.Infof("Regular download %s, size: %d \n", source.ObjectName, *(object.Size))
		err = cds.Download(source, target, downloadOpts)
	} else {
		cds.logger.Infof("Multipart download %s, size: %d \n", source.ObjectName, *(object.Size))
		err = cds.MultipartDownload(source, target, downloadOpts)
	}

	if err != nil {
		return fmt.Errorf("failed to download object %s in object storage bucket: %s, in namespace: %s: %+v", source.ObjectName, source.BucketName, source.Namespace, err)
	}

	return nil
}

func (cds *CasperDataStore) Download(source ObjectURI, target string, opts DownloadOptions) error {
	downloadOpts := applyDownloadDefaults(&opts)
	objectFullName := fmt.Sprintf(
		"%s/%s/%s", source.Namespace, source.BucketName, source.ObjectName)

	response, err := cds.GetObject(source)
	if err != nil {
		return err
	}
	responseContent := response.Content
	defer func(responseContent io.ReadCloser) {
		err := responseContent.Close()
		if err != nil {
			cds.logger.Errorf("Failed to close response content: %+v", err)
		}
	}(responseContent)

	// Write the downloaded object to the target file
	var targetFilePath string
	if downloadOpts.StripPrefix {
		targetFilePath = filepath.Join(target, ExtractPureObjectName(source.ObjectName))
	} else if downloadOpts.JoinWithTailOverlap {
		targetFilePath = JoinWithTailOverlap(target, source.ObjectName)
	} else {
		targetFilePath = filepath.Join(target, source.ObjectName)
	}

	err = os.MkdirAll(path.Dir(targetFilePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf(
			"failed to create the directory %s under the target path %s, error: %+v",
			path.Dir(targetFilePath), target, err)
	}

	err = CopyReaderToFilePath(responseContent, targetFilePath)
	if err != nil {
		return fmt.Errorf(
			"failed to load downloaded object %s to the target path %s, error: %+v",
			objectFullName, target, err)
	}
	return nil
}

// Upload uploads a file (or string content) to OCI Object Storage.
//
// If the `source` is a file path, the file is read and uploaded.
// If the `source` is a raw string, it is uploaded as the object body.
func (cds *CasperDataStore) Upload(source string, target ObjectURI) error {
	if target.Namespace == "" {
		namespace, err := cds.GetNamespace()
		if err != nil {
			return fmt.Errorf("error upload object due to no namespace found: %+v", err)
		}
		target.Namespace = *namespace
	}

	objectFullName := fmt.Sprintf(
		"%s/%s/%s", target.Namespace, target.BucketName, target.ObjectName)

	var putObjectBody io.ReadCloser
	var uploadObjectSize *int64

	// When source is the path of the file which needs to be uploaded
	if sourceFile, err := os.Open(source); err == nil {
		fileInfo, err := sourceFile.Stat()
		if err != nil {
			return fmt.Errorf(
				"failed to get source file info %q: %+v",
				source,
				err)
		}
		putObjectBody = io.NopCloser(sourceFile)
		tmp := fileInfo.Size()
		uploadObjectSize = &tmp
	} else {
		// When the source is pure string content that needs to be uploaded
		putObjectBody = io.NopCloser(strings.NewReader(source))
		tmp := int64(len(source))
		uploadObjectSize = &tmp
	}

	putObjectRequest := objectstorage.PutObjectRequest{
		NamespaceName: &target.Namespace,
		BucketName:    &target.BucketName,
		ObjectName:    &target.ObjectName,
		ContentLength: uploadObjectSize,
		PutObjectBody: putObjectBody,
	}
	// Make the put request to Casper
	response, err := cds.Client.PutObject(context.Background(), putObjectRequest)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"failed to put object %q with response %+v: %s",
			objectFullName,
			response,
			err.Error())
	}
	return nil
}

// HeadObject fetches metadata headers for an object in OCI Object Storage.
//
// It returns an OCI HeadObjectResponse which contains fields such as size, ETag, and MD5 checksum.
func (cds *CasperDataStore) HeadObject(target ObjectURI) (*objectstorage.HeadObjectResponse, error) {
	if target.Namespace == "" {
		namespace, err := cds.GetNamespace()
		if err != nil {
			return nil, fmt.Errorf("error head object due to no namespace found: %+v", err)
		}
		target.Namespace = *namespace
	}

	objectFullName := fmt.Sprintf(
		"%s/%s/%s", target.Namespace, target.BucketName, target.ObjectName)
	headObjectRequest := objectstorage.HeadObjectRequest{
		NamespaceName: &target.Namespace,
		BucketName:    &target.BucketName,
		ObjectName:    &target.ObjectName,
	}

	response, err := cds.Client.HeadObject(context.Background(), headObjectRequest)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"failed to head object %q with response %+v: %s",
			objectFullName,
			response,
			err.Error())
	}
	return &response, nil
}

// GetObject retrieves the object body and headers from OCI Object Storage.
// The caller is responsible for closing the returned ReadCloser.
func (cds *CasperDataStore) GetObject(source ObjectURI) (*objectstorage.GetObjectResponse, error) {
	if source.Namespace == "" {
		namespace, err := cds.GetNamespace()
		if err != nil {
			return nil, fmt.Errorf("error get object due to no namespace found: %+v", err)
		}
		source.Namespace = *namespace
	}

	objectFullName := fmt.Sprintf(
		"%s/%s/%s", source.Namespace, source.BucketName, source.ObjectName)

	getObjectRequest := objectstorage.GetObjectRequest{
		NamespaceName: &source.Namespace,
		BucketName:    &source.BucketName,
		ObjectName:    &source.ObjectName,
	}
	response, err := cds.Client.GetObject(context.Background(), getObjectRequest)

	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"failed to download object %s with response %+v: %+v",
			objectFullName,
			response,
			err)
	}

	return &response, nil
}

// GetNamespace returns the current OCI Object Storage namespace for the authenticated principal.
// Required for all object storage operations.
func (cds *CasperDataStore) GetNamespace() (*string, error) {
	getNamespaceClient := objectstorage.GetNamespaceRequest{}
	if cds.Config.CompartmentId != nil {
		getNamespaceClient.CompartmentId = cds.Config.CompartmentId
	}

	response, err := cds.Client.GetNamespace(context.Background(), getNamespaceClient)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting casper namespace: %+v", err)
	}

	return response.Value, nil
}

// ListObjects lists all objects under the given prefix (virtual folder) in the target bucket.
//
// Returns a list of object summaries containing name, size, and MD5 info.
func (cds *CasperDataStore) ListObjects(target ObjectURI) ([]objectstorage.ObjectSummary, error) {
	if target.Namespace == "" {
		namespace, err := cds.GetNamespace()
		if err != nil {
			return nil, fmt.Errorf("error list objects due to no namespace found: %+v", err)
		}
		target.Namespace = *namespace
	}

	listObjectsRequest := objectstorage.ListObjectsRequest{
		NamespaceName: &target.Namespace,
		BucketName:    &target.BucketName,
		Prefix:        &target.Prefix, //Virtual folder name within bucket
		Fields:        common.String("name,size,md5"),
	}

	var allObjects []objectstorage.ObjectSummary
	page := 0
	for {
		response, err := cds.Client.ListObjects(context.Background(), listObjectsRequest)
		if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("error listing objects at page %d: %+v", page, err)
		}
		allObjects = append(allObjects, response.Objects...)

		if response.NextStartWith == nil {
			break
		}

		listObjectsRequest.Start = response.NextStartWith
		page++
	}

	return allObjects, nil
}

// IsLocalCopyValid checks whether a local file matches the expected object in size and MD5 checksum.
// If the object was uploaded via multipart and lacks a standard MD5, it attempts to verify via custom metadata.
//
// Returns true if the local file is a valid, verified copy of the object.
func (cds *CasperDataStore) IsLocalCopyValid(source ObjectURI, localFilePath string) (bool, error) {
	fileInfo, err := os.Stat(localFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	headResponse, err := cds.HeadObject(source)
	if err != nil {
		return false, fmt.Errorf("failed to get object metadata: %w", err)
	}
	objectMd5 := headResponse.ContentMd5
	objectLength := headResponse.ContentLength

	if objectLength != nil && fileInfo.Size() != *objectLength {
		cds.logger.Warnf("File size mismatch for %s: expected %d, got %d",
			localFilePath, *objectLength, fileInfo.Size())
		// File size mismatch should always return false
		return false, nil
	}

	if objectMd5 == nil && headResponse.OpcMultipartMd5 != nil && isMultipartMd5(*headResponse.OpcMultipartMd5) {
		cds.logger.Infof("Detected multipart upload for %s", source.ObjectName)

		if realMd5, ok := headResponse.OpcMeta["md5"]; ok && realMd5 != "" {
			cds.logger.Infof("Using MD5 from metadata for %s: %s", source.ObjectName, realMd5)
			objectMd5 = &realMd5
		} else {
			cds.logger.Warnf("No MD5 in metadata for multipart object %s; skipping integrity check", source.ObjectName)
			return true, nil
		}
	}

	file, err := os.Open(localFilePath)
	if err != nil {
		return false, err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			cds.logger.Warnf("Failed to close file %s: %v", localFilePath, err)
		}
	}(file)

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, err
	}

	localMd5 := base64.StdEncoding.EncodeToString(hash.Sum(nil))
	if *objectMd5 == localMd5 {
		return true, nil
	}

	cds.logger.Warnf("MD5 mismatch for %s: expected %s, got %s",
		localFilePath, *objectMd5, localMd5)

	return false, nil
}

// isMultipartMd5 determines if the given object MD5 string represents a multipart upload checksum.
// OCI and S3 multipart MD5s often take the form: "<base64md5>-<part count>"
func isMultipartMd5(md5 string) bool {
	parts := strings.Split(md5, "-")
	if len(parts) != 2 {
		return false
	}
	_, err := strconv.Atoi(parts[1])
	return err == nil
}
