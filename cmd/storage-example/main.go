package main

import (
	"context"
	"fmt"
	"log"

	"github.com/sgl-project/ome/pkg/auth"
	authaws "github.com/sgl-project/ome/pkg/auth/aws"
	authazure "github.com/sgl-project/ome/pkg/auth/azure"
	authgcp "github.com/sgl-project/ome/pkg/auth/gcp"
	authgithub "github.com/sgl-project/ome/pkg/auth/github"
	authoci "github.com/sgl-project/ome/pkg/auth/oci"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
	storageaws "github.com/sgl-project/ome/pkg/storage/aws"
	storageazure "github.com/sgl-project/ome/pkg/storage/azure"
	storagegcp "github.com/sgl-project/ome/pkg/storage/gcp"
	storagegithub "github.com/sgl-project/ome/pkg/storage/github"
	storageoci "github.com/sgl-project/ome/pkg/storage/oci"
)

func main() {
	// Create logger
	logger := logging.NewNopLogger()

	// Create and initialize auth factory
	authFactory := auth.NewDefaultFactory(logger)
	authFactory.RegisterProvider(auth.ProviderOCI, authoci.NewFactory(logger))
	authFactory.RegisterProvider(auth.ProviderAWS, authaws.NewFactory(logger))
	authFactory.RegisterProvider(auth.ProviderGCP, authgcp.NewFactory(logger))
	authFactory.RegisterProvider(auth.ProviderAzure, authazure.NewFactory(logger))
	authFactory.RegisterProvider(auth.ProviderGitHub, authgithub.NewFactory(logger))

	// Create and initialize storage factory
	storageFactory := storage.NewDefaultFactory(authFactory, logger)
	storageFactory.RegisterProvider(storage.ProviderOCI, storageoci.NewFactory(logger))
	storageFactory.RegisterProvider(storage.ProviderAWS, storageaws.NewFactory(logger))
	storageFactory.RegisterProvider(storage.ProviderGCP, storagegcp.NewFactory(logger))
	storageFactory.RegisterProvider(storage.ProviderAzure, storageazure.NewFactory(logger))
	storageFactory.RegisterProvider(storage.ProviderGitHub, storagegithub.NewFactory(logger))

	// Example: Parse various storage URIs
	fmt.Println("=== Storage URI Parsing Examples ===")
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

		fmt.Printf("\nURI: %s\n", uri)
		fmt.Printf("  Provider: %s\n", parsed.Provider)
		fmt.Printf("  Bucket: %s\n", parsed.BucketName)
		if parsed.ObjectName != "" {
			fmt.Printf("  Object: %s\n", parsed.ObjectName)
		}
		if parsed.Prefix != "" {
			fmt.Printf("  Prefix: %s\n", parsed.Prefix)
		}
	}

	// Example: Create OCI storage instance
	fmt.Println("\n=== OCI Storage Example ===")
	config := storage.StorageConfig{
		Provider: storage.ProviderOCI,
		Region:   "us-ashburn-1",
		AuthConfig: auth.Config{
			Provider: auth.ProviderOCI,
			AuthType: auth.OCIInstancePrincipal,
			// In production, you might have a fallback:
			Fallback: &auth.Config{
				Provider: auth.ProviderOCI,
				AuthType: auth.OCIUserPrincipal,
				Extra: map[string]interface{}{
					"user_principal": map[string]interface{}{
						"config_path": "~/.oci/config",
						"profile":     "DEFAULT",
					},
				},
			},
		},
		Extra: map[string]interface{}{
			"compartment_id": "ocid1.compartment.oc1..example",
		},
	}

	ctx := context.Background()

	// Note: This will fail without actual OCI credentials, but shows the pattern
	_, err := storageFactory.Create(ctx, config.Provider, &config)
	if err != nil {
		fmt.Printf("Expected error (no real credentials): %v\n", err)
	} else {
		fmt.Println("Storage instance created successfully!")
	}

	// Example: Download options
	fmt.Println("\n=== Download Options Example ===")
	downloadOpts := []storage.DownloadOption{
		storage.WithChunkSize(50),        // 50MB chunks
		storage.WithThreads(20),          // 20 parallel threads
		storage.WithForceMultipart(true), // Force multipart
	}

	fmt.Println("Download options configured:")
	opts := storage.DefaultDownloadOptions()
	for _, opt := range downloadOpts {
		opt(&opts)
	}
	fmt.Printf("  Chunk Size: %dMB\n", opts.ChunkSizeInMB)
	fmt.Printf("  Threads: %d\n", opts.Threads)
	fmt.Printf("  Force Multipart: %v\n", opts.ForceMultipart)
}
