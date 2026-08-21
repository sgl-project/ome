package defrag

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/ome/pkg/alfred/config"
	"sigs.k8s.io/ome/pkg/alfred/policy"
	"sigs.k8s.io/ome/pkg/alfred/snapshot"
	"sigs.k8s.io/ome/pkg/alfred/testutil"
	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// consolidationBuilder is the canonical single-move scenario: prod/mover's
// 1-GPU pod on node1 (7 free) can fill node2's hole (1 free), freeing a full
// 8-GPU node. Observed F = 0.2175, best F = 0 (the pinScenario numbers).
func consolidationBuilder() *testutil.SnapshotBuilder {
	b := testutil.NewSnapshot().
		WithNode("node1", "h100", 8).
		WithNode("node2", "h100", 8).
		WithInstance("prod/mover", v1beta1.EngineComponent, constants.RawDeployment, "node1", 1)
	b.WithOtherOccupant("node2", 7)
	return b
}

// lowGate returns a config whose gate the consolidation scenario passes
// (0.2175 sits below the default 0.25 threshold by design — the default gate
// is deliberately conservative).
func lowGate() *config.Config {
	cfg := config.Default()
	*cfg.Policies.Defragmentation.FragmentationThreshold = 0.1
	return cfg
}

func evaluate(t *testing.T, snap *snapshot.ClusterSnapshot, cfg *config.Config) []policy.Candidate {
	t.Helper()
	var p Policy
	if p.Name() != PolicyName {
		t.Fatalf("policy name = %q", p.Name())
	}
	return p.Evaluate(snap, cfg)
}

func executables(cands []policy.Candidate) []policy.Candidate {
	var out []policy.Candidate
	for _, c := range cands {
		if c.Executable {
			out = append(out, c)
		}
	}
	return out
}

func advisories(cands []policy.Candidate, reason string) []policy.Candidate {
	var out []policy.Candidate
	for _, c := range cands {
		if !c.Executable && c.AdvisoryReason == reason {
			out = append(out, c)
		}
	}
	return out
}

// TestGate: below the threshold (or disabled) the policy returns nothing —
// Alfred stays quiescent on a cluster it cannot improve.
func TestGate(t *testing.T) {
	snap := consolidationBuilder().Build()

	if got := evaluate(t, snap, config.Default()); got != nil {
		// 0.2175 < default 0.25: the reclaimable shape alone must not
		// wake the policy at default settings.
		t.Fatalf("below-threshold Evaluate = %v, want nil", got)
	}

	cfg := lowGate()
	*cfg.Policies.Defragmentation.Enabled = false
	if got := evaluate(t, snap, cfg); got != nil {
		t.Fatalf("disabled Evaluate = %v, want nil", got)
	}
}

// TestConsolidationCandidate pins the whole pipeline on the canonical move:
// surge-shaped (single ready replica), benefit = the full reclaimable 0.2175,
// cost = rolling restart, hints = the hole it should fill.
func TestConsolidationCandidate(t *testing.T) {
	cands := evaluate(t, consolidationBuilder().Build(), lowGate())
	execs := executables(cands)
	if len(execs) != 1 {
		t.Fatalf("want exactly one executable candidate, got %+v", cands)
	}
	c := execs[0]
	if c.Workload.String() != "prod/mover" || c.Component != v1beta1.EngineComponent || c.Instance != 0 {
		t.Fatalf("candidate identity: %+v", c)
	}
	if c.Mode != constants.RawDeployment || !c.SurgeShaped {
		t.Fatalf("single ready replica must simulate surge-first rolling restart: %+v", c)
	}
	if c.FromNode != "node1" {
		t.Fatalf("FromNode = %q, want node1", c.FromNode)
	}
	if len(c.HintTargetNodes) != 1 || c.HintTargetNodes[0] != "node2" {
		t.Fatalf("hints = %v, want [node2] (fill the hole)", c.HintTargetNodes)
	}
	almost(t, "Benefit", c.Benefit, 0.2175)
	almost(t, "Cost", c.Cost, costRollingRestart)
	almost(t, "Score", c.Score, 0.2175-costRollingRestart)
	if c.Emergency {
		t.Fatal("no pending pod, must not be an emergency")
	}
	if c.FootprintGPUs != 1 {
		t.Fatalf("FootprintGPUs = %d, want 1", c.FootprintGPUs)
	}
}

