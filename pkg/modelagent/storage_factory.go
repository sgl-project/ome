package modelagent

import (
	"fmt"

	"github.com/sgl-project/ome/pkg/auth"
	authaws "github.com/sgl-project/ome/pkg/auth/aws"
	authazure "github.com/sgl-project/ome/pkg/auth/azure"
	authgcp "github.com/sgl-project/ome/pkg/auth/gcp"
	authoci "github.com/sgl-project/ome/pkg/auth/oci"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
	storageaws "github.com/sgl-project/ome/pkg/storage/aws"
	storageazure "github.com/sgl-project/ome/pkg/storage/azure"
	storagegcp "github.com/sgl-project/ome/pkg/storage/gcp"
	storageoci "github.com/sgl-project/ome/pkg/storage/oci"
)

// initializeStorageFactory creates and configures the storage factory with all providers
func initializeStorageFactory(logger logging.Interface) (storage.StorageFactory, error) {
	// Create auth factory
	authFactory := auth.NewDefaultFactory(logger)

	// Register auth providers
	authFactory.RegisterProvider(auth.ProviderOCI, authoci.NewFactory(logger))
	authFactory.RegisterProvider(auth.ProviderAWS, authaws.NewFactory(logger))
	authFactory.RegisterProvider(auth.ProviderGCP, authgcp.NewFactory(logger))
	authFactory.RegisterProvider(auth.ProviderAzure, authazure.NewFactory(logger))

	// Create storage factory
	storageFactory := storage.NewDefaultFactory(authFactory, logger)

	// Register storage providers
	storageFactory.RegisterProvider(storage.ProviderOCI, storageoci.NewFactory(logger))
	storageFactory.RegisterProvider(storage.ProviderAWS, storageaws.NewFactory(logger))
	storageFactory.RegisterProvider(storage.ProviderGCP, storagegcp.NewFactory(logger))
	storageFactory.RegisterProvider(storage.ProviderAzure, storageazure.NewFactory(logger))

	return storageFactory, nil
}

// parseStorageURI parses a storage URI and returns provider and object URI
func parseStorageURI(uri string) (storage.Provider, *storage.ObjectURI, error) {
	// Check for different provider prefixes
	switch {
	case hasPrefix(uri, "oci://"):
		return parseOCIURI(uri)
	case hasPrefix(uri, "s3://") || hasPrefix(uri, "aws://"):
		return parseAWSURI(uri)
	case hasPrefix(uri, "gs://") || hasPrefix(uri, "gcp://"):
		return parseGCPURI(uri)
	case hasPrefix(uri, "az://") || hasPrefix(uri, "azure://"):
		return parseAzureURI(uri)
	default:
		return "", nil, fmt.Errorf("unsupported storage URI format: %s", uri)
	}
}

// hasPrefix checks if a string has a prefix (case-insensitive)
func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// parseOCIURI parses an OCI storage URI
// Formats supported:
// - oci://n/{namespace}/b/{bucket}/o/{prefix}
// - oci://{namespace}@{region}/{bucket}/{prefix}
func parseOCIURI(uri string) (storage.Provider, *storage.ObjectURI, error) {
	// Remove prefix
	uri = uri[6:] // Remove "oci://"

	// Check for the explicit format
	if hasPrefix(uri, "n/") {
		// Format: n/{namespace}/b/{bucket}/o/{prefix}
		parts := splitPath(uri, 6)
		if len(parts) < 5 || parts[0] != "n" || parts[2] != "b" || parts[4] != "o" {
			return "", nil, fmt.Errorf("invalid OCI URI format: expected oci://n/{namespace}/b/{bucket}/o/{prefix}")
		}

		namespace := parts[1]
		bucket := parts[3]
		prefix := ""
		if len(parts) > 5 {
			prefix = joinPath(parts[5:])
		}

		return storage.ProviderOCI, &storage.ObjectURI{
			Provider:   storage.ProviderOCI,
			Namespace:  namespace,
			BucketName: bucket,
			Prefix:     prefix,
		}, nil
	}

	// Check for namespace@region format
	if idx := findChar(uri, '@'); idx >= 0 {
		namespace := uri[:idx]
		remaining := uri[idx+1:]

		// Split region and path
		if idx2 := findChar(remaining, '/'); idx2 >= 0 {
			region := remaining[:idx2]
			path := remaining[idx2+1:]

			// Split bucket and prefix
			if idx3 := findChar(path, '/'); idx3 >= 0 {
				bucket := path[:idx3]
				prefix := path[idx3+1:]

				return storage.ProviderOCI, &storage.ObjectURI{
					Provider:   storage.ProviderOCI,
					Namespace:  namespace,
					BucketName: bucket,
					Prefix:     prefix,
					Region:     region,
				}, nil
			}

			// Just bucket, no prefix
			return storage.ProviderOCI, &storage.ObjectURI{
				Provider:   storage.ProviderOCI,
				Namespace:  namespace,
				BucketName: path,
				Region:     region,
			}, nil
		}
	}

	// Simple format: bucket/prefix
	if idx := findChar(uri, '/'); idx >= 0 {
		bucket := uri[:idx]
		prefix := uri[idx+1:]

		return storage.ProviderOCI, &storage.ObjectURI{
			Provider:   storage.ProviderOCI,
			BucketName: bucket,
			Prefix:     prefix,
		}, nil
	}

	// Just bucket
	return storage.ProviderOCI, &storage.ObjectURI{
		Provider:   storage.ProviderOCI,
		BucketName: uri,
	}, nil
}

