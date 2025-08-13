package ociobjectstore

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage/transfer"
)

const (
	DefaultMultipartUploadFilePartSize = 128 * 1024 * 1024 // 128MB
)

func (cds *OCIOSDataStore) prepareMultipartUploadRequest(target ObjectURI, chunkSizeInMB int, uploadThreads int) (*transfer.UploadRequest, error) {
	if target.Namespace == "" {
		namespace, err := cds.GetNamespace()
		if err != nil {
			return nil, fmt.Errorf("error upload object due to no namespace found: %+v", err)
		}
		target.Namespace = *namespace
	}

	uploadRequest := transfer.UploadRequest{
		NamespaceName:                       common.String(target.Namespace),
		BucketName:                          common.String(target.BucketName),
		ObjectName:                          common.String(target.ObjectName),
		PartSize:                            common.Int64(int64(chunkSizeInMB) * int64(MB)),
		NumberOfGoroutines:                  common.Int(uploadThreads),
		ObjectStorageClient:                 cds.Client,
		EnableMultipartChecksumVerification: common.Bool(true),
		Metadata:                            target.Metadata,
	}
	return &uploadRequest, nil
}

func (cds *OCIOSDataStore) MultipartStreamUpload(streamReader io.ReadCloser, target ObjectURI, chunkSizeInMB int, uploadThreads int) error {
	uploadRequest, err := cds.prepareMultipartUploadRequest(target, chunkSizeInMB, uploadThreads)
	if err != nil {
		return err
	}
	cds.adjustMetadataForStreamUpload(uploadRequest, streamReader)

	callBack := func(multiPartUploadPart transfer.MultiPartUploadPart) {
		if multiPartUploadPart.Err == nil {
			fmt.Printf("Part: %d is uploaded for object %s.\n", multiPartUploadPart.PartNum, target.ObjectName)
		}
	}
	uploadRequest.CallBack = callBack

	uploadManager := transfer.NewUploadManager()
	req := transfer.UploadStreamRequest{
		UploadRequest: *uploadRequest,
		StreamReader:  streamReader,
	}
	_, err = uploadManager.UploadStream(context.Background(), req)
	// streaming multipart upload is not resumable
	if err != nil {
		return err
	}
	return nil
}

func (cds *OCIOSDataStore) MultipartFileUpload(filePath string, target ObjectURI, chunkSizeInMB int, uploadThreads int) error {
	uploadRequest, err := cds.prepareMultipartUploadRequest(target, chunkSizeInMB, uploadThreads)
	if err != nil {
		return err
	}

	err = cds.adjustMetadataForFileUpload(uploadRequest, filePath)
	if err != nil {
		cds.logger.Warnf("Failed to adjust metadata for file upload: %v", err)
	}

	callBack := func(multiPartUploadPart transfer.MultiPartUploadPart) {
		if multiPartUploadPart.Err == nil {
			cds.logger.Infof("Part: %d / %d is uploaded for object %s.", multiPartUploadPart.PartNum, multiPartUploadPart.TotalParts, target.ObjectName)
			// refer following fmt to get each part opc-md5 res.
			// fmt.Printf("and this part opcMD5(64BasedEncoding) is: %s.\n", *multiPartUploadPart.OpcMD5 )
		}
	}
	uploadRequest.CallBack = callBack

	uploadManager := transfer.NewUploadManager()
	req := transfer.UploadFileRequest{
		UploadRequest: *uploadRequest,
		FilePath:      filePath,
	}
	resp, err := uploadManager.UploadFile(context.Background(), req)
	if err != nil {
		cds.logger.Errorf("Failed to upload file %s: %+v", filePath, err)
		// file multipart upload is resumable
		if resp.MultipartUploadResponse != nil && resp.IsResumable() {
			resp, err = uploadManager.ResumeUploadFile(context.Background(), *resp.MultipartUploadResponse.UploadID)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func (cds *OCIOSDataStore) adjustMetadataForFileUpload(uploadRequest *transfer.UploadRequest, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file stats for %s: %w", filePath, err)
	}
	fileSize := fi.Size()

	partSize := common.Int64(DefaultMultipartUploadFilePartSize)
	if uploadRequest.PartSize != nil {
		partSize = uploadRequest.PartSize
	}

	// Check if multipart upload is not allowed or file size is less than part size
	if (uploadRequest.AllowMultipartUploads != nil && !*uploadRequest.AllowMultipartUploads) || fileSize <= *partSize {
		// Update metadata map to remove "opc-meta-" prefix from keys: since single part upload (UploadFilePutObject) will have metadata keys attached with "opc-meta-" prefix automatically
		uploadRequest.Metadata = RemoveOpcMetaPrefix(uploadRequest.Metadata)
		cds.logger.Debugf("File size %d is less than or equal to part size %d, removed 'opc-meta-' prefix from metadata keys: %v", fileSize, *partSize, uploadRequest.Metadata)
	}
	return nil
}

func (cds *OCIOSDataStore) adjustMetadataForStreamUpload(uploadRequest *transfer.UploadRequest, streamReader io.ReadCloser) {
	if streamReader == nil {
		return
	}
	if IsReaderEmpty(streamReader) {
		// Update metadata map to remove "opc-meta-" prefix from keys:
		//   since single part upload (UploadFilePutObject) which used when it comes to empty stream will have metadata keys attached with "opc-meta-" prefix automatically
		uploadRequest.Metadata = RemoveOpcMetaPrefix(uploadRequest.Metadata)
		cds.logger.Debugf("Stream is empty, removed 'opc-meta-' prefix from metadata keys: %v", uploadRequest.Metadata)
	}
	return
}
