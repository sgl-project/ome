// ScalingPolicy admission validation shared by InferenceServices and runtimes.
package validation

import (
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// ReasonScalingModeNotImplemented is the admission rejection reason for a
// ScalingPolicy mode the API accepts but no controller applies. Operators and
// tests reference it by name, so it's an exported constant.
const ReasonScalingModeNotImplemented = "ScalingModeNotImplemented"

// ValidateScalingPolicy rejects modes that are not ready for admission.
// fieldPath names the field in error messages (e.g. "spec.scalingPolicy").
func ValidateScalingPolicy(policy *v1beta1.ScalingPolicy, fieldPath string) error {
	if policy == nil {
		return nil
	}
	switch policy.Mode {
	case "", v1beta1.ScalingIndependent:
		if policy.Proportional != nil {
			return fmt.Errorf("%s.proportional requires mode %q", fieldPath, v1beta1.ScalingProportional)
		}
		return nil
	case v1beta1.ScalingProportional:
		return ValidateProportionalShape(policy, fieldPath)
	case v1beta1.ScalingPinned:
		return fmt.Errorf(
			"%s.mode %q is accepted by the API but not implemented; use %q or omit the policy (%s)",
			fieldPath, policy.Mode, v1beta1.ScalingIndependent,
			ReasonScalingModeNotImplemented)
	default:
		// The CRD enum already rejects unknown modes; this covers callers
		// that bypass the schema (e.g. direct Go API use).
		return fmt.Errorf("%s.mode %q is not one of %s|%s|%s",
			fieldPath, policy.Mode,
			v1beta1.ScalingIndependent, v1beta1.ScalingProportional, v1beta1.ScalingPinned)
	}
}

// ValidateProportionalShape enforces the exact integer ratio contract.
func ValidateProportionalShape(policy *v1beta1.ScalingPolicy, fieldPath string) error {
	if policy == nil || policy.Proportional == nil || len(policy.Proportional.Ratios) == 0 {
		return fmt.Errorf("%s.proportional.ratios must contain a follower", fieldPath)
	}
	anchor := policy.Proportional.Anchor
	if !validScalingComponent(anchor) {
		return fmt.Errorf("%s.proportional.anchor %q is not a component", fieldPath, anchor)
	}
	for component, ratio := range policy.Proportional.Ratios {
		if !validScalingComponent(component) || component == anchor {
			return fmt.Errorf("%s.proportional.ratios[%q] must name a follower distinct from the anchor", fieldPath, component)
		}
		milli := ratio.MilliValue()
		if ratio.Sign() <= 0 || milli <= 0 || milli > 1_000_000 || ratio.Cmp(*resource.NewMilliQuantity(milli, resource.DecimalSI)) != 0 {
			return fmt.Errorf("%s.proportional.ratios[%q] must be in (0, 1000] with at most three decimal places", fieldPath, component)
		}
	}
	return nil
}

func validScalingComponent(component v1beta1.ComponentType) bool {
	return component == v1beta1.EngineComponent || component == v1beta1.DecoderComponent || component == v1beta1.RouterComponent
}

// ValidateScalingPolicyComponents checks conflicting explicitly declared
// follower autoscalers. Missing component blocks may come from the runtime;
// the reconciler validates their resolved presence before changing a count.
func ValidateScalingPolicyComponents(policy *v1beta1.ScalingPolicy, components map[v1beta1.ComponentType]*v1beta1.ComponentExtensionSpec) error {
	if policy == nil || policy.Mode != v1beta1.ScalingProportional || policy.Proportional == nil {
		return nil
	}
	for component := range policy.Proportional.Ratios {
		block := components[component]
		if block == nil {
			continue
		}
		if block.AutoscalerPolicyRef != nil {
			return fmt.Errorf("%s follower cannot reference AutoscalerPolicy %q when scaling is Proportional: remove the reference or remove %s from ratios",
				component, block.AutoscalerPolicyRef.Name, component)
		}
		if block.Autoscaler == nil {
			continue
		}
		if block.Autoscaler.Class != v1beta1.AutoscalerNone {
			return fmt.Errorf("%s follower autoscaler must have class None when scaling is Proportional", component)
		}
	}
	return nil
}

// ValidateScalingPolicyService rejects explicit modes that cannot own a
// follower's InferenceReplica and checks visible component autoscalers.
func ValidateScalingPolicyService(isvc *v1beta1.InferenceService) error {
	if isvc == nil || isvc.Spec.ScalingPolicy == nil || isvc.Spec.ScalingPolicy.Mode != v1beta1.ScalingProportional {
		return nil
	}
	if err := ValidateScalingPolicy(isvc.Spec.ScalingPolicy, "spec.scalingPolicy"); err != nil {
		return err
	}
	if isvc.Spec.ReplicaRefs != nil {
		return fmt.Errorf("Proportional scaling requires InferenceService-owned replicas")
	}
	components := map[v1beta1.ComponentType]*v1beta1.ComponentExtensionSpec{}
	modes := map[v1beta1.ComponentType]string{}
	if isvc.Spec.Engine != nil {
		components[v1beta1.EngineComponent] = &isvc.Spec.Engine.ComponentExtensionSpec
		modes[v1beta1.EngineComponent] = isvc.Spec.Engine.Annotations[constants.DeploymentMode]
	}
	if isvc.Spec.Decoder != nil {
		components[v1beta1.DecoderComponent] = &isvc.Spec.Decoder.ComponentExtensionSpec
		modes[v1beta1.DecoderComponent] = isvc.Spec.Decoder.Annotations[constants.DeploymentMode]
	}
	if isvc.Spec.Router != nil {
		components[v1beta1.RouterComponent] = &isvc.Spec.Router.ComponentExtensionSpec
		modes[v1beta1.RouterComponent] = isvc.Spec.Router.Annotations[constants.DeploymentMode]
	}
	return validateScalingComponents(isvc.Spec.ScalingPolicy, components, modes, isvc.Spec.DeploymentMode)
}

// ValidateScalingPolicyRuntime checks the declared component defaults.
func ValidateScalingPolicyRuntime(runtime *v1beta1.ServingRuntimeSpec) error {
	if runtime == nil || runtime.ScalingPolicy == nil || runtime.ScalingPolicy.Mode != v1beta1.ScalingProportional {
		return nil
	}
	if err := ValidateScalingPolicy(runtime.ScalingPolicy, "spec.scalingPolicy"); err != nil {
		return err
	}
	components := map[v1beta1.ComponentType]*v1beta1.ComponentExtensionSpec{}
	modes := map[v1beta1.ComponentType]string{}
	if runtime.EngineConfig != nil {
		components[v1beta1.EngineComponent] = &runtime.EngineConfig.ComponentExtensionSpec
		modes[v1beta1.EngineComponent] = runtime.EngineConfig.Annotations[constants.DeploymentMode]
	}
	if runtime.DecoderConfig != nil {
		components[v1beta1.DecoderComponent] = &runtime.DecoderConfig.ComponentExtensionSpec
		modes[v1beta1.DecoderComponent] = runtime.DecoderConfig.Annotations[constants.DeploymentMode]
	}
	if runtime.RouterConfig != nil {
		components[v1beta1.RouterComponent] = &runtime.RouterConfig.ComponentExtensionSpec
		modes[v1beta1.RouterComponent] = runtime.RouterConfig.Annotations[constants.DeploymentMode]
	}
	return validateScalingComponents(runtime.ScalingPolicy, components, modes, nil)
}

func validateScalingComponents(policy *v1beta1.ScalingPolicy, components map[v1beta1.ComponentType]*v1beta1.ComponentExtensionSpec,
	modes map[v1beta1.ComponentType]string, defaultMode *constants.DeploymentModeType) error {
	if components[policy.Proportional.Anchor] == nil {
		return fmt.Errorf("spec.scalingPolicy.proportional.anchor %q requires a declared component", policy.Proportional.Anchor)
	}
	for follower := range policy.Proportional.Ratios {
		if components[follower] == nil {
			return fmt.Errorf("spec.scalingPolicy.proportional.ratios[%q] requires a declared component", follower)
		}
	}
	for component := range policy.Proportional.Ratios {
		if err := validateScalingMode(component, modes[component], defaultMode); err != nil {
			return err
		}
	}
	if err := validateScalingMode(policy.Proportional.Anchor, modes[policy.Proportional.Anchor], defaultMode); err != nil {
		return err
	}
	return ValidateScalingPolicyComponents(policy, components)
}

func validateScalingMode(component v1beta1.ComponentType, explicit string, defaultMode *constants.DeploymentModeType) error {
	if explicit == "" && defaultMode != nil {
		explicit = string(*defaultMode)
	}
	if explicit != "" && explicit != string(constants.OMENative) {
		return fmt.Errorf("Proportional scaling requires OMENative %s, not deployment mode %q", component, explicit)
	}
	return nil
}

// Update validation revisits ownership when the policy or one of its writers
// changes. Unrelated updates to stored policies do not revalidate old state.
func ValidateScalingPolicyServiceUpdate(old, current *v1beta1.InferenceService) error {
	if old != nil && apiequality.Semantic.DeepEqual(old.Spec.ScalingPolicy, current.Spec.ScalingPolicy) &&
		apiequality.Semantic.DeepEqual(old.Spec.ReplicaRefs, current.Spec.ReplicaRefs) &&
		apiequality.Semantic.DeepEqual(old.Spec.DeploymentMode, current.Spec.DeploymentMode) &&
		!scalingOwnerChanged(engineExtension(old.Spec.Engine), engineExtension(current.Spec.Engine),
			engineMode(old.Spec.Engine), engineMode(current.Spec.Engine)) &&
		!scalingOwnerChanged(decoderExtension(old.Spec.Decoder), decoderExtension(current.Spec.Decoder),
			decoderMode(old.Spec.Decoder), decoderMode(current.Spec.Decoder)) &&
		!scalingOwnerChanged(routerExtension(old.Spec.Router), routerExtension(current.Spec.Router),
			routerMode(old.Spec.Router), routerMode(current.Spec.Router)) {
		return nil
	}
	return ValidateScalingPolicyService(current)
}

func ValidateScalingPolicyRuntimeUpdate(old, current *v1beta1.ServingRuntimeSpec) error {
	if old != nil && apiequality.Semantic.DeepEqual(old.ScalingPolicy, current.ScalingPolicy) &&
		!scalingOwnerChanged(engineExtension(old.EngineConfig), engineExtension(current.EngineConfig),
			engineMode(old.EngineConfig), engineMode(current.EngineConfig)) &&
		!scalingOwnerChanged(decoderExtension(old.DecoderConfig), decoderExtension(current.DecoderConfig),
			decoderMode(old.DecoderConfig), decoderMode(current.DecoderConfig)) &&
		!scalingOwnerChanged(routerExtension(old.RouterConfig), routerExtension(current.RouterConfig),
			routerMode(old.RouterConfig), routerMode(current.RouterConfig)) {
		return nil
	}
	return ValidateScalingPolicyRuntime(current)
}

func scalingOwnerChanged(old, current *v1beta1.ComponentExtensionSpec, oldMode, currentMode string) bool {
	if (old == nil) != (current == nil) || oldMode != currentMode {
		return true
	}
	return old != nil && (!apiequality.Semantic.DeepEqual(old.Autoscaler, current.Autoscaler) ||
		!apiequality.Semantic.DeepEqual(old.AutoscalerPolicyRef, current.AutoscalerPolicyRef))
}

func engineExtension(engine *v1beta1.EngineSpec) *v1beta1.ComponentExtensionSpec {
	if engine == nil {
		return nil
	}
	return &engine.ComponentExtensionSpec
}

func engineMode(engine *v1beta1.EngineSpec) string {
	if engine == nil {
		return ""
	}
	return engine.Annotations[constants.DeploymentMode]
}

func decoderExtension(decoder *v1beta1.DecoderSpec) *v1beta1.ComponentExtensionSpec {
	if decoder == nil {
		return nil
	}
	return &decoder.ComponentExtensionSpec
}

func decoderMode(decoder *v1beta1.DecoderSpec) string {
	if decoder == nil {
		return ""
	}
	return decoder.Annotations[constants.DeploymentMode]
}

func routerExtension(router *v1beta1.RouterSpec) *v1beta1.ComponentExtensionSpec {
	if router == nil {
		return nil
	}
	return &router.ComponentExtensionSpec
}

func routerMode(router *v1beta1.RouterSpec) string {
	if router == nil {
		return ""
	}
	return router.Annotations[constants.DeploymentMode]
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
