package common

import (
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// shouldRetry returns true if the error should be retried
func ShouldRetry(err error) bool {
	if errors.Unwrap(err) != nil {
		return true
	}

	// Alternatively, check for known error types
	var statusErr *apierrors.StatusError
	if errors.As(err, &statusErr) {
		switch statusErr.Status().Reason {
		case metav1.StatusReasonConflict,
			metav1.StatusReasonTimeout,
			metav1.StatusReasonServerTimeout,
			metav1.StatusReasonTooManyRequests:
			return true
		}
	}

	return false
}
