package ops

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// The in-place roll over a pod-template label change: a label the template
// adds, changes or drops reaches the live pod the way an annotation does,
// foreign labels are left alone, and the workload's own labels stand.

const tierLabel = "example.com/tier"

// labelRoll is a single-pod Instance, Ready and serving, whose running and
// target revisions record the same image under previous and target: a
// metadata-only diff an in-place step applies without replacing a container.
type labelRoll struct {
	isvc   *v1beta1.InferenceService
	pod    *corev1.Pod
	c      client.Client
	target *appsv1.ControllerRevision
	spec   *corev1.PodSpec
	plan   workload.ComponentPlan
}

func newLabelRoll(t *testing.T, podLabels, podAnnotations map[string]string, previous, target *metav1.ObjectMeta) *labelRoll {
	t.Helper()
	fixture.ResetExpectations(t)
	isvc, ir := fixture.ServiceWithReadyInstance("llama-70b", "prod", 1)
	spec := fixture.PodSpecWithImage("example.com/app:v1")
	pod := fixture.ServingPodWithImage(isvc, 0, 1, "example.com/app:v1")
	for k, v := range podLabels {
		pod.Labels[k] = v
	}
	pod.Annotations = podAnnotations
	c := fixture.FakeClient(t, isvc, ir, pod)
	fixture.SeedRunningRevisionWithMeta(t, c, isvc, workload.ComponentEngine, 0, spec, previous)
	targetCR := fixture.EnsureTargetRevisionWithMeta(t, c, isvc, spec, target)
	markNotReady := false
	plan := fixture.SinglePodPlan(workload.UpdateStrategyInPlaceIfPossible,
		&workload.InPlaceUpdateStrategy{MarkNotReadyDuringLifecycle: &markNotReady})
	return &labelRoll{isvc: isvc, pod: pod, c: c, target: targetCR, spec: spec, plan: plan}
}

// roll drives Update until the Instance is promoted on the target within
// a bounded number of passes and returns the live pod as promoted.
func (r *labelRoll) roll(t *testing.T) *corev1.Pod {
	t.Helper()
	for pass := 1; pass <= 6; pass++ {
		done, err := Update(context.Background(), fixture.Deps(r.c), fixture.ReconcileInput(r.isvc, r.c, workload.ComponentEngine),
			r.plan, r.plan.Instances[0], r.target, r.spec)
		if err != nil {
			t.Fatalf("pass %d: Update: %v", pass, err)
		}
		if !done {
			continue
		}
		final := fixture.StoredInstanceStatuses(r.c, r.isvc, workload.ComponentEngine)[0]
		if final.Phase != v1beta1.OMENativeInstanceReady || final.RunningRevision != r.target.Name || final.Operation != nil {
			t.Fatalf("promoted status: %+v", final)
		}
		return r.livePod(t)
	}
	t.Fatalf("the in-place roll did not promote within 6 passes; pod labels %v", r.livePod(t).Labels)
	return nil
}

func (r *labelRoll) livePod(t *testing.T) *corev1.Pod {
	t.Helper()
	got := &corev1.Pod{}
	if err := r.c.Get(context.Background(), client.ObjectKeyFromObject(r.pod), got); err != nil {
		t.Fatalf("get pod: %v", err)
	}
	return got
}

// TestUpdateInPlace_TemplateLabelAddedIsWrittenToTheLivePod: the target
// template sets a label no revision set before. The roll writes it to the
// pod it patches and the Instance promotes with the pod carrying it.
func TestUpdateInPlace_TemplateLabelAddedIsWrittenToTheLivePod(t *testing.T) {
	r := newLabelRoll(t, nil, nil,
		nil,
		&metav1.ObjectMeta{Labels: map[string]string{tierLabel: "gold"}})
	pod := r.roll(t)
	if got := pod.Labels[tierLabel]; got != "gold" {
		t.Fatalf("live pod label %s = %q after the in-place roll, want %q; labels %v", tierLabel, got, "gold", pod.Labels)
	}
}

// TestUpdateInPlace_TemplateLabelChangeWritesTheNewValue: the target
// template changes the value of a label the previous template set; the
// pod ends with the new value.
func TestUpdateInPlace_TemplateLabelChangeWritesTheNewValue(t *testing.T) {
	r := newLabelRoll(t, map[string]string{tierLabel: "silver"}, nil,
		&metav1.ObjectMeta{Labels: map[string]string{tierLabel: "silver"}},
		&metav1.ObjectMeta{Labels: map[string]string{tierLabel: "gold"}})
	pod := r.roll(t)
	if got := pod.Labels[tierLabel]; got != "gold" {
		t.Fatalf("live pod label %s = %q after the in-place roll, want %q", tierLabel, got, "gold")
	}
}

