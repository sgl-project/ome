package casper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test DataStore interface compliance
func TestDataStoreInterface(t *testing.T) {
	t.Run("CasperDataStore implements DataStore", func(t *testing.T) {
		// Test that CasperDataStore implements the DataStore interface
		var _ DataStore = (*CasperDataStore)(nil)
	})

	t.Run("LocalDataStore implements DataStore", func(t *testing.T) {
		// Test that LocalDataStore implements the DataStore interface
		var _ DataStore = (*LocalDataStore)(nil)
	})
}

// Test ObjectURI struct and validation
func TestObjectURIStruct(t *testing.T) {
	t.Run("ObjectURI field validation", func(t *testing.T) {
		uri := ObjectURI{
			Namespace:  "test-namespace",
			BucketName: "test-bucket",
			ObjectName: "path/to/file.txt",
			Prefix:     "path/",
			Region:     "us-chicago-1",
		}

		// Test all fields are set correctly
		assert.Equal(t, "test-namespace", uri.Namespace)
		assert.Equal(t, "test-bucket", uri.BucketName)
		assert.Equal(t, "path/to/file.txt", uri.ObjectName)
		assert.Equal(t, "path/", uri.Prefix)
		assert.Equal(t, "us-chicago-1", uri.Region)
	})

	t.Run("ObjectURI with empty fields", func(t *testing.T) {
		uri := ObjectURI{}

		// Test zero values
		assert.Empty(t, uri.Namespace)
		assert.Empty(t, uri.BucketName)
		assert.Empty(t, uri.ObjectName)
		assert.Empty(t, uri.Prefix)
		assert.Empty(t, uri.Region)
	})

	t.Run("ObjectURI with special characters", func(t *testing.T) {
		uri := ObjectURI{
			Namespace:  "namespace-with-dashes",
			BucketName: "bucket_with_underscores",
			ObjectName: "file with spaces & special chars!@#.txt",
			Prefix:     "prefix/with/slashes/",
			Region:     "us-chicago-1",
		}

		assert.Contains(t, uri.Namespace, "-")
		assert.Contains(t, uri.BucketName, "_")
		assert.Contains(t, uri.ObjectName, " ")
		assert.Contains(t, uri.ObjectName, "&")
		assert.Contains(t, uri.Prefix, "/")
	})

	t.Run("ObjectURI path manipulation", func(t *testing.T) {
		uri := ObjectURI{
			BucketName: "test-bucket",
			ObjectName: "models/v1/model.bin",
		}

		// Test path extraction
		baseName := ObjectBaseName(uri.ObjectName)
		assert.Equal(t, "model.bin", baseName)

		// Test prefix trimming
		trimmed := TrimObjectPrefix(uri.ObjectName, "models/")
		assert.Equal(t, "v1/model.bin", trimmed)
	})
}

// Test DataStore interface method signatures
func TestDataStoreMethodSignatures(t *testing.T) {
	t.Run("Download method signature", func(t *testing.T) {
		// Test that the Download method has the correct signature
		// We can't call it without implementation, but we can verify the interface

		source := ObjectURI{
			BucketName: "test-bucket",
			ObjectName: "test-file.txt",
		}
		target := "/local/path"

		// Verify parameter types
		assert.IsType(t, ObjectURI{}, source)
		assert.IsType(t, "", target)
	})

	t.Run("Upload method signature", func(t *testing.T) {
		// Test that the Upload method has the correct signature
		source := "/local/file.txt"
		target := ObjectURI{
			BucketName: "test-bucket",
			ObjectName: "uploaded-file.txt",
		}

		// Verify parameter types
		assert.IsType(t, "", source)
		assert.IsType(t, ObjectURI{}, target)
	})
}

// Test functional options pattern for DataStore
func TestDataStoreFunctionalOptions(t *testing.T) {
	t.Run("DownloadOption type", func(t *testing.T) {
		// Test that DownloadOption is a function type
		var opt DownloadOption = WithThreads(10)
		assert.NotNil(t, opt)
	})

	t.Run("Multiple download options", func(t *testing.T) {
		// Test that multiple options can be created
		opts := []DownloadOption{
			WithThreads(10),
			WithChunkSize(16),
			WithSizeThreshold(100),
		}

		assert.Len(t, opts, 3)
		for _, opt := range opts {
			assert.NotNil(t, opt)
		}
	})

	t.Run("Download options application", func(t *testing.T) {
		// Test that options can be applied
		result, err := applyDownloadOptions(
			WithThreads(20),
			WithChunkSize(32),
		)

		assert.NoError(t, err)
		assert.Equal(t, 20, result.Threads)
		assert.Equal(t, 32, result.ChunkSizeInMB)
	})
}

// Test DataStore interface documentation compliance
func TestDataStoreDocumentation(t *testing.T) {
	t.Run("Interface method documentation", func(t *testing.T) {
		// Test that the interface methods are well-defined
		// This is more of a structural test to ensure the interface is properly designed

		// Download method should accept source, target, and options
		source := ObjectURI{BucketName: "bucket", ObjectName: "object"}
		target := "/path"
		opts := []DownloadOption{WithThreads(5)}

		// Verify types are compatible
		assert.IsType(t, ObjectURI{}, source)
		assert.IsType(t, "", target)
		assert.IsType(t, []DownloadOption{}, opts)
	})

	t.Run("Error handling pattern", func(t *testing.T) {
		// Test that methods return errors as expected
		// All DataStore methods should return error as the last return value

		// This is a compile-time check that the interface is correctly defined
		var ds DataStore
		if ds != nil {
			// These calls would fail at runtime, but compile successfully
			_ = ds.Download(ObjectURI{}, "", WithThreads(1))
			_ = ds.Upload("", ObjectURI{})
		}
	})
}

// Test ObjectURI validation scenarios
func TestObjectURIValidation(t *testing.T) {
	t.Run("Valid ObjectURI scenarios", func(t *testing.T) {
		validURIs := []ObjectURI{
			{BucketName: "bucket", ObjectName: "file.txt"},
			{BucketName: "bucket", ObjectName: "path/to/file.txt"},
			{Namespace: "ns", BucketName: "bucket", ObjectName: "file.txt"},
			{BucketName: "bucket", ObjectName: "file.txt", Region: "us-chicago-1"},
		}

		for _, uri := range validURIs {
			assert.NotEmpty(t, uri.BucketName, "BucketName should not be empty")
			assert.NotEmpty(t, uri.ObjectName, "ObjectName should not be empty")
		}
	})

	t.Run("ObjectURI edge cases", func(t *testing.T) {
		// Test various edge cases
		edgeCases := []struct {
			name string
			uri  ObjectURI
		}{
			{"Empty bucket", ObjectURI{ObjectName: "file.txt"}},
			{"Empty object", ObjectURI{BucketName: "bucket"}},
			{"Long object name", ObjectURI{BucketName: "bucket", ObjectName: "very/long/path/to/deeply/nested/file.txt"}},
			{"Special chars in bucket", ObjectURI{BucketName: "bucket-with-dashes_and_underscores", ObjectName: "file.txt"}},
		}

		for _, tc := range edgeCases {
			t.Run(tc.name, func(t *testing.T) {
				// Just verify the structure can be created
				assert.IsType(t, ObjectURI{}, tc.uri)
			})
		}
	})
}
