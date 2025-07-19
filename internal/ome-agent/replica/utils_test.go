package replica

import (
	"testing"

	"github.com/oracle/oci-go-sdk/v65/objectstorage"
	"github.com/stretchr/testify/assert"

	"github.com/sgl-project/ome/pkg/hfutil/hub"
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
			result := convertToReplicationObjectsFromObjectSummary(tt.summaries)

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

func TestConvertToReplicationObjectsFromRepoFile(t *testing.T) {
	tests := []struct {
		name     string
		repo     []hub.RepoFile
		expected int
	}{
		{
			name:     "empty slice",
			repo:     []hub.RepoFile{},
			expected: 0,
		},
		{
			name: "single file",
			repo: []hub.RepoFile{
				{
					Path: "test-file.txt",
					Size: 1024,
					Type: "file",
				},
			},
			expected: 1,
		},
		{
			name: "multiple files",
			repo: []hub.RepoFile{
				{
					Path: "test/file1.txt",
					Size: 1024,
					Type: "file",
				},
				{
					Path: "test/file2.txt",
					Size: 2048,
					Type: "file",
				},
				{
					Path: "test/file3.txt",
					Size: 3072,
					Type: "file",
				},
			},
			expected: 3,
		},
		{
			name: "files with different types",
			repo: []hub.RepoFile{
				{
					Path: "config.json",
					Size: 512,
					Type: "file",
				},
				{
					Path: "model/",
					Size: 0,
					Type: "directory",
				},
				{
					Path: "model/weights.bin",
					Size: 1024 * 1024,
					Type: "file",
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToReplicationObjectsFromRepoFile(tt.repo)

			assert.Len(t, result, tt.expected)

			for i, obj := range result {
				// Verify the result implements ReplicationObject interface
				replicationObj, ok := obj.(RepoFileReplicationObject)
				assert.True(t, ok, "Result should be RepoFileReplicationObject")

				// Verify the underlying RepoFile is preserved
				assert.Equal(t, tt.repo[i], replicationObj.RepoFile)

				// Test the interface methods
				assert.Equal(t, tt.repo[i].Path, replicationObj.GetPath())
				assert.Equal(t, tt.repo[i].Size, replicationObj.GetSize())
				assert.Equal(t, tt.repo[i].Path, replicationObj.GetName())
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
			err := requireNonNil("testValue", tt.value)

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
