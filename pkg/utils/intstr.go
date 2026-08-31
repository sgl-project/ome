package utils

import "k8s.io/apimachinery/pkg/util/intstr"

// PtrIntOrStringFromString returns a pointer to an IntOrString carrying
// the supplied percentage string (e.g. "25%"). Convenience for pacing
// defaulters that need an addressable percent literal.
func PtrIntOrStringFromString(s string) *intstr.IntOrString {
	out := intstr.FromString(s)
	return &out
}

// ScaledCountFromIntOrString resolves an IntOrString budget (a pod
// count or a percentage of total) into an absolute, non-negative count.
// It is the single int/percent → count helper shared across the
// coordination, workload-budget, and canary-capacity callers.
//
// Semantics:
//
//   - nil v → 0.
//   - Integer form: the literal value, floored at 0. NOT clamped to
//     total — an integer MaxSurge larger than the replica count is a
//     deliberate over-allocation the former coordination/workload copies
//     returned as-is. Callers that need an upper bound (e.g. canary's
//     new-pod count) compose with ClampInt32.
//   - Percent form: round(total * pct / 100) via
//     intstr.GetScaledValueFromIntOrPercent (ceil when roundUp, floor
//     otherwise), then floored at 0 and CLAMPED to total. The clamp
//     reproduces the former copies' "a 100%+ expression never exceeds
//     total" rule: for pct in [0,100] the ceil already sits at or below
//     total so the clamp is a no-op; for pct > 100 it caps at total,
//     exactly as the old percent→count helpers did by clamping the
//     percent to 100 first.
//   - Malformed string (no '%' suffix, non-numeric): the stdlib parser
//     errors and we return 0 — matching the conservative
//     parse-failure → 0 contract of the former strPercent /
//     parsePercentString helpers. Validated webhook payloads never reach
//     this branch.
func ScaledCountFromIntOrString(v *intstr.IntOrString, total int32, roundUp bool) int32 {
	if v == nil {
		return 0
	}
	if v.Type == intstr.Int {
		n := int32(v.IntValue())
		if n < 0 {
			return 0
		}
		return n
	}
	n, err := intstr.GetScaledValueFromIntOrPercent(v, int(total), roundUp)
	if err != nil || n < 0 {
		return 0
	}
	// Compare at full width: narrowing first lets a value above MaxInt32
	// wrap and slip past the clamp.
	if total >= 0 && n > int(total) {
		return total
	}
	return int32(n)
}

// AvailabilityFloor returns the minimum number of pods (out of total) that must
// remain available to satisfy a disruption budget, in precedence order:
//
//   - minAvailable set → that many must be available (a percent resolves via
//     ceil, then clamped to total);
//   - else maxUnavailable set → total minus that many may be down (floored at 0);
//   - else fallbackMaxUnavailable set → total minus that many (floored at 0);
//   - else total (strict — all must be available).
//
// Percent budgets resolve with ScaledCountFromIntOrString (ceil), the same
// resolver the rollout drain-budget math uses, so an availability check and the
// pacing gate agree byte-for-byte. total <= 0 yields 0.
func AvailabilityFloor(total int32, minAvailable, maxUnavailable, fallbackMaxUnavailable *intstr.IntOrString) int32 {
	if total <= 0 {
		return 0
	}
	if minAvailable != nil {
		f := ScaledCountFromIntOrString(minAvailable, total, true)
		if f > total {
			return total
		}
		return f
	}
	mu := maxUnavailable
	if mu == nil {
		mu = fallbackMaxUnavailable
	}
	if mu != nil {
		if f := total - ScaledCountFromIntOrString(mu, total, true); f > 0 {
			return f
		}
		return 0
	}
	return total
}
