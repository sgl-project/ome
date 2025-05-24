package casper

import (
	"context"
	"fmt"
	"io"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage/transfer"
)

func (cds *CasperDataStore) prepareMultipartUploadRequest(target ObjectURI, chunkSizeInMB int, uploadThreads int) (*transfer.UploadRequest, error) {
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
	}
	return &uploadRequest, nil
}

func (cds *CasperDataStore) MultipartStreamUpload(streamReader io.ReadCloser, target ObjectURI, chunkSizeInMB int, uploadThreads int) error {
	uploadRequest, err := cds.prepareMultipartUploadRequest(target, chunkSizeInMB, uploadThreads)
	if err != nil {
		return err
	}

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

func (cds *CasperDataStore) MultipartFileUpload(filePath string, target ObjectURI, chunkSizeInMB int, uploadThreads int) error {
	uploadRequest, err := cds.prepareMultipartUploadRequest(target, chunkSizeInMB, uploadThreads)
	if err != nil {
		return err
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
	// file multipart upload is resumable
	if err != nil {
		if resp.IsResumable() {
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
