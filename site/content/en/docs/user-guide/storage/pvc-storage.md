---
title: "PVC Storage for Models"
date: 2025-07-25
weight: 10
description: >
  Use models stored in Kubernetes Persistent Volume Claims (PVCs) directly with OME,
  eliminating the need to copy models to object storage.
---

## Quick Start

### Prerequisites

- Kubernetes cluster with PVC support
- Model files already in a PVC with `config.json`

### Step 1: Create PVC (if needed)

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: models-pvc
  namespace: default
spec:
  accessModes: [ReadWriteMany]
  resources:
    requests:
      storage: 100Gi
```

### Step 2: Create BaseModel

```yaml
apiVersion: ome.io/v1beta1
kind: BaseModel
metadata:
  name: my-model
spec:
  storage:
    storageUri: "pvc://models-pvc/path/to/model"
```

### Step 3: Verify & Use

```bash
# Check status
kubectl get basemodel my-model -o wide
```

```yaml
# Deploy an InferenceService that consumes the PVC model
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: my-service
spec:
  model:
    name: my-model
    runtime: vllm-runtime
  predictor:
    containers:
    - name: predictor
      image: ghcr.io/sgl-project/vllm-runtime:latest
      args:
      - "--model"
      - "$(MODEL_PATH)"
      resources:
        limits:
          nvidia.com/gpu: "1"
```

`MODEL_PATH` is automatically injected by the controller and points at the PVC mount. The PVC
volume is also mounted into the predictor container—no additional `volumeMounts` are required.

**Done!** Your PVC model is now serving.

---

## When to Use PVC Storage

| Use PVC When              | Don't Use When                |
| ------------------------- | ----------------------------- |
| Models already in PVCs    | Need models on specific nodes |
| Avoiding data duplication | Want model agent management   |
| High-performance storage  | Need node-specific labeling   |
| Shared model repositories | Require local caching         |

## URI Format Reference

### BaseModel (same namespace)

```
pvc://{pvc-name}/{sub-path}
```

### ClusterBaseModel (explicit namespace)

```
pvc://{namespace}:{pvc-name}/{sub-path}
```

**Examples:**

```yaml
# BaseModel - PVC in same namespace
storageUri: "pvc://model-storage/llama/llama-3-70b"

# ClusterBaseModel - specify namespace
storageUri: "pvc://ai-models:model-storage/llama/llama-3-70b"
```

## Common Use Cases

### Shared NFS Models

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nfs-models
spec:
  accessModes: [ReadWriteMany]
  storageClassName: nfs-csi
  resources:
    requests:
      storage: 1Ti
---
apiVersion: ome.io/v1beta1
kind: ClusterBaseModel
metadata:
  name: shared-llama
spec:
  storage:
    storageUri: "pvc://ai-models:nfs-models/models/llama-3-70b"
```

### High-Performance Block Storage

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: fast-models
spec:
  accessModes: [ReadWriteOnce] # Single node only
  storageClassName: fast-ssd
  resources:
    requests:
      storage: 500Gi
---
apiVersion: ome.io/v1beta1
kind: BaseModel
metadata:
  name: fast-llama
spec:
  storage:
    storageUri: "pvc://fast-models/models/llama-3-70b"
```

### Manual Metadata (No config.json)

```yaml
apiVersion: ome.io/v1beta1
kind: BaseModel
metadata:
  name: custom-model
  annotations:
    ome.io/skip-config-parsing: "true"
spec:
  modelType: "llama"
  modelArchitecture: "LlamaForCausalLM"
  modelParameterSize: "70B"
  maxTokens: 8192
  modelCapabilities: [text-to-text]
  modelFormat:
    name: "safetensors"
  storage:
    storageUri: "pvc://models-pvc/custom/my-model"
