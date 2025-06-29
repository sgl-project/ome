package storage_test

import (
	"context"
	"fmt"
	"log"

	"github.com/sgl-project/ome/pkg/auth"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
)

func ExampleStorageFactory() {
	// Create a logger
	logger := logging.NewNopLogger()

	// Create auth factory
	authFactory := auth.NewDefaultFactory(logger)

	// Create storage factory
	storageFactory := storage.NewDefaultFactory(authFactory, logger)

	// Configure storage for OCI
	config := storage.StorageConfig{
		Provider: storage.ProviderOCI,
		Region:   "us-ashburn-1",
		AuthConfig: auth.Config{
			Provider: auth.ProviderOCI,
			AuthType: auth.OCIInstancePrincipal,
		},
		Extra: map[string]interface{}{
			"compartment_id": "ocid1.compartment.oc1..example",
		},
	}

	// Create storage instance
	ctx := context.Background()
	store, err := storageFactory.Create(ctx, config.Provider, &config)
	if err != nil {
		log.Fatalf("Failed to create storage: %v", err)
	}

	// Use the storage instance
	fmt.Printf("Storage provider: %s\n", store.Provider())
}

func ExampleParseURI() {
	// Parse different storage URIs
	uris := []string{
		"oci://namespace@us-ashburn-1/bucket/prefix/object.txt",
		"s3://my-bucket/path/to/object.txt",
		"gs://gcs-bucket/data/file.json",
		"azure://container@storageaccount/blob/path/file.bin",
		"github://owner/repo@branch/src/main.go",
	}

	for _, uri := range uris {
		parsed, err := storage.ParseURI(uri)
		if err != nil {
			log.Printf("Failed to parse %s: %v", uri, err)
			continue
		}

		fmt.Printf("URI: %s\n", uri)
		fmt.Printf("  Provider: %s\n", parsed.Provider)
		fmt.Printf("  Bucket: %s\n", parsed.BucketName)
		fmt.Printf("  Object: %s\n", parsed.ObjectName)
		fmt.Printf("  Prefix: %s\n", parsed.Prefix)
		fmt.Println()
	}
}

func ExampleStorage_Download() {
	// This example shows how to download an object with custom options
	ctx := context.Background()

	// Assume we have a storage instance
	var store storage.Storage

	// Parse source URI
	source, _ := storage.ParseURI("oci://namespace/bucket/large-file.zip")

	// Download with custom options
	err := store.Download(ctx, *source, "/local/path/large-file.zip",
		storage.WithChunkSize(50),        // 50MB chunks
		storage.WithThreads(20),          // 20 parallel threads
		storage.WithForceMultipart(true), // Force multipart download
	)

	if err != nil {
		log.Fatalf("Download failed: %v", err)
	}
}

func ExampleStorage_Upload() {
	// This example shows how to upload a file with custom options
	ctx := context.Background()

	// Assume we have a storage instance
	var store storage.Storage

	// Parse target URI
	target, _ := storage.ParseURI("s3://my-bucket/uploads/data.json")

	// Upload with custom options
	err := store.Upload(ctx, "/local/data.json", *target,
		storage.WithContentType("application/json"),
		storage.WithStorageClass("STANDARD_IA"),
		storage.WithMetadata(map[string]string{
			"uploaded-by": "example-app",
			"version":     "1.0",
		}),
	)

	if err != nil {
		log.Fatalf("Upload failed: %v", err)
	}
}

func ExampleChainProvider() {
	// This example shows how to use chain authentication
	ctx := context.Background()

	// Create multiple credential providers
	providers := []auth.CredentialsProvider{
		// Try environment variables first
		&envCredentialsProvider{},
		// Fall back to instance profile
		&instanceCredentialsProvider{},
		// Finally try config file
		&fileCredentialsProvider{path: "~/.cloud/config"},
	}

	// Create chain provider
	chain := &auth.ChainProvider{
		Providers: providers,
	}

	// Get credentials - will try each provider in order
	creds, err := chain.GetCredentials(ctx)
	if err != nil {
		log.Fatalf("Failed to get credentials: %v", err)
	}

	fmt.Printf("Got credentials from provider: %s\n", creds.Provider())
}

// Mock credential providers for example
type envCredentialsProvider struct{}

func (e *envCredentialsProvider) GetCredentials(ctx context.Context) (auth.Credentials, error) {
	// Check environment variables
	return nil, fmt.Errorf("no credentials in environment")
}

type instanceCredentialsProvider struct{}

func (i *instanceCredentialsProvider) GetCredentials(ctx context.Context) (auth.Credentials, error) {
	// Try to get instance metadata
	return nil, fmt.Errorf("not running on cloud instance")
}

type fileCredentialsProvider struct {
	path string
}

func (f *fileCredentialsProvider) GetCredentials(ctx context.Context) (auth.Credentials, error) {
	// Read credentials from file
	return nil, fmt.Errorf("config file not found: %s", f.path)
}
