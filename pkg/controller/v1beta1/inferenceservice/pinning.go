package inferenceservice

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knapis "knative.dev/pkg/apis"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimerevision"
)

// pinResult tells the caller how to proceed after the pinning helper
// runs. When skipChildren is true, the helper has already mutated +
// persisted the ISVC status; the caller returns without reconciling
// engine/decoder/router/etc.
type pinResult struct {
	spec         *v1beta1.ServingRuntimeSpec
	skipChildren bool
}

// resolvePinnedRuntime fetches the runtime spec via the
// ControllerRevision pin. Called only when spec.runtime.autoSync is
// false.
//
//   - First reconcile (no PinnedRevisionName yet) → resolve live
//     spec, find-or-create a revision, save the pin.
//   - Subsequent reconcile, matching hash → fetch the pinned
//     revision, use as-is.
//   - Subsequent reconcile, mismatched hash:
//   - ome.io/runtime-sync annotation bumped → advance the pin to a
//     fresh revision of the live spec, clear drift.
//   - Otherwise → set RuntimeDrifted=True, skip children.
//   - Explicit spec.runtime.revision set → fetch that revision; if
//     missing, RuntimeDrifted=True (Reason=RevisionMissing) + skip.
func (r *InferenceServiceReconciler) resolvePinnedRuntime(
	ctx context.Context,
	isvc *v1beta1.InferenceService,
) (pinResult, error) {
	runtimeName := isvc.Spec.Runtime.Name
	kind := sourceKindFor(isvc.Spec.Runtime)
	sourceNS := sourceNamespaceFor(isvc, kind)
	omeNS := constants.OMENamespace

	// Resolve the LIVE spec once — needed for hash compare, and to
	// seed the first-reconcile pin / advance on ack.
	liveSpec, _, err := r.RuntimeSelector.GetRuntime(ctx, runtimeName, isvc.Namespace)
	if err != nil {
		return pinResult{}, fmt.Errorf("resolve live runtime %s: %w", runtimeName, err)
	}
	_, liveHash, err := runtimerevision.Hash(liveSpec)
	if err != nil {
		return pinResult{}, err
	}

	// Explicit revision pin overrides everything else.
	if isvc.Spec.Runtime.Revision != nil && *isvc.Spec.Runtime.Revision != "" {
		pinName := *isvc.Spec.Runtime.Revision
		rev, err := runtimerevision.FetchRevision(ctx, r.Client, omeNS, pinName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return r.driftAndSkip(ctx, isvc, "RevisionMissing",
					fmt.Sprintf("explicit pin %s not found in namespace %s", pinName, omeNS))
			}
			return pinResult{}, err
		}
		// Defense in depth: the webhook should have caught a wrong-runtime
		// pin at admission, but if a revision was renamed or relabeled,
		// surface it here instead of silently using another runtime's spec.
		if got := rev.Labels[constants.RuntimeRevisionOfLabelKey]; got != runtimeName {
			return r.driftAndSkip(ctx, isvc, "RuntimeMismatch",
				fmt.Sprintf("revision %s belongs to runtime %q but ISVC references %q",
					pinName, got, runtimeName))
		}
		pinned, err := runtimerevision.DecodeSpec(rev)
		if err != nil {
			return pinResult{}, err
		}
		isvc.Status.PinnedRevisionName = pinName
		clearRuntimeDriftedCondition(isvc)
		return pinResult{spec: pinned}, nil
	}

	// First reconcile: nothing pinned yet → create-or-find from live.
	if isvc.Status.PinnedRevisionName == "" {
		revName, err := runtimerevision.FindOrCreate(ctx, r.Client, omeNS, kind, sourceNS, runtimeName, liveSpec)
		if err != nil {
			return pinResult{}, fmt.Errorf("create initial pin for %s: %w", runtimeName, err)
		}
		isvc.Status.PinnedRevisionName = revName
		isvc.Status.LastRuntimeSyncToken = isvc.Annotations[constants.RuntimeSyncAnnotationKey]
		clearRuntimeDriftedCondition(isvc)
		return pinResult{spec: liveSpec}, nil
	}

	// Already pinned: fetch the pinned revision; missing → drift.
	pinned, err := runtimerevision.Fetch(ctx, r.Client, omeNS, isvc.Status.PinnedRevisionName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.driftAndSkip(ctx, isvc, "RevisionMissing",
				fmt.Sprintf("pinned revision %s missing from %s", isvc.Status.PinnedRevisionName, omeNS))
		}
		return pinResult{}, err
	}
	_, pinnedHash, err := runtimerevision.Hash(pinned)
	if err != nil {
		return pinResult{}, err
	}

	if liveHash == pinnedHash {
		clearRuntimeDriftedCondition(isvc)
		return pinResult{spec: pinned}, nil
	}

	// Hashes differ → drift. Check if user acked.
	currentToken := isvc.Annotations[constants.RuntimeSyncAnnotationKey]
	if currentToken != "" && currentToken != isvc.Status.LastRuntimeSyncToken {
		revName, err := runtimerevision.FindOrCreate(ctx, r.Client, omeNS, kind, sourceNS, runtimeName, liveSpec)
		if err != nil {
			return pinResult{}, fmt.Errorf("advance pin for %s: %w", runtimeName, err)
		}
		isvc.Status.PinnedRevisionName = revName
		isvc.Status.LastRuntimeSyncToken = currentToken
		clearRuntimeDriftedCondition(isvc)
		return pinResult{spec: liveSpec}, nil
	}

	return r.driftAndSkip(ctx, isvc, "RevisionMismatch",
		fmt.Sprintf("pinned %s, live spec differs; bump %s annotation to advance",
			isvc.Status.PinnedRevisionName, constants.RuntimeSyncAnnotationKey))
}

