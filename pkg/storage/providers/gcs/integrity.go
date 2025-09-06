package gcs

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"

	"cloud.google.com/go/storage"
)

// verifyChecksum verifies the integrity of a downloaded file using CRC32C or MD5
func (p *Provider) verifyChecksum(ctx context.Context, bucketName, objectName, filePath string) error {
	// Get object attributes for checksum
	obj := p.client.Bucket(bucketName).Object(objectName)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get object attributes for checksum verification: %w", err)
	}

	// Open the downloaded file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for checksum verification: %w", err)
	}
	defer file.Close()

	// GCS provides CRC32C by default
	if attrs.CRC32C != 0 {
		if err := p.verifyCRC32C(file, attrs.CRC32C); err != nil {
			return fmt.Errorf("CRC32C verification failed: %w", err)
		}
		p.logger.WithField("crc32c", attrs.CRC32C).Debug("CRC32C verification successful")
	}

	// MD5 is available for non-composite objects
	if attrs.MD5 != nil && len(attrs.MD5) > 0 {
		// Reset file position
		if _, err := file.Seek(0, 0); err != nil {
			return fmt.Errorf("failed to reset file position: %w", err)
		}

		if err := p.verifyMD5(file, attrs.MD5); err != nil {
			return fmt.Errorf("MD5 verification failed: %w", err)
		}
		p.logger.WithField("md5", hex.EncodeToString(attrs.MD5)).Debug("MD5 verification successful")
	}

	return nil
}

// verifyCRC32C verifies the CRC32C checksum of a file
func (p *Provider) verifyCRC32C(reader io.Reader, expectedCRC uint32) error {
	// Use the Castagnoli polynomial (same as GCS)
	table := crc32.MakeTable(crc32.Castagnoli)
	hasher := crc32.New(table)

	if _, err := io.Copy(hasher, reader); err != nil {
		return fmt.Errorf("failed to compute CRC32C: %w", err)
	}

	computedCRC := hasher.Sum32()
	if computedCRC != expectedCRC {
		return fmt.Errorf("CRC32C mismatch: expected %d, got %d", expectedCRC, computedCRC)
	}

	return nil
}

// verifyMD5 verifies the MD5 checksum of a file
func (p *Provider) verifyMD5(reader io.Reader, expectedMD5 []byte) error {
	hasher := md5.New()
	if _, err := io.Copy(hasher, reader); err != nil {
		return fmt.Errorf("failed to compute MD5: %w", err)
	}

	computedMD5 := hasher.Sum(nil)
	if !bytes.Equal(computedMD5, expectedMD5) {
		return fmt.Errorf("MD5 mismatch: expected %s, got %s",
			hex.EncodeToString(expectedMD5),
			hex.EncodeToString(computedMD5))
	}

	return nil
}

// calculateFileChecksum calculates both MD5 and CRC32C for a file in a single pass
func (p *Provider) calculateFileChecksum(filePath string) (md5Hash []byte, crc32c uint32, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create both hashers
	md5Hasher := md5.New()
	table := crc32.MakeTable(crc32.Castagnoli)
	crc32Hasher := crc32.New(table)

	// Use io.MultiWriter to calculate both hashes in a single pass
	multiWriter := io.MultiWriter(md5Hasher, crc32Hasher)

	if _, err := io.Copy(multiWriter, file); err != nil {
		return nil, 0, fmt.Errorf("failed to calculate checksums: %w", err)
	}

	md5Hash = md5Hasher.Sum(nil)
	crc32c = crc32Hasher.Sum32()

	return md5Hash, crc32c, nil
}

// uploadWithChecksum uploads a file with checksum verification
func (p *Provider) uploadWithChecksum(ctx context.Context, bucketName, objectName, filePath string) error {
	// Calculate checksums
	md5Hash, crc32c, err := p.calculateFileChecksum(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate checksums: %w", err)
	}

	// Open file for upload
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for upload: %w", err)
	}
	defer file.Close()

	// Create object writer with checksums
	obj := p.client.Bucket(bucketName).Object(objectName)
	writer := obj.NewWriter(ctx)

	// Set MD5 for verification (GCS will verify on upload)
	writer.MD5 = md5Hash
	writer.SendCRC32C = true
	writer.CRC32C = crc32c

	// Copy file content
	if _, err := io.Copy(writer, file); err != nil {
		writer.Close()
		return fmt.Errorf("failed to upload file: %w", err)
	}

	// Close writer (this triggers the checksum verification on GCS side)
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer (checksum verification may have failed): %w", err)
	}

	p.logger.WithField("md5", hex.EncodeToString(md5Hash)).
		WithField("crc32c", crc32c).
		Debug("Upload completed with checksum verification")

	return nil
}

// getObjectChecksum retrieves the checksums for an object
func (p *Provider) getObjectChecksum(ctx context.Context, bucketName, objectName string) (md5 string, crc32c uint32, err error) {
	obj := p.client.Bucket(bucketName).Object(objectName)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get object attributes: %w", err)
	}

	if attrs.MD5 != nil {
		md5 = base64.StdEncoding.EncodeToString(attrs.MD5)
	}
	crc32c = attrs.CRC32C

	return md5, crc32c, nil
}

// ChecksumMetadata represents checksum information for an object
type ChecksumMetadata struct {
	MD5    string
	CRC32C uint32
	Size   int64
}

// GetChecksumMetadata retrieves checksum metadata for an object
func (p *Provider) GetChecksumMetadata(ctx context.Context, uri string) (*ChecksumMetadata, error) {
	bucket, objectName, err := parseGCSURI(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URI: %w", err)
	}

	obj := p.client.Bucket(bucket).Object(objectName)
	attrs, err := obj.Attrs(ctx)
	if err != nil {
		if err == storage.ErrObjectNotExist {
			return nil, fmt.Errorf("object not found: %s", uri)
		}
		return nil, fmt.Errorf("failed to get object attributes: %w", err)
	}

	metadata := &ChecksumMetadata{
		CRC32C: attrs.CRC32C,
		Size:   attrs.Size,
	}

	if attrs.MD5 != nil {
		metadata.MD5 = base64.StdEncoding.EncodeToString(attrs.MD5)
	}

	return metadata, nil
}
