---
title: "Organization"
date: 2023-03-14
weight: 1
description: >
  Organization manages AI platform vendors and their configurations.
---

An _Organization_ is a resource that represents an AI platform vendor (e.g., OpenAI, Anthropic) and manages vendor-specific configurations including API keys and settings.

## Overview

Organizations provide the foundation for accessing external AI services. Each organization represents a vendor account and contains the necessary credentials and configuration to interact with that vendor's APIs.

## Example Configuration

```yaml
apiVersion: ome.io/v1beta1
kind: Organization
metadata:
  name: openai-org
spec:
  vendor: openai
  organizationId: "org-123456789"
  secretRef:
    name: openai-credentials
    namespace: default
    key: api-key
  config:
    region: "us-west-2"
    endpoint: "https://api.openai.com/v1"
```

## Spec Attributes

| Attribute          | Description                                              | Required |
|-------------------|----------------------------------------------------------|----------|
| `vendor`          | AI platform vendor (e.g., "openai", "anthropic")       | Yes      |
| `organizationId`  | Platform-specific organization ID                       | Yes      |
| `disabled`        | Whether the organization is disabled                     | No       |
| `secretRef`       | Reference to secret containing API key                  | No       |
| `config`          | Vendor-specific configuration as key-value pairs        | No       |

## Supported Vendors

OME supports the following AI platform vendors:

- **openai**: OpenAI platform
- **anthropic**: Anthropic platform
- **cohere**: Cohere platform

Each vendor may have specific configuration requirements and capabilities.

## Secret Configuration

API keys and other sensitive information should be stored in Kubernetes secrets:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: openai-credentials
  namespace: default
type: Opaque
data:
  api-key: <base64-encoded-api-key>
```

The secret reference in the Organization spec should point to this secret:

```yaml
secretRef:
  name: openai-credentials
  namespace: default
  key: api-key
```

## Vendor-Specific Configuration

Different vendors may require specific configuration parameters:

### OpenAI Configuration

```yaml
config:
  endpoint: "https://api.openai.com/v1"
  timeout: "30s"
```

### Anthropic Configuration

```yaml
config:
  endpoint: "https://api.anthropic.com"
  version: "2023-06-01"
```

## Status

The Organization status provides information about the current state:

```yaml
status:
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2023-03-14T10:00:00Z"
      reason: "OrganizationReady"
      message: "Organization is ready for use"
```

### Condition Types

- **Ready**: Indicates whether the organization is ready for use
- **CredentialsValid**: Indicates whether the provided credentials are valid
- **ConfigurationValid**: Indicates whether the configuration is valid

## Usage in Projects

Organizations are referenced by Projects to establish vendor connections:

```yaml
apiVersion: ome.io/v1beta1
kind: Project
metadata:
  name: my-project
spec:
  name: "My AI Project"
  organizationRef:
    name: openai-org
    namespace: default
```

## Best Practices

1. **Security**: Always store API keys in secrets, never in plain text
2. **Access Control**: Use RBAC to control who can create and manage organizations
3. **Monitoring**: Monitor organization status and credential validity
4. **Configuration**: Use vendor-specific configurations for optimal performance
5. **Naming**: Use descriptive names that indicate the vendor and purpose

## Troubleshooting

Common issues and solutions:

### Organization Not Ready

```bash
kubectl describe organization openai-org
```

Check the conditions section for error details. Common causes:
- Invalid API key
- Network connectivity issues
- Incorrect vendor configuration

### Secret Not Found

Ensure the referenced secret exists and contains the correct key:

```bash
kubectl get secret openai-credentials -o yaml
```

### Invalid Vendor Configuration

Verify vendor-specific configuration parameters match the vendor's API requirements. 