package types

import (
	"testing"
	"time"
)

func TestNextRetryDelay(t *testing.T) {
	p := &RetryPolicy{MaxAttempts: 3, InitialDelay: time.Minute, MaxDelay: 30 * time.Minute, Multiplier: 2.0}
	cases := []struct {
		attempts int32
		want     time.Duration
	}{
		{1, time.Minute},       // first failure → initial
		{2, 2 * time.Minute},   // initial * m^1
		{3, 4 * time.Minute},   // initial * m^2
		{10, 30 * time.Minute}, // capped at MaxDelay
	}
	for _, c := range cases {
		if got := p.NextRetryDelay(c.attempts); got != c.want {
			t.Errorf("NextRetryDelay(%d) = %v, want %v", c.attempts, got, c.want)
		}
	}
}

func TestRetryPolicyExhausted(t *testing.T) {
	p := &RetryPolicy{MaxAttempts: 3, InitialDelay: time.Minute, MaxDelay: 30 * time.Minute, Multiplier: 2.0}
	if p.Exhausted(2) {
		t.Error("2 < 3 attempts must not be exhausted")
	}
	if !p.Exhausted(3) {
		t.Error("3 >= 3 attempts must be exhausted")
	}
	var nilP *RetryPolicy
	if !nilP.Exhausted(0) {
		t.Error("nil policy (unconfigured) is always exhausted → fail-safe Held")
	}
}

func TestFindRetryBlock(t *testing.T) {
	blocks := []RetryBlock{{TargetRevision: "a"}, {TargetRevision: "b"}}
	if got := FindRetryBlock(blocks, "b"); got == nil || got.TargetRevision != "b" {
		t.Errorf("FindRetryBlock(b) = %v", got)
	}
	if got := FindRetryBlock(blocks, "c"); got != nil {
		t.Errorf("FindRetryBlock(missing) must be nil, got %v", got)
	}
	// Returned pointer aliases the slice element (callers mutate in place).
	FindRetryBlock(blocks, "a").AttemptsStarted = 7
	if blocks[0].AttemptsStarted != 7 {
		t.Error("FindRetryBlock must return a pointer into the slice")
	}
}
