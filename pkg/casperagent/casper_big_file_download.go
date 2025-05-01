package casper

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/oracle/oci-go-sdk/v65/common"
	cas "github.com/oracle/oci-go-sdk/v65/objectstorage"
	"go.uber.org/zap"
)

type ChunkUnit int

const (
	MB                       ChunkUnit = 1000000
	SmallChunkSizeInByte     int       = 25 * 1000000
	SmallChunkFileBufferSize int       = 100000
	SmallChunkSize50mbInByte int       = 50 * 1000000
	LargeChunkSizeInByte     int       = 500 * 1024 * 1024
	LargeChunkFileBufferSize int       = 65536
)

// PrepareDownloadPart wraps a GetObjectRequest with split part related info
type PrepareDownloadPart struct {
	request *cas.GetObjectRequest
	offset  int64
	partNum int
	size    int64
}

// DownloadedPart contains the data downloaded from object storage and the body part info
type DownloadedPart struct {
	size     int64
	partBody []byte
	offset   int64
	partNum  int
	err      error
}

func (cds *CasperDataStore) PrepareDownload(source ObjectURI, target string, prefix string) (*FileToDownload, error) {
	targetFilePath := filepath.Join(target, ExtractNonPrefixObjectName(source.ObjectName, prefix))

	err := os.MkdirAll(path.Dir(targetFilePath), os.ModePerm)
	if err != nil {
		return &FileToDownload{}, fmt.Errorf(
			"failed to create the directory %s under the target path %s, error: %+v",
			path.Dir(targetFilePath), target, err)
	}

	return &FileToDownload{
		source:         source,
		targetFilePath: targetFilePath,
	}, nil
}

