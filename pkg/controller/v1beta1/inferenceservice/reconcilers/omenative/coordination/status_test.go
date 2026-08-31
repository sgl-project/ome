package coordination

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestBuildGroupStatus_PopulatesCoreFields(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	maxSurge := intstr.FromString("25%")
	g := ResolvedGroup{
		Name:       "0",
		Components: []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent},
		Policy:     v1beta1.CoordinationPolicyBlueGreen,
		Pacing:     v1beta1.CoordinationPacing{Type: v1beta1.CoordinationPacingPerComponent, MaxSurge: &maxSurge},
	}
	tr := GroupTransition{
		Phase:          v1beta1.CoordinationPhaseSurging,
		CompositePhase: "Surging",
		Message:        "creating new-revision pods",
	}
	out := BuildGroupStatus(g, tr, nil, now)
	if out.Name != "0" {
		t.Errorf("Name: got %q want 0", out.Name)
	}
	if out.Phase != v1beta1.CoordinationPhaseSurging {
		t.Errorf("Phase: got %q want Surging", out.Phase)
	}
	if out.Policy != v1beta1.CoordinationPolicyBlueGreen {
		t.Errorf("Policy: got %q want BlueGreen", out.Policy)
	}
	if out.LastTransitionTime == nil || !out.LastTransitionTime.Time.Equal(now) {
		t.Errorf("LastTransitionTime: got %+v want %v", out.LastTransitionTime, now)
	}
	if out.ObservedSurge == nil || out.ObservedSurge.StrVal != "25%" {
		t.Errorf("ObservedSurge: got %+v want 25%%", out.ObservedSurge)
	}
}

func TestBuildGroupStatus_AttachesRatioForRatioBalanced(t *testing.T) {
	g := ResolvedGroup{
		Name:   "0",
		Policy: v1beta1.CoordinationPolicyBlueGreen,
		Pacing: v1beta1.CoordinationPacing{Type: v1beta1.CoordinationPacingRatioBalanced},
	}
	r := &RatioState{
		Original: map[v1beta1.ComponentType]int32{v1beta1.EngineComponent: 40, v1beta1.DecoderComponent: 20},
	}
	out := BuildGroupStatus(g, GroupTransition{}, r, time.Now())
	if out.ObservedRatio == nil {
		t.Fatalf("ObservedRatio should be set for RatioBalanced")
	}
	if out.ObservedRatio.Original[v1beta1.EngineComponent] != 40 {
		t.Errorf("ratio engine: got %d want 40", out.ObservedRatio.Original[v1beta1.EngineComponent])
	}
}

func TestBuildGroupStatus_OmitsRatioForPerComponent(t *testing.T) {
	g := ResolvedGroup{
		Name:   "0",
		Policy: v1beta1.CoordinationPolicyBlueGreen,
		Pacing: v1beta1.CoordinationPacing{Type: v1beta1.CoordinationPacingPerComponent},
	}
	r := &RatioState{Original: map[v1beta1.ComponentType]int32{v1beta1.EngineComponent: 10}}
	out := BuildGroupStatus(g, GroupTransition{}, r, time.Now())
	if out.ObservedRatio != nil {
		t.Errorf("PerComponent should not attach ratio: got %+v", out.ObservedRatio)
	}
}

func TestMergeGroupStatus_PreservesLastTransitionTimeOnSamePhase(t *testing.T) {
	older := time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	prev := &v1beta1.RolloutCoordinationStatus{
		Groups: []v1beta1.RolloutCoordinationGroupStatus{
			{
				Name:               "0",
				Phase:              v1beta1.CoordinationPhaseSurging,
				LastTransitionTime: ptrMetav1Time(older),
			},
		},
	}
	next := v1beta1.RolloutCoordinationGroupStatus{
		Name:               "0",
		Phase:              v1beta1.CoordinationPhaseSurging,
		LastTransitionTime: ptrMetav1Time(newer),
	}
	merged := MergeGroupStatus(prev, next)
	if !merged.Groups[0].LastTransitionTime.Time.Equal(older) {
		t.Errorf("same phase: LastTransitionTime should be preserved, got %v want %v",
			merged.Groups[0].LastTransitionTime.Time, older)
	}
}

