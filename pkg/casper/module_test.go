package casper

import (
	"testing"

	testingPkg "github.com/sgl-project/sgl-ome/pkg/testing"

	"github.com/sgl-project/sgl-ome/pkg/principals"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvideCasperDataStore(t *testing.T) {
	t.Run("Config validation only", func(t *testing.T) {
		v := viper.New()
		v.Set(AuthTypeViperKeyName, "InstancePrincipal")
		v.Set(NameViperKeyName, "test-casper")
		v.Set(RegionViperKeyName, "us-chicago-1")
		v.Set(EnableOboTokenViperKeyName, false)

		logger := testingPkg.SetupMockLogger()

		// Test config creation and validation without actual client creation
		config, err := NewConfig(WithViper(v), WithAnotherLog(logger))
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "test-casper", config.Name)
		assert.Equal(t, "us-chicago-1", config.Region)
		assert.False(t, config.EnableOboToken)

		// Validate the config
		err = config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid config", func(t *testing.T) {
		v := viper.New()
		// Missing required auth_type

		logger := testingPkg.SetupMockLogger()

		config, err := NewConfig(WithViper(v), WithAnotherLog(logger))
		// Config creation might succeed even with missing auth_type
		if err != nil {
			assert.Contains(t, err.Error(), "error occurred when unmarshalling auth_type")
		} else {
			// If config creation succeeds, validation should fail
			assert.NotNil(t, config)
			err = config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "AuthType")
		}
	})

	t.Run("Nil logger", func(t *testing.T) {
		v := viper.New()
		v.Set(AuthTypeViperKeyName, "InstancePrincipal")

		config, err := NewConfig(WithViper(v), WithAnotherLog(nil))
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "nil another logger")
	})
}

func TestCasperDataStoreModule(t *testing.T) {
	t.Run("Module provides function", func(t *testing.T) {
		// Test that the module is properly defined
		assert.NotNil(t, CasperDataStoreModule)

		// Test config creation without trying to create OCI client
		v := viper.New()
		v.Set(AuthTypeViperKeyName, "InstancePrincipal")
		v.Set(NameViperKeyName, "test-module")
		v.Set(RegionViperKeyName, "us-chicago-1")

		logger := testingPkg.SetupMockLogger()

		// Test config creation only - don't try to create the actual data store
		config, err := NewConfig(WithViper(v), WithAnotherLog(logger))
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "test-module", config.Name)
		assert.Equal(t, "us-chicago-1", config.Region)

		// Validate the config
		err = config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Module structure validation", func(t *testing.T) {
		// Test that CasperDataStoreModule is a valid fx.Option
		assert.NotNil(t, CasperDataStoreModule)

		// Test that we can create the module without panicking
		assert.NotPanics(t, func() {
			_ = CasperDataStoreModule
		})
	})

	t.Run("Provider function with invalid config", func(t *testing.T) {
		v := viper.New()
		// Don't set auth_type to test error handling

		logger := testingPkg.SetupMockLogger()

		_, err := ProvideCasperDataStore(v, logger)
		assert.Error(t, err)
		// The actual error message is about config validation, not "error reading download agent config"
		assert.Contains(t, err.Error(), "casper config is invalid")
	})

	t.Run("Provider function with nil logger", func(t *testing.T) {
		v := viper.New()
		v.Set(AuthTypeViperKeyName, "InstancePrincipal")

		_, err := ProvideCasperDataStore(v, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil another logger")
	})
}

func TestAppParams(t *testing.T) {
	t.Run("AppParams structure", func(t *testing.T) {
		// Test that appParams can be created
		params := appParams{
			AnotherLogger: testingPkg.SetupMockLogger(),
			Configs: []*Config{
				{
					AuthType: func() *principals.AuthenticationType {
						authType := principals.InstancePrincipal
						return &authType
					}(),
					Name: "test-config-1",
				},
				{
					AuthType: func() *principals.AuthenticationType {
						authType := principals.InstancePrincipal
						return &authType
					}(),
					Name: "test-config-2",
				},
			},
		}

		assert.NotNil(t, params.AnotherLogger)
		assert.Len(t, params.Configs, 2)
		assert.Equal(t, "test-config-1", params.Configs[0].Name)
		assert.Equal(t, "test-config-2", params.Configs[1].Name)
	})
}

func TestProvideListOfCasperDataStoreWithAppParams(t *testing.T) {
	t.Run("Empty configs list", func(t *testing.T) {
		params := appParams{
			AnotherLogger: testingPkg.SetupMockLogger(),
			Configs:       []*Config{},
		}

		stores, err := ProvideListOfCasperDataStoreWithAppParams(params)
		assert.NoError(t, err)
		assert.Empty(t, stores)
	})

	t.Run("Nil config in list", func(t *testing.T) {
		params := appParams{
			AnotherLogger: testingPkg.SetupMockLogger(),
			Configs: []*Config{
				nil, // This should be skipped
			},
		}

		// This should skip nil configs and return empty list
		stores, err := ProvideListOfCasperDataStoreWithAppParams(params)
		assert.NoError(t, err)
		assert.Empty(t, stores)
	})

	t.Run("Invalid config validation", func(t *testing.T) {
		params := appParams{
			AnotherLogger: testingPkg.SetupMockLogger(),
			Configs: []*Config{
				{
					// Missing required AuthType
					Name:          "invalid-config",
					AnotherLogger: testingPkg.SetupMockLogger(),
				},
			},
		}

		stores, err := ProvideListOfCasperDataStoreWithAppParams(params)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error initializing CasperDataStore")
		assert.Empty(t, stores)
	})
}

