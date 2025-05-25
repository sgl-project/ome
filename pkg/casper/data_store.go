package casper

// ObjectURI defines the identity and location of an object in a logical or physical data store.
// It can represent an object in local file systems, object storage (like OCI, S3), or other backends.
type ObjectURI struct {
	Namespace  string `mapstructure:"namespace" json:"namespace"`                         // Optional: Storage namespace (used in OCI Object Storage)
	BucketName string `mapstructure:"bucket_name" json:"bucket_name" validate:"required"` // Name of the bucket or container (required)
	ObjectName string `mapstructure:"object_name" json:"object_name"`                     // Full object name or file name (e.g., "foo/bar.txt")
	Prefix     string `mapstructure:"prefix" json:"prefix"`                               // Optional prefix to identify folder or logical path
	Region     string `mapstructure:"region" json:"region"`                               // Optional region where the object is located
}

// DataStore defines a common interface for reading and writing data to/from a storage backend.
// Implementations may include local file systems, OCI Object Storage, S3, etc.
// All download methods now use functional options for flexible configuration.
type DataStore interface {
	// Download retrieves the object specified by `source` and writes it into the local `target` directory.
	// Uses functional options for configuration.
	//
	// Example usage:
	//   err := store.Download(source, target, WithThreads(10), WithChunkSize(16))
	//
	// Parameters:
	//   - source: the object to be downloaded, including bucket and object name
	//   - target: the local path where the object should be placed
	//   - opts: functional options to configure download behavior
	//
	// Returns:
	//   - error: non-nil if the operation fails
	Download(source ObjectURI, target string, opts ...DownloadOption) error

	// Upload reads the file at `source` path and stores it as `target` object in the data store.
	//
	// Parameters:
	//   - source: full file path to upload
	//   - target: object metadata describing where to store the file
	//
	// Returns:
	//   - error: non-nil if the operation fails
	Upload(source string, target ObjectURI) error
}
