package github

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

// GitHubLFSStorage implements storage.Storage for GitHub LFS
type GitHubLFSStorage struct {
	httpClient  *http.Client
	credentials auth.Credentials
	logger      logging.Interface
	config      *Config
	apiEndpoint string
}

// Config represents GitHub LFS storage configuration
type Config struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	APIEndpoint string `json:"api_endpoint"`
	LFSEndpoint string `json:"lfs_endpoint"`
	ChunkSize   int64  `json:"chunk_size"`
}

// DefaultConfig returns default GitHub LFS storage configuration
func DefaultConfig() *Config {
	return &Config{
		APIEndpoint: "https://api.github.com",
		ChunkSize:   100 * 1024 * 1024, // 100MB chunks
	}
}

// LFS API structures

type LFSBatchRequest struct {
	Operation string      `json:"operation"`
	Transfers []string    `json:"transfers"`
	Objects   []LFSObject `json:"objects"`
}

type LFSObject struct {
	OID  string `json:"oid"`
	Size int64  `json:"size"`
}

type LFSBatchResponse struct {
	Transfer string                   `json:"transfer"`
	Objects  []LFSBatchResponseObject `json:"objects"`
}

type LFSBatchResponseObject struct {
	OID     string               `json:"oid"`
	Size    int64                `json:"size"`
	Actions map[string]LFSAction `json:"actions,omitempty"`
	Error   *LFSError            `json:"error,omitempty"`
}

type LFSAction struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

type LFSError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// New creates a new GitHub LFS storage instance
func New(ctx context.Context, cfg *Config, credentials auth.Credentials, logger logging.Interface) (*GitHubLFSStorage, error) {
	// Ensure we have GitHub credentials
	type githubCredentialsInterface interface {
		auth.Credentials
		GetHTTPClient() *http.Client
	}

	githubCreds, ok := credentials.(githubCredentialsInterface)
	if !ok {
		return nil, fmt.Errorf("invalid credentials type: expected GitHub credentials")
	}

	// Apply defaults
	if cfg == nil {
		cfg = DefaultConfig()
	} else {
		defaultConfig := DefaultConfig()
		if cfg.APIEndpoint == "" {
			cfg.APIEndpoint = defaultConfig.APIEndpoint
		}
		if cfg.ChunkSize == 0 {
			cfg.ChunkSize = defaultConfig.ChunkSize
		}
	}

	// Validate config
	if cfg.Owner == "" {
		return nil, fmt.Errorf("owner is required")
	}
	if cfg.Repo == "" {
		return nil, fmt.Errorf("repo is required")
	}

	// Set LFS endpoint if not provided
	if cfg.LFSEndpoint == "" {
		cfg.LFSEndpoint = fmt.Sprintf("https://github.com/%s/%s.git/info/lfs", cfg.Owner, cfg.Repo)
	}

	return &GitHubLFSStorage{
		httpClient:  githubCreds.GetHTTPClient(),
		credentials: credentials,
		logger:      logger,
		config:      cfg,
		apiEndpoint: cfg.APIEndpoint,
	}, nil
}

// Provider returns the storage provider type
func (s *GitHubLFSStorage) Provider() storage.Provider {
	return storage.ProviderGitHub
}

// Download retrieves the object and writes it to the target path
func (s *GitHubLFSStorage) Download(ctx context.Context, source storage.ObjectURI, target string, opts ...storage.DownloadOption) error {
	// Apply download options
	downloadOpts := storage.DefaultDownloadOptions()
	for _, opt := range opts {
		if err := opt(&downloadOpts); err != nil {
			return err
		}
	}

	// Get download URL from LFS
	downloadURL, headers, err := s.getDownloadURL(ctx, source.ObjectName)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}

	// Create directory if needed
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create file
	file, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Upload stores the file at source path as the target object
func (s *GitHubLFSStorage) Upload(ctx context.Context, source string, target storage.ObjectURI, opts ...storage.UploadOption) error {
	// Open file
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Calculate OID (SHA256)
	oid, err := calculateOID(file)
	if err != nil {
		return fmt.Errorf("failed to calculate OID: %w", err)
	}

	// Reset file position
	file.Seek(0, 0)

	// Get upload URL from LFS
	uploadURL, headers, err := s.getUploadURL(ctx, oid, info.Size())
	if err != nil {
		return fmt.Errorf("failed to get upload URL: %w", err)
	}

	// Create upload request
	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, file)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.ContentLength = info.Size()

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload failed with status %d", resp.StatusCode)
	}

	return nil
}

