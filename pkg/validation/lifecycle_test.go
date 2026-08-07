package validation

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// TestValidateLifecycle_NilAndEmptyAccepted asserts the validator
// short-circuits cleanly on nil specs and empty Lifecycle blocks.
// Every Component is optional and a Lifecycle-less ISVC is valid.
func TestValidateLifecycle_NilAndEmptyAccepted(t *testing.T) {
	if err := ValidateLifecycle(nil); err != nil {
		t.Fatalf("nil spec: unexpected error %v", err)
	}
	if err := ValidateLifecycle(&v1beta1.InferenceServiceSpec{}); err != nil {
		t.Fatalf("empty spec: unexpected error %v", err)
	}
	if err := ValidateLifecycle(&v1beta1.InferenceServiceSpec{
		Engine: &v1beta1.EngineSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{},
		},
	}); err != nil {
		t.Fatalf("empty engine lifecycle: unexpected error %v", err)
	}
}

// TestValidateLifecycle_RollingUpdate_AcceptsAllValidShapes runs the
// happy-path matrix: nil pointers, integer counts, percent strings
// within 0..100, and zero values on EITHER (but not both) of MaxSurge
// / MaxUnavailable. The MaxSurge=0 AND MaxUnavailable=0 combination is
// rejected by ValidateLifecycle_RollingUpdate_RejectsBothZero below.
func TestValidateLifecycle_RollingUpdate_AcceptsAllValidShapes(t *testing.T) {
	cases := []struct {
		name string
		mu   *intstr.IntOrString
		ms   *intstr.IntOrString
	}{
		{"both-nil", nil, nil},
		{"int-zero-mu-only", intstrPtr(intstr.FromInt(0)), nil},
		{"int-zero-ms-only", nil, intstrPtr(intstr.FromInt(0))},
		{"int-zero-mu-positive-ms", intstrPtr(intstr.FromInt(0)), intstrPtr(intstr.FromInt(1))},
		{"int-positive-mu-zero-ms", intstrPtr(intstr.FromInt(2)), intstrPtr(intstr.FromInt(0))},
		{"int-positive", intstrPtr(intstr.FromInt(2)), intstrPtr(intstr.FromInt(1))},
		{"percent-mid", intstrPtr(intstr.FromString("25%")), intstrPtr(intstr.FromString("50%"))},
		{"percent-zero-mu-only", intstrPtr(intstr.FromString("0%")), nil},
		{"percent-zero-mu-positive-ms", intstrPtr(intstr.FromString("0%")), intstrPtr(intstr.FromString("25%"))},
		{"percent-edge-hundred", intstrPtr(intstr.FromString("100%")), intstrPtr(intstr.FromString("100%"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := lifecycleSpec(tc.mu, tc.ms)
			if err := ValidateLifecycle(spec); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateLifecycle_RollingUpdate_RejectsBothZero pins the new rule
// added when the per-Component layer started actively gating rollouts:
// MaxSurge=0 AND MaxUnavailable=0 on the same Component is unstartable
// — no surge headroom, no drain headroom. Mirrors upstream
// appsv1.Deployment.Strategy.RollingUpdate validation. Both 0% / 0
// integer combinations and the mixed (0%, 0) combination trip the
// rejection.
func TestValidateLifecycle_RollingUpdate_RejectsBothZero(t *testing.T) {
	cases := []struct {
		name string
		mu   *intstr.IntOrString
		ms   *intstr.IntOrString
	}{
		{"int-zero-both", intstrPtr(intstr.FromInt(0)), intstrPtr(intstr.FromInt(0))},
		{"pct-zero-both", intstrPtr(intstr.FromString("0%")), intstrPtr(intstr.FromString("0%"))},
		{"int-zero-mu-pct-zero-ms", intstrPtr(intstr.FromInt(0)), intstrPtr(intstr.FromString("0%"))},
		{"pct-zero-mu-int-zero-ms", intstrPtr(intstr.FromString("0%")), intstrPtr(intstr.FromInt(0))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := lifecycleSpec(tc.mu, tc.ms)
			err := ValidateLifecycle(spec)
			if err == nil {
				t.Fatalf("expected rejection for MaxSurge=0 AND MaxUnavailable=0; got nil")
			}
			if !strings.Contains(err.Error(), ReasonRollingUpdateZeroBudget) {
				t.Fatalf("expected reason %q in error %q", ReasonRollingUpdateZeroBudget, err.Error())
			}
		})
	}
}

// TestValidateLifecycle_RollingUpdate_RejectsNegativeInteger covers
// the int-form rejection path — both MaxUnavailable and MaxSurge must
// reject negative absolute counts with ReasonInvalidRollingUpdateInteger.
func TestValidateLifecycle_RollingUpdate_RejectsNegativeInteger(t *testing.T) {
	negative := intstr.FromInt(-1)
	cases := []struct {
		name string
		mu   *intstr.IntOrString
		ms   *intstr.IntOrString
	}{
		{"negative-mu", &negative, nil},
		{"negative-ms", nil, &negative},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := lifecycleSpec(tc.mu, tc.ms)
			err := ValidateLifecycle(spec)
			if err == nil {
				t.Fatalf("expected error for negative integer; got nil")
			}
			if !strings.Contains(err.Error(), ReasonInvalidRollingUpdateInteger) {
				t.Fatalf("expected reason %q in error %q", ReasonInvalidRollingUpdateInteger, err.Error())
			}
		})
	}
}

// TestValidateLifecycle_RollingUpdate_RejectsBadPercentString covers
// the string-form rejection path — non-percent strings, out-of-range
// percents, and malformed integers in the percent slot must all fail
// with ReasonInvalidRollingUpdatePercent.
func TestValidateLifecycle_RollingUpdate_RejectsBadPercentString(t *testing.T) {
	cases := []struct {
		name string
		val  intstr.IntOrString
	}{
		{"no-percent-suffix", intstr.FromString("abc")},
		{"plain-number-string", intstr.FromString("25")},
		{"negative-percent", intstr.FromString("-25%")},
		{"over-hundred-percent", intstr.FromString("150%")},
		{"non-numeric-percent", intstr.FromString("foo%")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := tc.val
			spec := lifecycleSpec(&v, nil)
			err := ValidateLifecycle(spec)
			if err == nil {
				t.Fatalf("expected error for %+v; got nil", v)
			}
			if !strings.Contains(err.Error(), ReasonInvalidRollingUpdatePercent) {
				t.Fatalf("expected reason %q in error %q", ReasonInvalidRollingUpdatePercent, err.Error())
			}
		})
	}
}

// TestValidateLifecycle_RollingUpdate_AppliesAcrossAllComponents
// confirms the validator walks Engine, Decoder, and Router. A bad
// MaxSurge on any Component triggers rejection — guards against the
// validator forgetting to recurse into a new Component.
func TestValidateLifecycle_RollingUpdate_AppliesAcrossAllComponents(t *testing.T) {
	bad := intstr.FromInt(-3)
	cases := []struct {
		name string
		spec *v1beta1.InferenceServiceSpec
	}{
		{
			name: "engine",
			spec: &v1beta1.InferenceServiceSpec{
				Engine: &v1beta1.EngineSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
						Lifecycle: lifecycleWithRolling(&bad, nil),
					},
				},
			},
		},
		{
			name: "decoder",
			spec: &v1beta1.InferenceServiceSpec{
				Decoder: &v1beta1.DecoderSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
						Lifecycle: lifecycleWithRolling(nil, &bad),
					},
				},
			},
		},
		{
			name: "router",
			spec: &v1beta1.InferenceServiceSpec{
				Router: &v1beta1.RouterSpec{
					ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
						Lifecycle: lifecycleWithRolling(&bad, nil),
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateLifecycle(tc.spec); err == nil {
				t.Fatalf("expected error for bad %s rollingUpdate", tc.name)
			}
		})
	}
}

// --- helpers ------------------------------------------------------------

func intstrPtr(v intstr.IntOrString) *intstr.IntOrString { return &v }

// lifecycleSpec returns an InferenceServiceSpec whose Engine carries
// the supplied MaxUnavailable / MaxSurge on its rollingUpdate block.
// Most tests only exercise a single Component; the multi-Component
// matrix uses the dedicated test above.
func lifecycleSpec(mu, ms *intstr.IntOrString) *v1beta1.InferenceServiceSpec {
	return &v1beta1.InferenceServiceSpec{
		Engine: &v1beta1.EngineSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{
				Lifecycle: lifecycleWithRolling(mu, ms),
			},
		},
	}
}

func lifecycleWithRolling(mu, ms *intstr.IntOrString) *v1beta1.LifecycleSpec {
	return &v1beta1.LifecycleSpec{
		UpdateStrategy: &v1beta1.UpdateStrategy{
			Type: v1beta1.UpdateStrategySurgeThenDrain,
			RollingUpdate: &v1beta1.RollingUpdate{
				MaxUnavailable: mu,
				MaxSurge:       ms,
			},
		},
	}
}
