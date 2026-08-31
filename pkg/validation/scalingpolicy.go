// ScalingPolicy admission validation.
//
// The ScalingPolicy schema (on InferenceService.spec and on the
// ServingRuntime/ClusterServingRuntime default field) accepts three modes, but
// no controller applies Proportional or Pinned: admitting either would store a
// policy that silently behaves like Independent. Admission rejects them
// instead, with ratchet semantics on update so stored objects that already
// carry such a policy keep accepting unrelated writes.
package validation

import (
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// ReasonScalingModeNotImplemented is the admission rejection reason for a
// ScalingPolicy mode the API accepts but no controller applies. Operators and
// tests reference it by name, so it's an exported constant.
const ReasonScalingModeNotImplemented = "ScalingModeNotImplemented"

// ValidateScalingPolicy rejects a ScalingPolicy whose mode no controller
// applies (Proportional, Pinned) so the policy can never be a silent no-op.
// nil, empty mode, and Independent are accepted — they all mean "every
// Component autoscales independently", which is the implemented behavior.
// fieldPath names the field in error messages (e.g. "spec.scalingPolicy").
func ValidateScalingPolicy(policy *v1beta1.ScalingPolicy, fieldPath string) error {
	if policy == nil {
		return nil
	}
	switch policy.Mode {
	case "", v1beta1.ScalingIndependent:
		return nil
	case v1beta1.ScalingProportional, v1beta1.ScalingPinned:
		return fmt.Errorf(
			"%s.mode %q is accepted by the API but not implemented: no controller applies it, so the policy would silently behave like %q; use %q or omit the policy (%s)",
			fieldPath, policy.Mode, v1beta1.ScalingIndependent, v1beta1.ScalingIndependent,
			ReasonScalingModeNotImplemented)
	default:
		// The CRD enum already rejects unknown modes; this covers callers
		// that bypass the schema (e.g. direct Go API use).
		return fmt.Errorf("%s.mode %q is not one of %s|%s|%s",
			fieldPath, policy.Mode,
			v1beta1.ScalingIndependent, v1beta1.ScalingProportional, v1beta1.ScalingPinned)
	}
}

// ValidateScalingPolicyUpdate runs ValidateScalingPolicy only when this write
// sets or changes the policy. An unchanged policy — however it was admitted —
// never blocks the update, so objects stored with a now-rejected mode keep
// reconciling and accepting unrelated spec changes. Equality is semantic, so
// rewriting a ratio quantity in an equivalent form (e.g. "1" vs "1000m") does
// not count as a change.
func ValidateScalingPolicyUpdate(oldPolicy, newPolicy *v1beta1.ScalingPolicy, fieldPath string) error {
	if apiequality.Semantic.DeepEqual(oldPolicy, newPolicy) {
		return nil
	}
	return ValidateScalingPolicy(newPolicy, fieldPath)
}
