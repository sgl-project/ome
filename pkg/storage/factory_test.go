package storage

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sgl-project/ome/pkg/logging"
)

// mockStorage is a mock storage implementation for testing
type mockStorage struct {
	provider Provider
}

func (m *mockStorage) Provider() Provider {
	return m.provider
}

func (m *mockStorage) Download(ctx context.Context, source string, target string, opts ...DownloadOption) error {
	return nil
}

func (m *mockStorage) Upload(ctx context.Context, source string, target string, opts ...UploadOption) error {
	return nil
}

func (m *mockStorage) Get(ctx context.Context, uri string) (io.ReadCloser, error) {
	return io.NopCloser(nil), nil
}

func (m *mockStorage) Put(ctx context.Context, uri string, reader io.Reader, size int64, opts ...UploadOption) error {
	return nil
}

func (m *mockStorage) Delete(ctx context.Context, uri string) error {
	return nil
}

func (m *mockStorage) Exists(ctx context.Context, uri string) (bool, error) {
	return true, nil
}

func (m *mockStorage) List(ctx context.Context, uri string, opts ...ListOption) ([]ObjectInfo, error) {
	return []ObjectInfo{}, nil
}

func (m *mockStorage) Stat(ctx context.Context, uri string) (*Metadata, error) {
	return &Metadata{Name: uri}, nil
}

func (m *mockStorage) Copy(ctx context.Context, source string, target string) error {
	return nil
}

func TestFactory_Register(t *testing.T) {
	logger := logging.Discard()
	factory := NewFactory(logger)

	// Test successful registration
	err := factory.Register(ProviderS3, func(ctx context.Context, config Config, logger logging.Interface) (Storage, error) {
		return &mockStorage{provider: ProviderS3}, nil
	})
	assert.NoError(t, err)

	// Test duplicate registration
	err = factory.Register(ProviderS3, func(ctx context.Context, config Config, logger logging.Interface) (Storage, error) {
		return &mockStorage{provider: ProviderS3}, nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")

	// Test empty provider registration
	err = factory.Register("", func(ctx context.Context, config Config, logger logging.Interface) (Storage, error) {
		return &mockStorage{}, nil
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid storage provider")
}

func TestFactory_CreateStorage(t *testing.T) {
	logger := logging.Discard()
	factory := NewFactory(logger)

	// Register a mock storage
	factory.Register(ProviderS3, func(ctx context.Context, config Config, logger logging.Interface) (Storage, error) {
		return &mockStorage{provider: ProviderS3}, nil
	})

	tests := []struct {
		name      string
		config    Config
		wantErr   bool
		errString string
	}{
		{
			name: "successful creation",
			config: Config{
				Provider: ProviderS3,
				Bucket:   "test-bucket",
				AuthConfig: &AuthConfig{
					Provider: "aws",
					Type:     "access_key",
				},
			},
			wantErr: false,
		},
		{
			name: "missing bucket for cloud storage",
			config: Config{
				Provider: ProviderS3,
				AuthConfig: &AuthConfig{
					Provider: "aws",
					Type:     "access_key",
				},
			},
			wantErr:   true,
			errString: "bucket is required",
		},
		{
			name: "missing auth for cloud storage",
			config: Config{
				Provider: ProviderS3,
				Bucket:   "test-bucket",
			},
			wantErr:   true,
			errString: "auth configuration is required",
		},
		{
			name: "unregistered provider",
			config: Config{
				Provider: ProviderAzure,
				Bucket:   "test-container",
				AuthConfig: &AuthConfig{
					Provider: "azure",
					Type:     "client_secret",
				},
			},
			wantErr:   true,
			errString: "not registered",
		},
		{
			name: "empty storage provider",
			config: Config{
				Provider: "",
			},
			wantErr:   true,
			errString: "invalid configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage, err := factory.CreateStorage(context.Background(), tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errString != "" {
					assert.Contains(t, err.Error(), tt.errString)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, storage)
				assert.Equal(t, tt.config.Provider, storage.Provider())
			}
		})
	}
}

func TestFactory_SupportedProviders(t *testing.T) {
	logger := logging.Discard()
	factory := NewFactory(logger)

	// Initially empty
	providers := factory.SupportedProviders()
	assert.Empty(t, providers)

	// Register some providers
	factory.Register(ProviderS3, func(ctx context.Context, config Config, logger logging.Interface) (Storage, error) {
		return &mockStorage{provider: ProviderS3}, nil
	})
	factory.Register(ProviderAzure, func(ctx context.Context, config Config, logger logging.Interface) (Storage, error) {
		return &mockStorage{provider: ProviderAzure}, nil
	})

	// Check supported providers
	providers = factory.SupportedProviders()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, ProviderS3)
	assert.Contains(t, providers, ProviderAzure)
}

func TestFactory_ValidateConfig(t *testing.T) {
	logger := logging.Discard()
	factory := NewFactory(logger)

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid S3 config",
			config: Config{
				Provider: ProviderS3,
				Bucket:   "test-bucket",
				AuthConfig: &AuthConfig{
					Provider: "aws",
				},
			},
			wantErr: false,
		},
		{
			name: "S3 missing bucket",
			config: Config{
				Provider: ProviderS3,
				AuthConfig: &AuthConfig{
					Provider: "aws",
				},
			},
			wantErr: true,
		},
		{
			name: "S3 missing auth",
			config: Config{
				Provider: ProviderS3,
				Bucket:   "test-bucket",
			},
			wantErr: true,
		},
		{
			name: "valid PVC config",
			config: Config{
				Provider: ProviderPVC,
				Extra: map[string]interface{}{
					"pvc_name": "test-pvc",
				},
			},
			wantErr: false,
		},
		{
			name: "PVC missing extra",
			config: Config{
				Provider: ProviderPVC,
			},
			wantErr: true,
		},
		{
			name: "valid local config",
			config: Config{
				Provider: ProviderLocal,
				Extra: map[string]interface{}{
					"base_path": "/tmp/storage",
				},
			},
			wantErr: false,
		},
		{
			name: "local missing base_path",
			config: Config{
				Provider: ProviderLocal,
				Extra:    map[string]interface{}{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := factory.validateConfig(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGlobalFactory(t *testing.T) {
	logger := logging.Discard()

	// Initialize with logger
	InitGlobalFactory(logger)

	// Get global factory multiple times
	factory1 := GetGlobalFactory()
	factory2 := GetGlobalFactory()

	// Should be the same instance
	assert.Same(t, factory1, factory2)

	// Test MustRegister
	assert.NotPanics(t, func() {
		MustRegister(ProviderGitHub, func(ctx context.Context, config Config, logger logging.Interface) (Storage, error) {
			return &mockStorage{provider: ProviderGitHub}, nil
		})
	})

	// Should panic on duplicate registration
	assert.Panics(t, func() {
		MustRegister(ProviderGitHub, func(ctx context.Context, config Config, logger logging.Interface) (Storage, error) {
			return &mockStorage{provider: ProviderGitHub}, nil
		})
	})
}
