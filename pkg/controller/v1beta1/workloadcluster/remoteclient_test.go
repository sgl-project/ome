package workloadcluster

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

func TestDirectClientBypassesInformerReads(t *testing.T) {
	for _, tt := range []struct {
		name string
		wrap func(client.WithWatch, client.Reader) SelectivelyCachingClient
		want bool
	}{
		{name: "uncached transport", wrap: func(c client.WithWatch, _ client.Reader) SelectivelyCachingClient { return NewNeverCachingClient(c) }, want: true},
		{name: "selectively cached transport", wrap: func(c client.WithWatch, cached client.Reader) SelectivelyCachingClient {
			return &selectivelyCachingClient{WithWatch: c, cachedReader: cached}
		}, want: true},
		{name: "nil transport", wrap: func(client.WithWatch, client.Reader) SelectivelyCachingClient { return NewNeverCachingClient(nil) }},
		{name: "unsupported wrapper", wrap: func(c client.WithWatch, _ client.Reader) SelectivelyCachingClient {
			return struct{ SelectivelyCachingClient }{NewNeverCachingClient(c)}
		}},
		{name: "nil client", wrap: func(client.WithWatch, client.Reader) SelectivelyCachingClient { return nil }},
		{name: "nil direct wrapper", wrap: func(client.WithWatch, client.Reader) SelectivelyCachingClient { return (*neverCachingClient)(nil) }},
		{name: "nil cached wrapper", wrap: func(client.WithWatch, client.Reader) SelectivelyCachingClient {
			return (*selectivelyCachingClient)(nil)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			direct, directReads := recordingClient(t, fakeclient.New(t))
			cached := &recordingReader{}
			got, ok := DirectClient(tt.wrap(direct, cached))
			if diff := cmp.Diff(tt.want, ok); diff != "" {
				t.Fatalf("available (-want +got):\n%s", diff)
			}
			if !ok {
				if got != nil {
					t.Fatal("unverified transport returned a client")
				}
				return
			}
			if err := got.Get(t.Context(), client.ObjectKey{Name: "service-a"}, &v1beta1.InferenceService{}); err != nil {
				t.Fatal(err)
			}
			if err := got.List(t.Context(), &corev1.PodList{}); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff([4]int{1, 1, 0, 0}, [4]int{directReads.gets.Count(), directReads.lists.Count(), cached.getCalls, cached.listCalls}); diff != "" {
				t.Errorf("direct and cached read counts (-want +got):\n%s", diff)
			}
		})
	}
}

// readCalls counts the Gets and Lists one half of a split client served.
type readCalls struct {
	gets, lists *fakeclient.Calls
}

// recordingClient layers read counters over base and answers every Get and
// List with success and no object: these tests only ask which half served a
// read. Watch and the writes still reach base.
func recordingClient(t *testing.T, base client.WithWatch) (client.WithWatch, readCalls) {
	t.Helper()
	calls := readCalls{gets: fakeclient.CountingVerb(fakeclient.Get), lists: fakeclient.CountingVerb(fakeclient.List)}
	return fakeclient.Wrap(t, base, calls.gets, calls.lists, fakeclient.WithInterceptors(interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return nil
		},
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return nil
		},
	})), calls
}

type recordingReader struct {
	getCalls  int
	listCalls int
}

func (r *recordingReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	r.getCalls++
	return nil
}

func (r *recordingReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	r.listCalls++
	return nil
}

// blockingReader parks inside the cached read until its context ends, standing
// in for controller-runtime's WaitForCacheSync on an informer that can never
// sync. It signals entry so a test can tear the cache down at exactly the
// moment a read is in flight.
type blockingReader struct {
	entered chan struct{}
	once    sync.Once
}

func (r *blockingReader) enter() {
	r.once.Do(func() { close(r.entered) })
}

func (r *blockingReader) Get(ctx context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	r.enter()
	<-ctx.Done()
	return ctx.Err()
}

