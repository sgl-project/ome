// Package exitcode maps typed CLI failures to stable process exit codes.
package exitcode

import "errors"

const (
	Success          = 0
	GeneralError     = 1
	AssertionUnmet   = 2
	MutationConflict = 3
)

// UnmetAssertionError reports a completed observation whose explicit
// assertion or wait predicate was not satisfied.
type UnmetAssertionError struct {
	Err error
}

func (e *UnmetAssertionError) Error() string {
	if e == nil || e.Err == nil {
		return "assertion unmet"
	}
	return e.Err.Error()
}

func (e *UnmetAssertionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// PreconditionError reports that a guarded mutation was rejected because its
// live target no longer satisfies the verified precondition.
type PreconditionError struct {
	Err error
}

func (e *PreconditionError) Error() string {
	if e == nil || e.Err == nil {
		return "mutation precondition failed"
	}
	return e.Err.Error()
}

func (e *PreconditionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// FromError returns the stable process exit code for err.
func FromError(err error) int {
	if err == nil {
		return Success
	}
	var assertion *UnmetAssertionError
	if errors.As(err, &assertion) {
		return AssertionUnmet
	}
	var precondition *PreconditionError
	if errors.As(err, &precondition) {
		return MutationConflict
	}
	return GeneralError
}
