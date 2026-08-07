package validation

import (
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// canarySpec builds a spec with a single engine-component canary group carrying
// the given steps (v2 spec.rollout.groups[].canary).
func canarySpec(steps ...v1beta1.RolloutGroupStep) *v1beta1.InferenceServiceSpec {
	mode := constants.OMENative
	return &v1beta1.InferenceServiceSpec{
		DeploymentMode: &mode,
		Engine:         &v1beta1.EngineSpec{},
		Rollout: &v1beta1.RolloutSpec{
			Groups: []v1beta1.RolloutGroup{
				{
					Components: []v1beta1.ComponentType{v1beta1.EngineComponent},
					Canary:     &v1beta1.GroupCanary{Steps: steps},
				},
			},
		},
	}
}

// canarySpecMode builds a canary spec with an engine component at the given
// deployment mode, for the OMENative-gate tests.
func canarySpecMode(mode constants.DeploymentModeType, steps ...v1beta1.RolloutGroupStep) *v1beta1.InferenceServiceSpec {
	s := canarySpec(steps...)
	s.DeploymentMode = &mode
	s.Engine = &v1beta1.EngineSpec{}
	return s
}

func TestValidateCanary_OK(t *testing.T) {
	spec := canarySpec(
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("10%"), Traffic: 10},
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100},
	)
	if err := ValidateCanary(spec); err != nil {
		t.Fatalf("valid canary rejected: %v", err)
	}
}

func TestValidateCanary_Empty(t *testing.T) {
	if err := ValidateCanary(canarySpec()); err == nil {
		t.Fatal("empty steps must be rejected")
	}
}

func TestValidateCanary_FinalWeightNot100(t *testing.T) {
	spec := canarySpec(v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 50})
	if err := ValidateCanary(spec); err == nil {
		t.Fatal("final step traffic must be 100")
	}
}

func TestValidateCanary_TrafficToZeroCapacity(t *testing.T) {
	spec := canarySpec(
		v1beta1.RolloutGroupStep{Capacity: intstr.FromInt(0), Traffic: 10},
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100},
	)
	if err := ValidateCanary(spec); err == nil {
		t.Fatal("traffic>0 with zero Capacity must be rejected")
	}
}

func TestValidateCanary_TrafficToZeroPercentCapacity(t *testing.T) {
	spec := canarySpec(
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("0%"), Traffic: 10},
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100},
	)
	if err := ValidateCanary(spec); err == nil {
		t.Fatal(`traffic>0 with "0%" Capacity must be rejected`)
	}
}

func TestValidateCanary_Unset(t *testing.T) {
	if err := ValidateCanary(&v1beta1.InferenceServiceSpec{}); err != nil {
		t.Fatalf("unset canary must pass: %v", err)
	}
	var nilSpec *v1beta1.InferenceServiceSpec
	if err := ValidateCanary(nilSpec); err != nil {
		t.Fatalf("nil spec must pass: %v", err)
	}
}

func TestValidateCanary_RequiresOMENative(t *testing.T) {
	final := v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100}
	// Non-OMENative component → rejected.
	spec := canarySpecMode(constants.RawDeployment, final)
	err := ValidateCanary(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonCanaryRequiresOMENative) {
		t.Fatalf("canary on RawDeployment must be rejected, got: %v", err)
	}
	// OMENative component → accepted.
	if err := ValidateCanary(canarySpecMode(constants.OMENative, final)); err != nil {
		t.Fatalf("canary on OMENative must pass: %v", err)
	}
	// Unset mode (empty) → rejected (canary needs explicit OMENative).
	if err := ValidateCanary(canarySpecMode("", final)); err == nil {
		t.Fatal("canary with unset deploymentMode must be rejected")
	}
}

func TestValidateCanary_DecreasingWeight(t *testing.T) {
	spec := canarySpec(
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("50%"), Traffic: 50},
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("60%"), Traffic: 30},
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100},
	)
	if err := ValidateCanary(spec); err == nil {
		t.Fatal("decreasing traffic must be rejected")
	}
}

