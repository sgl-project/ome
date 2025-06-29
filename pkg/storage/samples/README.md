# Storage Provider Testing Sample

This sample demonstrates how to test all storage providers with various authentication methods.

## Prerequisites

Before running the tests, ensure you have:

1. Go 1.19 or later installed
2. Access to the cloud providers you want to test
3. Proper credentials configured

## Configuration

### Environment Variables

Set the following environment variables based on the providers you want to test:

#### OCI (Oracle Cloud Infrastructure)
```bash
export OCI_CONFIG_FILE=~/.oci/config
export OCI_PROFILE=DEFAULT
export OCI_REGION=us-ashburn-1
export OCI_COMPARTMENT_ID=ocid1.compartment.oc1..xxxxx
```

#### AWS S3
```bash
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key
export AWS_REGION=us-east-1
export AWS_SESSION_TOKEN=your-session-token  # For temporary credentials
```

#### Google Cloud Storage
```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account-key.json
export GCP_PROJECT_ID=your-project-id
```

#### Azure Blob Storage
```bash
export AZURE_TENANT_ID=your-tenant-id
export AZURE_CLIENT_ID=your-client-id
export AZURE_CLIENT_SECRET=your-client-secret
export AZURE_STORAGE_ACCOUNT=your-storage-account
```

#### GitHub LFS
```bash
export GITHUB_TOKEN=your-personal-access-token
export GITHUB_OWNER=your-github-username
export GITHUB_REPO=your-repo-name
```

## Building

First, ensure you're in the samples directory and run:

```bash
go mod tidy
```

Then build the examples:

```bash
go build simple_example.go
go build main.go
```

## Usage

### List Supported Combinations

```bash
go run main.go -list
```

### Test Specific Provider

```bash
# OCI with User Principal
go run main.go \
  -provider oci \
  -auth user-principal \
  -uri "oci://namespace@region/bucket/prefix" \
  -file test.txt \
  -verbose

# AWS S3 with Access Key
go run main.go \
  -provider aws \
  -auth access-key \
  -uri "s3://my-bucket/test-prefix" \
  -file test.txt \
  -verbose

# GCP with Service Account
go run main.go \
  -provider gcp \
  -auth service-account \
  -uri "gs://my-bucket/test-prefix" \
  -file test.txt \
  -verbose

# Azure with Service Principal
go run main.go \
  -provider azure \
  -auth service-principal \
  -uri "azure://container@account/test-prefix" \
  -file test.txt \
  -verbose

# GitHub LFS with PAT
go run main.go \
  -provider github \
  -auth personal-access-token \
  -uri "github://owner/repo@main/test-path" \
  -file test.txt \
  -verbose
```

## Storage Operations Tested

The sample tests the following storage operations:

1. **Upload** - Upload a file with metadata
2. **Download** - Download a file with options
3. **Get** - Stream object content
4. **Put** - Upload from reader
5. **Exists** - Check if object exists
6. **GetObjectInfo** - Get basic object metadata
7. **Stat** - Get extended object metadata
8. **List** - List objects with prefix
9. **Copy** - Copy object within storage
10. **Delete** - Delete object
11. **Multipart** - Multipart upload (if supported)
12. **Bulk Operations** - Bulk download/upload (if supported)
13. **Validation** - MD5 validation (if supported)
14. **Progress** - Progress tracking (if supported)

## Provider-Specific Notes

### OCI
- Supports all authentication types: user-principal, instance-principal, resource-principal
- Full multipart upload support
- MD5 validation for multipart uploads stored in metadata

### AWS S3
- Supports access-key, iam-role, web-identity authentication
- Native S3 multipart upload
- ETag validation

### Google Cloud Storage
- Supports service-account, workload-identity, application-default
- Composite object support
- Strong consistency

### Azure Blob Storage
- Supports service-principal, managed-identity, device-flow
- Block blob operations
- Lease support

### GitHub LFS
- Supports personal-access-token, github-app
- SHA256 validation (not MD5)
- Limited to repository files

## Troubleshooting

### Authentication Failures

1. **OCI**: Ensure your OCI config file is properly set up
2. **AWS**: Check AWS credentials and region
3. **GCP**: Verify service account key file path
4. **Azure**: Confirm Azure AD app registration
5. **GitHub**: Ensure PAT has appropriate scopes

### Network Issues

- Check firewall rules
- Verify proxy settings
- Ensure DNS resolution works

### Permission Errors

- OCI: Check IAM policies for compartment
- AWS: Verify S3 bucket policies
- GCP: Check Cloud Storage IAM roles
- Azure: Verify RBAC assignments
- GitHub: Check repository permissions

## Examples

### Testing All Operations for OCI

```bash
# Create a test file
echo "Hello, World!" > test.txt

# Run comprehensive test
go run main.go \
  -provider oci \
  -auth user-principal \
  -uri "oci://my-namespace@us-ashburn-1/my-bucket/test" \
  -file test.txt \
  -verbose
```

### Testing Multipart Upload

```bash
# Create a larger file (10MB)
dd if=/dev/urandom of=large-test.txt bs=1M count=10

# Test multipart
go run main.go \
  -provider aws \
  -auth access-key \
  -uri "s3://my-bucket/multipart-test" \
  -file large-test.txt \
  -verbose
```

### Testing Bulk Operations

```bash
# The test will automatically create multiple files and test bulk operations
go run main.go \
  -provider gcp \
  -auth service-account \
  -uri "gs://my-bucket/bulk-test" \
  -verbose
```

## Expected Output

Successful test run will show:

```
Test Results:
=============
✓ PASS oci/user-principal - upload (1.23s)
✓ PASS oci/user-principal - download (0.45s)
✓ PASS oci/user-principal - get (0.12s)
✓ PASS oci/user-principal - put (0.34s)
✓ PASS oci/user-principal - exists (0.08s)
✓ PASS oci/user-principal - get-object-info (0.09s)
✓ PASS oci/user-principal - stat (0.10s)
✓ PASS oci/user-principal - list (0.15s)
✓ PASS oci/user-principal - copy (0.22s)
✓ PASS oci/user-principal - multipart (2.34s)
✓ PASS oci/user-principal - bulk-download (1.56s)
✓ PASS oci/user-principal - validation (0.45s)
✓ PASS oci/user-principal - progress (0.67s)
✓ PASS oci/user-principal - delete (0.11s)

Summary: 14 passed, 0 failed
```

## Extending the Tests

To add more test scenarios:

1. Add new test functions following the pattern `testXXX`
2. Update `runProviderTests` to include the new tests
3. Handle provider-specific features appropriately

## Performance Testing

For performance testing, you can:

1. Use larger files
2. Increase concurrency for bulk operations
3. Test with various chunk sizes
4. Measure throughput and latency

```bash
# Create 100MB test file
dd if=/dev/urandom of=perf-test.txt bs=1M count=100

# Run with timing
time go run main.go \
  -provider aws \
  -auth access-key \
  -uri "s3://my-bucket/perf-test" \
  -file perf-test.txt
```