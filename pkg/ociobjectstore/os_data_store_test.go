package ociobjectstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	testingPkg "github.com/sgl-project/ome/pkg/testing"

	"github.com/sgl-project/ome/pkg/principals"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOCIOSDataStore(t *testing.T) {
	t.Run("Nil config", func(t *testing.T) {
		cds, err := NewOCIOSDataStore(nil)
		assert.Error(t, err)
		assert.Nil(t, cds)
		assert.Contains(t, err.Error(), "ociobjectstore config is nil")
	})

	t.Run("Invalid config", func(t *testing.T) {
		config := &Config{
			// Missing required AuthType
		}

		cds, err := NewOCIOSDataStore(config)
		assert.Error(t, err)
		assert.Nil(t, cds)
		assert.Contains(t, err.Error(), "ociobjectstore config is invalid")
	})

	t.Run("Valid config validation", func(t *testing.T) {
		authType := principals.InstancePrincipal
		config := &Config{
			AuthType: &authType,
			Name:     "test-config",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})
}

func TestIsMultipartMd5(t *testing.T) {
	tests := []struct {
		name     string
		md5      string
		expected bool
	}{
		{
			name:     "Valid multipart MD5",
			md5:      "d41d8cd98f00b204e9800998ecf8427e-5",
			expected: true,
		},
		{
			name:     "Invalid multipart MD5 - no dash",
			md5:      "d41d8cd98f00b204e9800998ecf8427e",
			expected: false,
		},
		{
			name:     "Invalid multipart MD5 - non-numeric part count",
			md5:      "d41d8cd98f00b204e9800998ecf8427e-abc",
			expected: false,
		},
		{
			name:     "Invalid multipart MD5 - multiple dashes",
			md5:      "d41d8cd98f00b204e9800998ecf8427e-5-2",
			expected: false,
		},
		{
			name:     "Empty string",
			md5:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMultipartMd5(tt.md5)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyDownloadDefaults(t *testing.T) {
	t.Run("Nil options", func(t *testing.T) {
		result := applyDownloadDefaults(nil)
		defaults := DefaultDownloadOptions()
		assert.Equal(t, defaults, result)
	})

	t.Run("Partial options", func(t *testing.T) {
		opts := &DownloadOptions{
			SizeThresholdInMB: 200,
			// Other fields left as zero values
		}

		result := applyDownloadDefaults(opts)
		defaults := DefaultDownloadOptions()

		assert.Equal(t, 200, result.SizeThresholdInMB)
		assert.Equal(t, defaults.ChunkSizeInMB, result.ChunkSizeInMB)
		assert.Equal(t, defaults.Threads, result.Threads)
	})

	t.Run("Complete options", func(t *testing.T) {
		opts := &DownloadOptions{
			SizeThresholdInMB: 200,
			ChunkSizeInMB:     16,
			Threads:           50,
			ForceStandard:     true,
		}

		result := applyDownloadDefaults(opts)
		assert.Equal(t, *opts, result)
	})
}

func TestOCIOSDefaultDownloadOptions(t *testing.T) {
	opts := DefaultDownloadOptions()

	assert.Equal(t, defaultThresholdMB, opts.SizeThresholdInMB)
	assert.Equal(t, 8, opts.ChunkSizeInMB)
	assert.Equal(t, 100, opts.Threads)
	assert.False(t, opts.StripPrefix)
	assert.False(t, opts.ForceStandard)
	assert.False(t, opts.ForceMultipart)
	assert.True(t, opts.DisableOverride)
	assert.Empty(t, opts.ExcludePatterns)
	assert.False(t, opts.JoinWithTailOverlap)
	assert.False(t, opts.UseBaseNameOnly)
	assert.Empty(t, opts.PrefixToStrip)
}

func TestObjectURI(t *testing.T) {
	t.Run("Complete ObjectURI", func(t *testing.T) {
		uri := ObjectURI{
			Namespace:  "test-namespace",
			BucketName: "test-bucket",
			ObjectName: "test-object.txt",
			Prefix:     "test-prefix/",
			Region:     "us-chicago-1",
		}

		assert.Equal(t, "test-namespace", uri.Namespace)
		assert.Equal(t, "test-bucket", uri.BucketName)
		assert.Equal(t, "test-object.txt", uri.ObjectName)
		assert.Equal(t, "test-prefix/", uri.Prefix)
		assert.Equal(t, "us-chicago-1", uri.Region)
	})

	t.Run("Minimal ObjectURI", func(t *testing.T) {
		uri := ObjectURI{
			BucketName: "test-bucket",
			ObjectName: "test-object.txt",
		}

		assert.Empty(t, uri.Namespace)
		assert.Equal(t, "test-bucket", uri.BucketName)
		assert.Equal(t, "test-object.txt", uri.ObjectName)
		assert.Empty(t, uri.Prefix)
		assert.Empty(t, uri.Region)
	})
}

// Test the path manipulation functions used in downloads
func TestPathManipulation(t *testing.T) {
	t.Run("ObjectBaseName", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"file.txt", "file.txt"},
			{"path/to/file.txt", "file.txt"},
			{"deep/nested/path/file.txt", "file.txt"},
			{"", ""},
			{"/", ""},
			{"path/", ""},
		}

		for _, tt := range tests {
			result := ObjectBaseName(tt.input)
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		}
	})

	t.Run("TrimObjectPrefix", func(t *testing.T) {
		tests := []struct {
			objectPath string
			prefix     string
			expected   string
		}{
			{"prefix/file.txt", "prefix/", "file.txt"},
			{"models/v1/file.txt", "models/", "v1/file.txt"},
			{"file.txt", "prefix/", "file.txt"},
			{"", "prefix/", ""},
			{"prefix/file.txt", "", "prefix/file.txt"},
		}

		for _, tt := range tests {
			result := TrimObjectPrefix(tt.objectPath, tt.prefix)
			assert.Equal(t, tt.expected, result, "ObjectPath: %s, Prefix: %s", tt.objectPath, tt.prefix)
		}
	})
}

// Test error handling in download options
func TestDownloadOptionsErrorHandling(t *testing.T) {
	t.Run("Invalid option function", func(t *testing.T) {
		invalidOption := func(opts *DownloadOptions) error {
			return fmt.Errorf("invalid option error")
		}

		_, err := applyDownloadOptions(invalidOption)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid option error")
	})

	t.Run("Multiple options with one error", func(t *testing.T) {
		validOption := WithThreads(10)
		invalidOption := func(opts *DownloadOptions) error {
			return fmt.Errorf("invalid option error")
		}

		_, err := applyDownloadOptions(validOption, invalidOption)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid option error")
	})

	t.Run("Nil option in list", func(t *testing.T) {
		opts, err := applyDownloadOptions(WithThreads(10), nil, WithChunkSize(8))
		assert.NoError(t, err)
		assert.Equal(t, 10, opts.Threads)
		assert.Equal(t, 8, opts.ChunkSizeInMB)
	})
}

// Test file operations that don't require OCI client
func TestFileOperations(t *testing.T) {
	t.Run("CopyReaderToFilePath", func(t *testing.T) {
		tempDir := t.TempDir()
		targetFile := filepath.Join(tempDir, "test-file.txt")
		content := "test content for file operations"
		reader := strings.NewReader(content)

		err := CopyReaderToFilePath(reader, targetFile)
		assert.NoError(t, err)

		// Verify file was created with correct content
		assert.FileExists(t, targetFile)
		fileContent, err := os.ReadFile(targetFile)
		assert.NoError(t, err)
		assert.Equal(t, content, string(fileContent))
	})

	t.Run("CopyReaderToFilePath with nested directory", func(t *testing.T) {
		tempDir := t.TempDir()
		targetFile := filepath.Join(tempDir, "nested", "dir", "test-file.txt")
		content := "test content"
		reader := strings.NewReader(content)

		// This should fail because the directory doesn't exist
		err := CopyReaderToFilePath(reader, targetFile)
		assert.Error(t, err)
	})
}

// Test configuration edge cases
func TestConfigurationEdgeCases(t *testing.T) {
	t.Run("Config with OBO token enabled but empty token", func(t *testing.T) {
		authType := principals.InstancePrincipal
		config := &Config{
			AuthType:       &authType,
			EnableOboToken: true,
			OboToken:       "", // Empty token
		}

		err := config.Validate()
		assert.Error(t, err)
	})

	t.Run("Config with OBO token enabled and valid token", func(t *testing.T) {
		authType := principals.InstancePrincipal
		config := &Config{
			AuthType:       &authType,
			EnableOboToken: true,
			OboToken:       "valid-token",
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Config with compartment ID", func(t *testing.T) {
		authType := principals.InstancePrincipal
		compartmentId := "ocid1.compartment.oc1..test"
		config := &Config{
			AuthType:      &authType,
			CompartmentId: &compartmentId,
		}

		err := config.Validate()
		assert.NoError(t, err)
		assert.Equal(t, compartmentId, *config.CompartmentId)
	})
}

// Test download options combinations
func TestDownloadOptionsCombinations(t *testing.T) {
	t.Run("All options combined", func(t *testing.T) {
		opts, err := applyDownloadOptions(
			WithSizeThreshold(150),
			WithChunkSize(32),
			WithThreads(25),
			WithForceStandard(true),
			WithForceMultipart(false),
			WithOverrideEnabled(true),
			WithExcludePatterns([]string{"*.tmp", "*.log"}),
			WithStripPrefix("models/v1/"),
			WithBaseNameOnly(false),
			WithTailOverlap(true),
		)
		require.NoError(t, err)

		assert.Equal(t, 150, opts.SizeThresholdInMB)
		assert.Equal(t, 32, opts.ChunkSizeInMB)
		assert.Equal(t, 25, opts.Threads)
		assert.True(t, opts.ForceStandard)
		assert.False(t, opts.ForceMultipart)
		assert.False(t, opts.DisableOverride) // Override enabled means DisableOverride is false
		assert.Equal(t, []string{"*.tmp", "*.log"}, opts.ExcludePatterns)
		assert.True(t, opts.StripPrefix)
		assert.Equal(t, "models/v1/", opts.PrefixToStrip)
		assert.False(t, opts.UseBaseNameOnly)
		assert.True(t, opts.JoinWithTailOverlap)
	})

	t.Run("Conflicting options", func(t *testing.T) {
		opts, err := applyDownloadOptions(
			WithForceStandard(true),
			WithForceMultipart(true), // Conflicting with above
		)
		require.NoError(t, err)

		// Last option should win
		assert.True(t, opts.ForceStandard)
		assert.True(t, opts.ForceMultipart)
	})

	t.Run("Override options", func(t *testing.T) {
		opts, err := applyDownloadOptions(
			WithBaseNameOnly(true),
			WithStripPrefix("prefix/"),
			WithTailOverlap(true),
		)
		require.NoError(t, err)

		// All path manipulation options can be set simultaneously
		assert.True(t, opts.UseBaseNameOnly)
		assert.True(t, opts.StripPrefix)
		assert.Equal(t, "prefix/", opts.PrefixToStrip)
		assert.True(t, opts.JoinWithTailOverlap)
	})
}

// Test OCIOSDataStore methods that don't require OCI client
func TestOCIOSDataStoreSetRegion(t *testing.T) {
	authType := principals.InstancePrincipal
	config := &Config{
		AuthType:      &authType,
		Name:          "test-config",
		Region:        "us-west-1",
		AnotherLogger: testingPkg.SetupMockLogger(),
	}

	// We can't create a real OCIOSDataStore without OCI client, but we can test the config
	assert.Equal(t, "us-west-1", config.Region)

	// Test region update
	config.Region = "us-chicago-1"
	assert.Equal(t, "us-chicago-1", config.Region)
}

// Test download options application logic
func TestDownloadOptionsApplication(t *testing.T) {
	t.Run("Default options when none provided", func(t *testing.T) {
		opts, err := applyDownloadOptions()
		require.NoError(t, err)

		defaults := DefaultDownloadOptions()
		assert.Equal(t, defaults.SizeThresholdInMB, opts.SizeThresholdInMB)
		assert.Equal(t, defaults.ChunkSizeInMB, opts.ChunkSizeInMB)
		assert.Equal(t, defaults.Threads, opts.Threads)
		assert.Equal(t, defaults.DisableOverride, opts.DisableOverride)
	})

	t.Run("Partial options with defaults", func(t *testing.T) {
		opts, err := applyDownloadOptions(WithThreads(50))
		require.NoError(t, err)

		defaults := DefaultDownloadOptions()
		assert.Equal(t, 50, opts.Threads)                                   // Custom value
		assert.Equal(t, defaults.SizeThresholdInMB, opts.SizeThresholdInMB) // Default
		assert.Equal(t, defaults.ChunkSizeInMB, opts.ChunkSizeInMB)         // Default
	})

	t.Run("Override enabled/disabled logic", func(t *testing.T) {
		// Test override enabled
		opts, err := applyDownloadOptions(WithOverrideEnabled(true))
		require.NoError(t, err)
		assert.False(t, opts.DisableOverride)

		// Test override disabled
		opts, err = applyDownloadOptions(WithOverrideEnabled(false))
		require.NoError(t, err)
		assert.True(t, opts.DisableOverride)
	})
}

// Test constants and default values
func TestOCIOSConstants(t *testing.T) {
	t.Run("Default threshold", func(t *testing.T) {
		assert.Equal(t, 100, defaultThresholdMB)
	})

	t.Run("Max retries", func(t *testing.T) {
		assert.Equal(t, 3, maxRetries)
	})

	t.Run("Retry delay", func(t *testing.T) {
		assert.Equal(t, 2*time.Second, retryDelay)
	})
}

// Test path manipulation functions used in downloads
func TestDownloadPathManipulation(t *testing.T) {
	t.Run("ObjectBaseName edge cases", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"", ""},
			{"/", ""},
			{"file.txt", "file.txt"},
			{"path/to/file.txt", "file.txt"},
			{"deep/nested/path/file.txt", "file.txt"},
			{"path/", ""},
			{"path//file.txt", "file.txt"},
			{"path/to/file with spaces.txt", "file with spaces.txt"},
			{"path/to/file-with-dashes_and_underscores.txt", "file-with-dashes_and_underscores.txt"},
		}

		for _, tt := range tests {
			result := ObjectBaseName(tt.input)
			assert.Equal(t, tt.expected, result, "Input: %s", tt.input)
		}
	})

	t.Run("TrimObjectPrefix edge cases", func(t *testing.T) {
		tests := []struct {
			objectPath string
			prefix     string
			expected   string
		}{
			{"", "", ""},
			{"file.txt", "", "file.txt"},
			{"", "prefix/", ""},
			{"prefix/file.txt", "prefix/", "file.txt"},
			{"models/v1/file.txt", "models/", "v1/file.txt"},
			{"file.txt", "prefix/", "file.txt"},                      // No separator in path
			{"prefix/file.txt", "", "prefix/file.txt"},               // Empty prefix
			{"prefix/prefix/file.txt", "prefix/", "prefix/file.txt"}, // Only first occurrence
		}

		for _, tt := range tests {
			result := TrimObjectPrefix(tt.objectPath, tt.prefix)
			assert.Equal(t, tt.expected, result, "ObjectPath: %s, Prefix: %s", tt.objectPath, tt.prefix)
		}
	})
}

