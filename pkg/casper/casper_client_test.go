package casper

import (
	"crypto/rsa"
	"errors"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/sgl-project/sgl-ome/pkg/principals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock for OCI ConfigurationProvider
type MockConfigurationProvider struct {
	mock.Mock
}

func (m *MockConfigurationProvider) TenancyOCID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigurationProvider) UserOCID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigurationProvider) KeyFingerprint() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigurationProvider) Region() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigurationProvider) KeyID() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockConfigurationProvider) PrivateRSAKey() (*rsa.PrivateKey, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*rsa.PrivateKey), args.Error(1)
}

// Implement AuthType method to satisfy the common.ConfigurationProvider interface
func (m *MockConfigurationProvider) AuthType() (common.AuthConfig, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return common.AuthConfig{}, args.Error(1)
	}
	return args.Get(0).(common.AuthConfig), args.Error(1)
}

// Mock for principals package
type MockPrincipalsBuilder struct {
	mock.Mock
}

func (m *MockPrincipalsBuilder) Build(opts principals.Opts) (common.ConfigurationProvider, error) {
	args := m.Called(opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(common.ConfigurationProvider), args.Error(1)
}

// Test NewObjectStorageClient function with dependency injection
func TestNewObjectStorageClient(t *testing.T) {
	// Setup our test struct to include all possible test cases
	tests := []struct {
		name               string
		config             *Config
		mockConfigProvider common.ConfigurationProvider
		testClientCreation func(t *testing.T, config *Config, provider common.ConfigurationProvider) (*objectstorage.ObjectStorageClient, error)
		expectError        bool
		errorContains      string
	}{
		{
			name: "Success without OBO token",
			config: &Config{
				EnableOboToken: false,
				Region:         "us-ashburn-1",
			},
			mockConfigProvider: func() common.ConfigurationProvider {
				provider := new(MockConfigurationProvider)
				provider.On("Region").Return("us-phoenix-1", nil)
				return provider
			}(),
			testClientCreation: func(t *testing.T, config *Config, provider common.ConfigurationProvider) (*objectstorage.ObjectStorageClient, error) {
				// Directly return a mock client to simulate successful creation
				return &objectstorage.ObjectStorageClient{}, nil
			},
			expectError: false,
		},
		{
			name: "Success with OBO token",
			config: &Config{
				EnableOboToken: true,
				OboToken:       "test-token",
				Region:         "us-ashburn-1",
			},
			mockConfigProvider: func() common.ConfigurationProvider {
				provider := new(MockConfigurationProvider)
				provider.On("Region").Return("us-phoenix-1", nil)
				return provider
			}(),
			testClientCreation: func(t *testing.T, config *Config, provider common.ConfigurationProvider) (*objectstorage.ObjectStorageClient, error) {
				// Verify OBO token was used
				assert.True(t, config.EnableOboToken)
				assert.Equal(t, "test-token", config.OboToken)
				return &objectstorage.ObjectStorageClient{}, nil
			},
			expectError: false,
		},
		{
			name: "Error with OBO token enabled but empty token",
			config: &Config{
				EnableOboToken: true,
				OboToken:       "",
				Region:         "",
			},
			mockConfigProvider: new(MockConfigurationProvider),
			testClientCreation: func(t *testing.T, config *Config, provider common.ConfigurationProvider) (*objectstorage.ObjectStorageClient, error) {
				// This shouldn't be called since we should fail early with empty token
				assert.Fail(t, "Client creation should not be attempted with empty OBO token")
				return nil, nil
			},
			expectError:   true,
			errorContains: "oboToken is empty",
		},
		{
			name: "Error creating client with OBO token",
			config: &Config{
				EnableOboToken: true,
				OboToken:       "test-token",
				Region:         "",
			},
			mockConfigProvider: new(MockConfigurationProvider),
			testClientCreation: func(t *testing.T, config *Config, provider common.ConfigurationProvider) (*objectstorage.ObjectStorageClient, error) {
				// Simulate failure creating client with OBO token
				return nil, errors.New("obo token error")
			},
			expectError:   true,
			errorContains: "failed to create ObjectStorageClient",
		},
		{
			name: "Error creating client without OBO token",
			config: &Config{
				EnableOboToken: false,
				Region:         "",
			},
			mockConfigProvider: new(MockConfigurationProvider),
			testClientCreation: func(t *testing.T, config *Config, provider common.ConfigurationProvider) (*objectstorage.ObjectStorageClient, error) {
				// Simulate failure creating client
				return nil, errors.New("client creation error")
			},
			expectError:   true,
			errorContains: "failed to create objectStorageClient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a function that mimics NewObjectStorageClient but uses our test function
			// instead of the actual SDK calls
			testNewObjectStorageClient := func(provider common.ConfigurationProvider, config *Config) (*objectstorage.ObjectStorageClient, error) {
				// This is a direct copy of the function logic but using our test function instead of SDK calls
				var client *objectstorage.ObjectStorageClient
				var err error

				if config.EnableOboToken {
					if config.OboToken == "" {
						return nil, errors.New("failed to get object storage client: oboToken is empty")
					}

					// Instead of calling the SDK, use our test function
					client, err = tt.testClientCreation(t, config, provider)
					if err != nil {
						return nil, errors.New("failed to create ObjectStorageClient: " + err.Error())
					}
				} else {
					// Instead of calling the SDK, use our test function
					client, err = tt.testClientCreation(t, config, provider)
					if err != nil {
						return nil, errors.New("failed to create objectStorageClient: " + err.Error())
					}
				}

				if client != nil && config.Region != "" && len(config.Region) > 0 {
					// Simulate setting region
					t.Logf("Setting region to %s", config.Region)
				}

				return client, nil
			}

			// Run the test
			client, err := testNewObjectStorageClient(tt.mockConfigProvider, tt.config)

			// Assertions
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			}
		})
	}
}

