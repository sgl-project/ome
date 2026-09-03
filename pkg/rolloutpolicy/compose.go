package rolloutpolicy

import (
	"errors"
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// ErrProgressionMismatch marks a declared-kind lie: the ref promised one
// progression kind and the policy body carries another. Callers park the run
// on it — the ISVC's shape rules were admitted against the declaration, so
// executing the actual body could violate them.
var ErrProgressionMismatch = errors.New("declared progression does not match the policy body")

// ComposeGroup resolves one spec group into the group a run pins: the
// consumer-owned shape (components, soak, maintainRatio) plus exactly one
// progression, taken from the inline arm when present (inline always outranks
// the ref — the preview/escape mechanism) and otherwise from the referenced
// policy's body, verbatim. The pinned group never carries a PolicyRef: the
// ref was resolved here.
//
// A declared-kind mismatch is an error, never a fallback: the ISVC's shape
// rules were admitted against the DECLARED progression, so executing a body
// of a different kind could violate them (e.g. soak accepted because the ref
// declared blueGreen, silently dropped on a canary body). The caller parks
// the run on error.
func ComposeGroup(g *v1beta1.RolloutGroup, policy *v1beta1.RolloutPolicySpec) (v1beta1.RolloutGroup, error) {
	out := *g.DeepCopy()
	out.PolicyRef = nil
	if hasInlineProgression(g) {
		return out, nil
	}
	if g.PolicyRef == nil {
		// No inline arm and no ref: the group defaults to blueGreen at
		// resolution. Pin it explicitly so the frozen plan is self-describing.
		out.BlueGreen = &v1beta1.GroupBlueGreen{}
		return out, nil
	}
	if policy == nil {
		return v1beta1.RolloutGroup{}, fmt.Errorf("policyRef %q: no policy body supplied", g.PolicyRef.Name)
	}
	actual, ok := policy.Progression()
	if !ok {
		return v1beta1.RolloutGroup{}, fmt.Errorf("policyRef %q: policy carries no progression body", g.PolicyRef.Name)
	}
	if actual != g.PolicyRef.Progression {
		return v1beta1.RolloutGroup{}, fmt.Errorf("policyRef %q declares progression %q but the policy body is %q: %w",
			g.PolicyRef.Name, g.PolicyRef.Progression, actual, ErrProgressionMismatch)
	}
	body := policy.DeepCopy()
	out.Canary = body.Canary
	out.BlueGreen = body.BlueGreen
	out.RollingUpdate = body.RollingUpdate
	return out, nil
}

// hasInlineProgression reports whether the group sets any inline progression
// arm (which then outranks a coexisting PolicyRef).
func hasInlineProgression(g *v1beta1.RolloutGroup) bool {
	return g.Canary != nil || g.BlueGreen != nil || g.RollingUpdate != nil
}

// GroupSource reports which source a run opened now would pin for the group.
func GroupSource(g *v1beta1.RolloutGroup) v1beta1.RolloutPlanSource {
	if !hasInlineProgression(g) && g.PolicyRef != nil {
		return v1beta1.RolloutPlanSourcePolicy
	}
	return v1beta1.RolloutPlanSourceInline
}
