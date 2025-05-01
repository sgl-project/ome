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
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	cas "github.com/oracle/oci-go-sdk/v65/objectstorage"
	"go.uber.org/zap"
)

/*
 * CasperDataStore used to perform data store operations with Object Storage(Casper)
 */

type CasperDataStore struct {
	CasperClient  *cas.ObjectStorageClient
	CompartmentId *string
}

func (cds *CasperDataStore) DownloadBasedOnObjectSize(source ObjectURI, target string, prefix string, sizeThresholdInMB int, downloadingChunkSize int, downloadingThread int) error {
	source.Prefix = source.ObjectName

	objectSummary, err := cds.ListObjects(source)
	if err != nil {
		return fmt.Errorf("failed to do object list: %+v", err)
	}

	if len(objectSummary) == 0 {
		return fmt.Errorf("object %s not found in object storage bucket: %s, in namespace: %s", source.ObjectName, source.BucketName, source.Namespace)
	}

	object := objectSummary[0]

	if object.Size == nil {
		fmt.Printf("Regular download %s \n", source.ObjectName)
		err = cds.Download(source, target, prefix)
	} else if *(object.Size) < (int64(sizeThresholdInMB) * int64(MB)) {
		fmt.Printf("Regular download %s, size: %d \n", source.ObjectName, *(object.Size))
		err = cds.Download(source, target, prefix)
	} else {
		fmt.Printf("Multipart download %s, size: %d \n", source.ObjectName, *(object.Size))
		err = cds.MultipartDownload(source, target, prefix, &object, int(downloadingChunkSize), int(downloadingThread))
	}

	if err != nil {
		return fmt.Errorf("failed to download object %s in object storage bucket: %s, in namespace: %s", source.ObjectName, source.BucketName, source.Namespace)
	}

	return nil
}

func (cds *CasperDataStore) Download(source ObjectURI, target string, prefix string) error {
	objectFullName := fmt.Sprintf(
		"%s/%s/%s", source.Namespace, source.BucketName, source.ObjectName)

	logger := zap.NewNop().Sugar() // Use no-op logger since we don't have one passed in

	response, err := cds.GetObject(source)
	if err != nil {
		return err
	}
	responseContent := response.Content
	defer func(responseContent io.ReadCloser) {
		err := responseContent.Close()
		if err != nil {
			logger.Warnf("[%s] Failed to close response content: %v", source.ObjectName, err)
		}
	}(responseContent)

	// Write a downloaded object to the target file
	targetFilePath := filepath.Join(target, ExtractNonPrefixObjectName(source.ObjectName, prefix))

	err = os.MkdirAll(path.Dir(targetFilePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf(
			"failed to create the directory %s under the target path %s, error: %+v",
			path.Dir(targetFilePath), target, err)
	}

	// Use a temporary file for download
	tempTargetFilePath := targetFilePath + ".temp"

	err = CopyReaderToFilePath(responseContent, tempTargetFilePath)
	if err != nil {
		logger.Errorf("[%s] Failed to write to temporary file: %v", source.ObjectName, err)
		err := os.Remove(tempTargetFilePath)
		if err != nil {
			logger.Warnf("[%s] Failed to clean up temporary file after error: %v", source.ObjectName, err)
		}
		return fmt.Errorf(
			"failed to load downloaded object %s to the target path %s, error: %+v",
			objectFullName, target, err)
	}
	logger.Infof("[%s] Successfully wrote to temporary file", source.ObjectName)

	// Verify MD5 checksum if available
	if response.ContentMd5 != nil {
		match, verifyErr := cds.VerifyFileMd5(tempTargetFilePath, response.ContentMd5, logger)
		if verifyErr != nil {
			logger.Errorf("[%s] MD5 verification failed with error: %v", source.ObjectName, verifyErr)
			err := os.Remove(tempTargetFilePath)
			if err != nil {
				logger.Warnf("[%s] Failed to clean up temporary file after verification error: %v", source.ObjectName, err)
			}
			return fmt.Errorf("failed to verify MD5 checksum after download: %w", verifyErr)
		}

		if !match {
			logger.Errorf("[%s] MD5 checksum mismatch - downloaded file is corrupt", source.ObjectName)
			err := os.Remove(tempTargetFilePath)
			if err != nil {
				logger.Warnf("[%s] Failed to clean up corrupt temporary file: %v", source.ObjectName, err)
			}
			return fmt.Errorf("MD5 checksum verification failed after download for %s", objectFullName)
		}
		logger.Infof("[%s] MD5 verification successful", source.ObjectName)
	} else {
		logger.Warnf("[%s] No MD5 checksum available for verification", source.ObjectName)
	}

	// Only move a file to the final location after successful verification
	if err = os.Rename(tempTargetFilePath, targetFilePath); err != nil {
		logger.Errorf("[%s] Failed to rename temporary file to final location: %v", source.ObjectName, err)
		return fmt.Errorf("failed to rename temporary file after verification: %w", err)
	}
	logger.Infof("[%s] Download completed successfully", source.ObjectName)

	return nil
}

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
		// When source is pure string content which needs to be uploaded
		putObjectBody = io.NopCloser(strings.NewReader(source))
		tmp := int64(len(source))
		uploadObjectSize = &tmp
	}

	putObjectRequest := cas.PutObjectRequest{
		NamespaceName: &target.Namespace,
		BucketName:    &target.BucketName,
		ObjectName:    &target.ObjectName,
		ContentLength: uploadObjectSize,
		PutObjectBody: putObjectBody,
	}
	// Make the put request to Casper
	response, err := cds.CasperClient.PutObject(context.Background(), putObjectRequest)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"failed to put object %q with response %+v: %s",
			objectFullName,
			response,
			err.Error())
	}
	return nil
}

