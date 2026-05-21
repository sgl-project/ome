# OEP-0011: Model-Agent Ready Artifact Reconciliation

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [Ready Artifact Model](#ready-artifact-model)
  - [Periodic Ready Artifact Check](#periodic-ready-artifact-check)
  - [Validation Sources](#validation-sources)
  - [Validation Policy](#validation-policy)
  - [Failure Behavior](#failure-behavior)
  - [Work Split](#work-split)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
  - [API Specifications](#api-specifications)
  - [Model-Agent Configuration](#model-agent-configuration)
  - [Node-Scoped Status Record](#node-scoped-status-record)
  - [Validation Modes](#validation-modes)
  - [Storage Provider Behavior](#storage-provider-behavior)
  - [Reconciliation Flow](#reconciliation-flow)
  - [Readiness, Status, and Failure Behavior](#readiness-status-and-failure-behavior)
  - [Observability](#observability)
  - [Validation Safety](#validation-safety)
  - [Backward Compatibility](#backward-compatibility)
  - [Implementation Plan](#implementation-plan)
  - [Test Plan](#test-plan)
    - [Prerequisite Testing Updates](#prerequisite-testing-updates)
    - [Unit Tests](#unit-tests)
    - [Integration Tests](#integration-tests)
  - [Graduation Criteria](#graduation-criteria)
- [Implementation History](#implementation-history)
- [Drawbacks](#drawbacks)
- [Alternatives](#alternatives)
<!-- /toc -->

## Summary

This OEP proposes periodic Ready artifact reconciliation in model-agent. After
model-agent initially downloads or accepts a model artifact and marks that model
Ready on a node, the agent should be able to re-check that the local artifact
still exists and still matches the current `storageUri` and `storage.path` for
the `BaseModel` or `ClusterBaseModel` being advertised as Ready on that node.

The initial alpha is intentionally narrow. It proposes adding model-agent
process configuration, node-scoped status metadata, validation helpers,
metrics, and a background reconciliation loop.

The proposed reconciler validates only models that are currently Ready on the
local node. When it finds a concrete artifact failure, it would stop
advertising that node as Ready for the affected model by updating the
model-ready node label and the node-scoped model status ConfigMap. The existing
`BaseModel` and `ClusterBaseModel` controllers can then remove the node from
`status.nodesReady` and expose it in `status.nodesFailed`.

## Motivation

Today, model-agent validates some artifacts during the initial download path and
then marks the model Ready on the node. After that point, OME may keep
advertising the model as Ready even if files are later deleted, truncated,
replaced, or damaged by disk cleanup, manual maintenance, filesystem problems,
or an interrupted update.

InferenceService scheduling relies on model-ready node labels and model status.
If a node continues to advertise Ready for a model whose local files are no
longer valid, serving pods can be scheduled onto that node and fail later during
engine startup. The resulting error is far from the actual fault boundary:
model-agent's node-local Ready signal is stale.

There is a related freshness problem when a model keeps the same Kubernetes name
but its storage URI or local path changes. A node-scoped status record that does
not identify the artifact source can make an older local artifact look Ready for
the current model spec. Ready reconciliation should therefore check storage
identity, not just model name.

### Goals

1. Add a periodic model-agent reconciliation loop for artifacts already marked
   Ready on the local node.
2. Validate only current `BaseModel` and `ClusterBaseModel` entries that are
   Ready in the node-scoped model status ConfigMap.
3. Record storage identity in node-scoped status so stale Ready entries are not
   counted for a newer model spec that points at a different artifact.
4. Reuse existing OCI Object Storage validation logic where it fits.
5. Validate Hugging Face and local artifacts using
   known required config files, known model files, file size, and optional
   node-local baseline manifests.
6. Support a low-cost Basic mode and an optional Deep checksum mode.
7. On concrete validation failure, update the model-ready node label and
   node-scoped ConfigMap so the node is no longer selected as Ready.
8. Emit logs, events, and metrics for check result, reason, duration, and
   checked bytes.
9. Implement alpha using internal model-agent metadata and configuration,
   without requiring changes to existing model specs.

### Non-Goals

1. Add public model-spec fields for artifact integrity policy in alpha.
2. Prove model content quality, license compliance, malware status, or approval
   for a workload.
3. Treat Hugging Face revision SHAs as file digests.
4. Delete or redownload failed artifacts as part of the periodic check.
5. Require existing local artifacts to already have checksum manifests.
6. Change serving runtime, InferenceService, or scheduling APIs.

## Proposal

### Ready Artifact Model

For this OEP, a Ready artifact is the node-local artifact that model-agent is
advertising for one `BaseModel` or `ClusterBaseModel` on one Kubernetes node.
The Ready signal has two pieces:

1. the model-ready node label managed by model-agent; and
2. the node-scoped model status ConfigMap entry used by model controllers.

The status entry should describe both state and artifact identity:

- model key;
- status: `Ready`, `Updating`, `Failed`, or `Deleted`;
- existing model config metadata;
- observed model generation or storage identity hash, when available;
- source URI, when available;
- local storage path, when available;
- last validation result, reason, checked time, files checked, and bytes
  checked.

The storage identity fields are not a new user API. They are model-agent's
record of which artifact source and path were validated when the node claimed
Ready. Controllers can then ignore stale status records whose identity does not
match the current model spec.

### Periodic Ready Artifact Check

The alpha reconciler runs inside each model-agent DaemonSet pod. It wakes on a
configurable interval after a startup jitter and checks only models that are
currently Ready on that node.

Each cycle:

1. reads the local node-scoped model status ConfigMap;
2. lists current `BaseModel` and `ClusterBaseModel` specs;
3. builds an index from model ConfigMap keys to current model specs;
4. skips entries that are not `Ready`;
5. skips entries that no longer have a current `BaseModel` or
   `ClusterBaseModel`;
6. skips or clears stale entries whose storage identity does not match the
   current model spec;
7. resolves the local artifact path using the same storage logic as download
   and reuse paths;
8. runs the configured validation mode;
9. records success, failure, or inconclusive result; and
10. marks the node Failed only when the current Ready entry is still current
    and validation found a concrete artifact failure.

The final write is guarded by a fresh read of the model ConfigMap entry and
current model storage identity. This prevents a slow integrity check from
marking a node Failed after a newer download has already replaced the artifact
or changed the model status.

### Validation Sources

Alpha uses sources that model-agent already owns or can derive locally:

1. **Current model spec.** Provides storage URI, local path, download policy,
   and storage provider type.
2. **Node-scoped status ConfigMap.** Provides current Ready state, storage
   identity, config metadata, prior validation metadata, and artifact reuse
   metadata.
3. **Local filesystem.** Provides path existence, file type, file size,
   modification time, and optional file digest.
4. **OCI Object Storage metadata.** Existing object-size and MD5 checks can be
   reused where object metadata is available.
5. **Node-local baseline manifest.** For providers without durable external
   checksum metadata, model-agent may write a baseline after a successful
   initial download. The baseline is useful for detecting later local drift; it
   is not a proof that the original bytes were correct.

### Validation Policy

Alpha exposes model-agent process configuration, not per-model spec fields:

- `Disabled`: do not run periodic Ready artifact checks.
- `Basic`: verify low-cost invariants such as path existence, required config
  files, known model files, and file sizes where metadata exists.
- `Deep`: run Basic checks and also compute file checksums from a baseline or
  provider metadata when available.

The initial alpha default should preserve existing behavior. Operators enable
the reconciler by configuring a positive interval and a validation mode. A later
release can revisit defaults after operational experience.

The result model is tri-state:

- `Valid`: the local artifact satisfies the configured checks.
- `Invalid`: model-agent found a concrete local artifact failure.
- `Inconclusive`: model-agent could not prove validity or invalidity, usually
  because metadata is missing or the storage type is unsupported.

Only `Invalid` removes Ready in alpha. `Inconclusive` is logged and counted but
does not fail the node by default.

### Failure Behavior

When validation returns `Invalid`, model-agent:

1. rechecks that the ConfigMap entry is still Ready and still matches the
   current model storage identity;
2. updates the model-ready node label to `Failed`;
3. updates the node-scoped ConfigMap entry to `Failed`;
4. records validation reason and time in the ConfigMap entry;
5. emits a Kubernetes event when event recording is available;
6. records failure metrics; and
7. leaves artifact deletion or redownload to existing model-agent flows or a
   future remediation policy.

This is intended to stop scheduling from selecting that node for the affected
model without introducing a destructive cleanup loop.

### Work Split

This OEP is intended to split the original large implementation into reviewable
PRs:

1. **Storage identity bugfix.** Persist storage URI/path in node ConfigMap
   entries, ignore stale Ready entries in controllers, and refresh downloads
   when storage identity changes.
2. **Validation primitives.** Add reusable validators and result types for OCI,
   local filesystem, Hugging Face/local baseline manifests, and storage-path
   resolution.
3. **Ready artifact reconciler.** Add the periodic loop, race guards, status
   transitions, and controller propagation tests.
4. **Configuration, metrics, and docs.** Wire model-agent flags/config, static
   manifests, metrics documentation, operator guidance, and Helm values if a
   model-agent chart is added or identified.
5. **Future public integrity APIs.** Add user-declared manifests or digest
   policy only after the reconciliation behavior is accepted and measured.

### Risks and Mitigations

**Risk 1: Checks create excessive disk I/O**

- *Mitigation:* Basic is the default mode when the reconciler is enabled, Deep
  mode is separately configured, and both modes use interval, jitter,
  concurrency, and byte-budget limits.

**Risk 2: Legacy artifacts lack enough metadata**

- *Mitigation:* Missing metadata returns `Inconclusive`, not `Invalid`, unless a
  future explicit policy changes that behavior. New downloads can write a
  baseline for future drift detection.

**Risk 3: Baseline manifests are mistaken for source-of-truth integrity**

- *Mitigation:* Documentation states that node-local baselines detect later
  local changes only. They do not prove that the original remote bytes were
  correct.

**Risk 4: A slow check marks a newer download Failed**

- *Mitigation:* The reconciler rechecks ConfigMap state and storage identity
  before demoting Ready.

**Risk 5: Unsupported storage types are treated as failures**

- *Mitigation:* Unsupported or externally managed storage returns
  `Inconclusive` by default.

## Design Details

### API Specifications

Alpha does not add public model-spec fields. It only proposes internal
model-agent status metadata in the node-scoped ConfigMap and model-agent
process configuration.

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

Field meanings:

1. `enabled` starts the reconciliation loop.
2. `interval` controls Basic check cadence.
3. `startup_jitter` spreads checks across nodes after daemon restart.
4. `mode` selects `Basic` or `Deep`.
5. `deep_interval` controls checksum-heavy validation cadence when Deep mode is
   enabled.
6. `max_concurrent_checks` limits local I/O pressure.
7. `max_bytes_per_second` optionally rate-limits deep hashing.
8. `fail_on_inconclusive` remains false by default in alpha.
9. `baseline.enabled` writes a node-local manifest after successful downloads
   for providers without external checksum metadata.
10. `baseline.include_hashes` controls whether new baselines include SHA256
    hashes or only paths and sizes.

### Node-Scoped Status Record

The node-scoped ConfigMap stores one JSON entry per model key. The current
entry already includes `name`, `status`, `config`, and `progress`. Alpha should
extend that entry with storage identity and last-validation metadata:

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
    Mode               string `json:"mode,omitempty"`
    Result             string `json:"result,omitempty"`
    Reason             string `json:"reason,omitempty"`
    LastCheckedTime    string `json:"lastCheckedTime,omitempty"`
    FilesChecked       int64  `json:"filesChecked,omitempty"`
    BytesChecked       int64  `json:"bytesChecked,omitempty"`
    BaselinePath       string `json:"baselinePath,omitempty"`
    BaselineHash       string `json:"baselineHash,omitempty"`
    ObservedGeneration int64  `json:"observedGeneration,omitempty"`
}
```

`storageUri` and `storagePath` let controllers and model-agent determine
whether a Ready record belongs to the current model spec. `validation` is
observational and should not become the source of truth for required integrity.

### Validation Modes

Basic mode checks:

1. resolved artifact path exists;
2. path is a directory or file as expected by storage type;
3. known required configuration file exists, such as `config.json` or
   `model_index.json` when applicable;
4. known model data files exist, using current metadata, baseline, or
   provider listing when available;
5. file sizes match baseline or provider metadata when available; and
6. symlinks do not escape the artifact root.

Deep mode checks:

1. all Basic mode checks;
2. SHA256 hashes from the node-local baseline when present;
3. provider checksum metadata when available; and
4. optional artifact-set hash for new baselines.

Deep mode is the alpha mode intended to catch same-size corruption for
providers without external checksum metadata, when a baseline includes hashes.

### Storage Provider Behavior

**OCI Object Storage**

Reuse existing local-copy validation logic where possible. When object size and
MD5 are available, Basic mode can validate size and MD5. If custom SHA256
metadata exists, Deep mode should prefer it. Missing object metadata returns
`Inconclusive` unless a future policy says otherwise.

**Hugging Face**

Use the local download result to write a baseline manifest after a successful
download. Basic mode checks file presence and size. Deep mode checks SHA256
only when the baseline includes hashes. Hugging Face repository revision SHA is
recordable metadata, but it is not treated as a file digest in alpha.

**Local filesystem**

There is no remote source of truth. Basic mode checks known required files and
any existing baseline. Deep mode can detect drift from a prior baseline, but it
cannot prove the original files were correct.

**PVC**

PVC-backed artifacts are outside model-agent's download ownership in alpha.
Return `Inconclusive` unless model-agent has a local baseline it owns.

**Vendor and unsupported storage**

Return `Inconclusive` by default and record metrics.

### Reconciliation Flow

```text
Start model-agent
  wait startup_jitter
  every interval:
    read node ConfigMap
    list current BaseModels and ClusterBaseModels
    for each ConfigMap entry:
      skip if status != Ready
      find current model spec
      skip/delete stale entry if model no longer exists
      skip if storage identity does not match current spec
      resolve artifact path
      run Basic or Deep validator
      record validation result
      if result == Invalid:
        re-read ConfigMap entry and current model spec
        if still Ready and identity still matches:
          set node label to Failed
          set ConfigMap status to Failed
```

The reconciler should not block normal download workers. It should share
ConfigMap update helpers with existing model-agent code so label and ConfigMap
state remain consistent.

### Readiness, Status, and Failure Behavior

The `BaseModel` and `ClusterBaseModel` controllers already compute
`nodesReady` and `nodesFailed` from node-scoped ConfigMap entries. This OEP
proposes updating them to ignore stale entries whose storage identity no longer
matches the current model spec. Once model-agent marks the current entry
Failed, the controller should remove that node from `nodesReady` and add it to
`nodesFailed`.

No new model status fields are required in alpha.

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

`model_agent_ready_artifact_check_results_total` is a counter of completed
checks partitioned by outcome. `result` should use the bounded values `valid`,
`invalid`, and `inconclusive`. `reason` should use a bounded reason enum and
`none` for valid checks so this metric is also the source for failure and
inconclusive counts.

Events:

1. `ReadyArtifactValidated`
2. `ReadyArtifactValidationFailed`
3. `ReadyArtifactValidationInconclusive`

Logs should include model identity, node, storage type, mode, result, reason,
path, files checked, bytes checked, and duration. Logs must not include tokens,
authorization headers, pre-authenticated request URLs, or secret values.

### Validation Safety

1. Baseline paths must be relative to the artifact root.
2. Absolute paths, `..` segments, and paths resolving outside the artifact root
   must be rejected.
3. Symlinks must not allow validation to escape the artifact root.
4. Checksum and path errors must produce bounded log messages.
5. The reconciler must not require source-bucket write permissions.

### Backward Compatibility

The alpha design is intended to be backward compatible:

1. Existing `BaseModel` and `ClusterBaseModel` specs do not need new fields.
2. The alpha reconciler is disabled unless configured.
3. With `fail_on_inconclusive: false`, missing metadata does not demote Ready.
4. Existing download-time OCI validation remains available.
5. Existing `downloadPolicy` behavior is preserved.
6. Legacy ConfigMap entries without storage identity can be accepted during
   upgrade and can be rewritten with identity on the next current update.

### Implementation Plan

1. Add storage identity fields to node-scoped ConfigMap entries and in-memory
   cache.
2. Update model controllers to ignore stale ConfigMap entries whose storage
   identity does not match the current model spec.
3. Refresh model-agent downloads when storage URI or path changes on a model
   that still targets the node.
4. Add validation result types: `Valid`, `Invalid`, and `Inconclusive`.
5. Add storage-path resolution helpers shared by download and validation paths.
6. Reuse OCI local-copy validation and expose structured failure reasons.
7. Add filesystem and baseline validators for Hugging Face and local storage.
8. Write node-local baseline manifests after successful downloads when enabled.
9. Add the periodic Ready artifact reconciler with interval, jitter, and
   concurrency limits.
10. Add race guards before demoting Ready to Failed.
11. Add metrics, events, and logs.
12. Add model-agent configuration, static manifests, docs, and Helm values if a
    model-agent chart is added or identified.

### Test Plan

[x] I/we understand that component owners may require updates to existing tests
before accepting changes necessary for this enhancement.

#### Prerequisite Testing Updates

Existing tests should cover storage identity before periodic reconciliation is
enabled:

1. ConfigMap entries store `storageUri` and `storagePath`.
2. Controllers skip stale Ready entries for a changed storage identity.
3. Model-agent refreshes downloads when storage URI or path changes.
4. Superseded downloads cannot publish stale Ready after a newer identity wins.

#### Unit Tests

- Validation result classification for Valid, Invalid, and Inconclusive.
- OCI local-copy validation with missing file, size mismatch, MD5 mismatch,
  metadata unavailable, and valid copy.
- Filesystem validator with missing artifact path, missing known config file,
  missing known model file, size mismatch, symlink escape, and valid directory.
- Baseline manifest read/write, path normalization, duplicate paths, invalid
  JSON, missing file, size mismatch, hash mismatch, and stable artifact-set
  digest.
- Reconciler skips non-Ready entries.
- Reconciler skips stale storage identity.
- Reconciler handles missing current `BaseModel` or `ClusterBaseModel`.
- Reconciler marks current Ready entry Failed on concrete invalid result.
- Reconciler does not fail on Inconclusive by default.
- Reconciler rechecks identity before demoting Ready.
- Metrics labels and reason mapping remain bounded.

#### Integration Tests

1. Download an OCI-backed model, mark it Ready, delete one local file, and
   verify the node moves from Ready to Failed.
2. Change a model storage path under the same model name and verify stale Ready
   status is ignored.
3. Enable Basic mode for a Hugging Face-backed model with a baseline, truncate a
   file, and verify failure.
4. Enable Deep mode with baseline hashes, modify file contents without changing
   size, and verify hash mismatch.
5. Use a legacy Ready entry without baseline metadata and verify the result is
   Inconclusive rather than Failed.
6. Disable the reconciler and verify existing behavior is preserved.

### Graduation Criteria

#### Alpha

1. Storage identity prevents stale Ready entries from satisfying changed model
   specs.
2. The Ready artifact reconciler is configurable and disabled by default.
3. Basic mode detects missing paths, missing known required files, and size
   mismatch where metadata is available.
4. Concrete invalid results move nodes from Ready to Failed.
5. Inconclusive results are observable and non-disruptive by default.
6. Metrics, logs, and events expose actionable reasons.

#### Beta

1. Operators have documented rollout guidance for enabling Basic mode in
   selected environments.
2. Deep mode has documented I/O controls and performance measurements.
3. Baseline manifest behavior is documented for Hugging Face and local storage.
4. OCI validation exposes structured reasons and reuses existing checks.
5. Upgrade behavior for legacy ConfigMap entries is documented.

#### Stable

1. Production-like environments have validated Ready artifact reconciliation
   with clear rollback guidance.
2. Default behavior is revisited with measured false-positive and performance
   data.
3. Any public digest policy is covered by a separate accepted OEP.

## Implementation History

- 2026-05-21: Initial OEP drafted.

## Drawbacks

Periodic checks add background disk and object-store work to every model-agent
pod where they are enabled. Basic checks are cheap, but Deep checks can read
large model files and compete with serving workloads for I/O bandwidth.

Node-local baseline manifests can detect drift only after the baseline is
created. They do not prove that pre-existing legacy artifacts were correct at
the time they became Ready.

The design adds internal ConfigMap fields and model-agent configuration that
operators need to understand.

## Alternatives

### Do Nothing After Initial Download

OME could continue relying only on initial download checks.

This leaves nodes advertising Ready after files are deleted or corrupted
post-download, which is the primary gap this OEP addresses.

### Add Public Digest Policy First

OME could start by adding `StorageSpec` fields for manifests, digests, and
required verification policy.

That may be useful later, but it is larger than the immediate Ready artifact
reconciliation problem. Alpha can improve node-local correctness with existing
metadata and internal status before committing to public model-spec fields.

### Always Run Deep Hashing

OME could compute checksums for every Ready artifact on every interval.

This can catch same-size corruption when comparison hashes exist, but it can be
expensive for large models and many nodes. The proposal separates Basic and
Deep modes so operators can choose an I/O budget.

### Mark Inconclusive as Failed

OME could fail any Ready artifact that cannot be fully validated.

That would make rollout risky for legacy artifacts and storage providers
without complete metadata. Alpha records Inconclusive results without demoting
Ready by default.

### Let Serving Pods Validate Artifacts

Serving containers could validate model files before loading them.

This duplicates logic across runtimes and still schedules pods onto bad nodes.
Model-agent owns node-local artifact readiness, so it is the right alpha
enforcement point.
