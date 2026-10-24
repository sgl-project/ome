package workloadcluster

import (
	"context"
	"errors"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	toolscache "k8s.io/client-go/tools/cache"
)

// ErrMemberCacheUnavailable distinguishes an unreadable informer from a missing
// object. Callers must retain uncertainty rather than treating it as absence.
var ErrMemberCacheUnavailable = errors.New("member informer cache is unavailable")

// cacheStream tracks known transport failures, not the age of an unchanged
// object. Kubernetes watches may be quiet while remaining healthy.
type cacheStream struct {
	mu         sync.RWMutex
	generation uint64
	available  bool
}

// begin starts a LIST, whose result replaces the informer's store, so reads
// fail until a watch opens.
func (s *cacheStream) begin() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generation++
	s.available = false
	return s.generation
}

// resume starts a watch from the informer's last resource version. The store
// is unchanged, so reads keep their current state until the watch opens or fails.
func (s *cacheStream) resume() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.generation++
	return s.generation
}

// expire fails reads after the given time unless a newer LIST or watch has
// started by then. A non-positive time fails them at once.
func (s *cacheStream) expire(generation uint64, after time.Duration) {
	if after <= 0 {
		s.set(generation, false)
		return
	}
	time.AfterFunc(after, func() { s.set(generation, false) })
}

func (s *cacheStream) set(generation uint64, available bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation == s.generation {
		s.available = available
	}
}

func (s *cacheStream) ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.available
}

type trackedListWatch struct {
	toolscache.ListerWatcher
	stream                *cacheStream
	listTimeout           time.Duration
	watchEstablishTimeout time.Duration
}

func (l *trackedListWatch) List(options metav1.ListOptions) (runtime.Object, error) {
	return l.ListWithContext(context.Background(), options)
}

func (l *trackedListWatch) ListWithContext(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
	l.stream.begin()
	if l.listTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, l.listTimeout)
		defer cancel()
	}
	return toolscache.ToListerWatcherWithContext(l.ListerWatcher).ListWithContext(ctx, options)
}

func (l *trackedListWatch) Watch(options metav1.ListOptions) (watch.Interface, error) {
	return l.WatchWithContext(context.Background(), options)
}

func (l *trackedListWatch) WatchWithContext(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
	generation := l.stream.resume()
	watchCtx, cancel := context.WithCancel(ctx)
	var timer *time.Timer
	if l.watchEstablishTimeout > 0 {
		timer = time.AfterFunc(l.watchEstablishTimeout, cancel)
	}
	w, err := toolscache.ToListerWatcherWithContext(l.ListerWatcher).WatchWithContext(watchCtx, options)
	if timer != nil {
		timer.Stop()
	}
	if err == nil {
		err = watchCtx.Err()
	}
	if err != nil {
		l.stream.set(generation, false)
		cancel()
		if w != nil {
			w.Stop()
		}
		return nil, err
	}
	l.stream.set(generation, true)
	tracked := &trackedWatch{Interface: &cancelOnStopWatcher{Interface: w, cancel: cancel}, results: make(chan watch.Event), done: make(chan struct{}), stream: l.stream, generation: generation, reopenTimeout: l.watchEstablishTimeout}
	go tracked.run(watchCtx)
	return tracked, nil
}

func (l *trackedListWatch) IsWatchListSemanticsUnSupported() bool {
	if capability, ok := l.ListerWatcher.(interface{ IsWatchListSemanticsUnSupported() bool }); ok {
		return capability.IsWatchListSemanticsUnSupported()
	}
	return false
}

type trackedWatch struct {
	watch.Interface
	results    chan watch.Event
	done       chan struct{}
	once       sync.Once
	stream     *cacheStream
	generation uint64
	// reopenTimeout bounds how long reads keep their state after the server
	// closes this watch, while the informer opens the next one.
	reopenTimeout time.Duration
}

func (w *trackedWatch) ResultChan() <-chan watch.Event { return w.results }

func (w *trackedWatch) Stop() { w.stop(false) }

// stop ends the watch once. The server closes every watch at the timeout
// client-go asks for and the informer reopens it, so a server close keeps
// reads for up to reopenTimeout. Any other stop fails them at once.
func (w *trackedWatch) stop(closedByServer bool) {
	w.once.Do(func() {
		if closedByServer {
			w.stream.expire(w.generation, w.reopenTimeout)
		} else {
			w.stream.set(w.generation, false)
		}
		close(w.done)
		w.Interface.Stop()
	})
}

func (w *trackedWatch) run(ctx context.Context) {
	defer close(w.results)
	defer w.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.done:
			return
		case event, ok := <-w.Interface.ResultChan():
			if !ok {
				w.stop(true)
				return
			}
			if event.Type == watch.Error {
				w.stream.set(w.generation, false)
			}
			select {
			case w.results <- event:
			case <-ctx.Done():
				return
			case <-w.done:
				return
			}
		}
	}
}
