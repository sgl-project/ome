package ops

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// renderForSplitTest renders a pod through the same path the create loop
// uses, so the split-risk check sees the injector's real output (an
// injected topologyKey term, a preserved user term, or nothing).
func renderForSplitTest(t *testing.T, ps *corev1.PodSpec, plan workload.ComponentPlan, inst workload.InstancePlan, runner workload.RunnerPlan) *corev1.Pod {
	t.Helper()
	pod, err := testRender(basicISVC(), ps, plan, inst, runner, 0)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return pod
}

// countGangSplitWarnings drains the recorder and returns how many
// GangSplitRisk Warning events it buffered.
func countGangSplitWarnings(rec *record.FakeRecorder) int {
	n := 0
	for drained := false; !drained; {
		select {
		case e := <-rec.Events:
			if strings.Contains(e, string(workload.EventReasonGangSplitRisk)) {
				n++
			}
		default:
			drained = true
		}
	}
	return n
}

// withUserPodAffinity stamps a hand-written required podAffinity term on
// ps — the "operator already co-located the gang themselves" shape.
func withUserPodAffinity(ps *corev1.PodSpec) *corev1.PodSpec {
	ps.Affinity = &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				TopologyKey:   "kubernetes.io/hostname",
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}},
			}},
		},
	}
	return ps
}

func TestMaybeWarnGangSplitRisk(t *testing.T) {
	gangPlan, gangInst, gangRunners := multiPodPlan(3) // TopologyKey unset
	keyedPlan, keyedInst, keyedRunners := multiPodPlanWithTopologyKey(3, "topology.example.com/domain")
	singlePlan, singleInst, singleRunner := singlePodPlan()

	leader := gangRunners[0]
	worker := gangRunners[1]

	cases := []struct {
		name     string
		ps       *corev1.PodSpec
		plan     workload.ComponentPlan
		inst     workload.InstancePlan
		runner   workload.RunnerPlan
		wantWarn bool
	}{
		// The only risk shape: a gang worker that, after render, carries no
		// required podAffinity — no key resolved and no user term.
		{"gang worker, no key, no user affinity", basicPodSpec(), gangPlan, gangInst, worker, true},
		// A resolved topologyKey means the injector added a co-location term.
		{"gang worker, resolved topologyKey", basicPodSpec(), keyedPlan, keyedInst, keyedRunners[1], false},
		// A user-declared podAffinity is preserved on the pod — operator owns it.
		{"gang worker, user podAffinity", withUserPodAffinity(basicPodSpec()), gangPlan, gangInst, worker, false},
		// The leader is the domain anchor; it never carries the term.
		{"gang leader (anchor)", basicPodSpec(), gangPlan, gangInst, leader, false},
		// Single-pod Instances have nothing to co-locate.
		{"single-pod default", basicPodSpec(), singlePlan, singleInst, singleRunner, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetGangSplitRiskSeen()
			rec := record.NewFakeRecorder(8)
			input := workload.ReconcileInput{OwnerObject: basicISVC()}
			pod := renderForSplitTest(t, tc.ps, tc.plan, tc.inst, tc.runner)

			maybeWarnGangSplitRisk(rec, input, tc.plan, tc.inst, tc.runner, pod)

			got := countGangSplitWarnings(rec)
			want := 0
			if tc.wantWarn {
				want = 1
			}
			if got != want {
				t.Errorf("GangSplitRisk warnings: got %d want %d", got, want)
			}
		})
	}
}

// TestMaybeWarnGangSplitRisk_DedupPerComponent pins that the warning
// fires once per (owner, Component) per process — a 3-worker gang
// rendering three at-risk worker pods yields a single event, not three.
func TestMaybeWarnGangSplitRisk_DedupPerComponent(t *testing.T) {
	resetGangSplitRiskSeen()
	rec := record.NewFakeRecorder(8)
	input := workload.ReconcileInput{OwnerObject: basicISVC()}
	plan, inst, runners := multiPodPlan(3)
	worker := runners[1]
	pod := renderForSplitTest(t, basicPodSpec(), plan, inst, worker)

	maybeWarnGangSplitRisk(rec, input, plan, inst, worker, pod)
	maybeWarnGangSplitRisk(rec, input, plan, inst, worker, pod)

	if got := countGangSplitWarnings(rec); got != 1 {
		t.Errorf("want exactly 1 warning after two calls (dedup), got %d", got)
	}
}

// TestMaybeWarnGangSplitRisk_NilSafe pins the nil-recorder / nil-target
// no-op contract so callers never have to branch.
func TestMaybeWarnGangSplitRisk_NilSafe(t *testing.T) {
	resetGangSplitRiskSeen()
	plan, inst, runners := multiPodPlan(3)
	worker := runners[1]
	pod := renderForSplitTest(t, basicPodSpec(), plan, inst, worker)

	// nil recorder, valid target → no panic.
	maybeWarnGangSplitRisk(nil, workload.ReconcileInput{OwnerObject: basicISVC()}, plan, inst, worker, pod)
	// valid recorder, nil target (no OwnerObject/EventTarget) → no panic, no event.
	rec := record.NewFakeRecorder(8)
	maybeWarnGangSplitRisk(rec, workload.ReconcileInput{}, plan, inst, worker, pod)
	if got := countGangSplitWarnings(rec); got != 0 {
		t.Errorf("nil-target must emit nothing, got %d", got)
	}
}

func TestMaybeWarnGangSplitRisk_RecommendationIsProviderNeutral(t *testing.T) {
	resetGangSplitRiskSeen()
	rec := record.NewFakeRecorder(1)
	plan, inst, runners := multiPodPlan(2)
	pod := renderForSplitTest(t, basicPodSpec(), plan, inst, runners[1])

	maybeWarnGangSplitRisk(rec, workload.ReconcileInput{OwnerObject: basicISVC()}, plan, inst, runners[1], pod)

	select {
	case event := <-rec.Events:
		if !strings.Contains(event, ".topologyKey") {
			t.Fatalf("recommendation does not name topologyKey: %q", event)
		}
		if strings.Contains(event, "cloud.google.com") {
			t.Fatalf("recommendation must not prescribe a provider-specific label: %q", event)
		}
	default:
		t.Fatal("expected GangSplitRisk event")
	}
}
