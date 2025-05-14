package zipper

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func setupTestDir(t *testing.T) (string, func()) {
	// Create a temporary directory for our tests
	tempDir, err := os.MkdirTemp("", "zipper_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Create some test files and directories
	testDirs := []string{
		filepath.Join(tempDir, "dir1"),
		filepath.Join(tempDir, "dir1/subdir1"),
		filepath.Join(tempDir, "dir2"),
		filepath.Join(tempDir, "dir3/subdir2"), // Will create parent too
	}

	for _, dir := range testDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create test dir %s: %v", dir, err)
		}
	}

	// Create some test files with content
	testFiles := map[string]string{
		filepath.Join(tempDir, "file1.txt"):              "Content of file1",
		filepath.Join(tempDir, "dir1/file2.txt"):         "Content of file2",
		filepath.Join(tempDir, "dir1/subdir1/file3.txt"): "Content of file3",
		filepath.Join(tempDir, "dir2/file4.txt"):         "Content of file4",
		filepath.Join(tempDir, "dir3/subdir2/file5.txt"): "Content of file5",
	}

	for filePath, content := range testFiles {
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	// Return cleanup function
	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// Helper to get files in a zip archive
func getFilesInZip(t *testing.T, zipPath string) []string {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("Failed to open zip file %s: %v", zipPath, err)
	}
	defer reader.Close()

	var files []string
	for _, file := range reader.File {
		files = append(files, file.Name)
	}

	// Sort for consistent comparison
	sort.Strings(files)
	return files
}

// Helper to read file content
func readFileContent(t *testing.T, path string) string {
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", path, err)
	}
	return string(content)
}

func TestZipDirectory(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Define the output zip file
	zipFile := filepath.Join(tempDir, "output.zip")

	// Test ZipDirectory
	err := ZipDirectory(tempDir, zipFile)
	if err != nil {
		t.Fatalf("ZipDirectory failed: %v", err)
	}

	// Verify the zip file exists
	if _, err := os.Stat(zipFile); os.IsNotExist(err) {
		t.Fatalf("Output zip file was not created")
	}

	// Get a list of files in the zip archive
	zipFiles := getFilesInZip(t, zipFile)

	// Verify expected files exist in the zip
	expectedFiles := []string{
		"file1.txt",
		"dir1/file2.txt",
		"dir1/subdir1/file3.txt",
		"dir2/file4.txt",
		"dir3/subdir2/file5.txt",
	}

	// Check that all expected files are in the zip
	for _, expected := range expectedFiles {
		found := false
		for _, zipFile := range zipFiles {
			if zipFile == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file %s not found in zip archive", expected)
		}
	}
}

func TestZipFilesWithPrefixes(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Define the output zip file
	zipFile := filepath.Join(tempDir, "prefixed.zip")

	// Test ZipFilesWithPrefixes with "dir1/" and "dir3/" prefixes
	prefixes := []string{"dir1/", "dir3/"}
	err := ZipFilesWithPrefixes(tempDir, zipFile, prefixes)
	if err != nil {
		t.Fatalf("ZipFilesWithPrefixes failed: %v", err)
	}

	// Verify the zip file exists
	if _, err := os.Stat(zipFile); os.IsNotExist(err) {
		t.Fatalf("Output zip file was not created")
	}

	// Expected files with specified prefixes
	expectedFiles := []string{
		"dir1/file2.txt",
		"dir1/subdir1/file3.txt",
		"dir3/subdir2/file5.txt",
	}

	// Get a list of files in the zip archive
	zipFiles := getFilesInZip(t, zipFile)

	// Check that all expected files are in the zip
	for _, expected := range expectedFiles {
		found := false
		for _, zipFile := range zipFiles {
			if zipFile == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file %s not found in zip archive", expected)
		}
	}

	// Check that no files with other prefixes are in the zip
	for _, zipFile := range zipFiles {
		if !strings.HasPrefix(zipFile, "dir1/") && !strings.HasPrefix(zipFile, "dir3/") {
			t.Errorf("Unexpected file %s found in zip archive", zipFile)
		}
	}
}