// parseAWSURI parses an AWS S3 URI
// Formats supported:
// - s3://{bucket}/{prefix}
// - aws://{region}/{bucket}/{prefix}
func parseAWSURI(uri string) (storage.Provider, *storage.ObjectURI, error) {
	// Remove prefix
	if hasPrefix(uri, "s3://") {
		uri = uri[5:]
	} else if hasPrefix(uri, "aws://") {
		uri = uri[6:]
	}

	// For aws:// format, first part might be region
	if hasPrefix(uri, "aws://") {
		if idx := findChar(uri, '/'); idx >= 0 {
			region := uri[:idx]
			remaining := uri[idx+1:]

			if idx2 := findChar(remaining, '/'); idx2 >= 0 {
				bucket := remaining[:idx2]
				prefix := remaining[idx2+1:]

				return storage.ProviderAWS, &storage.ObjectURI{
					Provider:   storage.ProviderAWS,
					BucketName: bucket,
					Prefix:     prefix,
					Region:     region,
				}, nil
			}

			// Just bucket
			return storage.ProviderAWS, &storage.ObjectURI{
				Provider:   storage.ProviderAWS,
				BucketName: remaining,
				Region:     region,
			}, nil
		}
	}

	// Standard s3:// format
	if idx := findChar(uri, '/'); idx >= 0 {
		bucket := uri[:idx]
		prefix := uri[idx+1:]

		return storage.ProviderAWS, &storage.ObjectURI{
			Provider:   storage.ProviderAWS,
			BucketName: bucket,
			Prefix:     prefix,
		}, nil
	}

	// Just bucket
	return storage.ProviderAWS, &storage.ObjectURI{
		Provider:   storage.ProviderAWS,
		BucketName: uri,
	}, nil
}

// parseGCPURI parses a GCP Cloud Storage URI
// Formats supported:
// - gs://{bucket}/{prefix}
// - gcp://{project}/{bucket}/{prefix}
func parseGCPURI(uri string) (storage.Provider, *storage.ObjectURI, error) {
	// Remove prefix
	if hasPrefix(uri, "gs://") {
		uri = uri[5:]
	} else if hasPrefix(uri, "gcp://") {
		uri = uri[6:]

		// For gcp:// format, first part is project
		if idx := findChar(uri, '/'); idx >= 0 {
			project := uri[:idx]
			remaining := uri[idx+1:]

			if idx2 := findChar(remaining, '/'); idx2 >= 0 {
				bucket := remaining[:idx2]
				prefix := remaining[idx2+1:]

				return storage.ProviderGCP, &storage.ObjectURI{
					Provider:   storage.ProviderGCP,
					BucketName: bucket,
					Prefix:     prefix,
					Extra: map[string]interface{}{
						"project": project,
					},
				}, nil
			}

			// Just bucket
			return storage.ProviderGCP, &storage.ObjectURI{
				Provider:   storage.ProviderGCP,
				BucketName: remaining,
				Extra: map[string]interface{}{
					"project": project,
				},
			}, nil
		}
	}

	// Standard gs:// format
	if idx := findChar(uri, '/'); idx >= 0 {
		bucket := uri[:idx]
		prefix := uri[idx+1:]

		return storage.ProviderGCP, &storage.ObjectURI{
			Provider:   storage.ProviderGCP,
			BucketName: bucket,
			Prefix:     prefix,
		}, nil
	}

	// Just bucket
	return storage.ProviderGCP, &storage.ObjectURI{
		Provider:   storage.ProviderGCP,
		BucketName: uri,
	}, nil
}

// parseAzureURI parses an Azure Blob Storage URI
// Formats supported:
// - az://{container}/{prefix}
// - azure://{account}/{container}/{prefix}
func parseAzureURI(uri string) (storage.Provider, *storage.ObjectURI, error) {
	// Remove prefix
	if hasPrefix(uri, "az://") {
		uri = uri[5:]
	} else if hasPrefix(uri, "azure://") {
		uri = uri[8:]

		// For azure:// format, first part is account
		if idx := findChar(uri, '/'); idx >= 0 {
			account := uri[:idx]
			remaining := uri[idx+1:]

			if idx2 := findChar(remaining, '/'); idx2 >= 0 {
				container := remaining[:idx2]
				prefix := remaining[idx2+1:]

				return storage.ProviderAzure, &storage.ObjectURI{
					Provider:   storage.ProviderAzure,
					BucketName: container,
					Prefix:     prefix,
					Extra: map[string]interface{}{
						"account": account,
					},
				}, nil
			}

			// Just container
			return storage.ProviderAzure, &storage.ObjectURI{
				Provider:   storage.ProviderAzure,
				BucketName: remaining,
				Extra: map[string]interface{}{
					"account": account,
				},
			}, nil
		}
	}

	// Standard az:// format
	if idx := findChar(uri, '/'); idx >= 0 {
		container := uri[:idx]
		prefix := uri[idx+1:]

		return storage.ProviderAzure, &storage.ObjectURI{
			Provider:   storage.ProviderAzure,
			BucketName: container,
			Prefix:     prefix,
		}, nil
	}

	// Just container
	return storage.ProviderAzure, &storage.ObjectURI{
		Provider:   storage.ProviderAzure,
		BucketName: uri,
	}, nil
}

// Helper functions for string manipulation (to avoid importing strings package in examples)
func findChar(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func splitPath(s string, max int) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s) && len(parts) < max-1; i++ {
		if s[i] == '/' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func joinPath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += "/" + parts[i]
	}
	return result
}