func TestMergeGroupStatus_NewPhaseUpdatesTransitionTime(t *testing.T) {
	older := time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	prev := &v1beta1.RolloutCoordinationStatus{
		Groups: []v1beta1.RolloutCoordinationGroupStatus{
			{
				Name:               "0",
				Phase:              v1beta1.CoordinationPhaseSurging,
				LastTransitionTime: ptrMetav1Time(older),
			},
		},
	}
	next := v1beta1.RolloutCoordinationGroupStatus{
		Name:               "0",
		Phase:              v1beta1.CoordinationPhaseShifting,
		LastTransitionTime: ptrMetav1Time(newer),
	}
	merged := MergeGroupStatus(prev, next)
	if !merged.Groups[0].LastTransitionTime.Time.Equal(newer) {
		t.Errorf("new phase: LastTransitionTime should advance, got %v want %v",
			merged.Groups[0].LastTransitionTime.Time, newer)
	}
}

func TestMergeGroupStatus_AppendsNewGroup(t *testing.T) {
	prev := &v1beta1.RolloutCoordinationStatus{
		Groups: []v1beta1.RolloutCoordinationGroupStatus{
			{Name: "0", Phase: v1beta1.CoordinationPhaseIdle},
		},
	}
	next := v1beta1.RolloutCoordinationGroupStatus{
		Name:  "1",
		Phase: v1beta1.CoordinationPhaseSurging,
	}
	merged := MergeGroupStatus(prev, next)
	if len(merged.Groups) != 2 {
		t.Fatalf("groups: got %d want 2", len(merged.Groups))
	}
}

func TestComputeCoordinationReady_AllIdle(t *testing.T) {
	cond := ComputeCoordinationReady([]v1beta1.RolloutCoordinationGroupStatus{
		{Name: "0", Phase: v1beta1.CoordinationPhaseIdle},
	})
	if cond == nil || cond.Status != corev1.ConditionTrue || cond.Reason != "Idle" {
		t.Errorf("all idle: got %+v want True/Idle", cond)
	}
}

func TestComputeCoordinationReady_Staged(t *testing.T) {
	// A group parked at a static partition (Staged) is a terminal resting
	// state — ready, distinct Reason from Idle.
	cond := ComputeCoordinationReady([]v1beta1.RolloutCoordinationGroupStatus{
		{Name: "0", Phase: v1beta1.CoordinationPhaseStaged},
	})
	if cond == nil || cond.Status != corev1.ConditionTrue || cond.Reason != "Staged" {
		t.Errorf("staged: got %+v want True/Staged", cond)
	}
	// Staged alongside Idle is still ready.
	cond = ComputeCoordinationReady([]v1beta1.RolloutCoordinationGroupStatus{
		{Name: "0", Phase: v1beta1.CoordinationPhaseIdle},
		{Name: "1", Phase: v1beta1.CoordinationPhaseStaged},
	})
	if cond == nil || cond.Status != corev1.ConditionTrue {
		t.Errorf("staged+idle: got %+v want True", cond)
	}
	// Staged does not mask a real in-flight group.
	cond = ComputeCoordinationReady([]v1beta1.RolloutCoordinationGroupStatus{
		{Name: "0", Phase: v1beta1.CoordinationPhaseStaged},
		{Name: "1", Phase: v1beta1.CoordinationPhaseSurging},
	})
	if cond == nil || cond.Status != corev1.ConditionFalse || cond.Reason != "InProgress" {
		t.Errorf("staged+surging: got %+v want False/InProgress", cond)
	}
}

