// Cross-Component rollout coordination validation (v2 spec.rollout.groups[]).
//
// Webhook-level admission rules for the coordination-style progressions
// (blueGreen / rollingUpdate) on spec.rollout.groups[]: per-group structural
// checks (valid components, order subset), cross-group checks (a Component
// appears in at most one group; a group references a declared Component; groups
// are OMENative), pacing zero-budget, and the RollingUpdate lockstep on update.
// The one-of progression shape and order⊆components are also enforced by the CRD
// CEL rules; these are the value-level mirrors plus the cross-group rules CEL
// can't express. Canary groups are validated by ValidateCanary.
package validation

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// Admission rejection reasons. Operators and tests reference these by
// name, so they're exported constants.
const (
	ReasonDuplicateComponentInCoordinationGroups = "DuplicateComponentInCoordinationGroups"
	ReasonInvalidComponentInCoordinationGroup    = "InvalidComponentInCoordinationGroup"
	ReasonCoordinationRequiresOMENative          = "CoordinationRequiresOMENative"
	ReasonRatioToleranceTooHigh                  = "RatioToleranceTooHigh"
	ReasonOrphanCoordinationGroup                = "OrphanCoordinationGroup"
	ReasonInvalidCoordinationOrder               = "InvalidCoordinationOrder"
	ReasonRollingUpdateLockstepViolation         = "RollingUpdateLockstepViolation"
	// ReasonSequentialDuplicateOrder flags a duplicate entry in a group's Order
	// (the rule applies to any group's surge Order, not only the v1 Sequential
	// policy that is gone).
	ReasonSequentialDuplicateOrder = "SequentialDuplicateOrder"
	// ReasonSoakNotHonored flags a Soak the engine would silently drop. Soak is
	// honored only by the Sequential state machine (a collapsed run of
	// single-Component blueGreen groups); it is never honored on a canary,
	// multi-Component, or rollingUpdate group, nor on a rollout that does not
	// collapse to Sequential.
	ReasonSoakNotHonored = "SoakNotHonored"
)

// ratioToleranceWarningThreshold is the upper bound the validator considers
// operator-safe for the MaintainRatio tolerance knob. Values above this trigger a
// non-blocking warning since they effectively disable ratio enforcement.
const ratioToleranceWarningThreshold int32 = 50

// validComponents is the closed set of Components allowed in a rollout group.
var validComponents = map[v1beta1.ComponentType]struct{}{
	v1beta1.RouterComponent:  {},
	v1beta1.EngineComponent:  {},
	v1beta1.DecoderComponent: {},
}

// ValidateCoordination runs the coordination-style (blueGreen / rollingUpdate)
// admission rules over spec.rollout.groups[]. Returns the first error; nil on
// success. Update-time (RollingUpdate lockstep) checks live in
// ValidateCoordinationUpdate.
func ValidateCoordination(spec *v1beta1.InferenceServiceSpec) error {
	groups := spec.GetRolloutGroups()
	if len(groups) == 0 {
		return nil
	}
	if err := validateGroupComponents(groups); err != nil {
		return err
	}
	if err := validateNoDuplicateMembership(groups); err != nil {
		return err
	}
	if err := validateOrphanGroups(spec, groups); err != nil {
		return err
	}
	if err := validateComponentsAreOMENative(spec, groups); err != nil {
		return err
	}
	if err := validatePacingNotZeroBudget(groups); err != nil {
		return err
	}
	if err := validateSoakOnlyWhenSequenced(groups); err != nil {
		return err
	}
	return nil
}

// ValidateCoordinationUpdate runs the update-time rule comparing old vs new spec:
// a rollingUpdate group bumps all its Components together (lockstep). Structural
// validation runs separately on the new spec.
func ValidateCoordinationUpdate(oldSpec, newSpec *v1beta1.InferenceServiceSpec) error {
	groups := newSpec.GetRolloutGroups()
	for i := range groups {
		g := &groups[i]
		if g.RollingUpdate == nil {
			continue
		}
		if err := validateRollingUpdateLockstep(oldSpec, newSpec, g.Components); err != nil {
			return err
		}
	}
	return nil
}