func (r *blockingReader) List(ctx context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	r.enter()
	<-ctx.Done()
	return ctx.Err()
}

func TestNeverCachingClient_NoCacheHandler(t *testing.T) {
	fc := fakeclient.New(t)
	c := NewNeverCachingClient(fc)
	if _, err := c.AddCacheEventHandler(context.Background(), &corev1.Pod{}, nil); err == nil {
		t.Errorf("AddCacheEventHandler on NeverCachingClient: got nil error, want error")
	}
}

// NewSelectivelyCachingClient must refuse to cache Pods before any cluster
// contact: a cluster-wide Pod informer is exactly the OOM the remote cache
// guards against. The guard returns before cache.New, so a nil restConfig
// proves it short-circuits.
func TestNewSelectivelyCachingClient_RejectsPodKind(t *testing.T) {
	podGK := schema.GroupKind{Group: "", Kind: "Pod"}
	_, err := NewSelectivelyCachingClient(
		context.Background(),
		(*rest.Config)(nil),
		fakeclient.New(t),
		nil,
		sets.New(podGK),
		nil,
		nil,
		time.Second,
	)
	if err == nil {
		t.Fatalf("NewSelectivelyCachingClient with Pod in cachedKinds: got nil error, want rejection")
	}
}

func TestSelectivelyCachingClient_MetadataOnlyReadsBypassCache(t *testing.T) {
	direct, directReads := recordingClient(t, fakeclient.New(t))
	cached := &recordingReader{}
	metadataGK := schema.GroupKind{Group: "ome.io", Kind: "InferenceService"}
	var synced atomic.Bool
	synced.Store(true)
	c := &selectivelyCachingClient{
		WithWatch:    direct,
		cachedReader: cached,
		cachedKinds:  sets.New(metadataGK),
		synced:       &synced,
	}

	metadata := &metav1.PartialObjectMetadata{}
	metadata.SetGroupVersionKind(metadataGK.WithVersion("v1beta1"))
	if err := c.Get(context.Background(), client.ObjectKey{Name: "example"}, metadata); err != nil {
		t.Fatalf("Get PartialObjectMetadata: %v", err)
	}

	metadataList := &metav1.PartialObjectMetadataList{}
	metadataList.SetGroupVersionKind(metadataGK.WithVersion("v1beta1").GroupVersion().WithKind("InferenceServiceList"))
	if err := c.List(context.Background(), metadataList); err != nil {
		t.Fatalf("List PartialObjectMetadataList: %v", err)
	}

	if directReads.gets.Count() != 1 || directReads.lists.Count() != 1 {
		t.Fatalf("direct metadata reads: got Get=%d List=%d, want Get=1 List=1", directReads.gets.Count(), directReads.lists.Count())
	}
	if cached.getCalls != 0 || cached.listCalls != 0 {
		t.Fatalf("cached metadata reads: got Get=%d List=%d, want Get=0 List=0", cached.getCalls, cached.listCalls)
	}
}

// newCachedTestClient builds a selectivelyCachingClient over recording halves,
// caching isvcGK() — the kind the remote cache holds in production
// (cmd/manager) and in the integration suites alike — and reporting the cache
// as already synced.
func newCachedTestClient(t *testing.T, cacheCtx context.Context) (*selectivelyCachingClient, readCalls, *recordingReader) {
	t.Helper()
	direct, directReads := recordingClient(t, fakeclient.New(t))
	cached := &recordingReader{}
	var synced atomic.Bool
	synced.Store(true)
	return &selectivelyCachingClient{
		WithWatch:    direct,
		cachedReader: cached,
		cacheCtx:     cacheCtx,
		scheme:       fakeclient.Scheme(t),
		cachedKinds:  sets.New(isvcGK()),
		synced:       &synced,
	}, directReads, cached
}

