package ociobjectstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalDataStore_createWorkingDirectory(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func() (*LocalDataStore, func(), error)
		expectedError bool
		errorContains string
		validateFunc  func(*testing.T, *LocalDataStore)
	}{
		{
			name: "Create simple directory",
			setupFunc: func() (*LocalDataStore, func(), error) {
				tempDir, err := os.MkdirTemp("", "lds-create-test")
				if err != nil {
					return nil, nil, err
				}

				cleanupFunc := func() {
					os.RemoveAll(tempDir)
				}

				workingDir := filepath.Join(tempDir, "working_dir")
				lds := &LocalDataStore{
					WorkingDirectory: workingDir,
				}

				return lds, cleanupFunc, nil
			},
			expectedError: false,
			validateFunc: func(t *testing.T, lds *LocalDataStore) {
				// Check if directory was created
				dirInfo, err := os.Stat(lds.WorkingDirectory)
				assert.NoError(t, err)
				assert.True(t, dirInfo.IsDir())
			},
		},
		{
			name: "Create nested directory structure",
			setupFunc: func() (*LocalDataStore, func(), error) {
				tempDir, err := os.MkdirTemp("", "lds-create-nested-test")
				if err != nil {
					return nil, nil, err
				}

				cleanupFunc := func() {
					os.RemoveAll(tempDir)
				}

				workingDir := filepath.Join(tempDir, "level1", "level2", "level3")
				lds := &LocalDataStore{
					WorkingDirectory: workingDir,
				}

				return lds, cleanupFunc, nil
			},
			expectedError: false,
			validateFunc: func(t *testing.T, lds *LocalDataStore) {
				// Check if directory was created with all parents
				dirInfo, err := os.Stat(lds.WorkingDirectory)
				assert.NoError(t, err)
				assert.True(t, dirInfo.IsDir())
			},
		},
		{
			name: "Create directory with special characters",
			setupFunc: func() (*LocalDataStore, func(), error) {
				tempDir, err := os.MkdirTemp("", "lds-special-chars-test")
				if err != nil {
					return nil, nil, err
				}

				cleanupFunc := func() {
					os.RemoveAll(tempDir)
				}

				workingDir := filepath.Join(tempDir, "dir with spaces", "special-chars_dir")
				lds := &LocalDataStore{
					WorkingDirectory: workingDir,
				}

				return lds, cleanupFunc, nil
			},
			expectedError: false,
			validateFunc: func(t *testing.T, lds *LocalDataStore) {
				// Check if directory with special chars was created
				dirInfo, err := os.Stat(lds.WorkingDirectory)
				assert.NoError(t, err)
				assert.True(t, dirInfo.IsDir())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lds, cleanup, err := tt.setupFunc()
			require.NoError(t, err)
			defer cleanup()

			// Call the function under test
			err = lds.createWorkingDirectory()

			// Check results
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, lds)
				}
			}
		})
	}
}