func TestValidateCanary_BarePauseUnderManualOK(t *testing.T) {
	// A bare pause (no duration) is a manual gate: "hold here for the promote
	// annotation." Valid.
	spec := canarySpec(
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("50%"), Traffic: 50, Pause: &v1beta1.RolloutPause{}},
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100},
	)
	if err := ValidateCanary(spec); err != nil {
		t.Fatalf("bare pause (manual) must pass: %v", err)
	}
}

func TestValidateCanary_TimedStepOK(t *testing.T) {
	// A step with pause.duration and no analysis is a timed gate. Valid (the
	// old "duration-under-manual" rejection is gone — the gate is per-step now).
	spec := canarySpec(
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("50%"), Traffic: 50, Pause: &v1beta1.RolloutPause{Duration: &metav1.Duration{Duration: time.Minute}}},
		v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100},
	)
	if err := ValidateCanary(spec); err != nil {
		t.Fatalf("timed step (pause.duration) must pass: %v", err)
	}
}

func goodAnalysis() *v1beta1.RolloutAnalysis {
	return &v1beta1.RolloutAnalysis{
		Interval:     metav1.Duration{Duration: time.Minute},
		FailureLimit: 3,
		Metrics:      []v1beta1.AnalysisMetric{{Name: "err", Query: "rate(x[1m])", Operator: v1beta1.ComparisonLTE, Threshold: "0.05"}},
	}
}

// analysisCanary builds a canary whose single (final) step carries the given
// self-contained analysis.
func analysisCanary(step *v1beta1.RolloutAnalysis) *v1beta1.InferenceServiceSpec {
	return canarySpec(v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100, Analysis: step})
}

func TestValidateCanary_AnalysisOK(t *testing.T) {
	if err := ValidateCanary(analysisCanary(goodAnalysis())); err != nil {
		t.Fatalf("valid analysis step rejected: %v", err)
	}
}

func TestValidateCanary_AnalysisIncompleteRejected(t *testing.T) {
	// interval + failureLimit set but no metrics — the step can never pass, rejected.
	spec := analysisCanary(&v1beta1.RolloutAnalysis{Interval: metav1.Duration{Duration: time.Minute}, FailureLimit: 1})
	if err := ValidateCanary(spec); err == nil || !strings.Contains(err.Error(), ReasonAnalysisInvalid) {
		t.Fatalf("analysis step with no metrics must be rejected, got %v", err)
	}
}

func TestValidateCanary_AnalysisBadThreshold(t *testing.T) {
	a := goodAnalysis()
	a.Metrics[0].Threshold = "notanumber"
	if err := ValidateCanary(analysisCanary(a)); err == nil || !strings.Contains(err.Error(), ReasonAnalysisInvalid) {
		t.Fatalf("non-numeric threshold must be rejected, got %v", err)
	}
}

func TestValidateCanary_MultipleCanaryGroupsRejected(t *testing.T) {
	// The canary engine drives exactly one canary group (GetCanaryGroup returns
	// the first); a second canary group would be executed by neither engine and
	// roll ungated. Until cross-group sequencing lands, admission rejects more
	// than one canary group rather than silently mis-executing the rest.
	final := v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100}
	spec := &v1beta1.InferenceServiceSpec{Rollout: &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent}, Canary: &v1beta1.GroupCanary{Steps: []v1beta1.RolloutGroupStep{final}}},
			{Components: []v1beta1.ComponentType{v1beta1.DecoderComponent}, Canary: &v1beta1.GroupCanary{Steps: []v1beta1.RolloutGroupStep{final}}},
		},
	}}
	err := ValidateCanary(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonMultipleCanaryGroups) {
		t.Fatalf("multiple canary groups: got %v want error with %s", err, ReasonMultipleCanaryGroups)
	}
}

