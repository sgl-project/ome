package vibe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	require.Equal(t, config.MetadataFilePath, DefaultMetadataFilePath)
}

func TestConfig_Validate(t *testing.T) {
	var config Config

	t.Run("error on empty file path", func(t *testing.T) {
		err := config.Validate()
		require.ErrorContainsf(t, err, "vibe_metadata_file_path empty", "unexpected error message")
	})

	t.Run("happy path", func(t *testing.T) {
		config.MetadataFilePath = "/path/to/metadata.json"
		err := config.Validate()
		require.NoError(t, err)
	})
}
