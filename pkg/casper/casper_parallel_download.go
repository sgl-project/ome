package casper

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

type ChunkUnit int

const (
	MB             ChunkUnit = 1000000
	maxPartRetries int       = 3
)

// PrepareDownloadPart holds just the info needed to construct a GetObjectRequest at download time
// (to avoid signing requests too early)
type PrepareDownloadPart struct {
	namespace string
	bucket    string
	object    string
	byteRange string
	offset    int64
	partNum   int
	size      int64
}

// DownloadedPart contains the data downloaded from object storage and the body part info
type DownloadedPart struct {
	size     int64
	partBody []byte
	offset   int64
	partNum  int
	err      error
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

// MultipartDownload used to download big file, or the download will timeout
func (cds *CasperDataStore) MultipartDownload(source ObjectURI, target string, opts ...DownloadOption) error {
	downloadOpts, err := applyDownloadOptions(opts...)
	if err != nil {
		return fmt.Errorf("failed to apply download options: %w", err)
	}

	if source.Namespace == "" {
		namespace, err := cds.GetNamespace()
		if err != nil {
			return fmt.Errorf("error list objects due to no namespace found: %+v", err)
		}
		source.Namespace = *namespace
	}

	objects, err := cds.ListObjects(source)
	if err != nil {
		return err
	}

	// Filter for exact object name match
	var exactMatches []objectstorage.ObjectSummary
	for _, obj := range objects {
		if obj.Name != nil && *obj.Name == source.ObjectName {
			exactMatches = append(exactMatches, obj)
		}
	}
	if len(exactMatches) == 0 {
		return fmt.Errorf("no object found with exact name %s", source.ObjectName)
	}
	if len(exactMatches) > 1 {
		return fmt.Errorf("multiple objects found with exact name %s", source.ObjectName)
	}

	objectSummary := &exactMatches[0]

	objectSize := int(*objectSummary.Size)
	partSize := downloadOpts.ChunkSizeInMB * 1024 * 1024
	if downloadOpts.ChunkSizeInMB <= 0 {
		partSize = 4 * 1024 * 1024 // Default to 4MB chunks if not set
		cds.logger.Warnf("ChunkSizeInMB was not set or <= 0 for %s, defaulting to 4MB chunks", source.ObjectName)
	}

	threads := downloadOpts.Threads
	if threads < 1 {
		threads = 16
	}

	cds.logger.Infof("[%s] Preparing multipart download: size=%d bytes, chunk size=%d bytes, threads=%d",
		source.ObjectName, objectSize, partSize, threads)

	totalParts := objectSize / partSize
	if objectSize%partSize != 0 {
		totalParts++
	}

	prepareDownloadParts := splitToParts(totalParts, partSize, objectSize, source)
	downloadedParts := cds.multipartDownload(context.Background(), threads, prepareDownloadParts)

	// Compute the relative local file path by removing the first two path segments (vendor/model)
	var targetFilePath string
	if downloadOpts.UseBaseNameOnly {
		targetFilePath = filepath.Join(target, ObjectBaseName(source.ObjectName))
	} else if downloadOpts.StripPrefix {
		targetFilePath = filepath.Join(target, TrimObjectPrefix(source.ObjectName, downloadOpts.PrefixToStrip))
	} else if downloadOpts.JoinWithTailOverlap {
		targetFilePath = JoinWithTailOverlap(target, source.ObjectName)
	} else {
		targetFilePath = filepath.Join(target, source.ObjectName)
	}
	tempTargetFilePath := targetFilePath + ".temp"

	// Ensure target directory exists
	targetDir := filepath.Dir(targetFilePath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %v", targetDir, err)
	}

	// Clean up any existing temporary file
	os.Remove(tempTargetFilePath)

	// Create a new temporary file
	tmpFile, err := os.Create(tempTargetFilePath)
	if err != nil {
		return err
	}

	// Use a file closure flag to avoid double-closing the file
	fileClosed := false
	defer func(tmpFile *os.File) {
		// Only close if not already closed
		if !fileClosed {
			err := tmpFile.Close()
			if err != nil {
				cds.logger.Warnf("[%s] Failed to close temporary file: %v", source.ObjectName, err)
			}
		}
	}(tmpFile)

	startTime := time.Now()
	for part := range downloadedParts {
		if part.err != nil {
			err := os.Remove(tempTargetFilePath)
			if err != nil {
				cds.logger.Warnf("[%s] Failed to clean up temporary file after error: %v", source.ObjectName, err)
			}
			return fmt.Errorf("error downloading part %d: %v", part.partNum, part.err)
		}

		// Check part size matches expected size
		if int64(len(part.partBody)) != part.size {
			cds.logger.Warnf("[%s] Part %d size mismatch: expected %d bytes, got %d bytes",
				source.ObjectName, part.partNum, part.size, len(part.partBody))
		}

		// Write the part to the temp file at the correct offset
		_, err := tmpFile.WriteAt(part.partBody, part.offset)
		if err != nil {
			os.Remove(tempTargetFilePath)
			return fmt.Errorf("failed to write part %d at offset %d: %v", part.partNum, part.offset, err)
		}

		// Free up memory by clearing the part data
		part.partBody = nil
	}

	// Ensure all data is flushed to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file to disk: %v", err)
	}

	// Close the file explicitly before renaming
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %v", err)
	}
	// Mark as closed to prevent deferred function from trying to close again
	fileClosed = true

	// Rename the temporary file to the final target path
	if err := os.Rename(tempTargetFilePath, targetFilePath); err != nil {
		// Try to clean up the temp file if rename fails
		cleanupErr := os.Remove(tempTargetFilePath)
		if cleanupErr != nil {
			cds.logger.Warnf("[%s] Failed to clean up temporary file after rename error: %v",
				source.ObjectName, cleanupErr)
		}
		return fmt.Errorf("failed to rename temporary file to target: %v", err)
	}

	// Double-check the final file size
	fileInfo, err := os.Stat(targetFilePath)
	if err != nil {
		cds.logger.Warnf("[%s] Failed to stat final file: %v", source.ObjectName, err)
	} else if fileInfo.Size() != int64(objectSize) {
		cds.logger.Warnf("[%s] Final file size mismatch: expected %d bytes, got %d bytes",
			source.ObjectName, objectSize, fileInfo.Size())
	}

	duration := time.Since(startTime)
	speedMBs := float64(objectSize) / 1024.0 / 1024.0 / duration.Seconds()
	cds.logger.Infof("[%s] Multipart download completed in %.2fs (%.2f MB/s)", source.ObjectName, duration.Seconds(), speedMBs)
	cds.logger.Infof("[%s] Multipart download completed successfully", source.ObjectName)
	return nil
}

