# OEP-0011: Model-Agent Ready Artifact Reconciliation

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [Ready Artifact State](#ready-artifact-state)
  - [Periodic Reconciliation](#periodic-reconciliation)
  - [Validation Policy](#validation-policy)
  - [Failure Behavior](#failure-behavior)
  - [Work Split](#work-split)
- [Design Details](#design-details)
  - [Model-Agent Configuration](#model-agent-configuration)
  - [Node-Scoped ConfigMap Entry](#node-scoped-configmap-entry)
  - [Provider Behavior](#provider-behavior)
  - [Observability](#observability)
  - [Safety and Compatibility](#safety-and-compatibility)
  - [Test Plan](#test-plan)
  - [Graduation Criteria](#graduation-criteria)
- [Implementation History](#implementation-history)
- [Drawbacks](#drawbacks)
- [Alternatives](#alternatives)
<!-- /toc -->

## Summary

This OEP proposes a model-agent reconciliation loop for models that are already
advertised as Ready on a node. The loop verifies that the local artifact still
exists and still matches the current `storageUri` and `storage.path` for the
`BaseModel` or `ClusterBaseModel` being advertised as Ready.

The alpha is intentionally narrow:

1. store artifact identity in the node-scoped model status ConfigMap;
2. ignore stale Ready entries when a model's storage identity changes;
3. periodically validate already-Ready artifacts on each node; and
4. demote only concrete local artifact failures from Ready to Failed.

Alpha uses internal model-agent metadata and configuration. It does not require
new public model-spec fields.

## Motivation

Today, model-agent validates some artifacts during the initial download path and
then marks the model Ready on the node. After that point, OME may keep
advertising the model as Ready even if files are later altered.

InferenceService scheduling relies on model-ready node labels and model status.
If a node continues to advertise Ready for a model whose local files are no
longer valid, serving pods can be scheduled onto that node and fail later during
engine startup.

A related issue exists when a model keeps the same Kubernetes name but changes
storage URI or local path. Without storage identity in the node-scoped status
record, an older Ready entry can look valid for the newer model spec.

### Goals

1. Add storage identity to node-scoped Ready records.
2. Make controllers ignore stale Ready records whose identity no longer matches
   the current model spec.
3. Add periodic model-agent validation for artifacts already marked Ready on
   the local node.
4. Support Basic validation for missing paths, known required files, and size
   mismatches where metadata exists.
5. Support optional Deep validation for checksum comparison when hashes exist.
6. Move a node from Ready to Failed only for concrete artifact failures.
7. Emit logs, events, and metrics for check result, reason, duration, and
   checked bytes.

### Non-Goals

1. Add new user-facing model spec fields in alpha.
2. Prove model quality, safety, licensing, malware status, or provenance.
3. Automatically delete or redownload failed artifacts from the periodic
   checker.
4. Change serving runtime, InferenceService, or scheduling APIs.

## Proposal

### Ready Artifact State

For this OEP, a Ready artifact is the node-local artifact that model-agent is
advertising for one `BaseModel` or `ClusterBaseModel` on one Kubernetes node.

The Ready signal is made of:

1. the model-ready node label managed by model-agent; and
2. the node-scoped model status ConfigMap entry read by model controllers.

The ConfigMap entry must identify both state and artifact identity:

- model key;
- status: `Ready`, `Updating`, `Failed`, or `Deleted`;
- existing model config metadata;
- source URI, when available;
- local storage path, when available;
- observed model generation or storage identity hash, when available; and
- last validation result, reason, checked time, files checked, and bytes
  checked.

`storageUri` and `storagePath` are the key alpha additions. They let
model-agent and controllers decide whether a Ready record belongs to the
current model spec instead of trusting the model name alone.

### Periodic Reconciliation

Each model-agent pod runs the reconciler on its own node. The loop is
configurable and disabled by default.

Each cycle:

1. reads the node-scoped ConfigMap;
2. lists current `BaseModel` and `ClusterBaseModel` specs;
3. skips entries that are not `Ready`;
4. skips entries whose storage identity does not match the current model spec;
5. validates the local artifact path; and
6. marks the node Failed only if the entry is still current and validation
   returns `Invalid`.

Before demoting Ready to Failed, model-agent re-reads the ConfigMap entry and
checks storage identity again. This prevents a slow validation pass from
failing a newer download or a newer model spec.

### Validation Policy

Alpha uses model-agent process configuration, not per-model public API.

Validation results are tri-state:

- `Valid`: the artifact satisfies the configured checks.
- `Invalid`: model-agent found a concrete local artifact failure.
- `Inconclusive`: model-agent could not prove valid or invalid, usually because
  metadata is missing or the storage type is unsupported.

Only `Invalid` removes Ready in alpha. `Inconclusive` is logged and counted but
does not fail the node by default.

Validation modes:

- `Disabled`: no periodic checks.
- `Basic`: check path existence, known required files, known model files, and
  file sizes where metadata exists.
- `Deep`: run Basic checks and compare checksums when provider metadata or a
  node-local baseline has hashes.

Basic mode catches missing files and many truncated artifacts. Deep mode is the
path for same-size corruption detection, but only when hashes are available and
I/O limits make the check safe to run.

### Failure Behavior

When validation returns `Invalid`, model-agent:

1. confirms the ConfigMap entry is still Ready and still matches the current
   model storage identity;
2. updates the model-ready node label away from Ready for that model;
3. updates the node-scoped ConfigMap entry to `Failed`;
4. records validation reason and time; and
5. emits event, log, and metric signals.

The periodic checker does not delete or redownload artifacts in alpha. It only
stops OME from advertising a bad local artifact as Ready.

### Work Split

This OEP is intended to split the implementation into reviewable PRs:

1. **Storage identity bugfix.**
   Persist `storageUri` and `storagePath` in node ConfigMap entries, ignore
   stale Ready entries in controllers, and refresh downloads when identity
   changes.
2. **Validation primitives.**
   Add reusable validation result types and validators for OCI, filesystem, and
   baseline-manifest checks.
3. **Ready artifact reconciler.**
   Add the periodic loop, race guards, Ready-to-Failed transitions, and
   controller propagation tests.
4. **Configuration, metrics, and docs.**
   Wire model-agent config, manifests, metrics, events, logs, and operator
   guidance.

Future public digest or manifest APIs should be proposed separately after the
internal reconciliation behavior is accepted and measured.

## Design Details

### Model-Agent Configuration

Model-agent should accept process configuration equivalent to:

```yaml
ready_artifact_reconciliation:
  enabled: false
  interval: 10m
  startup_jitter: 2m
  mode: Basic
  deep_interval: 6h
  max_concurrent_checks: 1
  max_bytes_per_second: 0
  fail_on_inconclusive: false
  baseline:
    enabled: true
    include_hashes: false
    manifest_name: .ome/artifact-baseline.json
```

Important defaults:

- `enabled: false` preserves current behavior.
- `fail_on_inconclusive: false` avoids failing legacy artifacts with missing
  metadata.
- Deep checksum work is separately controlled by interval, concurrency, and
  byte-rate limits.

### Node-Scoped ConfigMap Entry

The current `ModelEntry` already includes `name`, `status`, `config`, and
`progress`. Alpha extends it with storage identity and last-validation metadata:

```go
type ModelEntry struct {
    Name        string            `json:"name"`
    Status      ModelStatus       `json:"status"`
    StorageURI  string            `json:"storageUri,omitempty"`
    StoragePath string            `json:"storagePath,omitempty"`
    Config      *ModelConfig      `json:"config,omitempty"`
    Progress    *DownloadProgress `json:"progress,omitempty"`
    Validation  *ValidationStatus `json:"validation,omitempty"`
}

type ValidationStatus struct {
    Mode            string `json:"mode,omitempty"`
    Result          string `json:"result,omitempty"`
    Reason          string `json:"reason,omitempty"`
    LastCheckedTime string `json:"lastCheckedTime,omitempty"`
    FilesChecked    int64  `json:"filesChecked,omitempty"`
    BytesChecked    int64  `json:"bytesChecked,omitempty"`
    BaselinePath    string `json:"baselinePath,omitempty"`
}
```

`validation` is observational. Readiness still comes from the entry status and
the model-ready node label.

### Provider Behavior

Provider handling in alpha:

- **OCI Object Storage:** reuse existing local-copy validation where possible;
  validate object size and MD5 when metadata is available.
- **Hugging Face:** write a node-local baseline after successful downloads;
  Basic checks presence and size; Deep checks SHA256 only when the baseline
  includes hashes.
- **Local filesystem:** validate known required files and any model-agent-owned
  baseline; there is no external source of truth.
- **PVC, vendor, unsupported storage:** return `Inconclusive` unless model-agent
  owns enough metadata to validate safely.

Node-local baselines detect changes after the baseline is written. They do not
prove the original bytes were correct.

### Observability

Metrics:

1. `model_agent_ready_artifact_check_results_total`
   - labels: `model_type`, `namespace`, `name`, `mode`, `storage_type`,
     `result`, `reason`
2. `model_agent_ready_artifact_check_duration_seconds`
   - labels: `model_type`, `namespace`, `name`, `mode`, `storage_type`,
     `result`
3. `model_agent_ready_artifact_bytes_checked_total`
   - labels: `model_type`, `namespace`, `name`, `mode`, `storage_type`

Events:

1. `ReadyArtifactValidated`
2. `ReadyArtifactValidationFailed`
3. `ReadyArtifactValidationInconclusive`

Logs should include model identity, node, storage type, mode, result, reason,
path, files checked, bytes checked, and duration. Logs must not include tokens,
authorization headers, pre-authenticated request URLs, or secret values.

### Safety and Compatibility

Alpha is backward compatible:

1. existing model specs do not need new fields;
2. periodic checks are disabled unless configured;
3. missing metadata returns `Inconclusive` by default;
4. legacy ConfigMap entries without storage identity can be accepted during
   upgrade and rewritten on the next current update; and
5. existing download-time OCI validation and `downloadPolicy` behavior are
   preserved.

Validation must stay inside the artifact root. Baseline paths must be relative,
`..` and absolute paths must be rejected, and symlinks must not allow checks to
escape the artifact directory.

### Test Plan

[x] I/we understand that component owners may require updates to existing tests
before accepting changes necessary for this enhancement.

Storage identity tests:

1. ConfigMap entries store `storageUri` and `storagePath`.
2. Controllers skip stale Ready entries for a changed storage identity.
3. Model-agent refreshes downloads when storage URI or path changes.
4. Superseded downloads cannot publish stale Ready after a newer identity wins.

Validator tests:

1. `Valid`, `Invalid`, and `Inconclusive` classification.
2. OCI validation with missing file, size mismatch, MD5 mismatch, missing
   metadata, and valid copy.
3. Filesystem validation with missing path, missing config, missing model file,
   size mismatch, symlink escape, and valid directory.
4. Baseline manifest read/write, path normalization, size mismatch, hash
   mismatch, and invalid manifest.

Reconciler tests:

1. skip non-Ready entries;
2. skip stale storage identity;
3. mark current Ready entry Failed on concrete invalid result;
4. do not fail on `Inconclusive` by default;
5. recheck identity before demoting Ready; and
6. emit bounded metrics labels and reasons.

Integration tests:

1. delete a file from a Ready artifact and verify node Ready becomes Failed;
2. change storage path under the same model name and verify stale Ready is
   ignored;
3. truncate a baseline-backed Hugging Face file and verify Basic failure; and
4. modify same-size contents with baseline hashes and verify Deep failure.

### Graduation Criteria

Alpha:

1. storage identity prevents stale Ready entries from satisfying changed model
   specs;
2. the reconciler is configurable and disabled by default;
3. Basic mode detects missing paths, missing known required files, and size
   mismatches where metadata exists;
4. invalid results move nodes from Ready to Failed; and
5. metrics, logs, and events expose actionable reasons.

Beta:

1. rollout guidance is documented;
2. Deep mode has I/O controls and performance data;
3. baseline behavior is documented for Hugging Face and local storage; and
4. legacy ConfigMap upgrade behavior is documented.

Stable:

1. production-like environments have validated the reconciler with rollback
   guidance; and
2. defaults are revisited with measured false-positive and performance data.

## Implementation History

- 2026-05-21: Initial OEP drafted.

## Drawbacks

Periodic checks add background disk work to model-agent pods. Deep checks can
read large model files and compete with serving workloads without careful
limits.

Node-local baselines detect drift only after the baseline is created. They do
not prove that pre-existing legacy artifacts were correct when they became
Ready.

## Alternatives

### Do Nothing After Initial Download

This leaves nodes advertising Ready after files are deleted or corrupted.

### Add Public Digest Policy First

Public digest policy may be useful later, but it is larger than the immediate
Ready artifact reconciliation problem. Alpha can improve node-local correctness
with internal metadata first.

### Always Run Deep Hashing

This can catch same-size corruption when comparison hashes exist, but it is too
expensive to make the only mode for large models.

### Mark Inconclusive as Failed

This would make rollout risky for legacy artifacts and storage providers
without complete metadata.

### Let Serving Pods Validate Artifacts

This duplicates validation logic across runtimes and still schedules pods onto
bad nodes. Model-agent owns node-local artifact readiness, so it is the right
alpha enforcement point.