// Get retrieves an object and returns a reader
func (s *GitHubLFSStorage) Get(ctx context.Context, uri storage.ObjectURI) (io.ReadCloser, error) {
	// Get download URL from LFS
	downloadURL, headers, err := s.getDownloadURL(ctx, uri.ObjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to get download URL: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("get failed with status %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// Put stores data from reader as an object
func (s *GitHubLFSStorage) Put(ctx context.Context, uri storage.ObjectURI, reader io.Reader, size int64, opts ...storage.UploadOption) error {
	// For LFS, we need to calculate the OID first
	// This requires reading the entire content
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	// Calculate OID
	oid := calculateOIDFromBytes(data)

	// Get upload URL from LFS
	uploadURL, headers, err := s.getUploadURL(ctx, oid, int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed to get upload URL: %w", err)
	}

	// Create upload request
	req, err := http.NewRequestWithContext(ctx, "PUT", uploadURL, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.ContentLength = int64(len(data))

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("upload failed with status %d", resp.StatusCode)
	}

	return nil
}

// Delete is not supported by GitHub LFS
func (s *GitHubLFSStorage) Delete(ctx context.Context, uri storage.ObjectURI) error {
	return fmt.Errorf("delete operation not supported by GitHub LFS")
}

// Exists checks if an object exists
func (s *GitHubLFSStorage) Exists(ctx context.Context, uri storage.ObjectURI) (bool, error) {
	// Try to get download URL
	_, _, err := s.getDownloadURL(ctx, uri.ObjectName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// List is not directly supported by GitHub LFS
func (s *GitHubLFSStorage) List(ctx context.Context, uri storage.ObjectURI, opts storage.ListOptions) ([]storage.ObjectInfo, error) {
	return nil, fmt.Errorf("list operation not supported by GitHub LFS")
}

// GetObjectInfo retrieves metadata about an object
func (s *GitHubLFSStorage) GetObjectInfo(ctx context.Context, uri storage.ObjectURI) (*storage.ObjectInfo, error) {
	// LFS batch API to get object info
	batch := &LFSBatchRequest{
		Operation: "download",
		Transfers: []string{"basic"},
		Objects: []LFSObject{
			{OID: uri.ObjectName},
		},
	}

	resp, err := s.lfsBatchRequest(ctx, batch)
	if err != nil {
		return nil, err
	}

	if len(resp.Objects) == 0 {
		return nil, fmt.Errorf("object not found")
	}

	obj := resp.Objects[0]
	if obj.Error != nil {
		return nil, fmt.Errorf("LFS error: %s", obj.Error.Message)
	}

	return &storage.ObjectInfo{
		Name: obj.OID,
		Size: obj.Size,
	}, nil
}

// Copy is not supported by GitHub LFS
func (s *GitHubLFSStorage) Copy(ctx context.Context, source, target storage.ObjectURI) error {
	return fmt.Errorf("copy operation not supported by GitHub LFS")
}

// Multipart operations are not supported by GitHub LFS
func (s *GitHubLFSStorage) InitiateMultipartUpload(ctx context.Context, uri storage.ObjectURI, opts ...storage.UploadOption) (string, error) {
	return "", fmt.Errorf("multipart upload not supported by GitHub LFS")
}

func (s *GitHubLFSStorage) UploadPart(ctx context.Context, uri storage.ObjectURI, uploadID string, partNumber int, reader io.Reader, size int64) (string, error) {
	return "", fmt.Errorf("multipart upload not supported by GitHub LFS")
}

func (s *GitHubLFSStorage) CompleteMultipartUpload(ctx context.Context, uri storage.ObjectURI, uploadID string, parts []storage.CompletedPart) error {
	return fmt.Errorf("multipart upload not supported by GitHub LFS")
}

func (s *GitHubLFSStorage) AbortMultipartUpload(ctx context.Context, uri storage.ObjectURI, uploadID string) error {
	return fmt.Errorf("multipart upload not supported by GitHub LFS")
}

// Helper functions

func (s *GitHubLFSStorage) getDownloadURL(ctx context.Context, oid string) (string, map[string]string, error) {
	batch := &LFSBatchRequest{
		Operation: "download",
		Transfers: []string{"basic"},
		Objects: []LFSObject{
			{OID: oid},
		},
	}

	resp, err := s.lfsBatchRequest(ctx, batch)
	if err != nil {
		return "", nil, err
	}

	if len(resp.Objects) == 0 {
		return "", nil, fmt.Errorf("object not found")
	}

	obj := resp.Objects[0]
	if obj.Error != nil {
		return "", nil, fmt.Errorf("LFS error: %s", obj.Error.Message)
	}

	download, ok := obj.Actions["download"]
	if !ok {
		return "", nil, fmt.Errorf("no download action available")
	}

	return download.Href, download.Header, nil
}

func (s *GitHubLFSStorage) getUploadURL(ctx context.Context, oid string, size int64) (string, map[string]string, error) {
	batch := &LFSBatchRequest{
		Operation: "upload",
		Transfers: []string{"basic"},
		Objects: []LFSObject{
			{
				OID:  oid,
				Size: size,
			},
		},
	}

	resp, err := s.lfsBatchRequest(ctx, batch)
	if err != nil {
		return "", nil, err
	}

	if len(resp.Objects) == 0 {
		return "", nil, fmt.Errorf("no upload response")
	}

	obj := resp.Objects[0]
	if obj.Error != nil {
		return "", nil, fmt.Errorf("LFS error: %s", obj.Error.Message)
	}

	upload, ok := obj.Actions["upload"]
	if !ok {
		return "", nil, fmt.Errorf("no upload action available")
	}

	return upload.Href, upload.Header, nil
}

func (s *GitHubLFSStorage) lfsBatchRequest(ctx context.Context, batch *LFSBatchRequest) (*LFSBatchResponse, error) {
	// Marshal request
	data, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	url := fmt.Sprintf("%s/objects/batch", s.config.LFSEndpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/vnd.git-lfs+json")
	req.Header.Set("Accept", "application/vnd.git-lfs+json")

	// Sign request
	if err := s.credentials.SignRequest(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to sign request: %w", err)
	}

	// Execute request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("batch request failed with status %d", resp.StatusCode)
	}

	// Parse response
	var batchResp LFSBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &batchResp, nil
}

func calculateOID(file *os.File) (string, error) {
	// LFS uses SHA256 for OID
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func calculateOIDFromBytes(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}
