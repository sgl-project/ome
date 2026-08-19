package ociobjectstore

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

// DownloadedPart reports completion metadata for a range already written into
// the shared object-level temporary file.
type DownloadedPart struct {
	size    int64
	partNum int
	err     error
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
func (cds *OCIOSDataStore) MultipartDownload(source ObjectURI, target string, opts ...DownloadOption) (returnErr error) {
	multipartStartedAt := time.Now()
	defer func() {
		outcome := DownloadOutcomeSuccess
		if returnErr != nil {
			outcome = DownloadOutcomeError
		}
		cds.observeDownloadPhase(DownloadObservation{
			Phase:      PhaseMultipartTotal,
			Duration:   time.Since(multipartStartedAt),
			Outcome:    outcome,
			ObjectName: source.ObjectName,
			Err:        returnErr,
		})
	}()

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

	listStartedAt := time.Now()
	objects, err := cds.ListObjects(source)
	listOutcome := DownloadOutcomeSuccess
	if err != nil {
		listOutcome = DownloadOutcomeError
	}
	cds.observeDownloadPhase(DownloadObservation{
		Phase:      PhaseMultipartList,
		Duration:   time.Since(listStartedAt),
		Outcome:    listOutcome,
		ObjectName: source.ObjectName,
		Err:        err,
	})
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

	targetFilePath := ComputeTargetFilePath(source, target, &downloadOpts)
	tempTargetFilePath := targetFilePath + ".temp"

	// Ensure target directory exists
	targetDir := filepath.Dir(targetFilePath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %v", targetDir, err)
	}

	// Clean up any existing temporary file
	os.Remove(tempTargetFilePath)

	// Create one object-level temporary file. Part workers write directly to
	// disjoint offsets; the file is published only after every part succeeds.
	tmpFile, err := os.OpenFile(tempTargetFilePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	preallocateStartedAt := time.Now()
	usedFallocate, preallocateErr := preallocateFile(tmpFile, int64(objectSize))
	preallocateOutcome := DownloadOutcomeSuccess
	if preallocateErr != nil {
		preallocateOutcome = DownloadOutcomeError
	}
	cds.observeDownloadPhase(DownloadObservation{
		Phase:      PhaseModelFileAllocate,
		Duration:   time.Since(preallocateStartedAt),
		Bytes:      int64(objectSize),
		Outcome:    preallocateOutcome,
		ObjectName: source.ObjectName,
		Err:        preallocateErr,
	})
	if preallocateErr != nil {
		tmpFile.Close()
		os.Remove(tempTargetFilePath)
		return fmt.Errorf("failed to preallocate temporary file %s: %w", tempTargetFilePath, preallocateErr)
	}
	cds.logger.Debugf("[%s] Preallocated %d bytes for multipart target (fallocate=%t)", source.ObjectName, objectSize, usedFallocate)

	// Close the file before removing an unpublished temporary path. Workers are
	// always joined before this function returns.
	fileClosed := false
	filePublished := false
	defer func() {
		if !fileClosed {
			if closeErr := tmpFile.Close(); closeErr != nil {
				cds.logger.Warnf("[%s] Failed to close temporary file: %v", source.ObjectName, closeErr)
			}
		}
		if !filePublished {
			if removeErr := os.Remove(tempTargetFilePath); removeErr != nil && !os.IsNotExist(removeErr) {
				cds.logger.Warnf("[%s] Failed to remove temporary file: %v", source.ObjectName, removeErr)
			}
		}
	}()

	startTime := time.Now()
	downloadCtx, cancelDownload := context.WithCancel(context.Background())
	prepareDownloadParts := splitToParts(totalParts, partSize, objectSize, source)
	downloadedParts := cds.multipartDownload(downloadCtx, threads, prepareDownloadParts, tmpFile)
	var downloadErr error
	for part := range downloadedParts {
		if part.err != nil && downloadErr == nil {
			downloadErr = fmt.Errorf("error downloading part %d: %w", part.partNum, part.err)
			cancelDownload()
		}
	}
	cancelDownload()
	if downloadErr != nil {
		return downloadErr
	}

	// Ensure all data is flushed to disk
	syncStartedAt := time.Now()
	if err := tmpFile.Sync(); err != nil {
		cds.observeDownloadPhase(DownloadObservation{
			Phase:      PhaseModelFileSync,
			Duration:   time.Since(syncStartedAt),
			Outcome:    DownloadOutcomeError,
			ObjectName: source.ObjectName,
			Err:        err,
		})
		return fmt.Errorf("failed to sync temporary file to disk: %v", err)
	}
	cds.observeDownloadPhase(DownloadObservation{
		Phase:      PhaseModelFileSync,
		Duration:   time.Since(syncStartedAt),
		Outcome:    DownloadOutcomeSuccess,
		ObjectName: source.ObjectName,
	})

	// Close the file explicitly before renaming
	closeStartedAt := time.Now()
	if err := tmpFile.Close(); err != nil {
		cds.observeDownloadPhase(DownloadObservation{
			Phase:      PhaseModelFileClose,
			Duration:   time.Since(closeStartedAt),
			Outcome:    DownloadOutcomeError,
			ObjectName: source.ObjectName,
			Err:        err,
		})
		return fmt.Errorf("failed to close temporary file: %v", err)
	}
	cds.observeDownloadPhase(DownloadObservation{
		Phase:      PhaseModelFileClose,
		Duration:   time.Since(closeStartedAt),
		Outcome:    DownloadOutcomeSuccess,
		ObjectName: source.ObjectName,
	})
	// Mark as closed to prevent deferred function from trying to close again
	fileClosed = true

	// Rename the temporary file to the final target path
	renameStartedAt := time.Now()
	if err := os.Rename(tempTargetFilePath, targetFilePath); err != nil {
		cds.observeDownloadPhase(DownloadObservation{
			Phase:      PhaseModelFileRename,
			Duration:   time.Since(renameStartedAt),
			Outcome:    DownloadOutcomeError,
			ObjectName: source.ObjectName,
			Err:        err,
		})
		// Try to clean up the temp file if rename fails
		cleanupErr := os.Remove(tempTargetFilePath)
		if cleanupErr != nil {
			cds.logger.Warnf("[%s] Failed to clean up temporary file after rename error: %v",
				source.ObjectName, cleanupErr)
		}
		return fmt.Errorf("failed to rename temporary file to target: %v", err)
	}
	cds.observeDownloadPhase(DownloadObservation{
		Phase:      PhaseModelFileRename,
		Duration:   time.Since(renameStartedAt),
		Outcome:    DownloadOutcomeSuccess,
		ObjectName: source.ObjectName,
	})
	filePublished = true

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
	// Buffer every descriptor so cancellation of the worker pool cannot leave a
	// producer goroutine blocked on a send. A large model has only hundreds to a
	// few thousand descriptors, so the memory cost is negligible.
	prepareDownloadParts := make(chan *PrepareDownloadPart, totalParts)
	defer close(prepareDownloadParts)

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

	return prepareDownloadParts
}

func (cds *OCIOSDataStore) multipartDownload(ctx context.Context, downloadThreads int, prepareDownloadParts chan *PrepareDownloadPart, targetFile *os.File) chan *DownloadedPart {
	result := make(chan *DownloadedPart, downloadThreads)

	var wg sync.WaitGroup
	wg.Add(downloadThreads)

	for i := 0; i < downloadThreads; i++ {
		go func() {
			defer wg.Done()
			cds.downloadFilePart(ctx, prepareDownloadParts, result, targetFile)
		}()
	}

	go func() {
		wg.Wait()
		close(result)
	}()

	return result
}

// downloadFilePart wraps objectStorage GetObject API call
func (cds *OCIOSDataStore) downloadFilePart(ctx context.Context, prepareDownloadParts chan *PrepareDownloadPart, result chan *DownloadedPart, targetFile *os.File) {
	for part := range prepareDownloadParts {
		if ctx.Err() != nil {
			return
		}

		var lastErr error
		var size int64
		start := time.Now()

		for attempt := 1; attempt <= maxPartRetries; attempt++ {
			requestStartedAt := time.Now()
			resp, err := cds.Client.GetObject(ctx, objectstorage.GetObjectRequest{
				NamespaceName: common.String(part.namespace),
				BucketName:    common.String(part.bucket),
				ObjectName:    common.String(part.object),
				Range:         common.String(part.byteRange),
			})
			requestOutcome := DownloadOutcomeSuccess
			if err != nil {
				requestOutcome = DownloadOutcomeError
			}
			cds.observeDownloadPhase(DownloadObservation{
				Phase:      PhaseGetObjectRequest,
				Duration:   time.Since(requestStartedAt),
				Outcome:    requestOutcome,
				ObjectName: part.object,
				PartNumber: part.partNum,
				HasPart:    true,
				Attempt:    attempt,
				ChunkSize:  part.size,
				Err:        err,
			})
			if err != nil {
				cds.logger.Warnf("Error getting object for part %d (attempt %d/%d): %s", part.partNum, attempt, maxPartRetries, err)
				lastErr = err
			} else {
				// Stream this range directly into its disjoint offset in the shared
				// object-level temporary file. OffsetWriter uses WriteAt and does not
				// mutate the shared file cursor.
				copyStartedAt := time.Now()
				bodyReader := newTimedReader(resp.Content, copyStartedAt)
				written, writeStats, streamErr := writePartAtWithStats(targetFile, part, bodyReader)
				copyDuration := time.Since(copyStartedAt)

				copyOutcome := DownloadOutcomeSuccess
				if streamErr != nil {
					copyOutcome = DownloadOutcomeError
				}
				if bodyReader.observedFirstRead {
					cds.observeDownloadPhase(DownloadObservation{
						Phase:      PhaseGetObjectFirstRead,
						Duration:   bodyReader.firstReadDuration,
						Outcome:    copyOutcome,
						ObjectName: part.object,
						PartNumber: part.partNum,
						HasPart:    true,
						Attempt:    attempt,
						ChunkSize:  part.size,
						Err:        streamErr,
					})
				}
				cds.observeDownloadPhase(DownloadObservation{
					Phase:      PhaseGetObjectBodyRead,
					Duration:   bodyReader.duration,
					Bytes:      bodyReader.bytes,
					Outcome:    copyOutcome,
					ObjectName: part.object,
					PartNumber: part.partNum,
					HasPart:    true,
					Attempt:    attempt,
					ChunkSize:  part.size,
					Err:        streamErr,
				})
				cds.observeDownloadPhase(DownloadObservation{
					Phase:      PhaseModelFileWrite,
					Duration:   writeStats.Duration,
					Bytes:      writeStats.Bytes,
					Outcome:    copyOutcome,
					ObjectName: part.object,
					PartNumber: part.partNum,
					HasPart:    true,
					Attempt:    attempt,
					ChunkSize:  part.size,
					WriteStats: &writeStats,
					Err:        streamErr,
				})
				cds.observeDownloadPhase(DownloadObservation{
					Phase:      PhaseObjectToModelWrite,
					Duration:   copyDuration,
					Bytes:      written,
					Outcome:    copyOutcome,
					ObjectName: part.object,
					PartNumber: part.partNum,
					HasPart:    true,
					Attempt:    attempt,
					ChunkSize:  part.size,
					Err:        streamErr,
				})

				closeErr := resp.Content.Close()

				if streamErr != nil {
					cds.logger.Warnf("Error streaming response to target file for part %d (attempt %d/%d): %s", part.partNum, attempt, maxPartRetries, streamErr)
					lastErr = streamErr
				} else if closeErr != nil {
					cds.logger.Warnf("Error closing response body for part %d (attempt %d/%d): %s", part.partNum, attempt, maxPartRetries, closeErr)
					lastErr = closeErr
				} else {
					// Success
					size = written
					lastErr = nil
					break
				}
			}
			if attempt < maxPartRetries && lastErr != nil {
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
			}
		}
		if ctx.Err() != nil {
			return
		}

		duration := time.Since(start)
		speedMBs := float64(size) / 1024.0 / 1024.0 / duration.Seconds()
		if lastErr == nil {
			cds.logger.Debugf("[Chunk %d] Downloaded %d bytes in %.2fs (%.2f MB/s) for file %s", part.partNum, size, duration.Seconds(), speedMBs, part.object)
		}

		if lastErr != nil {
			// All retries failed for this part
			channelWaitStartedAt := time.Now()
			partResult := &DownloadedPart{
				err:     lastErr,
				partNum: part.partNum,
			}
			select {
			case result <- partResult:
			case <-ctx.Done():
				return
			}
			cds.observeDownloadPhase(DownloadObservation{
				Phase:      PhasePartChannelWait,
				Duration:   time.Since(channelWaitStartedAt),
				Outcome:    DownloadOutcomeError,
				ObjectName: part.object,
				PartNumber: part.partNum,
				HasPart:    true,
				ChunkSize:  part.size,
				Err:        lastErr,
			})
			continue
		}

		// Success: send the downloaded part
		channelWaitStartedAt := time.Now()
		partResult := &DownloadedPart{
			size:    size,
			partNum: part.partNum,
		}
		select {
		case result <- partResult:
		case <-ctx.Done():
			return
		}
		cds.observeDownloadPhase(DownloadObservation{
			Phase:      PhasePartChannelWait,
			Duration:   time.Since(channelWaitStartedAt),
			Bytes:      size,
			Outcome:    DownloadOutcomeSuccess,
			ObjectName: part.object,
			PartNumber: part.partNum,
			HasPart:    true,
			ChunkSize:  part.size,
		})
	}
}

// writePartAt writes exactly one range into its preassigned location. The
// LimitedReader prevents a malformed range response from crossing into the
// next part, while OffsetWriter makes concurrent writes independent of the
// shared file cursor.
func writePartAt(targetFile *os.File, part *PrepareDownloadPart, source io.Reader) (int64, error) {
	written, _, err := writePartAtWithStats(targetFile, part, source)
	return written, err
}

func writePartAtWithStats(targetFile *os.File, part *PrepareDownloadPart, source io.Reader) (int64, WriteStats, error) {
	writer := io.NewOffsetWriter(targetFile, part.offset)
	timedWriter := &writeStatsWriter{writer: writer}
	limitedSource := &io.LimitedReader{R: source, N: part.size}
	bufp := BufferPool.Get().(*[]byte)
	defer BufferPool.Put(bufp)

	written, err := copyCoalesced(timedWriter, limitedSource, *bufp)
	if err == nil && written != part.size {
		err = io.ErrUnexpectedEOF
	}
	return written, timedWriter.stats, err
}

// copyCoalesced fills buffer before writing it, except for the final partial
// buffer. This is intentionally different from io.CopyBuffer: io.CopyBuffer
// forwards every short source read to the destination immediately, which can
// turn transport-sized reads (commonly 16 KiB) into the same number of small
// WriteAt calls even when the supplied buffer is much larger.
func copyCoalesced(dst io.Writer, src io.Reader, buffer []byte) (int64, error) {
	if len(buffer) == 0 {
		return 0, io.ErrShortBuffer
	}

	var written int64
	for {
		n, readErr := io.ReadFull(src, buffer)
		if n > 0 {
			writeN, writeErr := dst.Write(buffer[:n])
			written += int64(writeN)
			if writeErr != nil {
				return written, writeErr
			}
			if writeN != n {
				return written, io.ErrShortWrite
			}
		}

		switch readErr {
		case nil:
			continue
		case io.EOF, io.ErrUnexpectedEOF:
			return written, nil
		default:
			return written, readErr
		}
	}
}

type writeStatsWriter struct {
	writer io.Writer
	stats  WriteStats
}

func (w *writeStatsWriter) Write(buffer []byte) (int, error) {
	startedAt := time.Now()
	n, err := w.writer.Write(buffer)
	duration := time.Since(startedAt)

	requestedBytes := int64(len(buffer))
	w.stats.Calls++
	w.stats.Bytes += int64(n)
	w.stats.Duration += duration
	if duration > w.stats.MaxDuration {
		w.stats.MaxDuration = duration
	}
	if w.stats.MinRequestBytes == 0 || requestedBytes < w.stats.MinRequestBytes {
		w.stats.MinRequestBytes = requestedBytes
	}
	if requestedBytes > w.stats.MaxRequestBytes {
		w.stats.MaxRequestBytes = requestedBytes
	}
	switch {
	case requestedBytes <= 16*1024:
		w.stats.CallsUpTo16KiB++
	case requestedBytes <= 64*1024:
		w.stats.Calls16KiBTo64KiB++
	case requestedBytes <= 256*1024:
		w.stats.Calls64KiBTo256KiB++
	case requestedBytes <= 1024*1024:
		w.stats.Calls256KiBTo1MiB++
	default:
		w.stats.CallsOver1MiB++
	}

	return n, err
}

func (cds *OCIOSDataStore) DownloadWithMultiThreads(downloadThreads int, filesToDownload chan *FileToDownload) chan *DownloadedFile {
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

func (cds *OCIOSDataStore) downloadFiles(filesToDownload chan *FileToDownload, result chan *DownloadedFile) {
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

func (cds *OCIOSDataStore) downloadFile(fileToDownload *FileToDownload) error {
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
