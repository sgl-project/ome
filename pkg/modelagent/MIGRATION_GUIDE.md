# ModelAgent Multi-Cloud Storage Migration Guide

## Overview

This guide describes how to migrate the modelagent package from using OCI-specific storage implementations to the new multi-cloud storage abstractions.

## Key Changes

### 1. Storage Client Creation

**Before:**
```go
// Direct OCI client creation
ociOSDataStore, err := s.createOCIOSDataStore(baseModelSpec)
```

**After:**
```go
// Provider-agnostic storage client creation
storageClient, provider, objectURI, err := s.createStorageClient(ctx, baseModelSpec)
```

### 2. URI Format Support

The new implementation supports multiple cloud provider URI formats:

- **OCI**: `oci://n/{namespace}/b/{bucket}/o/{prefix}` or `oci://{namespace}@{region}/{bucket}/{prefix}`
- **AWS**: `s3://{bucket}/{prefix}` or `aws://{region}/{bucket}/{prefix}`
- **GCP**: `gs://{bucket}/{prefix}` or `gcp://{project}/{bucket}/{prefix}`
- **Azure**: `az://{container}/{prefix}` or `azure://{account}/{container}/{prefix}`

### 3. Authentication Configuration

Authentication is now handled through a unified auth factory pattern:

```go
// Storage parameters in BaseModelSpec
parameters:
  auth: "instance_principal"  # Auth type
  region: "us-ashburn-1"      # Provider-specific config
  compartment_id: "ocid..."   # OCI-specific
```

Supported auth types by provider:
- **OCI**: `instance_principal`, `user_principal`, `resource_principal`
- **AWS**: `iam_role`, `access_key`, `assume_role` 
- **GCP**: `service_account`, `workload_identity`, `application_default`
- **Azure**: `managed_identity`, `service_principal`, `cli`

### 4. Download Operations

**Before:**
```go
errs := ociOSDataStore.BulkDownload(objectUris, destPath, s.concurrency,
    ociobjectstore.WithThreads(s.multipartConcurrency),
    ociobjectstore.WithChunkSize(BigFileSizeInMB),
    // ...
)
```

**After:**
```go
// Automatic detection of bulk support
bulkStorage, supportsBulk := storageClient.(storage.BulkStorage)
if supportsBulk {
    results, err := bulkStorage.BulkDownload(ctx, downloadURIs, destPath, bulkOpts)
} else {
    // Fall back to sequential downloads
    err := storageClient.Download(ctx, objURI, targetPath, downloadOpts...)
}
```

### 5. Object Listing

**Before:**
```go
objects, err := ociOSDataStore.ListObjects(*uri)
```

**After:**
```go
listOpts := storage.ListOptions{
    Prefix: uri.Prefix,
}
objects, err := storageClient.List(ctx, *uri, listOpts)
```

## Migration Steps

### Step 1: Update Imports

Remove OCI-specific imports:
```go
// Remove these
import (
    "github.com/oracle/oci-go-sdk/v65/objectstorage"
    "github.com/sgl-project/ome/pkg/ociobjectstore"
    "github.com/sgl-project/ome/pkg/principals"
)
```

Add multi-cloud imports:
```go
// Add these
import (
    "github.com/sgl-project/ome/pkg/auth"
    "github.com/sgl-project/ome/pkg/storage"
    storageoci "github.com/sgl-project/ome/pkg/storage/oci"
    // Add other providers as needed
)
```

### Step 2: Initialize Storage Factory

In the Gopher constructor:
```go
// Initialize storage factory with all providers
storageFactory, err := initializeStorageFactory(logging.ForZap(logger.Desugar()))
if err != nil {
    return nil, fmt.Errorf("failed to initialize storage factory: %w", err)
}
```

### Step 3: Update Download Logic

Replace the `downloadModel` method to use the new abstraction:
```go
func (s *Gopher) downloadModel(uri *storage.ObjectURI, destPath string, task *GopherTask) error {
    ctx := context.Background()
    
    // Create storage client
    storageClient, provider, objectURI, err := s.createStorageClient(ctx, baseModelSpec)
    if err != nil {
        return err
    }
    
    // Use new download method
    return s.downloadModelV2(ctx, storageClient, objectURI, destPath, task)
}
```

### Step 4: Update Configuration

Update model configurations to specify provider if not using OCI:

```yaml
apiVersion: ome.io/v1beta1
kind: BaseModel
metadata:
  name: my-model
spec:
  storage:
    storageUri: "s3://my-bucket/models/my-model"  # AWS S3
    parameters:
      auth: "iam_role"
      region: "us-west-2"
```

### Step 5: Test with Different Providers

Test model downloads with different cloud providers:

1. **OCI** (existing models should work unchanged)
2. **AWS S3** with IAM roles
3. **GCP Cloud Storage** with service accounts
4. **Azure Blob Storage** with managed identities

## Backward Compatibility

The migration maintains full backward compatibility:

1. Existing OCI URIs continue to work without changes
2. Default auth types are set appropriately for each provider
3. Storage parameters are mapped to provider-specific configurations
4. Error messages are preserved for debugging

## Benefits

1. **Multi-Cloud Support**: Deploy models from any major cloud provider
2. **Unified Interface**: Consistent API across all providers
3. **Better Testing**: Provider-agnostic interfaces enable better unit testing
4. **Flexibility**: Easy to add new storage providers
5. **Reduced Vendor Lock-in**: Switch between providers without code changes

## Troubleshooting

### Common Issues

1. **Authentication Failures**
   - Verify auth type is supported for the provider
   - Check credentials/roles are properly configured
   - Ensure necessary IAM permissions are granted

2. **URI Parsing Errors**
   - Validate URI format matches provider expectations
   - Check for typos in bucket/container names
   - Ensure proper URL encoding for special characters

3. **Network/Connectivity**
   - Verify network access to storage endpoints
   - Check firewall rules and security groups
   - Validate proxy settings if applicable

### Debug Logging

Enable debug logging to troubleshoot issues:
```go
logger.WithField("provider", provider).
       WithField("uri", objectURI).
       Debug("Creating storage client")
```

## Future Enhancements

1. **Additional Providers**: Support for MinIO, DigitalOcean Spaces, etc.
2. **Cross-Region Replication**: Automatic failover between regions
3. **Caching Layer**: Local caching for frequently accessed models
4. **Metrics Integration**: Provider-specific metrics and monitoring
5. **Cost Optimization**: Smart routing based on egress costs