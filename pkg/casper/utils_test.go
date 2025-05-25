package casper

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPureObjectName(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{
			name:           "Simple path with filename",
			input:          "bucket/folder/file.txt",
			expectedOutput: "file.txt",
		},
		{
			name:           "Path with multiple directories",
			input:          "namespace/bucket/folder/subfolder/deep/file.txt",
			expectedOutput: "file.txt",
		},
		{
			name:           "Path with no directory separator",
			input:          "filename.txt",
			expectedOutput: "filename.txt",
		},
		{
			name:           "Path with empty filename after separator",
			input:          "bucket/folder/",
			expectedOutput: "",
		},
		{
			name:           "Empty string",
			input:          "",
			expectedOutput: "",
		},
		{
			name:           "Just a separator",
			input:          "/",
			expectedOutput: "",
		},
		{
			name:           "Multiple consecutive separators",
			input:          "bucket///file.txt",
			expectedOutput: "file.txt",
		},
		{
			name:           "Filename with periods",
			input:          "bucket/file.with.multiple.periods.txt",
			expectedOutput: "file.with.multiple.periods.txt",
		},
		{
			name:           "Filename with spaces",
			input:          "bucket/file with spaces.txt",
			expectedOutput: "file with spaces.txt",
		},
		{
			name:           "Filename with special characters",
			input:          "bucket/file-name_with-special@chars.txt",
			expectedOutput: "file-name_with-special@chars.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ObjectBaseName(tt.input)
			assert.Equal(t, tt.expectedOutput, result)
		})
	}
}

func TestExtractNonPrefixObjectName(t *testing.T) {
	tests := []struct {
		name           string
		objectPath     string
		prefix         string
		expectedOutput string
	}{
		{
			name:           "Remove simple prefix",
			objectPath:     "bucket/folder/file.txt",
			prefix:         "bucket/",
			expectedOutput: "folder/file.txt",
		},
		{
			name:           "Prefix not at beginning",
			objectPath:     "bucket/folder/file.txt",
			prefix:         "folder/",
			expectedOutput: "bucket/file.txt", // This is what the function actually returns
		},
		{
			name:           "Prefix matches exactly",
			objectPath:     "bucket/folder/file.txt",
			prefix:         "bucket/folder/file.txt",
			expectedOutput: "",
		},
		{
			name:           "Empty prefix",
			objectPath:     "bucket/folder/file.txt",
			prefix:         "",
			expectedOutput: "bucket/folder/file.txt", // Unchanged due to empty prefix
		},
		{
			name:           "Empty object path",
			objectPath:     "",
			prefix:         "bucket/",
			expectedOutput: "", // Empty remains empty
		},
		{
			name:           "No separator in path",
			objectPath:     "filename.txt",
			prefix:         "bucket/",
			expectedOutput: "filename.txt", // Unchanged as per function definition
		},
		{
			name:           "Prefix longer than path",
			objectPath:     "bucket/file.txt",
			prefix:         "bucket/file.txt/extra/",
			expectedOutput: "bucket/file.txt", // Unchanged as prefix doesn't match
		},
		{
			name:           "Multiple occurrences of prefix",
			objectPath:     "bucket/folder/bucket/folder/file.txt",
			prefix:         "bucket/folder/",
			expectedOutput: "bucket/folder/file.txt", // Only first occurrence removed
		},
		{
			name:           "Partial match at start",
			objectPath:     "bucket/folder/file.txt",
			prefix:         "buck",
			expectedOutput: "et/folder/file.txt", // First occurrence of 'buck' removed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimObjectPrefix(tt.objectPath, tt.prefix)
			assert.Equal(t, tt.expectedOutput, result)
		})
	}
}

