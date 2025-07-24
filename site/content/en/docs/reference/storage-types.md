---
title: "Storage Types"
date: 2025-07-25
weight: 20
description: >
  Complete API reference for all supported storage types in OME BaseModel and
  ClusterBaseModel resources.
---

## Storage Type Overview

| Type            | URI Format                    | Agent Role | Metadata       | Authentication                  |
| --------------- | ----------------------------- | ---------- | -------------- | -------------------------------- |
| **PVC**         | `pvc://[ns:]name/path`        | Skipped    | Controller+Job | Kubernetes RBAC only             |
| **OCI**         | `oci://n/ns/b/bucket/o/path`  | Downloads  | Agent          | Instance/User/Resource Principal |
| **HuggingFace** | `hf://model[@branch]`         | Downloads  | Agent          | Token (optional)                 |
| **AWS S3**      | `s3://bucket/path`            | Downloads  | Agent          | Access key / IRSA                |
| **Azure Blob**  | `azure://account/container/path` | Downloads | Agent        | Account key / MSI                |
| **GCS**         | `gs://bucket/path`            | Downloads  | Agent          | Service account                  |
| **GitHub**      | `gh://owner/repo@tag/path`    | Downloads  | Agent          | Token (optional)                 |

> Note: Additional cloud/object storage integrations will be documented once they graduate from preview support.

## BaseModel Field Reference

PVC and all other storage providers share the same `spec.storage` block of the
[BaseModel](https://sgl-project.github.io/ome/docs/reference/ome.v1beta1/#ome-io-v1beta1-BaseModel)
CRD:

```yaml
spec:
  storage:
    storageUri: <scheme://...>         # Required for every storage type
    path: /raid/models/optional-local  # Optional node-local path for agent downloads
    storageKey: my-secret              # Secret containing credentials (non-PVC)
    parameters:                        # Provider-specific hints
      region: us-east-1
  annotations:
    ome.io/skip-config-parsing: "true" # Optional PVC-specific override
```

Use `ClusterBaseModel.spec.storage.storageUri` to reference PVCs across namespaces (via
`pvc://{namespace}:{name}/{sub-path}`) while normal BaseModels must live beside the PVC.

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
- BaseModel service account must be able to `get` the PVC (namespace scoped)

### Related CRD Fields

- `spec.storage.storageUri` — `pvc://` URI including namespace prefix for ClusterBaseModel.
- `metadata.annotations["ome.io/skip-config-parsing"]` — opt out of metadata extraction if
  `config.json` is absent.
- `spec.storage.parameters["subPath"]` (optional) — override auto-detected subpath inside the PVC.

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

## Authentication Patterns

### Credential Storage

```yaml
# HuggingFace
apiVersion: v1
kind: Secret
metadata:
  name: hf-token
type: Opaque
stringData:
  token: hf_xxx

# OCI (User Principal)
apiVersion: v1
kind: Secret
metadata:
  name: oci-credentials
type: Opaque
stringData:
  tenancy_ocid: ocid1.tenancy.oc1..example
  user_ocid: ocid1.user.oc1..example
  fingerprint: 1a:2b:3c:4d
  private_key: |-
    -----BEGIN PRIVATE KEY-----
    ...
    -----END PRIVATE KEY-----
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
| **Cloud Native**     | OCI Object Store | Durable, versioned     |
| **Hybrid**           | HF + PVC         | Sync public to private |

## Common Configurations

### Multi-Environment

```yaml
# Dev
storageUri: "hf://model-name"

# Prod
storageUri: "pvc://prod-models/model-name"
```

### Hybrid Sync (OCI to PVC)

```yaml
# Authoritative copy in Object Storage
storageUri: "oci://n/ml/b/prod-models/o/llama-3.1-70b-instruct"

# Cached copy inside the cluster
storageUri: "pvc://prod-models/llama-3-1-70b"
```

## Related Documentation

- [PVC Storage Guide](/ome/docs/user-guide/storage/pvc-storage/) - PVC usage
- [BaseModel Reference](/ome/docs/concepts/base_model/) - Complete BaseModel spec
- [Troubleshooting](/ome/docs/troubleshooting/pvc-storage/) - Common issues
- [Architecture: PVC Flow](/ome/docs/architecture/pvc-storage-flow/) - Controller and job design
