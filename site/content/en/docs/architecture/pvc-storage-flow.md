---
title: "PVC Storage"
date: 2025-07-25
weight: 15
description: >
  Technical architecture and design decisions for PVC storage support in OME.
---

## Architecture Overview

PVC storage in OME uses a **controller-only architecture** that bypasses the model
agent entirely. This design leverages Kubernetes native volume mounting and eliminates
model duplication.

## Key Design Decision: Skip Model Agent

**Why?** DaemonSet pods cannot efficiently mount PVCs, especially ReadWriteOnce
volumes.

```mermaid
flowchart LR
    subgraph "Traditional Storage"
        MA1[Model Agent] --> Download[Download Model]
        Download --> HostPath[Store on Host]
        HostPath --> Label[Label Node]
    end

    subgraph "PVC Storage"
        BMC[BaseModel Controller] --> Validate[Validate PVC]
        Validate --> Extract[Extract Metadata]
        Extract --> Mount[Direct PVC Mount]
        MA2[Model Agent] -.-> Skip[Skips PVC Models]
    end

    classDef traditional fill:#ffebee
    classDef pvc fill:#e8f5e8

    class MA1,Download,HostPath,Label traditional
    class BMC,Validate,Extract,Mount pvc
```

## Component Flow

```mermaid
sequenceDiagram
    participant User
    participant BMC as BaseModel Controller
    participant K8s as Kubernetes API
    participant Job as Metadata Job
    participant ISC as InferenceService Controller

    User->>BMC: Create BaseModel with PVC URI
    BMC->>K8s: Validate PVC exists & bound

    alt PVC Ready
        BMC->>K8s: Create metadata extraction Job
        Job->>Job: Mount PVC, read config.json
        Job->>BMC: Update BaseModel metadata
        BMC->>BMC: Set status: Ready
    else PVC Not Ready
        BMC->>BMC: Set status: Failed
    end

    User->>ISC: Create InferenceService
    ISC->>K8s: Create pods with PVC volumes
    K8s->>K8s: Schedule based on PVC constraints
```

## Component Responsibilities

| Component                       | Role                           | PVC Handling                                       |
| ------------------------------- | ------------------------------ | -------------------------------------------------- |
| **Model Agent**                 | Downloads models, labels nodes | **Skips PVC** entirely                             |
| **BaseModel Controller**        | Manages BaseModel lifecycle    | **Primary owner** - validates, extracts metadata   |
| **Metadata Job**                | Extracts model config          | **Temporary** - mounts PVC, reads config.json      |
| **InferenceService Controller** | Manages serving pods           | **Volume mounter** - creates pods with PVC volumes |

## Core Design Decisions

### 1. Why Skip Model Agent?

**Problem**: DaemonSet + PVC incompatibility

- DaemonSets run on every node
- ReadWriteOnce PVCs can't be mounted by multiple pods
- Complex coordination needed for RWO volumes

**Solution**: Controller-only approach

```go
// Model agent explicitly skips PVC storage
switch storageType {
case storage.StorageTypePVC:
    s.logger.Infof("Skipping PVC storage for model %s", modelInfo)
    return nil
}
```

### 2. Why Use Jobs for Metadata?

**Problem**: Need to read model config from PVC
**Solution**: Ephemeral Jobs with PVC mount

```yaml
# Metadata extraction job template
apiVersion: batch/v1
kind: Job
metadata:
  name: metadata-{model}-{hash}
spec:
  template:
    spec:
      containers:
        - name: extractor
          image: ome/metadata-agent
          volumeMounts:
            - name: model-pvc
              mountPath: /models
              readOnly: true
      volumes:
        - name: model-pvc
          persistentVolumeClaim:
            claimName: { pvc-name }
```

### 3. Why No Node Labeling?

**Traditional**: Model agent labels nodes with available models
**PVC**: Kubernetes scheduler handles PVC placement constraints

```mermaid
graph TB
    subgraph "Traditional Storage"
        MA[Model Agent] --> Label[Label Node:<br/>model-xyz=ready]
        Label --> Schedule[Pod scheduled<br/>to labeled node]
    end

    subgraph "PVC Storage"
        PVC[PVC Constraint] --> K8sScheduler[Kubernetes Scheduler]
        K8sScheduler --> AutoSchedule[Pod scheduled<br/>where PVC accessible]
    end
```

## Storage Type Comparison

| Aspect            | PVC Storage       | Object Storage   | HuggingFace      |
| ----------------- | ----------------- | ---------------- | ---------------- |
| **Model Agent**   | Skipped           | Downloads        | Downloads        |
| **Node Labels**   | None              | Creates labels   | Creates labels   |
| **Scheduling**    | PVC constraints   | Node selectors   | Node selectors   |
| **Data Transfer** | None              | Network download | Network download |
| **Availability**  | Storage dependent | Node replicated  | Node replicated  |

## Security Model

### RBAC Requirements

```yaml
# BaseModel Controller permissions
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["batch"]
  resources: ["jobs"]
  verbs: ["create", "get", "list", "watch"]

# Metadata Job permissions
- apiGroups: [""]
  resources: ["persistentvolumeclaims"]
  verbs: ["get"]
- apiGroups: ["ome.io"]
  resources: ["basemodels"]
  verbs: ["update"]
```

### Security Boundaries

- **Namespace isolation**: BaseModel → same namespace PVC only
- **Read-only mounts**: All PVC mounts are read-only
- **Minimal permissions**: Jobs have least-privilege access

## Performance Profile

| Operation              | PVC Storage    | Object Storage      |
| ---------------------- | -------------- | ------------------- |
| **Model Loading**      | Immediate      | Minutes (download)  |
| **Scaling Up**         | Fast           | Slow (re-download)  |
| **Storage Efficiency** | No duplication | Replicated per node |

**Performance depends on storage backend:**

- **NFS**: Good for sharing, may bottleneck with many pods
- **Block storage**: Excellent single-pod, RWO limits concurrency
- **Distributed**: Scales well, varies by implementation

## Common Issues & Solutions

| Issue                  | Cause                | Solution                              |
| ---------------------- | -------------------- | ------------------------------------- |
| MetadataPending        | PVC not bound        | Check PVC status, storage provisioner |
| Pod scheduling failure | PVC node constraints | Verify PVC accessible from nodes      |
| Slow model loading     | Storage performance  | Use faster storage class              |

## Future Enhancements

**Planned:**

- Cross-namespace PVC access with RBAC
- Volume snapshot integration for versioning
- Multi-PVC model support
- Performance optimization hints

**Integration:**

- CSI driver advanced features
- Automatic storage class selection
- Volume expansion for growing repos

## Related Documentation

- [PVC Storage User Guide](/ome/docs/user-guide/storage/pvc-storage/) - How to use
- [Troubleshooting PVC Storage](/ome/docs/troubleshooting/pvc-storage/) - Common issues
- [Storage Types Reference](/ome/docs/reference/storage-types/) - Complete API spec
