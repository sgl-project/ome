package workloadcluster

import (
	"context"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// originResolver is a test resolver: it accepts objects carrying a non-empty
// "origin" label and maps them to their own namespace/name (mirroring how the
// real derived-ISVC resolver works), and rejects the rest.
func originResolver(obj client.Object) (types.NamespacedName, bool) {
	if obj == nil || obj.GetLabels()["origin"] == "" {
		return types.NamespacedName{}, false
	}
	return types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}, true
}

func testFunnel(buffer int) *StatusFunnel {
	return NewStatusFunnel(NewManager(nil), FunnelConfig{
		NewList:    func() client.ObjectList { return &corev1.ConfigMapList{} },
		NewObject:  func() client.Object { return &corev1.ConfigMap{} },
		Resolve:    originResolver,
		BufferSize: buffer,
	})
}

// TestFunnel_PushResolvesAndEmits: an object the resolver accepts produces a
// GenericEvent on the channel carrying the resolved LOCAL key (namespace/name).
func TestFunnel_PushResolvesAndEmits(t *testing.T) {
	f := testFunnel(4)
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: "team-a", Name: "llama", Labels: map[string]string{"origin": "uid-1"},
	}}
	f.push(context.Background(), obj)

	select {
	case ev := <-f.Events():
		if ev.Object == nil {
			t.Fatalf("emitted event has nil object")
		}
		if ev.Object.GetNamespace() != "team-a" || ev.Object.GetName() != "llama" {
			t.Errorf("emitted key = %s/%s, want team-a/llama", ev.Object.GetNamespace(), ev.Object.GetName())
		}
	default:
		t.Fatalf("expected an event on the channel, got none")
	}
}

// TestFunnel_PushDropsUnresolved: an object the resolver rejects (no origin
// marker) produces NO event — the funnel never re-reconciles off a foreign object.
func TestFunnel_PushDropsUnresolved(t *testing.T) {
	f := testFunnel(4)
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "user-owned"}}
	f.push(context.Background(), obj)

	select {
	case ev := <-f.Events():
		t.Fatalf("expected no event for unresolved object, got %v", ev.Object)
	default:
	}
}

// TestFunnel_PushNonObject: a non-client.Object payload (defensive) is dropped
// without panic.
func TestFunnel_PushNonObject(t *testing.T) {
	f := testFunnel(4)
	f.push(context.Background(), "not an object")
	f.push(context.Background(), nil)
	select {
	case ev := <-f.Events():
		t.Fatalf("expected no event for non-object payloads, got %v", ev.Object)
	default:
	}
}

// TestFunnel_PushDropsWhenFull: with a full buffer the push is non-blocking and
// drops the overflow rather than wedging the informer callback. The buffered
// events remain; the dropped one is simply absent.
func TestFunnel_PushDropsWhenFull(t *testing.T) {
	f := testFunnel(2)
	mk := func(name string) *corev1.ConfigMap {
		return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns", Name: name, Labels: map[string]string{"origin": "u"},
		}}
	}
	// Fill the buffer (2) then overflow with a 3rd; push must not block.
	done := make(chan struct{})
	go func() {
		f.push(context.Background(), mk("a"))
		f.push(context.Background(), mk("b"))
		f.push(context.Background(), mk("c")) // dropped (buffer full)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("push blocked on a full channel; want non-blocking drop")
	}

	// Exactly the two buffered events are present.
	if got := drainNames(f); len(got) != 2 {
		t.Errorf("buffered events = %d (%v), want 2", len(got), got)
	}
}

func drainNames(f *StatusFunnel) []string {
	var names []string
	for {
		select {
		case ev := <-f.Events():
			names = append(names, ev.Object.GetName())
		default:
			return names
		}
	}
}

// TestFunnel_WatchListOpts: when a selector is configured the establish-watch is
// scoped by it; without one no options are passed (watch unfiltered).
func TestFunnel_WatchListOpts(t *testing.T) {
	// No selector => no options.
	f := testFunnel(1)
	if opts := f.watchListOpts(); len(opts) != 0 {
		t.Errorf("watchListOpts with no selector = %d opts, want 0", len(opts))
	}

	// Selector => exactly one MatchingLabelsSelector option carrying it.
	req, err := labels.NewRequirement("origin", selection.Exists, nil)
	if err != nil {
		t.Fatalf("building requirement: %v", err)
	}
	sel := labels.NewSelector().Add(*req)
	f2 := NewStatusFunnel(NewManager(nil), FunnelConfig{
		NewList:       func() client.ObjectList { return &corev1.ConfigMapList{} },
		NewObject:     func() client.Object { return &corev1.ConfigMap{} },
		Resolve:       originResolver,
		WatchSelector: sel,
	})
	opts := f2.watchListOpts()
	if len(opts) != 1 {
		t.Fatalf("watchListOpts with selector = %d opts, want 1", len(opts))
	}
	lo := &client.ListOptions{}
	opts[0].ApplyToList(lo)
	if lo.LabelSelector == nil || !lo.LabelSelector.Matches(labels.Set{"origin": "x"}) {
		t.Errorf("watchListOpts selector did not carry the configured selector")
	}
}

