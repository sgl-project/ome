package integration

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	schedutil "sigs.k8s.io/scheduler-plugins/test/util"
)

// OME renders topologySpread as a standard DoNotSchedule constraint on each
// gang's anchor (leader) pod — the scheduler carries NO spread logic of its
// own. These tests prove the plugin's existing machinery absorbs that
// constraint: gangpack best-fit-pins a packed domain, PodTopologySpread's
// Filter (kept enabled for hard constraints) vetoes it, PostFilter remembers
// the failed domain, and the retry re-plans into a compatible one.

// cubeLabelKey is the coarser fault-domain label the split-key test spreads
// across while co-locating by domainLabelKey (the TPU shape: gang-sized
// partitions inside a cube).
const cubeLabelKey = "topology.ome.io/cube"

func makeCubeNode(name, partition, cube string) *v1.Node {
	n := makeGPUNode(name, partition, 1)
	n.Labels[cubeLabelKey] = cube
	return n
}

func makePlainPG(t *testing.T, tc *testContext, ns, name string) {
	t.Helper()
	pg := schedutil.MakePG(name, ns, 2, nil, nil)
	pg.Annotations = map[string]string{topologyKeyAnnotation: domainLabelKey}
	if _, err := tc.SchedClient.SchedulingV1alpha1().PodGroups(ns).Create(tc.Ctx, pg, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create podgroup %s: %v", name, err)
	}
}

// spreadLeaderPod mirrors OME's rendered anchor: the leader carries the
// spread constraint (selector matching sibling leaders, maxSkew 1).
func spreadLeaderPod(name, ns, pgName, spreadKey string, when v1.UnsatisfiableConstraintAction) *v1.Pod {
	p := makeGangPod(name, ns, pgName)
	p.Labels["app"] = "tsc-svc"
	p.Labels["runner"] = "leader"
	p.Spec.TopologySpreadConstraints = []v1.TopologySpreadConstraint{{
		MaxSkew:           1,
		TopologyKey:       spreadKey,
		WhenUnsatisfiable: when,
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
			"app": "tsc-svc", "runner": "leader",
		}},
	}}
	return p
}

// spreadWorkerPod mirrors the worker's spread-relevant surface: no
// constraint of its own. Under this scheduler the gang PIN is what
// co-locates workers with their leader (the rendered worker→leader
// affinity is defense-in-depth for non-gang schedulers and is exercised
// by the controller-side suites); the TSC-gated leader decides the fault
// domain because only leaders match the constraint's selector.
func spreadWorkerPod(name, ns, pgName string) *v1.Pod {
	p := makeGangPod(name, ns, pgName)
	p.Labels["runner"] = "worker"
	return p
}

// placeTSCGang creates a 2-member gang whose leader carries the spread
// constraint, waits for both binds, asserts gang integrity, and returns the
// value of domainLabel on the gang's nodes.
func placeTSCGang(t *testing.T, tc *testContext, ns, pgName, spreadKey, domainLabel string) string {
	t.Helper()
	makePlainPG(t, tc, ns, pgName)
	if _, err := tc.ClientSet.CoreV1().Pods(ns).Create(tc.Ctx,
		spreadLeaderPod(pgName+"-0", ns, pgName, spreadKey, v1.DoNotSchedule), metav1.CreateOptions{}); err != nil {
		t.Fatalf("create leader: %v", err)
	}
	if _, err := tc.ClientSet.CoreV1().Pods(ns).Create(tc.Ctx,
		spreadWorkerPod(pgName+"-1", ns, pgName), metav1.CreateOptions{}); err != nil {
		t.Fatalf("create worker: %v", err)
	}
	labelOf := func(node string) string {
		n, err := tc.ClientSet.CoreV1().Nodes().Get(tc.Ctx, node, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("get node %s: %v", node, err)
		}
		return n.Labels[domainLabel]
	}
	n0 := waitForPodBound(t, tc, ns, pgName+"-0", 60*time.Second)
	n1 := waitForPodBound(t, tc, ns, pgName+"-1", 60*time.Second)
	if labelOf(n0) != labelOf(n1) {
		t.Fatalf("gang %s split across %s values %s/%s", pgName, domainLabel, labelOf(n0), labelOf(n1))
	}
	return labelOf(n0)
}

// TestTSCSpreadsGangsAcrossCubes: the split-key (TPU) shape with zero plugin
// spread configuration. Two gangs co-locate by gang-sized partitions and must
// land in different CUBES purely from the leaders' vanilla constraint.
func TestTSCSpreadsGangsAcrossCubes(t *testing.T) {
	tc := startScheduler(t, globalKubeConfig, gangPackOptions(t)...)
	defer tc.teardown(t)

	const ns = "tsc-cubes"
	createNamespace(t, tc, ns)
	for _, n := range []struct{ name, partition, cube string }{
		{"tc-p1a", "p1", "cube1"}, {"tc-p1b", "p1", "cube1"},
		{"tc-p2a", "p2", "cube1"}, {"tc-p2b", "p2", "cube1"},
		{"tc-p3a", "p3", "cube2"}, {"tc-p3b", "p3", "cube2"},
	} {
		if _, err := tc.ClientSet.CoreV1().Nodes().Create(tc.Ctx, makeCubeNode(n.name, n.partition, n.cube), metav1.CreateOptions{}); err != nil {
			t.Fatalf("create node %s: %v", n.name, err)
		}
	}

	c0 := placeTSCGang(t, tc, ns, "v0", cubeLabelKey, cubeLabelKey)
	c1 := placeTSCGang(t, tc, ns, "v1", cubeLabelKey, cubeLabelKey)
	if c0 == c1 {
		t.Fatalf("both gangs in cube %q — the leader constraint did not spread on the gang scheduler", c0)
	}
}