// TestNoSurgeHeadroomAdvisory: an 8-GPU OMENative instance with no 8-free
// target must not be dispatched — it degrades to advisory instead of
// stalling in SurgePending until timeout.
func TestNoSurgeHeadroomAdvisory(t *testing.T) {
	b := testutil.NewSnapshot().
		WithNode("node1", "h100", 8).
		WithNode("node2", "h100", 8).
		WithNode("node3", "h100", 8).
		WithInstance("prod/big", v1beta1.EngineComponent, constants.OMENative, "node1", 8).
		WithInstance("prod/mover", v1beta1.EngineComponent, constants.RawDeployment, "node2", 1)
	b.WithOtherOccupant("node3", 7)
	cfg := lowGate()
	*cfg.Policies.Defragmentation.FragmentationThreshold = 0.05

	cands := evaluate(t, b.Build(), cfg)
	blocked := advisories(cands, policy.AdvisoryNoSurgeHeadroom)
	if len(blocked) != 1 {
		t.Fatalf("want one NoSurgeHeadroom advisory, got %+v", cands)
	}
	if blocked[0].Workload.String() != "prod/big" || blocked[0].FromNode != "node1" ||
		blocked[0].FootprintGPUs != 8 || !blocked[0].SurgeShaped {
		t.Fatalf("advisory shape: %+v", blocked[0])
	}
	execs := executables(cands)
	if len(execs) != 1 || execs[0].Workload.String() != "prod/mover" {
		t.Fatalf("the small consolidation move must still be executable: %+v", cands)
	}
}

// TestEvictionShape: with two ready replicas the controller would evict, so
// the simulation is free-then-place and candidates need no surge headroom.
func TestEvictionShape(t *testing.T) {
	b := testutil.NewSnapshot().
		WithNode("node1", "h100", 8).
		WithNode("node2", "h100", 8).
		WithNode("node3", "h100", 8).
		WithInstance("prod/wide", v1beta1.EngineComponent, constants.RawDeployment, "node1", 1).
		WithInstance("prod/wide", v1beta1.EngineComponent, constants.RawDeployment, "node2", 1)
	b.WithOtherOccupant("node3", 7)
	cfg := lowGate()
	*cfg.Policies.Defragmentation.FragmentationThreshold = 0.01

	execs := executables(evaluate(t, b.Build(), cfg))
	if len(execs) != 2 {
		t.Fatalf("want both replicas as candidates, got %+v", execs)
	}
	for _, c := range execs {
		if c.SurgeShaped {
			t.Fatalf("multi-replica RawDeployment must simulate free-then-place: %+v", c)
		}
		almost(t, "Cost", c.Cost, costTargetedEviction)
	}
}

// TestEvictionNoFeasibleTargetAdvisory: a replica whose ranking is empty has
// nowhere to be evicted to — dispatching would strand the replacement in
// Pending. It degrades to advisory, the free-then-place mirror of
// NoSurgeHeadroom, while its sibling with a real target stays executable.
func TestEvictionNoFeasibleTargetAdvisory(t *testing.T) {
	b := testutil.NewSnapshot().
		WithNode("node1", "h100", 8).
		WithNode("node2", "h100", 8).
		WithNode("node3", "h100", 8).
		WithInstance("prod/wide", v1beta1.EngineComponent, constants.RawDeployment, "node1", 1).
		WithInstance("prod/wide", v1beta1.EngineComponent, constants.RawDeployment, "node2", 1)
	b.WithOtherOccupant("node1", 6)
	b.WithOtherOccupant("node2", 7)
	b.WithOtherOccupant("node3", 7)
	b.WithModel("prod/wide", &snapshot.ModelAvailability{
		Key:        snapshot.ModelKey{Kind: snapshot.ModelKindBaseModel, Namespace: "prod", Name: "llm"},
		Backend:    snapshot.BackendPerNode,
		NodesReady: []string{"node1", "node2"},
	})
	cfg := lowGate()
	*cfg.Policies.Defragmentation.FragmentationThreshold = 0.01

	cands := evaluate(t, b.Build(), cfg)

	// The node1 replica is boxed in: node2 is full and node3 lacks the model.
	blocked := advisories(cands, policy.AdvisoryNoFeasibleTarget)
	if len(blocked) != 1 {
		t.Fatalf("want one NoFeasibleTarget advisory, got %+v", cands)
	}
	if blocked[0].FromNode != "node1" || blocked[0].SurgeShaped || blocked[0].FootprintGPUs != 1 {
		t.Fatalf("advisory shape: %+v", blocked[0])
	}
	if len(blocked[0].HintTargetNodes) != 0 {
		t.Fatalf("advisory must carry no hints, got %v", blocked[0].HintTargetNodes)
	}

	// The node2 replica can still reach node1's hole: unchanged.
	execs := executables(cands)
	if len(execs) != 1 || execs[0].FromNode != "node2" {
		t.Fatalf("want the node2 replica executable, got %+v", execs)
	}
	if len(execs[0].HintTargetNodes) != 1 || execs[0].HintTargetNodes[0] != "node1" {
		t.Fatalf("hints = %v, want [node1]", execs[0].HintTargetNodes)
	}
}

