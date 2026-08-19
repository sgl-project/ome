package workloadcluster

import (
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
)

// ClientTuning carries the rate-limit and per-call timeout applied to every
// remote workload-cluster client. The control plane talks to N remote
// apiservers concurrently, so the client-go default QPS/Burst (5/10) throttles
// fan-out reconciles; the operative values are config-driven (manager flag /
// chart) with NO in-code default — the zero value leaves client-go's own
// defaults in place, degrading gracefully rather than baking a magic number in.
type ClientTuning struct {
	// QPS is the steady-state request rate to a remote apiserver. Zero leaves
	// the client-go default.
	QPS float32
	// Burst is the token-bucket burst over QPS. Zero leaves the client-go default.
	Burst int
	// PerCallTimeout bounds a single remote request (rest.Config.Timeout) so one
	// hung remote cannot wedge a reconcile worker indefinitely. Zero leaves no
	// client-level timeout (callers still pass request contexts).
	PerCallTimeout time.Duration
}

// apply writes the non-zero tuning fields onto cfg. A zero field is left as-is
// so client-go's own default applies (graceful degradation, no magic literal).
func (t ClientTuning) apply(cfg *rest.Config) {
	if t.QPS > 0 {
		cfg.QPS = t.QPS
	}
	if t.Burst > 0 {
		cfg.Burst = t.Burst
	}
	if t.PerCallTimeout > 0 {
		cfg.Timeout = t.PerCallTimeout
	}
}

// CacheOptions configures the remote informer cache for a workload-cluster
// client. It is supplied by the caller (the control-plane wiring) rather than
// hardcoded here: this package must not import the placement package (which
// already imports it), and the cached kinds / origin selector are placement
// concerns. An empty CachedKinds set selects the never-caching client, so the
// transport degrades gracefully when no cache is wired.
type CacheOptions struct {
	// CachedKinds are the GroupKinds served from the background informer cache
	// instead of live reads. Pods are rejected by NewSelectivelyCachingClient.
	CachedKinds sets.Set[schema.GroupKind]
	// DefaultSelector scopes every cached kind to a label selector (the
	// placement-origin selector, so the cache holds only this control plane's
	// derived objects — not every object of the kind on the remote cluster).
	// Nil caches unfiltered.
	DefaultSelector labels.Selector
	// Indexes are field indexes registered on the cache before it starts.
	Indexes []CacheIndexOption
}

// enabled reports whether a real (selectively-caching) client should be built.
func (o CacheOptions) enabled() bool {
	return o.CachedKinds.Len() > 0
}

// reconnectBackoff bundles the two backoffs that govern (re)establishing a
// remote connection, mirroring the Kueue MultiKueue cluster controller: an
// establish backoff that bounds how long the initial watch attempt waits before
// it is abandoned and retried, and a retry backoff that spaces failed
// connection attempts. The operative values are config-driven; the constructor
// supplies graceful-degradation fallbacks (see defaultReconnectBackoff).
type reconnectBackoff struct {
	// establishInitial is the first establish-watch timeout; it grows by
	// establishFactor up to establishMax.
	establishInitial time.Duration
	establishMax     time.Duration
	establishFactor  float64
	// retryInitial is the first inter-attempt retry delay; it doubles up to
	// retryMax. retryInitial==0 means "retry immediately on the first failure".
	retryInitial time.Duration
	retryMax     time.Duration
}

// controllerConfig is the resolved, internal timing configuration the
// reconciler runs with. It is assembled from functional Options at Setup; every
// field has a graceful-degradation fallback (the Default* consts) applied by
// resolveConfig so an unset option never injects a magic literal mid-package.
type controllerConfig struct {
	// healthInterval is the steady-state re-probe cadence.
	healthInterval time.Duration
	// connectionGracePeriod (a.k.a. worker-lost timeout) is how long a
	// previously-reachable cluster tolerates transient probe failures before it
	// is flipped Ready=False and disconnected.
	connectionGracePeriod time.Duration
	// probeTimeout bounds the default reachability probe's single request.
	probeTimeout time.Duration
	// eventsBatchPeriod debounces Secret-change re-enqueues: a rotated kubeconfig
	// Secret often updates several keys in quick succession, and batching folds
	// that burst into one reconcile.
	eventsBatchPeriod time.Duration
	// reconnect governs (re)establish/retry backoff for the remote transport.
	reconnect reconnectBackoff
}

