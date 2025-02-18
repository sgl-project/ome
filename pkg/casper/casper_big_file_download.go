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
	"sync"

	"bitbucket.oci.oraclecorp.com/genaicore/ome/pkg/logging"
	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

type ChunkUnit int

const (
	MB                            ChunkUnit = 1000000
	SMALL_CHUNK_SIZE_IN_BYTE      int       = 25 * 1000000
	SMALL_CHUNK_FILE_BUFFER_SIZE  int       = 100000
	SMALL_CHUNK_SIZE_50MB_IN_BYTE int       = 50 * 1000000
	LARGE_CHUNK_SIZE_IN_BYTE      int       = 500 * 1024 * 1024
	LARGE_CHUNK_FILE_BUFFER_SIZE  int       = 65536
)

// PrepareDownloadPart wraps an GetObjectRequest with split part related info
type PrepareDownloadPart struct {
	request *objectstorage.GetObjectRequest
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

func (cds *CasperDataStore) PrepareDownload(source ObjectURI, target string, excludeBucketPath bool) (*FileToDownload, error) {
	var targetFilePath string
	if excludeBucketPath {
		targetFilePath = filepath.Join(target, ExtractPureObjectName(source.ObjectName))
	} else {
		targetFilePath = filepath.Join(target, source.ObjectName)
	}

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

// MultipartDownload used to download big file, or the download will timeout
func (cds *CasperDataStore) MultipartDownload(source ObjectURI, target string, excludeBucketPath bool, objectSummary *objectstorage.ObjectSummary, chunkSizeInMB int, downloadThreads int) error {
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

	totalParts := objectSize / partSize
	if objectSize%partSize != 0 {
		totalParts++
	}

	prepareDownloadParts := splitToParts(totalParts, partSize, objectSize, source)
	downloadedParts := multipartDownload(context.Background(), cds.Client, downloadThreads, prepareDownloadParts)

	var targetFilePath string
	if excludeBucketPath {
		targetFilePath = filepath.Join(target, ExtractPureObjectName(source.ObjectName))
	} else {
		targetFilePath = filepath.Join(target, source.ObjectName)
	}
	tempTargetFilePath := targetFilePath + ".temp"

	err := os.Remove(tempTargetFilePath)
	if err != nil && !os.IsNotExist(err) {
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
		err = tmpFile.Close()
		if err != nil {
			cds.logger.Errorf("Failed to close temp file: %+v", err)
		}
	}(tmpFile)
	if err != nil {
		return err
	}

	for part := range downloadedParts {
		if part.err != nil {
			err = os.Remove(tempTargetFilePath)
			if err != nil && !os.IsNotExist(err) {
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

	return nil
}

// bufferSizeInByte must be divisible by partSizeInByte
func calculateMultipartMd5(partSizeInByte int, bufferSizeInByte int, targetFilePath string, logger logging.Interface) (string, error) {
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
	for eof == false {
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
			_, err := io.Copy(md5Calculator, bytes.NewReader(buf[:bytesRead]))
			if err != nil {
				return nil, false
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
func multipartMd5Matched(targetFilePath string, objectMd5 *string, logger logging.Interface) (bool, error) {
	chunkSizes := []int{SMALL_CHUNK_SIZE_IN_BYTE, SMALL_CHUNK_SIZE_50MB_IN_BYTE, LARGE_CHUNK_SIZE_IN_BYTE}
	bufferSize := []int{SMALL_CHUNK_FILE_BUFFER_SIZE, SMALL_CHUNK_FILE_BUFFER_SIZE, LARGE_CHUNK_FILE_BUFFER_SIZE}
	var err error
	var finalMd5 string
	for i, chunkSize := range chunkSizes {
		finalMd5, err = calculateMultipartMd5(chunkSize, bufferSize[i], targetFilePath, logger)
		if err == nil && *objectMd5 == finalMd5 {
			return true, nil
		}
	}

	return false, err
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
				request: &objectstorage.GetObjectRequest{
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

func multipartDownload(ctx context.Context, osClient *objectstorage.ObjectStorageClient, downloadThreads int, prepareDownloadParts chan *PrepareDownloadPart) chan *DownloadedPart {
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
func downloadFilePart(ctx context.Context, osClient *objectstorage.ObjectStorageClient, prepareDownloadParts chan *PrepareDownloadPart, result chan *DownloadedPart) {
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

func (cds *CasperDataStore) DownloadWithMultiThreads(downloadThreads int, logger logging.Interface, filesToDownload chan *FileToDownload) chan *DownloadedFile {
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

func (cds *CasperDataStore) DownloadFiles(filesToDownload chan *FileToDownload, logger logging.Interface, result chan *DownloadedFile) {
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

func (cds *CasperDataStore) DownloadFile(fileToDownload *FileToDownload, logger logging.Interface) error {
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

	// Write downloaded object to the target file
	err = CopyReaderToFilePath(responseContent, fileToDownload.targetFilePath)
	if err != nil {
		return fmt.Errorf(
			"failed to download object %s to the target path %s, error: %+v",
			objectFullName, fileToDownload.targetFilePath, err)
	}
	return nil
}

func (cds *CasperDataStore) FilterObjectsMultiThreads(threads int, logger logging.Interface, objectStoreUri *ObjectURI, target string, objectSummaries chan objectstorage.ObjectSummary) chan objectstorage.ObjectSummary {
	logger.Infof("Filter objects with %d threads", threads)
	result := make(chan objectstorage.ObjectSummary)

	var wg sync.WaitGroup
	wg.Add(threads)

	for i := 0; i < threads; i++ {
		go func() {
			cds.FilterObjects(objectSummaries, logger, objectStoreUri, target, result)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	return result
}

func (cds *CasperDataStore) FilterObjects(objectSummaries chan objectstorage.ObjectSummary, logger logging.Interface, objectStoreUri *ObjectURI, target string, result chan objectstorage.ObjectSummary) {
	for object := range objectSummaries {
		objectURI := ObjectURI{
			Namespace:  objectStoreUri.Namespace,
			BucketName: objectStoreUri.BucketName,
			ObjectName: *object.Name,
		}

		exist, err := cds.ObjectExists(logger, objectURI, target, object.Md5, object.Size)
		if err != nil {
			logger.Errorf("Error when check object existence: %s", err)
			panic(err)
		}

		if exist {
			logger.Infof("%s already exists with md5 check.", objectURI.ObjectName)
		} else {
			result <- object
		}
	}
}
