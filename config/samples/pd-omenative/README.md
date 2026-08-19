# Prefill/decode disaggregation, and how metadata reaches the pods

This sample answers one question: **when I put an annotation on an
InferenceService, where does it end up, and what decides that?** It uses a
prefill/decode (PD) topology because that is the case where the answer is
non-obvious — one service, two independently sized Components, each with its own
pod template.

## Apply it

```sh
kubectl apply -f config/samples/pd-omenative/
```

The runtime's `nodeSelector`, `tolerations` and resource key are placeholders for
whatever your accelerator nodes advertise. Nothing in the propagation described
below depends on them.

## What the sample models

The shape is taken from a real multi-host accelerator deployment, with every
site-specific value replaced:

| Pattern | Why it is there |
|---|---|
| **Asymmetric gang counts** (engine 3, decoder 2) | prefill and decode do not sustain the same requests/sec per gang, so equal counts starve one stage. Derive the ratio from your own measurements. |
| **`topologyKey` per Component** | names the slice one gang must fit inside. Read on the OMENative path only, where it is projected onto `ir.Spec.TopologyKey`; the InferenceReplica controller is what turns it into worker→leader affinity, and that controller is absent here. See [Slice co-location by path](#slice-co-location-by-path). |
| **zone `nodeAffinity`** | bounds the pool gangs are placed from, so a gang is never scheduled into a zone with no accelerator capacity for it. Unlike `topologyKey`, this is plain pod-spec affinity and applies on every path. |
| **`readyPolicy: AllPodReady`** | a partially ready multi-host gang cannot answer a request, so readiness is all-or-nothing per gang. |
| **`restartPolicy: RecreateInstanceOnPodRestart`** | one crashlooping pod invalidates its gang's collective state; replacing the gang is the recovery, not restarting the pod. |
| **`SurgeThenDrain`, `maxUnavailable: 0`** | stand up a replacement gang before draining an old one so a roll never gives up serving capacity. |
| **Long `instanceReadyTimeout`** | a cold accelerator compile can dwarf a normal container start; too short a budget tears gangs down mid-compile so they never converge. |
| **`bounce` label** | changes the pod template with no real spec change, which is how you force a restart through the rollout policy above. |

Substitute your own accelerator selector, slice-identity label, topology and
resource key. None of the propagation below depends on them.

## The annotations in the sample are probes

| Annotation | Declared on | Expected outcome |
|---|---|---|
| `example.com/owner` | service only | appears on **both** engine and decoder pods |
| `example.com/tier` | service **and** each Component | Component value wins (`engine-level` / `decoder-level`) |
| `example.com/slice-hint` | service only | survives to the pods, so a scheduler can read it |

## Where the merge happens

Per Component, in `processAnnotations` (`components/engine.go`,
`components/decoder.go`, `components/router.go`):

1. Start from the InferenceService's annotations, dropping any key in
   `constants.ServiceAnnotationDisallowedList`.
2. Union the Component's own `annotations` over that — **the Component wins on a
   key collision**.
3. `ProcessBaseAnnotations` adds derived entries (model format, model category,
   fine-tuned weight wiring).

Labels follow the same shape via `processLabels` + `ProcessBaseLabels`.

The result is the Component's ObjectMeta, and that is what reaches the pods. Only
the last hop differs by deployment mode:

| Mode | Component ObjectMeta lands on | Pods created by |
|---|---|---|
| `RawDeployment` | Deployment pod template | this build |
| `MultiNode` | LeaderWorkerSet pod template | this build |
| `OMENative` | `InferenceReplica` `spec.runners[].template.metadata` | **not this build** — see below |

`spec.engine.topologyKey` is projected onto `ir.Spec.TopologyKey`. Deriving the
worker→leader affinity from it is the InferenceReplica controller's job, so in
this build the key reaches the InferenceReplica and stops there.

### Slice co-location by path

The field that co-locates a gang is not the same on every path, which is easy to
get wrong because both are named after topology:

| Path | What constrains a gang | Notes |
|---|---|---|
| `OMENative` | `spec.<component>.topologyKey` | projected onto `ir.Spec.TopologyKey`; the absent InferenceReplica controller would derive the affinity |
| `MultiNode` | `leaderworkerset.sigs.k8s.io/exclusive-topology` annotation | Component annotations are copied onto the LWS object, and LWS places each group exclusively within one domain |
| `RawDeployment` | neither | a single-pod Component has no gang to co-locate |

`spec.<component>.topologyKey` is **ignored on the MultiNode path** — it is not
passed to the LWS builder — so the gang sample uses the LWS annotation instead.
`TestCreateLWSCarriesExclusiveTopologyToTheLWSObject` pins that the annotation
reaches the LWS object, which is where LWS reads it.

## Queue-admitted gangs (Kueue)

`30-inferenceservice-gang.yaml` puts a multi-host gang behind a quota. A gang
cannot serve until every pod runs, so admitting its pods one at a time is worse
than not admitting them: partial gangs hold accelerators they cannot use while
blocking the gang behind them.

OME derives the Kueue labels from two annotations on the service:

| Annotation on the service | Becomes, on every pod |
|---|---|
| `ome.io/dedicated-ai-cluster: <queue>` | label `kueue.x-k8s.io/queue-name: <queue>` |
| `kueue-enabled` (presence) | label `kueue.x-k8s.io/priority-class: kueue-scheduling-high-priority` |

That is a second propagation shape worth knowing: an **annotation on the service
becomes a label on the pods**, not a copy. `SetPodLabelsFromAnnotations` also has
a Volcano branch — `ome.io/volcano-queue` becomes `volcano.sh/queue-name`.

`schedulerName` is a separate knob from admission, and the sample sets it to show
it is passed through: Kueue gates pods through its webhook, and whichever
scheduler is named binds them once admitted. Point it at a topology-aware
scheduler, or leave it unset for the default one. Volcano is the exception —
there `schedulerName: volcano` is load-bearing, because Volcano *is* the
scheduler doing admission.

Two limits are worth stating plainly:

- **The rewrite runs on the `RawDeployment` and `MultiNode` paths only** —
  `deployment_reconciler.go` and `lws_reconciler.go` call it; the OMENative
  projection path does not. A queue annotation on an `OMENative` service derives
  no queue label today. That is why this sample uses `MultiNode`, which is also
  the mode that creates pods in this build.
- **All-or-nothing admission needs a pod-group identity that OME does not
  derive.** Kueue treats plain pods as one workload only when they carry
  `kueue.x-k8s.io/pod-group-name` and a matching
  `kueue.x-k8s.io/pod-group-total-count`. The sample sets them through the normal
  Component metadata path, which is why it stays at one replica: the values name a
  single group, so more replicas would fold every gang into one group and
  misreport its size.

The multi-cluster placement path derives the queue label differently — from the
`ome.io/local-queue` annotation or the operator-configured `placement.localQueue`
— so a derived InferenceService lands in the target cluster's queue without these
annotations.

## Inspecting the projection

In `OMENative` mode each Component becomes one InferenceReplica named
`<isvc>-<component>`:

```sh
kubectl get inferencereplica pd-demo-engine -o yaml
kubectl get inferencereplica pd-demo-engine \
  -o jsonpath='{.spec.runners[*].template.metadata.annotations}'
```

Every Runner template carries the merged set, so what you read there is what the
pods rendered from it will carry. `TestOMENativeProjectionCarriesMergedPodMetadata`
in `components/ir_projection_test.go` pins this, including that the
Component-level value wins.

## What this build does not do

`OMENative` mode **projects the InferenceReplica and stops.** No pods are
rendered from it, because the InferenceReplica controller is not part of this
repository yet. Concretely, absent here:

- **`InferenceReplica` → pods.** Nothing reconciles an InferenceReplica, so the
  IR is an inspectable description of intended pods, not a running workload. Use
  `RawDeployment` or `MultiNode` for a workload that actually starts.
- **PodGroups / gang admission.** `ir.Spec.TopologyKey` is projected but nothing
  consumes it here; there is no PodGroup creation and no gang-aware scheduler, so
  a partially-schedulable gang is not held back.
- **Slice-aware packing.** Choosing *which* topology domain a gang lands in — and
  packing several gangs across domains without stranding capacity — is a
  scheduler concern and lives outside this repository.

For `RawDeployment` and `MultiNode` the full chain works end to end, so those are
the modes to use when you want to observe annotations on running pods today.