func (cds *CasperDataStore) HeadObject(target ObjectURI) (*cas.HeadObjectResponse, error) {
	if target.Namespace == "" {
		namespace, err := cds.GetNamespace()
		if err != nil {
			return nil, fmt.Errorf("error head object due to no namespace found: %+v", err)
		}
		target.Namespace = *namespace
	}

	objectFullName := fmt.Sprintf(
		"%s/%s/%s", target.Namespace, target.BucketName, target.ObjectName)
	headObjectRequest := cas.HeadObjectRequest{
		NamespaceName: &target.Namespace,
		BucketName:    &target.BucketName,
		ObjectName:    &target.ObjectName,
	}

	response, err := cds.CasperClient.HeadObject(context.Background(), headObjectRequest)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"failed to head object %q with response %+v: %s",
			objectFullName,
			response,
			err.Error())
	}
	return &response, nil
}

func (cds *CasperDataStore) GetObject(source ObjectURI) (*cas.GetObjectResponse, error) {
	if source.Namespace == "" {
		namespace, err := cds.GetNamespace()
		if err != nil {
			return nil, fmt.Errorf("error get object due to no namespace found: %+v", err)
		}
		source.Namespace = *namespace
	}

	objectFullName := fmt.Sprintf(
		"%s/%s/%s", source.Namespace, source.BucketName, source.ObjectName)

	getObjectRequest := cas.GetObjectRequest{
		NamespaceName: &source.Namespace,
		BucketName:    &source.BucketName,
		ObjectName:    &source.ObjectName,
	}
	response, err := cds.CasperClient.GetObject(context.Background(), getObjectRequest)

	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"failed to download object %s with response %+v: %+v",
			objectFullName,
			response,
			err)
	}

	return &response, nil
}

func (cds *CasperDataStore) GetNamespace() (*string, error) {
	getNamespaceClient := cas.GetNamespaceRequest{}
	if cds.CompartmentId != nil {
		getNamespaceClient.CompartmentId = cds.CompartmentId
	}

	response, err := cds.CasperClient.GetNamespace(context.Background(), getNamespaceClient)
	if err != nil || response.RawResponse == nil || response.RawResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting casper namespace: %+v", err)
	}

	return response.Value, nil
}

func (cds *CasperDataStore) ListObjects(target ObjectURI) ([]cas.ObjectSummary, error) {
	if target.Namespace == "" {
		namespace, err := cds.GetNamespace()
		if err != nil {
			return nil, fmt.Errorf("error list objects due to no namespace found: %+v", err)
		}
		target.Namespace = *namespace
	}

	listObjectsRequest := cas.ListObjectsRequest{
		NamespaceName: &target.Namespace,
		BucketName:    &target.BucketName,
		Prefix:        &target.Prefix, //Virtual folder name within bucket
		Fields:        common.String("name,size,md5"),
	}

	var allObjects []cas.ObjectSummary
	page := 0
	for {
		response, err := cds.CasperClient.ListObjects(context.Background(), listObjectsRequest)
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

func (cds *CasperDataStore) ObjectExists(logger *zap.SugaredLogger, source ObjectURI, target string, objectMd5 *string, objectLength *int64, prefix string) (bool, error) {
	var err error
	targetFilePath := filepath.Join(target, ExtractNonPrefixObjectName(source.ObjectName, prefix))
	fileInfo, err := os.Stat(targetFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, err
	}

	if objectMd5 == nil || objectLength == nil {
		headResponse, err := cds.HeadObject(source)
		if err != nil {
			return false, fmt.Errorf("failed to get object metadata: %w", err)
		}
		objectMd5 = headResponse.ContentMd5
		objectLength = headResponse.ContentLength
	}

	if objectLength != nil && fileInfo.Size() != *objectLength {
		logger.Warnf("File size mismatch for %s: expected %d, got %d",
			targetFilePath, *objectLength, fileInfo.Size())
		return false, nil
	}

	if objectMd5 == nil {
		logger.Warnf("No MD5 available for %s, cannot verify integrity", source.ObjectName)
		return false, nil
	}

	// For multipart uploads that have a special MD5 format
	if strings.Contains(*objectMd5, "==-") {
		matched, err := multipartMd5Matched(targetFilePath, objectMd5, logger)
		if err != nil {
			logger.Errorf("Failed to verify multipart MD5 for %s: %v", targetFilePath, err)
			// Propagate the error to the caller so it can be properly handled
			return false, fmt.Errorf("MD5 verification error: %w", err)
		}

		if matched {
			logger.Infof("Multipart MD5 matched for %s", source.ObjectName)
			return true, nil
		}

		logger.Warnf("Multipart MD5 mismatch for %s", source.ObjectName)
		return false, nil
	}

	file, err := os.Open(targetFilePath)
	if err != nil {
		return false, fmt.Errorf("failed to open file for MD5 verification: %w", err)
	}

	defer func() {
		if err := file.Close(); err != nil {
			logger.Errorf("Failed to close file after MD5 verification: %v", err)
		}
	}()

	fileMd5 := md5.New()
	if _, err := io.Copy(fileMd5, file); err != nil {
		return false, fmt.Errorf("failed to calculate MD5 hash: %w", err)
	}

	calculatedMd5 := base64.StdEncoding.EncodeToString(fileMd5.Sum(nil))
	if *objectMd5 == calculatedMd5 {
		logger.Infof("MD5 hash matched for %s", source.ObjectName)
		return true, nil
	}

	logger.Warnf("MD5 hash mismatch for %s: expected %s, got %s",
		source.ObjectName, *objectMd5, calculatedMd5)
	return false, nil
}
