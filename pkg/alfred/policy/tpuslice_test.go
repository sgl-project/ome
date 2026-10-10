package policy_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/ome/pkg/alfred/config"
	"sigs.k8s.io/ome/pkg/alfred/policy"
	"sigs.k8s.io/ome/pkg/alfred/snapshot"
	"sigs.k8s.io/ome/pkg/alfred/testutil"
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

const (
	topologyLabel = "cloud.google.com/gke-tpu-topology"
	idLabel       = "cloud.google.com/gke-tpu-partition-2x2x2-id"
	stateLabel    = "cloud.google.com/gke-tpu-partition-2x2x2-state"
)

// partition labels a node as a host of the 2x2x2 partition id in the given state.
func partition(id, state string) testutil.NodeOption {
	return testutil.NodeLabels(map[string]string{idLabel: id, stateLabel: state})
}

// tpuSliceSnapshot places a two-host 2x2x2 TPU gang on source-a/source-b
// (partition "p-src") and adds the given free-capacity nodes.
func tpuSliceSnapshot(extra func(*testutil.SnapshotBuilder)) (*snapshot.ClusterSnapshot, *snapshot.Instance) {
	b := testutil.NewSnapshot().
		WithNode("source-a", "", 0, partition("p-src", "HEALTHY"), testutil.NodeMaintenance("de-schedule")).
		WithNode("source-b", "", 0, partition("p-src", "HEALTHY"), testutil.NodeMaintenance("de-schedule"))
	extra(b)
	snap := b.WithMultiPodTPUInstance("prod/tpu", v1beta1.EngineComponent, constants.OMENative, 4, "source-a", "source-b").Build()
	inst := snap.Workloads[types.NamespacedName{Namespace: "prod", Name: "tpu"}].Components[v1beta1.EngineComponent].Instances[0]
	for i := range inst.Pods {
		inst.Pods[i].TPUSliceProvisioned = true
		inst.Pods[i].NodeSelector = map[string]string{topologyLabel: "2x2x2"}
	}
	return snap, inst
}

// enabledConfig returns the default config with TPU slice moves turned on.
func enabledConfig() *config.Config {
	cfg := config.Default()
	enabled := true
	cfg.TPUSliceMigrationEnabled = &enabled
	return cfg
}

// TestPlanTPUSliceReplacementCountsFreePartitions checks every complete, usable
// partition of the topology is counted.
func TestPlanTPUSliceReplacementCountsFreePartitions(t *testing.T) {
	snap, inst := tpuSliceSnapshot(func(b *testutil.SnapshotBuilder) {
		b.WithNode("free-1a", "", 0, partition("p-free-1", "HEALTHY")).
			WithNode("free-1b", "", 0, partition("p-free-1", "HEALTHY")).
			WithNode("free-2a", "", 0, partition("p-free-2", "HEALTHY")).
			WithNode("free-2b", "", 0, partition("p-free-2", "HEALTHY"))
	})
	plan, reason := policy.PlanTPUSliceReplacement(snap, enabledConfig(), inst)
	if reason != "" || plan == nil || plan.Topology != "2x2x2" || plan.FreePartitions != 2 {
		t.Fatalf("plan=%+v reason=%q, want two free 2x2x2 partitions", plan, reason)
	}
}