// driftAndSkip sets RuntimeDrifted=True, persists the status, and
// returns a pinResult that tells the caller to skip children
// reconciliation. Persists immediately because the caller bails out
// before the normal end-of-reconcile status update path runs.
func (r *InferenceServiceReconciler) driftAndSkip(
	ctx context.Context,
	isvc *v1beta1.InferenceService,
	reason, message string,
) (pinResult, error) {
	setRuntimeDriftedCondition(isvc, v1.ConditionTrue, reason, message)
	if err := r.Status().Update(ctx, isvc); err != nil {
		if apierrors.IsConflict(err) {
			// Next reconcile will retry; safe to swallow.
			return pinResult{skipChildren: true}, nil
		}
		return pinResult{}, fmt.Errorf("persist RuntimeDrifted: %w", err)
	}
	return pinResult{skipChildren: true}, nil
}

// sourceKindFor returns the SourceKind based on the runtime ref's
// Kind field. Defaults to ClusterServingRuntime to match the field's
// kubebuilder default.
func sourceKindFor(ref *v1beta1.ServingRuntimeRef) runtimerevision.SourceKind {
	if ref == nil || ref.Kind == nil || *ref.Kind == string(runtimerevision.KindClusterServingRuntime) {
		return runtimerevision.KindClusterServingRuntime
	}
	return runtimerevision.KindServingRuntime
}

// sourceNamespaceFor returns the source-runtime namespace for revision
// naming. Empty when cluster-scoped; the ISVC's own namespace when
// namespaced (ServingRuntimes only live alongside ISVCs that reference
// them).
func sourceNamespaceFor(isvc *v1beta1.InferenceService, kind runtimerevision.SourceKind) string {
	if kind == runtimerevision.KindClusterServingRuntime {
		return ""
	}
	return isvc.Namespace
}

// setRuntimeDriftedCondition upserts the RuntimeDrifted condition in
// the ISVC's knative-style conditions slice without disturbing other
// conditions (EngineReady, DecoderReady, Ready, …).
func setRuntimeDriftedCondition(isvc *v1beta1.InferenceService, status v1.ConditionStatus, reason, message string) {
	now := knapis.VolatileTime{Inner: metav1.Now()}
	conds := isvc.Status.Conditions
	for i, c := range conds {
		if string(c.Type) == constants.RuntimeDriftedConditionType {
			if c.Status != status || c.Reason != reason || c.Message != message {
				conds[i].Status = status
				conds[i].Reason = reason
				conds[i].Message = message
				conds[i].LastTransitionTime = now
			}
			isvc.Status.Conditions = conds
			return
		}
	}
	isvc.Status.Conditions = append(conds, knapis.Condition{
		Type:               knapis.ConditionType(constants.RuntimeDriftedConditionType),
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})
}

// clearRuntimeDriftedCondition removes RuntimeDrifted entirely.
func clearRuntimeDriftedCondition(isvc *v1beta1.InferenceService) {
	conds := isvc.Status.Conditions
	out := conds[:0]
	for _, c := range conds {
		if string(c.Type) != constants.RuntimeDriftedConditionType {
			out = append(out, c)
		}
	}
	isvc.Status.Conditions = out
}
