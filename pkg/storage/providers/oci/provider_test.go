package oci

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sgl-project/ome/pkg/storage"
)

func TestParseOCIURI(t *testing.T) {
	tests := []struct {
		name             string
		uri              string
		defaultNamespace string
		defaultBucket    string
		expected         *ociURI
		expectError      bool
	}{
		{
			name:             "full oci scheme",
			uri:              "oci://namespace/bucket/path/to/object.txt",
			defaultNamespace: "default-ns",
			defaultBucket:    "default-bucket",
			expected: &ociURI{
				Namespace: "namespace",
				Bucket:    "bucket",
				Object:    "path/to/object.txt",
			},
			expectError: false,
		},
		{
			name:             "oci scheme without namespace",
			uri:              "oci://bucket/path/to/object.txt",
			defaultNamespace: "default-ns",
			defaultBucket:    "default-bucket",
			expected: &ociURI{
				Namespace: "default-ns",
				Bucket:    "bucket",
				Object:    "path/to/object.txt",
			},
			expectError: false,
		},
		{
			name:             "oci scheme object only",
			uri:              "oci://object.txt",
			defaultNamespace: "default-ns",
			defaultBucket:    "default-bucket",
			expected: &ociURI{
				Namespace: "default-ns",
				Bucket:    "default-bucket",
				Object:    "object.txt",
			},
			expectError: false,
		},
		{
			name:             "https URL format",
			uri:              "https://objectstorage.us-ashburn-1.oraclecloud.com/n/namespace/b/bucket/o/path/to/object.txt",
			defaultNamespace: "default-ns",
			defaultBucket:    "default-bucket",
			expected: &ociURI{
				Namespace: "namespace",
				Bucket:    "bucket",
				Object:    "path/to/object.txt",
			},
			expectError: false,
		},
		{
			name:             "relative path with bucket",
			uri:              "/bucket/path/to/object.txt",
			defaultNamespace: "default-ns",
			defaultBucket:    "default-bucket",
			expected: &ociURI{
				Namespace: "default-ns",
				Bucket:    "bucket",
				Object:    "path/to/object.txt",
			},
			expectError: false,
		},
		{
			name:             "relative path object only",
			uri:              "/object.txt",
			defaultNamespace: "default-ns",
			defaultBucket:    "default-bucket",
			expected: &ociURI{
				Namespace: "default-ns",
				Bucket:    "default-bucket",
				Object:    "object.txt",
			},
			expectError: false,
		},
		{
			name:             "bucket/object format",
			uri:              "bucket/object.txt",
			defaultNamespace: "default-ns",
			defaultBucket:    "default-bucket",
			expected: &ociURI{
				Namespace: "default-ns",
				Bucket:    "bucket",
				Object:    "object.txt",
			},
			expectError: false,
		},
		{
			name:             "object only with default bucket",
			uri:              "object.txt",
			defaultNamespace: "default-ns",
			defaultBucket:    "default-bucket",
			expected: &ociURI{
				Namespace: "default-ns",
				Bucket:    "default-bucket",
				Object:    "object.txt",
			},
			expectError: false,
		},
		{
			name:             "missing namespace error",
			uri:              "oci://bucket/object.txt",
			defaultNamespace: "",
			defaultBucket:    "default-bucket",
			expected:         nil,
			expectError:      true,
		},
		{
			name:             "missing bucket error",
			uri:              "object.txt",
			defaultNamespace: "default-ns",
			defaultBucket:    "",
			expected:         nil,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseOCIURI(tt.uri, tt.defaultNamespace, tt.defaultBucket)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.Namespace, result.Namespace)
				assert.Equal(t, tt.expected.Bucket, result.Bucket)
				assert.Equal(t, tt.expected.Object, result.Object)
			}
		})
	}
}

func TestConvertMetadata(t *testing.T) {
	t.Run("convert to OCI", func(t *testing.T) {
		input := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		result := convertMetadataToOCI(input)
		assert.Equal(t, input, result)

		// Test nil input
		assert.Nil(t, convertMetadataToOCI(nil))
	})

	t.Run("convert from OCI", func(t *testing.T) {
		input := map[string]string{
			"opc-meta-key1": "value1",
			"key2":          "value2",
		}

		expected := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		result := convertMetadataFromOCI(input)
		assert.Equal(t, expected, result)

		// Test nil input
		assert.Nil(t, convertMetadataFromOCI(nil))
	})
}

