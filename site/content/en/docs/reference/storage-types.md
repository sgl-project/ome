---
title: "Storage Types"
date: 2025-07-25
weight: 20
description: >
  Complete API reference for all supported storage types in OME BaseModel and
  ClusterBaseModel resources.
---

## Storage Type Overview

| Type            | URI Format                    | Agent Role | Metadata       | Authentication          |
| --------------- | ----------------------------- | ---------- | -------------- | ----------------------- |
| **PVC**         | `pvc://[ns:]name/path`        | Skipped    | Controller+Job | None                    |
| **OCI**         | `oci://n/ns/b/bucket/o/path`  | Downloads  | Agent          | Instance/User Principal |
| **HuggingFace** | `hf://model[@branch]`         | Downloads  | Agent          | Token (optional)        |
| **S3**          | `s3://bucket[@region]/path`   | Downloads  | Agent          | AWS credentials         |
| **Azure**       | `az://account/container/path` | Downloads  | Agent          | Storage key             |
| **GCS**         | `gs://bucket/path`            | Downloads  | Agent          | Service account         |
| **GitHub**      | `github://owner/repo[@tag]`   | Downloads  | Agent          | Token (private)         |
| **Vendor**      | `vendor://name/type/path`     | Downloads  | Agent          | Custom                  |

## PVC Storage

### URI Format

```
# BaseModel (same namespace)
pvc://{pvc-name}/{sub-path}

# ClusterBaseModel (explicit namespace)
pvc://{namespace}:{pvc-name}/{sub-path}
```

### Parameters

| Field       | Required              | Description                        |
| ----------- | --------------------- | ---------------------------------- |
| `pvc-name`  | Yes                   | Name of PVC containing models      |
| `namespace` | ClusterBaseModel only | Namespace containing PVC           |
| `sub-path`  | Yes                   | Path within PVC to model directory |

### Examples

```yaml
# BaseModel
storage:
  storageUri: "pvc://models-pvc/llama/llama-3-70b"

# ClusterBaseModel
storage:
  storageUri: "pvc://ai-models:models-pvc/llama/llama-3-70b"
```

### Requirements

- PVC must be `Bound`
- Model directory must contain `config.json`
- Files readable by metadata extraction job

## OCI Object Storage

**URI Format:** `oci://n/{namespace}/b/{bucket}/o/{object_path}`

### URI Components

| Component     | Required | Description                       |
| ------------- | -------- | --------------------------------- |
| `namespace`   | Yes      | OCI compartment namespace         |
| `bucket`      | Yes      | Object storage bucket name        |
| `object_path` | Yes      | Path to model files within bucket |

### Examples

```yaml
apiVersion: ome.io/v1beta1
kind: BaseModel
metadata:
  name: llama-oci
spec:
  storage:
    storageUri: "oci://n/ai-models/b/llm-store/o/meta/llama-3.1-70b-instruct/"
    path: "/raid/models/llama-3.1-70b-instruct"
    storageKey: "oci-credentials"
    parameters:
      region: "us-phoenix-1"
      auth_type: "InstancePrincipal"
```

### Authentication Methods

| Method              | Description                   | Configuration                    |
| ------------------- | ----------------------------- | -------------------------------- |
| `InstancePrincipal` | Use compute instance identity | No credentials needed            |
| `UserPrincipal`     | User-based authentication     | Requires API key in secret       |
| `ResourcePrincipal` | OKE resource principal        | Automatic in OKE clusters        |
| `WorkloadIdentity`  | Service account based         | Requires workload identity setup |

## Other Storage Types

### OCI Object Storage

```
oci://n/{namespace}/b/{bucket}/o/{object_path}
```

**Parameters:** `region`, `auth_type` (InstancePrincipal/UserPrincipal/ResourcePrincipal)

### HuggingFace Hub

```
hf://{model-id}[@{branch}]
```

**Parameters:** `revision`, `cache_dir`, `secretKey`

### AWS S3

```
s3://{bucket}[@{region}]/{prefix}
```

**Parameters:** `region`

### Azure Blob Storage

```
az://{account}[.blob.core.windows.net]/{container}/{blob_path}
```

### Google Cloud Storage

```
gs://{bucket}/{object_path}
```

### GitHub Releases

```
github://{owner}/{repository}[@{tag}]
```

### Vendor Storage

```
vendor://{vendor-name}/{resource-type}/{resource-path}
```

**Parameters:** `api_version`, `endpoint`, custom vendor parameters

## Authentication Patterns

### Credential Storage

```yaml
# HuggingFace
apiVersion: v1
kind: Secret
metadata:
  name: hf-token
data:
  token: <base64-token>

# AWS
apiVersion: v1
kind: Secret
metadata:
  name: aws-credentials
data:
  access_key_id: <base64-key>
  secret_access_key: <base64-secret>

# Azure
apiVersion: v1
kind: Secret
metadata:
  name: azure-credentials
data:
  account_name: <base64-name>
  account_key: <base64-key>

# GCP
apiVersion: v1
kind: Secret
metadata:
  name: gcp-credentials
data:
  service_account_key: <base64-json-key>
```

### BaseModel Template

```yaml
apiVersion: ome.io/v1beta1
kind: BaseModel
metadata:
  name: example-model
spec:
  storage:
    storageUri: "<storage-uri>"
    path: "/local/path/to/model"
    storageKey: "credential-secret-name"
    parameters:
      key: "value" # Storage-specific parameters
```

## Storage Selection Guide

| Use Case             | Recommended Type | Access Pattern         |
| -------------------- | ---------------- | ---------------------- |
| **Development**      | HuggingFace      | Frequent model updates |
| **High Performance** | PVC (NVMe/SSD)   | Low latency serving    |
| **Shared Models**    | PVC (NFS/RWX)    | Multiple consumers     |
| **Cloud Native**     | Object Storage   | Scalable, versioned    |
| **Version Control**  | GitHub Releases  | Git-based workflow     |
| **Enterprise**       | Vendor Storage   | Proprietary systems    |

## Common Configurations

### Multi-Environment

```yaml
# Dev
storageUri: "hf://model-name"

# Prod
storageUri: "pvc://prod-models/model-name"
```

### Multi-Cloud Backup

```yaml
# Primary
storageUri: "s3://primary-bucket/model"

# Backup
storageUri: "az://backup-account/models/model"
```

## Related Documentation

- [PVC Storage Guide](/ome/docs/user-guide/storage/pvc-storage/) - PVC usage
- [BaseModel Reference](/ome/docs/concepts/base_model/) - Complete BaseModel spec
- [Troubleshooting](/ome/docs/troubleshooting/pvc-storage/) - Common issues
