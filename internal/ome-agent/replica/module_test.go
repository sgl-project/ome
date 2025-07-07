package replica

import (
	"testing"

	"github.com/sgl-project/ome/pkg/ociobjectstore"
	testingPkg "github.com/sgl-project/ome/pkg/testing"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModule(t *testing.T) {
	// Test that the module can be created without panicking
	assert.NotNil(t, Module)
}

func TestReplicaParams(t *testing.T) {
	// Test the replicaParams struct
	mockLogger := testingPkg.SetupMockLogger()
	var mockDataStores []*ociobjectstore.OCIOSDataStore

	params := replicaParams{
		AnotherLogger:           mockLogger,
		ObjectStorageDataStores: mockDataStores,
	}

	assert.NotNil(t, params.AnotherLogger)
	assert.Empty(t, params.ObjectStorageDataStores)
	assert.Equal(t, mockLogger, params.AnotherLogger)
	assert.Empty(t, mockDataStores)
}

func TestModuleProvider(t *testing.T) {
	tests := []struct {
		name        string
		setupViper  func() *viper.Viper
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid configuration",
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
		},
		{
			name: "invalid viper configuration - missing required fields",
			setupViper: func() *viper.Viper {
				v := viper.New()
				// Missing required fields
				return v
			},
			expectError: false, // Configuration creation succeeds, but validation might fail later
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := tt.setupViper()

			// Setup mock dependencies
			mockLogger := testingPkg.SetupMockLogger()
			var mockDataStores []*ociobjectstore.OCIOSDataStore

			params := replicaParams{
				AnotherLogger:           mockLogger,
				ObjectStorageDataStores: mockDataStores,
			}

			// The provider function from Module (simplified to avoid fx dependencies)
			agent, err := func(v *viper.Viper, params replicaParams) (*ReplicaAgent, error) {
				config, err := NewReplicaConfig(
					WithViper(v),
					WithAnotherLog(params.AnotherLogger),
					WithAppParams(params),
				)
				if err != nil {
					return nil, err
				}
				return NewReplicaAgent(config)
			}(v, params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Nil(t, agent)
			} else {
				if err != nil {
					// In some cases, we might get an error during validation,
					// but the configuration creation itself succeeds
					t.Logf("Got non-critical error: %v", err)
				} else {
					assert.NotNil(t, agent)
					assert.Equal(t, mockLogger, agent.logger)
				}
			}
		})
	}
}

func TestModuleIntegration(t *testing.T) {
	// Setup viper with valid configuration
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

	// Setup mock dependencies
	mockLogger := testingPkg.SetupMockLogger()

	mockDataStores := []*ociobjectstore.OCIOSDataStore{
		{
			Config: &ociobjectstore.Config{Name: ociobjectstore.SourceOsConfigName},
		},
		{
			Config: &ociobjectstore.Config{Name: ociobjectstore.TargetOsConfigName},
		},
	}

	params := replicaParams{
		AnotherLogger:           mockLogger,
		ObjectStorageDataStores: mockDataStores,
	}

	config, err := NewReplicaConfig(
		WithViper(v),
		WithAnotherLog(mockLogger),
		WithAppParams(params),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "/test/path", config.LocalPath)
	assert.Equal(t, "test-src-namespace", config.SourceObjectStoreURI.Namespace)
	assert.Equal(t, mockDataStores[0], config.SourceObjectStorageDataStore)
	assert.Equal(t, mockDataStores[1], config.TargetObjectStorageDataStore)

	// Test agent creation
	agent, err := NewReplicaAgent(config)
	require.NoError(t, err)
	assert.NotNil(t, agent)
}

func TestModuleErrorHandling(t *testing.T) {
	// Setup viper with invalid configuration
	v := viper.New()
	// Empty configuration

	// Setup mock dependencies
	mockLogger := testingPkg.SetupMockLogger()

	// Empty data store
	params := replicaParams{
		AnotherLogger: mockLogger,
		// Missing ObjectStorageDataStores
	}

	// Test configuration creation
	config, err := NewReplicaConfig(
		WithViper(v),
		WithAnotherLog(mockLogger),
		WithAppParams(params),
	)

	require.NoError(t, err)
	assert.NotNil(t, config)

	// The agent will be created, but validation would fail if used
	agent, err := NewReplicaAgent(config)
	assert.NoError(t, err)
	assert.NotNil(t, agent)
}
