// Shared semantic validation for rolling-update budget knobs
// (maxSurge / maxUnavailable *intstr.IntOrString).
//
// The CRD schema's XIntOrString marker only gates the structural shape
// (must parse as integer or string). These helpers add the semantic
// checks the apiserver can't express — integers must be >= 0, strings
// must be "<n>%" with 0 <= n <= 100 — parameterized by the full field
// path so every caller (per-Component lifecycle, rollout groups) reports
// the exact location of the invalid value. Anything outside those forms
// resolves to zero in the runtime parser (ScaledCountFromIntOrString),
// silently deleting the budget, so admission rejects it instead.
package validation

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
)

// validateBudgetIntOrString enforces the semantic rules for a
// rolling-update budget value:
//   - integer values >= 0
//   - string values match "<n>%" with 0 <= n <= 100
//
// path is the full field path (e.g.
// "spec.rollout.groups[0].rollingUpdate.maxSurge") reported verbatim in
// every error. Returns nil for nil inputs — nil means "use default",
// and every defaulting layer fills a non-zero budget.
func validateBudgetIntOrString(path string, v *intstr.IntOrString) error {
	if v == nil {
		return nil
	}
	if v.Type == intstr.Int {
		if v.IntValue() < 0 {
			return fmt.Errorf("%s=%d must be >= 0 (%s)",
				path, v.IntValue(), ReasonInvalidRollingUpdateInteger)
		}
		return nil
	}
	// String form — must be "<n>%" with 0 <= n <= 100.
	s := v.StrVal
	if !strings.HasSuffix(s, "%") {
		return fmt.Errorf("%s=%q must be an integer count or a percent string like \"25%%\" (%s)",
			path, s, ReasonInvalidRollingUpdatePercent)
	}
	pctStr := strings.TrimSuffix(s, "%")
	pct, err := strconv.Atoi(pctStr)
	if err != nil {
		return fmt.Errorf("%s=%q has a malformed percent value (%s)",
			path, s, ReasonInvalidRollingUpdatePercent)
	}
	if pct < 0 || pct > 100 {
		return fmt.Errorf("%s=%q percent must be between 0%% and 100%% (%s)",
			path, s, ReasonInvalidRollingUpdatePercent)
	}
	return nil
}

// budgetResolvesToZero reports whether the runtime budget resolver
// (ScaledCountFromIntOrString) turns v into a zero budget: explicit
// zeros ("0%" or integer 0), negative integers (floored to 0), and
// malformed strings (parse error yields 0) all qualify. A positive
// percent is never zero here — its scaled value depends on the replica
// count, which admission cannot know. nil is NOT zero: nil means "use
// default", and every defaulting layer fills a non-zero budget.
func budgetResolvesToZero(v *intstr.IntOrString) bool {
	if v == nil {
		return false
	}
	if v.Type == intstr.Int {
		return v.IntValue() <= 0
	}
	s := v.StrVal
	if !strings.HasSuffix(s, "%") {
		return true
	}
	pct, err := strconv.Atoi(strings.TrimSuffix(s, "%"))
	return err != nil || pct <= 0
}