func TestCopyByFilePath(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "casper-utils-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after test

	// Setup test cases
	tests := []struct {
		name          string
		setupFunc     func() (string, string, error) // Returns source path, target path, and setup error
		fileContent   string
		expectedError bool
		errorContains string
	}{
		{
			name: "Successful file copy",
			setupFunc: func() (string, string, error) {
				sourcePath := filepath.Join(tempDir, "source.txt")
				targetPath := filepath.Join(tempDir, "target.txt")
				return sourcePath, targetPath, nil
			},
			fileContent:   "This is test content for successful copy",
			expectedError: false,
		},
		{
			name: "Source file doesn't exist",
			setupFunc: func() (string, string, error) {
				sourcePath := filepath.Join(tempDir, "nonexistent.txt")
				targetPath := filepath.Join(tempDir, "target_nonexistent.txt")
				return sourcePath, targetPath, nil
			},
			expectedError: true,
			errorContains: "failed to open source file",
		},
		{
			name: "Target directory doesn't exist",
			setupFunc: func() (string, string, error) {
				sourcePath := filepath.Join(tempDir, "source_dir_test.txt")
				// Write content to source
				if err := os.WriteFile(sourcePath, []byte("Test content"), 0644); err != nil {
					return "", "", err
				}
				targetPath := filepath.Join(tempDir, "nonexistent_dir", "target.txt")
				return sourcePath, targetPath, nil
			},
			expectedError: true,
			errorContains: "failed to create target file",
		},
		{
			name: "Copy large file",
			setupFunc: func() (string, string, error) {
				sourcePath := filepath.Join(tempDir, "large_file.bin")
				targetPath := filepath.Join(tempDir, "large_file_copy.bin")
				return sourcePath, targetPath, nil
			},
			fileContent:   generateLargeContent(1024 * 1024), // 1MB of data
			expectedError: false,
		},
		{
			name: "Copy to existing target file (overwrite)",
			setupFunc: func() (string, string, error) {
				sourcePath := filepath.Join(tempDir, "source_overwrite.txt")
				targetPath := filepath.Join(tempDir, "target_overwrite.txt")

				// Create target file with different content
				if err := os.WriteFile(targetPath, []byte("Original target content"), 0644); err != nil {
					return "", "", err
				}

				return sourcePath, targetPath, nil
			},
			fileContent:   "This will overwrite the target file",
			expectedError: false,
		},
		{
			name: "Copy with read-only target directory",
			setupFunc: func() (string, string, error) {
				if os.Geteuid() == 0 {
					// Skip this test as root (e.g., in CI or Docker), permissions can't be enforced
					return "", "", nil
				}
				dir := filepath.Join(tempDir, "readonly_dir")
				err := os.Mkdir(dir, 0500)
				if err != nil {
					return "", "", err
				}
				sourcePath := filepath.Join(tempDir, "readonly_source.txt")
				if err := os.WriteFile(sourcePath, []byte("test content"), 0644); err != nil {
					return "", "", err
				}
				targetPath := filepath.Join(dir, "should_fail.txt")
				return sourcePath, targetPath, nil
			},
			fileContent:   "test content",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test files
			sourcePath, targetPath, err := tt.setupFunc()
			if err != nil {
				t.Fatalf("Failed to setup test: %v", err)
			}

			// Write content to source file if needed
			if tt.fileContent != "" && !tt.expectedError {
				err = os.WriteFile(sourcePath, []byte(tt.fileContent), 0644)
				require.NoError(t, err, "Failed to create source file")
			}

			// Call the function
			err = CopyByFilePath(sourcePath, targetPath)

			// Check results
			if tt.expectedError {
				if os.Geteuid() == 0 && tt.name == "Copy with read-only target directory" {
					t.Skip("Skipping read-only directory test as root (CI/Docker), permissions cannot be enforced.")
				}
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)

				// Verify file content
				sourceContent, err := os.ReadFile(sourcePath)
				require.NoError(t, err)

				targetContent, err := os.ReadFile(targetPath)
				require.NoError(t, err)

				assert.Equal(t, sourceContent, targetContent, "File content should match")
			}
		})
	}
}

// Helper function to generate large content for testing
func generateLargeContent(size int) string {
	// Use a simple pattern to avoid excessive memory usage
	pattern := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	patternLen := len(pattern)

	result := make([]byte, size)
	for i := 0; i < size; i++ {
		result[i] = pattern[i%patternLen]
	}

	return string(result)
}

// TestCopyByFilePathConcurrent tests concurrent copying of files
func TestCopyByFilePathConcurrent(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "casper-utils-concurrent-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up after test

	// Number of concurrent copies to perform
	const concurrentCopies = 10

	// Create source files with unique content
	sourceFiles := make([]string, concurrentCopies)
	targetFiles := make([]string, concurrentCopies)
	fileContents := make([][]byte, concurrentCopies)

	for i := 0; i < concurrentCopies; i++ {
		sourceFiles[i] = filepath.Join(tempDir, fmt.Sprintf("source_%d.txt", i))
		targetFiles[i] = filepath.Join(tempDir, fmt.Sprintf("target_%d.txt", i))
		fileContents[i] = []byte(fmt.Sprintf("Concurrent test content for file %d", i))

		err := os.WriteFile(sourceFiles[i], fileContents[i], 0644)
		require.NoError(t, err, "Failed to create source file")
	}

	// Use channels to coordinate the goroutines
	errChan := make(chan error, concurrentCopies)
	doneChan := make(chan bool, concurrentCopies)

	// Launch concurrent copy operations
	for i := 0; i < concurrentCopies; i++ {
		go func(index int) {
			err := CopyByFilePath(sourceFiles[index], targetFiles[index])
			if err != nil {
				errChan <- fmt.Errorf("Copy %d failed: %v", index, err)
				return
			}
			doneChan <- true
		}(i)
	}

	// Collect results
	for i := 0; i < concurrentCopies; i++ {
		select {
		case err := <-errChan:
			t.Errorf("Concurrent copy failed: %v", err)
		case <-doneChan:
			// Success, do nothing
		}
	}

	// Verify all files were copied correctly
	for i := 0; i < concurrentCopies; i++ {
		targetContent, err := os.ReadFile(targetFiles[i])
		assert.NoError(t, err)
		assert.Equal(t, fileContents[i], targetContent, "File content should match for file %d", i)
	}
}

