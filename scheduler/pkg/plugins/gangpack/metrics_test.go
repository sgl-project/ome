package gangpack

import (
	"testing"

	compmetrics "k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/testutil"
	"k8s.io/kube-scheduler/framework"

	"sigs.k8s.io/ome/scheduler/pkg/placement"
)

// TestPinMetrics: a successful pin bumps gang_pin_total{result=pinned} and the
// pinned_groups gauge; an unplaceable gang bumps gang_pin_total{result=no_fit}.
// Deltas (not absolutes) keep the test independent of the shared global registry.
func TestPinMetrics(t *testing.T) {
	g := &GangPack{pins: placement.New()}
	nodes := []framework.NodeInfo{
		nodeInfo(gpuNode("n1", "a", "4")), nodeInfo(gpuNode("n2", "a", "4")),
	}
	pod := gpuPod("4")

	pinnedBefore := counterValue(t, gangPinTotal.WithLabelValues("pinned"))
	g.pinGang(newCycleState(), nodes, gangInfo{key: "team/pf", minMember: 2, topologyKey: testKey}, pod)
	if d := counterValue(t, gangPinTotal.WithLabelValues("pinned")) - pinnedBefore; d != 1 {
		t.Fatalf("pinned counter delta = %v, want 1", d)
	}
	if v := gaugeValue(t, pinnedGroups); v < 1 {
		t.Fatalf("pinned_groups gauge = %v, want >= 1 after a pin", v)
	}

	noFitBefore := counterValue(t, gangPinTotal.WithLabelValues("no_fit"))
	g.pinGang(newCycleState(), nodes, gangInfo{key: "team/big", minMember: 9, topologyKey: testKey}, pod)
	if d := counterValue(t, gangPinTotal.WithLabelValues("no_fit")) - noFitBefore; d != 1 {
		t.Fatalf("no_fit counter delta = %v, want 1", d)
	}
}

func counterValue(t *testing.T, m compmetrics.CounterMetric) float64 {
	t.Helper()
	v, err := testutil.GetCounterMetricValue(m)
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	return v
}

func gaugeValue(t *testing.T, m compmetrics.GaugeMetric) float64 {
	t.Helper()
	v, err := testutil.GetGaugeMetricValue(m)
	if err != nil {
		t.Fatalf("read gauge: %v", err)
	}
	return v
}
