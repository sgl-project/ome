package replica

import (
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/sgl-ome/pkg/ociobjectstore"
	testingPkg "github.com/sgl-project/sgl-ome/pkg/testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewReplicaConfig(t *testing.T) {
	tests := []struct {
		name        string
		options     []Option
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty config",
			options:     []Option{},
			expectError: false,
		},
		{
			name: "config with logger",
			options: []Option{
				WithAnotherLog(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
		{
			name: "config with viper",
			options: []Option{
				WithViper(viper.New()),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := NewReplicaConfig(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, config)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, config)
			}
		})
	}
}

func TestConfig_Apply(t *testing.T) {
	tests := []struct {
		name        string
		options     []Option
		expectError bool
	}{
		{
			name:        "apply empty options",
			options:     []Option{},
			expectError: false,
		},
		{
			name: "apply valid options",
			options: []Option{
				WithAnotherLog(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
		{
			name: "apply nil option",
			options: []Option{
				nil,
				WithAnotherLog(testingPkg.SetupMockLogger()),
			},
			expectError: false,
		},
		{
			name: "apply option that returns error",
			options: []Option{
				func(c *Config) error {
					return assert.AnError
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{}
			err := config.Apply(tt.options...)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithAnotherLog(t *testing.T) {
	mockLogger := testingPkg.SetupMockLogger()
	config := &Config{}
	option := WithAnotherLog(mockLogger)

	err := option(config)
	assert.NoError(t, err)
	assert.Equal(t, mockLogger, config.AnotherLogger)
}

func TestWithViper(t *testing.T) {
	tests := []struct {
		name         string
		setupViper   func() *viper.Viper
		expectError  bool
		validateFunc func(*testing.T, *Config)
	}{
		{
			name: "valid viper config",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("local_path", "/test/path")
				v.Set("source.namespace", "test-src-namespace")
				v.Set("source.bucket_name", "test-src-bucket")
				v.Set("source.prefix", "models")
				v.Set("target.namespace", "test-tgt-namespace")
				v.Set("target.bucket_name", "test-tgt-bucket")
				v.Set("target.prefix", "models")
				v.Set("num_connections", 5)
				v.Set("download_size_limit_gb", 100)
				v.Set("enable_size_limit_check", true)
				return v
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *Config) {
				assert.Equal(t, "/test/path", c.LocalPath)
				assert.Equal(t, "test-src-namespace", c.SourceObjectStoreURI.Namespace)
				assert.Equal(t, "test-src-bucket", c.SourceObjectStoreURI.BucketName)
				assert.Equal(t, "models/", c.SourceObjectStoreURI.Prefix)
				assert.Equal(t, "test-tgt-namespace", c.TargetObjectStoreURI.Namespace)
				assert.Equal(t, "test-tgt-bucket", c.TargetObjectStoreURI.BucketName)
				assert.Equal(t, "models/", c.TargetObjectStoreURI.Prefix)
				assert.Equal(t, 5, c.NumConnections)
				assert.Equal(t, 100, c.DownloadSizeLimitGB)
				assert.Equal(t, true, c.EnableSizeLimitCheck)
			},
		},
		{
			name: "prefix without trailing slash",
			setupViper: func() *viper.Viper {
				v := viper.New()
				v.Set("local_path", "/test/path")
				v.Set("source.namespace", "test-namespace")
				v.Set("source.bucket_name", "test-bucket")
				v.Set("source.prefix", "models")
				v.Set("target.namespace", "test-namespace")
				v.Set("target.bucket_name", "test-bucket")
				v.Set("target.prefix", "target")
				return v
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *Config) {
				assert.Equal(t, "models/", c.SourceObjectStoreURI.Prefix)
				assert.Equal(t, "target/", c.TargetObjectStoreURI.Prefix)
			},
		},
		{
			name: "empty viper config",
			setupViper: func() *viper.Viper {
				return viper.New()
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *Config) {
				// Should set defaults
				assert.Equal(t, 10, c.NumConnections)
				assert.Equal(t, 650, c.DownloadSizeLimitGB)
				assert.Equal(t, true, c.EnableSizeLimitCheck)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.setupViper()
			config := &Config{}

			option := WithViper(v)
			err := option(config)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, config)
				}
			}
		})
	}
}

func TestWithAppParams(t *testing.T) {
	mockDataStore := &ociobjectstore.OCIOSDataStore{}
	params := replicaParams{
		AnotherLogger:           testingPkg.SetupMockLogger(),
		ObjectStorageDataStores: mockDataStore,
	}

	config := &Config{}
	err := WithAppParams(params)(config)

	assert.NoError(t, err)
	assert.Equal(t, mockDataStore, config.ObjectStorageDataStore)
}

func TestConfig_Validate(t *testing.T) {
	// Create a mock OCI client to satisfy the required validation
	ociClient, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(common.DefaultConfigProvider())
	if err != nil {
		t.Skip("Skipping test due to OCI client initialization failure")
	}

	tests := []struct {
		name        string
		setupConfig func() *Config
		expectError bool
	}{
		{
			name: "valid config",
			setupConfig: func() *Config {
				return &Config{
					LocalPath:              "/test/path",
					DownloadSizeLimitGB:    100,
					EnableSizeLimitCheck:   true,
					NumConnections:         5,
					SourceObjectStoreURI:   ociobjectstore.ObjectURI{Namespace: "src-ns", BucketName: "src-bucket"},
					TargetObjectStoreURI:   ociobjectstore.ObjectURI{Namespace: "tgt-ns", BucketName: "tgt-bucket"},
					ObjectStorageDataStore: &ociobjectstore.OCIOSDataStore{Client: &ociClient},
				}
			},
			expectError: false,
		},
		{
			name: "missing LocalPath",
			setupConfig: func() *Config {
				return &Config{
					DownloadSizeLimitGB:    100,
					EnableSizeLimitCheck:   true,
					NumConnections:         5,
					SourceObjectStoreURI:   ociobjectstore.ObjectURI{Namespace: "src-ns", BucketName: "src-bucket"},
					TargetObjectStoreURI:   ociobjectstore.ObjectURI{Namespace: "tgt-ns", BucketName: "tgt-bucket"},
					ObjectStorageDataStore: &ociobjectstore.OCIOSDataStore{Client: &ociClient},
				}
			},
			expectError: true,
		},
		{
			name: "missing SourceObjectStoreURI",
			setupConfig: func() *Config {
				return &Config{
					LocalPath:              "/test/path",
					DownloadSizeLimitGB:    100,
					EnableSizeLimitCheck:   true,
					NumConnections:         5,
					TargetObjectStoreURI:   ociobjectstore.ObjectURI{Namespace: "tgt-ns", BucketName: "tgt-bucket"},
					ObjectStorageDataStore: &ociobjectstore.OCIOSDataStore{Client: &ociClient},
				}
			},
			expectError: true,
		},
		{
			name: "missing TargetObjectStoreURI",
			setupConfig: func() *Config {
				return &Config{
					LocalPath:              "/test/path",
					DownloadSizeLimitGB:    100,
					EnableSizeLimitCheck:   true,
					NumConnections:         5,
					SourceObjectStoreURI:   ociobjectstore.ObjectURI{Namespace: "src-ns", BucketName: "src-bucket"},
					ObjectStorageDataStore: &ociobjectstore.OCIOSDataStore{Client: &ociClient},
				}
			},
			expectError: true,
		},
		{
			name: "missing ObjectStorageDataStore",
			setupConfig: func() *Config {
				return &Config{
					LocalPath:            "/test/path",
					DownloadSizeLimitGB:  100,
					EnableSizeLimitCheck: true,
					NumConnections:       5,
					SourceObjectStoreURI: ociobjectstore.ObjectURI{Namespace: "src-ns", BucketName: "src-bucket"},
					TargetObjectStoreURI: ociobjectstore.ObjectURI{Namespace: "tgt-ns", BucketName: "tgt-bucket"},
				}
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.setupConfig()
			err := config.Validate()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := defaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 10, config.NumConnections)
	assert.Equal(t, 650, config.DownloadSizeLimitGB)
	assert.Equal(t, true, config.EnableSizeLimitCheck)
}