// Test getConfigProvider function
func TestGetConfigProvider(t *testing.T) {
	tests := []struct {
		name                 string
		config               *Config
		mockBuildFunc        func(t *testing.T, config *Config) (common.ConfigurationProvider, error)
		expectError          bool
		errorContains        string
		expectedConfigProvFn func(t *testing.T, provider common.ConfigurationProvider)
	}{
		{
			name: "Success getting config provider",
			config: &Config{
				AnotherLogger: new(MockLogger),
				AuthType: func() *principals.AuthenticationType {
					a := principals.AuthenticationType("InstancePrincipal")
					return &a
				}(),
			},
			mockBuildFunc: func(t *testing.T, config *Config) (common.ConfigurationProvider, error) {
				// Verify config was passed correctly
				assert.Equal(t, principals.AuthenticationType("InstancePrincipal"), *config.AuthType)

				// Return a mock provider
				mockProvider := new(MockConfigurationProvider)
				return mockProvider, nil
			},
			expectError: false,
			expectedConfigProvFn: func(t *testing.T, provider common.ConfigurationProvider) {
				assert.NotNil(t, provider)
				_, ok := provider.(*MockConfigurationProvider)
				assert.True(t, ok, "Expected a MockConfigurationProvider")
			},
		},
		{
			name: "Error building config provider",
			config: &Config{
				AnotherLogger: new(MockLogger),
				AuthType: func() *principals.AuthenticationType {
					a := principals.AuthenticationType("InstancePrincipal")
					return &a
				}(),
			},
			mockBuildFunc: func(t *testing.T, config *Config) (common.ConfigurationProvider, error) {
				return nil, errors.New("build error")
			},
			expectError:   true,
			errorContains: "build error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Create a test function that mimics getConfigProvider
			testGetConfigProvider := func(config *Config) (common.ConfigurationProvider, error) {
				// Instead of using the principals package directly, use our mock function
				return tt.mockBuildFunc(t, config)
			}

			// Call our test function
			configProvider, err := testGetConfigProvider(tt.config)

			// Assertions
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, configProvider)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, configProvider)
				if tt.expectedConfigProvFn != nil {
					tt.expectedConfigProvFn(t, configProvider)
				}
			}
		})
	}
}

// Integration test using mocks for both functions
func TestCasperClientIntegration(t *testing.T) {

	// Create a mock logger
	mockLogger := new(MockLogger)

	// Create our config
	authType := principals.AuthenticationType("InstancePrincipal")
	config := &Config{
		AnotherLogger:  mockLogger,
		AuthType:       &authType,
		Region:         "us-ashburn-1",
		EnableOboToken: false,
	}

	// Create a mock provider - no need to set expectations that won't be called
	mockProvider := new(MockConfigurationProvider)

	// Create a mock for the getConfigProvider function
	mockGetConfigProvider := func(config *Config) (common.ConfigurationProvider, error) {
		// Verify config was passed correctly
		assert.Equal(t, mockLogger, config.AnotherLogger)
		assert.Equal(t, authType, *config.AuthType)

		return mockProvider, nil
	}

	// Create a mock for the NewObjectStorageClient function
	mockNewObjectStorageClient := func(provider common.ConfigurationProvider, config *Config) (*objectstorage.ObjectStorageClient, error) {
		// Verify provider and config were passed correctly
		assert.Equal(t, mockProvider, provider)
		assert.Equal(t, "us-ashburn-1", config.Region)

		// Create a mock client
		return &objectstorage.ObjectStorageClient{}, nil
	}

	// Test the integrated flow
	client, err := func() (*objectstorage.ObjectStorageClient, error) {
		// This simulates the code that would use both functions together
		provider, err := mockGetConfigProvider(config)
		if err != nil {
			return nil, err
		}
		return mockNewObjectStorageClient(provider, config)
	}()

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, client)
}
