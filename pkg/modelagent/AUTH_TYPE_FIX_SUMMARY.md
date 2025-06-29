# Auth Type Mapping Fix Summary

## Issue
Runtime error: `failed to create credentials: unsupported OCI auth type:` (empty string)

The error occurred because:
1. The `extractAuthConfig` method was passing raw string auth types from parameters directly to the auth factory, but the factories expect specific auth package constants.
2. When `storage.parameters` was nil (not specified in the YAML), no auth type was being set at all.
3. The code was using a local `StorageConfig` type instead of the `storage.StorageConfig` type expected by the storage factory.

## Root Cause
In `gopher.go` line 595 (old code):
```go
config.AuthType = auth.AuthType(authTypeStr)
```
This was casting the raw string (e.g., "instance_principal") to an AuthType, but the OCI factory expects constants like `auth.OCIInstancePrincipal`.

## Solution
Added a `mapAuthType` function that properly maps string auth types from parameters to the correct auth package constants:

```go
func mapAuthType(provider storage.Provider, authTypeStr string) auth.AuthType {
    switch provider {
    case storage.ProviderOCI:
        switch authTypeStr {
        case "instance_principal":
            return auth.OCIInstancePrincipal
        case "user_principal":
            return auth.OCIUserPrincipal
        // ... more mappings
        }
    // ... other providers
    }
}
```

## Changes Made

### 1. Added mapAuthType Function (gopher.go lines 576-653)
- Maps string auth types to proper constants for each provider
- Handles all auth types for OCI, AWS, GCP, and Azure
- Provides sensible defaults if auth type is not recognized

### 2. Updated extractAuthConfig Method (gopher.go)
- Changed from direct casting to using mapAuthType function when auth is specified:
  ```go
  config.AuthType = mapAuthType(provider, authTypeStr)
  ```
- Added handling for nil parameters case (lines 736-772) to set default auth types even when no parameters are specified in the YAML

### 3. Fixed StorageConfig Type (gopher.go)
- Removed local `StorageConfig` type definition (lines 60-66)
- Updated `createStorageClient` to use `storage.StorageConfig` from the storage package
- This ensures the config implements the `GetAuthConfig()` method expected by the storage factory

### 4. Updated Tests (storage_factory_test.go)
- Fixed test expectations to use proper auth constants instead of raw strings
- Added tests for nil parameters case
- All TestExtractAuthConfig tests now pass

## Auth Type Mappings

### OCI
- "instance_principal" → auth.OCIInstancePrincipal
- "user_principal" → auth.OCIUserPrincipal
- "resource_principal" → auth.OCIResourcePrincipal
- "oke_workload_identity" → auth.OCIOkeWorkloadIdentity

### AWS
- "access_key" → auth.AWSAccessKey
- "instance_profile", "iam_role" → auth.AWSInstanceProfile
- "assume_role" → auth.AWSAssumeRole
- "web_identity" → auth.AWSWebIdentity
- "ecs_task_role" → auth.AWSECSTaskRole
- "process" → auth.AWSProcess
- "default" → auth.AWSDefault

### GCP
- "service_account" → auth.GCPServiceAccount
- "application_default" → auth.GCPApplicationDefault
- "workload_identity" → auth.GCPWorkloadIdentity
- "default" → auth.GCPDefault

### Azure
- "service_principal" → auth.AzureServicePrincipal
- "managed_identity" → auth.AzureManagedIdentity
- "device_flow" → auth.AzureDeviceFlow
- "client_secret" → auth.AzureClientSecret
- "client_certificate" → auth.AzureClientCertificate
- "default" → auth.AzureDefault
- "account_key" → auth.AzureAccountKey
- "pod_identity" → auth.AzurePodIdentity

## Enhanced with Fallback Support

The auth package supports fallback authentication, allowing automatic retry with alternative auth methods if the primary fails. I've enhanced the solution to use this feature:

### Fallback Chains When No Auth Type is Specified:

1. **OCI**: Instance Principal → Resource Principal
2. **AWS**: Instance Profile → Default Chain
3. **GCP**: Workload Identity → Application Default  
4. **Azure**: Managed Identity → Default

### Implementation:
```go
case storage.ProviderOCI:
    // Try instance principal first, with resource principal as fallback
    config.AuthType = auth.OCIInstancePrincipal
    config.Fallback = &auth.Config{
        Provider: auth.Provider(provider),
        AuthType: auth.OCIResourcePrincipal,
        Region:   config.Region, // Propagate region
        Extra:    copyExtra(config.Extra), // Propagate secret and other extras
    }
```

This provides better resilience - if instance principal auth fails (e.g., not running on OCI instance), it will automatically try resource principal authentication.

## Verification
- All modelagent tests pass
- Code compiles without errors
- Auth type mapping now properly converts string values to expected constants
- Fallback authentication chains configured for better resilience