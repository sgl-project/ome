// Package apierror translates raw Kubernetes API errors into messages that
// name the actual problem a kubectl-ome user has to fix.
package apierror

import (
	"fmt"
	"strings"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

// Friendly detects the "CRD not installed" flavor of NotFound (a 404 on the
// resource path itself, whose message carries no object name) and returns an
// actionable error; anything else passes through unchanged.
func Friendly(err error) error {
	if err == nil || !kerrors.IsNotFound(err) {
		return err
	}
	if strings.Contains(err.Error(), "the server could not find the requested resource") {
		return fmt.Errorf("OME does not appear to be installed on this cluster (the ome.io API is not available; install OME first — see https://github.com/ome-projects/ome#installation): %w", err)
	}
	return err
}
