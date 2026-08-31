package types

// Verifies the Expectations TTL failsafe against the injected clock: an
// unobserved expectation blocks Satisfied until expectationsTTL elapses,
// then expires (treated satisfied) without any watch event.

import (
	"testing"
	"time"

	clocktesting "k8s.io/utils/clock/testing"
)

func TestExpectations_TTLBoundary(t *testing.T) {
	t0 := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	fc := clocktesting.NewFakeClock(t0)
	e := NewExpectationsWithClock(fc)

	e.ExpectCreates("ns", "isvc", ComponentEngine, 0, 1)
	if e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("outstanding create must block Satisfied")
	}

	// Just inside the TTL: still blocked.
	fc.SetTime(t0.Add(expectationsTTL - time.Second))
	if e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("1s before TTL expiry must still block Satisfied")
	}

	// Past the TTL: the entry expires and Satisfied reports true.
	fc.SetTime(t0.Add(expectationsTTL + time.Second))
	if !e.Satisfied("ns", "isvc", ComponentEngine, 0) {
		t.Fatal("expired expectation must be treated satisfied (TTL failsafe)")
	}
}
