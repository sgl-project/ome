---
title: "Storage Configuration"
linkTitle: "Storage Configuration"
date: 2025-07-25
weight: 10
description: >
  Configure different storage backends for your OME models.
---

OME supports multiple storage backends for BaseModel and ClusterBaseModel resources. This
section provides detailed guides for configuring and using different storage types.

## Available Storage Types

- **[PVC Storage](/ome/docs/user-guide/storage/pvc-storage/)** - Use models stored in
  Kubernetes Persistent Volume Claims
- **OCI Object Storage** - Oracle Cloud Infrastructure object storage
- **HuggingFace Hub** - Public and private models from HuggingFace
- **AWS S3** - Amazon S3 compatible storage
- **Azure Blob Storage** - Microsoft Azure blob storage
- **Google Cloud Storage** - Google Cloud Platform storage
- **GitHub Releases** - Models distributed via GitHub releases

## Choosing a Storage Type

The choice of storage type depends on your specific requirements:

- **Use PVC Storage** when you have models already stored in Kubernetes persistent volumes
- **Use Object Storage** (OCI, S3, Azure, GCS) for cloud-native deployments
- **Use HuggingFace** for public models or when developing with transformer models
- **Use GitHub Releases** for open-source model projects with version control

For a complete comparison of storage types, see the [Storage Types Reference](/ome/docs/reference/storage-types/).
