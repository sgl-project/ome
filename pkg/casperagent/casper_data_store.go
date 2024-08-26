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

	response, err := cds.GetObject(source)
	if err != nil {
		return err
	}
	responseContent := response.Content
	defer func(responseContent io.ReadCloser) {
		err := responseContent.Close()
		if err != nil {
			fmt.Printf("failed to close the response content, error: %+v", err)
		}
	}(responseContent)

	// Write downloaded object to the target file
	targetFilePath := filepath.Join(target, ExtractNonPrefixObjectName(source.ObjectName, prefix))

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

	var putObjectBody io.ReadCloser = nil
	var uploadObjectSize *int64 = nil

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
			return false, err
		}
		objectMd5 = headResponse.ContentMd5
		objectLength = headResponse.ContentLength
	}

	if objectLength != nil && fileInfo.Size() != *objectLength {
		return false, nil
	}

	if objectMd5 == nil {
		return false, nil
	}

	var finalMd5 string
	var matched bool
	if strings.Contains(*objectMd5, "==-") {
		matched, err = multipartMd5Matched(targetFilePath, objectMd5, logger)
		if err != nil {
			logger.Infof("Failed to get multipart md5 for %s, error: %s", targetFilePath, err)
		}

		if matched {
			logger.Infof("multipart md5 matched, source %s, target:%s, finalMd5: %s", source.ObjectName, targetFilePath, finalMd5)
			return true, nil
		}

		return false, nil
	}

	file, err := os.Open(targetFilePath)
	if err != nil {
		return false, err
	}

	defer func() {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}()

	fileMd5 := md5.New()
	if _, err := io.Copy(fileMd5, file); err != nil {
		return false, err
	}

	if *objectMd5 == base64.StdEncoding.EncodeToString(fileMd5.Sum(nil)) {
		return true, nil
	}

	return false, nil
}