// TestPlanTPUSliceReplacementRejectsUnusablePartitions checks each reason a
// partition cannot host the replacement leaves no free capacity.
func TestPlanTPUSliceReplacementRejectsUnusablePartitions(t *testing.T) {
	tests := []struct {
		name  string
		nodes func(*testutil.SnapshotBuilder)
	}{
		{"one host under maintenance", func(b *testutil.SnapshotBuilder) {
			b.WithNode("m-a", "", 0, partition("p-m", "HEALTHY")).
				WithNode("m-b", "", 0, partition("p-m", "HEALTHY"), testutil.NodeMaintenance("de-schedule"))
		}},
		{"one host cordoned", func(b *testutil.SnapshotBuilder) {
			b.WithNode("c-a", "", 0, partition("p-c", "HEALTHY"), testutil.NodeCordoned()).
				WithNode("c-b", "", 0, partition("p-c", "HEALTHY"))
		}},
		{"one host unhealthy", func(b *testutil.SnapshotBuilder) {
			b.WithNode("u-a", "", 0, partition("p-u", "HEALTHY")).
				WithNode("u-b", "", 0, partition("p-u", "HEALTHY"), testutil.NodeUnhealthy())
		}},
		{"partition state is not healthy", func(b *testutil.SnapshotBuilder) {
			b.WithNode("s-a", "", 0, partition("p-s", "DEGRADED")).
				WithNode("s-b", "", 0, partition("p-s", "DEGRADED"))
		}},
		{"one host holds an accelerator pod", func(b *testutil.SnapshotBuilder) {
			b.WithNode("o-a", "", 0, partition("p-o", "HEALTHY")).
				WithNode("o-b", "", 0, partition("p-o", "HEALTHY")).
				WithMultiPodTPUInstance("prod/other", v1beta1.EngineComponent, constants.OMENative, 4, "o-b")
		}},
		{"no partition label", func(b *testutil.SnapshotBuilder) {
			b.WithNode("n-a", "", 0).WithNode("n-b", "", 0)
		}},
		{"partition missing a host for the two-pod gang", func(b *testutil.SnapshotBuilder) {
			b.WithNode("h-a", "", 0, partition("p-h", "HEALTHY"))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap, inst := tpuSliceSnapshot(tt.nodes)
			plan, reason := policy.PlanTPUSliceReplacement(snap, enabledConfig(), inst)
			if plan != nil || reason != policy.AdvisoryNoTPUSliceCapacity {
				t.Fatalf("plan=%+v reason=%q, want %s", plan, reason, policy.AdvisoryNoTPUSliceCapacity)
			}
		})
	}
}

// TestPlanTPUSliceReplacementStaysUnmodeled checks the instances the path must
// not move keep the default advisory.
func TestPlanTPUSliceReplacementStaysUnmodeled(t *testing.T) {
	free := func(b *testutil.SnapshotBuilder) {
		b.WithNode("f-a", "", 0, partition("p-f", "HEALTHY")).WithNode("f-b", "", 0, partition("p-f", "HEALTHY"))
	}
	tests := []struct {
		name   string
		cfg    func() *config.Config
		mutate func(*snapshot.Instance)
	}{
		{"path off by default", config.Default, func(*snapshot.Instance) {}},
		{"a pod is not slice-provisioned", enabledConfig, func(i *snapshot.Instance) { i.Pods[1].TPUSliceProvisioned = false }},
		{"a pod has no topology", enabledConfig, func(i *snapshot.Instance) { i.Pods[0].NodeSelector = nil }},
		{"pods disagree on topology", enabledConfig, func(i *snapshot.Instance) {
			i.Pods[1].NodeSelector = map[string]string{topologyLabel: "2x2x4"}
		}},
		{"instance also holds GPUs", enabledConfig, func(i *snapshot.Instance) { i.TotalGPUs = 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap, inst := tpuSliceSnapshot(free)
			tt.mutate(inst)
			plan, reason := policy.PlanTPUSliceReplacement(snap, tt.cfg(), inst)
			if plan != nil || reason != policy.AdvisoryAcceleratorPlacementUnmodeled {
				t.Fatalf("plan=%+v reason=%q, want %s", plan, reason, policy.AdvisoryAcceleratorPlacementUnmodeled)
			}
		})
	}
}

