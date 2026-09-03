package integration

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schedutil "sigs.k8s.io/scheduler-plugins/test/util"
)

// leaderMarkerLabel is the label a gang's anchor pod carries so a worker's
// required podAffinity term can target it specifically — mirroring how OME's
// real multi-node gangs mark their leader Runner (see spread_tsc_test.go's
// "runner" convention).
const leaderMarkerLabel = "runner"

// TestPermitActivationRescuesAffinityBlockedWorker pins the production shape
// for a multi-pod gang: the worker carries a REQUIRED podAffinity term to its
// leader (worker-follows-leader, never mutual — a mutual requirement would
// deadlock the very first pod scheduled), gated together by the same
// PodGroup. That affinity is enforced by a different plugin than this one's
// gate, so a worker whose own scheduling attempt outraces its leader fails
// Filter with no gang-aware cluster event left to retry it on: the leader
// that eventually pins the gang is only ASSUMED while it waits at Permit for
// its sibling, and an assumed pod produces no Pod Update for anything to
// react to. activateGangMembers is what rescues it: the instant the leader
// reaches Permit, every live gang member is activated, so the worker retries
// immediately and finds the leader already assumed.
//
// Creating the worker before the leader forces exactly this ordering. Both
// members must bind well inside the gate's permit timeout — a pass here is a
// regression guard on activateGangMembers's unconditional per-member
// activation, which otherwise has no test at this exact affinity-ordered
// shape.
func TestPermitActivationRescuesAffinityBlockedWorker(t *testing.T) {
	tc := startScheduler(t, globalKubeConfig, gangPackOptions(t)...)
	defer tc.teardown(t)

	const ns = "gang-sibling-wake"
	createNamespace(t, tc, ns)

	for _, n := range []string{"sw1", "sw2"} {
		if _, err := tc.ClientSet.CoreV1().Nodes().Create(tc.Ctx, makeGPUNode(n, "a", 1), metav1.CreateOptions{}); err != nil {
			t.Fatalf("create node %s: %v", n, err)
		}
	}

	pg := schedutil.MakePG("gang", ns, 2, nil, nil)
	pg.Annotations = map[string]string{topologyKeyAnnotation: domainLabelKey}
	// Generous on purpose: the assertion below is a tight bind window, so a pass
	// must come from prompt convergence, never from this timeout firing an
	// unwind/retry that happens to land in time by coincidence.
	longTimeout := int32(120)
	pg.Spec.ScheduleTimeoutSeconds = &longTimeout
	if _, err := tc.SchedClient.SchedulingV1alpha1().PodGroups(ns).Create(tc.Ctx, pg, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create podgroup: %v", err)
	}

	leader := makeGangPod("sw-leader", ns, "gang")
	leader.Labels[leaderMarkerLabel] = "leader"

	worker := makeGangPod("sw-worker", ns, "gang")
	worker.Labels[leaderMarkerLabel] = "worker"
	worker.Spec.Affinity = &v1.Affinity{PodAffinity: &v1.PodAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{{
			TopologyKey: domainLabelKey,
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
				podGroupLabel:     "gang",
				leaderMarkerLabel: "leader",
			}},
		}},
	}}

	// Worker first: its earliest attempt(s) can only fail — the leader either
	// does not exist yet, or exists but has not been assumed anywhere the
	// required affinity term could match.
	if _, err := tc.ClientSet.CoreV1().Pods(ns).Create(tc.Ctx, worker, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create worker: %v", err)
	}
	if _, err := tc.ClientSet.CoreV1().Pods(ns).Create(tc.Ctx, leader, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create leader: %v", err)
	}

	const tight = 8 * time.Second
	for _, name := range []string{"sw-leader", "sw-worker"} {
		node := waitForPodBound(t, tc, ns, name, tight)
		if node != "sw1" && node != "sw2" {
			t.Errorf("pod %s bound to %s, want sw1/sw2", name, node)
		}
	}
}
