package workloadcluster

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestCacheStreamFailureAndRecovery(t *testing.T) {
	c, direct, cached := newCachedTestClient(t, t.Context())
	stream := &cacheStream{}
	c.streams = &sync.Map{}
	c.streams.Store(isvcGK(), stream)
	read := func() error {
		return c.Get(t.Context(), client.ObjectKey{Name: "service-a"}, &v1beta1.InferenceService{})
	}
	require.ErrorIs(t, read(), ErrMemberCacheUnavailable)

	underlying := watch.NewRaceFreeFake()
	lw := &trackedListWatch{stream: stream, ListerWatcher: &toolscache.ListWatch{
		WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) { return underlying, nil },
	}}
	w, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	t.Cleanup(w.Stop)
	require.NoError(t, read())

	underlying.Error(&metav1.Status{Reason: metav1.StatusReasonExpired})
	require.Equal(t, watch.Error, (<-w.ResultChan()).Type)
	require.ErrorIs(t, read(), ErrMemberCacheUnavailable)
	w.Stop()

	underlying = watch.NewRaceFreeFake()
	recovered, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	t.Cleanup(recovered.Stop)
	require.NoError(t, read())
	// With no establishment budget there is no time to reopen a closed watch,
	// so the close fails reads at once.
	underlying.Stop()
	_, open := <-recovered.ResultChan()
	require.False(t, open)
	require.ErrorIs(t, read(), ErrMemberCacheUnavailable)
	require.Equal(t, 2, cached.getCalls)
	require.Zero(t, direct.getCalls, "watch failure must not trigger live GETs")
}

func TestCacheStreamRejectsFailedListAndWatch(t *testing.T) {
	stream := &cacheStream{}
	lw := &trackedListWatch{stream: stream, ListerWatcher: &toolscache.ListWatch{
		ListWithContextFunc: func(context.Context, metav1.ListOptions) (runtime.Object, error) {
			return nil, errors.New("member unavailable")
		},
		WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) {
			return nil, errors.New("member unavailable")
		},
	}}
	stream.set(stream.begin(), true)
	_, err := lw.ListWithContext(t.Context(), metav1.ListOptions{})
	require.Error(t, err)
	require.False(t, stream.ready())
	_, err = lw.WatchWithContext(t.Context(), metav1.ListOptions{})
	require.Error(t, err)
	require.False(t, stream.ready())
}

func TestCacheStreamOldWatchCannotInvalidateReplacement(t *testing.T) {
	stream := &cacheStream{}
	lw := &trackedListWatch{stream: stream, ListerWatcher: &toolscache.ListWatch{
		WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) {
			return watch.NewRaceFreeFake(), nil
		},
	}}
	old, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	t.Cleanup(old.Stop)
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	current, err := lw.WatchWithContext(ctx, metav1.ListOptions{})
	require.NoError(t, err)
	t.Cleanup(current.Stop)
	old.Stop()
	<-old.ResultChan()
	require.True(t, stream.ready())
	cancel()
	_, open := <-current.ResultChan()
	require.False(t, open)
	require.False(t, stream.ready())
}

