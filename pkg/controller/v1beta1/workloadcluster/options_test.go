package workloadcluster

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// errProbe is a sentinel returned by failing test probes.
var errProbe = errors.New("dial tcp: timeout")

// validKubeconfigRealCA is like validKubeconfig but carries a real (throwaway)
// CA PEM in certificate-authority-data, so client.NewWithWatch can actually load
// the root certificates — needed by buildRemoteClient tests that construct a
// live client (the dummy "ZHVtbXk=" CA in validKubeconfig fails PEM parsing).
const validKubeconfigRealCA = `apiVersion: v1
kind: Config
clusters:
- name: c
  cluster:
    server: https://example.com
    certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJWRENCKzZBREFnRUNBZ0VCTUFvR0NDcUdTTTQ5QkFNQ01CSXhFREFPQmdOVkJBTVRCM1JsYzNRdFkyRXcKSGhjTk1qWXdOakkyTVRrek5UTXpXaGNOTWpZd05qSTNNakF6TlRNeldqQVNNUkF3RGdZRFZRUURFd2QwWlhOMApMV05oTUZrd0V3WUhLb1pJemowQ0FRWUlLb1pJemowREFRY0RRZ0FFa2hDVzNvckdaV2xzOWlwUUVUWjFPZFJnClA4T0xEQlR3Z3U3ckJTcVc1M0JxMkx1TU1VNEV0c1pXVVB1YzVkOStncHk1M1NXUTQxbm8vRnc4RmNjRXJxTkMKTUVBd0RnWURWUjBQQVFIL0JBUURBZ0lFTUE4R0ExVWRFd0VCL3dRRk1BTUJBZjh3SFFZRFZSME9CQllFRktBVwo3TWpCUVB3Q0JMTnBSb2loa2lUaW1HSnRNQW9HQ0NxR1NNNDlCQU1DQTBnQU1FVUNJRUZ1Nm5vYUNEZDBYZDdNCmRkN3JlOVlBenVMblpReS92L1VjSEJwVWo4eDVBaUVBN2dackFMQjRrYmhlUnNyQlk0UHVjV1ZzZHhJQk0vS3kKQ0s4RXhUZzFwRlk9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
contexts:
- name: ctx
  context: {cluster: c, user: u}
current-context: ctx
users:
- name: u
  user:
    token: abc123
`

// --- ClientTuning -----------------------------------------------------------

// TestClientTuning_Apply: non-zero fields land on the rest.Config; zero fields
// are left untouched so client-go's own default applies (no magic literal).
func TestClientTuning_Apply(t *testing.T) {
	cfg := &rest.Config{}
	ClientTuning{QPS: 50, Burst: 100, PerCallTimeout: 7 * time.Second}.apply(cfg)
	assert.EqualValues(t, 50, cfg.QPS)
	assert.EqualValues(t, 100, cfg.Burst)
	assert.Equal(t, 7*time.Second, cfg.Timeout)

	// Zero tuning leaves the config's fields untouched.
	cfg2 := &rest.Config{QPS: 3, Burst: 6, Timeout: time.Second}
	ClientTuning{}.apply(cfg2)
	assert.EqualValues(t, 3, cfg2.QPS, "zero QPS must not overwrite an existing value")
	assert.EqualValues(t, 6, cfg2.Burst, "zero Burst must not overwrite an existing value")
	assert.Equal(t, time.Second, cfg2.Timeout, "zero timeout must not overwrite an existing value")
}

// TestManager_SetClientTuning_RoundTrips confirms the setter stores the tuning
// that buildRemoteClient later reads.
func TestManager_SetClientTuning_RoundTrips(t *testing.T) {
	m := NewManager(scheme(t))
	m.SetClientTuning(ClientTuning{QPS: 50, Burst: 100, PerCallTimeout: 7 * time.Second})
	m.mu.RLock()
	got := m.tuning
	m.mu.RUnlock()
	assert.EqualValues(t, 50, got.QPS)
	assert.EqualValues(t, 100, got.Burst)
	assert.Equal(t, 7*time.Second, got.PerCallTimeout)
}

// --- CacheOptions -----------------------------------------------------------

func isvcGK() schema.GroupKind {
	return schema.GroupKind{Group: "ome.io", Kind: "InferenceService"}
}

func TestCacheOptions_EnabledOnlyWhenKindsPresent(t *testing.T) {
	assert.False(t, CacheOptions{}.enabled(), "empty options must not enable caching")
	assert.False(t, CacheOptions{CachedKinds: sets.New[schema.GroupKind]()}.enabled(),
		"empty kind set must not enable caching")
	assert.True(t, CacheOptions{CachedKinds: sets.New(isvcGK())}.enabled(),
		"a non-empty kind set enables caching")
}