```

## PVC Access Modes

| Mode              | Use Case                     | Behavior                | Storage Type        |
| ----------------- | ---------------------------- | ----------------------- | ------------------- |
| **ReadWriteMany** | Shared models, multiple pods | Multiple pods can mount | NFS, distributed    |
| **ReadWriteOnce** | High-performance, single pod | Only one pod can mount  | Block storage, SSDs |
| **ReadOnlyMany**  | Immutable model repos        | Multiple read-only pods | Any storage         |

## Model Directory Structure

Your PVC must contain models in this structure:

```
/models/
├── llama-3-70b-instruct/
│   ├── config.json          # Required for auto-metadata
│   ├── model-*.safetensors   # Model files
│   ├── tokenizer.json
│   └── tokenizer_config.json
```

## Monitoring & Status

### Quick Status Check

```bash
# Check BaseModel + metadata
kubectl get basemodel my-model -o yaml | yq '.status'

# Check PVC
kubectl get pvc models-pvc

# Check metadata extraction job (if any)
kubectl get jobs -l "app.kubernetes.io/component=metadata-extraction" -n default
```

### Fields to Inspect

```yaml
status:
  state: Ready                # Overall lifecycle state
  lifecycle: Ready            # Detailed lifecycle marker
  nodesReady: []              # PVC models skip node labeling, so this is normally empty
spec:
  modelType: llama            # Auto-populated from config.json when available
  modelArchitecture: LlamaForCausalLM
```

If metadata parsing is skipped or fails, set `ome.io/skip-config-parsing: "true"` and fill
`spec.modelType`, `spec.modelArchitecture`, and optional `modelParameterSize` manually.

### PVC Metrics to Monitor

- PVC `ACCESS MODES` (must align with the number of replicas you intend to run)
- PVC capacity/usage via `kubectl describe pvc models-pvc`
- Metadata job logs for config parsing failures

## Populate & Migrate Models

### Example: Populating a PVC (HuggingFace)

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: download-model-to-pvc
spec:
  template:
    spec:
      containers:
      - name: downloader
        image: ghcr.io/huggingface/transformers-pytorch-gpu:latest
        command:
        - /bin/bash
        - -c
        - |
          git lfs install
          cd /models
          git clone https://huggingface.co/meta-llama/Llama-2-7b-hf
      volumeMounts:
      - mountPath: /models
        name: model-storage
      restartPolicy: OnFailure
      volumes:
      - name: model-storage
        persistentVolumeClaim:
          claimName: models-pvc
```

### Migrating from Object Storage

```bash
# Copy from S3/Object storage into the PVC
kubectl apply -f - <<'MANIFEST'
apiVersion: batch/v1
kind: Job
metadata:
  name: migrate-to-pvc
spec:
  template:
    spec:
      serviceAccountName: pvc-migrator
      containers:
      - name: migrator
        image: amazon/aws-cli
        command: ["aws", "s3", "sync", "s3://my-bucket/model/", "/models/"]
        env:
        - name: AWS_REGION
          value: us-east-1
        volumeMounts:
        - name: target-pvc
          mountPath: /models
      volumes:
      - name: target-pvc
        persistentVolumeClaim:
          claimName: models-pvc
      restartPolicy: OnFailure
MANIFEST

# Point the BaseModel at the PVC once files exist
kubectl patch basemodel my-model --type='merge' \
  -p='{"spec":{"storage":{"storageUri":"pvc://models-pvc/model-path"}}}'
```

## Best Practices

- **Storage Class**: Use appropriate performance tier (fast-ssd for inference, nfs for sharing)
- **Sizing**: Plan for 100GB+ per large model
- **Organization**: Clear directory structure with consistent naming
- **Monitoring**: Track PVC usage and model status
- **Security**: Implement RBAC and storage encryption

## Limitations

- No model agent involvement (no node labels)
- BaseModel can only access same-namespace PVCs
- Requires PVC to be bound and accessible
- Performance depends on storage backend

## Related Documentation

- [Troubleshooting PVC Storage](/ome/docs/troubleshooting/pvc-storage/) - Common issues
- [PVC Storage Architecture](/ome/docs/architecture/pvc-storage-flow/) - Technical details
- [Storage Types Reference](/ome/docs/reference/storage-types/) - Complete API spec
