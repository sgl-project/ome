package canary

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/rollout"
)

// releaseFixture is a service whose engine carries everything a canary
// projects: a record that rolled back, the RolledBack phase, a split and the
// legacy alias. spec and run shape the plan the pass reads. The split stays
// through a release: the coordination pass writes it from the pods.
func releaseFixture(spec *v1beta1.RolloutSpec, run *v1beta1.RolloutRun) *v1beta1.InferenceService {
	record := &v1beta1.CanaryStatus{CanaryRevisionHash: "bbbbbbbb", StableRevisionHash: "aaaaaaaa", RolledBackRevisionHash: "bbbbbbbb"}
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "ns"},
		Spec:       v1beta1.InferenceServiceSpec{Engine: &v1beta1.EngineSpec{}, Rollout: spec},
	}
	isvc.Status.Components = map[v1beta1.ComponentType]v1beta1.ComponentStatusSpec{
		v1beta1.EngineComponent: {
			RolloutPhase: v1beta1.RolloutPhaseRolledBack,
			Canary:       record.DeepCopy(),
			Traffic:      []v1beta1.ComponentTrafficTarget{{RevisionName: "svc-engine-rev-aaaaaaaa", Percent: 100}},
		},
	}
	isvc.Status.Canary = record.DeepCopy()
	if run != nil {
		isvc.Status.Rollout = &v1beta1.RolloutStatus{ActiveRun: run}
	}
	return isvc
}

func engineCanaryGroup() v1beta1.RolloutGroup {
	return v1beta1.RolloutGroup{
		Components: []v1beta1.ComponentType{v1beta1.EngineComponent},
		Canary: &v1beta1.GroupCanary{Steps: []v1beta1.RolloutGroupStep{
			{Capacity: intstr.FromString("50%"), Traffic: 50, Pause: &v1beta1.RolloutPause{}},
			{Capacity: intstr.FromString("100%"), Traffic: 100},
		}},
	}
}

func requireEngineKept(t *testing.T, isvc *v1beta1.InferenceService, released []v1beta1.ComponentType) {
	t.Helper()
	cs := isvc.Status.Components[v1beta1.EngineComponent]
	if len(released) != 0 || cs.Canary == nil || cs.RolloutPhase != v1beta1.RolloutPhaseRolledBack || len(cs.Traffic) != 1 || isvc.Status.Canary == nil {
		t.Fatalf("released %v; engine status %+v alias %v, want everything kept", released, cs, isvc.Status.Canary)
	}
}

// A unit that no group of the effective plan names any more keeps none of
// the status its canary projected: the record, the phase and the legacy
// alias go, so the Component reads like one no rollout ever governed; the
// split stays for the coordination pass to rewrite from the pods.
func TestReleaseUndeclaredUnits_DropsTheStatusOfAUnitNoGroupNames(t *testing.T) {
	for name, spec := range map[string]*v1beta1.RolloutSpec{
		"spec.rollout removed":        nil,
		"spec.rollout.groups emptied": {Groups: nil},
		"the canary group replaced by one over another Component": {Groups: []v1beta1.RolloutGroup{{
			Components: []v1beta1.ComponentType{v1beta1.RouterComponent}, BlueGreen: &v1beta1.GroupBlueGreen{},
		}}},
	} {
		t.Run(name, func(t *testing.T) {
			isvc := releaseFixture(spec, nil)
			released := ReleaseUndeclaredUnits(isvc, rollout.Policies{})
			if len(released) != 1 || released[0] != v1beta1.EngineComponent {
				t.Fatalf("released %v, want the engine", released)
			}
			cs := isvc.Status.Components[v1beta1.EngineComponent]
			if cs.Canary != nil || cs.RolloutPhase != "" || len(cs.Traffic) != 1 || isvc.Status.Canary != nil {
				t.Fatalf("engine status %+v alias %v, want no record, no phase, no alias, the split kept", cs, isvc.Status.Canary)
			}
			if again := ReleaseUndeclaredUnits(isvc, rollout.Policies{}); len(again) != 0 {
				t.Fatalf("a second pass released %v, want nothing", again)
			}
		})
	}
}

