---
title: "Controller Configuration"
linkTitle: "Controller Configuration"
weight: 6
description: >
  Command-line flags for the OME controller manager, including runtime-revision garbage collection.
---

The OME controller manager (`ome-manager`) is configured through command-line flags passed to its container. This page documents those flags and how to change them on a running cluster.

## Setting flags

The flags are parsed once at startup, so a change takes effect when the controller pod restarts. No image rebuild is required.

- **Helm** (recommended): set them in your chart values / the manager Deployment's `args` and run `helm upgrade`.
- **Directly on the Deployment** (quick, but reverted by the next `helm upgrade`):

```bash
kubectl -n ome patch deployment <ome-controller-manager> --type=json -p='[
  {"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--runtime-revision-retention=20"},
  {"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--runtime-revision-grace-period=72h"}
]'
```

The manager logs its resolved configuration at startup, so you can confirm a value landed:

```bash
kubectl -n ome logs deploy/<ome-controller-manager> | grep -iE 'retention|gracePeriod'
```

## Flags

### Server and webhook

| Flag                  | Type | Default | Description                                                        |
|-----------------------|------|---------|--------------------------------------------------------------------|
| `--webhook`           | bool | `false` | Enable the webhook server.                                         |
| `--webhook-port`      | int  | `9443`  | Port the webhook server binds to.                                  |
| `--health-probe-addr` | string | `:8081` | Address the health/readiness probe endpoint binds to.            |
| `--enable-http2`      | bool | `false` | Enable HTTP/2 for the metrics and webhook servers.                 |

### Leader election

| Flag                          | Type   | Default       | Description                                             |
|-------------------------------|--------|---------------|--------------------------------------------------------|
| `--leader-elect`              | bool   | `false`       | Enable leader election so only one manager is active.  |
| `--leader-election-namespace` | string | OME namespace | Namespace for the leader-election lease.               |

### Metrics

| Flag                     | Type   | Default  | Description                                                                                     |
|--------------------------|--------|----------|-------------------------------------------------------------------------------------------------|
| `--metrics-bind-address` | string | `:8080`  | Address the metrics endpoint binds to (`:8443` for HTTPS, `:8080` for HTTP, `0` to disable).   |
| `--metrics-secure`       | bool   | `false`  | Serve metrics over HTTPS.                                                                        |

### Runtime-revision garbage collection

These control cleanup of the OME-managed `ControllerRevision` snapshots created for [runtime pinning](/ome/docs/concepts/runtime-revision).

| Flag                             | Type     | Default | Description                                                                                    |
|----------------------------------|----------|---------|------------------------------------------------------------------------------------------------|
| `--runtime-revision-retention`   | int      | `10`    | Number of `ControllerRevision` snapshots to retain per source runtime before garbage collection. |
| `--runtime-revision-grace-period`| duration | `24h`   | How long a snapshot must stay unreferenced and over-retention before the GC deletes it.        |

Logging is configured with the standard controller-runtime zap flags (for example `--zap-log-level`, `--zap-encoder`).

## Tuning runtime-revision garbage collection

The garbage collector keeps the newest `--runtime-revision-retention` snapshots per runtime and never deletes a snapshot that an InferenceService still references (via `spec.runtime.revision` or `status.pinnedRevisionName`). Anything else becomes GC-eligible; once it has stayed unreferenced and over-retention for longer than `--runtime-revision-grace-period`, it is deleted.

- Raise **retention** to keep a longer rollback history per runtime.
- Lower the **grace period** to reclaim storage sooner; raise it to keep a longer safety window before deletion.
- The grace period is a Go duration, so minutes and hours both work (`30m`, `72h`). There is no day unit - a week is `168h`.

See [Runtime Revisions and Pinning](/ome/docs/concepts/runtime-revision) for the full pinning workflow.
