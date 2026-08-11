package controllerconfig

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// MultiClusterConfigName is the inferenceservice-config ConfigMap key holding
// the multi-cluster tuning block.
const MultiClusterConfigName = "multicluster"

// +kubebuilder:object:generate=false
// MultiClusterConfig holds operator-level tuning for the multi-cluster fan-out
// layer (the WorkloadCluster connection/transport, the placement controller,
// and the global endpoint publisher). It is loaded once at manager startup, from
// the inferenceservice-config ConfigMap, only when multi-cluster is enabled.
//
// Topology, role, identity, and security stay manager flags, not config: they
// decide which controllers run and the manager's identity, so they are
// deploy-time decisions (a restart), not hot-tunable config.
//
// Every field degrades gracefully when omitted. Durations are stored as strings
// and parsed by the *Duration() accessors, which yield 0 on an empty or
// unparsable value; a zero handed to the workloadcluster/placement options
// makes those packages apply their OWN in-package default. So the default for
// each knob stays single-sourced in the package that owns it — never duplicated
// as a literal here — and an absent "multicluster" block reproduces the
// built-in behavior exactly.
type MultiClusterConfig struct {
	WorkloadCluster WorkloadClusterConfig `json:"workloadCluster,omitempty"`
	Placement       PlacementConfig       `json:"placement,omitempty"`
	Endpoint        EndpointConfig        `json:"endpoint,omitempty"`
}

// +kubebuilder:object:generate=false
// WorkloadClusterConfig tunes the remote-cluster connection and transport layer
// and the cross-cluster status watch funnel.
type WorkloadClusterConfig struct {
	// ClientQPS / ClientBurst are the steady-state request rate and burst to each
	// remote workload-cluster apiserver. Zero leaves the client-go default.
	ClientQPS   float64 `json:"clientQPS,omitempty"`
	ClientBurst int     `json:"clientBurst,omitempty"`
	// PerCallTimeout bounds one remote request. Empty leaves no client-level timeout.
	PerCallTimeout string `json:"perCallTimeout,omitempty"`
	// CacheEnabled serves cross-cluster derived-InferenceService reads from a
	// per-cluster informer cache instead of live apiserver reads, and is the
	// prerequisite event source for the status watch funnel. False (the default)
	// keeps reads live and the funnel off.
	CacheEnabled bool `json:"cacheEnabled,omitempty"`
	// HealthInterval is the re-probe cadence for an otherwise-idle WorkloadCluster.
	HealthInterval string `json:"healthInterval,omitempty"`
	// ConnectionGrace is how long a previously reachable cluster tolerates transient
	// probe failures before it is flipped to Ready=False and disconnected.
	ConnectionGrace string `json:"connectionGrace,omitempty"`
	// EventsBatchPeriod debounces a rotated-kubeconfig Secret's burst of key updates
	// into one reconcile.
	EventsBatchPeriod string `json:"eventsBatchPeriod,omitempty"`
	// EstablishInitial / EstablishMax bound a single remote watch-establish attempt
	// (the timeout grows from initial to max). ReconnectRetryMax caps the
	// inter-attempt backoff for an unreachable cluster.
	EstablishInitial  string `json:"establishInitial,omitempty"`
	EstablishMax      string `json:"establishMax,omitempty"`
	ReconnectRetryMax string `json:"reconnectRetryMax,omitempty"`
	// FunnelResyncInterval is how often the status funnel reconciles its per-cluster
	// watch set against the connected clusters (how fast a newly-connected cluster
	// is noticed, not the status latency). FunnelBufferSize is the depth of its
	// buffered event channel; a full channel drops events (the safety requeue
	// recovers).
	FunnelResyncInterval string `json:"funnelResyncInterval,omitempty"`
	FunnelBufferSize     int    `json:"funnelBufferSize,omitempty"`
}

