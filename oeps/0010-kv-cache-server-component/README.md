# OEP-0010: KV Cache Server Component for InferenceService

<!--
This OEP proposes a new provider-neutral KV cache component for OME
InferenceServices. The component is implemented as a Deployment and Service
owned by the InferenceService, and is reconciled alongside engine, decoder, and
router.
-->

<!-- toc -->
- [Summary](#summary)
- [Motivation](#motivation)
  - [Goals](#goals)
  - [Non-Goals](#non-goals)
- [Proposal](#proposal)
  - [Naming](#naming)
  - [User Stories](#user-stories)
    - [Story 1: Enable LMCache MP for One Model Endpoint](#story-1-enable-lmcache-mp-for-one-model-endpoint)
    - [Story 2: Platform Runtime Defaults with Per-Service Opt-In](#story-2-platform-runtime-defaults-with-per-service-opt-in)
    - [Story 3: Shared Cache Across Engine Replicas](#story-3-shared-cache-across-engine-replicas)
    - [Story 4: Prefill-Decode Disaggregation](#story-4-prefill-decode-disaggregation)
    - [Story 5: Independent Cache Operations](#story-5-independent-cache-operations)
  - [Notes/Constraints/Caveats](#notesconstraintscaveats)
  - [Risks and Mitigations](#risks-and-mitigations)
- [Design Details](#design-details)
  - [API Specifications](#api-specifications)
    - [InferenceService Extensions](#inferenceservice-extensions)
    - [ServingRuntime Extensions](#servingruntime-extensions)
    - [Status Extensions](#status-extensions)
  - [Example Configuration](#example-configuration)
    - [InferenceService with LMCache](#inferenceservice-with-lmcache)
    - [ServingRuntime Defaults](#servingruntime-defaults)
  - [Component Lifecycle](#component-lifecycle)
  - [Reconciliation Flow](#reconciliation-flow)
  - [Provider Adapter Contract](#provider-adapter-contract)
  - [Connector Injection](#connector-injection)
  - [Scheduling and Resource Management](#scheduling-and-resource-management)
  - [Readiness, Status, and Failure Behavior](#readiness-status-and-failure-behavior)
  - [Observability](#observability)
  - [Security](#security)
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

This OEP introduces a new `kvCache` component to `InferenceService`. The
component provides a per-service distributed KV cache pool for a single model
endpoint and is reconciled in parallel with the existing `engine`, `decoder`,
and `router` components. The component is backed by a Kubernetes Deployment and
ClusterIP Service owned by the `InferenceService`; deleting the
`InferenceService` deletes the cache workload through normal owner references
and component cleanup.

The API is provider-neutral. Users configure `spec.kvCache` with common cache
intent such as provider, resources, cache policy, replicas, service ports,
scheduling, and connector policy. Provider-specific options live behind a
`providerConfig` escape hatch. The first implementation targets LMCache in
multiprocess mode, where a cache server runs as a standalone process and vLLM
engine or decoder pods attach to it through a KV transfer connector. The API is
designed to also support providers such as MoonCake without introducing
provider-specific fields into the top-level OME API.

This design deliberately does not introduce an independently managed
`CacheServer` custom resource. Cross-model cache sharing can cause unexpected
behavior, and OME already models serving endpoints as an `InferenceService`
composed of components. Therefore, the cache server should be owned by the same
`InferenceService` as the model-serving engine, decoder, and router.

## Motivation

LLM inference systems increasingly rely on KV cache reuse, prefill reuse, and
prefix-aware scheduling to improve latency and throughput. The serving SLO is to
continuously improve inference speed, but a single engine pod's local KV cache
is insufficient when requests are distributed across replicas, prefill and
decode are separated, or workloads need to survive engine restarts without
losing all reusable cache state.

Providers such as LMCache and MoonCake address this by running a cache service
that serving engines connect to. LMCache's recommended multiprocess mode is a
standalone service with management, metrics, and connector endpoints. vLLM's
production-stack follows the same operational split: it runs a cache server as
a separate Deployment and Service, then injects LMCache connector configuration
into the vLLM runtime. OME needs the same separation of concerns, but with an
API boundary that matches OME's component model and avoids a provider-specific
or independently shared cache resource.

### Goals

1. Add `kvCache` as a first-class `InferenceService` component parallel to
   `engine`, `decoder`, and `router`.
2. Keep the `spec.kvCache` API provider-neutral while supporting LMCache as the
   first provider.
3. Run the cache server as a Deployment per `InferenceService` in alpha.
4. Ensure cache capacity is not shared across different models or different
   `InferenceService` objects by default.
5. Let `ServingRuntime` provide reusable KV cache defaults while requiring
   per-`InferenceService` opt-in.
6. Reconcile the cache server Service before injecting cache endpoint settings
   into engine and decoder pods.
7. Support independent cache sizing, scheduling, health, status, and metrics.
8. Preserve existing `InferenceService` behavior when `spec.kvCache` is absent.

### Non-Goals

1. Introduce a standalone OME `CacheServer` or `KVCacheServer` CRD.
2. Share a cache pool across different models or different `InferenceService`
   objects.
3. Run the cache component as a DaemonSet.
4. Make the cache component required for all inference workloads.
5. Implement a provider-specific top-level API such as `spec.lmCache`.
6. Replace local engine prefix caching or provider-native local cache features.
7. Provide router-level KV-cache-aware routing in the initial alpha.
8. Guarantee durable persistence of KV entries across cache server replacement.
9. Change BaseModel, model download, or node-local model cache semantics.

## Proposal

Add a `spec.kvCache` field to `InferenceServiceSpec` and a matching
`spec.kvCacheConfig` field to `ServingRuntimeSpec`. `spec.kvCache` enables a
cache component for that service. `spec.kvCacheConfig` supplies runtime defaults
only when the service has opted in with `spec.kvCache`; it does not create a
cache component by itself. This matches the existing engine, decoder, and router
merge model, where a runtime provides defaults for a component that the
`InferenceService` chooses to use.

The new component reconciler creates a Deployment, Service, HPA, and PDB using
the existing raw Kubernetes component reconcilers where possible. In alpha,
`kvCache` only supports `RawDeployment` because the cache server is a
state-bearing service dependency for engine and decoder pods. Knative
serverless, LeaderWorkerSet, Ray, and DaemonSet support are out of scope for the
initial implementation.

When `kvCache` is enabled, the controller derives a stable in-cluster endpoint
for the cache server and uses a provider adapter to inject connector settings
into the selected target components. For LMCache with vLLM, this means adding
the appropriate `--kv-transfer-config` argument and LMCache environment
variables to the engine and, when present, decoder containers. The exact
connector class and provider-specific keys are owned by the LMCache adapter and
can be overridden through `providerConfig`.

### Naming

The API field should be named `kvCache`, not `cacheServer`.

`kvCache` describes the user intent: provide distributed KV cache capacity for
this inference endpoint. `cacheServer` describes one implementation detail.
Using `kvCache` keeps the API aligned with the model-serving domain and leaves
room for providers whose architecture is more complex than a single server
process.

Recommended names:

- API field: `spec.kvCache`
- Runtime default field: `spec.kvCacheConfig`
- Go spec type: `KVCacheSpec`
- Component type: `KVCacheComponent`
- Status key: `status.components.kvCache`
- Ready condition: `KVCacheReady`
- Kubernetes resource suffix: `-kv-cache`
- Internal reconciler name: `KVCacheReconciler`

The term `KVCacheServer` may be used internally for helpers that build the
Deployment-backed server workload, but it should not be the top-level user API.

### User Stories

#### Story 1: Enable LMCache MP for One Model Endpoint

Alice deploys a vLLM-backed model and wants repeated prompts to reuse KV cache
across engine replicas. She enables `spec.kvCache` with provider `LMCache`.
OME creates `<isvc>-kv-cache` as a Deployment and Service, then injects the
LMCache MP connector settings into the engine pods.

Alice does not create a separate cache CRD, does not manually wire a Service
DNS name into vLLM arguments, and does not risk another model attaching to the
same cache pool.

#### Story 2: Platform Runtime Defaults with Per-Service Opt-In

Bob owns cluster-wide `ClusterServingRuntime` objects. He wants a standard
LMCache image, default ports, and default connector behavior for all vLLM
runtimes, but he does not want every service to launch a cache server
automatically.

Bob defines `spec.kvCacheConfig` on the runtime. A model owner opts in by
adding `spec.kvCache.provider: LMCache` to an `InferenceService`. OME merges the
runtime defaults with the service's overrides and reconciles the cache component
only for that service.

#### Story 3: Shared Cache Across Engine Replicas

Carol scales an `InferenceService` from one engine replica to four engine
replicas. All engine replicas attach to the same `kvCache` Service for that
`InferenceService`, so prefix reuse is not limited to the pod that saw the
original prompt.

The cache is still scoped to one model endpoint. A different `InferenceService`
for a different model gets a different cache Deployment and Service.

#### Story 4: Prefill-Decode Disaggregation

Dave uses OME's engine and decoder components for prefill-decode disaggregated
serving. He enables `kvCache` once on the `InferenceService`. OME injects the
provider connector into both engine and decoder, with component-specific roles
where required.

For example, an LMCache provider adapter may default the engine to `kv_both` in
non-disaggregated mode, and allow explicit prefill/decode roles for
disaggregated deployments.

#### Story 5: Independent Cache Operations

Emma is a platform operator. She needs to scale cache capacity, allocate cache
pods to CPU or memory-optimized nodes, scrape cache metrics, and inspect cache
readiness independently of engine readiness.

The `kvCache` component has its own resources, replicas, scheduling fields,
Service ports, status entry, and ready condition. Emma can tune it without
changing GPU scheduling for the engine or decoder.

### Notes/Constraints/Caveats

1. **Per-service scope:** A `kvCache` component belongs to exactly one
   `InferenceService`. OME does not share the cache component across models.

2. **Deployment in alpha:** The cache component runs as a Deployment per
   `InferenceService`. DaemonSet semantics fit cluster-level or node-level cache
   infrastructure better than a service-local component, so DaemonSet support is
   not included.

3. **Explicit opt-in:** `ServingRuntime.spec.kvCacheConfig` provides defaults,
   but does not enable the component unless `InferenceService.spec.kvCache` is
   present.

4. **RawDeployment only:** The first implementation supports only raw
   Kubernetes Deployment mode for the cache component. Engine, decoder, and
   router keep their existing deployment modes.

5. **Cache before serving:** The reconciler should create the cache Service
   before creating or updating engine and decoder pods that reference it.

6. **No model volume by default:** The cache server should not inherit model
   volume mounts or model-ready node affinity. It is a serving dependency, not a
   model loader.

7. **Provider adapter required:** OME should not hard-code LMCache arguments in
   generic component code. Provider-specific translation belongs in a provider
   adapter.

8. **Router integration deferred:** The alpha does not configure KV-cache-aware
   routing. A later enhancement can add router integration once OME's router
   semantics are clear.

### Risks and Mitigations

**Risk 1: Provider-specific fields leak into the OME API**

- *Mitigation:* Keep common cache intent in structured fields and move
  provider-specific values to `providerConfig`. Add provider adapters behind a
  stable interface.

**Risk 2: Users accidentally share cache across incompatible models**

- *Mitigation:* Do not provide an independent cache CRD in this proposal. Name,
  own, label, and garbage collect cache resources through the `InferenceService`.

**Risk 3: Cache readiness blocks otherwise functional inference**

- *Mitigation:* If users configure `spec.kvCache`, treat it as part of desired
  service readiness. Operators can remove `spec.kvCache` if cache is optional
  for a workload. Future work can add a degraded-but-serving status mode if this
  becomes necessary.

**Risk 4: Engine pods start before the cache Service is available**

- *Mitigation:* Reconcile the cache component first, derive a deterministic
  endpoint, and requeue on cache reconcile changes before reconciling target
  serving components when needed.

**Risk 5: Cache server resource sizing competes with engine GPUs**

- *Mitigation:* Do not apply engine or decoder accelerator selection to
  `kvCache`. Users can independently set node selectors, affinity, tolerations,
  resources, and priority for the cache component.

**Risk 6: Connector injection conflicts with user-provided runner args**

- *Mitigation:* Follow explicit merge rules. If the user provides a full
  `command`, OME should not rewrite the command. If the user provides `args`,
  OME can append or prepend provider-generated args according to the provider
  adapter contract, with user values taking precedence for environment
  variables.

## Design Details

### API Specifications

#### InferenceService Extensions

Add `KVCache *KVCacheSpec` to `InferenceServiceSpec`:

```go
type InferenceServiceSpec struct {
    // Existing fields omitted.

    // KVCache defines a per-InferenceService distributed KV cache component.
    // If omitted, no cache component is reconciled.
    // +optional
    KVCache *KVCacheSpec `json:"kvCache,omitempty"`
}
```

Add a new provider-neutral spec type:

```go
// KVCacheProvider identifies the provider used to implement the cache pool.
// +kubebuilder:validation:Enum=LMCache;MoonCake
type KVCacheProvider string

const (
    LMCacheProvider  KVCacheProvider = "LMCache"
    MoonCakeProvider KVCacheProvider = "MoonCake"
)

// KVCacheSpec defines a per-InferenceService cache component.
type KVCacheSpec struct {
    // PodSpec provides pod-level customization for the cache component.
    // +optional
    PodSpec `json:",inline"`

    // ComponentExtensionSpec controls replicas, scaling, PDB, labels,
    // annotations, and raw Deployment strategy.
    ComponentExtensionSpec `json:",inline"`

    // Provider selects the cache provider implementation.
    // +required
    Provider KVCacheProvider `json:"provider"`

    // Runner overrides the primary cache server container.
    // +optional
    Runner *RunnerSpec `json:"runner,omitempty"`

    // Cache describes provider-neutral cache policy intent.
    // +optional
    Cache *KVCachePolicySpec `json:"cache,omitempty"`

    // Connector describes how serving components attach to this cache.
    // +optional
    Connector *KVCacheConnectorSpec `json:"connector,omitempty"`

    // ProviderConfig contains provider-specific configuration that is not part
    // of the portable OME API. The selected provider adapter interprets it.
    // +optional
    ProviderConfig *runtime.RawExtension `json:"providerConfig,omitempty"`
}

// KVCachePolicySpec describes common cache behavior across providers.
type KVCachePolicySpec struct {
    // Capacity is the intended cache capacity. Providers translate this to
    // their native size flags or environment variables.
    // +optional
    Capacity *resource.Quantity `json:"capacity,omitempty"`

    // EvictionPolicy is the desired eviction policy, such as LRU.
    // +optional
    EvictionPolicy string `json:"evictionPolicy,omitempty"`

    // ChunkSize is the provider-neutral chunk size hint.
    // +optional
    ChunkSize *int32 `json:"chunkSize,omitempty"`
}

// KVCacheConnectorSpec controls connector injection into serving components.
type KVCacheConnectorSpec struct {
    // TargetComponents lists serving components that should attach to the cache.
    // Defaults to engine and decoder when present.
    // +optional
    // +listType=atomic
    TargetComponents []ComponentType `json:"targetComponents,omitempty"`

    // Roles maps component names to provider-neutral KV roles.
    // Examples: engine: Both, decoder: Both, engine: Prefill, decoder: Decode.
    // +optional
    Roles map[ComponentType]KVCacheRole `json:"roles,omitempty"`

    // Env provides additional connector environment variables.
    // User-provided component env values take precedence on name conflicts.
    // +optional
    // +listType=map
    // +listMapKey=name
    Env []corev1.EnvVar `json:"env,omitempty"`

    // Args provides additional connector args.
    // Provider-generated args are applied before user component args unless the
    // provider adapter documents otherwise.
    // +optional
    // +listType=atomic
    Args []string `json:"args,omitempty"`
}

// +kubebuilder:validation:Enum=Prefill;Decode;Both
type KVCacheRole string
```

The initial provider enum can include only `LMCache` if the first
implementation does not yet support MoonCake. The type should be shaped to add
MoonCake later without changing the field name or component model.

#### ServingRuntime Extensions

Add `KVCacheConfig *KVCacheSpec` to `ServingRuntimeSpec`:

```go
type ServingRuntimeSpec struct {
    // Existing fields omitted.

    // KVCacheConfig provides runtime defaults for the InferenceService kvCache
    // component. This field does not enable kvCache by itself.
    // +optional
    KVCacheConfig *KVCacheSpec `json:"kvCacheConfig,omitempty"`
}
```

Merge behavior should match the intent of existing component defaults:

1. If `InferenceService.spec.kvCache` is nil, the merged KV cache spec is nil.
2. If runtime `kvCacheConfig` is nil, use a deep copy of service `kvCache`.
3. If both are present, strategic-merge runtime defaults with service overrides.
4. Service values win over runtime values.

This keeps runtime authors in control of default images, ports, and connector
behavior while keeping cache creation a service-owner decision.

#### Status Extensions

Add a new component type and condition:

```go
const (
    KVCacheComponent ComponentType = "kvCache"
)

const (
    KVCacheReady apis.ConditionType = "KVCacheReady"
)
```

When `spec.kvCache` is present, `status.components.kvCache` should report the
cache component URL and revision details consistently with other raw
Deployment-backed components.

For raw deployment mode, the service `Ready` condition should require
`KVCacheReady=True` when `kvCache` is configured. The component should also be
included in cleanup and status pruning when it is removed from the spec.

### Example Configuration

#### InferenceService with LMCache

```yaml
apiVersion: ome.io/v1beta1
kind: InferenceService
metadata:
  name: qwen3-8b
spec:
  model:
    name: qwen3-8b
  runtime:
    name: vllm-qwen

  engine:
    minReplicas: 2
    runner:
      resources:
        limits:
          nvidia.com/gpu: "1"

  kvCache:
    provider: LMCache
    minReplicas: 1
    maxReplicas: 2
    cache:
      capacity: 20Gi
      evictionPolicy: LRU
      chunkSize: 256
    runner:
      image: lmcache/lmcache:latest
      command:
        - lmcache
      args:
        - server
      ports:
        - name: transfer
          containerPort: 5555
        - name: http
          containerPort: 8080
      resources:
        requests:
          cpu: "4"
          memory: 24Gi
        limits:
          cpu: "8"
          memory: 32Gi
    nodeSelector:
      node-type: memory-optimized
    connector:
      targetComponents:
        - engine
      roles:
        engine: Both
    providerConfig:
      lmcache:
        mode: MP
        connector: LMCacheMPConnector
        httpPortName: http
        transferPortName: transfer
```

Expected owned resources:

```text
Deployment/qwen3-8b-kv-cache
Service/qwen3-8b-kv-cache
HorizontalPodAutoscaler/qwen3-8b-kv-cache
PodDisruptionBudget/qwen3-8b-kv-cache
```

Expected status shape:

```yaml
status:
  components:
    kvCache:
      url: http://qwen3-8b-kv-cache.default.svc.cluster.local
  conditions:
    - type: KVCacheReady
      status: "True"
```

#### ServingRuntime Defaults

```yaml
apiVersion: ome.io/v1beta1
kind: ClusterServingRuntime
metadata:
  name: vllm-qwen
spec:
  supportedModelFormats:
    - name: safetensors
      autoSelect: true
  engineConfig:
    runner:
      image: vllm/vllm-openai:latest
  kvCacheConfig:
    provider: LMCache
    cache:
      evictionPolicy: LRU
      chunkSize: 256
    runner:
      image: lmcache/lmcache:latest
      command:
        - lmcache
      args:
        - server
      ports:
        - name: transfer
          containerPort: 5555
        - name: http
          containerPort: 8080
    connector:
      targetComponents:
        - engine
      roles:
        engine: Both
```

The runtime default above does not create a cache server unless the
`InferenceService` includes `spec.kvCache`.

### Component Lifecycle

The `kvCache` component lifecycle is bound to the `InferenceService`:

1. Creating an `InferenceService` with `spec.kvCache` creates cache resources.
2. Updating `spec.kvCache` updates the cache Deployment, Service, autoscaling,
   and connector injection into target components.
3. Removing `spec.kvCache` deletes cache resources through component cleanup and
   removes connector settings from target components on the next rollout.
4. Deleting the `InferenceService` deletes the cache resources through owner
   references and finalizer cleanup.

The cache component should not own or outlive the model endpoint. If users need
long-lived, node-local, or cross-service cache infrastructure, that should be a
separate future proposal.

### Reconciliation Flow

The `InferenceService` reconciler should extend the existing component path:

1. Reconcile the BaseModel and select or validate the ServingRuntime.
2. Merge runtime and service specs:
   - `mergedEngine`
   - `mergedDecoder`
   - `mergedRouter`
   - `mergedKVCache`
3. Determine deployment modes:
   - engine: existing logic
   - decoder: existing logic
   - router: existing logic
   - kvCache: `RawDeployment` in alpha
4. Build a deterministic cache endpoint from `<isvc>-kv-cache` and the
   configured Service ports.
5. If `mergedKVCache` is present, reconcile the cache component first.
6. Use the provider adapter to inject connector settings into merged engine and
   decoder specs.
7. Reconcile engine, decoder, and router through the selected workload strategy.
8. Reconcile ingress and external service using the existing entrypoint rules.
9. Propagate status for all configured components, including `kvCache`.
10. Cleanup removed components, including `kvCache`.

The `WorkloadReconcileRequest` should add:

```go
MergedKVCache *v1beta1.KVCacheSpec
KVCacheDeploymentMode constants.DeploymentModeType
KVCacheEndpoint *components.KVCacheEndpoint
```

The `ComponentDeploymentModes` struct should add:

```go
KVCache constants.DeploymentModeType
```

The `SingleComponentStrategy` should reconcile `kvCache` before engine and
decoder. This ordering ensures the Service exists before serving pods start with
connector configuration.

### Provider Adapter Contract

Introduce an internal provider adapter interface, for example:

```go
type KVCacheProviderAdapter interface {
    Provider() v1beta1.KVCacheProvider
    DefaultPorts(spec *v1beta1.KVCacheSpec) []corev1.ContainerPort
    BuildServerContainer(spec *v1beta1.KVCacheSpec) (*corev1.Container, error)
    BuildEndpoint(isvc *v1beta1.InferenceService, spec *v1beta1.KVCacheSpec) (*KVCacheEndpoint, error)
    InjectConnector(target v1beta1.ComponentType, spec *v1beta1.KVCacheSpec, endpoint *KVCacheEndpoint, podSpec *v1beta1.PodSpec, runner *v1beta1.RunnerSpec) error
    Validate(spec *v1beta1.KVCacheSpec) field.ErrorList
}
```

Provider adapters translate portable OME intent into provider-specific command
args, environment variables, ports, and connector configuration. The generic
`KVCacheReconciler` owns Kubernetes resources and status. Provider adapters own
provider semantics.

For LMCache alpha support, the adapter should:

1. Default transfer and HTTP ports when not explicitly provided.
2. Translate `cache.capacity`, `cache.evictionPolicy`, and `cache.chunkSize`
   into LMCache server args or environment variables.
3. Generate the cache server endpoint consumed by vLLM.
4. Inject the vLLM KV transfer config for the selected connector.
5. Support provider-specific overrides through `providerConfig.lmcache`.

### Connector Injection

Connector injection is the bridge between the cache server component and the
serving components.

The initial LMCache adapter should support vLLM engine and decoder components.
For vLLM, injection can include:

1. A `--kv-transfer-config` argument with the provider-selected connector and
   component role.
2. Environment variables for LMCache endpoint, serde, logging, timeouts, or
   experimental mode when required by the chosen LMCache version.
3. Optional component-specific roles for prefill-decode deployments.

Merge rules:

1. User-provided component env values win on name conflicts.
2. Provider-generated env is added only when missing.
3. If the component runner has a full `command`, OME does not attempt to parse
   and rewrite shell command strings.
4. If the component runner uses structured `args`, provider-generated connector
   args are applied in a deterministic order.
5. ProviderConfig may override the default connector class or extra connector
   JSON when the serving runtime requires a specific vLLM or LMCache version.

The adapter should mutate only the target components listed in
`spec.kvCache.connector.targetComponents`. If the list is empty, default targets
are:

1. `engine` when only engine is present.
2. `engine` and `decoder` when decoder is present.
3. `router` is not targeted in alpha.

### Scheduling and Resource Management

`kvCache` should use the same PodSpec and ComponentExtensionSpec patterns as
other components, but it should not inherit model-specific or accelerator-
specific placement by default.

The cache server:

1. Uses its own `resources`, `nodeSelector`, `affinity`, `tolerations`,
   `schedulerName`, `priorityClassName`, and `imagePullSecrets`.
2. Does not receive BaseModel hostPath mounts unless the user explicitly adds
   volumes and mounts.
3. Does not receive engine or decoder `AcceleratorClass` placement.
4. Can run on CPU or memory-optimized nodes while engine and decoder run on GPU
   nodes.
5. Uses `minReplicas`, `maxReplicas`, deployment strategy, PDB, and autoscaler
   behavior available through the existing raw deployment reconciler.

This keeps cache sizing independent from model-serving GPU sizing.

### Readiness, Status, and Failure Behavior

When `spec.kvCache` is present, the controller should create and update:

1. `status.components.kvCache`
2. `KVCacheReady`
3. Cache component events for validation, provider adapter, and reconcile
   failures
4. Controller metrics for cache reconciliation and provider adapter errors

The service should be considered not ready if `KVCacheReady=False` when
`spec.kvCache` is configured. This makes cache configuration an explicit part of
the desired service state. Silent fallback to no distributed cache would be hard
to detect and would undermine the performance SLO.

Provider adapter validation should fail fast for invalid configurations such as:

1. Unsupported provider.
2. Missing runner image when no runtime default provides one.
3. Connector targets that are not present in the `InferenceService`.
4. Required provider ports missing or duplicated.
5. ProviderConfig that cannot be decoded by the selected adapter.

### Observability

OME should expose controller-level metrics for the new component:

1. `ome_kvcache_reconcile_total`
2. `ome_kvcache_reconcile_duration_seconds`
3. `ome_kvcache_ready`
4. `ome_kvcache_connector_injection_total`
5. `ome_kvcache_provider_errors_total`

The cache server Service should preserve provider metrics ports when configured
on the runner. For LMCache, the HTTP management or metrics endpoint can be
exposed as a named Service port and scraped by the platform's metrics stack.

The component should also use standard OME labels:

```text
component=kvCache
ome.io/inferenceservice=<isvc-name>
```

### Security

The cache Service should default to `ClusterIP`. It should not be exposed
through the top-level inference ingress. Cache traffic is internal service
traffic between serving components and the cache component.

Security considerations:

1. Do not expose provider management endpoints externally by default.
2. Preserve pod security context and container security context overrides.
3. Allow NetworkPolicy integration through labels.
4. Avoid writing secrets into providerConfig; use Kubernetes Secret references
   if providers require credentials in future iterations.
5. Do not share a cache Service across namespaces or across `InferenceService`
   objects.

### Backward Compatibility

This proposal is backward compatible:

1. Existing `InferenceService` objects omit `spec.kvCache` and behave the same.
2. Existing `ServingRuntime` objects omit `spec.kvCacheConfig` and behave the
   same.
3. Existing engine, decoder, and router specs continue to use the current merge
   and deployment mode logic.
4. New status fields appear only when the component is configured.
5. No existing CRD is removed or renamed.

The feature should be guarded by normal API review and can optionally be gated
by a controller feature flag during alpha if the project wants staged rollout.

### Implementation Plan

1. Add API types:
   - `InferenceServiceSpec.KVCache`
   - `ServingRuntimeSpec.KVCacheConfig`
   - `KVCacheSpec`, `KVCachePolicySpec`, `KVCacheConnectorSpec`, `KVCacheRole`
   - `KVCacheComponent`, `KVCacheReady`

2. Add code generation updates:
   - `make generate`
   - `make manifests`

3. Add merge and deployment mode utilities:
   - `MergeKVCacheSpec`
   - `MergeRuntimeSpecs` returns `mergedKVCache`
   - `DetermineDeploymentModes` or a companion method returns `RawDeployment`
     for `kvCache`

4. Add provider adapter package:
   - generic adapter registry
   - LMCache adapter
   - validation helpers
   - connector injection helpers

5. Add `KVCache` component reconciler:
   - build metadata using `<isvc>-kv-cache`
   - process labels and annotations
   - build pod spec from `KVCacheSpec`
   - call raw Deployment reconciler
   - update `status.components.kvCache`

6. Extend workload reconciliation:
   - add merged `kvCache` to `WorkloadReconcileRequest`
   - reconcile cache before engine and decoder
   - inject connector settings before creating serving component reconcilers

7. Extend status and cleanup:
   - condition maps include `KVCacheComponent`
   - status cleanup removes `kvCache`
   - resource cleanup treats `kvCache` as an active component when present

8. Add samples:
   - LMCache MP with vLLM engine
   - LMCache with engine and decoder
   - runtime default plus service opt-in

9. Add documentation:
   - API reference
   - operational guidance
   - troubleshooting for connector injection and cache readiness

### Test Plan

[x] I/we understand that component owners may require updates to existing tests
before accepting changes necessary for this enhancement.

##### Prerequisite Testing Updates

Existing component tests assume the component set is engine, decoder, router,
and predictor. Tests for status maps, cleanup, and workload reconciliation must
be updated to include `kvCache` without changing behavior when it is absent.

#### Unit Tests

- `pkg/apis/ome/v1beta1`: verify defaults, validation markers, deep copy, and
  OpenAPI generation for new KV cache fields.
- `pkg/controller/v1beta1/inferenceservice/utils`: test `MergeKVCacheSpec`,
  nil semantics, runtime default merging, and service override precedence.
- `pkg/controller/v1beta1/inferenceservice/utils`: test cache deployment mode
  selection and validation.
- `pkg/controller/v1beta1/inferenceservice/components`: test KV cache metadata,
  labels, annotations, pod spec construction, default ports, and raw deployment
  reconciliation calls.
- `pkg/controller/v1beta1/inferenceservice/components`: test that cache pods do
  not inherit model volume mounts or accelerator node selectors by default.
- `pkg/controller/v1beta1/inferenceservice/status`: test `KVCacheReady` mapping,
  status component initialization, and ready aggregation.
- `pkg/controller/v1beta1/inferenceservice`: test cleanup when `spec.kvCache`
  is removed.
- Provider adapter package: test LMCache server args, endpoint construction,
  connector env injection, connector args injection, providerConfig overrides,
  and invalid providerConfig errors.
- `pkg/controller/v1beta1/inferenceservice/workload`: test that
  `SingleComponentStrategy` reconciles `kvCache` before engine and decoder.

#### Integration Tests

1. Create an `InferenceService` with `spec.kvCache.provider: LMCache` and verify
   that the controller creates Deployment, Service, HPA, and PDB with owner
   references to the `InferenceService`.
2. Verify engine Deployment contains provider-injected connector args and env
   when `kvCache` targets engine.
3. Verify engine and decoder both receive connector settings in a
   prefill-decode service.
4. Verify deleting the `InferenceService` deletes the cache resources.
5. Verify removing `spec.kvCache` deletes only the cache component and rolls out
   serving components without connector injection.
6. Verify runtime `kvCacheConfig` does not enable cache by itself.
7. Verify `KVCacheReady=False` blocks service readiness when the cache
   Deployment is unavailable.

### Graduation Criteria

#### Alpha

1. `spec.kvCache` and `spec.kvCacheConfig` are available in the v1beta1 API.
2. LMCache provider adapter supports a Deployment-backed cache server.
3. Engine connector injection works for vLLM single-component deployments.
4. Component status and cleanup work correctly.
5. Samples and basic documentation are available.

#### Beta

1. Prefill-decode engine and decoder connector roles are validated with
   integration tests.
2. Cache metrics and readiness are documented and surfaced consistently.
3. ProviderConfig validation produces actionable user errors.
4. Operational feedback from at least one real model deployment is incorporated.
5. Optional MoonCake provider design is validated or explicitly deferred.

#### Stable

1. The API has proven sufficient for at least two provider implementations or
   one provider plus a well-reviewed second-provider design.
2. Upgrade and rollback behavior is documented.
3. Performance and failure-mode guidance is documented for production use.
4. The feature is enabled by default, if it was feature-gated during alpha.

## Implementation History

- 2026-05-09: Initial OEP drafted.

## Drawbacks

Adding a new component increases the size of the `InferenceService` API and the
number of resources reconciled per service. Operators must now reason about
cache server sizing, readiness, and failure modes in addition to engine,
decoder, and router.

Connector injection also introduces provider-specific complexity into the
controller, even if that complexity is isolated behind an adapter. Provider
versions and serving runtime versions may require different connector names or
configuration payloads, so tests and documentation must track those
compatibility constraints.

Finally, making cache readiness part of service readiness may block an inference
endpoint that could technically serve without distributed cache. This is
intentional for the initial design because a configured cache is a declared
performance dependency, but it may need a more nuanced degraded mode later.

## Alternatives

### Standalone CacheServer CRD

A separate `CacheServer` or `KVCacheServer` CRD would allow independent cache
lifecycle management and could support shared cache infrastructure. This matches
some external operator designs.

This proposal does not choose that approach because OME's desired cache scope is
per `InferenceService`. A standalone CRD invites cross-model sharing and adds a
second ownership boundary that users must manage. It also complicates deletion:
removing an `InferenceService` might leave a cache server running unless extra
ownership or reference tracking is added.

### DaemonSet Per InferenceService

A DaemonSet could place cache pods on every selected node, making cache capacity
node-local. This can be attractive for infrastructure-level cache layers.

This proposal does not choose that approach because a DaemonSet is a poor fit
for a service-local optional component. If an `InferenceService` is deleted, its
DaemonSet disappears, and nodes no longer have that cache process. That behavior
is correct for service ownership but wasteful and confusing for node-level cache
semantics. DaemonSet is better suited to an independently managed cluster or
node cache resource, which is out of scope.

### Sidecar in Engine or Decoder Pods

The cache provider could run as a sidecar in each engine or decoder pod. This
keeps lifecycle local to serving pods and avoids another Service.

This proposal does not choose that approach because it does not provide a shared
cache pool across engine replicas. It also makes independent cache scaling and
observability harder.

### Runtime-Only LMCache Configuration

OME could add LMCache-specific fields to `ServingRuntime` and only inject vLLM
arguments, leaving users to deploy the cache server separately.

This proposal does not choose that approach because it splits responsibility
between OME and manual Kubernetes resources. It also fails to give users a
single `InferenceService` view of cache readiness, ownership, and cleanup.

### Provider-Specific `lmCache` Field

OME could add `spec.lmCache` with fields that directly mirror LMCache server
and connector settings.

This proposal does not choose that approach because it hard-codes one provider
into the API and would force either duplicate fields or breaking changes when
MoonCake or another provider is added.

### Router-Only KV-Aware Routing

OME could start by adding KV-cache-aware routing to the router instead of
reconciling a cache component.

This proposal does not choose that approach for alpha because the cache server
lifecycle and connector wiring are prerequisites. Router-aware routing can be a
later enhancement once the component exists and exposes provider status or
metrics that the router can consume.
