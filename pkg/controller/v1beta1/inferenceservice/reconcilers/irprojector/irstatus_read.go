package irprojector

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// ComponentIRStatus returns the authoritative InferenceReplica status for one
// Component of an ISVC, read via the supplied reader (cached client is fine —
// these are non-destructive coordination reads). Returns (nil, nil) when the IR
// does not exist yet, so callers treat "no authoritative status" the same as the
// pre-migration empty projection. Any other read failure is returned as a
// wrapped error — callers MUST distinguish it from the missing-IR case: a
// transient read error is not "no observation yet", and safety gates that treat
// it that way fail open. A nil reader is likewise an error, not a missing IR —
// it signals a wiring bug the caller must not silently absorb. This is the
// single source of truth every in-cluster decision reads instead of the ISVC's
// copied LifecycleStatus.
func ComponentIRStatus(ctx context.Context, reads client.Reader, namespace, isvcName string, c v1beta1.ComponentType) (*v1beta1.InferenceReplicaStatus, error) {
	// A nil reader is a wiring bug, not an absent IR. Surface it as an
	// error so safety gates fail closed instead of mistaking it for the
	// missing-IR case, while still not panicking the reconcile.
	if reads == nil {
		return nil, fmt.Errorf("nil reader for InferenceReplica %s/%s (component %s)",
			namespace, InferenceReplicaName(isvcName, c), c)
	}
	ir := &v1beta1.InferenceReplica{}
	key := types.NamespacedName{Namespace: namespace, Name: InferenceReplicaName(isvcName, c)}
	if err := reads.Get(ctx, key, ir); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get InferenceReplica %s/%s: %w", key.Namespace, key.Name, err)
	}
	return &ir.Status, nil
}