// Option mutates the reconciler's timing configuration at Setup: the
// events-batch debounce and the reconnect backoff. The health interval and
// worker-lost grace are set via the Reconciler struct fields and seeded into the
// config by resolveConfig.
type Option func(*controllerConfig)

// WithEventsBatchPeriod overrides the Secret-change debounce window.
func WithEventsBatchPeriod(d time.Duration) Option {
	return func(c *controllerConfig) { c.eventsBatchPeriod = d }
}

// WithReconnectBackoff overrides the establish/retry backoff for the remote
// transport. Any non-positive component falls back to its default in
// resolveConfig.
func WithReconnectBackoff(b ReconnectBackoffConfig) Option {
	return func(c *controllerConfig) {
		if b.EstablishInitial > 0 {
			c.reconnect.establishInitial = b.EstablishInitial
		}
		if b.EstablishMax > 0 {
			c.reconnect.establishMax = b.EstablishMax
		}
		if b.EstablishFactor > 0 {
			c.reconnect.establishFactor = b.EstablishFactor
		}
		// retryInitial may legitimately be 0 ("retry immediately"); only a
		// negative is nonsense and ignored.
		if b.RetryInitial >= 0 {
			c.reconnect.retryInitial = b.RetryInitial
		}
		if b.RetryMax > 0 {
			c.reconnect.retryMax = b.RetryMax
		}
	}
}

// ReconnectBackoffConfig is the public shape callers pass to
// WithReconnectBackoff (mirrors reconnectBackoff but is exported / flag-friendly).
type ReconnectBackoffConfig struct {
	EstablishInitial time.Duration
	EstablishMax     time.Duration
	EstablishFactor  float64
	RetryInitial     time.Duration
	RetryMax         time.Duration
}

// defaultReconnectBackoff is the graceful-degradation fallback for the remote
// transport backoff. It mirrors the Kueue MultiKueue schedule: establish starts
// at 1m and doubles to a 10m cap; retry starts immediately (0) and doubles in
// 5s increments up to ~5m20s. These are fallbacks only — the operative values
// are config-driven via WithReconnectBackoff.
func defaultReconnectBackoff() reconnectBackoff {
	return reconnectBackoff{
		establishInitial: DefaultEstablishInitialTimeout,
		establishMax:     DefaultEstablishMaxTimeout,
		establishFactor:  defaultEstablishFactor,
		retryInitial:     0,
		retryMax:         DefaultReconnectRetryMax,
	}
}

// resolveConfig builds the internal controllerConfig from options, then fills
// any still-unset field with its graceful-degradation default. Fields the
// Reconciler already carries (HealthInterval, ConnectionGracePeriod) seed the
// config so the existing struct-field API keeps working; an Option, when given,
// wins over the seeded field.
func (r *Reconciler) resolveConfig(opts ...Option) controllerConfig {
	c := controllerConfig{
		healthInterval:        r.HealthInterval,
		connectionGracePeriod: r.ConnectionGracePeriod,
		probeTimeout:          r.ProbeTimeout,
		reconnect:             defaultReconnectBackoff(),
	}
	for _, o := range opts {
		o(&c)
	}
	if c.healthInterval <= 0 {
		c.healthInterval = DefaultHealthInterval
	}
	// connectionGracePeriod==0 means "use the default"; a negative value is an
	// explicit "disable grace" and is preserved.
	if c.connectionGracePeriod == 0 {
		c.connectionGracePeriod = DefaultConnectionGracePeriod
	}
	if c.probeTimeout <= 0 {
		c.probeTimeout = DefaultProbeTimeout
	}
	if c.eventsBatchPeriod <= 0 {
		c.eventsBatchPeriod = DefaultEventsBatchPeriod
	}
	if c.reconnect.establishInitial <= 0 {
		c.reconnect.establishInitial = DefaultEstablishInitialTimeout
	}
	if c.reconnect.establishMax <= 0 {
		c.reconnect.establishMax = DefaultEstablishMaxTimeout
	}
	if c.reconnect.establishFactor <= 0 {
		c.reconnect.establishFactor = defaultEstablishFactor
	}
	if c.reconnect.retryMax <= 0 {
		c.reconnect.retryMax = DefaultReconnectRetryMax
	}
	return c
}
