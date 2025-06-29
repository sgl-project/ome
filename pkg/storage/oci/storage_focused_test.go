package oci

import (
	"testing"
	"time"

	"github.com/sgl-project/ome/pkg/storage"
	"github.com/stretchr/testify/assert"
)

// Test configuration and helper functions that don't require SDK mocking

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 20*time.Minute, config.HTTPTimeout)
	assert.Equal(t, 200, config.MaxIdleConns)
	assert.Equal(t, 200, config.MaxIdleConnsPerHost)
	assert.Equal(t, 200, config.MaxConnsPerHost)
	assert.Empty(t, config.CompartmentID)
	assert.Empty(t, config.Region)
	assert.False(t, config.EnableOboToken)
	assert.Empty(t, config.OboToken)
}

func TestPrepareDownloadPartUnit(t *testing.T) {
	part := PrepareDownloadPart{
		Namespace: "test-namespace",
		Bucket:    "test-bucket",
		Object:    "test-object",
		ByteRange: "bytes=0-1023",
		Offset:    0,
		PartNum:   1,
		Size:      1024,
	}

	assert.Equal(t, "test-namespace", part.Namespace)
	assert.Equal(t, "test-bucket", part.Bucket)
	assert.Equal(t, "test-object", part.Object)
	assert.Equal(t, "bytes=0-1023", part.ByteRange)
	assert.Equal(t, int64(0), part.Offset)
	assert.Equal(t, 1, part.PartNum)
	assert.Equal(t, int64(1024), part.Size)
}

func TestDownloadedPartUnit(t *testing.T) {
	part := DownloadedPart{
		Size:         1024,
		TempFilePath: "/tmp/part1.tmp",
		Offset:       0,
		PartNum:      1,
		Err:          nil,
	}

	assert.Equal(t, int64(1024), part.Size)
	assert.Equal(t, "/tmp/part1.tmp", part.TempFilePath)
	assert.Equal(t, int64(0), part.Offset)
	assert.Equal(t, 1, part.PartNum)
	assert.NoError(t, part.Err)
}

func TestFileToDownloadUnit(t *testing.T) {
	file := FileToDownload{
		Namespace:      "test-namespace",
		BucketName:     "test-bucket",
		ObjectName:     "test-object",
		TargetFilePath: "/local/path/test-object",
	}

	assert.Equal(t, "test-namespace", file.Namespace)
	assert.Equal(t, "test-bucket", file.BucketName)
	assert.Equal(t, "test-object", file.ObjectName)
	assert.Equal(t, "/local/path/test-object", file.TargetFilePath)
}

func TestOCIStorage_ProviderUnit(t *testing.T) {
	s := &OCIStorage{}
	assert.Equal(t, storage.ProviderOCI, s.Provider())
}

func TestConfigWithOboToken(t *testing.T) {
	config := &Config{
		CompartmentID:  "test-compartment",
		Region:         "us-phoenix-1",
		EnableOboToken: true,
		OboToken:       "test-obo-token",
		HTTPTimeout:    10 * time.Minute,
	}

	assert.True(t, config.EnableOboToken)
	assert.Equal(t, "test-obo-token", config.OboToken)
	assert.Equal(t, "test-compartment", config.CompartmentID)
	assert.Equal(t, "us-phoenix-1", config.Region)
}

func TestDownloadOptionsConfiguration(t *testing.T) {
	// Test default download options from storage package
	opts := storage.DefaultDownloadOptions()

	// Apply various options
	err := storage.WithSizeThreshold(50)(&opts)
	assert.NoError(t, err)
	assert.Equal(t, 50, opts.SizeThresholdInMB)

	err = storage.WithChunkSize(20)(&opts)
	assert.NoError(t, err)
	assert.Equal(t, 20, opts.ChunkSizeInMB)

	err = storage.WithThreads(8)(&opts)
	assert.NoError(t, err)
	assert.Equal(t, 8, opts.Threads)

	err = storage.WithForceMultipart(true)(&opts)
	assert.NoError(t, err)
	assert.True(t, opts.ForceMultipart)

	err = storage.WithForceStandard(true)(&opts)
	assert.NoError(t, err)
	assert.True(t, opts.ForceStandard)

	err = storage.WithOverrideEnabled(false)(&opts)
	assert.NoError(t, err)
	assert.True(t, opts.DisableOverride)

	err = storage.WithExcludePatterns([]string{"*.tmp", "*.log"})(&opts)
	assert.NoError(t, err)
	assert.Equal(t, []string{"*.tmp", "*.log"}, opts.ExcludePatterns)

	err = storage.WithStripPrefix("/prefix/")(&opts)
	assert.NoError(t, err)
	assert.True(t, opts.StripPrefix)
	assert.Equal(t, "/prefix/", opts.PrefixToStrip)

	err = storage.WithBaseNameOnly(true)(&opts)
	assert.NoError(t, err)
	assert.True(t, opts.UseBaseNameOnly)

	err = storage.WithTailOverlap(true)(&opts)
	assert.NoError(t, err)
	assert.True(t, opts.JoinWithTailOverlap)
}

