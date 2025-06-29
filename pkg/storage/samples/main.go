package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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

// TestConfig holds configuration for testing a storage provider
type TestConfig struct {
	Provider     storage.Provider
	AuthType     string
	URI          string
	TestFilePath string
	Logger       logging.Interface
}

// TestResult holds the result of a test
type TestResult struct {
	Provider  storage.Provider
	AuthType  string
	Operation string
	Success   bool
	Error     error
	Duration  time.Duration
}

func main() {
	var (
		provider  = flag.String("provider", "", "Storage provider: oci, aws, gcp, azure, github")
		authType  = flag.String("auth", "", "Auth type (provider specific)")
		uri       = flag.String("uri", "", "Storage URI for testing")
		testFile  = flag.String("file", "test.txt", "Test file to upload")
		listTests = flag.Bool("list", false, "List all supported provider/auth combinations")
		runAll    = flag.Bool("all", false, "Run tests for all providers")
		verbose   = flag.Bool("verbose", false, "Enable verbose logging")
	)
	flag.Parse()

	logger := createLogger(*verbose)

	if *listTests {
		listSupportedCombinations()
		return
	}

	// Create test file if it doesn't exist
	if *testFile == "" {
		*testFile = createTestFile()
		defer os.Remove(*testFile)
	}

	if *runAll {
		runAllTests(logger, *testFile)
		return
	}

	if *provider == "" || *authType == "" || *uri == "" {
		flag.Usage()
		log.Fatal("Provider, auth type, and URI are required")
	}

	// Run single test
	config := TestConfig{
		Provider:     storage.Provider(*provider),
		AuthType:     *authType,
		URI:          *uri,
		TestFilePath: *testFile,
		Logger:       logger,
	}

	results := runProviderTests(config)
	printResults(results)
}

func createLogger(verbose bool) logging.Interface {
	// Always use NopLogger for now
	// TODO: Add proper logging support
	return logging.NewNopLogger()
}

func createTestFile() string {
	content := []byte("This is a test file for storage provider testing.\n" +
		"Created at: " + time.Now().Format(time.RFC3339) + "\n" +
		"Random data: " + strings.Repeat("test", 100) + "\n")

	tmpFile, err := os.CreateTemp("", "storage-test-*.txt")
	if err != nil {
		log.Fatal("Failed to create test file:", err)
	}

	if _, err := tmpFile.Write(content); err != nil {
		log.Fatal("Failed to write test file:", err)
	}
	tmpFile.Close()

	return tmpFile.Name()
}

func listSupportedCombinations() {
	fmt.Println("Supported Storage Provider / Authentication Combinations:")
	fmt.Println("========================================================")

	fmt.Println("\nOCI Storage:")
	fmt.Println("  - user-principal: OCI config file authentication")
	fmt.Println("  - instance-principal: OCI instance metadata authentication")
	fmt.Println("  - resource-principal: OCI resource principal authentication")

	fmt.Println("\nAWS S3:")
	fmt.Println("  - access-key: AWS access key and secret")
	fmt.Println("  - iam-role: IAM role assumption")
	fmt.Println("  - web-identity: Web identity token")

	fmt.Println("\nGoogle Cloud Storage:")
	fmt.Println("  - service-account: Service account JSON key")
	fmt.Println("  - workload-identity: Workload identity federation")
	fmt.Println("  - application-default: Application default credentials")

	fmt.Println("\nAzure Blob Storage:")
	fmt.Println("  - service-principal: Service principal with secret")
	fmt.Println("  - managed-identity: Managed identity")
	fmt.Println("  - device-flow: Interactive device flow")

	fmt.Println("\nGitHub LFS:")
	fmt.Println("  - personal-access-token: GitHub PAT")
	fmt.Println("  - github-app: GitHub App authentication")
}

func runAllTests(logger logging.Interface, testFile string) {
	// This would run tests for all configured providers
	// In a real scenario, you would read configs from environment or files
	fmt.Println("Running tests for all providers requires configuration...")
	fmt.Println("Please run with specific provider and auth type.")
}

