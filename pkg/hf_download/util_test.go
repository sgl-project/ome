package hf_download

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"testing"
)

func TestVerifyChecksum(t *testing.T) {
	// Create a temp file with known content
	content := []byte("test content")
	tmpFile, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.Write(content)
	if err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Compute expected checksum
	hasher := sha256.New()
	hasher.Write(content)
	expectedChecksum := hex.EncodeToString(hasher.Sum(nil))

	// Test with correct checksum
	err = VerifyChecksum(tmpFile.Name(), expectedChecksum)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Test with incorrect checksum
	err = VerifyChecksum(tmpFile.Name(), "wrongchecksum")
	if err == nil {
		t.Error("expected error due to checksum mismatch, got none")
	}
}

func TestMergeFiles(t *testing.T) {

	t.Run("Non-Existent Temp Folder", func(t *testing.T) {
		nonExistentDir := "/nonexistentdir"
		outputFile := path.Join(nonExistentDir, "output.txt")
		err := MergeFiles(nonExistentDir, outputFile, 3)
		if err == nil {
			t.Error("expected error for non-existent temp folder, got none")
		}
	})

	t.Run("Missing Chunk File", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "missing_chunk_test")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create only 2 out of 3 expected chunks
		for i := 0; i < 2; i++ {
			tmpFilePath := path.Join(tempDir, fmt.Sprintf("output_%d.tmp", i))
			if err := os.WriteFile(tmpFilePath, []byte(fmt.Sprintf("chunk%d", i)), 0644); err != nil {
				t.Fatalf("failed to write chunk %d: %v", i, err)
			}
		}

		outputFile := path.Join(tempDir, "output.txt")
		err = MergeFiles(tempDir, outputFile, 3)
		if err == nil {
			t.Error("expected error for missing chunk file, got none")
		}
	})

	t.Run("Partial Write Error Handling", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "partial_write_test")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		numChunks := 3
		for i := 0; i < numChunks; i++ {
			tmpFilePath := path.Join(tempDir, fmt.Sprintf("output_%d.tmp", i))
			if err := os.WriteFile(tmpFilePath, []byte("chunk content"), 0644); err != nil {
				t.Fatalf("failed to write chunk %d: %v", i, err)
			}
		}

		// Use a non-writable directory for the output file to simulate a write error
		outputFile := "/nonwritable/output.txt"
		err = MergeFiles(tempDir, outputFile, numChunks)
		if err == nil {
			t.Error("expected error due to non-writable output directory, got none")
		}
	})
}

func TestAdjustStartByte(t *testing.T) {
	// Create a temp file to simulate download progress
	tmpFile, err := os.CreateTemp("", "resume_test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	progress := make(chan int64, 1)
	start, end := int64(0), int64(100)

	// Write some data to the temp file
	if _, err := tmpFile.Write(make([]byte, 20)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}

	newStart, err := AdjustStartByte(tmpFile.Name(), start, end, progress)
	if err != nil {
		t.Fatalf("AdjustStartByte returned error: %v", err)
	}

	expectedStart := int64(8) // Adjusted with compensationBytes of 12
	if newStart != expectedStart {
		t.Errorf("expected start %d, got %d", expectedStart, newStart)
	}
}

func TestNeedsDownload(t *testing.T) {
	// Create a temp file with specific size
	tmpFile, err := os.CreateTemp("", "need_download_test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write some data
	expectedSize := 50
	if _, err := tmpFile.Write(make([]byte, expectedSize)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}

	// Test cases
	tests := []struct {
		remoteSize int
		expected   bool
	}{
		{50, false}, // file already has expected size
		{100, true}, // remote size differs
		{0, true},   // file not present (simulate by using another file)
	}

	for _, tt := range tests {
		result := NeedsDownload(tmpFile.Name(), tt.remoteSize)
		if result != tt.expected {
			t.Errorf("for remoteSize %d, expected %v, got %v", tt.remoteSize, tt.expected, result)
		}
	}
}

func TestWriteToFile(t *testing.T) {
	content := "this is the content to write"
	reader := io.NopCloser(strings.NewReader(content))

	tmpFile, err := os.CreateTemp("", "write_to_file_test")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	progress := make(chan int64, 1)
	err = WriteToFile(reader, tmpFile.Name(), progress)
	if err != nil {
		t.Fatalf("WriteToFile returned error: %v", err)
	}

	// Check content
	writtenContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(writtenContent) != content {
		t.Errorf("expected content %q, got %q", content, string(writtenContent))
	}
}
