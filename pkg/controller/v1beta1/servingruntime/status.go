// Package servingruntime hosts the runtime inheritance controllers
// (one reconciler per CRD scope).
package servingruntime

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimeinheritance"
)

// Reasons surfaced on the InheritanceReady condition.
const (
	ReasonResolved         = "Resolved"
	ReasonParentNotFound   = "ParentNotFound"
	ReasonCycle            = "InheritanceCycle"
	ReasonMaxDepthExceeded = "MaxDepthExceeded"
	ReasonResolverInternal = "ResolverError"
)

// projectInheritanceResult applies a resolver outcome onto a copy of
// the current status. On error, the prior InheritanceChain is preserved
// (operator history) and only the condition flips.
func projectInheritanceResult(
	current v1beta1.ServingRuntimeStatus,
	generation int64,
	chain []string,
	resolveErr error,
) v1beta1.ServingRuntimeStatus {
	updated := *current.DeepCopy()
	cond := metav1.Condition{
		Type:               constants.InheritanceReadyConditionType,
		ObservedGeneration: generation,
	}

	if resolveErr != nil {
		cond.Status = metav1.ConditionFalse
		cond.Reason = classifyResolveError(resolveErr)
		cond.Message = resolveErr.Error()
	} else {
		updated.InheritanceChain = chain
		cond.Status = metav1.ConditionTrue
		cond.Reason = ReasonResolved
		cond.Message = "inheritance chain resolved"
	}

	setCondition(&updated.Conditions, cond)
	return updated
}

// classifyResolveError maps a resolver error onto a stable Reason.
func classifyResolveError(err error) string {
	var pnf *runtimeinheritance.ParentNotFoundError
	if errors.As(err, &pnf) {
		return ReasonParentNotFound
	}
	var ce *runtimeinheritance.CycleError
	if errors.As(err, &ce) {
		return ReasonCycle
	}
	var de *runtimeinheritance.MaxDepthExceededError
	if errors.As(err, &de) {
		return ReasonMaxDepthExceeded
	}
	return ReasonResolverInternal
}

// setCondition upserts cond on the slice, preserving LastTransitionTime
// when Status didn't flip.
func setCondition(conditions *[]metav1.Condition, cond metav1.Condition) {
	if cond.LastTransitionTime.IsZero() {
		cond.LastTransitionTime = metav1.Now()
	}
	for i := range *conditions {
		if (*conditions)[i].Type != cond.Type {
			continue
		}
		if (*conditions)[i].Status == cond.Status {
			cond.LastTransitionTime = (*conditions)[i].LastTransitionTime
		}
		(*conditions)[i] = cond
		return
	}
	*conditions = append(*conditions, cond)
}
