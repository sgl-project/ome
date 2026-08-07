// Per-Component LifecycleSpec.UpdateStrategy.RollingUpdate validation.
//
// The CRD schema's XIntOrString marker already gates the structural
// shape (must parse as integer or string). These helpers add the
// semantic checks the apiserver can't express: percent strings must
// match "<n>%" with 0 <= n <= 100, integer values must be >= 0.
//
// As of the per-Component MaxSurge/MaxUnavailable wire-up, the rollout
// budget now reads the per-Component RollingUpdate fields directly
// (see pkg/controller/v1beta1/workload/budget.go) and composes them
// with the coordination-group budgets as independent layers — the
// effective cap is the min of both. Because the per-Component layer
// now actively gates rollouts, we MUST also reject the unstartable
// configuration (MaxSurge=0 AND MaxUnavailable=0), mirroring upstream
// appsv1.Deployment's same rule. Either knob nil (operator unset) is
// fine — nil means "this layer does not cap" so the rollout falls
// through to the group layer.
//
// The webhook ALSO rejects the equivalent deadlock at the
// coordination-pacing layer (see validatePacingNotZeroBudget in
// coordination.go) so the operator-visible blast radius is consistent
// across both layers.
package validation

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// Admission rejection reasons for per-Component lifecycle settings.
const (
	ReasonInvalidRollingUpdatePercent = "InvalidRollingUpdatePercent"
	ReasonInvalidRollingUpdateInteger = "InvalidRollingUpdateInteger"
	// ReasonRollingUpdateZeroBudget rejects MaxSurge=0 AND MaxUnavailable=0
	// set explicitly on the same Component's rollingUpdate block — the
	// rollout would be unstartable (no surge headroom, no drain headroom).
	// Mirrors the upstream appsv1.Deployment validation rule. Either
	// field nil (defaulted upstream) does NOT trip this check.
	ReasonRollingUpdateZeroBudget = "RollingUpdateZeroBudget"
)

// ValidateLifecycle inspects every Component's LifecycleSpec on the
// InferenceService. Today the only per-Component knobs that need
// semantic validation past the CRD schema are
// UpdateStrategy.RollingUpdate.MaxUnavailable and .MaxSurge — both
// *intstr.IntOrString.
// Wired into the webhook from inference_service_validation.go.
func ValidateLifecycle(spec *v1beta1.InferenceServiceSpec) error {
	if spec == nil {
		return nil
	}
	if spec.Engine != nil {
		if err := validateComponentLifecycle("engine", spec.Engine.Lifecycle); err != nil {
			return err
		}
	}
	if spec.Decoder != nil {
		if err := validateComponentLifecycle("decoder", spec.Decoder.Lifecycle); err != nil {
			return err
		}
	}
	if spec.Router != nil {
		if err := validateComponentLifecycle("router", spec.Router.Lifecycle); err != nil {
			return err
		}
	}
	return nil
}

func validateComponentLifecycle(name string, lc *v1beta1.LifecycleSpec) error {
	if lc == nil || lc.UpdateStrategy == nil || lc.UpdateStrategy.RollingUpdate == nil {
		return nil
	}
	ru := lc.UpdateStrategy.RollingUpdate
	if err := validateRollingUpdateIntOrString(name, "maxUnavailable", ru.MaxUnavailable); err != nil {
		return err
	}
	if err := validateRollingUpdateIntOrString(name, "maxSurge", ru.MaxSurge); err != nil {
		return err
	}
	if err := validateRollingUpdateNotBothZero(name, ru.MaxSurge, ru.MaxUnavailable); err != nil {
		return err
	}
	return nil
}

