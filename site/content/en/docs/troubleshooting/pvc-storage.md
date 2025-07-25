---
title: "PVC Storage"
date: 2025-07-25
weight: 10
description: >
  Common issues, diagnostics, and solutions for PVC storage in OME models.
---

## Quick Diagnostic Checklist

Run these commands first to identify the issue:

```bash
# Check BaseModel status
kubectl get basemodel <model-name> -o wide

# Check PVC status
kubectl get pvc <pvc-name> -n <namespace>

# Check recent events
kubectl get events --sort-by='.lastTimestamp' | grep -E 'basemodel|pvc'

# Check metadata extraction jobs
kubectl get jobs -l "app.kubernetes.io/component=metadata-extraction"
```

## Common Issues Reference

| Symptom             | Likely Cause           | Quick Check             | Solution                                         |
| ------------------- | ---------------------- | ----------------------- | ------------------------------------------------ |
| **MetadataPending** | PVC not found          | `kubectl get pvc`       | [Create PVC](#pvc-not-found)                     |
| **MetadataPending** | PVC not bound          | `kubectl describe pvc`  | [Fix storage](#pvc-not-bound)                    |
| **MetadataPending** | config.json missing    | Check metadata job logs | [Fix model structure](#config-json-missing)      |
| **Pod FailedMount** | Multi-attach error     | RWO PVC + multiple pods | [Use RWX or single replica](#multi-attach-error) |
| **Pod Pending**     | Node affinity conflict | PVC not accessible      | [Check node labels](#node-affinity-conflict)     |
| **Slow loading**    | Storage performance    | Monitor I/O             | [Optimize storage](#storage-performance)         |

## Detailed Solutions

### PVC Not Found

**Error:** `PVC 'model-storage-pvc' not found in namespace 'models'`

**Diagnosis:**

```bash
# Check if PVC exists
kubectl get pvc -n <namespace>

# Check URI format in BaseModel
kubectl get basemodel <name> -o jsonpath='{.spec.storage.storageUri}'
```

**Solutions:**

1. **Create missing PVC:**

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: model-storage-pvc
  namespace: models
spec:
  accessModes: [ReadWriteMany]
  resources:
    requests:
      storage: 200Gi
  storageClassName: your-storage-class
```

2. **Fix URI format:**

```yaml
# BaseModel - same namespace
storageUri: "pvc://model-storage-pvc/path/to/model"

# ClusterBaseModel - explicit namespace
storageUri: "pvc://models:model-storage-pvc/path/to/model"
```

### PVC Not Bound

**Error:** PVC status shows "Pending" instead of "Bound"

**Diagnosis:**

```bash
# Check PVC status and events
kubectl describe pvc <pvc-name> -n <namespace>

# Check storage class and provisioner
kubectl get storageclass
kubectl get pods -n kube-system | grep provisioner
```

**Solutions:**

1. **Check storage class exists:**

```bash
kubectl get storageclass <storage-class-name>
```

2. **Verify provisioner is running:**

```bash
kubectl logs <provisioner-pod> -n kube-system
```

3. **Check resource quotas:**

```bash
kubectl describe quota -n <namespace>
```

### Config.json Missing

**Error:** `config.json not found at path /models/model-name/config.json`

**Diagnosis:**

```bash
# Debug PVC contents
kubectl run pvc-debug --rm -i --tty --image=alpine \
  --overrides='{"spec":{"volumes":[{"name":"pvc","persistentVolumeClaim":{"claimName":"<pvc-name>"}}],"containers":[{"name":"debug","image":"alpine","volumeMounts":[{"mountPath":"/models","name":"pvc"}],"command":["sh"]}]}}'

# Inside pod: check structure
find /models -name "config.json"
ls -la /models/<model-path>/
```

**Solutions:**

1. **Fix model path in URI:**

```yaml
# If config.json is at /models/subdir/config.json
storageUri: "pvc://pvc-name/subdir"
```

2. **Skip automatic parsing:**

```yaml
apiVersion: ome.io/v1beta1
kind: BaseModel
metadata:
  annotations:
    ome.io/skip-config-parsing: "true"
spec:
  modelType: "llama"
  modelArchitecture: "LlamaForCausalLM"
  # ... other metadata fields
  storage:
    storageUri: "pvc://pvc-name/path"
```

### Multi-Attach Error

**Error:** `Volume is already exclusively attached to one node`

**Cause:** ReadWriteOnce PVC with multiple pods

**Solutions:**

1. **Use ReadWriteMany PVC:**

```yaml
spec:
  accessModes: [ReadWriteMany] # Changed from ReadWriteOnce
```

2. **Limit to single replica:**

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
spec:
  engine:
    minReplicas: 1
    maxReplicas: 1
```

### Node Affinity Conflict

**Error:** `node(s) had volume node affinity conflict`

**Cause:** PVC not accessible from available nodes

**Diagnosis:**

```bash
# Check PV node affinity
kubectl get pvc <pvc-name> -o yaml | grep volumeName
kubectl describe pv <volume-name> | grep -A 10 "Node Affinity"

# Check node labels
kubectl get nodes --show-labels
```

**Solutions:**

1. **For local storage:** Ensure model files exist on labeled nodes
2. **For network storage:** Verify storage accessible from all nodes
3. **For CSI:** Check driver configuration and node topology

### Storage Performance

**Symptoms:** Slow model loading, high I/O wait

**Diagnosis:**

```bash
# Monitor pod resource usage
kubectl top pods <pod-name>

# Check I/O in pod
kubectl exec <pod-name> -- iostat -x 1
```

**Solutions:**

1. **Use faster storage class:**

```yaml
storageClassName: fast-ssd # or nvme, premium-ssd
```

2. **Optimize for your storage:**

- **NFS:** Tune mount options (`rsize=1048576,wsize=1048576`)
- **Block:** Use high-IOPS storage tiers
- **Cloud:** Request higher IOPS/throughput

## Diagnostic Scripts

### Complete Status Check

```bash
#!/bin/bash
# Usage: ./pvc-debug.sh <model-name> <namespace>

MODEL_NAME=${1:-"my-model"}
NAMESPACE=${2:-"default"}

echo "=== BaseModel Status ==="
kubectl get basemodel $MODEL_NAME -n $NAMESPACE -o wide

echo -e "\n=== PVC Status ==="
PVC_URI=$(kubectl get basemodel $MODEL_NAME -n $NAMESPACE -o jsonpath='{.spec.storage.storageUri}')
PVC_NAME=$(echo $PVC_URI | sed 's/.*pvc:\/\/\([^\/]*\).*/\1/')
kubectl get pvc $PVC_NAME -n $NAMESPACE -o wide

echo -e "\n=== Recent Events ==="
kubectl get events --sort-by='.lastTimestamp' | grep -E "$MODEL_NAME|$PVC_NAME" | tail -5

echo -e "\n=== Metadata Jobs ==="
kubectl get jobs -n $NAMESPACE -l "app.kubernetes.io/component=metadata-extraction"
```

### PVC Content Explorer

```bash
# Interactive PVC debugging
kubectl run pvc-debug-$(date +%s) --rm -i --tty --image=alpine \
  --overrides='{
    "spec": {
      "volumes": [{"name":"pvc","persistentVolumeClaim":{"claimName":"<PVC-NAME>"}}],
      "containers": [{
        "name":"debug",
        "image":"alpine",
        "volumeMounts":[{"mountPath":"/models","name":"pvc"}],
        "command":["sh","-c","apk add --no-cache file && sh"]
      }]
    }
  }' \
  --namespace=<NAMESPACE>

# Inside pod:
# ls -la /models/
# find /models -name "config.json"
# file /models/*/config.json
```

## Error Quick Reference

| Error                           | Component    | Fix                             |
| ------------------------------- | ------------ | ------------------------------- |
| `PVC not found`                 | Controller   | Create PVC or fix URI           |
| `PVC not bound`                 | Storage      | Check provisioner/storage class |
| `config.json not found`         | Metadata Job | Fix path or skip parsing        |
| `Multi-Attach error`            | Kubernetes   | Use RWX or single replica       |
| `Volume node affinity conflict` | Scheduler    | Check PV topology               |
| `Permission denied`             | Metadata Job | Fix file permissions            |

## Prevention Checklist

Before creating BaseModel:

- [ ] PVC exists and is bound
- [ ] Model files at correct path with config.json
- [ ] Appropriate access mode (RWX for sharing, RWO for performance)
- [ ] Storage class supports required performance
- [ ] RBAC permissions configured

## Related Documentation

- [PVC Storage User Guide](/ome/docs/user-guide/storage/pvc-storage/) - Usage instructions
- [PVC Storage Architecture](/ome/docs/architecture/pvc-storage-flow/) - Technical details
- [Storage Types Reference](/ome/docs/reference/storage-types/) - Complete API spec