// Test DownloadOptions struct and its methods
func TestDownloadOptionsStruct(t *testing.T) {
	t.Run("Default download options values", func(t *testing.T) {
		opts := DefaultDownloadOptions()

		assert.Equal(t, defaultThresholdMB, opts.SizeThresholdInMB)
		assert.Equal(t, 8, opts.ChunkSizeInMB)
		assert.Equal(t, 100, opts.Threads)
		assert.False(t, opts.StripPrefix)
		assert.False(t, opts.ForceStandard)
		assert.False(t, opts.ForceMultipart)
		assert.True(t, opts.DisableOverride)
		assert.Empty(t, opts.ExcludePatterns)
		assert.False(t, opts.JoinWithTailOverlap)
		assert.False(t, opts.UseBaseNameOnly)
		assert.Empty(t, opts.PrefixToStrip)
	})

	t.Run("Apply download defaults with nil", func(t *testing.T) {
		result := applyDownloadDefaults(nil)
		defaults := DefaultDownloadOptions()
		assert.Equal(t, defaults, result)
	})

	t.Run("Apply download defaults with partial options", func(t *testing.T) {
		opts := &DownloadOptions{
			SizeThresholdInMB: 200,
			ChunkSizeInMB:     0, // Should be filled with default
			Threads:           50,
		}

		result := applyDownloadDefaults(opts)
		assert.Equal(t, 200, result.SizeThresholdInMB)
		assert.Equal(t, 8, result.ChunkSizeInMB) // Default value
		assert.Equal(t, 50, result.Threads)
	})
}