// validateRollingUpdateNotBothZero rejects the unstartable configuration
// where both MaxSurge and MaxUnavailable are EXPLICITLY set to zero on
// the same Component's rollingUpdate block. With no surge headroom and
// no drain headroom the rollout can never make progress — the operator
// almost certainly mistyped one of the two.
//
// This applies to in-place strategies too. An in-place update marks the
// pod not-ready while it patches the running container, which consumes the
// per-Component MaxUnavailable budget — so maxUnavailable=0 stalls an
// in-place roll exactly like a surge/drain one: the patch never starts and
// the pods sit untouched. Exempting in-place on the premise that it
// "neither surges nor drains" would be wrong — the mark-not-ready does
// consume the budget, so an exemption only converts a fail-fast
// admission error into a silent runtime deadlock.
//
// Either field nil (operator left it for the layer below to default)
// does NOT trip this check; the per-Component layer treats nil as "no
// cap" and defers to the coordination-group ceiling, which has its
// own non-zero defaults (25% / 25%). The check only fires when the
// operator wrote 0 on BOTH knobs intentionally.
//
// Mirrors the upstream appsv1.Deployment.Strategy.RollingUpdate
// validation: see k/k pkg/apis/apps/validation/validation.go
// ValidateRollingUpdateDeployment.
func validateRollingUpdateNotBothZero(component string, maxSurge, maxUnavailable *intstr.IntOrString) error {
	if !explicitZero(maxSurge) || !explicitZero(maxUnavailable) {
		return nil
	}
	return fmt.Errorf("spec.%s.lifecycle.updateStrategy.rollingUpdate: maxSurge=0 and maxUnavailable=0 together yield an unstartable rollout (%s)",
		component, ReasonRollingUpdateZeroBudget)
}

// explicitZero reports whether v is non-nil AND parses to the integer
// 0 (either intstr.FromInt(0) or "0%"). A nil pointer is NOT explicit
// — operator left it unset for upstream defaulting. Any positive
// integer or percent fails the predicate.
func explicitZero(v *intstr.IntOrString) bool {
	if v == nil {
		return false
	}
	if v.Type == intstr.Int {
		return v.IntValue() == 0
	}
	s := v.StrVal
	if !strings.HasSuffix(s, "%") {
		return false
	}
	pctStr := strings.TrimSuffix(s, "%")
	pct, err := strconv.Atoi(pctStr)
	if err != nil {
		return false
	}
	return pct == 0
}

// validateRollingUpdateIntOrString enforces:
//   - integer values >= 0
//   - string values match "<n>%" with 0 <= n <= 100
//
// Returns nil for nil inputs (the workload reconciler defaults nil to
// 25% — both the API documentation and pacingWithDefaults agree on
// the same fallback).
func validateRollingUpdateIntOrString(component, field string, v *intstr.IntOrString) error {
	if v == nil {
		return nil
	}
	if v.Type == intstr.Int {
		if v.IntValue() < 0 {
			return fmt.Errorf("spec.%s.lifecycle.updateStrategy.rollingUpdate.%s=%d must be >= 0 (%s)",
				component, field, v.IntValue(), ReasonInvalidRollingUpdateInteger)
		}
		return nil
	}
	// String form — must be "<n>%" with 0 <= n <= 100.
	s := v.StrVal
	if !strings.HasSuffix(s, "%") {
		return fmt.Errorf("spec.%s.lifecycle.updateStrategy.rollingUpdate.%s=%q must be an integer count or a percent string like \"25%%\" (%s)",
			component, field, s, ReasonInvalidRollingUpdatePercent)
	}
	pctStr := strings.TrimSuffix(s, "%")
	pct, err := strconv.Atoi(pctStr)
	if err != nil {
		return fmt.Errorf("spec.%s.lifecycle.updateStrategy.rollingUpdate.%s=%q has a malformed percent value (%s)",
			component, field, s, ReasonInvalidRollingUpdatePercent)
	}
	if pct < 0 || pct > 100 {
		return fmt.Errorf("spec.%s.lifecycle.updateStrategy.rollingUpdate.%s=%q percent must be between 0%% and 100%% (%s)",
			component, field, s, ReasonInvalidRollingUpdatePercent)
	}
	return nil
}
