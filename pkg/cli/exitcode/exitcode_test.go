package exitcode

import (
	"errors"
	"fmt"
	"testing"
)

func TestFromError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "success", want: Success},
		{name: "ordinary error", err: errors.New("boom"), want: GeneralError},
		{name: "unmet assertion", err: &UnmetAssertionError{Err: errors.New("not ready")}, want: AssertionUnmet},
		{name: "wrapped assertion", err: fmt.Errorf("wait failed: %w", &UnmetAssertionError{Err: errors.New("not ready")}), want: AssertionUnmet},
		{name: "stale mutation", err: &PreconditionError{Err: errors.New("resource changed")}, want: MutationConflict},
		{name: "wrapped stale mutation", err: fmt.Errorf("apply failed: %w", &PreconditionError{Err: errors.New("resource changed")}), want: MutationConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := FromError(test.err); got != test.want {
				t.Fatalf("FromError(%v) = %d, want %d", test.err, got, test.want)
			}
		})
	}
}

func TestTypedErrorsPreserveCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("predicate not met")
	assertion := &UnmetAssertionError{Err: cause}
	if assertion.Error() != cause.Error() || !errors.Is(assertion, cause) {
		t.Fatalf("UnmetAssertionError does not preserve cause: %v", assertion)
	}

	stale := &PreconditionError{Err: cause}
	if stale.Error() != cause.Error() || !errors.Is(stale, cause) {
		t.Fatalf("PreconditionError does not preserve cause: %v", stale)
	}
}

func TestTypedErrorsHandleNilCause(t *testing.T) {
	t.Parallel()

	if got := (&UnmetAssertionError{}).Error(); got != "assertion unmet" {
		t.Fatalf("nil UnmetAssertionError = %q", got)
	}
	if got := (&PreconditionError{}).Error(); got != "mutation precondition failed" {
		t.Fatalf("nil PreconditionError = %q", got)
	}
}