// +kubebuilder:object:generate=false
// PlacementConfig tunes the fan-out placement controller, its status
// convergence, and its orphan GC.
type PlacementConfig struct {
	// RequeueInterval is the status-refresh poll cadence. With the cache/funnel off
	// it also paces the cross-cluster status re-read.
	RequeueInterval string `json:"requeueInterval,omitempty"`
	// GCInterval is the orphan-sweep cadence for the placement GC runnable.
	GCInterval string `json:"gcInterval,omitempty"`
	// MaxConcurrentReconciles caps placement reconciles in parallel (distinct
	// ISVCs). Zero falls back to controller-runtime's single worker.
	MaxConcurrentReconciles int `json:"maxConcurrentReconciles,omitempty"`
	// FanoutTimeout is the per-cluster deadline bounding a single fan-out apply, so
	// one slow remote cannot block placement to healthy peers.
	FanoutTimeout string `json:"fanoutTimeout,omitempty"`
	// WinnerLostGrace is the grace window held before re-placing when the sticky
	// winner's derived is absent on a still-connected winner. Empty re-places
	// immediately.
	WinnerLostGrace string `json:"winnerLostGrace,omitempty"`
	// StatusBatchPeriod debounces a burst of cross-cluster derived-status events for
	// one ISVC into a single placement reconcile.
	StatusBatchPeriod string `json:"statusBatchPeriod,omitempty"`
	// StatusSafetyRequeue is the steady-state re-read backstop when the funnel is on
	// (events drive freshness; this only recovers a missed event).
	StatusSafetyRequeue string `json:"statusSafetyRequeue,omitempty"`
	// DispatcherMode is the fan-out breadth policy: "AllAtOnce" clones onto every
	// matched candidate at once; "Incremental" probes candidates in batches. Empty
	// or unrecognized means AllAtOnce.
	DispatcherMode string `json:"dispatcherMode,omitempty"`
	// DispatcherStepSize is the candidates the Incremental dispatcher adds per round.
	// Non-positive advances by one.
	DispatcherStepSize int `json:"dispatcherStepSize,omitempty"`
	// DispatcherRoundTimeout is how long an Incremental round waits for a nominated
	// cluster to win before adding the next batch. Non-positive enforces no dwell.
	DispatcherRoundTimeout string `json:"dispatcherRoundTimeout,omitempty"`
}

// +kubebuilder:object:generate=false
// EndpointConfig tunes the global endpoint publisher (a Gateway API HTTPRoute to
// the placement winner). The host template, gateway, and namespace are
// deployment-identity values with no in-code default: empty disables publishing
// (a no-op), never a baked-in gateway or host.
type EndpointConfig struct {
	// GlobalHostTemplate is the text/template for the global host an ISVC publishes
	// to the winner. Empty means only ISVCs carrying the global-host annotation
	// publish.
	GlobalHostTemplate string `json:"globalHostTemplate,omitempty"`
	// GlobalGateway is the "namespace/name" of the global-traffic Gateway the
	// published HTTPRoute attaches to. Empty makes the publisher a no-op.
	GlobalGateway string `json:"globalGateway,omitempty"`
	// RouteNamespace is the namespace for the published HTTPRoute and backing
	// Service. Empty uses the ISVC's own namespace.
	RouteNamespace string `json:"routeNamespace,omitempty"`
	// BackendPort is the port on the winner cluster's ingress the global host
	// forwards to.
	BackendPort int `json:"backendPort,omitempty"`
}

// NewMultiClusterConfig loads the "multicluster" block from the
// inferenceservice-config ConfigMap. It is read once at manager startup (the
// multi-cluster wiring is built before any reconcile), so there is no
// ConfigCache-backed variant. An absent block yields a zero-valued config and
// every consumer applies its own in-package default.
func NewMultiClusterConfig(clientset kubernetes.Interface) (*MultiClusterConfig, error) {
	configMap, err := getInferenceServiceConfigMap(clientset)
	if err != nil {
		return nil, err
	}
	return parseMultiClusterConfig(configMap)
}

func parseMultiClusterConfig(configMap *v1.ConfigMap) (*MultiClusterConfig, error) {
	cfg := &MultiClusterConfig{}
	if err := getComponentConfig(MultiClusterConfigName, configMap, cfg); err != nil {
		return nil, fmt.Errorf("unable to parse multicluster config json: %w", err)
	}
	return cfg, nil
}