// splitToParts splits the file to the partSize and builds a new struct to prepare for multipart download
func splitToParts(totalParts, partSize, objectSize int, source ObjectURI) chan *PrepareDownloadPart {
	prepareDownloadParts := make(chan *PrepareDownloadPart)
	go func() {
		defer func() {
			close(prepareDownloadParts)
		}()

		for part := 0; part < totalParts; part++ {
			start := int64(part * partSize)
			// Calculate end position (inclusive for HTTP Range header)
			// Note: HTTP Range is inclusive of both start and end bytes
			end := int64(math.Min(float64((part+1)*partSize-1), float64(objectSize-1)))

			// Ensure we're not requesting beyond file size
			if start >= int64(objectSize) {
				break
			}

			// Format as "bytes=start-end" for HTTP Range header
			bytesRange := strconv.FormatInt(start, 10) + "-" + strconv.FormatInt(end, 10)

			part := PrepareDownloadPart{
				namespace: source.Namespace,
				bucket:    source.BucketName,
				object:    source.ObjectName,
				byteRange: "bytes=" + bytesRange,
				offset:    start,
				partNum:   part,
				// Corrected size calculation for inclusive ranges
				size: end - start + 1,
			}

			prepareDownloadParts <- &part
		}
	}()

	return prepareDownloadParts
}