func TestLocalDataStore_Download(t *testing.T) {
	tests := []struct {
		name          string
		setupFunc     func() (*LocalDataStore, ObjectURI, string, func(), error)
		expectedError bool
		errorContains string
		validateFunc  func(*testing.T, string, string) // Target dir, content
	}{
		{
			name: "Download file successfully",
			setupFunc: func() (*LocalDataStore, ObjectURI, string, func(), error) {
				// Create temp directories for source and target
				tempRoot, err := os.MkdirTemp("", "lds-download-test")
				if err != nil {
					return nil, ObjectURI{}, "", nil, err
				}

				sourceDir := filepath.Join(tempRoot, "source")
				targetDir := filepath.Join(tempRoot, "target")

				// Create source directory
				err = os.MkdirAll(sourceDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, ObjectURI{}, "", nil, err
				}

				// Create test file in source
				testFile := "test_file.txt"
				testContent := "Test content for download"
				testFilePath := filepath.Join(sourceDir, testFile)
				err = os.WriteFile(testFilePath, []byte(testContent), 0644)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, ObjectURI{}, "", nil, err
				}

				// Create LocalDataStore
				lds := &LocalDataStore{
					WorkingDirectory: sourceDir,
				}

				// Create ObjectURI
				objURI := ObjectURI{
					ObjectName: testFile,
				}

				cleanupFunc := func() {
					os.RemoveAll(tempRoot)
				}

				return lds, objURI, targetDir, cleanupFunc, nil
			},
			expectedError: false,
			validateFunc: func(t *testing.T, targetDir, expectedContent string) {
				// Check if file was downloaded to target
				targetFilePath := filepath.Join(targetDir, "test_file.txt")
				assert.FileExists(t, targetFilePath)

				// Verify content
				content, err := os.ReadFile(targetFilePath)
				assert.NoError(t, err)
				assert.Equal(t, expectedContent, string(content))
			},
		},
		{
			name: "Source file doesn't exist",
			setupFunc: func() (*LocalDataStore, ObjectURI, string, func(), error) {
				// Create temp directories for source and target
				tempRoot, err := os.MkdirTemp("", "lds-download-missing-test")
				if err != nil {
					return nil, ObjectURI{}, "", nil, err
				}

				sourceDir := filepath.Join(tempRoot, "source")
				targetDir := filepath.Join(tempRoot, "target")

				// Create source directory
				err = os.MkdirAll(sourceDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, ObjectURI{}, "", nil, err
				}

				// Create LocalDataStore
				lds := &LocalDataStore{
					WorkingDirectory: sourceDir,
				}

				// Create ObjectURI for non-existent file
				objURI := ObjectURI{
					ObjectName: "non_existent_file.txt",
				}

				cleanupFunc := func() {
					os.RemoveAll(tempRoot)
				}

				return lds, objURI, targetDir, cleanupFunc, nil
			},
			expectedError: true,
			errorContains: "failed to open source file",
		},
		{
			name: "Download to target directory that doesn't exist yet",
			setupFunc: func() (*LocalDataStore, ObjectURI, string, func(), error) {
				// Create temp directories for source
				tempRoot, err := os.MkdirTemp("", "lds-download-no-target-test")
				if err != nil {
					return nil, ObjectURI{}, "", nil, err
				}

				sourceDir := filepath.Join(tempRoot, "source")
				targetDir := filepath.Join(tempRoot, "nonexistent_target")

				// Create source directory
				err = os.MkdirAll(sourceDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, ObjectURI{}, "", nil, err
				}

				// Create test file in source
				testFile := "test_file2.txt"
				testContent := "Test content for non-existent target"
				testFilePath := filepath.Join(sourceDir, testFile)
				err = os.WriteFile(testFilePath, []byte(testContent), 0644)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, ObjectURI{}, "", nil, err
				}

				// Create LocalDataStore
				lds := &LocalDataStore{
					WorkingDirectory: sourceDir,
				}

				// Create ObjectURI
				objURI := ObjectURI{
					ObjectName: testFile,
				}

				cleanupFunc := func() {
					os.RemoveAll(tempRoot)
				}

				return lds, objURI, targetDir, cleanupFunc, nil
			},
			expectedError: false, // Should create the target directory automatically
			validateFunc: func(t *testing.T, targetDir, expectedContent string) {
				// Check if target directory was created
				targetDirInfo, err := os.Stat(targetDir)
				assert.NoError(t, err)
				assert.True(t, targetDirInfo.IsDir())

				// Check if file was downloaded to target
				targetFilePath := filepath.Join(targetDir, "test_file2.txt")
				assert.FileExists(t, targetFilePath)

				// Verify content
				content, err := os.ReadFile(targetFilePath)
				assert.NoError(t, err)
				assert.Equal(t, expectedContent, string(content))
			},
		},
		{
			name: "Download large file",
			setupFunc: func() (*LocalDataStore, ObjectURI, string, func(), error) {
				// Create temp directories for source and target
				tempRoot, err := os.MkdirTemp("", "lds-download-large-test")
				if err != nil {
					return nil, ObjectURI{}, "", nil, err
				}

				sourceDir := filepath.Join(tempRoot, "source")
				targetDir := filepath.Join(tempRoot, "target")

				// Create directories
				err = os.MkdirAll(sourceDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, ObjectURI{}, "", nil, err
				}
				err = os.MkdirAll(targetDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, ObjectURI{}, "", nil, err
				}

				// Create large test file in source (512KB)
				testFile := "large_file.bin"
				testContent := generateLargeContent(512 * 1024)
				testFilePath := filepath.Join(sourceDir, testFile)
				err = os.WriteFile(testFilePath, []byte(testContent), 0644)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, ObjectURI{}, "", nil, err
				}

				// Create LocalDataStore
				lds := &LocalDataStore{
					WorkingDirectory: sourceDir,
				}

				// Create ObjectURI
				objURI := ObjectURI{
					ObjectName: testFile,
				}

				cleanupFunc := func() {
					os.RemoveAll(tempRoot)
				}

				return lds, objURI, targetDir, cleanupFunc, nil
			},
			expectedError: false,
			validateFunc: func(t *testing.T, targetDir, expectedContent string) {
				// Check if file was downloaded to target
				targetFilePath := filepath.Join(targetDir, "large_file.bin")
				assert.FileExists(t, targetFilePath)

				// Verify content
				content, err := os.ReadFile(targetFilePath)
				assert.NoError(t, err)
				assert.Equal(t, expectedContent, string(content))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			lds, objURI, targetDir, cleanup, err := tt.setupFunc()
			require.NoError(t, err)
			defer cleanup()

			// Get expected content if validation function exists
			var expectedContent string
			if tt.validateFunc != nil && !tt.expectedError {
				sourceFilePath := filepath.Join(lds.WorkingDirectory, objURI.ObjectName)
				contentBytes, err := os.ReadFile(sourceFilePath)
				require.NoError(t, err)
				expectedContent = string(contentBytes)
			}

			// Call the function under test
			err = lds.Download(objURI, targetDir)

			// Check results
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, targetDir, expectedContent)
				}
			}
		})
	}
}

