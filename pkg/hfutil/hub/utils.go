package hub

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
)

// Regex patterns
var (
	commitHashRegex = regexp.MustCompile(`^[0-9a-f]{40}$`)
	sha256Regex     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// URL construction functions

// HfHubURL constructs the URL of a file from the given information
func HfHubURL(repoID, filename string, opts *DownloadConfig) (string, error) {
	if opts == nil {
		opts = &DownloadConfig{}
	}

	// Set defaults
	repoType := opts.RepoType
	if repoType == "" {
		repoType = RepoTypeModel
	}

	revision := opts.Revision
	if revision == "" {
		revision = DefaultRevision
	}

	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}

	// Validate repo type
	if !isValidRepoType(repoType) {
		return "", fmt.Errorf("invalid repo type: %s. Accepted types are: %v", repoType, RepoTypes)
	}

	// Handle subfolder
	if opts.Subfolder != "" && opts.Subfolder != "." {
		filename = path.Join(opts.Subfolder, filename)
	}

	// Add repo type prefix for datasets and spaces
	repoPath := repoID
	if prefix, exists := RepoTypesURLPrefixes[repoType]; exists {
		repoPath = prefix + repoID
	}

	// Construct the URL - escape path components individually but preserve forward slashes
	escapedRevision := url.PathEscape(revision)
	// For filename, escape each path component separately to preserve forward slashes
	escapedFilename := escapeFilePath(filename)

	return fmt.Sprintf("%s/%s/resolve/%s/%s", endpoint, repoPath, escapedRevision, escapedFilename), nil
}

// escapeFilePath escapes each component of a file path separately, preserving forward slashes
func escapeFilePath(filename string) string {
	if filename == "" {
		return ""
	}

	// Split by forward slashes, escape each component, then rejoin
	parts := strings.Split(filename, "/")
	escapedParts := make([]string, len(parts))
	for i, part := range parts {
		escapedParts[i] = url.PathEscape(part)
	}
	return strings.Join(escapedParts, "/")
}

// RepoFolderName returns a serialized version of a repo name and type, safe for disk storage
func RepoFolderName(repoID, repoType string) string {
	// Convert slashes to separator
	parts := []string{repoType + "s"}
	parts = append(parts, strings.Split(repoID, "/")...)
	return strings.Join(parts, RepoIdSeparator)
}

// Validation functions

// IsCommitHash checks if the revision is a commit hash
func IsCommitHash(revision string) bool {
	return commitHashRegex.MatchString(revision)
}

// IsSHA256 checks if the etag is a valid SHA256 hash
func IsSHA256(etag string) bool {
	return sha256Regex.MatchString(etag)
}

// isValidRepoType checks if the repository type is valid
func isValidRepoType(repoType string) bool {
	for _, validType := range RepoTypes {
		if repoType == validType {
			return true
		}
	}
	return false
}

// File operations

// EnsureDir creates directory if it doesn't exist
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// FileExists checks if a file exists
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// GetFileSize returns the size of a file
func GetFileSize(filename string) (int64, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// VerifyChecksum verifies the SHA256 checksum of a file
func VerifyChecksum(filename, expectedHash string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("failed to compute hash: %w", err)
	}

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// CheckDiskSpace checks if there's enough disk space for the expected file size
func CheckDiskSpace(expectedSize int64, targetDir string) error {
	if expectedSize <= 0 {
		return nil // Can't check if we don't know the size
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(targetDir, &stat); err != nil {
		// If we can't check disk space, just proceed
		return nil
	}

	// Available space in bytes
	availableSpace := int64(stat.Bavail) * int64(stat.Bsize)

	if availableSpace < expectedSize {
		return fmt.Errorf("insufficient disk space: need %d bytes, have %d bytes", expectedSize, availableSpace)
	}

	return nil
}

// Symlink support

// AreSymlinksSupported returns whether symlinks are supported on the current platform and directory
func AreSymlinksSupported(dir string) bool {
	// Windows generally doesn't support symlinks without admin privileges
	if runtime.GOOS == "windows" {
		return false
	}

	// Test if we can create a symlink in the directory
	if dir == "" {
		return false
	}

	if err := EnsureDir(dir); err != nil {
		return false
	}

	tempSrc := filepath.Join(dir, "test_symlink_src")
	tempDst := filepath.Join(dir, "test_symlink_dst")

	// Create source file
	if err := os.WriteFile(tempSrc, []byte("test"), 0644); err != nil {
		return false
	}
	defer os.Remove(tempSrc)

	// Try to create symlink
	if err := os.Symlink(tempSrc, tempDst); err != nil {
		return false
	}
	defer os.Remove(tempDst)

	return true
}

// CreateSymlink creates a symlink, with fallback to copying if symlinks aren't supported
func CreateSymlink(src, dst string) error {
	// Remove destination if it exists
	if FileExists(dst) {
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("failed to remove existing destination: %w", err)
		}
	}

	// Ensure destination directory exists
	if err := EnsureDir(filepath.Dir(dst)); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Try to create symlink with relative path
	dstDir := filepath.Dir(dst)
	relSrc, err := filepath.Rel(dstDir, src)
	if err == nil {
		// Use relative path
		if err := os.Symlink(relSrc, dst); err == nil {
			return nil
		}
	}

	// Fall back to absolute path
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if err := os.Symlink(absSrc, dst); err != nil {
		// Symlinks not supported, fall back to copying
		return copyFile(src, dst)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Copy file permissions
	srcInfo, err := os.Stat(src)
	if err == nil {
		// Preserve file permissions - ignore errors as this might fail on some filesystems
		_ = os.Chmod(dst, srcInfo.Mode())
	}

	return nil
}

// Pattern matching for allow/ignore patterns

// MatchesPattern checks if a filename matches any of the given glob patterns
func MatchesPattern(filename string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filename)
		if err == nil && matched {
			return true
		}

		// Also check if the pattern matches the path components
		if strings.Contains(filename, pattern) {
			return true
		}
	}

	return false
}