// MultipartDownload used to download a big file, or the download will timeout
func (cds *CasperDataStore) MultipartDownload(source ObjectURI, target string, prefix string, objectSummary *cas.ObjectSummary, chunkSizeInMB int, downloadThreads int) error {
	logger := zap.NewNop().Sugar() // Use no-op logger since we don't have one passed in

	if source.Namespace == "" {
		namespace, err := cds.GetNamespace()
		if err != nil {
			return fmt.Errorf("error list objects due to no namespace found: %+v", err)
		}
		source.Namespace = *namespace
	}

	if objectSummary == nil {
		objects, err := cds.ListObjects(source)
		if err != nil {
			return err
		}

		if len(objects) >= 2 {
			return fmt.Errorf("there are %d objects with the same prefix %s", len(objects), source.ObjectName)
		}

		if len(objects) == 0 {
			return fmt.Errorf("there is no object with the prefix %s", source.ObjectName)
		}

		objectSummary = &objects[0]
	}

	objectSize := int(*objectSummary.Size)
	partSize := chunkSizeInMB * int(MB)

	logger.Infof("[%s] Preparing multipart download: size=%d bytes, chunk size=%d MB, threads=%d",
		source.ObjectName, objectSize, chunkSizeInMB, downloadThreads)

	totalParts := objectSize / partSize
	if objectSize%partSize != 0 {
		totalParts++
	}

	prepareDownloadParts := splitToParts(totalParts, partSize, objectSize, source)

	downloadedParts := multipartDownload(context.Background(), cds.CasperClient, downloadThreads, prepareDownloadParts)

	targetFilePath := filepath.Join(target, ExtractNonPrefixObjectName(source.ObjectName, prefix))
	tempTargetFilePath := targetFilePath + ".temp"

	err := os.Remove(tempTargetFilePath)
	if err != nil && !os.IsNotExist(err) {
		logger.Warnf("[%s] Error removing existing temp file: %v", source.ObjectName, err)
		return err
	}

	err = os.MkdirAll(path.Dir(tempTargetFilePath), os.ModePerm)
	if err != nil {
		return fmt.Errorf(
			"failed to create the directory %s under the target path %s, error: %+v",
			path.Dir(tempTargetFilePath), target, err)
	}

	tmpFile, err := os.OpenFile(tempTargetFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer func(tmpFile *os.File) {
		err := tmpFile.Close()
		if err != nil {
			logger.Warnf("[%s] Failed to close temporary file: %v", source.ObjectName, err)
		}
	}(tmpFile)

	for part := range downloadedParts {
		if part.err != nil {
			err := os.Remove(tempTargetFilePath)
			if err != nil {
				logger.Warnf("[%s] Failed to clean up temporary file after error: %v", source.ObjectName, err)
				return err
			}
			return part.err
		}

		_, err = tmpFile.WriteAt(part.partBody, part.offset)
		if err != nil {
			return err
		}
	}

	err = os.Rename(tempTargetFilePath, targetFilePath)
	if err != nil {
		return err
	}

	// Verify MD5 checksum of the downloaded file
	if objectSummary.Md5 != nil {
		logger.Infof("[%s] Starting MD5 verification after download", source.ObjectName)
		match, err := cds.VerifyFileMd5(targetFilePath, objectSummary.Md5, logger)
		if err != nil {
			logger.Errorf("[%s] MD5 verification failed with error: %v", source.ObjectName, err)
			return fmt.Errorf("failed to verify MD5 checksum after download: %w", err)
		}

		if !match {
			logger.Errorf("[%s] MD5 checksum mismatch - downloaded file is corrupt", source.ObjectName)
			// Remove a corrupt file so it will be downloaded again on the next attempt
			err := os.Remove(targetFilePath)
			if err != nil {
				logger.Warnf("[%s] Failed to remove corrupt file: %v", source.ObjectName, err)
				return err
			}
			return fmt.Errorf("MD5 checksum verification failed after download")
		}
		logger.Infof("[%s] MD5 verification successful", source.ObjectName)
	} else {
		logger.Warnf("[%s] No MD5 checksum available for verification", source.ObjectName)
	}

	logger.Infof("[%s] Multipart download completed successfully", source.ObjectName)
	return nil
}

// VerifyFileMd5 checks if the file's MD5 matches the expected one
func (cds *CasperDataStore) VerifyFileMd5(filePath string, expectedMd5 *string, logger *zap.SugaredLogger) (bool, error) {
	if expectedMd5 == nil {
		logger.Warnf("No expected MD5 provided for %s, skipping verification", filePath)
		return true, nil
	}

	// For multipart uploads that have a special MD5 format
	if strings.Contains(*expectedMd5, "==-") {
		return multipartMd5Matched(filePath, expectedMd5, logger)
	}

	// For regular files with standard MD5
	file, err := os.Open(filePath)
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
	matched := *expectedMd5 == calculatedMd5

	if !matched {
		logger.Errorf("MD5 verification failed for %s: expected %s, got %s",
			filePath, *expectedMd5, calculatedMd5)
	} else {
		logger.Infof("MD5 verification successful for %s", filePath)
	}

	return matched, nil
}

func calculateMultipartMd5(partSizeInByte int, bufferSizeInByte int, targetFilePath string, logger *zap.SugaredLogger) (string, error) {
	file, err := os.Open(targetFilePath)
	if err != nil {
		logger.Infof("Failed to open target file:%s, error:%s", targetFilePath, err)
		return "", err
	}

	defer func() {
		if err := file.Close(); err != nil {
			panic(err)
		}
	}()

	eof := false
	allMd5Bytes := make([]byte, 0)
	var partMd5 []byte
	count := 0
	for !eof {
		partMd5, eof = calculateMd5(file, partSizeInByte, bufferSizeInByte)
		allMd5Bytes = append(allMd5Bytes, partMd5...)
		count++
	}

	fileMd5 := md5.New()
	if _, err := io.Copy(fileMd5, bytes.NewReader(allMd5Bytes)); err != nil {
		logger.Infof("Failed to compute multipart md5 for %s, error: %s", targetFilePath, err)
		return "", err
	}

	return fmt.Sprintf("%s-%d", base64.StdEncoding.EncodeToString(fileMd5.Sum(nil)), count), nil
}

func calculateMd5(file *os.File, chunkSizeInByte int, bufferSizeInByte int) ([]byte, bool) {
	md5Calculator := md5.New()
	eof := false
	buf := make([]byte, bufferSizeInByte)
	for i := 0; i < chunkSizeInByte/bufferSizeInByte; i++ {
		bytesRead, _ := file.Read(buf)
		if bytesRead > 0 {
			if _, err := io.Copy(md5Calculator, bytes.NewReader(buf[:bytesRead])); err != nil {
				// Since this function doesn't return an error, we'll just log it
				fmt.Printf("Error copying data to MD5 calculator: %v\n", err)
			}
		}

		if bytesRead < bufferSizeInByte {
			eof = true
			break
		}
	}

	return md5Calculator.Sum(nil), eof
}

// We used to upload big files with part size of 500MB, and small files with part size of 25000000.
// Now we're using 50000000 bytes for all files. This is for back compatibility
func multipartMd5Matched(targetFilePath string, objectMd5 *string, logger *zap.SugaredLogger) (bool, error) {
	chunkSizes := []int{SmallChunkSizeInByte, SmallChunkSize50mbInByte, LargeChunkSizeInByte}
	bufferSize := []int{SmallChunkFileBufferSize, SmallChunkFileBufferSize, LargeChunkFileBufferSize}

	var allErrors []string

	for i, chunkSize := range chunkSizes {
		finalMd5, err := calculateMultipartMd5(chunkSize, bufferSize[i], targetFilePath, logger)
		if err != nil {
			errMsg := fmt.Sprintf("MD5 calculation failed with chunk size %d: %v", chunkSize, err)
			logger.Warnf(errMsg)
			allErrors = append(allErrors, errMsg)
			continue
		}

		if *objectMd5 == finalMd5 {
			logger.Infof("MD5 match found using chunk size %d: %s", chunkSize, finalMd5)
			return true, nil
		}

		logger.Warnf("MD5 mismatch with chunk size %d. Expected: %s, Got: %s",
			chunkSize, *objectMd5, finalMd5)
	}

	if len(allErrors) > 0 {
		return false, fmt.Errorf("multiple MD5 calculation errors: %s", strings.Join(allErrors, "; "))
	}

	return false, fmt.Errorf("MD5 mismatch for all chunk sizes")
}

// splitToParts splits the file to the partSize and build a new struct to prepare for multipart download
func splitToParts(totalParts, partSize, objectSize int, source ObjectURI) chan *PrepareDownloadPart {
	prepareDownloadParts := make(chan *PrepareDownloadPart)
	go func() {
		defer func() {
			close(prepareDownloadParts)
		}()

		for part := 0; part < totalParts; part++ {
			start := int64(part * partSize)
			end := int64(math.Min(float64((part+1)*partSize), float64(objectSize)) - 1)
			bytesRange := strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end, 10)
			part := PrepareDownloadPart{
				request: &cas.GetObjectRequest{
					NamespaceName: common.String(source.Namespace),
					BucketName:    common.String(source.BucketName),
					ObjectName:    common.String(source.ObjectName),
					// This is the parameter where we control the download size/request
					Range: common.String("bytes=" + bytesRange),
				},
				offset:  start,
				partNum: part,
				size:    end - start,
			}

			prepareDownloadParts <- &part
		}
	}()

	return prepareDownloadParts
}