func TestComputeCoordinationReady_AnyFailed(t *testing.T) {
	cond := ComputeCoordinationReady([]v1beta1.RolloutCoordinationGroupStatus{
		{Name: "0", Phase: v1beta1.CoordinationPhaseIdle},
		{Name: "1", Phase: v1beta1.CoordinationPhaseFailed},
	})
	if cond == nil || cond.Status != corev1.ConditionFalse || cond.Reason != "Failed" {
		t.Errorf("any failed: got %+v want False/Failed", cond)
	}
}

func TestComputeCoordinationReady_InProgress(t *testing.T) {
	cond := ComputeCoordinationReady([]v1beta1.RolloutCoordinationGroupStatus{
		{Name: "0", Phase: v1beta1.CoordinationPhaseSurging},
	})
	if cond == nil || cond.Status != corev1.ConditionFalse || cond.Reason != "InProgress" {
		t.Errorf("in-progress: got %+v want False/InProgress", cond)
	}
}

func TestComputeCoordinationReady_NoGroupsReturnsNil(t *testing.T) {
	if cond := ComputeCoordinationReady(nil); cond != nil {
		t.Errorf("no groups: got %+v want nil", cond)
	}
}

func TestMaxSurgeBudget_DefaultsTo25Percent(t *testing.T) {
	pacing := v1beta1.CoordinationPacing{}
	if got := MaxSurgeBudget(pacing, 40); got != 10 {
		t.Errorf("default 25%% of 40: got %d want 10", got)
	}
}

func TestMaxSurgeBudget_PercentStringRoundsUp(t *testing.T) {
	val := intstr.FromString("33%")
	pacing := v1beta1.CoordinationPacing{MaxSurge: &val}
	if got := MaxSurgeBudget(pacing, 10); got != 4 {
		t.Errorf("33%% of 10 rounds up to 4: got %d", got)
	}
}

func TestMaxSurgeBudget_IntPassThrough(t *testing.T) {
	val := intstr.FromInt(7)
	pacing := v1beta1.CoordinationPacing{MaxSurge: &val}
	if got := MaxSurgeBudget(pacing, 100); got != 7 {
		t.Errorf("int pacing pass-through: got %d want 7", got)
	}
}

func TestMaxSurgeBudget_NegativeIntClampedToZero(t *testing.T) {
	val := intstr.FromInt(-5)
	pacing := v1beta1.CoordinationPacing{MaxSurge: &val}
	if got := MaxSurgeBudget(pacing, 10); got != 0 {
		t.Errorf("negative int: got %d want 0", got)
	}
}

func TestMaxUnavailableBudget_DefaultsToZero(t *testing.T) {
	pacing := v1beta1.CoordinationPacing{}
	if got := MaxUnavailableBudget(pacing, 10); got != 0 {
		t.Errorf("default: got %d want 0", got)
	}
}

func TestMaxUnavailableBudget_PercentString(t *testing.T) {
	val := intstr.FromString("50%")
	pacing := v1beta1.CoordinationPacing{MaxUnavailable: &val}
	if got := MaxUnavailableBudget(pacing, 10); got != 5 {
		t.Errorf("50%% of 10: got %d want 5", got)
	}
}

func TestSetRolloutCoordinationReady_NilISVCNoop(t *testing.T) {
	// Just verify it doesn't panic.
	SetRolloutCoordinationReady(nil, []v1beta1.RolloutCoordinationGroupStatus{{Phase: v1beta1.CoordinationPhaseIdle}}, time.Now())
}

func TestSetRolloutCoordinationReady_NoGroupsNoop(t *testing.T) {
	isvc := &v1beta1.InferenceService{}
	SetRolloutCoordinationReady(isvc, nil, time.Now())
	if len(isvc.Status.Conditions) != 0 {
		t.Errorf("no groups should not write a condition: got %+v", isvc.Status.Conditions)
	}
}

// ptrMetav1Time returns *metav1.Time for a time.Time literal — avoids
// the `cannot take address of metav1.NewTime(...)` compile error when
// constructing fixtures inline.
func ptrMetav1Time(t time.Time) *metav1.Time {
	out := metav1.NewTime(t)
	return &out
}