func TestCalculateChunks(t *testing.T) {
	tests := []struct {
		name         string
		totalSize    int64
		chunkSize    int64
		expectedNum  int
		expectedLast int64
	}{
		{
			name:         "even division",
			totalSize:    100 * 1024 * 1024,
			chunkSize:    10 * 1024 * 1024,
			expectedNum:  10,
			expectedLast: 10 * 1024 * 1024,
		},
		{
			name:         "uneven division",
			totalSize:    105 * 1024 * 1024,
			chunkSize:    10 * 1024 * 1024,
			expectedNum:  11,
			expectedLast: 5 * 1024 * 1024,
		},
		{
			name:         "single chunk",
			totalSize:    5 * 1024 * 1024,
			chunkSize:    10 * 1024 * 1024,
			expectedNum:  1,
			expectedLast: 5 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := calculateChunks(tt.totalSize, tt.chunkSize)

			assert.Len(t, chunks, tt.expectedNum)
			if len(chunks) > 0 {
				lastChunk := chunks[len(chunks)-1]
				assert.Equal(t, tt.expectedLast, lastChunk.size)

				// Verify chunks cover entire file
				var totalCovered int64
				for _, chunk := range chunks {
					totalCovered += chunk.size
				}
				assert.Equal(t, tt.totalSize, totalCovered)
			}
		})
	}
}

func TestCalculateOptimalPartSize(t *testing.T) {
	tests := []struct {
		name        string
		fileSize    int64
		minExpected int64
		maxExpected int64
	}{
		{
			name:        "small file",
			fileSize:    10 * 1024 * 1024, // 10MB
			minExpected: minPartSize,
			maxExpected: defaultPartSizeMB * 1024 * 1024,
		},
		{
			name:        "medium file",
			fileSize:    1024 * 1024 * 1024, // 1GB
			minExpected: minPartSize,
			maxExpected: defaultPartSizeMB * 1024 * 1024,
		},
		{
			name:        "large file requiring bigger parts",
			fileSize:    100 * 1024 * 1024 * 1024, // 100GB
			minExpected: defaultPartSizeMB * 1024 * 1024,
			maxExpected: 200 * 1024 * 1024, // Should be larger than default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partSize := calculateOptimalPartSize(tt.fileSize)

			assert.GreaterOrEqual(t, partSize, tt.minExpected)
			assert.LessOrEqual(t, partSize, tt.maxExpected)

			// Ensure we don't exceed max parts
			numParts := (tt.fileSize + partSize - 1) / partSize
			assert.LessOrEqual(t, numParts, int64(maxParts))
		})
	}
}

func TestShouldUseParallelDownload(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		options  storage.DownloadOptions
		expected bool
	}{
		{
			name: "large file with concurrency",
			size: 100 * 1024 * 1024, // 100MB
			options: storage.DownloadOptions{
				Concurrency: 5,
			},
			expected: true,
		},
		{
			name: "small file",
			size: 10 * 1024 * 1024, // 10MB
			options: storage.DownloadOptions{
				Concurrency: 5,
			},
			expected: false,
		},
		{
			name: "large file but no concurrency",
			size: 100 * 1024 * 1024, // 100MB
			options: storage.DownloadOptions{
				Concurrency: 0,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldUseParallelDownload(tt.size, tt.options)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldUseMultipartUpload(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		options  storage.UploadOptions
		expected bool
	}{
		{
			name: "large file with part size",
			size: 200 * 1024 * 1024, // 200MB
			options: storage.UploadOptions{
				PartSize: 128 * 1024 * 1024,
			},
			expected: true,
		},
		{
			name: "small file",
			size: 50 * 1024 * 1024, // 50MB
			options: storage.UploadOptions{
				PartSize: 128 * 1024 * 1024,
			},
			expected: false,
		},
		{
			name: "large file but no part size",
			size: 200 * 1024 * 1024, // 200MB
			options: storage.UploadOptions{
				PartSize: 0,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldUseMultipartUpload(tt.size, tt.options)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// MockProgressReporter for testing
type mockProgressReporter struct {
	updates []progressUpdate
	done    bool
	err     error
}

type progressUpdate struct {
	current int64
	total   int64
}

func (m *mockProgressReporter) Update(current, total int64) {
	m.updates = append(m.updates, progressUpdate{current: current, total: total})
}

func (m *mockProgressReporter) Done() {
	m.done = true
}

func (m *mockProgressReporter) Error(err error) {
	m.err = err
}

func TestProgressReader(t *testing.T) {
	data := "test data for progress reader"
	reader := strings.NewReader(data)
	reporter := &mockProgressReporter{}

	pr := &progressReader{
		reader:   io.NopCloser(reader),
		size:     int64(len(data)),
		progress: reporter,
	}

	// Read all data
	buf := make([]byte, len(data))
	n, err := io.ReadFull(pr, buf)

	assert.NoError(t, err)
	assert.Equal(t, len(data), n)
	assert.Equal(t, data, string(buf))

	// Check progress was reported
	assert.Greater(t, len(reporter.updates), 0)
	lastUpdate := reporter.updates[len(reporter.updates)-1]
	assert.Equal(t, int64(len(data)), lastUpdate.current)
	assert.Equal(t, int64(len(data)), lastUpdate.total)

	// Test Close method
	err = pr.Close()
	assert.NoError(t, err)
}
