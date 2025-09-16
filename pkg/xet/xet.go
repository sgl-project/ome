package xet

/*
#cgo LDFLAGS: -L${SRCDIR}/target/release -lxet -ldl -lm -lstdc++
#cgo darwin LDFLAGS: -framework CoreFoundation -framework Security
#include <stdlib.h>
#include "xet.h"

// Link-time version check
extern void xet_version_1_0_0(void);
void (*xet_version_check)(void) = &xet_version_1_0_0;

// Declare the snapshot download function
extern XetError* xet_download_snapshot(
    XetClient* client,
    const char* repo_id,
    const char* repo_type,
    const char* revision,
    const char* local_dir,
    XetProgressCallback progress,
    void* user_data,
    char** out_path
);

// Progress callback wrapper
void progressCallback(const char* file_path, uint64_t downloaded, uint64_t total, void* user_data);
*/
import "C"
import (
	"context"
	"fmt"
	"io"
	"os"
	"unsafe"
)

// Client represents an xet-core client for HF Hub operations
type Client struct {
	client *C.XetClient
}

// Config holds configuration for the xet client
type Config struct {
	Endpoint              string
	Token                 string
	CacheDir              string
	MaxConcurrentDownloads uint32
	EnableDedup           bool
}

// DownloadRequest represents a file download request
type DownloadRequest struct {
	RepoID   string
	RepoType string
	Revision string
	Filename string
	LocalDir string
}

// SnapshotRequest represents a snapshot download request
type SnapshotRequest struct {
	RepoID         string
	RepoType       string
	Revision       string
	LocalDir       string
	AllowPatterns  []string
	IgnorePatterns []string
}

// FileInfo represents file information
type FileInfo struct {
	Path string
	Hash string
	Size uint64
}

// ProgressFunc is the callback for download progress
type ProgressFunc func(filepath string, downloaded, total uint64)

// XetError represents an error from xet-core
type XetError struct {
	Code    int
	Message string
	Details string
}

func (e *XetError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("xet error %d: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("xet error %d: %s", e.Code, e.Message)
}

// NewClient creates a new xet client
func NewClient(config *Config) (*Client, error) {
	if config == nil {
		config = &Config{
			Endpoint:               "https://huggingface.co",
			MaxConcurrentDownloads: 4,
			EnableDedup:            true,
		}
	}

	cConfig := C.XetConfig{
		max_concurrent_downloads: C.uint32_t(config.MaxConcurrentDownloads),
		enable_dedup:             C.bool(config.EnableDedup),
	}

	// Set string fields
	if config.Endpoint != "" {
		cEndpoint := C.CString(config.Endpoint)
		defer C.free(unsafe.Pointer(cEndpoint))
		cConfig.endpoint = cEndpoint
	}

	if config.Token != "" {
		cToken := C.CString(config.Token)
		defer C.free(unsafe.Pointer(cToken))
		cConfig.token = cToken
	}

	if config.CacheDir != "" {
		cCacheDir := C.CString(config.CacheDir)
		defer C.free(unsafe.Pointer(cCacheDir))
		cConfig.cache_dir = cCacheDir
	}

	client := C.xet_client_new(&cConfig)
	if client == nil {
		return nil, fmt.Errorf("failed to create xet client")
	}

	return &Client{client: client}, nil
}

// Close releases the client resources
func (c *Client) Close() error {
	if c.client != nil {
		C.xet_client_free(c.client)
		c.client = nil
	}
	return nil
}

// Ensure Client implements io.Closer
var _ io.Closer = (*Client)(nil)

// ListFiles lists files in a repository
func (c *Client) ListFiles(repoID string, revision string) ([]FileInfo, error) {
	if c.client == nil {
		return nil, fmt.Errorf("client is closed")
	}

	cRepoID := C.CString(repoID)
	defer C.free(unsafe.Pointer(cRepoID))

	var cRevision *C.char
	if revision != "" {
		cRevision = C.CString(revision)
		defer C.free(unsafe.Pointer(cRevision))
	}

	var fileList *C.XetFileList
	err := C.xet_list_files(c.client, cRepoID, cRevision, &fileList)
	if err != nil {
		return nil, convertError(err)
	}
	defer C.xet_free_file_list(fileList)

	// Convert C file list to Go slice
	files := make([]FileInfo, fileList.count)
	for i := 0; i < int(fileList.count); i++ {
		cFile := (*C.XetFileInfo)(unsafe.Pointer(
			uintptr(unsafe.Pointer(fileList.files)) + uintptr(i)*unsafe.Sizeof(C.XetFileInfo{}),
		))
		files[i] = FileInfo{
			Path: C.GoString(cFile.path),
			Hash: C.GoString(cFile.hash),
			Size: uint64(cFile.size),
		}
	}

	return files, nil
}

// DownloadFile downloads a single file from a repository
func (c *Client) DownloadFile(req *DownloadRequest) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("client is closed")
	}
	
	fmt.Fprintf(os.Stderr, "[GO DEBUG] DownloadFile called: %s/%s\n", req.RepoID, req.Filename)

	cReq := convertDownloadRequest(req)
	defer freeDownloadRequest(&cReq)

	var outPath *C.char
	err := C.xet_download_file(c.client, &cReq, nil, nil, &outPath)
	if err != nil {
		return "", convertError(err)
	}
	defer C.xet_free_string(outPath)

	return C.GoString(outPath), nil
}

