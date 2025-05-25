---
title: "ReplicationJob"
linkTitle: "ReplicationJob"
weight: 90
description: >
  Understanding replication jobs for copying models and data across storage systems
---

ReplicationJob provides a mechanism to copy models, datasets, and other AI/ML artifacts between different storage systems. It enables data distribution, backup, migration, and synchronization across multiple storage backends.

## Overview

ReplicationJob enables you to:

- **Copy Models**: Replicate model artifacts between different storage systems
- **Distribute Datasets**: Copy training and validation datasets to multiple locations
- **Backup Data**: Create backups of critical AI/ML assets
- **Migrate Storage**: Move data between storage providers or regions
- **Synchronize**: Keep multiple storage locations in sync with latest artifacts

## ReplicationJob Specification

### Basic Structure

```yaml
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: model-backup-replication
  namespace: production
spec:
  source:
    storageUri: oci://source-ns/source-bucket/models/llama-3-8b/
    parameters:
      region: us-phoenix-1
      authentication: instance_principal
  destination:
    storageUri: oci://backup-ns/backup-bucket/models/llama-3-8b/
    parameters:
      region: us-ashburn-1
      authentication: instance_principal
  compartmentID: ocid1.compartment.oc1..xxxxx
```

### Key Components

#### Source Configuration
Defines where to copy data from:

```yaml
spec:
  source:
    storageUri: "oci://namespace/bucket/path/"
    parameters:
      region: "us-phoenix-1"
      authentication: "instance_principal"
      encryption: "oci-kms"
      storage_tier: "standard"
    nodeSelector:
      zone: "us-phoenix-1a"         # Prefer nodes close to source
```

#### Destination Configuration
Defines where to copy data to:

```yaml
spec:
  destination:
    storageUri: "oci://target-ns/target-bucket/backup/"
    parameters:
      region: "us-ashburn-1"
      authentication: "instance_principal"
      encryption: "oci-kms"
      storage_tier: "archive"       # Use cheaper storage for backups
    nodeSelector:
      zone: "us-ashburn-1a"
```

## Storage Backend Support

### OCI Object Storage to OCI Object Storage
Cross-region replication within OCI:

```yaml
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: cross-region-model-replication
spec:
  source:
    storageUri: oci://prod-ns/models-bucket/gpt-4-turbo/
    parameters:
      region: us-phoenix-1
      authentication: instance_principal
  destination:
    storageUri: oci://dr-ns/models-backup-bucket/gpt-4-turbo/
    parameters:
      region: us-ashburn-1
      authentication: instance_principal
      storage_tier: infrequent_access
```

### OCI to External Storage
Replicate to external storage providers:

```yaml
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: external-backup-replication
spec:
  source:
    storageUri: oci://prod-ns/models/critical-model/
    parameters:
      region: us-phoenix-1
      authentication: instance_principal
  destination:
    storageUri: s3://external-backup-bucket/ome-models/critical-model/
    parameters:
      region: us-west-2
      access_key_id_secret: aws-credentials
      secret_access_key_secret: aws-credentials
```

### PVC to Object Storage
Move data from persistent volumes to object storage:

```yaml
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: pvc-to-object-storage
spec:
  source:
    storageUri: pvc://training-data-pvc/completed-experiments/
    parameters:
      mount_path: /mnt/training-data
  destination:
    storageUri: oci://archive-ns/experiments-archive/
    parameters:
      region: us-phoenix-1
      authentication: instance_principal
      storage_tier: archive
```

## Replication Scenarios

### Model Distribution
Distribute models to multiple regions for serving:

```yaml
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: model-distribution-us-west
  labels:
    purpose: distribution
    model: llama-3-70b
    target-region: us-west
spec:
  source:
    storageUri: oci://central-models/models/llama-3-70b/
    parameters:
      region: us-phoenix-1
  destination:
    storageUri: oci://us-west-models/models/llama-3-70b/
    parameters:
      region: us-west-1
      storage_tier: standard        # Fast access for serving
---
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: model-distribution-europe
  labels:
    purpose: distribution
    model: llama-3-70b
    target-region: europe
spec:
  source:
    storageUri: oci://central-models/models/llama-3-70b/
    parameters:
      region: us-phoenix-1
  destination:
    storageUri: oci://eu-models/models/llama-3-70b/
    parameters:
      region: eu-frankfurt-1
      storage_tier: standard
```

### Dataset Synchronization
Keep training datasets synchronized across multiple locations:

