package zipper

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// createTestZip creates a test zip file with specified contents
func createTestZip(t *testing.T, zipFile string, files map[string]string) {
	outFile, err := os.Create(zipFile)
	if err != nil {
		t.Fatalf("Failed to create test zip file: %v", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	for name, content := range files {
		w, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("Failed to create zip entry %s: %v", name, err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatalf("Failed to write zip entry content: %v", err)
		}
	}
}

// Helper function to create a zip with a directory structure
func createNestedDirZip(t *testing.T, zipFile string) {
	outFile, err := os.Create(zipFile)
	if err != nil {
		t.Fatalf("Failed to create test zip file: %v", err)
	}
	defer outFile.Close()

	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	// Add directories (with trailing slash for directories)
	dirs := []string{"dir1/", "dir1/subdir1/", "dir2/"}
	for _, dir := range dirs {
		_, err := zipWriter.Create(dir)
		if err != nil {
			t.Fatalf("Failed to create directory entry %s: %v", dir, err)
		}
	}

	// Add files with content
	files := map[string]string{
		"file1.txt":              "Content of file1",
		"dir1/file2.txt":         "Content of file2",
		"dir1/subdir1/file3.txt": "Content of file3",
		"dir2/file4.txt":         "Content of file4",
	}

	for name, content := range files {
		w, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("Failed to create zip entry %s: %v", name, err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatalf("Failed to write zip entry content: %v", err)
		}
	}
}

// Verify if file exists and has expected content
func verifyFileContent(t *testing.T, path string, expectedContent string) {
	content, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("Failed to read file %s: %v", path, err)
		return
	}

	if string(content) != expectedContent {
		t.Errorf("Content mismatch for file %s.\nExpected: %s\nActual: %s",
			path, expectedContent, string(content))
	}
}

func TestUnzipBasicFiles(t *testing.T) {
	// Create a temporary directory for test
	tempDir, err := os.MkdirTemp("", "unzip_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a zip file with simple contents
	zipFile := filepath.Join(tempDir, "test.zip")
	files := map[string]string{
		"file1.txt": "Hello World",
		"file2.txt": "This is a test",
		"file3.txt": "Lorem ipsum dolor sit amet",
	}
	createTestZip(t, zipFile, files)

	// Create extraction directory
	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extraction directory: %v", err)
	}

	// Test Unzip function
	err = Unzip(zipFile, extractDir)
	if err != nil {
		t.Fatalf("Unzip failed: %v", err)
	}

	// Verify extracted files
	for name, content := range files {
		extractedPath := filepath.Join(extractDir, name)
		if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not extracted", name)
			continue
		}
		verifyFileContent(t, extractedPath, content)
	}
}

func TestUnzipWithDirectories(t *testing.T) {
	// Create a temporary directory for test
	tempDir, err := os.MkdirTemp("", "unzip_test_dirs")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a zip file with directory structure
	zipFile := filepath.Join(tempDir, "test_dirs.zip")
	createNestedDirZip(t, zipFile)

	// Create extraction directory
	extractDir := filepath.Join(tempDir, "extract_dirs")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extraction directory: %v", err)
	}

	// Test Unzip function
	err = Unzip(zipFile, extractDir)
	if err != nil {
		t.Fatalf("Unzip failed: %v", err)
	}

	// Verify directories were created
	dirs := []string{
		filepath.Join(extractDir, "dir1"),
		filepath.Join(extractDir, "dir1/subdir1"),
		filepath.Join(extractDir, "dir2"),
	}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if os.IsNotExist(err) {
			t.Errorf("Directory %s was not created", dir)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}

	// Verify file contents
	expectedFiles := map[string]string{
		filepath.Join(extractDir, "file1.txt"):              "Content of file1",
		filepath.Join(extractDir, "dir1/file2.txt"):         "Content of file2",
		filepath.Join(extractDir, "dir1/subdir1/file3.txt"): "Content of file3",
		filepath.Join(extractDir, "dir2/file4.txt"):         "Content of file4",
	}

	for path, expectedContent := range expectedFiles {
		verifyFileContent(t, path, expectedContent)
	}
}

func TestUnzipFileOverwrite(t *testing.T) {
	// Create a temporary directory for test
	tempDir, err := os.MkdirTemp("", "unzip_test_overwrite")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a zip file
	zipFile := filepath.Join(tempDir, "test_overwrite.zip")
	files := map[string]string{
		"file1.txt": "Original content",
	}
	createTestZip(t, zipFile, files)

	// Create extraction directory
	extractDir := filepath.Join(tempDir, "extract_overwrite")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		t.Fatalf("Failed to create extraction directory: %v", err)
	}

	// Create a file that will be overwritten
	existingFilePath := filepath.Join(extractDir, "file1.txt")
	if err := os.WriteFile(existingFilePath, []byte("Existing content"), 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Test Unzip function
	err = Unzip(zipFile, extractDir)
	if err != nil {
		t.Fatalf("Unzip failed: %v", err)
	}

	// Verify file was overwritten
	verifyFileContent(t, existingFilePath, "Original content")
}

func TestUnzipWithNonExistentZipFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unzip_test_nonexistent")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Try to unzip a non-existent file
	err = Unzip(filepath.Join(tempDir, "non_existent.zip"), tempDir)
	if err == nil {
		t.Error("Expected error when unzipping non-existent file, but got none")
	}
}

func TestUnzipWithInvalidZipFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unzip_test_invalid")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an invalid zip file (just plain text)
	invalidZipPath := filepath.Join(tempDir, "invalid.zip")
	if err := os.WriteFile(invalidZipPath, []byte("This is not a valid ZIP file"), 0644); err != nil {
		t.Fatalf("Failed to create invalid zip file: %v", err)
	}

	// Try to unzip an invalid file
	err = Unzip(invalidZipPath, tempDir)
	if err == nil {
		t.Error("Expected error when unzipping invalid file, but got none")
	}
}

func TestUnzipWithInvalidExtractionPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unzip_test_invalid_extract")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple zip file
	zipFile := filepath.Join(tempDir, "test.zip")
	files := map[string]string{
		"file1.txt": "Test content",
	}
	createTestZip(t, zipFile, files)

	// Create a file that will conflict with our extraction directory
	extractPath := filepath.Join(tempDir, "blocked_extract")
	if err := os.WriteFile(extractPath, []byte("This is a file, not a directory"), 0644); err != nil {
		t.Fatalf("Failed to create blocking file: %v", err)
	}

	// On some systems, this will fail because we can't create a directory where a file exists
	// But on others, it might succeed by removing the file first, so we need to check
	// the behavior of the Unzip function
	err = Unzip(zipFile, extractPath)
	if err == nil {
		// If no error, verify that the directory was created and the file was extracted
		extractedPath := filepath.Join(extractPath, "file1.txt")
		if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
			t.Error("Unzip didn't fail, but file wasn't extracted either")
		}
	}
}