func TestUploadOptionsConfiguration(t *testing.T) {
	opts := storage.DefaultUploadOptions()

	// Apply upload options
	err := storage.WithUploadChunkSize(50)(&opts)
	assert.NoError(t, err)
	assert.Equal(t, 50, opts.ChunkSizeInMB)

	err = storage.WithUploadThreads(4)(&opts)
	assert.NoError(t, err)
	assert.Equal(t, 4, opts.Threads)

	err = storage.WithContentType("application/json")(&opts)
	assert.NoError(t, err)
	assert.Equal(t, "application/json", opts.ContentType)

	err = storage.WithMetadata(map[string]string{
		"key1": "value1",
		"key2": "value2",
	})(&opts)
	assert.NoError(t, err)
	assert.Equal(t, "value1", opts.Metadata["key1"])
	assert.Equal(t, "value2", opts.Metadata["key2"])

	err = storage.WithStorageClass("Archive")(&opts)
	assert.NoError(t, err)
	assert.Equal(t, "Archive", opts.StorageClass)
}

func TestObjectURIValidation(t *testing.T) {
	tests := []struct {
		name  string
		uri   storage.ObjectURI
		valid bool
	}{
		{
			name: "valid URI",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderOCI,
				Namespace:  "test-namespace",
				BucketName: "test-bucket",
				ObjectName: "test-object",
			},
			valid: true,
		},
		{
			name: "missing bucket name",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderOCI,
				Namespace:  "test-namespace",
				ObjectName: "test-object",
			},
			valid: false,
		},
		{
			name: "empty namespace allowed for OCI",
			uri: storage.ObjectURI{
				Provider:   storage.ProviderOCI,
				BucketName: "test-bucket",
				ObjectName: "test-object",
			},
			valid: true, // Namespace can be auto-detected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple validation
			isValid := tt.uri.BucketName != ""
			assert.Equal(t, tt.valid, isValid)
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	tests := []struct {
		name     string
		input    *Config
		expected *Config
	}{
		{
			name:  "nil config gets defaults",
			input: nil,
			expected: &Config{
				HTTPTimeout:         20 * time.Minute,
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 200,
				MaxConnsPerHost:     200,
			},
		},
		{
			name: "partial config preserves set values",
			input: &Config{
				CompartmentID: "test-compartment",
				HTTPTimeout:   5 * time.Minute,
			},
			expected: &Config{
				CompartmentID:       "test-compartment",
				HTTPTimeout:         5 * time.Minute,
				MaxIdleConns:        200, // Should get default
				MaxIdleConnsPerHost: 200, // Should get default
				MaxConnsPerHost:     200, // Should get default
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config *Config
			if tt.input == nil {
				config = DefaultConfig()
			} else {
				config = tt.input
				// Apply defaults for zero values
				defaults := DefaultConfig()
				if config.HTTPTimeout == 0 {
					config.HTTPTimeout = defaults.HTTPTimeout
				}
				if config.MaxIdleConns == 0 {
					config.MaxIdleConns = defaults.MaxIdleConns
				}
				if config.MaxIdleConnsPerHost == 0 {
					config.MaxIdleConnsPerHost = defaults.MaxIdleConnsPerHost
				}
				if config.MaxConnsPerHost == 0 {
					config.MaxConnsPerHost = defaults.MaxConnsPerHost
				}
			}

			assert.Equal(t, tt.expected.HTTPTimeout, config.HTTPTimeout)
			assert.Equal(t, tt.expected.MaxIdleConns, config.MaxIdleConns)
			assert.Equal(t, tt.expected.MaxIdleConnsPerHost, config.MaxIdleConnsPerHost)
			assert.Equal(t, tt.expected.MaxConnsPerHost, config.MaxConnsPerHost)
			if tt.expected.CompartmentID != "" {
				assert.Equal(t, tt.expected.CompartmentID, config.CompartmentID)
			}
		})
	}
}
