package storage

import (
	"testing"
)

func TestUploadOptions(t *testing.T) {
	tests := []struct {
		name    string
		options []UploadOption
		check   func(*UploadOptions) bool
	}{
		{
			name:    "Default options",
			options: nil,
			check: func(opts *UploadOptions) bool {
				return opts.ChunkSizeInMB == 10 &&
					opts.Threads == 10 &&
					opts.ContentType == "" &&
					opts.StorageClass == "" &&
					opts.Metadata == nil
			},
		},
		{
			name:    "With chunk size",
			options: []UploadOption{WithUploadChunkSize(25)},
			check: func(opts *UploadOptions) bool {
				return opts.ChunkSizeInMB == 25
			},
		},
		{
			name:    "With threads",
			options: []UploadOption{WithUploadThreads(15)},
			check: func(opts *UploadOptions) bool {
				return opts.Threads == 15
			},
		},
		{
			name:    "With content type",
			options: []UploadOption{WithContentType("application/json")},
			check: func(opts *UploadOptions) bool {
				return opts.ContentType == "application/json"
			},
		},
		{
			name:    "With storage class",
			options: []UploadOption{WithStorageClass("STANDARD_IA")},
			check: func(opts *UploadOptions) bool {
				return opts.StorageClass == "STANDARD_IA"
			},
		},
		{
			name: "With metadata",
			options: []UploadOption{WithMetadata(map[string]string{
				"key1": "value1",
				"key2": "value2",
			})},
			check: func(opts *UploadOptions) bool {
				return len(opts.Metadata) == 2 &&
					opts.Metadata["key1"] == "value1" &&
					opts.Metadata["key2"] == "value2"
			},
		},
		{
			name: "Multiple options",
			options: []UploadOption{
				WithUploadChunkSize(50),
				WithUploadThreads(25),
				WithContentType("text/plain"),
				WithStorageClass("GLACIER"),
				WithMetadata(map[string]string{"author": "test"}),
			},
			check: func(opts *UploadOptions) bool {
				return opts.ChunkSizeInMB == 50 &&
					opts.Threads == 25 &&
					opts.ContentType == "text/plain" &&
					opts.StorageClass == "GLACIER" &&
					opts.Metadata["author"] == "test"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultUploadOptions()
			for _, opt := range tt.options {
				if err := opt(&opts); err != nil {
					t.Errorf("Failed to apply option: %v", err)
				}
			}

			if !tt.check(&opts) {
				t.Errorf("Options check failed for %s", tt.name)
			}
		})
	}
}