// ShouldIgnoreFile determines if a file should be ignored based on patterns
func ShouldIgnoreFile(filename string, allowPatterns, ignorePatterns []string) bool {
	// If allow patterns are specified, file must match at least one
	if len(allowPatterns) > 0 {
		if !MatchesPattern(filename, allowPatterns) {
			return true
		}
	}

	// If ignore patterns are specified, file must not match any
	if len(ignorePatterns) > 0 {
		if MatchesPattern(filename, ignorePatterns) {
			return true
		}
	}

	return false
}

// HTTP helpers

// BuildHeaders builds HTTP headers for requests
func BuildHeaders(token, userAgent string, extraHeaders map[string]string) map[string]string {
	headers := make(map[string]string)

	// Add user agent
	if userAgent != "" {
		headers[UserAgentHeader] = userAgent
	}

	// Add authorization
	if token != "" {
		headers[AuthorizationHeader] = "Bearer " + token
	}

	// Add extra headers
	for k, v := range extraHeaders {
		headers[k] = v
	}

	return headers
}

// NormalizeEtag normalizes ETag HTTP header
func NormalizeEtag(etag string) string {
	if etag == "" {
		return ""
	}

	// Remove W/ prefix and quotes
	etag = strings.TrimPrefix(etag, "W/")
	etag = strings.Trim(etag, `"`)

	return etag
}

// Cache helpers

// GetPointerPath returns the path to a symlink/pointer file in cache
func GetPointerPath(storageFolder, revision, relativeFilename string) (string, error) {
	snapshotPath := filepath.Join(storageFolder, "snapshots")
	pointerPath := filepath.Join(snapshotPath, revision, relativeFilename)

	// Validate that the pointer path is within the snapshot directory
	absSnapshotPath, err := filepath.Abs(snapshotPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute snapshot path: %w", err)
	}

	absPointerPath, err := filepath.Abs(pointerPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute pointer path: %w", err)
	}

	if !strings.HasPrefix(absPointerPath, absSnapshotPath) {
		return "", fmt.Errorf("invalid pointer path: %s is not within snapshot directory %s", pointerPath, snapshotPath)
	}

	return pointerPath, nil
}

// CacheCommitHashForRevision caches the mapping between a revision and commit hash
func CacheCommitHashForRevision(storageFolder, revision, commitHash string) error {
	if revision == commitHash {
		return nil // No need to cache if revision is already a commit hash
	}

	refPath := filepath.Join(storageFolder, "refs", revision)
	if err := EnsureDir(filepath.Dir(refPath)); err != nil {
		return fmt.Errorf("failed to create refs directory: %w", err)
	}

	// Only update if the cached value is different
	if FileExists(refPath) {
		existing, err := os.ReadFile(refPath)
		if err == nil && string(existing) == commitHash {
			return nil // Already cached correctly
		}
	}

	if err := os.WriteFile(refPath, []byte(commitHash), 0644); err != nil {
		return fmt.Errorf("failed to cache commit hash: %w", err)
	}

	return nil
}