// TestEnumerationFilters: cooldown (with override), in-flight migrations,
// and Movable=false all keep a workload out of the candidate set.
func TestEnumerationFilters(t *testing.T) {
	cfg := lowGate()

	recent := testutil.ReferenceTime.Add(-5 * time.Minute)
	b := consolidationBuilder().ConfigureWorkload("prod/mover", func(w *snapshot.Workload) {
		w.LastMigration = &recent
	})
	if got := executables(evaluate(t, b.Build(), cfg)); len(got) != 0 {
		t.Fatalf("workload inside the 30m cooldown must not produce candidates: %+v", got)
	}

	short := time.Minute
	b = consolidationBuilder().ConfigureWorkload("prod/mover", func(w *snapshot.Workload) {
		w.LastMigration = &recent
		w.CooldownOverride = &short
	})
	if got := executables(evaluate(t, b.Build(), cfg)); len(got) != 1 {
		t.Fatalf("cooldown override must re-admit the workload: %+v", got)
	}

	b = consolidationBuilder().ConfigureWorkload("prod/mover", func(w *snapshot.Workload) {
		w.ActiveMigrations = []snapshot.InFlight{{UUID: "u1", Component: v1beta1.EngineComponent}}
	})
	if got := executables(evaluate(t, b.Build(), cfg)); len(got) != 0 {
		t.Fatalf("in-flight migration must exclude the workload: %+v", got)
	}

	b = consolidationBuilder().ConfigureWorkload("prod/mover", func(w *snapshot.Workload) {
		w.Movable = false
	})
	if got := executables(evaluate(t, b.Build(), cfg)); len(got) != 0 {
		t.Fatalf("Movable=false must exclude the workload: %+v", got)
	}
}

// withBystander adds a third node carrying an extra workload without
// disturbing the mover's consolidation shape (node1 must still reach 8 free).
func withBystander(workload string, mode constants.DeploymentModeType) *testutil.SnapshotBuilder {
	return consolidationBuilder().
		WithNode("node3", "h100", 8).
		WithInstance(workload, v1beta1.EngineComponent, mode, "node3", 1)
}

// bystanderGate: the third node dilutes the pool, so the reclaimable share
// shrinks below the test gate of 0.1; open the gate further.
func bystanderGate() *config.Config {
	cfg := lowGate()
	*cfg.Policies.Defragmentation.FragmentationThreshold = 0.05
	return cfg
}

// TestLWSAdvisory: LWS-backed components surface as advisory-only, and the
// lwsRecommendationsEnabled switch silences them.
func TestLWSAdvisory(t *testing.T) {
	build := func() *snapshot.ClusterSnapshot {
		return withBystander("prod/lws", constants.MultiNode).Build()
	}
	cfg := bystanderGate()

	cands := evaluate(t, build(), cfg)
	lws := advisories(cands, policy.AdvisoryLWSMigrationUnsupported)
	if len(lws) != 1 {
		t.Fatalf("want one LWS advisory, got %+v", cands)
	}
	if lws[0].Instance != policy.ComponentWideInstance || lws[0].Mode != constants.MultiNode {
		t.Fatalf("LWS advisory shape: %+v", lws[0])
	}
	if len(executables(cands)) != 1 {
		t.Fatalf("mover must stay executable alongside the advisory: %+v", cands)
	}

	*cfg.LWSRecommendationsEnabled = false
	if got := advisories(evaluate(t, build(), cfg), policy.AdvisoryLWSMigrationUnsupported); len(got) != 0 {
		t.Fatalf("disabled LWS recommendations must not surface: %+v", got)
	}
}