// TestUpdateInPlace_TemplateLabelDroppedIsRemovedFromTheLivePod: the
// previous template set a label the target template no longer carries;
// the roll removes it from the pod, as a rebuild would leave it off.
func TestUpdateInPlace_TemplateLabelDroppedIsRemovedFromTheLivePod(t *testing.T) {
	r := newLabelRoll(t, map[string]string{tierLabel: "gold"}, nil,
		&metav1.ObjectMeta{Labels: map[string]string{tierLabel: "gold"}},
		nil)
	pod := r.roll(t)
	if got, present := pod.Labels[tierLabel]; present {
		t.Fatalf("live pod still carries label %s=%q after the template dropped it; labels %v", tierLabel, got, pod.Labels)
	}
}

// TestUpdateInPlace_ForeignPodLabelSurvivesTheRoll: a label another
// controller or a person put on the pod, set by no template, is left in
// place while the template's own label is written.
func TestUpdateInPlace_ForeignPodLabelSurvivesTheRoll(t *testing.T) {
	const foreign = "example.com/by-hand"
	r := newLabelRoll(t, map[string]string{foreign: "yes"}, nil,
		nil,
		&metav1.ObjectMeta{Labels: map[string]string{tierLabel: "gold"}})
	pod := r.roll(t)
	if got := pod.Labels[foreign]; got != "yes" {
		t.Fatalf("foreign label %s = %q after the in-place roll, want %q", foreign, got, "yes")
	}
	if got := pod.Labels[tierLabel]; got != "gold" {
		t.Fatalf("live pod label %s = %q after the in-place roll, want %q", tierLabel, got, "gold")
	}
}

// TestUpdateInPlace_AnnotationsAndLabelsRollTogether: a template that
// changes an annotation and a label in one revision brings both to the
// live pod before the Instance promotes.
func TestUpdateInPlace_AnnotationsAndLabelsRollTogether(t *testing.T) {
	r := newLabelRoll(t, map[string]string{tierLabel: "silver"}, map[string]string{"release": "one"},
		&metav1.ObjectMeta{Labels: map[string]string{tierLabel: "silver"}, Annotations: map[string]string{"release": "one"}},
		&metav1.ObjectMeta{Labels: map[string]string{tierLabel: "gold"}, Annotations: map[string]string{"release": "two"}})
	pod := r.roll(t)
	if got := pod.Annotations["release"]; got != "two" {
		t.Fatalf("live pod annotation release = %q after the in-place roll, want %q", got, "two")
	}
	if got := pod.Labels[tierLabel]; got != "gold" {
		t.Fatalf("live pod label %s = %q after the in-place roll, want %q", tierLabel, got, "gold")
	}
}

// TestUpdateInPlace_WorkloadLabelsKeepTheWorkloadValues: a template naming
// labels the workload stamps changes none of them on the live pod (the
// revision hash follows the target); the template's own label still lands.
func TestUpdateInPlace_WorkloadLabelsKeepTheWorkloadValues(t *testing.T) {
	r := newLabelRoll(t, nil, nil,
		&metav1.ObjectMeta{Labels: map[string]string{query.LabelPodOrdinal: "7"}},
		&metav1.ObjectMeta{Labels: map[string]string{
			tierLabel:               "gold",
			query.LabelRevisionHash: "bogus",
			query.LabelInstanceIdx:  "9",
		}})
	before := r.livePod(t).Labels
	pod := r.roll(t)
	if got := pod.Labels[tierLabel]; got != "gold" {
		t.Fatalf("live pod label %s = %q after the in-place roll, want %q", tierLabel, got, "gold")
	}
	if got, want := pod.Labels[query.LabelRevisionHash], query.RevisionOf(r.target).Hash(); got != want {
		t.Fatalf("revision hash label = %q, want the target's %q", got, want)
	}
	for _, key := range []string{query.LabelInstanceIdx, query.LabelPodOrdinal, query.LabelRunner, query.LabelInstanceIncarnation, query.LabelManagedBy} {
		if got, want := pod.Labels[key], before[key]; got != want {
			t.Errorf("workload label %s = %q after the in-place roll, want the rendered value %q", key, got, want)
		}
	}
}
