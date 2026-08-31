package validation

import (
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

func TestValidateCoordination_NilSpecIsOK(t *testing.T) {
	if err := ValidateCoordination(nil); err != nil {
		t.Errorf("nil spec: unexpected error %v", err)
	}
}

func TestValidateCoordination_MissingCoordinationBlockIsOK(t *testing.T) {
	spec := &v1beta1.InferenceServiceSpec{}
	if err := ValidateCoordination(spec); err != nil {
		t.Errorf("missing coordination: unexpected error %v", err)
	}
}

func TestValidateCoordination_EmptyGroupsIsOK(t *testing.T) {
	spec := &v1beta1.InferenceServiceSpec{
		Rollout: &v1beta1.RolloutSpec{Groups: nil},
	}
	if err := ValidateCoordination(spec); err != nil {
		t.Errorf("empty groups: unexpected error %v", err)
	}
}

func TestValidateCoordination_DuplicateComponentRejected(t *testing.T) {
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent}, BlueGreen: &v1beta1.GroupBlueGreen{}},
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}, BlueGreen: &v1beta1.GroupBlueGreen{}},
		},
	}
	err := ValidateCoordination(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonDuplicateComponentInCoordinationGroups) {
		t.Errorf("duplicate component: got %v want error with %s", err, ReasonDuplicateComponentInCoordinationGroups)
	}
}

func TestValidateCoordination_InvalidComponentName(t *testing.T) {
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{"frontend"}, BlueGreen: &v1beta1.GroupBlueGreen{}},
		},
	}
	err := ValidateCoordination(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonInvalidComponentInCoordinationGroup) {
		t.Errorf("invalid component: got %v want %s", err, ReasonInvalidComponentInCoordinationGroup)
	}
}

func TestValidateCoordination_OrphanGroup(t *testing.T) {
	// Spec declares only engine; group references decoder + router.
	spec := omeNativeEngineOnlySpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.DecoderComponent, v1beta1.RouterComponent}, BlueGreen: &v1beta1.GroupBlueGreen{}},
		},
	}
	err := ValidateCoordination(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonOrphanCoordinationGroup) {
		t.Errorf("orphan group: got %v want %s", err, ReasonOrphanCoordinationGroup)
	}
}

func TestValidateCoordination_ComponentMustBeOMENative(t *testing.T) {
	spec := omeNativePDSpec()
	// Strip the OMENative annotation off engine.
	spec.Engine.Annotations = map[string]string{}
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}, BlueGreen: &v1beta1.GroupBlueGreen{}},
		},
	}
	err := ValidateCoordination(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonCoordinationRequiresOMENative) {
		t.Errorf("non-OMENative component: got %v want %s", err, ReasonCoordinationRequiresOMENative)
	}
}

func TestValidateCoordination_InvalidOrderComponent(t *testing.T) {
	// Order references router, which is not one of the group's components → the
	// per-group order-subset rule rejects it (still enforced in v2).
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				BlueGreen:  &v1beta1.GroupBlueGreen{},
				Order:      []v1beta1.ComponentType{v1beta1.DecoderComponent, v1beta1.RouterComponent},
			},
		},
	}
	err := ValidateCoordination(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonInvalidCoordinationOrder) {
		t.Errorf("foreign order entry: got %v want %s", err, ReasonInvalidCoordinationOrder)
	}
}

func TestValidateCoordination_HappyPath_BlueGreen(t *testing.T) {
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}, BlueGreen: &v1beta1.GroupBlueGreen{}},
		},
	}
	if err := ValidateCoordination(spec); err != nil {
		t.Errorf("happy atomic: unexpected error %v", err)
	}
}

// Sequential is spelled in v2 as one single-Component group per Component, in
// list order (decoder first, then engine) — the controller collapses this back
// to Sequential. The validator must accept it.
func TestValidateCoordination_HappyPath_Sequential(t *testing.T) {
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.DecoderComponent}, BlueGreen: &v1beta1.GroupBlueGreen{}},
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent}, BlueGreen: &v1beta1.GroupBlueGreen{}},
		},
	}
	if err := ValidateCoordination(spec); err != nil {
		t.Errorf("happy sequential: unexpected error %v", err)
	}
}

// Positive case: a multi-Component group guarding the cross-Component ratio
// (v1 BlueGreen + RatioBalanced pacing → v2 BlueGreen + MaintainRatio) is
// accepted. The validator must not over-reject.
func TestValidateCoordination_RatioBalancedWithBlueGreenAccepted(t *testing.T) {
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components:    []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				BlueGreen:     &v1beta1.GroupBlueGreen{},
				MaintainRatio: &v1beta1.MaintainRatio{Tolerance: ptrInt32(5)},
			},
		},
	}
	if err := ValidateCoordination(spec); err != nil {
		t.Errorf("BlueGreen+MaintainRatio: unexpected error %v", err)
	}
}