// TestExtractPureObjectNameBenchmark benchmarks the ObjectBaseName function
func BenchmarkExtractPureObjectName(b *testing.B) {
	paths := []string{
		"simple.txt",
		"bucket/folder/file.txt",
		"very/deep/path/with/many/levels/file.txt",
		"bucket////file.txt", // Multiple consecutive separators
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			ObjectBaseName(path)
		}
	}
}

// TestExtractNonPrefixObjectNameBenchmark benchmarks the TrimObjectPrefix function
func BenchmarkExtractNonPrefixObjectName(b *testing.B) {
	testCases := []struct {
		path   string
		prefix string
	}{
		{"bucket/folder/file.txt", "bucket/"},
		{"bucket/folder/file.txt", ""},
		{"bucket/folder/file.txt", "bucket/folder/"},
		{"very/deep/path/with/many/levels/file.txt", "very/deep/"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			TrimObjectPrefix(tc.path, tc.prefix)
		}
	}
}

func TestJoinWithTailOverlap(t *testing.T) {
	tests := []struct {
		name      string
		directory string
		object    string
		expected  string
	}{
		{
			name:      "Exact tail match",
			directory: "/x/y/z/w/",
			object:    "/z/w/a.txt",
			expected:  "/x/y/z/w/a.txt",
		},
		{
			name:      "Partial tail overlap",
			directory: "/1/2/3/4",
			object:    "/2/3/4/5/a.txt",
			expected:  "/1/2/3/4/5/a.txt",
		},
		{
			name:      "No overlap at all",
			directory: "/a/b/c",
			object:    "/x/y/z.txt",
			expected:  "/a/b/c/x/y/z.txt",
		},
		{
			name:      "Directory is root",
			directory: "/",
			object:    "/a/b.txt",
			expected:  "/a/b.txt",
		},
		{
			name:      "Object is root",
			directory: "/a/b/c",
			object:    "/",
			expected:  "/a/b/c",
		},
		{
			name:      "Trailing slashes ignored",
			directory: "/a/b/c/",
			object:    "/b/c/d.txt",
			expected:  "/a/b/c/d.txt",
		},
		{
			name:      "File directly inside directory",
			directory: "/mnt/models",
			object:    "/models/model.bin",
			expected:  "/mnt/models/model.bin",
		},
		{
			name:      "Flat disjoint paths",
			directory: "/a",
			object:    "/b/c.txt",
			expected:  "/a/b/c.txt",
		},
		{
			name:      "No overlap with repeated path elements",
			directory: "/a/b/c/d/e",
			object:    "/a/d/e/1.txt",
			expected:  "/a/b/c/d/e/a/d/e/1.txt",
		},
		{
			name:      "Object path is subset but not a suffix",
			directory: "/a/b/c/d/e",
			object:    "/a/b/c/1.txt",
			expected:  "/a/b/c/d/e/a/b/c/1.txt",
		},
		{
			name:      "Complete overlap",
			directory: "/x/y/z",
			object:    "/x/y/z",
			expected:  "/x/y/z",
		},
		{
			name:      "Single file name only",
			directory: "/data/tmp",
			object:    "log.txt",
			expected:  "/data/tmp/log.txt",
		},
		{
			name:      "Single directory overlap",
			directory: "/a/b",
			object:    "/b/c/d.txt",
			expected:  "/a/b/c/d.txt",
		},
		{
			name:      "Multiple directories",
			directory: "/Users/simolin/mnt/models/intfloat/e5-mistral-7b-instruct",
			object:    "intfloat/e5-mistral-7b-instruct/lora/adapter_model.bin",
			expected:  "/Users/simolin/mnt/models/intfloat/e5-mistral-7b-instruct/lora/adapter_model.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinWithTailOverlap(tt.directory, tt.object)
			if result != tt.expected {
				t.Errorf("Expected %q but got %q", tt.expected, result)
			}
		})
	}
}
