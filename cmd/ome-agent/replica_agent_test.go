package main

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	testingPkg "github.com/sgl-project/ome/pkg/testing"
)

func TestOCIOSDataStoreListProvider(t *testing.T) {
	t.Run("Source/Target disabled returns empty wrapper and no error", func(t *testing.T) {
		logger := testingPkg.SetupMockLogger()
		v := viper.New()
		v.Set("source.oci.enabled", false)
		v.Set("target.oci.enabled", false)

		w, err := provideSourceOCIOSDataSourceConfig(logger, v)
		assert.NoError(t, err)
		assert.Nil(t, w.OCIOSDataStoreConfig)

		w, err = provideTargetOCIOSDataStoreConfig(logger, v)
		assert.NoError(t, err)
		assert.Nil(t, w.OCIOSDataStoreConfig)
	})

	t.Run("Source/Target enabled with minimal config sets Name and logger", func(t *testing.T) {
		logger := testingPkg.SetupMockLogger()
		v := viper.New()
		v.Set("source.oci.enabled", true)
		v.Set("target.oci.enabled", true)
		// Minimal config: no other fields set

		w, err := provideSourceOCIOSDataSourceConfig(logger, v)
		assert.NoError(t, err)
		if assert.NotNil(t, w.OCIOSDataStoreConfig) {
			assert.Equal(t, "source", w.OCIOSDataStoreConfig.Name)
			assert.Equal(t, logger, w.OCIOSDataStoreConfig.AnotherLogger)
		}

		w, err = provideTargetOCIOSDataStoreConfig(logger, v)
		assert.NoError(t, err)
		if assert.NotNil(t, w.OCIOSDataStoreConfig) {
			assert.Equal(t, "target", w.OCIOSDataStoreConfig.Name)
			assert.Equal(t, logger, w.OCIOSDataStoreConfig.AnotherLogger)
		}
	})

	t.Run("Source/Target enabled with invalid config key returns error", func(t *testing.T) {
		logger := testingPkg.SetupMockLogger()
		v := viper.New()
		v.Set("source.oci.enabled", true)
		v.Set("target.oci.enabled", true)
		// Set a value that will cause UnmarshalKey to fail
		v.Set("source.oci.enable_obo_token", "invalid_value")
		v.Set("target.oci.auth_type", []int{1, 2, 3})

		_, err := provideSourceOCIOSDataSourceConfig(logger, v)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshalling key source")

		_, err = provideTargetOCIOSDataStoreConfig(logger, v)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshalling key target")
	})
}
