# Alfred nested-KWOK E2E

This harness runs a self-contained KWOK Kubernetes API server in an isolated
namespace of an existing host cluster. A real Alfred pod connects to that
nested API; only the nested Nodes and workload Pods are simulated. No local
container runtime is required, and no fake Node is added to the host cluster.

The fragmentation fixture creates ten fake 8-GPU nodes, places one movable
1-GPU OME pod on each node, and leaves an 8-GPU demand pending. Its E2E-only
configuration weights the 8-GPU bucket, making the scenario deterministic
while retaining Alfred's normal `0.25` policy gate.

## Run

The host kubeconfig must point to a disposable/test Kubernetes cluster. The
Alfred image must match the host node architecture. For a private image, name
an existing `kubernetes.io/dockerconfigjson` pull secret to copy into the
isolated namespace.

```bash
make alfred-e2e-cluster \
  ALFRED_E2E_HOST_KUBECONFIG=/path/to/host-kubeconfig \
  ALFRED_E2E_IMG=fra.ocir.io/idqj093njucb/alfred-new:fra-test-6d936c56dea6 \
  ALFRED_E2E_PULL_SECRET_SOURCE=ome/alfred-build-registry

make alfred-e2e-fragmentation \
  ALFRED_E2E_HOST_KUBECONFIG=/path/to/host-kubeconfig
```

The second target succeeds only when:

- the synthetic 8-GPU demand remains Pending;
- `alfred_cluster_fragmentation_score` exceeds `0.25`;
- `alfred-recommendations/last-cycle.json` contains recommendations; and
- at least one recommendation is `withheld`, proving recommend-only admission.

Re-run assertions without recreating the fixture:

```bash
make alfred-e2e-verify \
  ALFRED_E2E_HOST_KUBECONFIG=/path/to/host-kubeconfig
```

Delete the host namespace, including the nested cluster and its complete
simulated state:

```bash
make alfred-e2e-clean \
  ALFRED_E2E_HOST_KUBECONFIG=/path/to/host-kubeconfig
```

Optional overrides:

- `ALFRED_E2E_NAMESPACE` — host namespace, default `alfred-e2e`.
- `ALFRED_E2E_HOST_CONTEXT` — explicit context inside the host kubeconfig.
- `ALFRED_E2E_NESTED_PORT` — local tunnel port, default `18080`.
- `KWOK_CLUSTER_IMAGE` — nested KWOK image, default
  `registry.k8s.io/kwok/cluster:v0.8.0-k8s.v1.33.12`.