// A stopped cache must not return stale state or start a network read.
func TestSelectivelyCachingClient_StoppedCacheIsUnavailable(t *testing.T) {
	cacheCtx, stopCache := context.WithCancel(t.Context())
	c, directReads, cached := newCachedTestClient(t, cacheCtx)
	stopCache()
	if err := c.Get(t.Context(), client.ObjectKey{Name: "example"}, &v1beta1.InferenceService{}); !errors.Is(err, ErrMemberCacheUnavailable) {
		t.Fatalf("Get on stopped cache: %v", err)
	}
	if err := c.List(t.Context(), &v1beta1.InferenceServiceList{}); !errors.Is(err, ErrMemberCacheUnavailable) {
		t.Fatalf("List on stopped cache: %v", err)
	}
	if directReads.gets.Count()+directReads.lists.Count()+cached.getCalls+cached.listCalls != 0 {
		t.Fatal("unavailable cache must not read from either client")
	}
}

// While the cache is live the cached reader still serves the read — the
// teardown guard must not quietly turn every cached read into a direct one.
func TestSelectivelyCachingClient_LiveCacheReadsUseCache(t *testing.T) {
	c, directReads, cached := newCachedTestClient(t, context.Background())

	if err := c.Get(context.Background(), client.ObjectKey{Name: "example"}, &v1beta1.InferenceService{}); err != nil {
		t.Fatalf("Get on live cache: %v", err)
	}
	if err := c.List(context.Background(), &v1beta1.InferenceServiceList{}); err != nil {
		t.Fatalf("List on live cache: %v", err)
	}

	if cached.getCalls != 1 || cached.listCalls != 1 {
		t.Fatalf("cached reads on live cache: got Get=%d List=%d, want Get=1 List=1", cached.getCalls, cached.listCalls)
	}
	if directReads.gets.Count() != 0 || directReads.lists.Count() != 0 {
		t.Fatalf("direct reads on live cache: got Get=%d List=%d, want Get=0 List=0", directReads.gets.Count(), directReads.lists.Count())
	}
}

// Teardown releases in-flight reads without falling back to network requests.
func TestSelectivelyCachingClient_CacheTeardownReleasesInFlightRead(t *testing.T) {
	cacheCtx, stopCache := context.WithCancel(context.Background())
	direct, directReads := recordingClient(t, fakeclient.New(t))
	blocking := &blockingReader{entered: make(chan struct{})}
	var synced atomic.Bool
	synced.Store(true)
	c := &selectivelyCachingClient{
		WithWatch:    direct,
		cachedReader: blocking,
		cacheCtx:     cacheCtx,
		scheme:       fakeclient.Scheme(t),
		cachedKinds:  sets.New(isvcGK()),
		synced:       &synced,
	}

	done := make(chan error, 1)
	go func() {
		done <- c.Get(context.Background(), client.ObjectKey{Name: "example"}, &v1beta1.InferenceService{})
	}()

	<-blocking.entered
	stopCache() // Disconnect lands while the read is parked inside the cache.

	select {
	case err := <-done:
		if !errors.Is(err, ErrMemberCacheUnavailable) {
			t.Fatalf("in-flight read after cache teardown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("in-flight cached read did not return after the cache was torn down")
	}
	if directReads.gets.Count() != 0 {
		t.Fatalf("direct Get after teardown: got %d, want 0", directReads.gets.Count())
	}
}

// Registering a watch handler on a torn-down cache must report an error rather
// than block in GetInformer, which reaches the same lazily-created-informer
// path as a cached read.
func TestSelectivelyCachingClient_AddCacheEventHandlerOnStoppedCache(t *testing.T) {
	cacheCtx, stopCache := context.WithCancel(context.Background())
	c, _, _ := newCachedTestClient(t, cacheCtx)
	stopCache()

	if _, err := c.AddCacheEventHandler(context.Background(), &v1beta1.InferenceService{}, nil); err == nil {
		t.Fatal("AddCacheEventHandler on a stopped cache: got nil error, want error")
	}
}