// TestBuildRemoteClient_NeverCachingWhenNoKinds: with no CacheOptions, the
// default builder returns a never-caching client (no cache handler support) and
// a no-op cancel.
func TestBuildRemoteClient_NeverCachingWhenNoKinds(t *testing.T) {
	m := NewManager(scheme(t))
	cl, cancel, err := m.buildRemoteClient(context.Background(), []byte(validKubeconfigRealCA), m.scheme)
	require.NoError(t, err)
	require.NotNil(t, cancel)
	cancel() // must be safe to call
	// A never-caching client rejects cache event handlers.
	_, err = cl.AddCacheEventHandler(context.Background(), &corev1.Pod{}, nil)
	assert.Error(t, err, "never-caching client must reject cache handlers")
}

// TestBuildRemoteClient_SelectivelyCachingWhenKinds: with CacheOptions naming a
// cached kind, the builder constructs a selectively-caching client and returns a
// real (non-nil) cancel that tears the background cache down. cache.New does not
// dial the apiserver, so this is hermetic; cancel() stops the (idle) informers.
func TestBuildRemoteClient_SelectivelyCachingWhenKinds(t *testing.T) {
	m := NewManager(scheme(t))
	sel := labels.SelectorFromSet(labels.Set{"ome.io/placement-origin": "x"})
	m.SetCacheOptions(CacheOptions{CachedKinds: sets.New(isvcGK()), DefaultSelector: sel})

	ctx, baseCancel := context.WithCancel(context.Background())
	defer baseCancel()
	cl, cancel, err := m.buildRemoteClient(ctx, []byte(validKubeconfigRealCA), m.scheme)
	require.NoError(t, err)
	require.NotNil(t, cl)
	require.NotNil(t, cancel)
	cancel() // tears the cache down; must not panic

	// A selectively-caching client accepts a handler for a cached kind (it errors
	// only for non-cached kinds); proving it is NOT the never-caching wrapper.
	_, err = cl.AddCacheEventHandler(context.Background(), &corev1.Pod{}, nil)
	assert.Error(t, err, "Pod is not a cached kind, so a handler for it must error")
}

// TestBuildRemoteClient_CacheRejectsPods: configuring Pod as a cached kind is
// refused by the underlying NewSelectivelyCachingClient guard.
func TestBuildRemoteClient_CacheRejectsPods(t *testing.T) {
	m := NewManager(scheme(t))
	m.SetCacheOptions(CacheOptions{CachedKinds: sets.New(corev1.SchemeGroupVersion.WithKind("Pod").GroupKind())})
	_, _, err := m.buildRemoteClient(context.Background(), []byte(validKubeconfigRealCA), m.scheme)
	require.Error(t, err, "the builder must surface the cache's Pod-ban")
}

// TestManager_SetCacheOptions_RoundTrips confirms the setter stores the cache
// options buildRemoteClient later reads (selector + kinds).
func TestManager_SetCacheOptions_RoundTrips(t *testing.T) {
	m := NewManager(scheme(t))
	sel := labels.SelectorFromSet(labels.Set{"ome.io/placement-origin": "x"})
	m.SetCacheOptions(CacheOptions{CachedKinds: sets.New(isvcGK()), DefaultSelector: sel})
	m.mu.RLock()
	got := m.cacheOpts
	m.mu.RUnlock()
	assert.True(t, got.enabled())
	assert.True(t, got.CachedKinds.Has(isvcGK()))
	require.NotNil(t, got.DefaultSelector)
}

// --- reconnect backoff ------------------------------------------------------

func TestRetryAfter_Schedule(t *testing.T) {
	b := defaultReconnectBackoff()
	// 0 on the first (zero) failure, then 5s doubling to the 5m20s cap.
	assert.Equal(t, time.Duration(0), b.retryAfter(0))
	assert.Equal(t, 5*time.Second, b.retryAfter(1))
	assert.Equal(t, 10*time.Second, b.retryAfter(2))
	assert.Equal(t, 20*time.Second, b.retryAfter(3))
	assert.Equal(t, 40*time.Second, b.retryAfter(4))
	assert.Equal(t, 80*time.Second, b.retryAfter(5))
	assert.Equal(t, 160*time.Second, b.retryAfter(6))
	assert.Equal(t, 320*time.Second, b.retryAfter(7))
	// Saturates at the cap and never overflows for large attempt counts.
	assert.Equal(t, DefaultReconnectRetryMax, b.retryAfter(8))
	assert.Equal(t, DefaultReconnectRetryMax, b.retryAfter(100))
}

