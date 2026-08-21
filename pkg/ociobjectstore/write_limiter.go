package ociobjectstore

import (
	"sync/atomic"
	"time"
)

// WriteLimiter bounds concurrent model-file writes across every object and
// download task that shares the limiter. A nil limiter leaves writes
// unrestricted.
type WriteLimiter struct {
	permits chan struct{}
	active  atomic.Int64
	waiting atomic.Int64
}

type writePermitStats struct {
	waitDuration  time.Duration
	activeWrites  int64
	waitingWrites int64
}

// NewWriteLimiter creates a limiter with the requested concurrency. Values
// less than one preserve the existing unrestricted behavior.
func NewWriteLimiter(concurrency int) *WriteLimiter {
	if concurrency < 1 {
		return nil
	}
	return &WriteLimiter{permits: make(chan struct{}, concurrency)}
}

func (limiter *WriteLimiter) acquire() writePermitStats {
	if limiter == nil {
		return writePermitStats{}
	}

	waitStartedAt := time.Now()
	select {
	case limiter.permits <- struct{}{}:
		return writePermitStats{
			waitDuration: time.Since(waitStartedAt),
			activeWrites: limiter.active.Add(1),
		}
	default:
	}

	waitingWrites := limiter.waiting.Add(1)
	limiter.permits <- struct{}{}
	limiter.waiting.Add(-1)
	return writePermitStats{
		waitDuration:  time.Since(waitStartedAt),
		activeWrites:  limiter.active.Add(1),
		waitingWrites: waitingWrites,
	}
}

func (limiter *WriteLimiter) release() {
	if limiter == nil {
		return
	}
	limiter.active.Add(-1)
	<-limiter.permits
}

// Limit returns the configured concurrency, or zero when writes are
// unrestricted.
func (limiter *WriteLimiter) Limit() int {
	if limiter == nil {
		return 0
	}
	return cap(limiter.permits)
}
