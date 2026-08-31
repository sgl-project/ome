package pdb

import (
	"fmt"
	"math"

	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/controllerconfig"
	omevalidation "sigs.k8s.io/ome/pkg/validation"
)

// ResolveBudget selects the component policy when present, otherwise fallback.
// The two sources are never merged and no default is injected.
func ResolveBudget(component *v1beta1.ComponentExtensionSpec, fallback *controllerconfig.PodDisruptionBudgetPolicy) (*Budget, error) {
	var selected *Budget
	if component != nil && (component.MinAvailable != nil || component.MaxUnavailable != nil) {
		selected = &Budget{
			MinAvailable:   copyIntOrString(component.MinAvailable),
			MaxUnavailable: copyIntOrString(component.MaxUnavailable),
		}
	} else if fallback != nil {
		selected = &Budget{
			MinAvailable:   copyIntOrString(fallback.MinAvailable),
			MaxUnavailable: copyIntOrString(fallback.MaxUnavailable),
		}
	} else {
		return nil, nil
	}

	if err := validateBudget(selected); err != nil {
		return nil, err
	}
	return selected, nil
}

// DesiredPodCount returns the committed replica count multiplied by the
// per-instance sum of Runner sizes, rejecting invalid or overflowing input.
func DesiredPodCount(ir *v1beta1.InferenceReplica) (int32, error) {
	if ir == nil {
		return 0, fmt.Errorf("InferenceReplica is required")
	}
	if ir.Spec.Replicas == nil {
		return 0, fmt.Errorf("InferenceReplica spec.replicas is required")
	}
	if *ir.Spec.Replicas < 0 {
		return 0, fmt.Errorf("InferenceReplica spec.replicas must be non-negative")
	}
	if len(ir.Spec.Runners) == 0 {
		return 0, fmt.Errorf("InferenceReplica spec.runners must not be empty")
	}

	var podsPerReplica int64
	for i := range ir.Spec.Runners {
		size := ir.Spec.Runners[i].Size
		if size <= 0 {
			return 0, fmt.Errorf("InferenceReplica spec.runners[%d].size must be positive", i)
		}
		if podsPerReplica > math.MaxInt32-int64(size) {
			return 0, fmt.Errorf("InferenceReplica runner size sum exceeds int32")
		}
		podsPerReplica += int64(size)
	}

	replicas := int64(*ir.Spec.Replicas)
	if replicas > 0 && podsPerReplica > math.MaxInt32/replicas {
		return 0, fmt.Errorf("InferenceReplica desired pod count exceeds int32")
	}
	return int32(replicas * podsPerReplica), nil
}

// NormalizeOMENativeBudget converts either budget form into an absolute
// integer minAvailable based on the desired pod count.
func NormalizeOMENativeBudget(budget *Budget, desiredPods int32) (*Budget, error) {
	if budget == nil {
		return nil, nil
	}
	if desiredPods < 0 {
		return nil, fmt.Errorf("desired pod count must be non-negative")
	}
	if err := validateBudget(budget); err != nil {
		return nil, err
	}

	if budget.MinAvailable != nil {
		minimum, err := intstr.GetScaledValueFromIntOrPercent(budget.MinAvailable, int(desiredPods), true)
		if err != nil {
			return nil, fmt.Errorf("resolve minAvailable: %w", err)
		}
		if minimum < 0 || int64(minimum) > math.MaxInt32 {
			return nil, fmt.Errorf("resolved minAvailable is outside int32")
		}
		value := intstr.FromInt32(int32(minimum))
		return &Budget{MinAvailable: &value}, nil
	}

	unavailable, err := intstr.GetScaledValueFromIntOrPercent(budget.MaxUnavailable, int(desiredPods), true)
	if err != nil {
		return nil, fmt.Errorf("resolve maxUnavailable: %w", err)
	}
	minimum := int64(desiredPods) - int64(unavailable)
	if minimum < 0 {
		minimum = 0
	}
	value := intstr.FromInt32(int32(minimum))
	return &Budget{MinAvailable: &value}, nil
}

func validateBudget(budget *Budget) error {
	if budget == nil {
		return nil
	}
	if (budget.MinAvailable == nil) == (budget.MaxUnavailable == nil) {
		return fmt.Errorf("exactly one of minAvailable or maxUnavailable must be set")
	}
	return omevalidation.ValidatePodDisruptionBudget(
		"PodDisruptionBudget budget",
		budget.MinAvailable,
		budget.MaxUnavailable,
	)
}