func TestEstablishWaitTime_Schedule(t *testing.T) {
	b := defaultReconnectBackoff()
	// attempt<=1 -> initial; doubling up to the 10m cap.
	assert.Equal(t, 1*time.Minute, b.establishWaitTime(0))
	assert.Equal(t, 1*time.Minute, b.establishWaitTime(1))
	assert.Equal(t, 2*time.Minute, b.establishWaitTime(2))
	assert.Equal(t, 4*time.Minute, b.establishWaitTime(3))
	assert.Equal(t, 8*time.Minute, b.establishWaitTime(4))
	assert.Equal(t, DefaultEstablishMaxTimeout, b.establishWaitTime(5))
	assert.Equal(t, DefaultEstablishMaxTimeout, b.establishWaitTime(50))
}

// --- establishWatch ---------------------------------------------------------

// blockingWatchClient embeds a fake WithWatch but its Watch blocks until the
// passed context is cancelled, so establishWatch's timeout path fires
// deterministically.
type blockingWatchClient struct {
	client.WithWatch
}

func (b *blockingWatchClient) Watch(ctx context.Context, _ client.ObjectList, _ ...client.ListOption) (watch.Interface, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// immediateWatchClient returns an empty watch right away (success path).
type immediateWatchClient struct {
	client.WithWatch
}

func (i *immediateWatchClient) Watch(_ context.Context, _ client.ObjectList, _ ...client.ListOption) (watch.Interface, error) {
	return watch.NewEmptyWatch(), nil
}

// stubbornWatchClient simulates a broken client that ignores context
// cancellation. establishWatch must still return at its timeout.
type stubbornWatchClient struct {
	client.WithWatch
	release <-chan struct{}
}

func (s *stubbornWatchClient) Watch(context.Context, client.ObjectList, ...client.ListOption) (watch.Interface, error) {
	<-s.release
	return nil, errors.New("released")
}

func TestEstablishWatch_TimesOut(t *testing.T) {
	c := &blockingWatchClient{WithWatch: fake.NewClientBuilder().Build()}
	_, err := establishWatch(context.Background(), c, &corev1.PodList{}, 20*time.Millisecond)
	require.ErrorIs(t, err, errWatchEstablishTimeout)
}

func TestEstablishWatch_Succeeds(t *testing.T) {
	c := &immediateWatchClient{WithWatch: fake.NewClientBuilder().Build()}
	w, err := establishWatch(context.Background(), c, &corev1.PodList{}, time.Second)
	require.NoError(t, err)
	require.NotNil(t, w)
	w.Stop() // cancelOnStopWatcher.Stop must not panic
}

func TestEstablishWatch_TimeoutDoesNotWaitForClientCancellation(t *testing.T) {
	release := make(chan struct{})
	c := &stubbornWatchClient{WithWatch: fake.NewClientBuilder().Build(), release: release}
	started := time.Now()
	_, err := establishWatch(context.Background(), c, &corev1.PodList{}, 20*time.Millisecond)
	require.ErrorIs(t, err, errWatchEstablishTimeout)
	assert.Less(t, time.Since(started), 500*time.Millisecond)
	close(release)
}

// TestManager_EstablishWatch_NotConnected: a cluster with no client returns
// (nil,false,nil) rather than erroring.
func TestManager_EstablishWatch_NotConnected(t *testing.T) {
	m := NewManager(scheme(t))
	w, generation, connected, err := m.EstablishWatch(context.Background(), "nope", &corev1.PodList{}, 1)
	require.NoError(t, err)
	assert.False(t, connected)
	assert.Nil(t, w)
	assert.Zero(t, generation)
}

// TestManager_EstablishWatch_TimesOut wires a connected blocking client and a
// tiny establish timeout so the establish half of the backoff fires.
func TestManager_EstablishWatch_TimesOut(t *testing.T) {
	m := NewManager(scheme(t))
	m.SetReconnectBackoff(reconnectBackoff{
		establishInitial: 20 * time.Millisecond,
		establishMax:     20 * time.Millisecond,
		establishFactor:  2,
		retryMax:         DefaultReconnectRetryMax,
	})
	m.newClient = func(_ context.Context, _ []byte, _ *runtime.Scheme) (SelectivelyCachingClient, context.CancelFunc, error) {
		return &blockingCachingClient{blockingWatchClient: &blockingWatchClient{WithWatch: fake.NewClientBuilder().Build()}}, func() {}, nil
	}
	require.NoError(t, m.Connect(context.Background(), "c1", []byte("kc")))

	_, generation, connected, err := m.EstablishWatch(context.Background(), "c1", &corev1.PodList{}, 1)
	assert.True(t, connected)
	require.ErrorIs(t, err, errWatchEstablishTimeout)
	assert.NotZero(t, generation)
}

// blockingCachingClient adapts blockingWatchClient to SelectivelyCachingClient
// (its Watch blocks; AddCacheEventHandler is unused by these tests).
type blockingCachingClient struct {
	*blockingWatchClient
}

func (b *blockingCachingClient) AddCacheEventHandler(context.Context, client.Object, toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	return nil, nil
}

// --- functional options -----------------------------------------------------

// TestResolveConfig_Defaults: with no options and no struct fields set, every
// knob falls back to its graceful-degradation default.
func TestResolveConfig_Defaults(t *testing.T) {
	r := &Reconciler{}
	c := r.resolveConfig()
	assert.Equal(t, DefaultHealthInterval, c.healthInterval)
	assert.Equal(t, DefaultConnectionGracePeriod, c.connectionGracePeriod)
	assert.Equal(t, DefaultEventsBatchPeriod, c.eventsBatchPeriod)
	assert.Equal(t, DefaultEstablishInitialTimeout, c.reconnect.establishInitial)
	assert.Equal(t, DefaultEstablishMaxTimeout, c.reconnect.establishMax)
	assert.Equal(t, DefaultReconnectRetryMax, c.reconnect.retryMax)
}

// TestResolveConfig_SeedsAndOptions: health interval and grace are seeded from
// the Reconciler struct fields; the events-batch and reconnect knobs come from
// options and override the defaults.
func TestResolveConfig_SeedsAndOptions(t *testing.T) {
	r := &Reconciler{HealthInterval: 2 * time.Millisecond, ConnectionGracePeriod: 3 * time.Millisecond}
	c := r.resolveConfig(
		WithEventsBatchPeriod(4*time.Millisecond),
		WithReconnectBackoff(ReconnectBackoffConfig{
			EstablishInitial: 5 * time.Millisecond,
			EstablishMax:     6 * time.Millisecond,
			EstablishFactor:  2,
			RetryInitial:     7 * time.Millisecond,
			RetryMax:         8 * time.Millisecond,
		}),
	)
	assert.Equal(t, 2*time.Millisecond, c.healthInterval)
	assert.Equal(t, 3*time.Millisecond, c.connectionGracePeriod)
	assert.Equal(t, 4*time.Millisecond, c.eventsBatchPeriod)
	assert.Equal(t, 5*time.Millisecond, c.reconnect.establishInitial)
	assert.Equal(t, 6*time.Millisecond, c.reconnect.establishMax)
	assert.Equal(t, 7*time.Millisecond, c.reconnect.retryInitial)
	assert.Equal(t, 8*time.Millisecond, c.reconnect.retryMax)
}

// TestResolveConfig_NegativeGraceDisables: a negative ConnectionGracePeriod is a
// deliberate "disable grace" and must be preserved (not replaced by the default).
func TestResolveConfig_NegativeGraceDisables(t *testing.T) {
	r := &Reconciler{ConnectionGracePeriod: -1}
	c := r.resolveConfig()
	assert.Equal(t, time.Duration(-1), c.connectionGracePeriod)
}

// TestReconcile_RetryBackoffSpacesRequeue: consecutive connection failures (with
// no live connection to hold) requeue with the growing retry backoff rather than
// the steady health interval.
func TestReconcile_RetryBackoffSpacesRequeue(t *testing.T) {
	s := scheme(t)
	wc := wcWithSecret("c1")
	secret := kcSecret(validKubeconfig)
	r, _ := newReconciler(s, func(context.Context, []byte) error { return errProbe }, wc, secret)
	// Resolve a tiny, deterministic retry backoff so the schedule is checkable.
	cfg := r.resolveConfig(WithReconnectBackoff(ReconnectBackoffConfig{
		RetryInitial: time.Second,
		RetryMax:     time.Hour,
	}))
	r.cfg = &cfg

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "c1"}}
	res1, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, time.Second, res1.RequeueAfter, "first failure -> retryInitial")

	res2, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 2*time.Second, res2.RequeueAfter, "second failure -> doubled")

	res3, err := r.Reconcile(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 4*time.Second, res3.RequeueAfter, "third failure -> doubled again")
}

// TestProbeTimeout_ConfiguredValueWins: the probe budget is config-driven — the
// configured per-call value reaches the reachability probe, and only an unset
// knob falls back to the in-package default.
func TestProbeTimeout_ConfiguredValueWins(t *testing.T) {
	r := &Reconciler{ProbeTimeout: 250 * time.Millisecond}
	assert.Equal(t, 250*time.Millisecond, r.probeTimeout(), "struct field must apply before Setup")

	c := r.resolveConfig()
	assert.Equal(t, 250*time.Millisecond, c.probeTimeout, "struct field must seed the resolved config")
	r.cfg = &c
	assert.Equal(t, 250*time.Millisecond, r.probeTimeout(), "resolved config must win after Setup")
}

// TestResolveConfig_NegativeGraceDisables: a negative ConnectionGracePeriod is a
// deliberate "disable grace" and must be preserved (not replaced by the default).