func TestLocalDataStore_Upload(t *testing.T) {
	// Get the complete implementation of the Upload method from local_data_store.go
	sourceCode, err := os.ReadFile("local_data_store.go")
	if err == nil {
		if !strings.Contains(string(sourceCode), "func (lds *LocalDataStore) Upload") {
			t.Skip("Upload method not fully implemented yet, skipping tests")
		}
	}

	tests := []struct {
		name          string
		setupFunc     func() (*LocalDataStore, string, ObjectURI, func(), error)
		expectedError bool
		errorContains string
		validateFunc  func(*testing.T, *LocalDataStore, ObjectURI, string) // LDS, ObjURI, Content
	}{
		{
			name: "Upload file successfully",
			setupFunc: func() (*LocalDataStore, string, ObjectURI, func(), error) {
				// Create temp directories for source and working dir
				tempRoot, err := os.MkdirTemp("", "lds-upload-test")
				if err != nil {
					return nil, "", ObjectURI{}, nil, err
				}

				sourceDir := filepath.Join(tempRoot, "source")
				workingDir := filepath.Join(tempRoot, "working")

				// Create source directory
				err = os.MkdirAll(sourceDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, "", ObjectURI{}, nil, err
				}

				// Create working directory
				err = os.MkdirAll(workingDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, "", ObjectURI{}, nil, err
				}

				// Create test file in source
				testFile := "upload_test_file.txt"
				testContent := "Test content for upload"
				sourceFilePath := filepath.Join(sourceDir, testFile)
				err = os.WriteFile(sourceFilePath, []byte(testContent), 0644)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, "", ObjectURI{}, nil, err
				}

				// Create LocalDataStore
				lds := &LocalDataStore{
					WorkingDirectory: workingDir,
				}

				// Create ObjectURI
				objURI := ObjectURI{
					ObjectName: testFile,
				}

				cleanupFunc := func() {
					os.RemoveAll(tempRoot)
				}

				return lds, sourceFilePath, objURI, cleanupFunc, nil
			},
			expectedError: false,
			validateFunc: func(t *testing.T, lds *LocalDataStore, objURI ObjectURI, expectedContent string) {
				// Check if file was uploaded to working directory
				targetFilePath := filepath.Join(lds.WorkingDirectory, objURI.ObjectName)
				assert.FileExists(t, targetFilePath)

				// Verify content
				content, err := os.ReadFile(targetFilePath)
				assert.NoError(t, err)
				assert.Equal(t, expectedContent, string(content))
			},
		},
		{
			name: "Source file doesn't exist",
			setupFunc: func() (*LocalDataStore, string, ObjectURI, func(), error) {
				// Create temp directories
				tempRoot, err := os.MkdirTemp("", "lds-upload-missing-test")
				if err != nil {
					return nil, "", ObjectURI{}, nil, err
				}

				workingDir := filepath.Join(tempRoot, "working")
				err = os.MkdirAll(workingDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, "", ObjectURI{}, nil, err
				}

				// Non-existent source file
				sourceFilePath := filepath.Join(tempRoot, "non_existent_file.txt")

				// Create LocalDataStore
				lds := &LocalDataStore{
					WorkingDirectory: workingDir,
				}

				// Create ObjectURI
				objURI := ObjectURI{
					ObjectName: "uploaded_file.txt",
				}

				cleanupFunc := func() {
					os.RemoveAll(tempRoot)
				}

				return lds, sourceFilePath, objURI, cleanupFunc, nil
			},
			expectedError: true,
			errorContains: "failed to open source file",
		},
		{
			name: "Working directory doesn't exist",
			setupFunc: func() (*LocalDataStore, string, ObjectURI, func(), error) {
				// Create temp directories
				tempRoot, err := os.MkdirTemp("", "lds-upload-no-working-test")
				if err != nil {
					return nil, "", ObjectURI{}, nil, err
				}

				sourceDir := filepath.Join(tempRoot, "source")
				workingDir := filepath.Join(tempRoot, "nonexistent_working")

				// Create source directory
				err = os.MkdirAll(sourceDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, "", ObjectURI{}, nil, err
				}

				// Create test file in source
				testFile := "upload_test_file2.txt"
				testContent := "Test content for non-existent working dir"
				sourceFilePath := filepath.Join(sourceDir, testFile)
				err = os.WriteFile(sourceFilePath, []byte(testContent), 0644)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, "", ObjectURI{}, nil, err
				}

				// Create LocalDataStore
				lds := &LocalDataStore{
					WorkingDirectory: workingDir,
				}

				// Create ObjectURI
				objURI := ObjectURI{
					ObjectName: testFile,
				}

				cleanupFunc := func() {
					os.RemoveAll(tempRoot)
				}

				return lds, sourceFilePath, objURI, cleanupFunc, nil
			},
			expectedError: false, // Should create the working directory automatically
			validateFunc: func(t *testing.T, lds *LocalDataStore, objURI ObjectURI, expectedContent string) {
				// Check if working directory was created
				workingDirInfo, err := os.Stat(lds.WorkingDirectory)
				assert.NoError(t, err)
				assert.True(t, workingDirInfo.IsDir())

				// Check if file was uploaded
				targetFilePath := filepath.Join(lds.WorkingDirectory, objURI.ObjectName)
				assert.FileExists(t, targetFilePath)

				// Verify content
				content, err := os.ReadFile(targetFilePath)
				assert.NoError(t, err)
				assert.Equal(t, expectedContent, string(content))
			},
		},
		{
			name: "Upload large file",
			setupFunc: func() (*LocalDataStore, string, ObjectURI, func(), error) {
				// Create temp directories
				tempRoot, err := os.MkdirTemp("", "lds-upload-large-test")
				if err != nil {
					return nil, "", ObjectURI{}, nil, err
				}

				sourceDir := filepath.Join(tempRoot, "source")
				workingDir := filepath.Join(tempRoot, "working")

				// Create directories
				err = os.MkdirAll(sourceDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, "", ObjectURI{}, nil, err
				}
				err = os.MkdirAll(workingDir, os.ModePerm)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, "", ObjectURI{}, nil, err
				}

				// Create large test file in source (512KB)
				testFile := "large_upload_file.bin"
				testContent := generateLargeContent(512 * 1024)
				sourceFilePath := filepath.Join(sourceDir, testFile)
				err = os.WriteFile(sourceFilePath, []byte(testContent), 0644)
				if err != nil {
					os.RemoveAll(tempRoot)
					return nil, "", ObjectURI{}, nil, err
				}

				// Create LocalDataStore
				lds := &LocalDataStore{
					WorkingDirectory: workingDir,
				}

				// Create ObjectURI
				objURI := ObjectURI{
					ObjectName: testFile,
				}

				cleanupFunc := func() {
					os.RemoveAll(tempRoot)
				}

				return lds, sourceFilePath, objURI, cleanupFunc, nil
			},
			expectedError: false,
			validateFunc: func(t *testing.T, lds *LocalDataStore, objURI ObjectURI, expectedContent string) {
				// Check if file was uploaded to working directory
				targetFilePath := filepath.Join(lds.WorkingDirectory, objURI.ObjectName)
				assert.FileExists(t, targetFilePath)

				// Verify content
				content, err := os.ReadFile(targetFilePath)
				assert.NoError(t, err)
				assert.Equal(t, expectedContent, string(content))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			lds, sourcePath, objURI, cleanup, err := tt.setupFunc()
			require.NoError(t, err)
			defer cleanup()

			// Get expected content if validation function exists
			var expectedContent string
			if tt.validateFunc != nil && !tt.expectedError {
				contentBytes, err := os.ReadFile(sourcePath)
				require.NoError(t, err)
				expectedContent = string(contentBytes)
			}

			// Call the function under test
			err = lds.Upload(sourcePath, objURI)

			// Check results
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				if tt.validateFunc != nil {
					tt.validateFunc(t, lds, objURI, expectedContent)
				}
			}
		})
	}
}