func TestValidateCanary_UndeclaredComponentRejected(t *testing.T) {
	// A canary group listing a Component not declared on the ISVC wedges at
	// runtime: primaryComponent picks it (router > engine > decoder) but it has no
	// spec/IR, so the step machine never advances while the real component is
	// pinned at the step partition. Reject at admission.
	mode := constants.OMENative
	final := v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100}
	spec := &v1beta1.InferenceServiceSpec{
		DeploymentMode: &mode,
		Engine:         &v1beta1.EngineSpec{}, // router is NOT declared
		Rollout: &v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.RouterComponent}, Canary: &v1beta1.GroupCanary{Steps: []v1beta1.RolloutGroupStep{final}}},
		}},
	}
	err := ValidateCanary(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonCanaryInvalid) {
		t.Fatalf("undeclared canary component: got %v want error with %s", err, ReasonCanaryInvalid)
	}
}

func TestValidateCanary_InvalidComponentRejected(t *testing.T) {
	mode := constants.OMENative
	final := v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100}
	spec := &v1beta1.InferenceServiceSpec{
		DeploymentMode: &mode,
		Engine:         &v1beta1.EngineSpec{},
		Rollout: &v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{"frontend"}, Canary: &v1beta1.GroupCanary{Steps: []v1beta1.RolloutGroupStep{final}}},
		}},
	}
	err := ValidateCanary(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonCanaryInvalid) {
		t.Fatalf("invalid canary component name: got %v want error with %s", err, ReasonCanaryInvalid)
	}
}

// TestValidateCanary_EntrypointRequired: a canary group must contain the ISVC's
// external entrypoint (router when present). A router-fronted PD ISVC whose canary
// group is [engine,decoder] would write the stepped traffic onto engine's internal
// Service while the router keeps shifting by pod ratio — the steps never reach real
// traffic — so admission rejects it.
func TestValidateCanary_EntrypointRequired(t *testing.T) {
	mode := constants.OMENative
	final := v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100}
	build := func(groupComps ...v1beta1.ComponentType) *v1beta1.InferenceServiceSpec {
		return &v1beta1.InferenceServiceSpec{
			DeploymentMode: &mode,
			Engine:         &v1beta1.EngineSpec{},
			Decoder:        &v1beta1.DecoderSpec{},
			Router:         &v1beta1.RouterSpec{}, // router-fronted → entrypoint is the router
			Rollout: &v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{
				{Components: groupComps, Canary: &v1beta1.GroupCanary{Steps: []v1beta1.RolloutGroupStep{final}}},
			}},
		}
	}
	if err := ValidateCanary(build(v1beta1.EngineComponent, v1beta1.DecoderComponent)); err == nil || !strings.Contains(err.Error(), ReasonCanaryInvalid) {
		t.Fatalf("canary group missing the router entrypoint must be rejected, got: %v", err)
	}
	if err := ValidateCanary(build(v1beta1.RouterComponent, v1beta1.EngineComponent, v1beta1.DecoderComponent)); err != nil {
		t.Fatalf("canary group including the router entrypoint must pass: %v", err)
	}
}

// TestValidateCanary_MaintainRatioRejected: the canary engine never reads
// MaintainRatio, so setting it on a canary group is a silent no-op (no ratio
// guard during the roll). Admission rejects it rather than mislead the operator.
func TestValidateCanary_MaintainRatioRejected(t *testing.T) {
	mode := constants.OMENative
	final := v1beta1.RolloutGroupStep{Capacity: intstr.FromString("100%"), Traffic: 100}
	spec := &v1beta1.InferenceServiceSpec{
		DeploymentMode: &mode,
		Engine:         &v1beta1.EngineSpec{}, // engine-fronted → entrypoint engine is in-group
		Decoder:        &v1beta1.DecoderSpec{},
		Rollout: &v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{
			{
				Components:    []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				MaintainRatio: &v1beta1.MaintainRatio{Tolerance: 5},
				Canary:        &v1beta1.GroupCanary{Steps: []v1beta1.RolloutGroupStep{final}},
			},
		}},
	}
	if err := ValidateCanary(spec); err == nil || !strings.Contains(err.Error(), ReasonCanaryInvalid) {
		t.Fatalf("maintainRatio on a canary group must be rejected, got: %v", err)
	}
}