```yaml
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: dataset-sync-to-training-region
  labels:
    purpose: synchronization
    dataset: large-language-corpus
spec:
  source:
    storageUri: oci://master-datasets/training-data/llm-corpus-v2/
    parameters:
      region: us-phoenix-1
      authentication: instance_principal
  destination:
    storageUri: oci://training-datasets/training-data/llm-corpus-v2/
    parameters:
      region: us-ashburn-1              # Close to training cluster
      authentication: instance_principal
      storage_tier: standard
```

### Backup and Archival
Create backups of critical model artifacts:

```yaml
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: daily-model-backup
  labels:
    purpose: backup
    schedule: daily
    retention: long-term
spec:
  source:
    storageUri: oci://production-models/active-models/
    parameters:
      region: us-phoenix-1
      authentication: instance_principal
  destination:
    storageUri: oci://backup-vault/model-backups/$(date +%Y-%m-%d)/
    parameters:
      region: us-ashburn-1
      authentication: instance_principal
      storage_tier: archive          # Cost-effective long-term storage
      encryption: customer_managed
```

### Migration Projects
Migrate data between storage systems:

```yaml
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: legacy-storage-migration
  labels:
    purpose: migration
    project: storage-modernization
spec:
  source:
    storageUri: nfs://legacy-nfs-server/models/
    parameters:
      mount_options: "vers=4,rsize=1048576,wsize=1048576"
  destination:
    storageUri: oci://modern-storage/migrated-models/
    parameters:
      region: us-phoenix-1
      authentication: instance_principal
      storage_tier: standard
```

## Status and Monitoring

### Job Lifecycle
Track replication job progress:

```yaml
status:
  status: Running                    # Pending, Running, Completed, Failed, Suspended
  startTime: "2024-01-15T10:00:00Z"
  lastReconcileTime: "2024-01-15T10:30:00Z"
  retryCount: 0
  conditions:
  - type: Started
    status: "True"
    reason: "JobStarted"
    message: "Replication job has started successfully"
  - type: SourceAccessible
    status: "True"
    reason: "SourceReachable"
    message: "Source storage is accessible"
  - type: DestinationReady
    status: "True"
    reason: "DestinationWritable"
    message: "Destination storage is ready for writes"
```

### Progress Tracking
Monitor replication progress:

```yaml
status:
  status: Running
  message: "Copied 15.2GB of 50.8GB (30% complete)"
  conditions:
  - type: InProgress
    status: "True"
    reason: "DataTransferring"
    message: "Currently transferring data at 120 MB/s"
  - type: HealthCheck
    status: "True"
    reason: "TransferHealthy"
    message: "Data transfer is proceeding without errors"
```

### Completion Status
Track successful completion:

```yaml
status:
  status: Completed
  startTime: "2024-01-15T10:00:00Z"
  completionTime: "2024-01-15T12:45:00Z"
  message: "Successfully replicated 50.8GB in 2h 45m"
  conditions:
  - type: Completed
    status: "True"
    reason: "TransferSuccessful"
    message: "All data transferred successfully"
  - type: Verified
    status: "True"
    reason: "ChecksumValidated"
    message: "Data integrity verified"
```

## Advanced Configuration

### Large File Optimization
Configure for large model files:

```yaml
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: large-model-replication
  annotations:
    ome.io/transfer-optimization: large-files
    ome.io/parallel-streams: "8"
    ome.io/chunk-size: "100MB"
spec:
  source:
    storageUri: oci://models/large-models/175b-model/
    parameters:
      region: us-phoenix-1
      multipart_threshold: "100MB"
      multipart_chunksize: "50MB"
  destination:
    storageUri: oci://distributed-models/large-models/175b-model/
    parameters:
      region: us-ashburn-1
      multipart_threshold: "100MB"
      multipart_chunksize: "50MB"
```

### Incremental Replication
Replicate only changed files:

```yaml
apiVersion: ome.io/v1beta1
kind: ReplicationJob
metadata:
  name: incremental-dataset-sync
  annotations:
    ome.io/replication-mode: incremental
    ome.io/checksum-verification: enabled
spec:
  source:
    storageUri: oci://live-datasets/training-data/
    parameters:
      sync_mode: incremental
      checksum_algorithm: sha256
  destination:
    storageUri: oci://backup-datasets/training-data/
    parameters:
      preserve_timestamps: true
      verify_after_copy: true
```

### Scheduled Replication
Automated replication with CronJob:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: nightly-model-backup
spec:
  schedule: "0 2 * * *"              # Run at 2 AM daily
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: replication-job
            image: ome-replication-controller:latest
            env:
            - name: REPLICATION_JOB_SPEC
              value: |
                apiVersion: ome.io/v1beta1
                kind: ReplicationJob
                metadata:
                  name: nightly-backup-$(date +%Y%m%d)
                spec:
                  source:
                    storageUri: oci://production-models/active/
                  destination:
                    storageUri: oci://backup-vault/nightly/$(date +%Y%m%d)/
          restartPolicy: OnFailure
