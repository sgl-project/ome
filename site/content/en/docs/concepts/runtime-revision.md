---
title: "Runtime Revisions and Pinning"
linkTitle: "Runtime Revisions"
weight: 8
description: >
  Pin an InferenceService to a snapshot of its ServingRuntime for stable, roll-forward/rollback-controlled updates.
---

By default an InferenceService always renders its pods from the **live** ServingRuntime it references: whenever the runtime changes, the next reconcile picks up the new spec. Runtime **pinning** lets you decouple an InferenceService from live runtime changes by pinning it to an immutable **snapshot** of the runtime, so updates roll out only when you ask for them.

Snapshots are stored as Kubernetes `ControllerRevision` objects in the OME namespace, are content-addressed (deduplicated by hash), and are garbage-collected on a configurable schedule.

## `autoSync`: live vs. pinned

The behavior is controlled by `spec.runtime.autoSync` on the InferenceService:

| `autoSync` | Behavior |
|------------|----------|
| `true` (default) | Pods re-render from the **live** runtime every reconcile. Runtime edits take effect immediately. |
| `false` | The InferenceService is **pinned** to a `ControllerRevision` snapshot. Live runtime changes are detected but not applied until you roll forward. |

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: my-service
spec:
  model:
    name: my-model
  runtime:
    name: my-runtime
    autoSync: false   # pin to a snapshot instead of tracking live
```

> **Note:** Setting `autoSync: false` is required before `spec.runtime.revision` can be used - the admission webhook rejects a `revision` pin while `autoSync` is `true`, because a live-tracking service would silently ignore the pin.

## How pinning works

Once `autoSync: false` is set, the controller manages the pin through these transitions:

1. **First reconcile** - the controller resolves the live runtime spec, finds-or-creates a `ControllerRevision` for it (reusing an existing snapshot if one with the same content hash already exists), and records its name in `status.pinnedRevisionName`. The service now renders from that snapshot.
2. **Steady state** - on each reconcile the controller compares the hash of the live runtime to the pinned snapshot. If they match, nothing changes.
3. **Drift** - if the live runtime hash differs from the pinned snapshot, the controller sets the `RuntimeDrifted` status condition to `True` and **keeps rendering from the old snapshot** (it does not adopt the new runtime automatically). The service keeps running unchanged until you roll forward.

### Status fields

| Field | Meaning |
|-------|---------|
| `status.pinnedRevisionName` | Name of the `ControllerRevision` currently driving the pods. Empty when `autoSync` is `true`. |
| `status.lastRuntimeSyncToken` | The value of the `ome.io/runtime-sync` annotation the controller last acted on (used to make roll-forward one-shot). |
| `RuntimeDrifted` condition | `True` when the live runtime has drifted from the pin. Reasons include `RevisionMismatch` (live spec differs) and `RevisionMissing` (the pinned/explicit revision no longer exists). |

## Rolling forward to the latest runtime

When the runtime has changed and you want the pinned service to adopt it, **bump the `ome.io/runtime-sync` annotation** to a new value. This acknowledges the drift; the controller advances the pin to a fresh snapshot of the current live runtime, updates `pinnedRevisionName`, and clears `RuntimeDrifted`.

```bash
kubectl annotate isvc my-service ome.io/runtime-sync="$(date +%s)" --overwrite
```

The advance is **one-shot**: the controller records the token in `status.lastRuntimeSyncToken`, so the same value will not roll forward again. To adopt a later runtime change, set the annotation to a new value.

## Pinning to a specific revision (rollback)

To pin to a specific, already-existing snapshot - for example to roll back to a previous runtime - set `spec.runtime.revision` to the `ControllerRevision` name:

```yaml
spec:
  runtime:
    name: my-runtime
    autoSync: false
    revision: cr-my-runtime-a1b2c3d4   # pin to this exact snapshot
```

An explicit `revision` overrides drift handling. If the named revision does not exist, the controller sets `RuntimeDrifted` with reason `RevisionMissing` rather than silently falling back to the live runtime.

> **Note:** A `revision` can only point at a snapshot that already exists. Snapshots for a *new* runtime version come into existence when an InferenceService rolls forward (via `ome.io/runtime-sync`) or is first pinned. To find available revisions: `kubectl -n ome get controllerrevisions -l ome.io/runtime-of=<runtime-name>`.

## Garbage collection

OME-managed `ControllerRevision` snapshots are pruned by a background garbage collector so they do not accumulate. Per source runtime, the collector keeps the newest N snapshots and never deletes one that any InferenceService still references (`spec.runtime.revision` or `status.pinnedRevisionName`). Everything else is marked (with the `ome.io/gc-eligible-since` timestamp annotation) and deleted after a grace period.

Two controller flags tune this behavior:

| Flag | Default | Description |
|------|---------|-------------|
| `--runtime-revision-retention` | `10` | Snapshots retained per source runtime before GC. |
| `--runtime-revision-grace-period` | `24h` | How long a snapshot stays unreferenced and over-retention before deletion. |

See [Controller Configuration](/ome/docs/administration/controller-configuration) for how to set these on a running cluster.

## RBAC

The controller manager creates, updates, and deletes `ControllerRevision` objects (`apps` API group) in the OME namespace to manage pins and GC. Its ClusterRole must grant `create;get;list;watch;update;patch;delete` on `controllerrevisions`. The bundled Helm chart grants this by default.

## Reference

- Annotations and labels: [`ome.io/runtime-sync`, `ome.io/gc-eligible-since`, and the `ome.io/runtime-of*` labels](/ome/docs/reference/labels-and-annotations)
- API fields: [`ServingRuntimeRef` and `InferenceServiceStatus`](/ome/docs/reference/ome.v1beta1)
- Related concept: [Inference Service](/ome/docs/concepts/inference_service)