// maxSurge=0 AND maxUnavailable=0 (both explicit zero) is unstartable —
// no surge, no drain — and must be rejected.
func TestValidateCoordination_ZeroBudgetPacingRejected(t *testing.T) {
	zero := intstr.FromInt(0)
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				RollingUpdate: &v1beta1.GroupRollingUpdate{
					MaxSurge:       &zero,
					MaxUnavailable: &zero,
				},
			},
		},
	}
	err := ValidateCoordination(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonZeroBudgetPacingUnstartable) {
		t.Errorf("zero-budget pacing: got %v want error with %s", err, ReasonZeroBudgetPacingUnstartable)
	}
}

// TestValidateCoordination_SoakOnNonCollapsingRejected: a multi-Component
// blueGreen group with a soak does not collapse to the Sequential state machine,
// so the engine would drop the soak. Admission rejects it.
func TestValidateCoordination_SoakOnNonCollapsingRejected(t *testing.T) {
	mode := constants.OMENative
	soak := &metav1.Duration{Duration: 10 * time.Minute}
	spec := &v1beta1.InferenceServiceSpec{
		DeploymentMode: &mode,
		Engine:         &v1beta1.EngineSpec{},
		Decoder:        &v1beta1.DecoderSpec{},
		Router:         &v1beta1.RouterSpec{},
		Rollout: &v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}, Soak: soak},
			{Components: []v1beta1.ComponentType{v1beta1.RouterComponent}},
		}},
	}
	if err := ValidateCoordination(spec); err == nil || !strings.Contains(err.Error(), ReasonSoakNotHonored) {
		t.Fatalf("soak on a non-collapsing group must be rejected, got: %v", err)
	}
}

// TestValidateCoordination_SoakOnSequenceAccepted: a run of single-Component
// blueGreen groups collapses to Sequential, which honors soak, so it passes.
func TestValidateCoordination_SoakOnSequenceAccepted(t *testing.T) {
	mode := constants.OMENative
	soak := &metav1.Duration{Duration: 10 * time.Minute}
	spec := &v1beta1.InferenceServiceSpec{
		DeploymentMode: &mode,
		Engine:         &v1beta1.EngineSpec{},
		Decoder:        &v1beta1.DecoderSpec{},
		Rollout: &v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.DecoderComponent}, Soak: soak},
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent}},
		}},
	}
	if err := ValidateCoordination(spec); err != nil {
		t.Fatalf("soak on a collapsing single-Component sequence must pass: %v", err)
	}
}

// String "0%" values are equivalent to literal 0 and must produce the
// same zero-budget rejection.
func TestValidateCoordination_ZeroBudgetPacingRejectedStringForm(t *testing.T) {
	zeroPct := intstr.FromString("0%")
	zeroInt := intstr.FromInt(0)
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				RollingUpdate: &v1beta1.GroupRollingUpdate{
					MaxSurge:       &zeroPct,
					MaxUnavailable: &zeroInt,
				},
			},
		},
	}
	err := ValidateCoordination(spec)
	if err == nil || !strings.Contains(err.Error(), ReasonZeroBudgetPacingUnstartable) {
		t.Errorf("zero-budget (str/int mix): got %v want error with %s", err, ReasonZeroBudgetPacingUnstartable)
	}
}

// Positive case: pacing with non-zero MaxSurge is accepted even when
// MaxUnavailable=0 — the rollout still has surge headroom.
func TestValidateCoordination_NonZeroSurgePacingAccepted(t *testing.T) {
	surge := intstr.FromInt(2)
	zero := intstr.FromInt(0)
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				RollingUpdate: &v1beta1.GroupRollingUpdate{
					MaxSurge:       &surge,
					MaxUnavailable: &zero,
				},
			},
		},
	}
	if err := ValidateCoordination(spec); err != nil {
		t.Errorf("non-zero surge + zero unavailable: unexpected error %v", err)
	}
}

// Positive case: pacing with non-zero MaxUnavailable is accepted even
// when MaxSurge=0 — the rollout still has drain headroom.
func TestValidateCoordination_NonZeroUnavailablePacingAccepted(t *testing.T) {
	zero := intstr.FromInt(0)
	pct := intstr.FromString("25%")
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				RollingUpdate: &v1beta1.GroupRollingUpdate{
					MaxSurge:       &zero,
					MaxUnavailable: &pct,
				},
			},
		},
	}
	if err := ValidateCoordination(spec); err != nil {
		t.Errorf("zero surge + non-zero unavailable: unexpected error %v", err)
	}
}

// Positive case: nil pacing values (no explicit zero) are accepted —
// the defaulter fills 25% for both MaxSurge and MaxUnavailable. The
// zero-budget check only fires when both are explicitly set to 0.
func TestValidateCoordination_NilPacingAccepted(t *testing.T) {
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components:    []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				RollingUpdate: &v1beta1.GroupRollingUpdate{
					// MaxSurge / MaxUnavailable nil — defaulter fills 25%.
				},
			},
		},
	}
	if err := ValidateCoordination(spec); err != nil {
		t.Errorf("nil pacing values: unexpected error %v", err)
	}
}

