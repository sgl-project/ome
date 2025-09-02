package oci

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// verifyMD5 computes and verifies MD5 checksum for a downloaded file
func (p *OCIProvider) verifyMD5(ctx context.Context, source *ociURI, filePath string) error {
	// Get object metadata for MD5
	headResp, err := p.client.HeadObject(ctx, objectstorage.HeadObjectRequest{
		NamespaceName: &source.Namespace,
		BucketName:    &source.Bucket,
		ObjectName:    &source.Object,
	})
	if err != nil {
		return fmt.Errorf("failed to get metadata for MD5 verification: %w", err)
	}

	// Check for MD5 in headers
	var expectedMD5 string
	if headResp.ContentMd5 != nil && *headResp.ContentMd5 != "" {
		expectedMD5 = *headResp.ContentMd5
		p.logger.WithField("md5", expectedMD5).Debug("Using ContentMd5 from headers")
	} else if headResp.OpcMultipartMd5 != nil && *headResp.OpcMultipartMd5 != "" {
		// Check if this is a multipart upload format
		if isMultipartMD5(*headResp.OpcMultipartMd5) {
			p.logger.WithField("multipart_md5", *headResp.OpcMultipartMd5).
				Debug("Detected multipart upload, checking custom metadata")

			// Try to get real MD5 from custom metadata
			if md5Val, ok := headResp.OpcMeta["md5"]; ok && md5Val != "" {
				expectedMD5 = md5Val
				p.logger.WithField("md5", expectedMD5).Debug("Using MD5 from custom metadata")
			} else {
				p.logger.Warn("No MD5 available for multipart object, skipping verification")
				return nil // Skip verification for multipart without custom MD5
			}
		} else {
			// OpcMultipartMd5 contains a regular MD5
			expectedMD5 = *headResp.OpcMultipartMd5
		}
	}

	if expectedMD5 == "" {
		p.logger.Debug("No MD5 available from server, skipping verification")
		return nil // No MD5 available
	}

	// Compute local MD5
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for MD5 verification: %w", err)
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to compute MD5: %w", err)
	}

	localMD5 := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	if localMD5 != expectedMD5 {
		return fmt.Errorf("MD5 mismatch: expected %s, got %s", expectedMD5, localMD5)
	}

	p.logger.Debug("MD5 verification successful")
	return nil
}

// isMultipartMD5 detects if the given MD5 string represents a multipart upload checksum
// OCI and S3 multipart MD5s often take the form: "<base64md5>-<part count>"
func isMultipartMD5(md5str string) bool {
	parts := strings.Split(md5str, "-")
	if len(parts) != 2 {
		return false
	}

	// The second part should be a number (part count)
	_, err := strconv.Atoi(parts[1])
	return err == nil
}

// isLocalCopyValid checks if a local file is a valid copy of the remote object
// It verifies both size and MD5 checksum when available
func (p *OCIProvider) isLocalCopyValid(ctx context.Context, source *ociURI, localPath string) (bool, error) {
	// Check if file exists
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			p.logger.WithField("path", localPath).Debug("Local file does not exist")
			return false, nil
		}
		return false, fmt.Errorf("failed to stat local file: %w", err)
	}

	// Get object metadata
	headResp, err := p.client.HeadObject(ctx, objectstorage.HeadObjectRequest{
		NamespaceName: &source.Namespace,
		BucketName:    &source.Bucket,
		ObjectName:    &source.Object,
	})
	if err != nil {
		return false, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Check size first (quick check)
	if headResp.ContentLength != nil && fileInfo.Size() != *headResp.ContentLength {
		p.logger.WithField("expected", *headResp.ContentLength).
			WithField("actual", fileInfo.Size()).
			Debug("File size mismatch")
		return false, nil
	}

	// Verify MD5 if available (slower but accurate)
	if err := p.verifyMD5(ctx, source, localPath); err != nil {
		p.logger.WithField("error", err).Debug("MD5 verification failed")
		return false, nil
	}

	p.logger.WithField("path", localPath).Info("Valid local copy exists")
	return true, nil
}

// computeMD5 computes the MD5 checksum of a file
func computeMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(hash.Sum(nil)), nil
}
