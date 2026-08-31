package validation

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// lockstepGroupSpec attaches a 2-Component rollingUpdate group
// (engine + decoder) to the spec so ValidateCoordinationUpdate runs
// the lockstep check.
func lockstepGroupSpec(spec *v1beta1.InferenceServiceSpec) *v1beta1.InferenceServiceSpec {
	spec.Rollout = &v1beta1.RolloutSpec{
		Groups: []v1beta1.RolloutGroup{
			{Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent}, RollingUpdate: &v1beta1.GroupRollingUpdate{}},
		},
	}
	return spec
}

func wantLockstepViolation(t *testing.T, err error, scenario string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), ReasonRollingUpdateLockstepViolation) {
		t.Errorf("%s: got %v want %s", scenario, err, ReasonRollingUpdateLockstepViolation)
	}
}

// One-sided revision-affecting changes with UNCHANGED image strings must
// trip lockstep — every one of these re-renders the engine pod template
// and rolls the engine while the grouped decoder stays put.
func TestValidateCoordinationUpdate_LockstepOneSidedNonImageChangesRejected(t *testing.T) {
	cases := map[string]func(spec *v1beta1.InferenceServiceSpec){
		"env": func(spec *v1beta1.InferenceServiceSpec) {
			spec.Engine.Runner.Env = []corev1.EnvVar{{Name: "CONFIG_VERSION", Value: "2"}}
		},
		"command": func(spec *v1beta1.InferenceServiceSpec) {
			spec.Engine.Runner.Command = []string{"serve", "--fast"}
		},
		"resources": func(spec *v1beta1.InferenceServiceSpec) {
			spec.Engine.Runner.Resources = corev1.ResourceRequirements{
				Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
			}
		},
		"volume": func(spec *v1beta1.InferenceServiceSpec) {
			spec.Engine.PodSpec.Volumes = []corev1.Volume{{Name: "scratch"}}
		},
		"pod-template label": func(spec *v1beta1.InferenceServiceSpec) {
			spec.Engine.Labels = map[string]string{"tier": "canary"}
		},
		"pod-template annotation": func(spec *v1beta1.InferenceServiceSpec) {
			ann := make(map[string]string, len(spec.Engine.Annotations)+1)
			for k, v := range spec.Engine.Annotations {
				ann[k] = v
			}
			ann["rollout-trigger"] = "1"
			spec.Engine.Annotations = ann
		},
	}
	for name, mutate := range cases {
		oldSpec := omeNativePDSpecWithImages("v1", "v1")
		newSpec := lockstepGroupSpec(omeNativePDSpecWithImages("v1", "v1"))
		mutate(newSpec)
		wantLockstepViolation(t, ValidateCoordinationUpdate(oldSpec, newSpec), name+"-only engine change")
	}
}

// The same revision-affecting change applied to BOTH grouped Components
// satisfies lockstep and is admitted.
func TestValidateCoordinationUpdate_LockstepBothSidesChangedAccepted(t *testing.T) {
	oldSpec := omeNativePDSpecWithImages("v1", "v1")
	newSpec := lockstepGroupSpec(omeNativePDSpecWithImages("v1", "v1"))
	newSpec.Engine.Runner.Env = []corev1.EnvVar{{Name: "CONFIG_VERSION", Value: "2"}}
	newSpec.Decoder.Runner.Env = []corev1.EnvVar{{Name: "CONFIG_VERSION", Value: "2"}}
	if err := ValidateCoordinationUpdate(oldSpec, newSpec); err != nil {
		t.Errorf("env change on both grouped Components: unexpected error %v", err)
	}
}

// Scaling knobs are not revision-affecting: a one-sided MinReplicas bump
// must NOT trip lockstep (it scales the Component without re-rendering
// its pod template).
func TestValidateCoordinationUpdate_LockstepScaleOnlyChangeAccepted(t *testing.T) {
	oldSpec := omeNativePDSpecWithImages("v1", "v1")
	newSpec := lockstepGroupSpec(omeNativePDSpecWithImages("v1", "v1"))
	three := 3
	newSpec.Engine.MinReplicas = &three
	if err := ValidateCoordinationUpdate(oldSpec, newSpec); err != nil {
		t.Errorf("engine-only MinReplicas change: unexpected error %v", err)
	}
}

// Lifecycle (pacing) is not revision-affecting either: a one-sided
// partition change stages the rollout without minting a new revision.
func TestValidateCoordinationUpdate_LockstepLifecycleOnlyChangeAccepted(t *testing.T) {
	oldSpec := omeNativePDSpecWithImages("v1", "v1")
	newSpec := lockstepGroupSpec(omeNativePDSpecWithImages("v1", "v1"))
	partition := int32(1)
	newSpec.Engine.Lifecycle = &v1beta1.LifecycleSpec{
		UpdateStrategy: &v1beta1.UpdateStrategy{
			RollingUpdate: &v1beta1.RollingUpdate{Partition: &partition},
		},
	}
	if err := ValidateCoordinationUpdate(oldSpec, newSpec); err != nil {
		t.Errorf("engine-only lifecycle change: unexpected error %v", err)
	}
}

// A change to a Component OUTSIDE the group never trips the group's
// lockstep — only grouped Components are compared.
func TestValidateCoordinationUpdate_LockstepIgnoresUngroupedComponent(t *testing.T) {
	oldSpec := omeNativePDSpecWithImages("v1", "v1")
	oldSpec.Router = &v1beta1.RouterSpec{}
	newSpec := lockstepGroupSpec(omeNativePDSpecWithImages("v1", "v1"))
	newSpec.Router = &v1beta1.RouterSpec{
		Runner: &v1beta1.RunnerSpec{Container: corev1.Container{Image: "router:v2"}},
	}
	if err := ValidateCoordinationUpdate(oldSpec, newSpec); err != nil {
		t.Errorf("router change outside the group: unexpected error %v", err)
	}
}

// The violation message names the changed and unchanged Components so an
// operator can see which side of the group diverged.
func TestValidateCoordinationUpdate_LockstepViolationNamesComponents(t *testing.T) {
	oldSpec := omeNativePDSpecWithImages("v1", "v1")
	newSpec := lockstepGroupSpec(omeNativePDSpecWithImages("v2", "v1"))
	err := ValidateCoordinationUpdate(oldSpec, newSpec)
	wantLockstepViolation(t, err, "one-sided image bump")
	if err != nil && (!strings.Contains(err.Error(), "engine") || !strings.Contains(err.Error(), "decoder")) {
		t.Errorf("violation should name changed and unchanged Components: %v", err)
	}
}
