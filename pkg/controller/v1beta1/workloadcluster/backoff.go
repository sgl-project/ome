// The establish/retry backoff schedule below is adapted from kubernetes-sigs/
// kueue (pkg/controller/admissionchecks/multikueue), Apache-2.0.

package workloadcluster

import (
	"context"
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Default* are the graceful-degradation fallbacks for the connection-layer
// timing knobs. They are NOT the operative values: those are config-driven
// (manager flags / chart) and injected via functional Options. Absent config,
// these keep the controller working with sane behavior rather than a magic
// literal buried mid-function.
const (
	// DefaultEventsBatchPeriod debounces Secret-change re-enqueues. A rotated
	// kubeconfig Secret typically updates several keys back-to-back; folding the
	// burst into one reconcile avoids a thundering rebuild of the remote client.
	DefaultEventsBatchPeriod = 100 * time.Millisecond

	// DefaultEstablishInitialTimeout bounds the first attempt to establish a
	// remote watch before it is abandoned and retried.
	DefaultEstablishInitialTimeout = 1 * time.Minute
	// DefaultEstablishMaxTimeout caps the (growing) establish-watch timeout.
	DefaultEstablishMaxTimeout = 10 * time.Minute
	// defaultEstablishFactor is the per-attempt growth factor for the establish
	// timeout (1m, 2m, 4m, 8m, 10m, ...).
	defaultEstablishFactor = 2.0

	// DefaultReconnectRetryMax caps the inter-attempt retry delay. With a 5s
	// increment doubling from 0, the schedule is 0, 5s, 10s, 20s, 40s, 80s,
	// 160s, 320s (~5m20s) — matching the Kueue reference cap.
	DefaultReconnectRetryMax = 320 * time.Second
	// reconnectRetryIncrement is the base step the retry delay doubles from.
	reconnectRetryIncrement = 5 * time.Second
)

// errWatchEstablishTimeout is returned when establishWatch's bounded attempt to
// open a remote watch does not complete before the establish timeout, so the
// caller falls back to the retry backoff.
var errWatchEstablishTimeout = errors.New("watch establishment timed out")

// establishWaitTime returns the establish-watch timeout for the given attempt
// (1-based), growing from establishInitial by establishFactor up to
// establishMax. attempt<=1 returns establishInitial.
func (b reconnectBackoff) establishWaitTime(attempt int) time.Duration {
	if attempt <= 1 {
		return b.establishInitial
	}
	bo := wait.Backoff{
		Duration: b.establishInitial,
		Factor:   b.establishFactor,
		Cap:      b.establishMax,
		Steps:    attempt,
	}
	var d time.Duration
	for i := 0; i < attempt; i++ {
		d = bo.Step()
		if d >= b.establishMax {
			return b.establishMax
		}
	}
	return d
}

// retryAfter returns the inter-attempt delay after failedAttempts consecutive
// failures: 0 for the first, then retryInitial doubling up to retryMax. With
// retryInitial==0 the increment falls back to reconnectRetryIncrement so the
// schedule still grows (0, 5s, 10s, ... up to retryMax).
func (b reconnectBackoff) retryAfter(failedAttempts uint) time.Duration {
	if failedAttempts == 0 {
		return 0
	}
	step := b.retryInitial
	if step <= 0 {
		step = reconnectRetryIncrement
	}
	d := step << (failedAttempts - 1)
	if d > b.retryMax || d <= 0 { // d<=0 guards shift overflow
		return b.retryMax
	}
	return d
}

// cancelOnStopWatcher carries the establishment context's cancel func so it runs
// when Stop is called, satisfying govet's lostcancel check without shortening
// the watch's stream lifetime on the success path.
type cancelOnStopWatcher struct {
	watch.Interface
	cancel context.CancelFunc
}

func (cw *cancelOnStopWatcher) Stop() {
	cw.Interface.Stop()
	cw.cancel()
}

// establishWatch opens a remote watch bounded by timeout. On timeout the
// in-flight Watch is canceled and errWatchEstablishTimeout is returned so the
// caller falls back to the retry backoff.
func establishWatch(ctx context.Context, c client.WithWatch, list client.ObjectList, timeout time.Duration, opts ...client.ListOption) (watch.Interface, error) {
	type result struct {
		w   watch.Interface
		err error
	}
	resultCh := make(chan result, 1)
	establishCtx, cancel := context.WithCancel(ctx)

	go func() {
		w, err := c.Watch(establishCtx, list, opts...)
		resultCh <- result{w: w, err: err}
	}()

	select {
	case r := <-resultCh:
		if r.err != nil {
			cancel()
			return nil, r.err
		}
		return &cancelOnStopWatcher{Interface: r.w, cancel: cancel}, nil
	case <-time.After(timeout):
		cancel()
		// Do not wait for a broken client implementation to honor cancellation.
		// The buffered result channel lets the producer finish independently; a
		// late watcher is still stopped so it cannot leak.
		go func() {
			if r := <-resultCh; r.w != nil {
				r.w.Stop()
			}
		}()
		return nil, errWatchEstablishTimeout
	}
}