func TestUnzip(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Create source directory with files for zipping
	sourceDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	// Create files to zip
	testFiles := map[string]string{
		filepath.Join(sourceDir, "file1.txt"):              "Content of file1",
		filepath.Join(sourceDir, "dir1/file2.txt"):         "Content of file2",
		filepath.Join(sourceDir, "dir1/subdir1/file3.txt"): "Content of file3",
	}

	for filePath, content := range testFiles {
		dir := filepath.Dir(filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filePath, err)
		}
	}

	// Create a zip file
	zipFile := filepath.Join(tempDir, "test.zip")
	err := ZipDirectory(sourceDir, zipFile)
	if err != nil {
		t.Fatalf("ZipDirectory failed during setup: %v", err)
	}

	// Create a directory for extraction
	extractDir := filepath.Join(tempDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extraction directory: %v", err)
	}

	// Test Unzip function
	err = Unzip(zipFile, extractDir)
	if err != nil {
		t.Fatalf("Unzip failed: %v", err)
	}

	// Verify file contents
	filesToCheck := []string{
		"file1.txt",
		"dir1/file2.txt",
		"dir1/subdir1/file3.txt",
	}

	for _, file := range filesToCheck {
		originalContent := readFileContent(t, filepath.Join(sourceDir, file))
		extractedFilePath := filepath.Join(extractDir, file)

		// Check that the extracted file exists
		if _, err := os.Stat(extractedFilePath); os.IsNotExist(err) {
			t.Errorf("Expected extracted file %s does not exist", extractedFilePath)
			continue
		}

		extractedContent := readFileContent(t, extractedFilePath)
		if originalContent != extractedContent {
			t.Errorf("Content mismatch for file %s.\nOriginal: %s\nExtracted: %s",
				file, originalContent, extractedContent)
		}
	}
}

func TestUnzipError(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Test with a non-existent zip file
	nonExistentFile := filepath.Join(tempDir, "does_not_exist.zip")
	err := Unzip(nonExistentFile, tempDir)
	if err == nil {
		t.Error("Expected error when unzipping non-existent file, but got none")
	}

	// Test with an invalid zip file
	invalidZipFile := filepath.Join(tempDir, "invalid.zip")
	err = os.WriteFile(invalidZipFile, []byte("this is not a valid zip file"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid zip file: %v", err)
	}

	err = Unzip(invalidZipFile, tempDir)
	if err == nil {
		t.Error("Expected error when unzipping invalid file, but got none")
	}
}

func TestZipDirectoryErrors(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Test with non-existent source directory
	nonExistentDir := filepath.Join(tempDir, "does_not_exist")
	zipFile := filepath.Join(tempDir, "error.zip")

	err := ZipDirectory(nonExistentDir, zipFile)
	if err == nil {
		t.Error("Expected error when zipping non-existent directory, but got none")
	}

	// Test with invalid output path
	invalidOutputPath := filepath.Join(tempDir, "non-existent-dir", "output.zip")

	err = ZipDirectory(tempDir, invalidOutputPath)
	if err == nil {
		t.Error("Expected error when using invalid output path, but got none")
	}
}

func TestZipFilesWithPrefixesErrors(t *testing.T) {
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	// Test with non-existent source directory
	nonExistentDir := filepath.Join(tempDir, "does_not_exist")
	zipFile := filepath.Join(tempDir, "error.zip")
	prefixes := []string{"dir1/"}

	err := ZipFilesWithPrefixes(nonExistentDir, zipFile, prefixes)
	if err == nil {
		t.Error("Expected error when zipping with non-existent directory, but got none")
	}

	// Test with invalid output path
	invalidOutputPath := filepath.Join(tempDir, "non-existent-dir", "output.zip")

	err = ZipFilesWithPrefixes(tempDir, invalidOutputPath, prefixes)
	if err == nil {
		t.Error("Expected error when using invalid output path, but got none")
	}
}