// CoordinationRatioToleranceWarning returns a warning when any group sets a
// MaintainRatio tolerance above the safe threshold. Empty when none applies.
func CoordinationRatioToleranceWarning(spec *v1beta1.InferenceServiceSpec) string {
	groups := spec.GetRolloutGroups()
	for i := range groups {
		mr := groups[i].MaintainRatio
		if mr == nil || mr.Tolerance == nil {
			continue
		}
		if *mr.Tolerance > ratioToleranceWarningThreshold {
			return fmt.Sprintf(
				"spec.rollout.groups[%d].maintainRatio.tolerance=%d effectively disables ratio enforcement (>%d) (%s)",
				i, *mr.Tolerance, ratioToleranceWarningThreshold, ReasonRatioToleranceTooHigh)
		}
	}
	return ""
}

// validateGroupComponents checks that every coordination-style group references
// only valid Components and that its surge Order is a duplicate-free subset of
// its Components. Canary groups are validated by ValidateCanary.
func validateGroupComponents(groups []v1beta1.RolloutGroup) error {
	for i := range groups {
		g := &groups[i]
		if g.Canary != nil {
			continue
		}
		for _, c := range g.Components {
			if _, ok := validComponents[c]; !ok {
				return fmt.Errorf("spec.rollout.groups[%d]: component %q is not one of router/engine/decoder (%s)",
					i, c, ReasonInvalidComponentInCoordinationGroup)
			}
		}
		if err := validateOrderSubset(i, g.Components, g.Order); err != nil {
			return err
		}
	}
	return nil
}

// validateOrderSubset enforces that order is a duplicate-free subset of
// components. Applies to any group with an Order (the surge sequence).
func validateOrderSubset(idx int, components, order []v1beta1.ComponentType) error {
	if len(order) == 0 {
		return nil
	}
	in := make(map[v1beta1.ComponentType]struct{}, len(components))
	for _, c := range components {
		in[c] = struct{}{}
	}
	seen := make(map[v1beta1.ComponentType]struct{}, len(order))
	for _, c := range order {
		if _, ok := in[c]; !ok {
			return fmt.Errorf("spec.rollout.groups[%d]: order entry %q is not in components (%s)",
				idx, c, ReasonInvalidCoordinationOrder)
		}
		if _, dup := seen[c]; dup {
			return fmt.Errorf("spec.rollout.groups[%d]: order entry %q appears more than once (%s)",
				idx, c, ReasonSequentialDuplicateOrder)
		}
		seen[c] = struct{}{}
	}
	return nil
}

// validateNoDuplicateMembership rejects a Component appearing in more than one
// group (across ALL groups, canary included). Each Component rolls under exactly
// one group.
func validateNoDuplicateMembership(groups []v1beta1.RolloutGroup) error {
	seen := map[v1beta1.ComponentType]int{}
	for i := range groups {
		for _, c := range groups[i].Components {
			if prev, ok := seen[c]; ok {
				return fmt.Errorf("spec.rollout.groups: component %q appears in groups[%d] and groups[%d] (%s)",
					c, prev, i, ReasonDuplicateComponentInCoordinationGroups)
			}
			seen[c] = i
		}
	}
	return nil
}

func validateOrphanGroups(spec *v1beta1.InferenceServiceSpec, groups []v1beta1.RolloutGroup) error {
	declared := declaredComponents(spec)
	for i := range groups {
		anyDeclared := false
		for _, c := range groups[i].Components {
			if _, ok := declared[c]; ok {
				anyDeclared = true
				break
			}
		}
		if !anyDeclared {
			return fmt.Errorf("spec.rollout.groups[%d]: no component in this group is declared on the InferenceService (%s)",
				i, ReasonOrphanCoordinationGroup)
		}
	}
	return nil
}