// The API server ends every watch at the timeout client-go asks for, and the
// informer opens a new one from its last resource version. Reads keep working
// while the new watch opens.
func TestCacheStreamStaysReadableWhileTheInformerReopensAWatch(t *testing.T) {
	c, _, _ := newCachedTestClient(t, t.Context())
	stream := &cacheStream{}
	c.streams = &sync.Map{}
	c.streams.Store(isvcGK(), stream)
	read := func() error {
		return c.Get(t.Context(), client.ObjectKey{Namespace: "team-a", Name: "service-a"}, &v1beta1.InferenceService{})
	}
	service := &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "service-a", ResourceVersion: "1"}}

	first := watch.NewRaceFreeFake()
	reopening, release := make(chan struct{}), make(chan struct{})
	var watches atomic.Int32
	lw := &trackedListWatch{stream: stream, watchEstablishTimeout: time.Minute, ListerWatcher: &toolscache.ListWatch{
		ListWithContextFunc: func(context.Context, metav1.ListOptions) (runtime.Object, error) {
			return &v1beta1.InferenceServiceList{ListMeta: metav1.ListMeta{ResourceVersion: "1"}, Items: []v1beta1.InferenceService{*service}}, nil
		},
		WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			switch watches.Add(1) {
			case 1:
				if options.SendInitialEvents != nil && *options.SendInitialEvents {
					first.Add(service.DeepCopy())
					first.Action(watch.Bookmark, &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{
						ResourceVersion: "1", Annotations: map[string]string{metav1.InitialEventsAnnotationKey: "true"},
					}})
				}
				// One event after the initial state, so the informer reads the
				// close as the end of a normal watch and reopens it at once.
				first.Action(watch.Bookmark, &v1beta1.InferenceService{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "2"}})
				return first, nil
			case 2:
				close(reopening)
			}
			select {
			case <-release:
				return watch.NewRaceFreeFake(), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	}}
	informer := toolscache.NewSharedIndexInformer(lw, &v1beta1.InferenceService{}, 0, toolscache.Indexers{})
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	go informer.RunWithContext(ctx)
	require.Eventually(t, func() bool { return informer.HasSynced() && read() == nil }, 5*time.Second, time.Millisecond)

	first.Stop()
	select {
	case <-reopening:
	case <-time.After(5 * time.Second):
		t.Fatal("the informer did not reopen the closed watch")
	}
	require.NoError(t, read(), "reads keep working while the informer reopens the watch")
	close(release)
	require.NoError(t, read())
}

// A closed watch that is not reopened within the establishment budget fails
// reads, so a member whose watches stop coming back is not served from an old
// snapshot without limit.
func TestCacheStreamFailsReadsWhenAClosedWatchIsNotReopenedInTime(t *testing.T) {
	stream := &cacheStream{}
	underlying := watch.NewRaceFreeFake()
	lw := &trackedListWatch{stream: stream, watchEstablishTimeout: 20 * time.Millisecond, ListerWatcher: &toolscache.ListWatch{
		WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) { return underlying, nil },
	}}
	w, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	t.Cleanup(w.Stop)
	require.True(t, stream.ready())

	underlying.Stop()
	_, open := <-w.ResultChan()
	require.False(t, open)
	require.Eventually(t, func() bool { return !stream.ready() }, 5*time.Second, time.Millisecond)
}

// A reopen that fails ends availability at once, without waiting out the
// establishment budget.
func TestCacheStreamFailsReadsWhenReopeningAWatchFails(t *testing.T) {
	stream := &cacheStream{}
	underlying := watch.NewRaceFreeFake()
	var watches atomic.Int32
	lw := &trackedListWatch{stream: stream, watchEstablishTimeout: time.Minute, ListerWatcher: &toolscache.ListWatch{
		WatchFuncWithContext: func(context.Context, metav1.ListOptions) (watch.Interface, error) {
			if watches.Add(1) == 1 {
				return underlying, nil
			}
			return nil, errors.New("member unavailable")
		},
	}}
	w, err := lw.WatchWithContext(t.Context(), metav1.ListOptions{})
	require.NoError(t, err)
	t.Cleanup(w.Stop)
	underlying.Stop()
	_, open := <-w.ResultChan()
	require.False(t, open)

	_, err = lw.WatchWithContext(t.Context(), metav1.ListOptions{})
	require.Error(t, err)
	require.False(t, stream.ready())
}

func TestCacheStreamBoundsListAndWatchEstablishment(t *testing.T) {
	lw := &trackedListWatch{
		stream: &cacheStream{}, listTimeout: 10 * time.Millisecond, watchEstablishTimeout: 10 * time.Millisecond,
		ListerWatcher: &toolscache.ListWatch{
			ListWithContextFunc: func(ctx context.Context, _ metav1.ListOptions) (runtime.Object, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
			WatchFuncWithContext: func(ctx context.Context, _ metav1.ListOptions) (watch.Interface, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		},
	}
	_, err := lw.ListWithContext(t.Context(), metav1.ListOptions{})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	_, err = lw.WatchWithContext(t.Context(), metav1.ListOptions{})
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, lw.stream.ready())
}