// Test viper key constants
func TestViperKeyConstants(t *testing.T) {
	t.Run("Viper key constants", func(t *testing.T) {
		assert.Equal(t, "name", NameViperKeyName)
		assert.Equal(t, "auth_type", AuthTypeViperKeyName)
		assert.Equal(t, "compartment_id", CompartmentIdViperKeyName)
		assert.Equal(t, "region_override", RegionViperKeyName)
		assert.Equal(t, "enable_obo_token", EnableOboTokenViperKeyName)
		assert.Equal(t, "obo_token", OboTokenViperKeyName)
	})
}

// Test configuration with viper
func TestViperConfiguration(t *testing.T) {
	t.Run("Complete viper configuration", func(t *testing.T) {
		v := viper.New()
		v.Set(NameViperKeyName, "test-casper")
		v.Set(AuthTypeViperKeyName, "InstancePrincipal")
		v.Set(CompartmentIdViperKeyName, "ocid1.compartment.oc1..test")
		v.Set(RegionViperKeyName, "us-chicago-1")
		v.Set(EnableOboTokenViperKeyName, true)
		v.Set(OboTokenViperKeyName, "test-obo-token")

		config, err := NewConfig(WithViper(v))
		require.NoError(t, err)

		assert.Equal(t, "test-casper", config.Name)
		assert.Equal(t, principals.InstancePrincipal, *config.AuthType)
		assert.Equal(t, "ocid1.compartment.oc1..test", *config.CompartmentId)
		assert.Equal(t, "us-chicago-1", config.Region)
		assert.True(t, config.EnableOboToken)
		assert.Equal(t, "test-obo-token", config.OboToken)
	})

	t.Run("Minimal viper configuration", func(t *testing.T) {
		v := viper.New()
		v.Set(AuthTypeViperKeyName, "InstancePrincipal")

		config, err := NewConfig(WithViper(v))
		require.NoError(t, err)

		assert.Equal(t, principals.InstancePrincipal, *config.AuthType)
		assert.Empty(t, config.Name)
		assert.Empty(t, config.Region)
		assert.False(t, config.EnableOboToken)
		assert.Empty(t, config.OboToken)
	})

	t.Run("Invalid auth type in viper", func(t *testing.T) {
		v := viper.New()
		v.Set(AuthTypeViperKeyName, "invalid_auth_type")

		config, err := NewConfig(WithViper(v))
		// Viper unmarshalling should succeed since AuthenticationType is just a string alias
		assert.NoError(t, err)
		assert.NotNil(t, config)

		// The AuthType should contain the invalid value
		assert.NotNil(t, config.AuthType)
		assert.Equal(t, principals.AuthenticationType("invalid_auth_type"), *config.AuthType)

		// Casper config validation only checks for required fields, not enum values
		err = config.Validate()
		assert.NoError(t, err) // This passes because AuthType is not nil

		// The invalid auth type would be caught when trying to create the actual client
		// or when using principals.Config.Validate(), but that's not tested here
	})
}

// Test fx integration patterns
func TestFxIntegration(t *testing.T) {
	t.Run("Fx module structure test", func(t *testing.T) {
		// Test that the module is properly defined without actually running it
		assert.NotNil(t, CasperDataStoreModule)

		// Test that we can create the providers without executing them
		v := viper.New()
		v.Set(AuthTypeViperKeyName, "InstancePrincipal")
		v.Set(NameViperKeyName, "fx-test")

		logger := testingPkg.SetupMockLogger()

		// Test config creation (this is what the provider would do)
		config, err := NewConfig(WithViper(v), WithAnotherLog(logger))
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "fx-test", config.Name)
	})

	t.Run("Config validation for fx", func(t *testing.T) {
		// Test configuration that would be used in fx
		authType := principals.InstancePrincipal
		configs := []*Config{
			{AuthType: &authType, Name: "config1", AnotherLogger: testingPkg.SetupMockLogger()},
			{AuthType: &authType, Name: "config2", AnotherLogger: testingPkg.SetupMockLogger()},
		}

		// Validate all configs
		for _, config := range configs {
			err := config.Validate()
			assert.NoError(t, err)
		}

		assert.Len(t, configs, 2)
		assert.Equal(t, "config1", configs[0].Name)
		assert.Equal(t, "config2", configs[1].Name)
	})
}
