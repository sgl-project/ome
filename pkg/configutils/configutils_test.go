package configutils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testutils "github.com/sgl-project/ome/pkg/testing"
)

const leafConfig = `imports:
  - intermediate.yaml

a:
  b: 1
`

const intermediateConfig = `imports:
  - root.yaml
  -

a:
  c: 2
`

const rootConfig = `
a:
  b: 2
  d: 3
`

const expectedConfig = `a:
    b: 1
    c: 2
    d: 3
imports:
    - intermediate.yaml
`

func TestConfigFileImports(t *testing.T) {
	// TODO: Would be ideal to use afero or similar in the future. Creating
	// files on the actual file system is not the worst thing ever (the Go
	// standard library does this in its own tests), but it's also not the
	// cleanest thing.

	t.Run("should import config files correctly", func(t *testing.T) {
		v := viper.New()

		tempDir, closer, err := testutils.TempDir()
		assert.NoError(t, err, "should not error creating temporary directory")
		defer closer()

		leafConfigPath := filepath.Join(tempDir, "leaf.yaml")
		err = os.WriteFile(leafConfigPath, []byte(leafConfig), 0666)
		assert.NoError(t, err, "should not error writing leaf config")

		intermediateConfigPath := filepath.Join(tempDir, "intermediate.yaml")
		err = os.WriteFile(intermediateConfigPath, []byte(intermediateConfig), 0666)
		assert.NoError(t, err, "should not error writing intermediate config")

		rootConfigPath := filepath.Join(tempDir, "root.yaml")
		err = os.WriteFile(rootConfigPath, []byte(rootConfig), 0666)
		assert.NoError(t, err, "should not error writing root config")

		err = ResolveAndMergeFile(v, leafConfigPath)
		assert.NoError(t, err, "should not error creating config")

		outputConfigPath := filepath.Join(tempDir, "assert.yaml")
		require.NoError(t, v.WriteConfigAs(outputConfigPath))

		writtenConfig, err := os.ReadFile(outputConfigPath)
		assert.NoError(t, err, "should not error reading config file")
		assert.Equal(t, expectedConfig, string(writtenConfig))
	})

	t.Run("should error when importing nonexistent configs", func(t *testing.T) {
		v := viper.New()

		tempDir, closer, err := testutils.TempDir()
		assert.NoError(t, err, "should not error creating temporary directory")
		defer closer()

		// create a nonexistent absolute path and a config referencing it
		nonexistentConfigPath := filepath.Join(tempDir, "nonexistent.yaml")
		badConfig := fmt.Sprintf("imports:\n- \"%s\"", nonexistentConfigPath)

		// write the config
		configPath := filepath.Join(tempDir, "test.yaml")
		err = os.WriteFile(configPath, []byte(badConfig), 0666)
		assert.NoError(t, err, "should not error writing config")

		err = ResolveAndMergeFile(v, configPath)
		assert.Error(t, err, "should error creating config")
		assert.Contains(t, err.Error(), "no such file or directory")
	})

	t.Run("should error when importing malformed configs", func(t *testing.T) {
		v := viper.New()

		tempDir, closer, err := testutils.TempDir()
		assert.NoError(t, err, "should not error creating temporary directory")
		defer closer()

		leafConfigPath := filepath.Join(tempDir, "leaf.yaml")
		err = os.WriteFile(leafConfigPath, []byte(leafConfig), 0666)
		assert.NoError(t, err, "should not error writing leaf config")

		// ensure the intermediate config is malformed
		intermediateConfigPath := filepath.Join(tempDir, "intermediate.yaml")
		err = os.WriteFile(intermediateConfigPath, []byte("malformed"), 0666)
		assert.NoError(t, err, "should not error writing intermediate config")

		err = ResolveAndMergeFile(v, leafConfigPath)
		assert.Error(t, err, "should error creating config")
		assert.Contains(t, err.Error(), "could not resolve configuration imports")
	})

	t.Run("should surface error when it occurs in child config", func(t *testing.T) {
		v := viper.New()

		tempDir, closer, err := testutils.TempDir()
		assert.NoError(t, err, "should not error creating temporary directory")
		defer closer()

		leafConfigPath := filepath.Join(tempDir, "leaf.yaml")
		err = os.WriteFile(leafConfigPath, []byte(leafConfig), 0666)
		assert.NoError(t, err, "should not error writing leaf config")

		intermediateConfigPath := filepath.Join(tempDir, "intermediate.yaml")
		err = os.WriteFile(intermediateConfigPath, []byte(intermediateConfig), 0666)
		assert.NoError(t, err, "should not error writing intermediate config")

		// the root config (referenced by the intermediate config) does not
		// exist, so the error should surface up
		err = ResolveAndMergeFile(v, leafConfigPath)
		assert.Error(t, err, "should error creating config")
		assert.Contains(t, err.Error(), "no such file or directory")
	})
}
