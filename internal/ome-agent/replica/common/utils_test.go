package common

import (
	"fmt"
	"testing"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/stretchr/testify/assert"

	"github.com/sgl-project/ome/pkg/afero"
	"github.com/sgl-project/ome/pkg/xet"
)

func TestConvertToReplicationObjectsFromObjectSummary(t *testing.T) {
	tests := []struct {
		name      string
		summaries []objectstorage.ObjectSummary
		expected  int
	}{
		{
			name:      "empty slice",
			summaries: []objectstorage.ObjectSummary{},
			expected:  0,
		},
		{
			name: "single object",
			summaries: []objectstorage.ObjectSummary{
				{
					Name: stringPtr("test-file.txt"),
					Size: int64Ptr(1024),
				},
			},
			expected: 1,
		},
		{
			name: "multiple objects",
			summaries: []objectstorage.ObjectSummary{
				{
					Name: stringPtr("models/file1.txt"),
					Size: int64Ptr(1024),
				},
				{
					Name: stringPtr("models/file2.txt"),
					Size: int64Ptr(2048),
				},
				{
					Name: stringPtr("models/file3.txt"),
					Size: int64Ptr(3072),
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToReplicationObjectsFromObjectSummary(tt.summaries)

			assert.Len(t, result, tt.expected)

			for i, obj := range result {
				// Verify the result implements ReplicationObject interface
				replicationObj, ok := obj.(ObjectSummaryReplicationObject)
				assert.True(t, ok, "Result should be ObjectSummaryReplicationObject")

				// Verify the underlying ObjectSummary is preserved
				assert.Equal(t, tt.summaries[i], replicationObj.ObjectSummary)

				// Test the interface methods
				if tt.summaries[i].Name != nil {
					assert.Equal(t, *tt.summaries[i].Name, replicationObj.GetName())
				} else {
					assert.Equal(t, "", replicationObj.GetName())
				}

				if tt.summaries[i].Size != nil {
					assert.Equal(t, *tt.summaries[i].Size, replicationObj.GetSize())
				} else {
					assert.Equal(t, int64(0), replicationObj.GetSize())
				}

				assert.Equal(t, replicationObj.GetName(), replicationObj.GetPath())
			}
		})
	}
}

func TestConvertToReplicationObjectsFromFileInfo(t *testing.T) {
	tests := []struct {
		name     string
		files    []afero.FileEntry
		expected int
	}{
		{
			name:     "empty slice",
			files:    []afero.FileEntry{},
			expected: 0,
		},
		{
			name: "multiple files",
			files: func() []afero.FileEntry {
				fs := afero.NewMemMapFs()
				paths := []string{"/mnt/pvc/models/file1.bin", "/mnt/pvc/models/file2.bin", "/mnt/pvc/models/file3.bin"}
				entries := make([]afero.FileEntry, 0, len(paths))
				for i, p := range paths {
					content := []byte(fmt.Sprintf("data%d", i))
					afero.WriteFile(fs, p, content, 0644)
					fileInfo, _ := fs.Stat(p)
					entries = append(entries, afero.FileEntry{FileInfo: fileInfo, FilePath: p})
				}
				return entries
			}(),
			expected: 3,
		},
		{
			name: "nil FileInfo",
			files: []afero.FileEntry{
				{FileInfo: nil, FilePath: ""},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToReplicationObjectsFromPVCFileEntry(tt.files)
			assert.Len(t, result, tt.expected)
			for i, obj := range result {
				replicationObj, ok := obj.(PVCFileReplicationObject)
				assert.True(t, ok, "Result should be PVCFileReplicationObject")
				// Verify the underlying FileEntry is preserved
				assert.Equal(t, tt.files[i], replicationObj.FileEntry)
				// Test the interface methods
				if tt.files[i].FileInfo != nil {
					assert.Equal(t, tt.files[i].FileInfo.Name(), replicationObj.GetName())
					assert.Equal(t, tt.files[i].FileInfo.Size(), replicationObj.GetSize())
				} else {
					assert.Equal(t, "", replicationObj.GetName())
					assert.Equal(t, int64(0), replicationObj.GetSize())
				}
				assert.Equal(t, tt.files[i].FilePath, replicationObj.GetPath())
			}
		})
	}
}

func TestConvertToReplicationObjectsFromHFRepoFileInfo(t *testing.T) {
	tests := []struct {
		name      string
		repoFiles []xet.FileInfo
		expected  int
	}{
		{
			name:      "empty slice",
			repoFiles: []xet.FileInfo{},
			expected:  0,
		},
		{
			name: "single file",
			repoFiles: []xet.FileInfo{
				{
					Path: "models/model.bin",
					Hash: "abc123def456",
					Size: 1024,
				},
			},
			expected: 1,
		},
		{
			name: "multiple files",
			repoFiles: []xet.FileInfo{
				{
					Path: "models/file1.bin",
					Hash: "hash1",
					Size: 1024,
				},
				{
					Path: "models/file2.bin",
					Hash: "hash2",
					Size: 2048,
				},
				{
					Path: "models/file3.bin",
					Hash: "hash3",
					Size: 3072,
				},
			},
			expected: 3,
		},
		{
			name: "files with empty path, zero size, without hash",
			repoFiles: []xet.FileInfo{
				{
					Path: "",
					Size: 0,
				},
				{
					Path: "models/empty.bin",
					Size: 0,
				},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertToReplicationObjectsFromHFRepoFileInfo(tt.repoFiles)

			assert.Len(t, result, tt.expected)

			for i, obj := range result {
				// Verify the result implements ReplicationObject interface
				replicationObj, ok := obj.(HFRepoFileInfoReplicationObject)
				assert.True(t, ok, "Result should be HFRepoFileInfoReplicationObject")

				// Verify the underlying FileInfo is preserved
				assert.Equal(t, tt.repoFiles[i], replicationObj.FileInfo)

				// Test the interface methods
				assert.Equal(t, tt.repoFiles[i].Path, replicationObj.GetPath())
				assert.Equal(t, tt.repoFiles[i].Path, replicationObj.GetName())
				assert.Equal(t, int64(tt.repoFiles[i].Size), replicationObj.GetSize())
			}
		})
	}
}

func TestRequireNonNil(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected error
	}{
		{
			name:     "nil interface",
			value:    nil,
			expected: assert.AnError,
		},
		{
			name:     "nil pointer",
			value:    (*string)(nil),
			expected: assert.AnError,
		},
		{
			name:     "nil slice",
			value:    []string(nil),
			expected: assert.AnError,
		},
		{
			name:     "nil map",
			value:    map[string]string(nil),
			expected: assert.AnError,
		},
		{
			name:     "nil function",
			value:    (func())(nil),
			expected: assert.AnError,
		},
		{
			name:     "nil channel",
			value:    (chan int)(nil),
			expected: assert.AnError,
		},
		{
			name:     "valid string",
			value:    "test",
			expected: nil,
		},
		{
			name:     "valid int",
			value:    42,
			expected: nil,
		},
		{
			name:     "valid pointer",
			value:    stringPtr("test"),
			expected: nil,
		},
		{
			name:     "valid slice",
			value:    []string{"test"},
			expected: nil,
		},
		{
			name:     "valid map",
			value:    map[string]string{"key": "value"},
			expected: nil,
		},
		{
			name:     "valid function",
			value:    func() {},
			expected: nil,
		},
		{
			name:     "valid channel",
			value:    make(chan int),
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RequireNonNil("testValue", tt.value)

			if tt.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "required testValue is nil")
			}
		})
	}
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}
