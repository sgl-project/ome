package storage

// WithSizeThreshold sets the size threshold (in MB) above which multipart download is used
func WithSizeThreshold(sizeThresholdInMB int) DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.SizeThresholdInMB = sizeThresholdInMB
		return nil
	}
}

// WithChunkSize sets the chunk size (in MB) for multipart downloads
func WithChunkSize(chunkSizeInMB int) DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.ChunkSizeInMB = chunkSizeInMB
		return nil
	}
}

// WithThreads sets the number of concurrent download threads
func WithThreads(threads int) DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.Threads = threads
		return nil
	}
}

// WithForceStandard forces standard download regardless of file size
func WithForceStandard(force bool) DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.ForceStandard = force
		return nil
	}
}

// WithForceMultipart forces multipart download regardless of file size
func WithForceMultipart(force bool) DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.ForceMultipart = force
		return nil
	}
}

// WithOverrideEnabled controls whether to re-download files if a valid local copy exists
func WithOverrideEnabled(enabled bool) DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.DisableOverride = !enabled
		return nil
	}
}

// WithExcludePatterns sets patterns for object names to exclude from download
func WithExcludePatterns(patterns []string) DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.ExcludePatterns = patterns
		return nil
	}
}

// WithStripPrefix enables stripping a specified prefix from object paths during download
func WithStripPrefix(prefix string) DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.StripPrefix = true
		opts.PrefixToStrip = prefix
		return nil
	}
}

// WithBaseNameOnly configures downloads to use only the object's base name (filename)
func WithBaseNameOnly(enabled bool) DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.UseBaseNameOnly = enabled
		return nil
	}
}

// WithTailOverlap enables joining paths with tail overlap detection
func WithTailOverlap(enabled bool) DownloadOption {
	return func(opts *DownloadOptions) error {
		opts.JoinWithTailOverlap = enabled
		return nil
	}
}