```

## Best Practices

### Performance Optimization
```yaml
metadata:
  name: optimized-replication
  annotations:
    ome.io/performance-tier: high
    ome.io/parallel-transfers: "16"
    ome.io/bandwidth-limit: "1Gbps"
spec:
  source:
    storageUri: oci://source/large-dataset/
    parameters:
      read_buffer_size: "64MB"
      connection_pool_size: "20"
  destination:
    storageUri: oci://destination/large-dataset/
    parameters:
      write_buffer_size: "64MB"
      upload_concurrency: "10"
```

### Cost Optimization
```yaml
metadata:
  name: cost-optimized-backup
  labels:
    cost-tier: archive
    priority: low
spec:
  source:
    storageUri: oci://production/models/
  destination:
    storageUri: oci://archive/models/
    parameters:
      storage_tier: archive         # Use cheapest storage tier
      compression: enabled          # Compress to save space
      deduplication: enabled        # Remove duplicates
```

### Error Handling
```yaml
metadata:
  name: resilient-replication
  annotations:
    ome.io/retry-policy: exponential-backoff
    ome.io/max-retries: "5"
spec:
  source:
    storageUri: oci://source/critical-data/
    parameters:
      timeout: 3600                 # 1 hour timeout
      retry_on_error: true
  destination:
    storageUri: oci://backup/critical-data/
    parameters:
      verify_integrity: true
      atomic_operations: true
```

## Monitoring and Observability

### Metrics Collection
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: replication-monitoring
data:
  prometheus-rules.yaml: |
    groups:
    - name: replication-job-alerts
      rules:
      - alert: ReplicationJobFailed
        expr: replication_job_status == "Failed"
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Replication job {{ $labels.job_name }} has failed"
      - alert: ReplicationJobSlow
        expr: (time() - replication_job_start_time) > 7200  # 2 hours
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Replication job {{ $labels.job_name }} is running slowly"
```

### Log Analysis
```bash
# Monitor replication progress
kubectl logs -f replication-job-pod

# Check job events
kubectl describe replicationjob job-name

# View transfer statistics
kubectl get replicationjob job-name -o jsonpath='{.status.message}'
```

## Troubleshooting

### Common Issues

**Authentication Failures**
```bash
# Check credentials
kubectl describe secret storage-credentials

# Verify service account permissions
kubectl auth can-i get secrets --as=system:serviceaccount:namespace:replication-sa

# Test connectivity
kubectl exec test-pod -- curl -I storage-endpoint
```

**Transfer Timeouts**
```bash
# Check network connectivity
kubectl exec replication-pod -- ping destination-endpoint

# Verify bandwidth limits
kubectl describe replicationjob job-name | grep -i timeout

# Check resource limits
kubectl describe pod replication-pod | grep -A 5 Limits
```

**Storage Access Errors**
```bash
# Verify storage URI format
kubectl get replicationjob job-name -o jsonpath='{.spec.source.storageUri}'

# Check storage permissions
kubectl logs replication-pod | grep -i "permission denied"

# Test storage accessibility
kubectl exec test-pod -- curl -I storage-uri
```

### Debugging Commands

```bash
# List all replication jobs
kubectl get replicationjobs -A

# Get job details
kubectl describe replicationjob job-name

# Check job status
kubectl get replicationjob job-name -o jsonpath='{.status.status}'

# Monitor job progress
kubectl get replicationjob job-name -o jsonpath='{.status.message}'

# Check conditions
kubectl get replicationjob job-name -o jsonpath='{.status.conditions[*]}'
```

## Security Considerations

### Access Control
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: replication-job-operator
rules:
- apiGroups: ["ome.io"]
  resources: ["replicationjobs"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list"]
```

### Data Encryption
```yaml
spec:
  source:
    storageUri: oci://encrypted-source/data/
    parameters:
      encryption: customer_managed
      kms_key_id: ocid1.key.oc1..xxxxx
  destination:
    storageUri: oci://encrypted-destination/data/
    parameters:
      encryption: customer_managed
      kms_key_id: ocid1.key.oc1..yyyyy
```

### Network Security
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: replication-job-network-policy
spec:
  podSelector:
    matchLabels:
      app: replication-job
  policyTypes:
  - Egress
  egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: storage-namespace
    ports:
    - protocol: TCP
      port: 443
``` 