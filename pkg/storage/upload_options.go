package storage

// WithUploadChunkSize sets the chunk size (in MB) for multipart uploads
func WithUploadChunkSize(chunkSizeInMB int) UploadOption {
	return func(opts *UploadOptions) error {
		opts.ChunkSizeInMB = chunkSizeInMB
		return nil
	}
}

// WithUploadThreads sets the number of concurrent upload threads
func WithUploadThreads(threads int) UploadOption {
	return func(opts *UploadOptions) error {
		opts.Threads = threads
		return nil
	}
}

// WithContentType sets the content type of the object
func WithContentType(contentType string) UploadOption {
	return func(opts *UploadOptions) error {
		opts.ContentType = contentType
		return nil
	}
}

// WithMetadata sets metadata to attach to the object
func WithMetadata(metadata map[string]string) UploadOption {
	return func(opts *UploadOptions) error {
		opts.Metadata = metadata
		return nil
	}
}

// WithStorageClass sets the storage class/tier
func WithStorageClass(storageClass string) UploadOption {
	return func(opts *UploadOptions) error {
		opts.StorageClass = storageClass
		return nil
	}
}
