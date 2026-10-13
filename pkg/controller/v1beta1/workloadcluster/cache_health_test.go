package workloadcluster

import (
	"context"
	"errors"
	"sync"
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