// TestTSCBalancesThenBlocksThenFrees pins the maxSkew semantics end to end
// on the same-key (rack) shape: three gangs on two racks balance 2/1; a
// fourth that would over-skew the only rack with capacity holds Pending; and
// draining a gang from the under-loaded rack lets it place there — the
// requeue path is PodTopologySpread's own pod-delete hint.
func TestTSCBalancesThenBlocksThenFrees(t *testing.T) {
	tc := startScheduler(t, globalKubeConfig, gangPackOptions(t)...)
	defer tc.teardown(t)

	const ns = "tsc-balance"
	createNamespace(t, tc, ns)
	// Rack a fits three 2-node gangs; rack b fits one.
	for _, n := range []struct{ name, rack string }{
		{"tb-a1", "a"}, {"tb-a2", "a"}, {"tb-a3", "a"},
		{"tb-a4", "a"}, {"tb-a5", "a"}, {"tb-a6", "a"},
		{"tb-b1", "b"}, {"tb-b2", "b"},
	} {
		if _, err := tc.ClientSet.CoreV1().Nodes().Create(tc.Ctx, makeGPUNode(n.name, n.rack, 1), metav1.CreateOptions{}); err != nil {
			t.Fatalf("create node %s: %v", n.name, err)
		}
	}

	r0 := placeTSCGang(t, tc, ns, "b0", domainLabelKey, domainLabelKey)
	r1 := placeTSCGang(t, tc, ns, "b1", domainLabelKey, domainLabelKey)
	if r0 == r1 {
		t.Fatalf("first two gangs share rack %q, want balanced 1/1", r0)
	}
	// Third gang: counts are 1/1, so either rack keeps skew ≤ 1; only rack a
	// has capacity left. Balanced spreading places it — Required never blocks
	// a balanced placement.
	if r2 := placeTSCGang(t, tc, ns, "b2", domainLabelKey, domainLabelKey); r2 != "a" {
		t.Fatalf("third gang in %q, want a (only rack with capacity, skew stays 1)", r2)
	}

	// Fourth gang: counts a=2, b=1. The minimum is b, but b is full; a would
	// make skew 2 — the leader must hold Pending.
	makePlainPG(t, tc, ns, "b3")
	if _, err := tc.ClientSet.CoreV1().Pods(ns).Create(tc.Ctx,
		spreadLeaderPod("b3-0", ns, "b3", domainLabelKey, v1.DoNotSchedule), metav1.CreateOptions{}); err != nil {
		t.Fatalf("create leader: %v", err)
	}
	if _, err := tc.ClientSet.CoreV1().Pods(ns).Create(tc.Ctx,
		spreadWorkerPod("b3-1", ns, "b3"), metav1.CreateOptions{}); err != nil {
		t.Fatalf("create worker: %v", err)
	}
	ensureNotBound(t, tc, ns, "b3-0", 3*time.Second)

	// Drain the rack-b gang: counts drop to a=2, b=0 — b still has no room
	// for skew until it does, but now the two free b nodes CAN take the gang
	// at skew ≤ 1. The scheduler carries no spread logic, so the retry that
	// finds b is driven by cluster events: here, as in production, the pod
	// churn OME's controller generates (attempt pods are recreated on its
	// escalation cadence) — modeled by recreating the held gang's pods. The
	// scheduler's periodic unschedulable flush remains the generic backstop.
	zero := int64(0)
	var bGang string
	for _, g := range []string{"b0", "b1"} {
		if map[string]bool{"b0": r0 == "b", "b1": r1 == "b"}[g] {
			bGang = g
		}
	}
	for i := 0; i < 2; i++ {
		name := bGang + "-" + string(rune('0'+i))
		if err := tc.ClientSet.CoreV1().Pods(ns).Delete(tc.Ctx, name, metav1.DeleteOptions{GracePeriodSeconds: &zero}); err != nil {
			t.Fatalf("delete pod %s: %v", name, err)
		}
	}
	for _, name := range []string{"b3-0", "b3-1"} {
		if err := tc.ClientSet.CoreV1().Pods(ns).Delete(tc.Ctx, name, metav1.DeleteOptions{GracePeriodSeconds: &zero}); err != nil {
			t.Fatalf("delete pod %s: %v", name, err)
		}
	}
	if _, err := tc.ClientSet.CoreV1().Pods(ns).Create(tc.Ctx,
		spreadLeaderPod("b3r-0", ns, "b3", domainLabelKey, v1.DoNotSchedule), metav1.CreateOptions{}); err != nil {
		t.Fatalf("recreate leader: %v", err)
	}
	if _, err := tc.ClientSet.CoreV1().Pods(ns).Create(tc.Ctx,
		spreadWorkerPod("b3r-1", ns, "b3"), metav1.CreateOptions{}); err != nil {
		t.Fatalf("recreate worker: %v", err)
	}
	for _, name := range []string{"b3r-0", "b3r-1"} {
		node := waitForPodBound(t, tc, ns, name, 60*time.Second)
		if d := domainOfNode(t, tc, node); d != "b" {
			t.Fatalf("freed gang pod %s bound in %q, want b", name, d)
		}
	}
}