// TestVolumePinnedAdvisory: an RWO-backed workload cannot move by any
// mechanism; it surfaces as advisory while other moves stay executable.
func TestVolumePinnedAdvisory(t *testing.T) {
	b := withBystander("prod/pinned", constants.RawDeployment)
	b.WithModel("prod/pinned", &snapshot.ModelAvailability{
		Key:            snapshot.ModelKey{Kind: snapshot.ModelKindBaseModel, Namespace: "prod", Name: "rwo"},
		Backend:        snapshot.BackendPVC,
		PVCAccessModes: []string{"ReadWriteOnce"},
		VolumePinned:   true,
	})
	cands := evaluate(t, b.Build(), bystanderGate())

	pinned := advisories(cands, policy.AdvisoryVolumePinned)
	if len(pinned) != 1 || pinned[0].Workload.String() != "prod/pinned" {
		t.Fatalf("want one VolumePinned advisory for prod/pinned, got %+v", cands)
	}
	execs := executables(cands)
	if len(execs) != 1 || execs[0].Workload.String() != "prod/mover" {
		t.Fatalf("mover must stay executable: %+v", execs)
	}
}

// TestOMENativeUnavailableAdvisory: without an executor the migration verb
// has no consumer; OMENative candidates degrade to advisory.
func TestOMENativeUnavailableAdvisory(t *testing.T) {
	b := withBystander("prod/omen", constants.OMENative).WithOMENative(false)
	cands := evaluate(t, b.Build(), bystanderGate())
	degraded := advisories(cands, policy.AdvisoryOMENativeUnavailable)
	if len(degraded) != 1 || degraded[0].Workload.String() != "prod/omen" {
		t.Fatalf("want one OMENativeUnavailable advisory, got %+v", cands)
	}
}

// TestEmergencyBoost: a move that unblocks an over-age pending pod in the
// workload's own namespace doubles its score; the same pending in a foreign
// namespace (no shared tenant group) must not boost it.
func TestEmergencyBoost(t *testing.T) {
	cfg := lowGate()
	oneExecutable := func(b *testutil.SnapshotBuilder) policy.Candidate {
		t.Helper()
		execs := executables(evaluate(t, b.Build(), cfg))
		if len(execs) != 1 {
			t.Fatalf("want exactly one executable candidate, got %+v", execs)
		}
		return execs[0]
	}

	// The pending demand shifts the blend weights, so the boost baseline
	// is the identically-shaped cross-namespace run: same weights, same
	// benefit, no tenant-compatible beneficiary.
	unboosted := oneExecutable(consolidationBuilder().WithPendingPodIn("other", 8, 30*time.Minute, "h100"))
	if unboosted.Emergency {
		t.Fatalf("cross-namespace pending without a shared tenant group must not boost: %+v", unboosted)
	}

	boosted := oneExecutable(consolidationBuilder().WithPendingPodIn("prod", 8, 30*time.Minute, "h100"))
	if !boosted.Emergency {
		t.Fatalf("unblocking an over-age same-namespace pending must be an emergency: %+v", boosted)
	}
	almost(t, "boosted score", boosted.Score, unboosted.Score*emergencyBoostFactor)

	if c := oneExecutable(consolidationBuilder().WithPendingPodIn("prod", 8, 5*time.Minute, "h100")); c.Emergency {
		t.Fatalf("a pending younger than emergencyPendingAgeMinutes must not boost: %+v", c)
	}
}

// TestInstanceFootprintsDeterministicOrder: the tie-break must travel with
// the sorted elements. The mixed-size shape below is the regression: the
// size comparison swaps the 8s past the 1 first, and a tie-break read
// through a stale parallel index would then leave "y" ahead of "x".
func TestInstanceFootprintsDeterministicOrder(t *testing.T) {
	inst := &snapshot.Instance{Pods: []snapshot.PodInfo{
		{Namespace: "p", Name: "a", Node: "n1", GPUs: 1},
		{Namespace: "p", Name: "y", Node: "n2", GPUs: 8},
		{Namespace: "p", Name: "x", Node: "n3", GPUs: 8},
	}}
	got := instanceFootprints(inst)
	wantNodes := []string{"n3", "n2", "n1"} // 8@p/x, 8@p/y, 1@p/a
	wantGPUs := []int64{8, 8, 1}
	if len(got) != len(wantNodes) {
		t.Fatalf("footprints = %+v", got)
	}
	for i := range wantNodes {
		if got[i].node != wantNodes[i] || got[i].gpus != wantGPUs[i] {
			t.Fatalf("footprint[%d] = %+v, want %d GPUs from %s (largest first, name tie-break)",
				i, got[i], wantGPUs[i], wantNodes[i])
		}
	}
}

