package vibe

import "errors"

const DefaultMetadataFilePath = "/opt/vibe/locale.json"

// Config for vibe env provider.
type Config struct {
	// MetadataFilePath can be used to override default metadata file path.
	MetadataFilePath string `mapstructure:"vibe_metadata_file_path"`
}

// DefaultConfig creates default vibe provider config.
func DefaultConfig() Config {
	return Config{
		MetadataFilePath: DefaultMetadataFilePath,
	}
}

// Validate helps validate the vibe provider config.
func (c Config) Validate() error {
	if c.MetadataFilePath == "" {
		return errors.New("vibe_metadata_file_path empty")
	}

	return nil
}
