package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/sgl-project/ome/pkg/auth"
	authoci "github.com/sgl-project/ome/pkg/auth/oci"
	"github.com/sgl-project/ome/pkg/logging"
	"github.com/sgl-project/ome/pkg/storage"
	storageoci "github.com/sgl-project/ome/pkg/storage/oci"
)

// SimpleExample demonstrates basic usage of storage package with OCI
func main() {
	ctx := context.Background()
	logger := logging.NewNopLogger()

	// Step 1: Create auth factory and register OCI provider
	authFactory := auth.NewDefaultFactory(logger)
	authFactory.RegisterProvider(auth.ProviderOCI, authoci.NewFactory(logger))

	// Step 2: Create OCI credentials
	authConfig := auth.Config{
		Provider: auth.ProviderOCI,
		AuthType: auth.OCIUserPrincipal,
		Region:   "us-ashburn-1",
	}

	// Step 3: Create storage factory and register OCI provider
	storageFactory := storage.NewDefaultFactory(authFactory, logger)
	storageFactory.RegisterProvider(storage.ProviderOCI, storageoci.NewFactory(logger))

	// Step 4: Create storage config with embedded auth config
	storageConfig := storage.StorageConfig{
		Provider:   storage.ProviderOCI,
		Region:     "us-ashburn-1",
		AuthConfig: authConfig,
		Extra: map[string]interface{}{
			"compartment_id": os.Getenv("OCI_COMPARTMENT_ID"),
		},
	}

	ociStorage, err := storageFactory.Create(ctx, storage.ProviderOCI, &storageConfig)
	if err != nil {
		log.Fatal("Failed to create storage:", err)
	}

	// Step 5: Parse storage URI
	uriPtr, err := storage.ParseURI("oci://my-namespace@us-ashburn-1/my-bucket/test-object.txt")
	if err != nil {
		log.Fatal("Failed to parse URI:", err)
	}
	uri := *uriPtr

	// Example 1: Upload content using Put
	fmt.Println("Example 1: Uploading content...")
	content := "Hello from OCI Storage!"
	err = ociStorage.Put(ctx, uri, strings.NewReader(content), int64(len(content)),
		storage.WithContentType("text/plain"),
		storage.WithMetadata(map[string]string{
			"created-by": "simple-example",
		}),
	)
	if err != nil {
		log.Fatal("Failed to upload:", err)
	}
	fmt.Println("✓ Upload successful")

	// Example 2: Check if object exists
	fmt.Println("\nExample 2: Checking if object exists...")
	exists, err := ociStorage.Exists(ctx, uri)
	if err != nil {
		log.Fatal("Failed to check existence:", err)
	}
	fmt.Printf("✓ Object exists: %v\n", exists)

	// Example 3: Get object metadata
	fmt.Println("\nExample 3: Getting object metadata...")
	metadata, err := ociStorage.Stat(ctx, uri)
	if err != nil {
		log.Fatal("Failed to get metadata:", err)
	}
	fmt.Printf("✓ Object metadata:\n")
	fmt.Printf("  - Name: %s\n", metadata.Name)
	fmt.Printf("  - Size: %d bytes\n", metadata.Size)
	fmt.Printf("  - ContentMD5: %s\n", metadata.ContentMD5)
	fmt.Printf("  - ContentType: %s\n", metadata.ContentType)

	// Example 4: Download content using Get
	fmt.Println("\nExample 4: Downloading content...")
	reader, err := ociStorage.Get(ctx, uri)
	if err != nil {
		log.Fatal("Failed to download:", err)
	}
	defer reader.Close()

	downloadedContent, err := io.ReadAll(reader)
	if err != nil {
		log.Fatal("Failed to read content:", err)
	}
	fmt.Printf("✓ Downloaded content: %s\n", string(downloadedContent))

	// Example 5: List objects with prefix
	fmt.Println("\nExample 5: Listing objects...")
	listURI := uri
	listURI.ObjectName = ""
	listURI.Prefix = "test-"

	objects, err := ociStorage.List(ctx, listURI, storage.ListOptions{
		Prefix:  "test-",
		MaxKeys: 10,
	})
	if err != nil {
		log.Fatal("Failed to list objects:", err)
	}
	fmt.Printf("✓ Found %d objects:\n", len(objects))
	for _, obj := range objects {
		fmt.Printf("  - %s (size: %d)\n", obj.Name, obj.Size)
	}

	// Example 6: Copy object
	fmt.Println("\nExample 6: Copying object...")
	copyURI := uri
	copyURI.ObjectName = "test-object-copy.txt"

	err = ociStorage.Copy(ctx, uri, copyURI)
	if err != nil {
		log.Fatal("Failed to copy object:", err)
	}
	fmt.Println("✓ Object copied successfully")

	// Example 7: Multipart upload (for larger files)
	fmt.Println("\nExample 7: Testing multipart capabilities...")
	if multipartStorage, ok := ociStorage.(storage.MultipartCapable); ok {
		multipartURI := uri
		multipartURI.ObjectName = "test-multipart.txt"

		// Initiate multipart upload
		uploadID, err := multipartStorage.InitiateMultipartUpload(ctx, multipartURI,
			storage.WithContentType("text/plain"),
		)
		if err != nil {
			log.Fatal("Failed to initiate multipart upload:", err)
		}

		// Upload parts
		part1Content := strings.Repeat("Part 1 ", 100)
		etag1, err := multipartStorage.UploadPart(ctx, multipartURI, uploadID, 1,
			strings.NewReader(part1Content), int64(len(part1Content)))
		if err != nil {
			log.Fatal("Failed to upload part 1:", err)
		}

		part2Content := strings.Repeat("Part 2 ", 100)
		etag2, err := multipartStorage.UploadPart(ctx, multipartURI, uploadID, 2,
			strings.NewReader(part2Content), int64(len(part2Content)))
		if err != nil {
			log.Fatal("Failed to upload part 2:", err)
		}

		// Complete multipart upload
		parts := []storage.CompletedPart{
			{PartNumber: 1, ETag: etag1},
			{PartNumber: 2, ETag: etag2},
		}
		err = multipartStorage.CompleteMultipartUpload(ctx, multipartURI, uploadID, parts)
		if err != nil {
			log.Fatal("Failed to complete multipart upload:", err)
		}
		fmt.Println("✓ Multipart upload completed successfully")
	}

	// Example 8: Bulk operations
	fmt.Println("\nExample 8: Testing bulk operations...")
	if bulkStorage, ok := ociStorage.(storage.BulkStorage); ok {
		// Upload multiple objects for bulk download test
		var objectsToDownload []storage.ObjectURI
		for i := 0; i < 3; i++ {
			bulkURI := uri
			bulkURI.ObjectName = fmt.Sprintf("bulk-test-%d.txt", i)
			content := fmt.Sprintf("Bulk content %d", i)
			_ = ociStorage.Put(ctx, bulkURI, strings.NewReader(content), int64(len(content)))
			objectsToDownload = append(objectsToDownload, bulkURI)
		}

		// Bulk download
		tempDir := os.TempDir()
		results, err := bulkStorage.BulkDownload(ctx, objectsToDownload, tempDir,
			storage.BulkDownloadOptions{
				Concurrency: 2,
				ProgressCallback: func(completed, total int, current *storage.BulkDownloadResult) {
					fmt.Printf("  Progress: %d/%d completed\n", completed, total)
				},
			},
		)
		if err != nil {
			log.Printf("Bulk download error: %v", err)
		}

		successCount := 0
		for _, result := range results {
			if result.Error == nil {
				successCount++
			}
		}
		fmt.Printf("✓ Bulk download completed: %d/%d successful\n", successCount, len(results))
	}

	// Example 9: Delete objects (cleanup)
	fmt.Println("\nExample 9: Cleaning up...")
	err = ociStorage.Delete(ctx, uri)
	if err != nil {
		log.Printf("Failed to delete object: %v", err)
	} else {
		fmt.Println("✓ Original object deleted")
	}

	// Delete the copy
	copyURI = uri
	copyURI.ObjectName = "test-object-copy.txt"
	_ = ociStorage.Delete(ctx, copyURI)

	fmt.Println("\n✅ All examples completed successfully!")
}