// TestMaintenanceWindowGatesDispatch: outside every configured window,
// executable candidates are dropped; advisories survive (they dispatch
// nothing). ReferenceTime is a Thursday, 12:00 UTC.
func TestMaintenanceWindowGatesDispatch(t *testing.T) {
	build := func() *snapshot.ClusterSnapshot {
		return withBystander("prod/lws", constants.MultiNode).Build()
	}

	cfg := bystanderGate()
	cfg.MaintenanceWindows = []config.MaintenanceWindow{{Days: []string{"Mon"}, Start: "09:00", End: "17:00"}}
	closed := evaluate(t, build(), cfg)
	if len(executables(closed)) != 0 {
		t.Fatalf("outside the window executables must be dropped: %+v", closed)
	}
	if len(advisories(closed, policy.AdvisoryLWSMigrationUnsupported)) != 1 {
		t.Fatalf("advisories must survive a closed window: %+v", closed)
	}

	cfg.MaintenanceWindows = []config.MaintenanceWindow{{Days: []string{"Thu"}, Start: "09:00", End: "17:00"}}
	if open := evaluate(t, build(), cfg); len(executables(open)) != 1 {
		t.Fatalf("inside the window the move must dispatch: %+v", open)
	}

	// An emergency — a pod starved past emergencyPendingAgeMinutes —
	// overrides a closed window: a steady-state optimization can wait, a
	// starving workload cannot. The age threshold is pinned explicitly so
	// the assertion does not lean on the default.
	cfg.MaintenanceWindows = []config.MaintenanceWindow{{Days: []string{"Mon"}, Start: "09:00", End: "17:00"}}
	cfg.EmergencyPendingAgeMinutes = 15
	starving := consolidationBuilder().WithPendingPodIn("prod", 8, 30*time.Minute, "h100")
	emergency := executables(evaluate(t, starving.Build(), cfg))
	if len(emergency) != 1 || !emergency[0].Emergency {
		t.Fatalf("an emergency must override the closed window: %+v", emergency)
	}
}

// TestModelReadinessFiltersTargets: per-node models restrict hints to nodes
// whose readiness label is set — migrating onto a node that must first pull
// a multi-hundred-GB model defeats the purpose.
func TestModelReadinessFiltersTargets(t *testing.T) {
	b := testutil.NewSnapshot().
		WithNode("node1", "h100", 8).
		WithNode("node2", "h100", 8).
		WithNode("node3", "h100", 8).
		WithInstance("prod/mover", v1beta1.EngineComponent, constants.RawDeployment, "node1", 1)
	b.WithOtherOccupant("node2", 7)
	b.WithOtherOccupant("node3", 7)
	b.WithModel("prod/mover", &snapshot.ModelAvailability{
		Key:        snapshot.ModelKey{Kind: snapshot.ModelKindBaseModel, Namespace: "prod", Name: "llm"},
		Backend:    snapshot.BackendPerNode,
		NodesReady: []string{"node1", "node3"},
	})
	cfg := lowGate()
	*cfg.Policies.Defragmentation.FragmentationThreshold = 0.05

	execs := executables(evaluate(t, b.Build(), cfg))
	if len(execs) != 1 {
		t.Fatalf("want the mover candidate, got %+v", execs)
	}
	if len(execs[0].HintTargetNodes) != 1 || execs[0].HintTargetNodes[0] != "node3" {
		t.Fatalf("hints = %v, want [node3] (node2 lacks the model)", execs[0].HintTargetNodes)
	}
}

// TestRankingPrefersScoreThenSmallerFootprint: candidates order by score
// descending; equal scores break toward the smaller footprint.
func TestRankingPrefersScoreThenSmallerFootprint(t *testing.T) {
	cands := []policy.Candidate{
		{Workload: types.NamespacedName{Namespace: "prod", Name: "b"}, Score: 0.1, FootprintGPUs: 8},
		{Workload: types.NamespacedName{Namespace: "prod", Name: "a"}, Score: 0.1, FootprintGPUs: 2},
		{Workload: types.NamespacedName{Namespace: "prod", Name: "c"}, Score: 0.3, FootprintGPUs: 8},
	}
	rankCandidates(cands)
	if cands[0].Workload.Name != "c" || cands[1].FootprintGPUs != 2 || cands[2].FootprintGPUs != 8 {
		t.Fatalf("ranking order wrong: %+v", cands)
	}
}