// validateComponentsAreOMENative requires every Component in a coordination-style
// group to declare OMENative. Canary groups are checked in ValidateCanary.
func validateComponentsAreOMENative(spec *v1beta1.InferenceServiceSpec, groups []v1beta1.RolloutGroup) error {
	modeFor := omenativeDeploymentModeMap(spec)
	for i := range groups {
		g := &groups[i]
		if g.Canary != nil {
			continue
		}
		for _, c := range g.Components {
			declared, ok := modeFor[c]
			if !ok {
				continue // not declared — handled by the orphan check
			}
			if declared != string(constants.OMENative) {
				return fmt.Errorf("spec.rollout.groups[%d]: component %q has deploymentMode=%q; coordination requires OMENative (%s)",
					i, c, declared, ReasonCoordinationRequiresOMENative)
			}
		}
	}
	return nil
}

// validatePacingNotZeroBudget rejects a rollingUpdate group that explicitly sets
// BOTH maxSurge=0 AND maxUnavailable=0: no surge headroom and no drain headroom
// deadlocks the roll. nil (defaulted) values resolve to 25% and are not zero.
func validatePacingNotZeroBudget(groups []v1beta1.RolloutGroup) error {
	for i := range groups {
		ru := groups[i].RollingUpdate
		if ru == nil {
			continue
		}
		if explicitZeroIntOrString(ru.MaxSurge) && explicitZeroIntOrString(ru.MaxUnavailable) {
			return fmt.Errorf("spec.rollout.groups[%d]: maxSurge=0 AND maxUnavailable=0 deadlocks rollouts — set at least one non-zero (%s)",
				i, ReasonZeroBudgetPacingUnstartable)
		}
	}
	return nil
}

// explicitZeroIntOrString reports whether the user explicitly set the
// IntOrString to a zero value (literal 0 or "0" or "0%"). Nil is not zero — nil
// means "use default" and the resolver fills in 25%.
func explicitZeroIntOrString(v *intstr.IntOrString) bool {
	if v == nil {
		return false
	}
	if v.Type == intstr.Int {
		return v.IntValue() == 0
	}
	return v.StrVal == "0" || v.StrVal == "0%"
}

func validateRollingUpdateLockstep(oldSpec, newSpec *v1beta1.InferenceServiceSpec, components []v1beta1.ComponentType) error {
	oldImages := componentImages(oldSpec, components)
	newImages := componentImages(newSpec, components)
	anyChanged := false
	allChanged := true
	for _, c := range components {
		o, n := oldImages[c], newImages[c]
		if o != n {
			anyChanged = true
		} else {
			allChanged = false
		}
	}
	if anyChanged && !allChanged {
		return fmt.Errorf("spec.rollout: rollingUpdate group requires all components to bump together (%s)",
			ReasonRollingUpdateLockstepViolation)
	}
	return nil
}

// declaredComponents returns the set of Components present on the spec.
func declaredComponents(spec *v1beta1.InferenceServiceSpec) map[v1beta1.ComponentType]struct{} {
	out := map[v1beta1.ComponentType]struct{}{}
	if spec == nil {
		return out
	}
	if spec.Engine != nil {
		out[v1beta1.EngineComponent] = struct{}{}
	}
	if spec.Decoder != nil {
		out[v1beta1.DecoderComponent] = struct{}{}
	}
	if spec.Router != nil {
		out[v1beta1.RouterComponent] = struct{}{}
	}
	return out
}

// omenativeDeploymentModeMap returns Component → effective
// deploymentMode value for each Component on the spec. Effective value
// follows the resolution chain: per-Component annotation >
// spec.deploymentMode > "" (empty when neither is set). Shape-derived
// behavior (Leader/Worker → OMENative; PDDisaggregated) is intentionally
// NOT considered — coordination only cares about the dispatch backend,
// not the shape.
func omenativeDeploymentModeMap(spec *v1beta1.InferenceServiceSpec) map[v1beta1.ComponentType]string {
	out := map[v1beta1.ComponentType]string{}
	if spec == nil {
		return out
	}
	if spec.Engine != nil {
		out[v1beta1.EngineComponent] = resolveComponentDeploymentMode(spec.Engine.Annotations, spec.DeploymentMode)
	}
	if spec.Decoder != nil {
		out[v1beta1.DecoderComponent] = resolveComponentDeploymentMode(spec.Decoder.Annotations, spec.DeploymentMode)
	}
	if spec.Router != nil {
		out[v1beta1.RouterComponent] = resolveComponentDeploymentMode(spec.Router.Annotations, spec.DeploymentMode)
	}
	return out
}