// TestPlanTPUSliceReplacementUsesConfiguredLabels checks non-default label
// settings are honored.
func TestPlanTPUSliceReplacementUsesConfiguredLabels(t *testing.T) {
	snap, inst := tpuSliceSnapshot(func(b *testutil.SnapshotBuilder) {
		labels := map[string]string{"example.com/part-2x2x2": "p-x", "example.com/part-2x2x2-st": "OK"}
		b.WithNode("x-a", "", 0, testutil.NodeLabels(labels)).WithNode("x-b", "", 0, testutil.NodeLabels(labels))
	})
	for i := range inst.Pods {
		inst.Pods[i].NodeSelector = map[string]string{"example.com/topo": "2x2x2"}
	}
	cfg := enabledConfig()
	cfg.TPUSlicePartitions.TopologyLabel = "example.com/topo"
	cfg.TPUSlicePartitions.IDLabel = "example.com/part-%s"
	cfg.TPUSlicePartitions.StateLabel = "example.com/part-%s-st"
	cfg.TPUSlicePartitions.HealthyState = "OK"
	plan, reason := policy.PlanTPUSliceReplacement(snap, cfg, inst)
	if reason != "" || plan == nil || plan.FreePartitions != 1 {
		t.Fatalf("plan=%+v reason=%q, want the one configured partition", plan, reason)
	}
}

// TestPlanTPUSliceReplacementMatchesRetainedSelectors checks free hosts must fit
// the member pods' node selectors and required node affinity, ignoring only the
// slice and topology selectors the slice provisioner writes.
func TestPlanTPUSliceReplacementMatchesRetainedSelectors(t *testing.T) {
	const zoneLabel = "topology.kubernetes.io/zone"
	inZone := func(zone string) testutil.NodeOption {
		return testutil.NodeLabels(map[string]string{zoneLabel: zone})
	}
	freePair := func(zone string) func(*testutil.SnapshotBuilder) {
		return func(b *testutil.SnapshotBuilder) {
			b.WithNode("f-a", "", 0, partition("p-f", "HEALTHY"), inZone(zone)).
				WithNode("f-b", "", 0, partition("p-f", "HEALTHY"), inZone(zone))
		}
	}
	zoneAffinity := func(zone string) *corev1.NodeSelector {
		return &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
			{Key: zoneLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{zone}},
		}}}}
	}
	tests := []struct {
		name   string
		zone   string
		mutate func(*snapshot.Instance)
		want   string
	}{
		{"selector matches the free hosts", "zone-a", func(i *snapshot.Instance) {
			for p := range i.Pods {
				i.Pods[p].NodeSelector[zoneLabel] = "zone-a"
			}
		}, ""},
		{"selector names another zone", "zone-b", func(i *snapshot.Instance) {
			for p := range i.Pods {
				i.Pods[p].NodeSelector[zoneLabel] = "zone-a"
			}
		}, policy.AdvisoryNoTPUSliceCapacity},
		{"the source slice selector is ignored", "zone-a", func(i *snapshot.Instance) {
			for p := range i.Pods {
				i.Pods[p].NodeSelector["cloud.google.com/gke-tpu-slice"] = "old-slice"
			}
		}, ""},
		{"required affinity matches the free hosts", "zone-a", func(i *snapshot.Instance) {
			i.Pods[0].RequiredNodeAffinity = zoneAffinity("zone-a")
		}, ""},
		{"required affinity names another zone", "zone-b", func(i *snapshot.Instance) {
			i.Pods[0].RequiredNodeAffinity = zoneAffinity("zone-a")
		}, policy.AdvisoryNoTPUSliceCapacity},
		{"members disagree on a selector", "zone-a", func(i *snapshot.Instance) {
			i.Pods[0].NodeSelector[zoneLabel] = "zone-a"
			i.Pods[1].NodeSelector[zoneLabel] = "zone-b"
		}, policy.AdvisoryNoTPUSliceCapacity},
		{"unparsable required affinity stays unmodeled", "zone-a", func(i *snapshot.Instance) {
			i.Pods[0].RequiredNodeAffinity = &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
				{Key: zoneLabel, Operator: "Bogus", Values: []string{"zone-a"}},
			}}}}
		}, policy.AdvisoryAcceleratorPlacementUnmodeled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap, inst := tpuSliceSnapshot(freePair(tt.zone))
			tt.mutate(inst)
			plan, reason := policy.PlanTPUSliceReplacement(snap, enabledConfig(), inst)
			if reason != tt.want || (tt.want == "" && (plan == nil || plan.FreePartitions != 1)) {
				t.Fatalf("plan=%+v reason=%q, want reason %q", plan, reason, tt.want)
			}
		})
	}
}
