# OEP-0011: kubectl-ome Plugin CLI

<!--
This OEP proposes an official kubectl plugin for OME, `kubectl-ome`,
distributed through krew. The first version focuses on visibility:
model-centric listings, deep readiness diagnostics for InferenceServices,
an explanation command for runtime selection that reuses the operator's own
scoring engine, and a component-aware log streamer. The architecture is
layered so that later command families (deploy workflows, admin tooling,
imperative CRUD) plug in without restructuring.
-->

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [Command Surface (v1)](#command-surface-v1)
  - [User Stories](#user-stories)
    - [Story 1: Diagnose a Stuck InferenceService](#story-1-diagnose-a-stuck-inferenceservice)
    - [Story 2: Explain Runtime Selection](#story-2-explain-runtime-selection)
    - [Story 3: Stream Component Logs](#story-3-stream-component-logs)
    - [Story 4: Model-centric Inventory](#story-4-model-centric-inventory)
  - [Notes/Constraints/Caveats](#notesconstraintscaveats)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
  - [Binary and Package Layout](#binary-and-package-layout)
  - [Command Framework](#command-framework)
  - [Resource Registry and `kubectl ome get`](#resource-registry-and-kubectl-ome-get)
  - [`kubectl ome status`](#kubectl-ome-status)
  - [`kubectl ome runtime explain`](#kubectl-ome-runtime-explain)
  - [`kubectl ome logs`](#kubectl-ome-logs)
  - [`kubectl ome version`](#kubectl-ome-version)
  - [Krew Distribution and Release Automation](#krew-distribution-and-release-automation)
  - [RBAC Requirements](#rbac-requirements)
  - [Version Skew Policy](#version-skew-policy)
  - [Test Plan](#test-plan)
  - [Graduation Criteria](#graduation-criteria)
- [Implementation History](#implementation-history)
- [Drawbacks](#drawbacks)
- [Alternatives](#alternatives)
  - [Separate Repository](#separate-repository)
  - [Hand-rolled CLI without cli-runtime](#hand-rolled-cli-without-cli-runtime)
  - [Rely on kubectl Alone](#rely-on-kubectl-alone)
<!-- /toc -->

## Summary

OME manages ten custom resources but ships no client-side tooling; everything
users learn about a deployment they learn through `kubectl get`/`describe`
against raw CRDs. This OEP adds an official kubectl plugin, **`kubectl-ome`**,
built in this repository and distributed through [krew](https://krew.sigs.k8s.io/),
so `kubectl krew install ome` gives operators and model users a purpose-built
view of the system.

Version 1 is deliberately scoped to **visibility**: rich, model-centric `get`
listings across all OME resources; a `status` command that assembles the full
readiness story of an InferenceService (conditions, components, pods, model
status, events) into one view; a `runtime explain` command that runs the
operator's own `pkg/runtimeselector` scoring engine to show which runtimes
match a model and why the others were rejected; and a `logs` command that
streams component logs without manual pod hunting. The CLI is layered —
a small command framework (kubeconfig handling, client factory, printers)
beneath independent command packages — so future command families
(deploy workflows, admin tooling, imperative CRUD) are additive.

## Motivation

Kubernetes operators of comparable scope ship official kubectl plugins —
Kueue (`kubectl-kueue`), CloudNativePG (`kubectl-cnpg`), KubeVirt (`virtctl`) —
because CRD columns and `kubectl describe` stop short of the questions users
actually ask. For OME those questions are:

1. **"Why isn't my InferenceService ready?"** The answer today spans the
   InferenceService's conditions (`EngineReady`, `DecoderReady`, `RouterReady`,
   `IngressReady`, `Ready`), its `status.components` map, the pods behind each
   component, and warning events — four `kubectl` invocations and manual
   correlation.
2. **"Which runtime will my model get, and why?"** Runtime selection is a
   weighted scoring decision inside the operator (`pkg/runtimeselector`),
   invisible from the outside. When no runtime matches, users see only a
   failed reconcile.
3. **"Show me the logs for the engine."** Users must first discover pod names
   and label conventions (`ome.io/inferenceservice`, `component`) before they
   can run `kubectl logs`.
4. **"What models are available?"** BaseModels and ClusterBaseModels are two
   separate listings, and the interesting attributes (architecture, parameter
   count, format, readiness) are buried in spec/status.

A first-party CLI answers these directly, and — because it lives in the same
module as the operator — answers them with the operator's own code, so CLI
output can never drift from controller behavior.

### Goals

- Ship an official kubectl plugin (`kubectl-ome`, invoked as `kubectl ome`)
  following kubectl CLI conventions (`k8s.io/cli-runtime` flags, printers,
  exit-code discipline).
- v1 command surface: `get` (all OME resources plus merged convenience views),
  `status`, `runtime explain`, `logs`, `version`.
- Reuse operator code paths — the generated typed clientset
  (`pkg/client/clientset/versioned`) and `pkg/runtimeselector` — rather than
  reimplementing behavior.
- A layered architecture where each future command family is a new package,
  with no changes to the framework layer.
- Release automation that cross-compiles the plugin for linux/darwin/windows ×
  amd64/arm64, attaches archives and checksums to GitHub releases, and
  maintains a krew manifest; submit the plugin to `krew-index` once the first
  release is out.

### Non-Goals

Deferred, not rejected — the architecture reserves room for each:

- Imperative deploy/create/edit/delete workflows (e.g.
  `kubectl ome deploy --model llama-3`).
- Installing, upgrading, or validating the OME control plane itself
  (Helm remains the installation path).
- Interactive TUI, color output, or watch modes.
- Benchmark orchestration and model upload/download tooling.
- A standalone `ome`-branded binary or brew/apt packaging (the single
  `kubectl-ome` binary happens to run standalone, but krew is the only
  supported channel in v1).

## Proposal

Add a `cmd/kubectl-ome` binary whose implementation lives under `pkg/cli/`:
a thin framework layer (client factory, printers, root command wiring) and
one package per command group. Add one release job that packages the binary
for krew. No controller, webhook, or API changes; the CLI is a pure API
client.

### Command Surface (v1)

| Command | Purpose |
| --- | --- |
| `kubectl ome get <resource> [NAME]` | Model-centric tables for all 10 OME CRDs, plus merged `models` and `runtimes` views; `-o json\|yaml\|wide`, `-A`, `-l` |
| `kubectl ome status <isvc>` | One-view readiness diagnosis: conditions, components, pods, model status, traffic/canary/rollout state, warning events |
| `kubectl ome runtime explain (--model M \| --isvc S)` | Ranked runtime compatibility for a model, with per-dimension match details and rejection reasons, computed by `pkg/runtimeselector` |
| `kubectl ome logs <isvc> [-c component]` | Stream logs from the pods behind an InferenceService, multiplexed with `[component/pod]` prefixes |
| `kubectl ome version` | Client version plus best-effort operator version from the `ome-controller-manager` Deployment |

### User Stories

#### Story 1: Diagnose a Stuck InferenceService

An SRE's InferenceService has been `Ready: False` for ten minutes. Instead of
chaining `kubectl get isvc -o yaml`, `kubectl get pods -l ...`, and
`kubectl get events`, they run:

```console
$ kubectl ome status llama-70b -n team-a
```

and see in one view that `DecoderReady` is `False`, the decoder pod is
`Pending`, and a warning event reports unschedulable GPUs.

#### Story 2: Explain Runtime Selection

A platform engineer adds a new ClusterBaseModel and no runtime picks it up.

```console
$ kubectl ome runtime explain --model qwen3-235b
```

shows every ServingRuntime and ClusterServingRuntime, which compatibility
dimensions failed (size range, format, architecture, quantization), and the
priority/weight ordering among the compatible ones — the same verdict the
operator's reconciler would reach, because it is the same code.

#### Story 3: Stream Component Logs

An ML engineer wants engine logs without learning OME's label conventions:

```console
$ kubectl ome logs llama-70b -c engine -f --tail 100
```

#### Story 4: Model-centric Inventory

A cluster admin wants one inventory across namespaced and cluster-scoped
models:

```console
$ kubectl ome get models -A
NAME                    SCOPE       ARCH       PARAMS   FORMAT       READY   AGE
llama-3-3-70b-instruct  Cluster     LlamaFor…  70B      safetensors  True    41d
mistral-7b (team-a)     Namespaced  MistralF…  7B       safetensors  True    12d
```

### Notes/Constraints/Caveats

- **Same module, same dependency graph.** The CLI imports the operator module,
  so its binary carries operator-sized dependencies (~50–80 MB static binary).
  This is the norm for in-repo plugins (kueuectl is comparable) and is
  irrelevant to krew users, who download prebuilt archives.
- **Pure-Go constraint.** `cmd/manager` and the agents link the Rust `xet`
  library and cannot be trivially cross-compiled. The CLI must remain
  `CGO_ENABLED=0`-buildable: `pkg/cli` must not (transitively) import
  `pkg/xet` or any cgo package. Verified today: neither `pkg/client` nor
  `pkg/runtimeselector` has such imports. CI enforces it by cross-compiling
  all six platform targets on every PR that touches the CLI.
- **`runtime explain` needs a controller-runtime client.**
  `runtimeselector.New` takes a `client.Client`, so the factory provides a
  lazily constructed controller-runtime client (scheme: core/v1, apps/v1,
  `ome.io/v1beta1`) alongside the typed clientset. Commands that don't need
  it never build it.
- **Read-only by construction.** v1 issues no writes, so a mis-scoped
  kubeconfig cannot mutate anything.
- **Optional spec fields.** `InferenceService.spec.model` may be omitted
  (see #717); columns render `-` for absent references rather than erroring.

### Risks and Mitigations

- **Output-format churn breaks scripts.** Until GA, human-readable output is
  explicitly not a stable interface; scripts must use `-o json|yaml`
  (documented in `--help` and the docs page).
- **krew-index acceptance is not in our control.** The release pipeline also
  publishes the stamped krew manifest as a release asset, so
  `kubectl krew install --manifest-url=<release-asset>` works from day one;
  index submission is additive.
- **CLI/operator version skew.** The CLI is a client of versioned APIs and
  tolerates unknown fields by construction; `kubectl ome version` prints both
  versions, and the skew policy below sets expectations.
- **Release pipeline regression risk.** The CLI packaging job is independent
  of the image and chart jobs; a packaging failure cannot block or corrupt
  existing artifacts.

## Design Details

### Binary and Package Layout

```
cmd/kubectl-ome/
  main.go                  # ~20 lines: construct root command, execute

pkg/cli/
  root.go                  # root cobra command; global flags; subcommand registry
  factory/
    factory.go             # Factory interface + default implementation
  printers/
    printers.go            # tabwriter tables; -o json|yaml passthrough
  cmd/
    get/                   # resource registry + get command
    status/                # status command
    runtime/               # runtime explain (future: other runtime subcommands)
    logs/                  # logs command
    version/               # version command
```

Future command families (`deploy`, `admin`, …) are new directories under
`pkg/cli/cmd/`, registered with one line in `root.go`. Nothing in
`factory/` or `printers/` changes.

### Command Framework

Kubeconfig plumbing comes from `k8s.io/cli-runtime`:
`genericclioptions.ConfigFlags` contributes `--kubeconfig`, `--context`,
`--namespace/-n`, `--as`, `--request-timeout`, etc., identically to kubectl.
All I/O flows through `genericiooptions.IOStreams` so tests substitute
buffers.

Clients come from a small factory owned by the framework layer:

```go
// pkg/cli/factory
type Factory interface {
    // RESTConfig returns the resolved client-go REST config.
    RESTConfig() (*rest.Config, error)
    // KubeClient returns a core Kubernetes clientset (pods, events, logs).
    KubeClient() (kubernetes.Interface, error)
    // OMEClient returns the generated OME typed clientset.
    OMEClient() (versioned.Interface, error)
    // RuntimeClient returns a lazily constructed controller-runtime client
    // (scheme: core/v1, apps/v1, ome.io/v1beta1) for commands that reuse
    // operator libraries such as pkg/runtimeselector.
    RuntimeClient() (client.Client, error)
    // Namespace returns the effective namespace and whether it was set
    // explicitly (flag or kubeconfig context).
    Namespace() (string, bool, error)
}
```

Every command follows the kubectl idiom — an options struct with
`Complete → Validate → Run`:

```go
func NewCmd(f factory.Factory, streams genericiooptions.IOStreams) *cobra.Command
func (o *Options) Complete(f factory.Factory, cmd *cobra.Command, args []string) error
func (o *Options) Validate() error
func (o *Options) Run(ctx context.Context) error
```

Errors go to stderr with exit code 1. Two error classes get dedicated
messages: OME CRDs absent from discovery ("OME does not appear to be
installed on this cluster") and `NotFound` for named arguments.

### Resource Registry and `kubectl ome get`

`get` is table-driven. Each OME resource contributes one registry entry;
adding a resource is one entry, not a new command:

```go
type entry struct {
    Canonical string          // "inferenceservices"
    Aliases   []string        // "isvc", "inferenceservice"
    Scope     scope           // Namespaced | Cluster | Merged
    Columns   []column        // name, wide-only flag, extractor func
    List      listFunc        // typed-clientset list call
}
```

All ten CRDs are registered: `inferenceservices`, `basemodels`,
`clusterbasemodels`, `servingruntimes`, `clusterservingruntimes`,
`acceleratorclasses`, `benchmarkjobs`, `finetunedweights`,
`inferencereplicas`, `workloadclusters`. Two **merged views** add value
kubectl cannot express:

- `models` = BaseModels + ClusterBaseModels in one table with a `SCOPE` column
- `runtimes` = ServingRuntimes + ClusterServingRuntimes likewise

Columns are model-centric, e.g. InferenceService:
`NAME MODEL RUNTIME READY URL AGE`; models:
`NAME SCOPE ARCH PARAMS FORMAT READY AGE`. `-o wide` adds detail columns;
`-o json|yaml` prints the underlying objects unmodified (single object for
one name, `List` for many). `-A/--all-namespaces` and `-l/--selector` behave
as in kubectl.

### `kubectl ome status`

Assembles the readiness story for one InferenceService:

1. InferenceService object (typed clientset).
2. Conditions from `status` (duck-typed knative conditions): `EngineReady`,
   `DecoderReady`, `RouterReady`, `IngressReady`, aggregate `Ready`.
3. Per-component detail from `status.components`
   (`ComponentStatusSpec`: latest revision, URLs) joined with live pods,
   discovered via the operator's own label constants
   (`ome.io/inferenceservice`, `component`).
4. `status.modelStatus`, and — only when present — `status.traffic`,
   `status.canary`, `status.placement`, `status.rolloutCoordination`.
5. Warning events for the InferenceService and its pods
   (`involvedObject` field selectors).

Output sketch:

```console
$ kubectl ome status llama-70b -n team-a
Name:       llama-70b
Namespace:  team-a
Ready:      False
URL:        https://llama-70b.team-a.example.com
Model:      llama-3-3-70b-instruct (ClusterBaseModel)
Runtime:    srt-llama-70b (ClusterServingRuntime, auto-selected)

Conditions:
  TYPE          STATUS   REASON             MESSAGE
  EngineReady   True
  DecoderReady  False    RevisionFailed     0/1 replicas ready
  RouterReady   True
  IngressReady  True
  Ready         False    DecoderNotReady

Components:
  engine   revision llama-70b-engine-00002
    POD                        PHASE     READY   RESTARTS   NODE
    llama-70b-engine-5d4-x2p   Running   2/2     0          gpu-node-1
  decoder  revision llama-70b-decoder-00002
    POD                        PHASE     READY   RESTARTS   NODE
    llama-70b-decoder-7c9-k4m  Pending   0/2     0          <none>

Recent Warning Events:
  LAST SEEN   OBJECT                          REASON             MESSAGE
  2m          Pod/llama-70b-decoder-7c9-k4m   FailedScheduling   0/12 nodes: insufficient nvidia.com/gpu
```

`status` is informational: exit code reflects only whether the command could
gather data, never the health of the service (a `--fail-if-not-ready` flag
is possible later without breaking that contract).

### `kubectl ome runtime explain`

Answers "which runtime serves this model, and why not the others" with the
operator's own engine. Input is `--model <name>` (BaseModel in the target
namespace, falling back to ClusterBaseModel — same resolution order as the
operator) or `--isvc <name>` (explain an existing service, including its
`spec.runtime` override if set, via `ValidateRuntime`).

The command builds the factory's controller-runtime client, calls
`runtimeselector.GetCompatibleRuntimes`, and renders the `[]RuntimeMatch`
results — each carrying `MatchDetails` (per-dimension booleans for format,
framework, size, architecture, diffusion pipeline, quantization, model-cache
provider, plus `Priority`, `Weight`, `AutoSelectEnabled`, and human-readable
`Reasons`):

```console
$ kubectl ome runtime explain --model qwen3-235b -n team-a
RUNTIME             SCOPE        COMPATIBLE   PRIORITY   WEIGHT   REASON
srt-qwen3-large     Cluster      Yes          2          100      selected
srt-generic-llm     Cluster      Yes          1          60       lower priority
srt-small-models    Cluster      No           -          -        size mismatch: 235B outside [1B, 32B]
vllm-team-a         Namespaced   No           -          -        format mismatch: requires gguf
```

A `-o wide|json` view exposes the full per-dimension breakdown for
debugging scoring itself.

### `kubectl ome logs`

Resolves pods with the `ome.io/inferenceservice=<name>` label, optionally
filtered by `component=<engine|decoder|router>` (`-c`), and streams the main
container of each (override: `--container`). Multiple streams are multiplexed
line-by-line with `[component/pod]` prefixes; `-f`, `--tail`, `--since` map
to the underlying pod log options. Single-pod, single-container invocations
print unprefixed output so pipes stay clean.

### `kubectl ome version`

Prints the client version (stamped via ldflags into the existing
`pkg/version` package, as the other binaries do) and a best-effort operator
version read from the `ome-controller-manager` Deployment's image tag,
searching the release namespace (`--ome-namespace`, default `ome`). Cleanly
reports "unknown (deployment not found or not readable)" when RBAC or
topology hides it.

### Krew Distribution and Release Automation

A new job in `.github/workflows/release.yaml` (independent of the image and
chart jobs):

1. Cross-compile `cmd/kubectl-ome` with `CGO_ENABLED=0` for
   linux/darwin/windows × amd64/arm64, stamping the version via ldflags.
2. Package each as `kubectl-ome_<version>_<os>_<arch>.tar.gz` (binary +
   `LICENSE`), generate `checksums.txt`.
3. Attach archives, checksums, and the stamped krew manifest to the GitHub
   release (which already uses `softprops/action-gh-release`).

The manifest template lives at `hack/krew/ome.yaml`; CI substitutes the
version and per-platform sha256 values:

```yaml
apiVersion: krew.googlecontainertools.github.com/v1alpha2
kind: Plugin
metadata:
  name: ome
spec:
  homepage: https://github.com/ome-projects/ome
  shortDescription: Inspect OME models, runtimes and inference services
  platforms:
    - selector: {matchLabels: {os: darwin, arch: arm64}}
      uri: https://github.com/ome-projects/ome/releases/download/{{ .Tag }}/kubectl-ome_{{ .Tag }}_darwin_arm64.tar.gz
      sha256: {{ .Sha256 }}
      bin: kubectl-ome
    # … linux/windows × amd64/arm64
```

Rollout sequence: (1) first release ships archives + manifest, installable
via `kubectl krew install --manifest-url=…`; (2) submit the manifest to
`kubernetes-sigs/krew-index` (one-time review); (3) automate subsequent
index bumps with `krew-release-bot`.

### RBAC Requirements

The CLI needs only read access; a documented example ClusterRole ships with
the docs page:

| API group | Resources | Verbs |
| --- | --- | --- |
| `ome.io` | all ten CRDs | `get`, `list` |
| core | `pods`, `events` | `get`, `list` |
| core | `pods/log` | `get` |
| `apps` | `deployments` (version lookup) | `get` |

### Version Skew Policy

The CLI supports the operator minor version it shipped with, one minor ahead,
and one minor behind. Unknown API fields are ignored by client-go decoding,
so older CLIs degrade gracefully against newer operators; `kubectl ome
version` surfaces both versions for support triage.

### Test Plan

#### Unit Tests

- `Complete`/`Validate` coverage for every command's flag and argument
  handling, including error paths (missing name, conflicting flags).
- `Run` coverage against the generated fake clientset
  (`pkg/client/clientset/versioned/fake`) and `k8s.io/client-go/kubernetes/fake`
  for pods/events; `runtime explain` against a fake controller-runtime client
  seeded with models and runtimes.
- Golden-file tests for every table renderer (`get` per resource, `status`,
  `explain`), asserted through `IOStreams` buffers.

#### Integration tests

The CLI has no controllers, so envtest adds nothing over fakes; integration
coverage arrives at beta as an e2e smoke job: kind cluster, install CRDs +
operator, apply fixtures, assert on `kubectl ome … -o json` output and exit
codes.

### Graduation Criteria

- **Alpha**: binary + archives in release assets; manifest-URL install path
  documented; unit tests in PR validation; cross-compile check in CI.
- **Beta**: accepted into krew-index; e2e smoke job in CI; output columns
  reviewed for scripting stability; `--fail-if-not-ready`-class UX decisions
  resolved.
- **Stable**: command surface and human-readable output frozen (additive
  changes only); version skew policy validated across two releases.

## Implementation History

- 2026-08-11: Provisional OEP created.

## Drawbacks

- One more released artifact family (six archives + manifest) to maintain.
- The binary inherits the operator module's dependency weight; a standalone
  thin client would be smaller but would fork behavior.
- Human-readable output creates an implicit interface users will script
  against despite documentation; freezing it is deferred to GA deliberately.

## Alternatives

### Separate Repository

`ome-projects/kubectl-ome` with its own release cadence. Rejected: the CLI
must import this module for API types regardless, so the dependency weight
returns; every CRD change forces a cross-repo version bump; CI/release infra
is duplicated; and ecosystem precedent (kueue, CloudNativePG, KubeVirt)
keeps plugins in-repo.

### Hand-rolled CLI without cli-runtime

Raw client-go plus hand-written kubeconfig flags and table writers. Rejected:
it reimplements what `cli-runtime` provides (flag conventions, printer
flags, kubeconfig resolution order) and inevitably drifts from kubectl UX —
the opposite of a plugin's purpose.

### Rely on kubectl Alone

Invest in `additionalPrinterColumns` and documentation instead. Rejected as
insufficient: server-side columns cannot join across resources (conditions +
pods + events), cannot run the runtime-selection engine, and cannot merge
cluster- and namespace-scoped listings into one view.