func multipartDownload(ctx context.Context, osClient *cas.ObjectStorageClient, downloadThreads int, prepareDownloadParts chan *PrepareDownloadPart) chan *DownloadedPart {
	result := make(chan *DownloadedPart)

	var wg sync.WaitGroup
	wg.Add(downloadThreads)

	for i := 0; i < downloadThreads; i++ {
		go func() {
			downloadFilePart(ctx, osClient, prepareDownloadParts, result)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	return result
}

// downloadFilePart wraps objectStorage GetObject API call
func downloadFilePart(ctx context.Context, osClient *cas.ObjectStorageClient, prepareDownloadParts chan *PrepareDownloadPart, result chan *DownloadedPart) {
	for part := range prepareDownloadParts {
		resp, err := osClient.GetObject(ctx, *part.request)
		downloadedPart := &DownloadedPart{}
		if err != nil {
			fmt.Println("Error in downloading: ", err)
			downloadedPart.err = err
		} else {
			content, _ := io.ReadAll(resp.Content)
			downloadedPart = &DownloadedPart{
				size:     int64(len(content)),
				partBody: content,
				offset:   part.offset,
				partNum:  part.partNum,
			}
		}

		result <- downloadedPart
	}
}

type FileToDownload struct {
	source         ObjectURI
	targetFilePath string
}

type DownloadedFile struct {
	source         ObjectURI
	targetFilePath string
	Err            error
}

func NewDownloadedFile(source ObjectURI, targetFilePath string) *DownloadedFile {
	return &DownloadedFile{
		source:         source,
		targetFilePath: targetFilePath,
	}
}

func (cds *CasperDataStore) DownloadWithMultiThreads(downloadThreads int, logger *zap.SugaredLogger, filesToDownload chan *FileToDownload) chan *DownloadedFile {
	logger.Infof("Download objects with %d threads", downloadThreads)
	result := make(chan *DownloadedFile)

	var wg sync.WaitGroup
	wg.Add(downloadThreads)

	for i := 0; i < downloadThreads; i++ {
		go func() {
			cds.DownloadFiles(filesToDownload, logger, result)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	return result
}

func (cds *CasperDataStore) DownloadFiles(filesToDownload chan *FileToDownload, logger *zap.SugaredLogger, result chan *DownloadedFile) {
	for fileToDownload := range filesToDownload {
		err := cds.DownloadFile(fileToDownload, logger)
		downloadedFile := &DownloadedFile{
			source:         fileToDownload.source,
			targetFilePath: fileToDownload.targetFilePath,
		}
		if err != nil {
			logger.Errorf("Error in downloading, err: %s ", err)
			downloadedFile.Err = err
		}

		result <- downloadedFile
	}
}

func (cds *CasperDataStore) DownloadFile(fileToDownload *FileToDownload, logger *zap.SugaredLogger) error {
	objectFullName := fmt.Sprintf(
		"%s/%s/%s", fileToDownload.source.Namespace, fileToDownload.source.BucketName, fileToDownload.source.ObjectName)

	response, err := cds.GetObject(fileToDownload.source)
	if err != nil {
		return err
	}
	responseContent := response.Content
	defer func(responseContent io.ReadCloser) {
		err := responseContent.Close()
		if err != nil {
			logger.Errorf("Failed to close response content: %+v", err)
		}
	}(responseContent)

	if response.ContentLength == nil {
		logger.Infof("Download %s", fileToDownload.source.ObjectName)
	} else {
		logger.Infof("Download %s, size: %d", fileToDownload.source.ObjectName, *(response.ContentLength))
	}

	// Write a downloaded object to the target file
	err = CopyReaderToFilePath(responseContent, fileToDownload.targetFilePath)
	if err != nil {
		return fmt.Errorf(
			"failed to download object %s to the target path %s, error: %+v",
			objectFullName, fileToDownload.targetFilePath, err)
	}
	return nil
}

func (cds *CasperDataStore) FilterObjectsMultiThreads(threads int, logger *zap.SugaredLogger, objectStoreUri *ObjectURI, target string, objectSummaries chan cas.ObjectSummary, prefix string) chan cas.ObjectSummary {
	logger.Infof("Filter objects with %d threads", threads)
	result := make(chan cas.ObjectSummary)

	var wg sync.WaitGroup
	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func() {
			cds.FilterObjects(objectSummaries, logger, objectStoreUri, target, result, prefix)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	return result
}

func (cds *CasperDataStore) FilterObjects(objectSummaries chan cas.ObjectSummary, logger *zap.SugaredLogger, objectStoreUri *ObjectURI, target string, result chan cas.ObjectSummary, prefix string) {
	for object := range objectSummaries {
		objectURI := ObjectURI{
			Namespace:  objectStoreUri.Namespace,
			BucketName: objectStoreUri.BucketName,
			ObjectName: *object.Name,
		}

		exist, err := cds.ObjectExists(logger, objectURI, target, object.Md5, object.Size, prefix)
		if err != nil {
			logger.Errorf("Error when checking object existence for %s: %v", objectURI.ObjectName, err)
			// Instead of panic, pass the object along for re-download
			// This is safer than failing the entire process
			logger.Warnf("Will re-download object %s due to MD5 verification error", objectURI.ObjectName)
			result <- object
			continue
		}

		if exist {
			logger.Infof("%s already exists with verified MD5 checksum", objectURI.ObjectName)
		} else {
			logger.Infof("Need to download %s (doesn't exist or MD5 mismatch)", objectURI.ObjectName)
			result <- object
		}
	}
}