// componentImages returns the container image per Component in `components`.
// Runner.Image takes precedence over the first PodSpec container image.
func componentImages(spec *v1beta1.InferenceServiceSpec, components []v1beta1.ComponentType) map[v1beta1.ComponentType]string {
	out := map[v1beta1.ComponentType]string{}
	if spec == nil {
		return out
	}
	for _, c := range components {
		var img string
		switch c {
		case v1beta1.EngineComponent:
			if spec.Engine != nil {
				img = engineRunnerOrPodSpecImage(spec.Engine)
			}
		case v1beta1.DecoderComponent:
			if spec.Decoder != nil {
				img = decoderRunnerOrPodSpecImage(spec.Decoder)
			}
		case v1beta1.RouterComponent:
			if spec.Router != nil {
				img = routerRunnerOrPodSpecImage(spec.Router)
			}
		}
		if img != "" {
			out[c] = img
		}
	}
	return out
}

func engineRunnerOrPodSpecImage(e *v1beta1.EngineSpec) string {
	if e.Runner != nil && e.Runner.Image != "" {
		return e.Runner.Image
	}
	if len(e.PodSpec.Containers) > 0 {
		return e.PodSpec.Containers[0].Image
	}
	return ""
}

func decoderRunnerOrPodSpecImage(d *v1beta1.DecoderSpec) string {
	if d.Runner != nil && d.Runner.Image != "" {
		return d.Runner.Image
	}
	if len(d.PodSpec.Containers) > 0 {
		return d.PodSpec.Containers[0].Image
	}
	return ""
}

func routerRunnerOrPodSpecImage(r *v1beta1.RouterSpec) string {
	if r.Runner != nil && r.Runner.Image != "" {
		return r.Runner.Image
	}
	if len(r.PodSpec.Containers) > 0 {
		return r.PodSpec.Containers[0].Image
	}
	return ""
}

// rolloutCollapsesToSequential mirrors the engine's collapseSequential: the
// coordination-style (non-canary) groups fold into the Sequential state machine
// — the only path that honors Soak — exactly when there are 2+ of them and every
// one is a single-Component blueGreen group (no rollingUpdate). Anything else (a
// multi-Component group, a rollingUpdate group, or fewer than 2) runs the groups
// concurrently, where Soak is dropped.
func rolloutCollapsesToSequential(groups []v1beta1.RolloutGroup) bool {
	n := 0
	for i := range groups {
		g := &groups[i]
		if g.Canary != nil {
			continue // canary groups are excluded from the coordination collapse
		}
		n++
		if len(g.Components) != 1 || g.RollingUpdate != nil {
			return false
		}
	}
	return n >= 2
}

// validateSoakOnlyWhenSequenced rejects a Soak the engine would silently drop.
// Soak is honored only by the Sequential state machine (the collapsed run of
// single-Component blueGreen groups). It is never honored on a canary group, and
// on a rollout that does not collapse the groups run concurrently and the soak is
// dropped. Accepting it there would make an operator believe a wait is enforced
// when it is not, so admission rejects it instead.
func validateSoakOnlyWhenSequenced(groups []v1beta1.RolloutGroup) error {
	collapses := rolloutCollapsesToSequential(groups)
	for i := range groups {
		g := &groups[i]
		if g.Soak == nil {
			continue
		}
		if g.Canary != nil {
			return fmt.Errorf("spec.rollout.groups[%d]: soak is not honored on a canary group (the canary engine does not gate on it) (%s)",
				i, ReasonSoakNotHonored)
		}
		if !collapses {
			return fmt.Errorf("spec.rollout.groups[%d]: soak is honored only when the rollout is a sequence of single-Component groups (which roll one-at-a-time); this rollout runs groups concurrently, so the soak would be dropped (%s)",
				i, ReasonSoakNotHonored)
		}
	}
	return nil
}
