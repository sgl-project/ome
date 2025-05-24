---
title: "Project"
date: 2023-03-14
weight: 2
description: >
  Project manages scoped access to AI services within an organization.
---

A _Project_ is a resource that represents a project within an organization, providing scoped access to models and AI services. Projects enable team-based organization and resource management.

## Overview

Projects provide a way to organize and manage access to AI resources within an organization. Each project belongs to an organization and can contain users, service accounts, and associated configurations.

## Example Configuration

```yaml
apiVersion: ome.io/v1beta1
kind: Project
metadata:
  name: nlp-research
spec:
  name: "Natural Language Processing Research"
  description: "Project for NLP research and development"
  organizationRef:
    name: openai-org
    namespace: default
  config:
    defaultModel: "gpt-4"
    billingAccount: "billing-123"
```

## Spec Attributes

| Attribute          | Description                                              | Required |
|-------------------|----------------------------------------------------------|----------|
| `name`            | Human-readable project name                              | Yes      |
| `description`     | Project description                                      | No       |
| `organizationRef` | Reference to the parent Organization                     | Yes      |
| `config`          | Project-specific configuration as key-value pairs       | No       |

## Organization Reference

Projects must reference a parent organization:

```yaml
organizationRef:
  name: openai-org
  namespace: default  # Optional if in same namespace
```

The organization provides the vendor credentials and base configuration that the project inherits.

## Project Configuration

Projects can override or extend organization-level configurations:

```yaml
config:
  defaultModel: "gpt-4"
  billingAccount: "billing-123"
  region: "us-east-1"
  quotaLimits: "standard"
```

Common configuration options:
- **defaultModel**: Default model to use for inference
- **billingAccount**: Billing account for resource usage
- **region**: Preferred region for resource deployment
- **quotaLimits**: Usage quota tier

## Status

The Project status provides information about the current state:

```yaml
status:
  projectId: "proj-abc123def456"
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2023-03-14T10:00:00Z"
      reason: "ProjectReady"
      message: "Project is ready for use"
  creationTime: "2023-03-14T09:30:00Z"
  lastUpdatedTime: "2023-03-14T10:00:00Z"
```

### Status Fields

- **projectId**: Platform-specific project identifier
- **conditions**: Current status conditions
- **creationTime**: When the project was created
- **lastUpdatedTime**: Last modification time

### Condition Types

- **Ready**: Indicates whether the project is ready for use
- **OrganizationReady**: Indicates whether the parent organization is available
- **ConfigurationValid**: Indicates whether the project configuration is valid

## User Management

Projects can contain users with different roles:

```yaml
apiVersion: ome.io/v1beta1
kind: User
metadata:
  name: alice-researcher
spec:
  email: "alice@company.com"
  projectRef:
    name: nlp-research
    namespace: default
  role: "member"
```

### User Roles

- **owner**: Full access to project resources and management
- **member**: Access to project resources with limited management capabilities

## Service Account Management

Projects can contain service accounts for programmatic access:

```yaml
apiVersion: ome.io/v1beta1
kind: ServiceAccount
metadata:
  name: nlp-pipeline
spec:
  name: "NLP Pipeline Service Account"
  projectRef:
    name: nlp-research
    namespace: default
  permissions:
    - "models:read"
    - "inference:create"
  role: "member"
```

## Rate Limiting

Projects can have rate limits applied to control resource usage:

```yaml
apiVersion: ome.io/v1beta1
kind: RateLimit
metadata:
  name: project-limits
spec:
  projectRef:
    name: nlp-research
    namespace: default
  targetRef:
    name: nlp-research
    namespace: default
  limits:
    - type: "requests"
      limit: 1000
      window: "1h"
    - type: "tokens"
      limit: 100000
      window: "1d"
```

## Project Lifecycle

### Creation

1. Create the parent Organization first
2. Create the Project with organization reference
3. Add users and service accounts as needed
4. Configure rate limits and quotas

### Updates

Project configurations can be updated:
- Modify description and configuration
- Cannot change organization reference
- Status fields are updated by the controller

### Deletion

When deleting a project:
1. Remove all users and service accounts
2. Clean up associated rate limits
3. Delete the project resource

## Best Practices

1. **Naming**: Use descriptive names that indicate the project purpose
2. **Organization**: Group related work under the same project
3. **Access Control**: Use appropriate user roles and permissions
4. **Monitoring**: Monitor project usage and costs
5. **Configuration**: Leverage project-level configurations for consistency

## Examples

### Research Project

```yaml
apiVersion: ome.io/v1beta1
kind: Project
metadata:
  name: ai-research
spec:
  name: "AI Research Lab"
  description: "Experimental AI research and model development"
  organizationRef:
    name: openai-org
  config:
    environment: "research"
    costCenter: "research-123"
```

### Production Service

```yaml
apiVersion: ome.io/v1beta1
kind: Project
metadata:
  name: customer-service
spec:
  name: "Customer Service AI"
  description: "Production AI services for customer support"
  organizationRef:
    name: anthropic-org
  config:
    environment: "production"
    slaLevel: "premium"
    monitoring: "enabled"
```

## Troubleshooting

### Project Not Ready

Check the project status and conditions:

```bash
kubectl describe project nlp-research
```

Common issues:
- Parent organization not ready
- Invalid configuration parameters
- Insufficient permissions

### Organization Reference Issues

Verify the organization exists and is ready:

```bash
kubectl get organization openai-org
kubectl describe organization openai-org
```

### Configuration Validation

Ensure project configuration values are valid for the organization's vendor. 