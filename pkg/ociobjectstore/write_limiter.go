package ociobjectstore

import "time"

// WriteLimiter bounds concurrent model-file writes across every object and
// download task that shares the limiter. A nil limiter leaves writes
// unrestricted.
type WriteLimiter struct {
	permits chan struct{}
}

// NewWriteLimiter creates a limiter with the requested concurrency. Values
// less than one preserve the existing unrestricted behavior.
func NewWriteLimiter(concurrency int) *WriteLimiter {
	if concurrency < 1 {
		return nil
	}
	return &WriteLimiter{permits: make(chan struct{}, concurrency)}
}

func (limiter *WriteLimiter) acquire() time.Duration {
	if limiter == nil {
		return 0
	}
	waitStartedAt := time.Now()
	limiter.permits <- struct{}{}
	return time.Since(waitStartedAt)
}

func (limiter *WriteLimiter) release() {
	if limiter == nil {
		return
	}
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