// Positive case: omitted pacing knobs entirely is accepted — the
// defaulter fills a complete budget.
func TestValidateCoordination_OmittedPacingAccepted(t *testing.T) {
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				BlueGreen:  &v1beta1.GroupBlueGreen{},
				// No pacing — BlueGreen carries none.
			},
		},
	}
	if err := ValidateCoordination(spec); err != nil {
		t.Errorf("omitted pacing: unexpected error %v", err)
	}
}

// The zero-budget check only fires for rollingUpdate groups. A BlueGreen group
// guarding the ratio (v1 RatioBalanced) has no maxSurge/maxUnavailable budget,
// so the same zero shape is not even expressible — and MaintainRatio is accepted.
func TestValidateCoordination_RatioBalancedWithZeroBudgetNotRejected(t *testing.T) {
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components:    []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				BlueGreen:     &v1beta1.GroupBlueGreen{},
				MaintainRatio: &v1beta1.MaintainRatio{Tolerance: ptrInt32(5)},
			},
		},
	}
	if err := ValidateCoordination(spec); err != nil {
		t.Errorf("BlueGreen + MaintainRatio: unexpected error %v (zero-budget check is rollingUpdate-only)", err)
	}
}

func TestValidateCoordinationUpdate_RollingUpdateBothBumped(t *testing.T) {
	oldSpec := omeNativePDSpecWithImages("v1", "v1")
	newSpec := omeNativePDSpecWithImages("v2", "v2")
	newSpec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}, RollingUpdate: &v1beta1.GroupRollingUpdate{}},
		},
	}
	if err := ValidateCoordinationUpdate(oldSpec, newSpec); err != nil {
		t.Errorf("pairedmixable both-bumped: unexpected error %v", err)
	}
}

func TestValidateCoordinationUpdate_RollingUpdateOnlyOneBumpedRejected(t *testing.T) {
	oldSpec := omeNativePDSpecWithImages("v1", "v1")
	newSpec := omeNativePDSpecWithImages("v2", "v1")
	newSpec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}, RollingUpdate: &v1beta1.GroupRollingUpdate{}},
		},
	}
	err := ValidateCoordinationUpdate(oldSpec, newSpec)
	if err == nil || !strings.Contains(err.Error(), ReasonRollingUpdateLockstepViolation) {
		t.Errorf("rollingupdate lockstep: got %v want %s", err, ReasonRollingUpdateLockstepViolation)
	}
}

func TestValidateCoordinationUpdate_RollingUpdateNoBumpsNoOp(t *testing.T) {
	oldSpec := omeNativePDSpecWithImages("v1", "v1")
	newSpec := omeNativePDSpecWithImages("v1", "v1")
	newSpec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}, RollingUpdate: &v1beta1.GroupRollingUpdate{}},
		},
	}
	if err := ValidateCoordinationUpdate(oldSpec, newSpec); err != nil {
		t.Errorf("no bumps: unexpected error %v", err)
	}
}

func TestCoordinationRatioToleranceWarning_AboveThreshold(t *testing.T) {
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components:    []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				BlueGreen:     &v1beta1.GroupBlueGreen{},
				MaintainRatio: &v1beta1.MaintainRatio{Tolerance: ptrInt32(80)},
			},
		},
	}
	w := CoordinationRatioToleranceWarning(spec)
	if !strings.Contains(w, ReasonRatioToleranceTooHigh) {
		t.Errorf("tolerance warning: got %q want contains %s", w, ReasonRatioToleranceTooHigh)
	}
}

func TestCoordinationRatioToleranceWarning_BelowThresholdEmpty(t *testing.T) {
	spec := omeNativePDSpec()
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{
				Components:    []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
				BlueGreen:     &v1beta1.GroupBlueGreen{},
				MaintainRatio: &v1beta1.MaintainRatio{Tolerance: ptrInt32(5)},
			},
		},
	}
	if w := CoordinationRatioToleranceWarning(spec); w != "" {
		t.Errorf("safe tolerance: got %q want empty", w)
	}
}

// Test fixtures.

func omeNativePDSpec() *v1beta1.InferenceServiceSpec {
	annot := map[string]string{constants.DeploymentMode: string(constants.OMENative)}
	return &v1beta1.InferenceServiceSpec{
		Engine: &v1beta1.EngineSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: annot},
		},
		Decoder: &v1beta1.DecoderSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: annot},
		},
	}
}

func omeNativePDSpecWithImages(engineImage, decoderImage string) *v1beta1.InferenceServiceSpec {
	spec := omeNativePDSpec()
	spec.Engine.Runner = &v1beta1.RunnerSpec{Container: corev1.Container{Image: engineImage}}
	spec.Decoder.Runner = &v1beta1.RunnerSpec{Container: corev1.Container{Image: decoderImage}}
	return spec
}

func omeNativeEngineOnlySpec() *v1beta1.InferenceServiceSpec {
	annot := map[string]string{constants.DeploymentMode: string(constants.OMENative)}
	return &v1beta1.InferenceServiceSpec{
		Engine: &v1beta1.EngineSpec{
			ComponentExtensionSpec: v1beta1.ComponentExtensionSpec{Annotations: annot},
		},
	}
}

func ptrInt32(v int32) *int32 { return &v }