// DownloadFileWithProgress downloads a file with progress callback
func (c *Client) DownloadFileWithProgress(req *DownloadRequest, progress ProgressFunc) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("client is closed")
	}

	// TODO: Implement progress callback marshalling
	// This requires more complex CGO callback handling
	return c.DownloadFile(req)
}

// DownloadFileWithContext downloads a file with context support
func (c *Client) DownloadFileWithContext(ctx context.Context, req *DownloadRequest) (string, error) {
	// TODO: Implement context cancellation
	// This requires passing cancellation token to Rust side
	return c.DownloadFile(req)
}

// DownloadSnapshot downloads all files from a repository in parallel
func (c *Client) DownloadSnapshot(req *SnapshotRequest) (string, error) {
	return c.DownloadSnapshotWithProgress(req, nil)
}

// DownloadSnapshotWithProgress downloads all files with progress callback
func (c *Client) DownloadSnapshotWithProgress(req *SnapshotRequest, progressFn func(string, uint64, uint64)) (string, error) {
	var outPath *C.char
	
	var cRepoID *C.char
	if req.RepoID != "" {
		cRepoID = C.CString(req.RepoID)
		defer C.free(unsafe.Pointer(cRepoID))
	}
	
	var cRepoType *C.char
	if req.RepoType != "" {
		cRepoType = C.CString(req.RepoType)
		defer C.free(unsafe.Pointer(cRepoType))
	}
	
	var cRevision *C.char
	if req.Revision != "" {
		cRevision = C.CString(req.Revision)
		defer C.free(unsafe.Pointer(cRevision))
	}
	
	var cLocalDir *C.char
	if req.LocalDir != "" {
		cLocalDir = C.CString(req.LocalDir)
		defer C.free(unsafe.Pointer(cLocalDir))
	}

	// For now, we'll call without progress callback
	// TODO: Implement proper callback marshalling
	cErr := C.xet_download_snapshot(
		c.client,
		cRepoID,
		cRepoType,
		cRevision,
		cLocalDir,
		nil, // No progress callback for now
		nil,
		&outPath,
	)
	
	if cErr != nil {
		defer C.xet_free_error(cErr)
		return "", fmt.Errorf("xet error %d: %s", cErr.code, C.GoString(cErr.message))
	}
	
	path := C.GoString(outPath)
	C.xet_free_string(outPath)
	
	return path, nil
}

// Helper functions

func convertDownloadRequest(req *DownloadRequest) C.XetDownloadRequest {
	cReq := C.XetDownloadRequest{}
	
	if req.RepoID != "" {
		cReq.repo_id = C.CString(req.RepoID)
	}
	if req.RepoType != "" {
		cReq.repo_type = C.CString(req.RepoType)
	}
	if req.Revision != "" {
		cReq.revision = C.CString(req.Revision)
	}
	if req.Filename != "" {
		cReq.filename = C.CString(req.Filename)
	}
	if req.LocalDir != "" {
		cReq.local_dir = C.CString(req.LocalDir)
	}
	
	return cReq
}

func freeDownloadRequest(req *C.XetDownloadRequest) {
	if req.repo_id != nil {
		C.free(unsafe.Pointer(req.repo_id))
	}
	if req.repo_type != nil {
		C.free(unsafe.Pointer(req.repo_type))
	}
	if req.revision != nil {
		C.free(unsafe.Pointer(req.revision))
	}
	if req.filename != nil {
		C.free(unsafe.Pointer(req.filename))
	}
	if req.local_dir != nil {
		C.free(unsafe.Pointer(req.local_dir))
	}
}

func convertError(err *C.XetError) error {
	if err == nil {
		return nil
	}
	defer C.xet_free_error(err)
	
	xetErr := &XetError{
		Code:    int(err.code),
		Message: C.GoString(err.message),
	}
	if err.details != nil {
		xetErr.Details = C.GoString(err.details)
	}
	
	return xetErr
}