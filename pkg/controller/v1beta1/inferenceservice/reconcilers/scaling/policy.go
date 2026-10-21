package scaling

import (
	"fmt"
	"math"

	"k8s.io/utils/ptr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/autoscaler"
	"sigs.k8s.io/ome/pkg/validation"
)

// Resolve chooses the complete policy declared closest to the service.
func Resolve(isvc *v1beta1.InferenceService, runtime *v1beta1.ServingRuntimeSpec) (*v1beta1.ScalingPolicy, string) {
	if isvc != nil && isvc.Spec.ScalingPolicy != nil {
		return isvc.Spec.ScalingPolicy.DeepCopy(), "isvc"
	}
	if runtime != nil && runtime.ScalingPolicy != nil {
		return runtime.ScalingPolicy.DeepCopy(), "runtime"
	}
	return nil, ""
}

// componentIssue is shared by ownership selection and reconciliation: never
// remove a component's default autoscaler unless the coordinator can write it.
func componentIssue(component v1beta1.ComponentType, components map[v1beta1.ComponentType]*v1beta1.ComponentExtensionSpec,
	modes map[v1beta1.ComponentType]constants.DeploymentModeType) string {
	if components[component] == nil {
		return fmt.Sprintf("%s is not declared on the InferenceService", component)
	}
	if modes[component] != constants.OMENative {
		return fmt.Sprintf("%s uses deployment mode %q, not OMENative", component, modes[component])
	}
	return ""
}

// Follows reports whether the coordinator can own this component's desired
// count. Another invalid follower does not prevent this one from following.
func Follows(policy *v1beta1.ScalingPolicy, component v1beta1.ComponentType,
	components map[v1beta1.ComponentType]*v1beta1.ComponentExtensionSpec,
	modes map[v1beta1.ComponentType]constants.DeploymentModeType) bool {
	if policy == nil || policy.Mode != v1beta1.ScalingProportional || policy.Proportional == nil {
		return false
	}
	if validation.ValidateProportionalShape(policy, "spec.scalingPolicy") != nil {
		return false
	}
	if componentIssue(policy.Proportional.Anchor, components, modes) != "" {
		return false
	}
	_, ok := policy.Proportional.Ratios[component]
	return ok && componentIssue(component, components, modes) == ""
}

// Field names of the two bounds, as the status and events name them.
const (
	boundMinReplicas = "minReplicas"
	boundMaxReplicas = "maxReplicas"
)

// Bounds are a follower's own replica bounds. Max is nil when the follower
// declares no maxReplicas: the coordinator is its only writer and the anchor's
// bounds already cap the product, so the follower has no ceiling of its own.
type Bounds struct {
	Min int32
	Max *int32
}

// FollowerBounds reads a follower's bounds from its component spec: the same
// minimum the autoscaler path uses, and a maximum only when one is declared.
func FollowerBounds(c *v1beta1.ComponentExtensionSpec) Bounds {
	bounds := Bounds{Min: autoscaler.EffectiveComponentBounds(c).Min}
	if c != nil && c.MaxReplicas > 0 {
		bounds.Max = ptr.To(max(int32(c.MaxReplicas), bounds.Min))
	}
	return bounds
}

// Count is the coordinator's answer for one follower.
type Count struct {
	// Wanted is ceil(anchor x ratio), the count the ratio alone asks for.
	Wanted int64
	// Target is Wanted after the follower's own bounds.
	Target int32
	// Bound names the field that moved Target away from Wanted; empty when
	// neither bound did.
	Bound string
}

// Target computes the follower count using exact milli-unit arithmetic: the
// ratio's answer raised to the follower's minimum and, only when the follower
// declares one, lowered to its maximum.
func Target(anchor int32, milli int64, bounds Bounds) (Count, error) {
	if anchor < 0 || milli <= 0 || milli > 1_000_000 || bounds.Min < 1 || (bounds.Max != nil && *bounds.Max < bounds.Min) {
		return Count{}, fmt.Errorf("invalid scaling bounds or ratio")
	}
	if int64(anchor) > (math.MaxInt64-999)/milli {
		return Count{}, fmt.Errorf("replica count overflows scaling computation")
	}
	wanted := (int64(anchor)*milli + 999) / 1000
	count := Count{Wanted: wanted}
	result := wanted
	if result < int64(bounds.Min) {
		result = int64(bounds.Min)
		count.Bound = boundMinReplicas
	}
	if bounds.Max != nil && result > int64(*bounds.Max) {
		result = int64(*bounds.Max)
		count.Bound = boundMaxReplicas
	}
	if result > math.MaxInt32 {
		return Count{}, fmt.Errorf("replica count overflows scaling computation")
	}
	count.Target = int32(result)
	return count, nil
}