func runProviderTests(config TestConfig) []TestResult {
	ctx := context.Background()
	results := []TestResult{}

	// Parse URI
	uri, err := parseURI(config.URI)
	if err != nil {
		return append(results, TestResult{
			Provider:  config.Provider,
			AuthType:  config.AuthType,
			Operation: "parse-uri",
			Success:   false,
			Error:     err,
		})
	}

	// Create auth factory and credentials
	authFactory := createAuthFactory(config.Logger)
	authConfig := createAuthConfig(config.Provider, config.AuthType)

	// Auth is handled by storage factory internally

	// Create storage factory and instance
	storageFactory := createStorageFactory(authFactory, config.Logger)
	storageConfig := createStorageConfig(config.Provider, authConfig)

	storage, err := storageFactory.Create(ctx, config.Provider, storageConfig)
	if err != nil {
		return append(results, TestResult{
			Provider:  config.Provider,
			AuthType:  config.AuthType,
			Operation: "create-storage",
			Success:   false,
			Error:     err,
		})
	}

	// Run all storage operations
	results = append(results, testUpload(ctx, storage, config, uri)...)
	results = append(results, testDownload(ctx, storage, config, uri)...)
	results = append(results, testGet(ctx, storage, config, uri)...)
	results = append(results, testPut(ctx, storage, config, uri)...)
	results = append(results, testExists(ctx, storage, config, uri)...)
	results = append(results, testGetObjectInfo(ctx, storage, config, uri)...)
	results = append(results, testStat(ctx, storage, config, uri)...)
	results = append(results, testList(ctx, storage, config, uri)...)
	results = append(results, testCopy(ctx, storage, config, uri)...)
	results = append(results, testMultipart(ctx, storage, config, uri)...)
	results = append(results, testBulkOperations(ctx, storage, config, uri)...)
	results = append(results, testValidation(ctx, storage, config, uri)...)
	results = append(results, testProgress(ctx, storage, config, uri)...)
	results = append(results, testDelete(ctx, storage, config, uri)...)

	return results
}

func parseURI(uriStr string) (storage.ObjectURI, error) {
	// Use the unified ParseURI function
	uriPtr, err := storage.ParseURI(uriStr)
	if err != nil {
		return storage.ObjectURI{}, err
	}
	return *uriPtr, nil
}

func createAuthFactory(logger logging.Interface) *auth.DefaultFactory {
	factory := auth.NewDefaultFactory(logger)

	// Register all auth providers
	factory.RegisterProvider(auth.ProviderOCI, authoci.NewFactory(logger))
	factory.RegisterProvider(auth.ProviderAWS, authaws.NewFactory(logger))
	factory.RegisterProvider(auth.ProviderGCP, authgcp.NewFactory(logger))
	factory.RegisterProvider(auth.ProviderAzure, authazure.NewFactory(logger))
	factory.RegisterProvider(auth.ProviderGitHub, authgithub.NewFactory(logger))

	return factory
}

func createStorageFactory(authFactory *auth.DefaultFactory, logger logging.Interface) *storage.DefaultFactory {
	factory := storage.NewDefaultFactory(authFactory, logger)

	// Register all storage providers
	factory.RegisterProvider(storage.ProviderOCI, storageoci.NewFactory(logger))
	factory.RegisterProvider(storage.ProviderAWS, storageaws.NewFactory(logger))
	factory.RegisterProvider(storage.ProviderGCP, storagegcp.NewFactory(logger))
	factory.RegisterProvider(storage.ProviderAzure, storageazure.NewFactory(logger))
	factory.RegisterProvider(storage.ProviderGitHub, storagegithub.NewFactory(logger))

	return factory
}