func (cds *CasperDataStore) multipartDownload(ctx context.Context, downloadThreads int, prepareDownloadParts chan *PrepareDownloadPart) chan *DownloadedPart {
	result := make(chan *DownloadedPart)

	var wg sync.WaitGroup
	wg.Add(downloadThreads)

	for i := 0; i < downloadThreads; i++ {
		go func() {
			cds.downloadFilePart(ctx, prepareDownloadParts, result)
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
func (cds *CasperDataStore) downloadFilePart(ctx context.Context, prepareDownloadParts chan *PrepareDownloadPart, result chan *DownloadedPart) {
	for part := range prepareDownloadParts {
		var lastErr error
		var content []byte
		start := time.Now()
		for attempt := 1; attempt <= maxPartRetries; attempt++ {
			resp, err := cds.Client.GetObject(ctx, objectstorage.GetObjectRequest{
				NamespaceName: common.String(part.namespace),
				BucketName:    common.String(part.bucket),
				ObjectName:    common.String(part.object),
				Range:         common.String(part.byteRange),
			})
			if err != nil {
				cds.logger.Warnf("Error getting object for part %d (attempt %d/%d): %s", part.partNum, attempt, maxPartRetries, err)
				lastErr = err
			} else {
				// Successfully got the object response
				var readErr, closeErr error
				content, readErr = io.ReadAll(resp.Content)
				closeErr = resp.Content.Close() // Close immediately

				if readErr != nil {
					cds.logger.Warnf("Error reading response body for part %d (attempt %d/%d): %s", part.partNum, attempt, maxPartRetries, readErr)
					lastErr = readErr // Report read error
				} else if closeErr != nil {
					cds.logger.Warnf("Error closing response body for part %d (attempt %d/%d): %s", part.partNum, attempt, maxPartRetries, closeErr)
					lastErr = closeErr // Report close error if read was ok
				} else {
					// Success reading and closing
					lastErr = nil // Clear any potential previous attempt's error
					break         // Exit retry loop on success
				}
			}
			if attempt < maxPartRetries && lastErr != nil {
				time.Sleep(2 * time.Second)
			}
		}
		duration := time.Since(start)
		speedMBs := float64(len(content)) / 1024.0 / 1024.0 / duration.Seconds()
		if lastErr == nil {
			cds.logger.Debugf("[Chunk %d] Downloaded %d bytes in %.2fs (%.2f MB/s) for file %s", part.partNum, len(content), duration.Seconds(), speedMBs, part.object)
		}
		if lastErr != nil {
			// All retries failed for this part
			result <- &DownloadedPart{
				err:     lastErr,
				partNum: part.partNum,
				offset:  part.offset,
			}
			continue
		}
		// Success: send the downloaded part
		result <- &DownloadedPart{
			size:     int64(len(content)),
			partBody: content,
			offset:   part.offset,
			partNum:  part.partNum,
		}
	}
}

func (cds *CasperDataStore) DownloadWithMultiThreads(downloadThreads int, filesToDownload chan *FileToDownload) chan *DownloadedFile {
	cds.logger.Infof("Download objects with %d threads", downloadThreads)
	result := make(chan *DownloadedFile)

	var wg sync.WaitGroup
	wg.Add(downloadThreads)

	for i := 0; i < downloadThreads; i++ {
		go func() {
			cds.downloadFiles(filesToDownload, result)
			wg.Done()
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	return result
}

func (cds *CasperDataStore) downloadFiles(filesToDownload chan *FileToDownload, result chan *DownloadedFile) {
	for fileToDownload := range filesToDownload {
		err := cds.downloadFile(fileToDownload)
		downloadedFile := &DownloadedFile{
			source:         fileToDownload.source,
			targetFilePath: fileToDownload.targetFilePath,
		}
		if err != nil {
			cds.logger.Errorf("Error in downloading, err: %s ", err)
			downloadedFile.Err = err
		}

		result <- downloadedFile
	}
}

func (cds *CasperDataStore) downloadFile(fileToDownload *FileToDownload) error {
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
			cds.logger.Errorf("Failed to close response content: %+v", err)
		}
	}(responseContent)

	if response.ContentLength == nil {
		cds.logger.Infof("Download %s", fileToDownload.source.ObjectName)
	} else {
		cds.logger.Infof("Download %s, size: %d", fileToDownload.source.ObjectName, *(response.ContentLength))
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
