package validation

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestValidatePodDisruptionBudgetAcceptsValidBudgets(t *testing.T) {
	tests := []struct {
		name           string
		minAvailable   *intstr.IntOrString
		maxUnavailable *intstr.IntOrString
	}{
		{name: "unset"},
		{name: "zero minimum", minAvailable: intOrStringPtr(intstr.FromInt(0))},
		{name: "integer maximum", maxUnavailable: intOrStringPtr(intstr.FromInt(2))},
		{name: "zero percent minimum", minAvailable: intOrStringPtr(intstr.FromString("0%"))},
		{name: "full percent maximum", maxUnavailable: intOrStringPtr(intstr.FromString("100%"))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidatePodDisruptionBudget("config.podDisruptionBudget", tt.minAvailable, tt.maxUnavailable); err != nil {
				t.Fatalf("ValidatePodDisruptionBudget() error = %v", err)
			}
		})
	}
}

func TestValidatePodDisruptionBudgetRejectsInvalidBudgets(t *testing.T) {
	const path = "config.podDisruptionBudget.rawDeployment"

	tests := []struct {
		name           string
		minAvailable   *intstr.IntOrString
		maxUnavailable *intstr.IntOrString
		wantFields     []string
	}{
		{
			name:           "both fields",
			minAvailable:   intOrStringPtr(intstr.FromInt(1)),
			maxUnavailable: intOrStringPtr(intstr.FromString("25%")),
			wantFields:     []string{path + ".minAvailable", path + ".maxUnavailable"},
		},
		{
			name:         "negative minimum integer",
			minAvailable: intOrStringPtr(intstr.FromInt(-1)),
			wantFields:   []string{path + ".minAvailable"},
		},
		{
			name:           "negative maximum integer",
			maxUnavailable: intOrStringPtr(intstr.FromInt(-1)),
			wantFields:     []string{path + ".maxUnavailable"},
		},
		{
			name:         "malformed percentage",
			minAvailable: intOrStringPtr(intstr.FromString("25.5%")),
			wantFields:   []string{path + ".minAvailable"},
		},
		{
			name:           "non-percentage string",
			maxUnavailable: intOrStringPtr(intstr.FromString("1")),
			wantFields:     []string{path + ".maxUnavailable"},
		},
		{
			name:         "negative percentage",
			minAvailable: intOrStringPtr(intstr.FromString("-1%")),
			wantFields:   []string{path + ".minAvailable"},
		},
		{
			name:           "percentage above one hundred",
			maxUnavailable: intOrStringPtr(intstr.FromString("101%")),
			wantFields:     []string{path + ".maxUnavailable"},
		},
		{
			name:           "unknown value type",
			maxUnavailable: intOrStringPtr(intstr.IntOrString{Type: 2}),
			wantFields:     []string{path + ".maxUnavailable"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePodDisruptionBudget(path, tt.minAvailable, tt.maxUnavailable)
			if err == nil {
				t.Fatal("ValidatePodDisruptionBudget() error = nil")
			}
			for _, field := range tt.wantFields {
				if !strings.Contains(err.Error(), field) {
					t.Errorf("ValidatePodDisruptionBudget() error = %q, want field %q", err, field)
				}
			}
		})
	}
}

func intOrStringPtr(value intstr.IntOrString) *intstr.IntOrString {
	return &value
}