func createAuthConfig(provider storage.Provider, authType string) auth.Config {
	// Convert string authType to proper auth.AuthType
	var authTypeEnum auth.AuthType
	switch provider {
	case storage.ProviderOCI:
		switch authType {
		case "user-principal":
			authTypeEnum = auth.OCIUserPrincipal
		case "instance-principal":
			authTypeEnum = auth.OCIInstancePrincipal
		case "resource-principal":
			authTypeEnum = auth.OCIResourcePrincipal
		}
	case storage.ProviderAWS:
		switch authType {
		case "access-key":
			authTypeEnum = auth.AWSAccessKey
		case "iam-role":
			authTypeEnum = auth.AWSAssumeRole
		case "web-identity":
			authTypeEnum = auth.AWSWebIdentity
		}
	case storage.ProviderGCP:
		switch authType {
		case "service-account":
			authTypeEnum = auth.GCPServiceAccount
		case "workload-identity":
			authTypeEnum = auth.GCPWorkloadIdentity
		case "application-default":
			authTypeEnum = auth.GCPApplicationDefault
		}
	case storage.ProviderAzure:
		switch authType {
		case "service-principal":
			authTypeEnum = auth.AzureServicePrincipal
		case "managed-identity":
			authTypeEnum = auth.AzureManagedIdentity
		case "device-flow":
			authTypeEnum = auth.AzureDeviceFlow
		}
	case storage.ProviderGitHub:
		switch authType {
		case "personal-access-token":
			authTypeEnum = auth.GitHubPersonalAccessToken
		case "github-app":
			authTypeEnum = auth.GitHubApp
		}
	}

	config := auth.Config{
		Provider: auth.Provider(provider),
		AuthType: authTypeEnum,
	}

	// Add provider-specific configuration from environment
	switch provider {
	case storage.ProviderOCI:
		config.Region = os.Getenv("OCI_REGION")
	case storage.ProviderAWS:
		config.Region = os.Getenv("AWS_REGION")
		if config.Region == "" {
			config.Region = "us-east-1"
		}
	case storage.ProviderGCP:
		// GCP specific config
	case storage.ProviderAzure:
		// Azure specific config
	case storage.ProviderGitHub:
		// GitHub specific config
	}

	return config
}

func createStorageConfig(provider storage.Provider, authConfig auth.Config) interface{} {
	// Create StorageConfig with provider-specific extras
	extra := make(map[string]interface{})

	switch provider {
	case storage.ProviderOCI:
		extra["compartment_id"] = os.Getenv("OCI_COMPARTMENT_ID")
	case storage.ProviderAWS:
		// AWS region is in the auth config
	case storage.ProviderGCP:
		extra["project_id"] = os.Getenv("GCP_PROJECT_ID")
	case storage.ProviderAzure:
		extra["account_name"] = os.Getenv("AZURE_STORAGE_ACCOUNT")
	case storage.ProviderGitHub:
		extra["owner"] = os.Getenv("GITHUB_OWNER")
		extra["repo"] = os.Getenv("GITHUB_REPO")
	}

	return &storage.StorageConfig{
		Provider:   provider,
		Region:     authConfig.Region,
		AuthConfig: authConfig,
		Extra:      extra,
	}
}

// Test operations