// Test error scenarios
func TestOCIOSErrorScenarios(t *testing.T) {
	t.Run("NewOCIOSDataStore with nil config", func(t *testing.T) {
		cds, err := NewOCIOSDataStore(nil)
		assert.Error(t, err)
		assert.Nil(t, cds)
		assert.Contains(t, err.Error(), "ociobjectstore config is nil")
	})

	t.Run("NewOCIOSDataStore with invalid config", func(t *testing.T) {
		config := &Config{
			// Missing required AuthType
			Name: "invalid-config",
		}

		cds, err := NewOCIOSDataStore(config)
		assert.Error(t, err)
		assert.Nil(t, cds)
		assert.Contains(t, err.Error(), "ociobjectstore config is invalid")
	})
}

// Test ObjectURI validation and usage
func TestObjectURIUsage(t *testing.T) {
	t.Run("Complete ObjectURI", func(t *testing.T) {
		uri := ObjectURI{
			Namespace:  "test-namespace",
			BucketName: "test-bucket",
			ObjectName: "path/to/file.txt",
			Prefix:     "path/",
			Region:     "us-chicago-1",
		}

		assert.Equal(t, "test-namespace", uri.Namespace)
		assert.Equal(t, "test-bucket", uri.BucketName)
		assert.Equal(t, "path/to/file.txt", uri.ObjectName)
		assert.Equal(t, "path/", uri.Prefix)
		assert.Equal(t, "us-chicago-1", uri.Region)
	})

	t.Run("Minimal ObjectURI", func(t *testing.T) {
		uri := ObjectURI{
			BucketName: "test-bucket",
			ObjectName: "file.txt",
		}

		assert.Empty(t, uri.Namespace)
		assert.Equal(t, "test-bucket", uri.BucketName)
		assert.Equal(t, "file.txt", uri.ObjectName)
		assert.Empty(t, uri.Prefix)
		assert.Empty(t, uri.Region)
	})

	t.Run("ObjectURI with special characters", func(t *testing.T) {
		uri := ObjectURI{
			BucketName: "bucket-with-dashes",
			ObjectName: "path/to/file with spaces & special chars.txt",
		}

		assert.Equal(t, "bucket-with-dashes", uri.BucketName)
		assert.Equal(t, "path/to/file with spaces & special chars.txt", uri.ObjectName)
	})
}
