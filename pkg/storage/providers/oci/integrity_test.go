package oci

import (
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsMultipartMD5(t *testing.T) {
	tests := []struct {
		name     string
		md5str   string
		expected bool
	}{
		{
			name:     "standard MD5",
			md5str:   "098f6bcd4621d373cade4e832627b4f6",
			expected: false,
		},
		{
			name:     "base64 MD5",
			md5str:   "CY9rzUYh03PK3k6DJie09g==",
			expected: false,
		},
		{
			name:     "multipart MD5 with 5 parts",
			md5str:   "abc123def456-5",
			expected: true,
		},
		{
			name:     "multipart MD5 with 10 parts",
			md5str:   "xyz789uvw012-10",
			expected: true,
		},
		{
			name:     "invalid multipart format - no dash",
			md5str:   "abc123def456",
			expected: false,
		},
		{
			name:     "invalid multipart format - non-numeric part count",
			md5str:   "abc123def456-abc",
			expected: false,
		},
		{
			name:     "invalid multipart format - multiple dashes",
			md5str:   "abc-123-def-456",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMultipartMD5(tt.md5str)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeMD5(t *testing.T) {
	// Create a temporary file with known content
	content := []byte("Hello, World!")
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	err := os.WriteFile(testFile, content, 0644)
	require.NoError(t, err)

	// Compute MD5 using our function
	md5Result, err := computeMD5(testFile)
	require.NoError(t, err)

	// Compute expected MD5
	hash := md5.New()
	_, err = hash.Write(content)
	require.NoError(t, err)
	expectedMD5 := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	assert.Equal(t, expectedMD5, md5Result)
}

func TestComputeMD5NonExistentFile(t *testing.T) {
	_, err := computeMD5("/non/existent/file.txt")
	assert.Error(t, err)
}

func TestLocalCopyValidation(t *testing.T) {
	// This test would require mocking the OCI client
	// For now, we'll test the helper functions

	t.Run("multipart MD5 detection", func(t *testing.T) {
		assert.True(t, isMultipartMD5("abc123-5"))
		assert.False(t, isMultipartMD5("abc123"))
	})

	t.Run("MD5 computation", func(t *testing.T) {
		// Create test file
		content := []byte("test content")
		tempFile, err := os.CreateTemp("", "test-*.txt")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())

		_, err = tempFile.Write(content)
		require.NoError(t, err)
		tempFile.Close()

		// Compute MD5
		md5Val, err := computeMD5(tempFile.Name())
		require.NoError(t, err)

		// Verify it's base64 encoded
		_, err = base64.StdEncoding.DecodeString(md5Val)
		assert.NoError(t, err)
	})
}

// BenchmarkMD5Computation benchmarks MD5 computation for different file sizes
func BenchmarkMD5Computation(b *testing.B) {
	sizes := []int{
		1024,             // 1KB
		1024 * 100,       // 100KB
		1024 * 1024,      // 1MB
		1024 * 1024 * 10, // 10MB
	}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("size-%dKB", size/1024), func(b *testing.B) {
			// Create test file with random content
			content := make([]byte, size)
			for i := range content {
				content[i] = byte(i % 256)
			}

			tempFile, err := os.CreateTemp("", "bench-*.dat")
			require.NoError(b, err)
			defer os.Remove(tempFile.Name())

			_, err = tempFile.Write(content)
			require.NoError(b, err)
			tempFile.Close()

			b.ResetTimer()
			b.SetBytes(int64(size))

			for i := 0; i < b.N; i++ {
				_, err := computeMD5(tempFile.Name())
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Helper to create a test file with specific content
func createTestFile(t *testing.T, content []byte) string {
	tempFile, err := os.CreateTemp("", "test-*.dat")
	require.NoError(t, err)

	_, err = tempFile.Write(content)
	require.NoError(t, err)

	err = tempFile.Close()
	require.NoError(t, err)

	return tempFile.Name()
}

// TestMD5StreamComparison tests MD5 computation via streaming vs full read
func TestMD5StreamComparison(t *testing.T) {
	content := []byte("This is a test file for MD5 comparison")
	testFile := createTestFile(t, content)
	defer os.Remove(testFile)

	// Compute using our function
	md5Result, err := computeMD5(testFile)
	require.NoError(t, err)

	// Compute using direct streaming
	file, err := os.Open(testFile)
	require.NoError(t, err)
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	require.NoError(t, err)

	streamMD5 := base64.StdEncoding.EncodeToString(hash.Sum(nil))

	assert.Equal(t, streamMD5, md5Result, "MD5 values should match")
}