func testUpload(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	start := time.Now()
	testObjectName := "test-upload-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	err := s.Upload(ctx, config.TestFilePath, uri,
		storage.WithContentType("text/plain"),
		storage.WithMetadata(map[string]string{
			"test":      "true",
			"timestamp": time.Now().Format(time.RFC3339),
		}),
	)

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "upload",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testDownload(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	start := time.Now()
	testObjectName := "test-upload-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	// First upload a file
	_ = s.Upload(ctx, config.TestFilePath, uri)

	// Then download it
	downloadPath := filepath.Join(os.TempDir(), "download-"+testObjectName)
	defer os.Remove(downloadPath)

	err := s.Download(ctx, uri, downloadPath,
		storage.WithChunkSize(5),
		storage.WithThreads(3),
	)

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "download",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testGet(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	start := time.Now()
	testObjectName := "test-get-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	// First upload a file
	_ = s.Upload(ctx, config.TestFilePath, uri)

	// Then get it
	reader, err := s.Get(ctx, uri)
	if err == nil {
		defer reader.Close()
		// Read the content
		_, err = io.ReadAll(reader)
	}

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "get",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testPut(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	start := time.Now()
	testObjectName := "test-put-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	content := strings.NewReader("Test content for PUT operation")
	err := s.Put(ctx, uri, content, int64(content.Len()),
		storage.WithContentType("text/plain"),
	)

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "put",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testExists(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	start := time.Now()
	testObjectName := "test-exists-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	// Check non-existent
	exists, err := s.Exists(ctx, uri)
	if err != nil || exists {
		return []TestResult{{
			Provider:  config.Provider,
			AuthType:  config.AuthType,
			Operation: "exists",
			Success:   false,
			Error:     fmt.Errorf("expected false for non-existent object"),
			Duration:  time.Since(start),
		}}
	}

	// Upload and check again
	_ = s.Upload(ctx, config.TestFilePath, uri)
	exists, err = s.Exists(ctx, uri)

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "exists",
		Success:   err == nil && exists,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testGetObjectInfo(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	start := time.Now()
	testObjectName := "test-info-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	// Upload first
	_ = s.Upload(ctx, config.TestFilePath, uri)

	info, err := s.GetObjectInfo(ctx, uri)
	if err == nil && info != nil {
		fmt.Printf("Object Info: Name=%s, Size=%d, ETag=%s\n",
			info.Name, info.Size, info.ETag)
	}

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "get-object-info",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testStat(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	start := time.Now()
	testObjectName := "test-stat-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	// Upload first
	_ = s.Upload(ctx, config.TestFilePath, uri)

	metadata, err := s.Stat(ctx, uri)
	if err == nil && metadata != nil {
		fmt.Printf("Metadata: Name=%s, Size=%d, MD5=%s, IsMultipart=%v\n",
			metadata.Name, metadata.Size, metadata.ContentMD5, metadata.IsMultipart)
	}

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "stat",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testList(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	start := time.Now()

	// Upload a few test files
	for i := 0; i < 3; i++ {
		testURI := uri
		testURI.ObjectName = fmt.Sprintf("test-list-%d.txt", i)
		_ = s.Upload(ctx, config.TestFilePath, testURI)
	}

	// List with prefix
	listURI := uri
	listURI.ObjectName = ""
	listURI.Prefix = "test-list-"

	objects, err := s.List(ctx, listURI, storage.ListOptions{
		Prefix:  "test-list-",
		MaxKeys: 10,
	})

	if err == nil {
		fmt.Printf("Listed %d objects\n", len(objects))
	}

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "list",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testCopy(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	start := time.Now()
	sourceURI := uri
	sourceURI.ObjectName = "test-copy-source.txt"
	targetURI := uri
	targetURI.ObjectName = "test-copy-target.txt"

	// Upload source
	_ = s.Upload(ctx, config.TestFilePath, sourceURI)

	// Copy
	err := s.Copy(ctx, sourceURI, targetURI)

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "copy",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testMultipart(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	// Check if storage supports multipart
	multipartStorage, ok := s.(storage.MultipartCapable)
	if !ok {
		return []TestResult{{
			Provider:  config.Provider,
			AuthType:  config.AuthType,
			Operation: "multipart",
			Success:   true,
			Error:     nil,
			Duration:  0,
		}}
	}

	start := time.Now()
	testObjectName := "test-multipart-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	// Create a larger test content
	part1 := []byte(strings.Repeat("Part 1 data ", 1000))
	part2 := []byte(strings.Repeat("Part 2 data ", 1000))

	// Initiate multipart upload
	uploadID, err := multipartStorage.InitiateMultipartUpload(ctx, uri)
	if err != nil {
		return []TestResult{{
			Provider:  config.Provider,
			AuthType:  config.AuthType,
			Operation: "multipart",
			Success:   false,
			Error:     err,
			Duration:  time.Since(start),
		}}
	}

	// Upload parts
	var parts []storage.CompletedPart

	etag1, err := multipartStorage.UploadPart(ctx, uri, uploadID, 1,
		strings.NewReader(string(part1)), int64(len(part1)))
	if err == nil {
		parts = append(parts, storage.CompletedPart{PartNumber: 1, ETag: etag1})
	}

	etag2, err := multipartStorage.UploadPart(ctx, uri, uploadID, 2,
		strings.NewReader(string(part2)), int64(len(part2)))
	if err == nil {
		parts = append(parts, storage.CompletedPart{PartNumber: 2, ETag: etag2})
	}

	// Complete multipart upload
	if len(parts) == 2 {
		err = multipartStorage.CompleteMultipartUpload(ctx, uri, uploadID, parts)
	} else {
		// Abort if parts failed
		_ = multipartStorage.AbortMultipartUpload(ctx, uri, uploadID)
		err = fmt.Errorf("failed to upload all parts")
	}

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "multipart",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testBulkOperations(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	results := []TestResult{}

	// Test bulk download
	bulkStorage, ok := s.(storage.BulkStorage)
	if ok {
		start := time.Now()

		// Upload multiple files first
		var objects []storage.ObjectURI
		for i := 0; i < 3; i++ {
			testURI := uri
			testURI.ObjectName = fmt.Sprintf("bulk-test-%d.txt", i)
			_ = s.Upload(ctx, config.TestFilePath, testURI)
			objects = append(objects, testURI)
		}

		// Bulk download
		downloadDir := filepath.Join(os.TempDir(), "bulk-download")
		os.MkdirAll(downloadDir, 0755)
		defer os.RemoveAll(downloadDir)

		downloadResults, err := bulkStorage.BulkDownload(ctx, objects, downloadDir,
			storage.BulkDownloadOptions{
				Concurrency: 2,
				ProgressCallback: func(completed, total int, current *storage.BulkDownloadResult) {
					fmt.Printf("Progress: %d/%d\n", completed, total)
				},
			})

		success := err == nil
		for _, r := range downloadResults {
			if r.Error != nil {
				success = false
				break
			}
		}

		results = append(results, TestResult{
			Provider:  config.Provider,
			AuthType:  config.AuthType,
			Operation: "bulk-download",
			Success:   success,
			Error:     err,
			Duration:  time.Since(start),
		})
	}

	return results
}

func testValidation(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	validatingStorage, ok := s.(storage.ValidatingStorage)
	if !ok {
		return []TestResult{{
			Provider:  config.Provider,
			AuthType:  config.AuthType,
			Operation: "validation",
			Success:   true,
			Error:     nil,
			Duration:  0,
		}}
	}

	start := time.Now()
	testObjectName := "test-validation-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	// Upload with validation
	content := "Test content for validation"
	reader := strings.NewReader(content)

	err := validatingStorage.PutWithValidation(ctx, uri, reader, int64(len(content)), "",
		storage.WithContentType("text/plain"))

	if err == nil {
		// Validate local file
		downloadPath := filepath.Join(os.TempDir(), testObjectName)
		_ = s.Download(ctx, uri, downloadPath)
		defer os.Remove(downloadPath)

		valid, err := validatingStorage.ValidateLocalFile(ctx, downloadPath, uri)
		if !valid || err != nil {
			err = fmt.Errorf("validation failed: %v", err)
		}
	}

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "validation",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testProgress(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	progressStorage, ok := s.(storage.ProgressStorage)
	if !ok {
		return []TestResult{{
			Provider:  config.Provider,
			AuthType:  config.AuthType,
			Operation: "progress",
			Success:   true,
			Error:     nil,
			Duration:  0,
		}}
	}

	start := time.Now()
	testObjectName := "test-progress-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	// Upload with progress
	progressCalled := false
	err := progressStorage.UploadWithProgress(ctx, config.TestFilePath, uri,
		func(progress storage.Progress) {
			progressCalled = true
			percentage := float64(progress.ProcessedBytes) / float64(progress.TotalBytes) * 100
			fmt.Printf("Upload progress: %.2f%%\n", percentage)
		},
		storage.WithContentType("text/plain"),
	)

	if err == nil && !progressCalled {
		// For small files, progress might not be called
		fmt.Println("Progress callback not called (file too small)")
	}

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "progress",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func testDelete(ctx context.Context, s storage.Storage, config TestConfig, uri storage.ObjectURI) []TestResult {
	start := time.Now()
	testObjectName := "test-delete-" + time.Now().Format("20060102-150405") + ".txt"
	uri.ObjectName = testObjectName

	// Upload first
	_ = s.Upload(ctx, config.TestFilePath, uri)

	// Delete
	err := s.Delete(ctx, uri)

	// Verify deletion
	if err == nil {
		exists, _ := s.Exists(ctx, uri)
		if exists {
			err = fmt.Errorf("object still exists after deletion")
		}
	}

	return []TestResult{{
		Provider:  config.Provider,
		AuthType:  config.AuthType,
		Operation: "delete",
		Success:   err == nil,
		Error:     err,
		Duration:  time.Since(start),
	}}
}

func printResults(results []TestResult) {
	fmt.Println("\nTest Results:")
	fmt.Println("=============")

	var passed, failed int
	for _, r := range results {
		status := "✓ PASS"
		if !r.Success {
			status = "✗ FAIL"
			failed++
		} else {
			passed++
		}

		fmt.Printf("%s %s/%s - %s (%.2fs)",
			status, r.Provider, r.AuthType, r.Operation, r.Duration.Seconds())

		if r.Error != nil {
			fmt.Printf(" - Error: %v", r.Error)
		}
		fmt.Println()
	}

	fmt.Printf("\nSummary: %d passed, %d failed\n", passed, failed)
}