// A unit a canary still governs keeps its status: an effective canary group,
// a declared canary-kind group whose plan does not resolve, and a run whose
// recorded plan names the group after the spec lost it.
func TestReleaseUndeclaredUnits_KeepsAGovernedUnit(t *testing.T) {
	canaryGroup := engineCanaryGroup()
	t.Run("an effective canary group", func(t *testing.T) {
		isvc := releaseFixture(&v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{canaryGroup}}, nil)
		requireEngineKept(t, isvc, ReleaseUndeclaredUnits(isvc, rollout.Policies{}))
	})
	t.Run("a declared canary whose policy does not resolve", func(t *testing.T) {
		isvc := releaseFixture(&v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{{
			Components: []v1beta1.ComponentType{v1beta1.EngineComponent},
			PolicyRef:  &v1beta1.RolloutPolicyRef{Name: "missing", Progression: v1beta1.RolloutProgressionCanary},
		}}}, nil)
		requireEngineKept(t, isvc, ReleaseUndeclaredUnits(isvc, rollout.Policies{Enabled: true}))
	})
	t.Run("a run whose recorded plan names the group", func(t *testing.T) {
		run := &v1beta1.RolloutRun{
			RunID:           "svc-run",
			TargetRevisions: []v1beta1.RolloutRunTarget{{Component: v1beta1.EngineComponent, Revision: "bbbbbbbb", StableRevision: "aaaaaaaa"}},
			Plan:            v1beta1.RolloutRunPlan{Groups: []v1beta1.RolloutRunGroup{{Source: v1beta1.RolloutPlanSourceInline, Group: canaryGroup}}},
		}
		isvc := releaseFixture(nil, run)
		requireEngineKept(t, isvc, ReleaseUndeclaredUnits(isvc, rollout.Policies{}))
	})
}

// A Component a coordination group owns keeps its status: that pass writes
// its phase and split.
func TestReleaseUndeclaredUnits_KeepsAComponentACoordinationGroupOwns(t *testing.T) {
	isvc := releaseFixture(&v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{{
		Components: []v1beta1.ComponentType{v1beta1.EngineComponent}, RollingUpdate: &v1beta1.GroupRollingUpdate{},
	}}}, nil)
	requireEngineKept(t, isvc, ReleaseUndeclaredUnits(isvc, rollout.Policies{}))
}

// The legacy alias follows the records left: releasing one unit while
// another keeps its record moves the alias onto that record.
func TestReleaseUndeclaredUnits_AliasFollowsTheRecordsLeft(t *testing.T) {
	routerGroup := v1beta1.RolloutGroup{Components: []v1beta1.ComponentType{v1beta1.RouterComponent}, Canary: engineCanaryGroup().Canary}
	isvc := releaseFixture(&v1beta1.RolloutSpec{Groups: []v1beta1.RolloutGroup{routerGroup}}, nil)
	isvc.Spec.Router = &v1beta1.RouterSpec{}
	routerRecord := &v1beta1.CanaryStatus{CanaryRevisionHash: "dddddddd", StableRevisionHash: "cccccccc", CurrentStep: 1}
	isvc.Status.Components[v1beta1.RouterComponent] = v1beta1.ComponentStatusSpec{RolloutPhase: v1beta1.RolloutPhasePaused, Canary: routerRecord}
	released := ReleaseUndeclaredUnits(isvc, rollout.Policies{})
	if len(released) != 1 || released[0] != v1beta1.EngineComponent {
		t.Fatalf("released %v, want the engine alone", released)
	}
	if isvc.Status.Components[v1beta1.RouterComponent].Canary == nil || isvc.Status.Canary == nil || isvc.Status.Canary.CanaryRevisionHash != "dddddddd" {
		t.Fatalf("router record %v alias %v, want the router kept and the alias on it", isvc.Status.Components[v1beta1.RouterComponent].Canary, isvc.Status.Canary)
	}
}