// PerCallTimeoutDuration returns the parsed PerCallTimeout (0 if absent/unparsable).
func (c WorkloadClusterConfig) PerCallTimeoutDuration() time.Duration {
	return parseDurationOrZero(c.PerCallTimeout)
}

// HealthIntervalDuration returns the parsed HealthInterval (0 if absent/unparsable).
func (c WorkloadClusterConfig) HealthIntervalDuration() time.Duration {
	return parseDurationOrZero(c.HealthInterval)
}

// ConnectionGraceDuration returns the parsed ConnectionGrace (0 if absent/unparsable).
func (c WorkloadClusterConfig) ConnectionGraceDuration() time.Duration {
	return parseDurationOrZero(c.ConnectionGrace)
}

// EventsBatchPeriodDuration returns the parsed EventsBatchPeriod (0 if absent/unparsable).
func (c WorkloadClusterConfig) EventsBatchPeriodDuration() time.Duration {
	return parseDurationOrZero(c.EventsBatchPeriod)
}

// EstablishInitialDuration returns the parsed EstablishInitial (0 if absent/unparsable).
func (c WorkloadClusterConfig) EstablishInitialDuration() time.Duration {
	return parseDurationOrZero(c.EstablishInitial)
}

// EstablishMaxDuration returns the parsed EstablishMax (0 if absent/unparsable).
func (c WorkloadClusterConfig) EstablishMaxDuration() time.Duration {
	return parseDurationOrZero(c.EstablishMax)
}

// ReconnectRetryMaxDuration returns the parsed ReconnectRetryMax (0 if absent/unparsable).
func (c WorkloadClusterConfig) ReconnectRetryMaxDuration() time.Duration {
	return parseDurationOrZero(c.ReconnectRetryMax)
}

// FunnelResyncIntervalDuration returns the parsed FunnelResyncInterval (0 if absent/unparsable).
func (c WorkloadClusterConfig) FunnelResyncIntervalDuration() time.Duration {
	return parseDurationOrZero(c.FunnelResyncInterval)
}

// RequeueIntervalDuration returns the parsed RequeueInterval (0 if absent/unparsable).
func (c PlacementConfig) RequeueIntervalDuration() time.Duration {
	return parseDurationOrZero(c.RequeueInterval)
}

// GCIntervalDuration returns the parsed GCInterval (0 if absent/unparsable).
func (c PlacementConfig) GCIntervalDuration() time.Duration {
	return parseDurationOrZero(c.GCInterval)
}

// FanoutTimeoutDuration returns the parsed FanoutTimeout (0 if absent/unparsable).
func (c PlacementConfig) FanoutTimeoutDuration() time.Duration {
	return parseDurationOrZero(c.FanoutTimeout)
}

// WinnerLostGraceDuration returns the parsed WinnerLostGrace (0 if absent/unparsable).
func (c PlacementConfig) WinnerLostGraceDuration() time.Duration {
	return parseDurationOrZero(c.WinnerLostGrace)
}

// StatusBatchPeriodDuration returns the parsed StatusBatchPeriod (0 if absent/unparsable).
func (c PlacementConfig) StatusBatchPeriodDuration() time.Duration {
	return parseDurationOrZero(c.StatusBatchPeriod)
}

// StatusSafetyRequeueDuration returns the parsed StatusSafetyRequeue (0 if absent/unparsable).
func (c PlacementConfig) StatusSafetyRequeueDuration() time.Duration {
	return parseDurationOrZero(c.StatusSafetyRequeue)
}

// DispatcherRoundTimeoutDuration returns the parsed DispatcherRoundTimeout (0 if absent/unparsable).
func (c PlacementConfig) DispatcherRoundTimeoutDuration() time.Duration {
	return parseDurationOrZero(c.DispatcherRoundTimeout)
}

// parseDurationOrZero parses s, returning 0 when it is empty, malformed, or
// non-positive. Callers hand the zero to a workloadcluster/placement option,
// which then applies its own in-package default — so the fallback stays
// single-sourced in the consuming package, not duplicated here.
func parseDurationOrZero(s string) time.Duration {
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return d
	}
	return 0
}
