package storage

import (
	"path/filepath"
	"testing"
)

func TestComputeLocalPath(t *testing.T) {
	tests := []struct {
		name       string
		targetDir  string
		objectName string
		opts       DownloadOptions
		expected   string
	}{
		{
			name:       "Default behavior",
			targetDir:  "/tmp/downloads",
			objectName: "data/files/document.pdf",
			opts:       DownloadOptions{},
			expected:   filepath.Join("/tmp/downloads", "data/files/document.pdf"),
		},
		{
			name:       "UseBaseNameOnly",
			targetDir:  "/tmp/downloads",
			objectName: "data/files/document.pdf",
			opts: DownloadOptions{
				UseBaseNameOnly: true,
			},
			expected: filepath.Join("/tmp/downloads", "document.pdf"),
		},
		{
			name:       "StripPrefix with match",
			targetDir:  "/tmp/downloads",
			objectName: "data/files/document.pdf",
			opts: DownloadOptions{
				StripPrefix:   true,
				PrefixToStrip: "data/",
			},
			expected: filepath.Join("/tmp/downloads", "files/document.pdf"),
		},
		{
			name:       "StripPrefix without match",
			targetDir:  "/tmp/downloads",
			objectName: "other/files/document.pdf",
			opts: DownloadOptions{
				StripPrefix:   true,
				PrefixToStrip: "data/",
			},
			expected: filepath.Join("/tmp/downloads", "other/files/document.pdf"),
		},
		{
			name:       "JoinWithTailOverlap - overlap exists",
			targetDir:  "/local/data",
			objectName: "data/files/document.pdf",
			opts: DownloadOptions{
				JoinWithTailOverlap: true,
			},
			expected: filepath.Join("/local/data", "files/document.pdf"),
		},
		{
			name:       "JoinWithTailOverlap - no overlap",
			targetDir:  "/local/downloads",
			objectName: "data/files/document.pdf",
			opts: DownloadOptions{
				JoinWithTailOverlap: true,
			},
			expected: filepath.Join("/local/downloads", "data/files/document.pdf"),
		},
		{
			name:       "JoinWithTailOverlap - multiple level overlap",
			targetDir:  "/local/project/data/files",
			objectName: "data/files/subfolder/document.pdf",
			opts: DownloadOptions{
				JoinWithTailOverlap: true,
			},
			expected: filepath.Join("/local/project/data/files", "subfolder/document.pdf"),
		},
		{
			name:       "Empty object name",
			targetDir:  "/tmp/downloads",
			objectName: "",
			opts:       DownloadOptions{},
			expected:   "/tmp/downloads",
		},
		{
			name:       "UseBaseNameOnly with nested path",
			targetDir:  "/downloads",
			objectName: "very/deep/nested/path/to/file.txt",
			opts: DownloadOptions{
				UseBaseNameOnly: true,
			},
			expected: filepath.Join("/downloads", "file.txt"),
		},
		{
			name:       "StripPrefix removes entire path except filename",
			targetDir:  "/downloads",
			objectName: "prefix/file.txt",
			opts: DownloadOptions{
				StripPrefix:   true,
				PrefixToStrip: "prefix/",
			},
			expected: filepath.Join("/downloads", "file.txt"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ComputeLocalPath(tt.targetDir, tt.objectName, tt.opts)
			if result != tt.expected {
				t.Errorf("ComputeLocalPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestJoinWithTailOverlap(t *testing.T) {
	tests := []struct {
		name       string
		targetDir  string
		objectName string
		expected   string
	}{
		{
			name:       "Single component overlap",
			targetDir:  "/local/data",
			objectName: "data/files/doc.pdf",
			expected:   filepath.Join("/local/data", "files/doc.pdf"),
		},
		{
			name:       "Multiple component overlap",
			targetDir:  "/local/project/src/data",
			objectName: "src/data/models/file.txt",
			expected:   filepath.Join("/local/project/src/data", "models/file.txt"),
		},
		{
			name:       "No overlap",
			targetDir:  "/local/downloads",
			objectName: "uploads/file.txt",
			expected:   filepath.Join("/local/downloads", "uploads/file.txt"),
		},
		{
			name:       "Complete overlap",
			targetDir:  "/path/to/files",
			objectName: "path/to/files",
			expected:   "/path/to/files",
		},
		{
			name:       "Partial word not overlapping",
			targetDir:  "/local/dat",
			objectName: "data/file.txt",
			expected:   filepath.Join("/local/dat", "data/file.txt"),
		},
		{
			name:       "Empty target",
			targetDir:  "",
			objectName: "data/file.txt",
			expected:   "data/file.txt",
		},
		{
			name:       "Empty object",
			targetDir:  "/local/data",
			objectName: "",
			expected:   "/local/data",
		},
		{
			name:       "Root paths",
			targetDir:  "/",
			objectName: "data/file.txt",
			expected:   filepath.Join("/", "data/file.txt"),
		},
		{
			name:       "Trailing slashes",
			targetDir:  "/local/data/",
			objectName: "data/files/",
			expected:   filepath.Join("/local/data", "files"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinWithTailOverlap(tt.targetDir, tt.objectName)
			if result != tt.expected {
				t.Errorf("joinWithTailOverlap(%q, %q) = %v, want %v",
					tt.targetDir, tt.objectName, result, tt.expected)
			}
		})
	}
}

func TestRemoveEmptyParts(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "No empty parts",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "Some empty parts",
			input:    []string{"", "a", "", "b", ""},
			expected: []string{"a", "b"},
		},
		{
			name:     "All empty",
			input:    []string{"", "", ""},
			expected: []string{},
		},
		{
			name:     "Empty slice",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeEmptyParts(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("removeEmptyParts() returned %d elements, want %d",
					len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("removeEmptyParts()[%d] = %v, want %v",
						i, result[i], tt.expected[i])
				}
			}
		})
	}
}