func BenchmarkLocalDataStore_Operations(b *testing.B) {
	// Create temp directories for benchmark
	tempRoot, err := os.MkdirTemp("", "lds-benchmark")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempRoot)

	sourceDir := filepath.Join(tempRoot, "source")
	workingDir := filepath.Join(tempRoot, "working")
	targetDir := filepath.Join(tempRoot, "target")

	// Create directories
	err = os.MkdirAll(sourceDir, os.ModePerm)
	if err != nil {
		b.Fatalf("Failed to create source dir: %v", err)
	}
	err = os.MkdirAll(workingDir, os.ModePerm)
	if err != nil {
		b.Fatalf("Failed to create working dir: %v", err)
	}
	err = os.MkdirAll(targetDir, os.ModePerm)
	if err != nil {
		b.Fatalf("Failed to create target dir: %v", err)
	}

	// Create test files of different sizes
	files := []struct {
		name string
		size int // in KB
	}{
		{"small.txt", 10},   // 10KB
		{"medium.bin", 100}, // 100KB
		{"large.bin", 1024}, // 1MB
	}

	for _, file := range files {
		content := generateLargeContent(file.size * 1024)
		err := os.WriteFile(filepath.Join(sourceDir, file.name), []byte(content), 0644)
		if err != nil {
			b.Fatalf("Failed to create test file %s: %v", file.name, err)
		}
	}

	lds := &LocalDataStore{
		WorkingDirectory: workingDir,
	}

	// Benchmark Upload operation
	b.Run("Upload", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fileIdx := i % len(files)
			sourcePath := filepath.Join(sourceDir, files[fileIdx].name)
			objURI := ObjectURI{ObjectName: fmt.Sprintf("bench_upload_%d_%s", i, files[fileIdx].name)}

			err := lds.Upload(sourcePath, objURI)
			if err != nil {
				b.Fatalf("Upload failed: %v", err)
			}
		}
	})

	// Prepare files for download benchmark
	for i, file := range files {
		// Copy file to working directory
		sourcePath := filepath.Join(sourceDir, file.name)
		targetPath := filepath.Join(workingDir, fmt.Sprintf("bench_download_%d.bin", i))
		err := CopyByFilePath(sourcePath, targetPath)
		if err != nil {
			b.Fatalf("Failed to prepare download benchmark: %v", err)
		}
	}

	// Benchmark Download operation
	b.Run("Download", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			fileIdx := i % len(files)
			objURI := ObjectURI{ObjectName: fmt.Sprintf("bench_download_%d.bin", fileIdx)}
			downloadDir := filepath.Join(targetDir, fmt.Sprintf("download_%d", i))

			err := lds.Download(objURI, downloadDir)
			if err != nil {
				b.Fatalf("Download failed: %v", err)
			}
		}
	})
}
