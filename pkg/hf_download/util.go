package hf_download

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path"
)

// VerifyChecksum checks if a file's SHA-256 checksum matches the expected checksum.
func VerifyChecksum(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("error computing checksum: %w", err)
	}

	actualChecksum := hex.EncodeToString(hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}
	return nil
}

// MergeFiles consolidates downloaded chunks from temp files into a single output file.
func MergeFiles(tempFolder, outputFileName string, numChunks int) error {
	outputFile, err := os.Create(outputFileName)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer outputFile.Close()

	for i := 0; i < numChunks; i++ {
		tmpFileName := fmt.Sprintf("%s_%d.tmp", path.Base(outputFileName), i)
		if err := appendChunk(tempFolder, tmpFileName, outputFile); err != nil {
			return err
		}
	}
	return nil
}

// appendChunk appends a temporary chunk file to the output file, then deletes the chunk.
func appendChunk(tempFolder, tmpFileName string, outputFile *os.File) error {
	tempFilePath := path.Join(tempFolder, tmpFileName)
	tempFile, err := os.Open(tempFilePath)
	if err != nil {
		return fmt.Errorf("error opening temp file %s: %w", tmpFileName, err)
	}
	defer tempFile.Close()

	if _, err = io.Copy(outputFile, tempFile); err != nil {
		return fmt.Errorf("error copying chunk to output file: %w", err)
	}

	if err = os.Remove(tempFilePath); err != nil {
		return fmt.Errorf("error removing temp file %s: %w", tmpFileName, err)
	}
	return nil
}

// AdjustStartByte calculates the appropriate start byte for resuming an incomplete download.
func AdjustStartByte(tmpFileName string, start, end int64, progress chan<- int64) (int64, error) {
	const compensationBytes int64 = 12

	if fi, err := os.Stat(tmpFileName); err == nil {
		if fi.Size() >= (end - start) {
			progress <- fi.Size()
			return start, nil
		}
		start = int64(math.Max(float64(start+fi.Size()-compensationBytes), 0))
		progress <- int64(math.Max(float64(fi.Size()-compensationBytes), 0))
	}
	return start, nil
}

// NeedsDownload checks if a file download is needed based on the existence and size of the local file.
func NeedsDownload(filePath string, remoteSize int) bool {
	info, err := os.Stat(filePath)
	return os.IsNotExist(err) || info.Size() != int64(remoteSize)
}

// WriteToFile writes data from an HTTP response body to a temp file, tracking progress.
func WriteToFile(body io.Reader, tmpFileName string, progress chan<- int64) error {
	tempFile, err := os.OpenFile(tmpFileName, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("error opening temp file: %w", err)
	}
	defer tempFile.Close()

	if _, err = tempFile.Seek(-12, io.SeekEnd); err != nil {
		_, _ = tempFile.Seek(0, io.SeekStart)
	}

	buffer := make([]byte, 32768)
	for {
		bytesRead, err := body.Read(buffer)
		if bytesRead > 0 {
			if _, writeErr := tempFile.Write(buffer[:bytesRead]); writeErr != nil {
				return fmt.Errorf("error writing to temp file: %w", writeErr)
			}
			progress <- int64(bytesRead)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading response body: %w", err)
		}
	}
	return nil
}
