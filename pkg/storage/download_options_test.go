package storage

import (
	"testing"
)

func TestDownloadOptions(t *testing.T) {
	tests := []struct {
		name    string
		options []DownloadOption
		check   func(*DownloadOptions) bool
	}{
		{
			name:    "Default options",
			options: nil,
			check: func(opts *DownloadOptions) bool {
				return opts.SizeThresholdInMB == 10 &&
					opts.ChunkSizeInMB == 10 &&
					opts.Threads == 10 &&
					!opts.DisableOverride
			},
		},
		{
			name:    "With size threshold",
			options: []DownloadOption{WithSizeThreshold(50)},
			check: func(opts *DownloadOptions) bool {
				return opts.SizeThresholdInMB == 50
			},
		},
		{
			name:    "With chunk size",
			options: []DownloadOption{WithChunkSize(20)},
			check: func(opts *DownloadOptions) bool {
				return opts.ChunkSizeInMB == 20
			},
		},
		{
			name:    "With threads",
			options: []DownloadOption{WithThreads(5)},
			check: func(opts *DownloadOptions) bool {
				return opts.Threads == 5
			},
		},
		{
			name:    "Force standard",
			options: []DownloadOption{WithForceStandard(true)},
			check: func(opts *DownloadOptions) bool {
				return opts.ForceStandard
			},
		},
		{
			name:    "Force multipart",
			options: []DownloadOption{WithForceMultipart(true)},
			check: func(opts *DownloadOptions) bool {
				return opts.ForceMultipart
			},
		},
		{
			name:    "Disable override",
			options: []DownloadOption{WithOverrideEnabled(false)},
			check: func(opts *DownloadOptions) bool {
				return opts.DisableOverride
			},
		},
		{
			name:    "Enable override",
			options: []DownloadOption{WithOverrideEnabled(true)},
			check: func(opts *DownloadOptions) bool {
				return !opts.DisableOverride
			},
		},
		{
			name:    "With exclude patterns",
			options: []DownloadOption{WithExcludePatterns([]string{"*.tmp", "*.log"})},
			check: func(opts *DownloadOptions) bool {
				return len(opts.ExcludePatterns) == 2 &&
					opts.ExcludePatterns[0] == "*.tmp" &&
					opts.ExcludePatterns[1] == "*.log"
			},
		},
		{
			name:    "With strip prefix",
			options: []DownloadOption{WithStripPrefix("prefix/")},
			check: func(opts *DownloadOptions) bool {
				return opts.StripPrefix && opts.PrefixToStrip == "prefix/"
			},
		},
		{
			name:    "With base name only",
			options: []DownloadOption{WithBaseNameOnly(true)},
			check: func(opts *DownloadOptions) bool {
				return opts.UseBaseNameOnly
			},
		},
		{
			name:    "With tail overlap",
			options: []DownloadOption{WithTailOverlap(true)},
			check: func(opts *DownloadOptions) bool {
				return opts.JoinWithTailOverlap
			},
		},
		{
			name: "Multiple options",
			options: []DownloadOption{
				WithSizeThreshold(100),
				WithChunkSize(50),
				WithThreads(20),
				WithForceMultipart(true),
			},
			check: func(opts *DownloadOptions) bool {
				return opts.SizeThresholdInMB == 100 &&
					opts.ChunkSizeInMB == 50 &&
					opts.Threads == 20 &&
					opts.ForceMultipart
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultDownloadOptions()
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
