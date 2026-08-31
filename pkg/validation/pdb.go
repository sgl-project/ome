package validation

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
)

// ValidatePodDisruptionBudget validates the mutually exclusive availability
// budgets accepted by a Kubernetes PodDisruptionBudget.
func ValidatePodDisruptionBudget(path string, minAvailable, maxUnavailable *intstr.IntOrString) error {
	if minAvailable != nil && maxUnavailable != nil {
		return fmt.Errorf("%s.minAvailable and %s.maxUnavailable cannot both be set", path, path)
	}
	if err := validatePodDisruptionBudgetValue(path+".minAvailable", minAvailable); err != nil {
		return err
	}
	return validatePodDisruptionBudgetValue(path+".maxUnavailable", maxUnavailable)
}

func validatePodDisruptionBudgetValue(path string, value *intstr.IntOrString) error {
	if value == nil {
		return nil
	}

	switch value.Type {
	case intstr.Int:
		if value.IntVal < 0 {
			return fmt.Errorf("%s must be a non-negative integer or a percentage from 0%% to 100%%", path)
		}
		return nil
	case intstr.String:
		percentage := value.StrVal
		if !strings.HasSuffix(percentage, "%") {
			return fmt.Errorf("%s must be a non-negative integer or a percentage from 0%% to 100%%", path)
		}
		number := strings.TrimSuffix(percentage, "%")
		if number == "" {
			return fmt.Errorf("%s must be a non-negative integer or a percentage from 0%% to 100%%", path)
		}
		for _, digit := range number {
			if digit < '0' || digit > '9' {
				return fmt.Errorf("%s must be a non-negative integer or a percentage from 0%% to 100%%", path)
			}
		}
		parsed, err := strconv.Atoi(number)
		if err != nil || parsed > 100 {
			return fmt.Errorf("%s must be a non-negative integer or a percentage from 0%% to 100%%", path)
		}
		return nil
	default:
		return fmt.Errorf("%s must be a non-negative integer or a percentage from 0%% to 100%%", path)
	}
}
