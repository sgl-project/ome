package replica

import (
	"fmt"
	"testing"

	"github.com/sgl-project/ome/internal/ome-agent/replica/common"

	"github.com/sgl-project/ome/pkg/afero"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	hf "github.com/sgl-project/ome/pkg/hfutil/hub"
	"github.com/sgl-project/ome/pkg/ociobjectstore"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

// Define reusable struct types to avoid repetition
type SourceStruct struct {
	StorageURIStr  string `mapstructure:"storage_uri" validate:"required"`
	OCIOSDataStore *ociobjectstore.OCIOSDataStore
	HubClient      *hf.HubClient
	PVCFileSystem  *afero.OsFs
}

type TargetStruct struct {
	StorageURIStr  string `mapstructure:"storage_uri" validate:"required"`
	OCIOSDataStore *ociobjectstore.OCIOSDataStore
	PVCFileSystem  *afero.OsFs
	ChecksumConfig *common.ChecksumConfig `mapstructure:"checksum"`
}

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
		{
			name: "option returns error",
			options: []Option{
				func(c *Config) error { return fmt.Errorf("fail") },
			},
			expectError: true,
			errorMsg:    "fail",
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
				v.Set("num_connections", 5)
				v.Set("download_size_limit_gb", 100)
				v.Set("enable_size_limit_check", true)
				v.Set("source.storage_uri", "oci://n/test-src-namespace/b/test-src-bucket/o/models")
				v.Set("target.storage_uri", "oci://n/test-tgt-namespace/b/test-tgt-bucket/o/models")
				return v
			},
			expectError: false,
			validateFunc: func(t *testing.T, c *Config) {
				assert.Equal(t, "/test/path", c.LocalPath)
				assert.Equal(t, 5, c.NumConnections)
				assert.Equal(t, 100, c.DownloadSizeLimitGB)
				assert.Equal(t, true, c.EnableSizeLimitCheck)
				assert.Equal(t, "oci://n/test-src-namespace/b/test-src-bucket/o/models", c.Source.StorageURIStr)
				assert.Equal(t, "oci://n/test-tgt-namespace/b/test-tgt-bucket/o/models", c.Target.StorageURIStr)
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
		{
			name: "invalid configuration causing unmarshal error",
			setupViper: func() *viper.Viper {
				v := viper.New()
				// Set invalid type for num_connections (string instead of int)
				v.Set("num_connections", "invalid_int_value")
				return v
			},
			expectError:  true,
			validateFunc: nil,
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
	sourceDataStore := &ociobjectstore.OCIOSDataStore{
		Config: &ociobjectstore.Config{Name: ociobjectstore.SourceOsConfigName},
	}
	targetDataStore := &ociobjectstore.OCIOSDataStore{
		Config: &ociobjectstore.Config{Name: ociobjectstore.TargetOsConfigName},
	}
	mockHubClient := &hf.HubClient{}
	sourcePVCFileSystem := afero.NewOsFs().(*afero.OsFs)
	targetPVCFileSystem := afero.NewOsFs().(*afero.OsFs)

	tests := []struct {
		name                string
		dataStores          []*ociobjectstore.OCIOSDataStore
		hubClient           *hf.HubClient
		sourcePVCFileSystem *afero.OsFs
		targetPVCFileSystem *afero.OsFs
		expectSource        *ociobjectstore.OCIOSDataStore
		expectTarget        *ociobjectstore.OCIOSDataStore
		expectHubClient     *hf.HubClient
		expectSourcePVC     *afero.OsFs
		expectTargetPVC     *afero.OsFs
	}{
		{
			name:         "both source OCI data store and target OCI data store present",
			dataStores:   []*ociobjectstore.OCIOSDataStore{sourceDataStore, targetDataStore},
			expectSource: sourceDataStore,
			expectTarget: targetDataStore,
		},
		{
			name:            "hub client and target OCI data store present",
			dataStores:      []*ociobjectstore.OCIOSDataStore{targetDataStore},
			hubClient:       mockHubClient,
			expectSource:    nil,
			expectTarget:    targetDataStore,
			expectHubClient: mockHubClient,
		},
		{
			name:                "source and target PVC file systems present",
			sourcePVCFileSystem: sourcePVCFileSystem,
			targetPVCFileSystem: targetPVCFileSystem,
			expectSourcePVC:     sourcePVCFileSystem,
			expectTargetPVC:     targetPVCFileSystem,
		},
		{
			name:                "mixed OCI and PVC storage",
			dataStores:          []*ociobjectstore.OCIOSDataStore{sourceDataStore},
			targetPVCFileSystem: targetPVCFileSystem,
			expectSource:        sourceDataStore,
			expectTargetPVC:     targetPVCFileSystem,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := replicaParams{
				AnotherLogger:       testingPkg.SetupMockLogger(),
				OCIOSDataStoreList:  tt.dataStores,
				HubClient:           tt.hubClient,
				SourcePVCFileSystem: tt.sourcePVCFileSystem,
				TargetPVCFileSystem: tt.targetPVCFileSystem,
			}
			config := &Config{}
			err := WithAppParams(params)(config)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectSource, config.Source.OCIOSDataStore)
			assert.Equal(t, tt.expectTarget, config.Target.OCIOSDataStore)
			assert.Equal(t, tt.expectHubClient, config.Source.HubClient)
			assert.Equal(t, tt.expectSourcePVC, config.Source.PVCFileSystem)
			assert.Equal(t, tt.expectTargetPVC, config.Target.PVCFileSystem)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	// Define common test values
	validSourceURI := "oci://n/src-ns/b/src-bucket/o/models"
	validTargetURI := "oci://n/tgt-ns/b/tgt-bucket/o/models"
	invalidURI := "invalid-uri"

	tests := []struct {
		name        string
		setupConfig func() *Config
		expectError bool
	}{
		{
			name: "valid config",
			setupConfig: func() *Config {
				return &Config{
					LocalPath:            "/test/path",
					DownloadSizeLimitGB:  100,
					EnableSizeLimitCheck: true,
					NumConnections:       5,
					Source: SourceStruct{
						StorageURIStr: validSourceURI,
					},
					Target: TargetStruct{
						StorageURIStr: validTargetURI,
					},
				}
			},
			expectError: false,
		},
		{
			name: "missing LocalPath",
			setupConfig: func() *Config {
				return &Config{
					DownloadSizeLimitGB:  100,
					EnableSizeLimitCheck: true,
					NumConnections:       5,
					Source: SourceStruct{
						StorageURIStr: validSourceURI,
					},
					Target: TargetStruct{
						StorageURIStr: validTargetURI,
					},
				}
			},
			expectError: true,
		},
		{
			name: "missing Source StorageURIStr",
			setupConfig: func() *Config {
				return &Config{
					LocalPath:            "/test/path",
					DownloadSizeLimitGB:  100,
					EnableSizeLimitCheck: true,
					NumConnections:       5,
					Source: SourceStruct{
						StorageURIStr: "",
					},
					Target: TargetStruct{
						StorageURIStr: validTargetURI,
					},
				}
			},
			expectError: true,
		},
		{
			name: "missing Target StorageURIStr",
			setupConfig: func() *Config {
				return &Config{
					LocalPath:            "/test/path",
					DownloadSizeLimitGB:  100,
					EnableSizeLimitCheck: true,
					NumConnections:       5,
					Source: SourceStruct{
						StorageURIStr: validSourceURI,
					},
					Target: TargetStruct{
						StorageURIStr: "",
					},
				}
			},
			expectError: true,
		},
		{
			name: "invalid source storage URI",
			setupConfig: func() *Config {
				return &Config{
					LocalPath:            "/test/path",
					DownloadSizeLimitGB:  100,
					EnableSizeLimitCheck: true,
					NumConnections:       5,
					Source: SourceStruct{
						StorageURIStr: invalidURI,
					},
					Target: TargetStruct{
						StorageURIStr: validTargetURI,
					},
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

func TestConfig_ValidateRequiredDependencies(t *testing.T) {
	// Create mock objects for testing
	mockOCIOSDataStore := &ociobjectstore.OCIOSDataStore{}
	mockHubClient := &hf.HubClient{}

	tests := []struct {
		name              string
		setupConfig       func() *Config
		sourceStorageType storage.StorageType
		targetStorageType storage.StorageType
		expectError       bool
		expectedErrorMsg  string
	}{
		{
			name: "valid OCI source and OCI target with all dependencies",
			setupConfig: func() *Config {
				return &Config{
					Source: SourceStruct{
						OCIOSDataStore: mockOCIOSDataStore,
					},
					Target: TargetStruct{
						OCIOSDataStore: mockOCIOSDataStore,
					},
				}
			},
			sourceStorageType: storage.StorageTypeOCI,
			targetStorageType: storage.StorageTypeOCI,
			expectError:       false,
		},
		{
			name: "valid HuggingFace source and OCI target with all dependencies",
			setupConfig: func() *Config {
				return &Config{
					Source: SourceStruct{
						HubClient: mockHubClient,
					},
					Target: TargetStruct{
						OCIOSDataStore: mockOCIOSDataStore,
					},
				}
			},
			sourceStorageType: storage.StorageTypeHuggingFace,
			targetStorageType: storage.StorageTypeOCI,
			expectError:       false,
		},
		{
			name: "missing Source.OCIOSDataStore for OCI source",
			setupConfig: func() *Config {
				return &Config{
					Source: SourceStruct{
						OCIOSDataStore: nil,
					},
					Target: TargetStruct{
						OCIOSDataStore: mockOCIOSDataStore,
					},
				}
			},
			sourceStorageType: storage.StorageTypeOCI,
			targetStorageType: storage.StorageTypeOCI,
			expectError:       true,
			expectedErrorMsg:  "required Source.OCIOSDataStore is nil",
		},
		{
			name: "missing Source.HubClient for HuggingFace source",
			setupConfig: func() *Config {
				return &Config{
					Source: SourceStruct{
						HubClient: nil,
					},
					Target: TargetStruct{
						OCIOSDataStore: mockOCIOSDataStore,
					},
				}
			},
			sourceStorageType: storage.StorageTypeHuggingFace,
			targetStorageType: storage.StorageTypeOCI,
			expectError:       true,
			expectedErrorMsg:  "required Source.HubClient is nil",
		},
		{
			name: "missing Target.OCIOSDataStore for OCI target",
			setupConfig: func() *Config {
				return &Config{
					Source: SourceStruct{
						OCIOSDataStore: mockOCIOSDataStore,
					},
					Target: TargetStruct{
						OCIOSDataStore: nil,
					},
				}
			},
			sourceStorageType: storage.StorageTypeOCI,
			targetStorageType: storage.StorageTypeOCI,
			expectError:       true,
			expectedErrorMsg:  "required Target.OCIOSDataStore is nil",
		},
		{
			name: "valid PVC source and PVC target with all dependencies",
			setupConfig: func() *Config {
				return &Config{
					Source: SourceStruct{
						PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
					},
					Target: TargetStruct{
						PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
					},
				}
			},
			sourceStorageType: storage.StorageTypePVC,
			targetStorageType: storage.StorageTypePVC,
			expectError:       false,
		},
		{
			name: "missing Source.PVCFileSystem for PVC source",
			setupConfig: func() *Config {
				return &Config{
					Source: SourceStruct{
						PVCFileSystem: nil,
					},
					Target: TargetStruct{
						PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
					},
				}
			},
			sourceStorageType: storage.StorageTypePVC,
			targetStorageType: storage.StorageTypePVC,
			expectError:       true,
			expectedErrorMsg:  "required Source.PVCFileSystem is nil",
		},
		{
			name: "valid HuggingFace source and PVC target with all dependencies",
			setupConfig: func() *Config {
				return &Config{
					Source: SourceStruct{
						HubClient: mockHubClient,
					},
					Target: TargetStruct{
						PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
					},
				}
			},
			sourceStorageType: storage.StorageTypeHuggingFace,
			targetStorageType: storage.StorageTypePVC,
			expectError:       false,
		},
		{
			name: "valid OCI source and PVC target with all dependencies",
			setupConfig: func() *Config {
				return &Config{
					Source: SourceStruct{
						OCIOSDataStore: mockOCIOSDataStore,
					},
					Target: TargetStruct{
						PVCFileSystem: afero.NewOsFs().(*afero.OsFs),
					},
				}
			},
			sourceStorageType: storage.StorageTypeOCI,
			targetStorageType: storage.StorageTypePVC,
			expectError:       false,
		},
		{
			name: "missing Target.PVCFileSystem for OCI source and PVC target",
			setupConfig: func() *Config {
				return &Config{
					Source: SourceStruct{
						OCIOSDataStore: mockOCIOSDataStore,
					},
					Target: TargetStruct{
						PVCFileSystem: nil,
					},
				}
			},
			sourceStorageType: storage.StorageTypeOCI,
			targetStorageType: storage.StorageTypePVC,
			expectError:       true,
			expectedErrorMsg:  "required Target.PVCFileSystem is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.setupConfig()
			err := config.ValidateRequiredDependencies(tt.sourceStorageType, tt.targetStorageType)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedErrorMsg != "" {
					assert.Contains(t, err.Error(), tt.expectedErrorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
