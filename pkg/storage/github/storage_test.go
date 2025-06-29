package github

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

func TestGitHubLFSStorage_Provider(t *testing.T) {
	s := &GitHubLFSStorage{
		logger: logging.NewNopLogger(),
	}

	if provider := s.Provider(); provider != storage.ProviderGitHub {
		t.Errorf("Expected provider %s, got %s", storage.ProviderGitHub, provider)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.APIEndpoint != "https://api.github.com" {
		t.Errorf("Expected APIEndpoint https://api.github.com, got %s", cfg.APIEndpoint)
	}

	if cfg.ChunkSize != 100*1024*1024 {
		t.Errorf("Expected ChunkSize 100MB, got %d", cfg.ChunkSize)
	}
}

func TestNew_InvalidCredentials(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewNopLogger()

	// Create non-GitHub credentials
	mockCreds := &mockCredentials{
		provider: auth.ProviderAWS,
	}

	cfg := &Config{
		Owner: "test-owner",
		Repo:  "test-repo",
	}

	_, err := New(ctx, cfg, mockCreds, logger)
	if err == nil {
		t.Error("Expected error for invalid credentials type")
	}
}

func TestNew_MissingOwner(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewNopLogger()

	mockCreds := &mockGitHubCredentials{
		mockCredentials: mockCredentials{
			provider: auth.ProviderGitHub,
		},
	}

	cfg := &Config{
		Repo: "test-repo",
		// Missing Owner
	}

	_, err := New(ctx, cfg, mockCreds, logger)
	if err == nil {
		t.Error("Expected error for missing owner")
	}
}

func TestNew_MissingRepo(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewNopLogger()

	mockCreds := &mockGitHubCredentials{
		mockCredentials: mockCredentials{
			provider: auth.ProviderGitHub,
		},
	}

	cfg := &Config{
		Owner: "test-owner",
		// Missing Repo
	}

	_, err := New(ctx, cfg, mockCreds, logger)
	if err == nil {
		t.Error("Expected error for missing repo")
	}
}

func TestNew_Valid(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewNopLogger()

	mockCreds := &mockGitHubCredentials{
		mockCredentials: mockCredentials{
			provider: auth.ProviderGitHub,
		},
	}

	cfg := &Config{
		Owner: "test-owner",
		Repo:  "test-repo",
	}

	storage, err := New(ctx, cfg, mockCreds, logger)
	if err != nil {
		t.Fatalf("Failed to create GitHub LFS storage: %v", err)
	}

	// Check LFS endpoint was set correctly
	expectedLFS := "https://github.com/test-owner/test-repo.git/info/lfs"
	if storage.config.LFSEndpoint != expectedLFS {
		t.Errorf("Expected LFS endpoint %s, got %s", expectedLFS, storage.config.LFSEndpoint)
	}
}

func TestNew_CustomEndpoints(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewNopLogger()

	mockCreds := &mockGitHubCredentials{
		mockCredentials: mockCredentials{
			provider: auth.ProviderGitHub,
		},
	}

	cfg := &Config{
		Owner:       "test-owner",
		Repo:        "test-repo",
		APIEndpoint: "https://github.enterprise.com/api/v3",
		LFSEndpoint: "https://github.enterprise.com/test-owner/test-repo.git/info/lfs",
	}

	storage, err := New(ctx, cfg, mockCreds, logger)
	if err != nil {
		t.Fatalf("Failed to create GitHub LFS storage: %v", err)
	}

	if storage.apiEndpoint != cfg.APIEndpoint {
		t.Errorf("Expected API endpoint %s, got %s", cfg.APIEndpoint, storage.apiEndpoint)
	}

	if storage.config.LFSEndpoint != cfg.LFSEndpoint {
		t.Errorf("Expected LFS endpoint %s, got %s", cfg.LFSEndpoint, storage.config.LFSEndpoint)
	}
}

func TestUnsupportedOperations(t *testing.T) {
	s := &GitHubLFSStorage{
		logger: logging.NewNopLogger(),
	}
	ctx := context.Background()
	uri := storage.ObjectURI{}

	// Test Delete
	err := s.Delete(ctx, uri)
	if err == nil {
		t.Error("Expected error for Delete operation")
	}

	// Test Copy
	err = s.Copy(ctx, uri, uri)
	if err == nil {
		t.Error("Expected error for Copy operation")
	}

	// Test List
	_, err = s.List(ctx, uri, storage.ListOptions{})
	if err == nil {
		t.Error("Expected error for List operation")
	}

	// Test multipart operations
	_, err = s.InitiateMultipartUpload(ctx, uri)
	if err == nil {
		t.Error("Expected error for InitiateMultipartUpload")
	}

	_, err = s.UploadPart(ctx, uri, "", 1, nil, 0)
	if err == nil {
		t.Error("Expected error for UploadPart")
	}

	err = s.CompleteMultipartUpload(ctx, uri, "", nil)
	if err == nil {
		t.Error("Expected error for CompleteMultipartUpload")
	}

	err = s.AbortMultipartUpload(ctx, uri, "")
	if err == nil {
		t.Error("Expected error for AbortMultipartUpload")
	}
}

func TestCalculateOID(t *testing.T) {
	// Create a temporary file
	content := []byte("Hello, GitHub LFS!")
	tmpFile, err := os.CreateTemp("", "lfs-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		t.Fatalf("Failed to write content: %v", err)
	}
	tmpFile.Seek(0, 0)

	// Calculate OID
	oid, err := calculateOID(tmpFile)
	if err != nil {
		t.Fatalf("Failed to calculate OID: %v", err)
	}

	// Verify OID
	expectedOID := fmt.Sprintf("%x", sha256.Sum256(content))
	if oid != expectedOID {
		t.Errorf("Expected OID %s, got %s", expectedOID, oid)
	}
}

func TestCalculateOIDFromBytes(t *testing.T) {
	content := []byte("Hello, GitHub LFS!")

	oid := calculateOIDFromBytes(content)

	expectedOID := fmt.Sprintf("%x", sha256.Sum256(content))
	if oid != expectedOID {
		t.Errorf("Expected OID %s, got %s", expectedOID, oid)
	}
}

// Mock implementations for testing

type mockCredentials struct {
	provider auth.Provider
}

func (m *mockCredentials) Provider() auth.Provider {
	return m.provider
}

func (m *mockCredentials) Type() auth.AuthType {
	return auth.GitHubPersonalAccessToken
}

func (m *mockCredentials) Token(ctx context.Context) (string, error) {
	return "mock-token", nil
}

func (m *mockCredentials) SignRequest(ctx context.Context, req *http.Request) error {
	req.Header.Set("Authorization", "Bearer mock-token")
	return nil
}

func (m *mockCredentials) Refresh(ctx context.Context) error {
	return nil
}

func (m *mockCredentials) IsExpired() bool {
	return false
}

type mockGitHubCredentials struct {
	mockCredentials
}

func (m *mockGitHubCredentials) GetHTTPClient() *http.Client {
	return http.DefaultClient
}