// TestFunnel_DeletedObjectUnwrapsTombstone: a cache DeletedFinalStateUnknown
// tombstone resolves to the wrapped object so a delete event is handled like an
// add/update.
func TestFunnel_DeletedObjectUnwrapsTombstone(t *testing.T) {
	inner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "m"}}
	tombstone := toolscache.DeletedFinalStateUnknown{Key: "ns/m", Obj: inner}
	got := deletedObject(tombstone)
	if got != any(inner) {
		t.Errorf("deletedObject(tombstone) = %v, want the wrapped object", got)
	}
	// A plain object passes through unchanged.
	if got := deletedObject(inner); got != any(inner) {
		t.Errorf("deletedObject(plain) = %v, want the object itself", got)
	}
}

// TestFunnel_DefaultBufferSize: a zero/negative buffer falls back to the default
// depth (no panic, channel usable).
func TestFunnel_DefaultBufferSize(t *testing.T) {
	f := NewStatusFunnel(NewManager(nil), FunnelConfig{
		NewObject: func() client.Object { return &corev1.ConfigMap{} },
		Resolve:   originResolver,
	})
	if cap(f.events) != DefaultFunnelBufferSize {
		t.Errorf("default buffer cap = %d, want %d", cap(f.events), DefaultFunnelBufferSize)
	}
}

// TestFunnel_StartNoOpWhenUnconfigured: a funnel missing its resolver/kind
// constructors must start and block (degrade to the consumer's safety requeue)
// rather than panic, and must return promptly on context cancel.
func TestFunnel_StartNoOpWhenUnconfigured(t *testing.T) {
	f := NewStatusFunnel(NewManager(nil), FunnelConfig{}) // no Resolve/NewList/NewObject
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- f.Start(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Start did not return after context cancel")
	}
}

// fakeWatchClient is a SelectivelyCachingClient that returns a fake watcher from
// Watch and records the cache event handler so a test can drive informer events
// through the funnel's resolve+push path without a real apiserver.
type fakeWatchClient struct {
	SelectivelyCachingClient
	mu      sync.Mutex
	watcher *watch.FakeWatcher
	handler toolscache.ResourceEventHandler
}

func (c *fakeWatchClient) Watch(_ context.Context, _ client.ObjectList, _ ...client.ListOption) (watch.Interface, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.watcher = watch.NewFake()
	return c.watcher, nil
}

func (c *fakeWatchClient) AddCacheEventHandler(_ context.Context, _ client.Object, h toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handler = h
	return nil, nil
}

func (c *fakeWatchClient) getHandler() toolscache.ResourceEventHandler {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.handler
}

// TestFunnel_WatchClusterRegistersHandlerAndPushes drives the full per-cluster
// path: establish the watch via the Manager, register the cache handler, then a
// cache Add of an owned object resolves to its local key and lands on the
// channel. A short-circuit establish backoff keeps the test fast.
func TestFunnel_WatchClusterRegistersHandlerAndPushes(t *testing.T) {
	fc := &fakeWatchClient{}
	m := NewManager(runtime.NewScheme())
	m.newClient = func(_ context.Context, _ []byte, _ *runtime.Scheme) (SelectivelyCachingClient, context.CancelFunc, error) {
		return fc, func() {}, nil
	}
	// Retry/establish backoff with no establish wait so EstablishWatch returns
	// immediately; retry is irrelevant on the success path.
	m.SetReconnectBackoff(reconnectBackoff{
		establishInitial: time.Second, establishMax: time.Second, establishFactor: 2,
		retryInitial: time.Millisecond, retryMax: time.Millisecond,
	})
	if err := m.Connect(context.Background(), "c1", []byte("kc")); err != nil {
		t.Fatalf("connect: %v", err)
	}

	f := NewStatusFunnel(m, FunnelConfig{
		NewList:    func() client.ObjectList { return &corev1.ConfigMapList{} },
		NewObject:  func() client.Object { return &corev1.ConfigMap{} },
		Resolve:    originResolver,
		BufferSize: 4,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go f.watchCluster(ctx, "c1")

	// Wait for the handler to be registered (establishment confirmed first).
	var h toolscache.ResourceEventHandler
	deadline := time.After(2 * time.Second)
	for h == nil {
		select {
		case <-deadline:
			t.Fatalf("cache handler was never registered")
		default:
			h = fc.getHandler()
			if h == nil {
				time.Sleep(5 * time.Millisecond)
			}
		}
	}

	// Drive an informer Add of an owned object through the handler.
	h.OnAdd(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Namespace: "team-a", Name: "llama", Labels: map[string]string{"origin": "uid-1"},
	}}, false)

	select {
	case ev := <-f.Events():
		if ev.Object.GetNamespace() != "team-a" || ev.Object.GetName() != "llama" {
			t.Errorf("emitted key = %s/%s, want team-a/llama", ev.Object.GetNamespace(), ev.Object.GetName())
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("no event after informer Add of an owned object")
	}
}
