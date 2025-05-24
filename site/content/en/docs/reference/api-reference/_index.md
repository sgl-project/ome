---
title: "API Reference"
linkTitle: "API Reference"
weight: 10
description: >
  Complete reference for OME Kubernetes API types
---

This section contains the reference documentation for all OME (Oracle Model Engine) Kubernetes Custom Resource Definitions (CRDs) and their associated API types.

## Overview

OME provides several categories of Kubernetes custom resources to manage AI/ML workloads:

## Core API Types

### AI Platform Management
- **[Organization](./organization/)** - Defines AI platform vendor configurations (OpenAI, Google, xAI)
- **[Project](./project/)** - Represents AI platform projects within an organization
- **[ServiceAccount](./service-account/)** - Service accounts for API access within projects
- **[User](./user/)** - User management within projects
- **[RateLimit](./rate-limit/)** - Rate limiting configurations for projects and users

### Model Management
- **[BaseModel](./base-model/)** - Defines base AI/ML models with metadata and storage configuration
- **[ClusterBaseModel](./cluster-base-model/)** - Cluster-scoped version of BaseModel
- **[FineTunedWeight](./fine-tuned-weight/)** - Fine-tuned model weights derived from base models

### Serving
- **[InferenceService](./inference-service/)** - Deploys and manages model serving endpoints
- **[ServingRuntime](./serving-runtime/)** - Defines serving runtime environments for models
- **[ClusterServingRuntime](./cluster-serving-runtime/)** - Cluster-scoped serving runtime
- **[InferenceGraph](./inference-graph/)** - Orchestrates complex inference workflows

### Training
- **[TrainingJob](./training-job/)** - Manages distributed training workloads
- **[TrainingRuntime](./training-runtime/)** - Defines training runtime environments
- **[ClusterTrainingRuntime](./cluster-training-runtime/)** - Cluster-scoped training runtime

### Infrastructure
- **[DedicatedAICluster](./dedicated-ai-cluster/)** - Dedicated compute clusters for AI workloads
- **[CapacityReservation](./capacity-reservation/)** - Resource capacity management and reservation

### Jobs and Operations
- **[BenchmarkJob](./benchmark-job/)** - Performance benchmarking of inference services
- **[ReplicationJob](./replication-job/)** - Replicates models across clusters or storage systems

## API Versions

OME currently supports the following API versions:

- **v1beta1** - Current stable version for all resources

## Common Types and Patterns

### Cross References
Many OME resources use `CrossReference` objects to reference other resources:

```yaml
crossReference:
  name: "resource-name"
  namespace: "resource-namespace"  # Optional for cluster-scoped resources
```

### Storage Specifications
OME uses `StorageSpec` for defining model and data storage:

```yaml
storage:
  storageUri: "oci://namespace/bucket/object"
  parameters:
    key: "value"
  nodeSelector:
    node-type: "gpu"
```

### Status Conditions
All OME resources follow Kubernetes conventions for status reporting using `Conditions`:

```yaml
status:
  conditions:
  - type: "Ready"
    status: "True"
    reason: "ResourceReady"
    message: "Resource is ready for use"
```

## Resource Relationships

### AI Platform Management Flow
```
Organization
├── Project
│   ├── ServiceAccount
│   ├── User
│   └── RateLimit
```

### Model Management Flow
```
BaseModel/ClusterBaseModel
├── Used by → InferenceService
├── Used by → TrainingJob
└── Produces → FineTunedWeight
    └── Used by → InferenceService
```

### Training and Serving Flow
```
TrainingRuntime → TrainingJob → FineTunedWeight
                      ↓
BaseModel → InferenceService ← ServingRuntime
              ↓
        BenchmarkJob
```

### Infrastructure and Resource Management
```
CapacityReservation
├── Linked to → DedicatedAICluster
│   ├── Hosts → InferenceService
│   └── Hosts → TrainingJob
└── Manages → Resource Quotas
```

### Data Operations
```
ReplicationJob
├── Copies → BaseModel artifacts
├── Copies → FineTunedWeight artifacts
├── Copies → Training datasets
└── Copies → Benchmark results
```

## Key Integration Patterns

1. **Model Development**: `BaseModel` → `TrainingRuntime` → `TrainingJob` → `FineTunedWeight`
2. **Model Serving**: `BaseModel` + `FineTunedWeight` → `ServingRuntime` → `InferenceService`
3. **Resource Management**: `CapacityReservation` → `DedicatedAICluster` → Workloads
4. **Performance Testing**: `InferenceService` → `BenchmarkJob`
5. **Data Distribution**: `ReplicationJob` for cross-region/cross-storage copying

## Authentication and Authorization

OME integrates with Kubernetes RBAC for access control. See the [Security Guide](../security/) for detailed information on setting up appropriate permissions.

## Labels and Annotations

OME uses standardized labels and annotations across all resources. See the [Labels and Annotations Reference](../labels-annotations/) for a complete list.

## Migration and Compatibility

When upgrading between OME versions, refer to the [Migration Guide](../migration/) for information about API changes and upgrade procedures. 