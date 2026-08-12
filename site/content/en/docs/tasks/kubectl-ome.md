---
title: "kubectl-ome Plugin"
linkTitle: "kubectl-ome"
weight: 20
description: >
  Install and use the official OME CLI as a kubectl plugin
---

## Install

Via [krew](https://krew.sigs.k8s.io/) once the plugin is accepted into the index:

```bash
kubectl krew install ome
```

Until then, install straight from a GitHub release:

```bash
VERSION=v0.10.0  # pick the release you want
kubectl krew install --manifest-url \
  https://github.com/ome-projects/ome/releases/download/${VERSION}/ome.yaml
```

## Commands

| Command | What it shows |
| --- | --- |
| `kubectl ome get isvc` | InferenceServices with model, runtime, readiness and URL |
| `kubectl ome get models -A` | BaseModels and ClusterBaseModels in one listing |
| `kubectl ome get runtimes` | ServingRuntimes and ClusterServingRuntimes in one listing |
| `kubectl ome status my-isvc` | Conditions, per-component pods, model status and warning events |
| `kubectl ome runtime explain --model my-model` | Which runtimes match the model, ranked, with rejection reasons |
| `kubectl ome logs my-isvc -c engine -f` | Live logs from the engine pods |
| `kubectl ome version` | Client and operator versions |

Every command accepts the standard kubectl connection flags
(`--kubeconfig`, `--context`, `-n`); listings accept `-o json|yaml|wide`,
`-A` and `-l`. Human-readable output is not a stable scripting interface
before GA — script against `-o json`.

## Required RBAC

The plugin is read-only. A minimal ClusterRole:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubectl-ome-reader
rules:
  - apiGroups: ["ome.io"]
    resources: ["*"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["pods", "events"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get"]
```
