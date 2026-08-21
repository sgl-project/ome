package defrag

import (
	"sort"
	"time"

	"sigs.k8s.io/ome/pkg/alfred/config"
	"sigs.k8s.io/ome/pkg/alfred/policy"
	"sigs.k8s.io/ome/pkg/alfred/snapshot"
	"sigs.k8s.io/ome/pkg/constants"
)

// PolicyName is the metric/event label value for Policy #1.
const PolicyName = "defragmentation"

// Disruption cost per migration mode (same unit as Benefit — a share of
// demand-weighted fragmentation). The OEP fixes the ordering (eviction,
// rolling restart, and OMENative rolling are low; OMENative surge is
// medium); the numeric scale is an implementation choice.
const (
	costTargetedEviction = 0.05
	costRollingRestart   = 0.05
	costOMENativeRolling = 0.05
	costOMENativeSurge   = 0.15
)

// emergencyBoostFactor multiplies a positive Score when the move unblocks an
// over-age pending pod — what lets a starving workload jump the queue ahead
// of routine consolidation.
const emergencyBoostFactor = 2.0

// spotSourceBoostFactor multiplies a positive Score when the source node is
// preemptible and spot policy prefers evacuating such nodes before a
// preemption event does.
const spotSourceBoostFactor = 1.25

// costWeight realizes the aggressiveness scoring knob: conservative demands
// more benefit per unit of disruption, aggressive tolerates more. Never a
// safety bypass — cooldowns, caps, and the breaker live in the Arbiter.
func costWeight(aggressiveness string) float64 {
	switch aggressiveness {
	case config.AggressivenessConservative:
		return 1.5
	case config.AggressivenessAggressive:
		return 0.5
	default:
		return 1.0
	}
}

// Policy is Policy #1 (Defragmentation): pure candidate selection over the
// snapshot. It holds no client; the engine routes its output.
type Policy struct{}

var _ policy.Policy = &Policy{}

// Name implements policy.Policy.
func (*Policy) Name() string { return PolicyName }

// evalCtx carries the per-pool evaluation state shared by every candidate.
type evalCtx struct {
	snap       *snapshot.ClusterSnapshot
	cfg        *config.Config
	pool       string
	bins       []binState
	ladder     []int64
	weights    map[int64]float64
	totalFree  int64
	before     float64
	pendings   []snapshot.PendingPod
	costWeight float64
}

// Evaluate turns the snapshot into a ranked []Candidate (OEP-0008
// §Candidate selection). Gate → enumerate → classify → simulate → score →
// boost → rank → filter.
func (*Policy) Evaluate(snap *snapshot.ClusterSnapshot, cfg *config.Config) []policy.Candidate {
	d := &cfg.Policies.Defragmentation
	if !*d.Enabled {
		return nil
	}
	scores := ComputeScores(snap, cfg)
	threshold := *d.FragmentationThreshold
	if scores.FragmentationScore <= threshold {
		return nil
	}

	ladder := int64Ladder(d.Scoring.SizeLadder)
	prior := parsePrior(d.Scoring.SizePrior)
	lambda := *d.Scoring.DemandBlendLambda
	demand := demandByPoolAndSize(snap, ladder)
	weight := costWeight(d.Aggressiveness)
	now := snap.Timestamp

	var out []policy.Candidate
	for _, pool := range snap.GPUPools() {
		cs := scores.PerPool[pool]
		// Per-pool gate: you defragment the worst pool — a healthy
		// pool's near-zero-benefit moves would be pure churn. A pool
		// with a starving fixable pending is above threshold through
		// the noisy-OR pressure term.
		if cs == nil || cs.Score <= threshold {
			continue
		}
		ctx := &evalCtx{
			snap:       snap,
			cfg:        cfg,
			pool:       pool,
			bins:       schedulableBins(snap, cfg, pool),
			ladder:     ladder,
			weights:    demandWeights(ladder, demand[pool], prior, lambda),
			totalFree:  cs.TotalFree,
			pendings:   poolPendings(snap, pool),
			costWeight: weight,
		}
		ctx.before = weightedFrag(ctx.bins, ladder, ctx.weights, ctx.totalFree)

		for _, w := range sortedWorkloads(snap) {
			// Step-1 enumeration filters: movable, no in-flight
			// migration, outside the per-workload cooldown. (The
			// Arbiter re-enforces its own bounds on every policy's
			// output; these pre-filters shape the candidate set.)
			if !w.Movable || len(w.ActiveMigrations) > 0 || inCooldown(w, cfg, now) {
				continue
			}
			for _, comp := range sortedComponents(w) {
				out = append(out, evaluateComponent(ctx, w, comp)...)
			}
		}
	}

	// Maintenance windows gate defragmentation dispatch: outside every
	// window, executable candidates are dropped; advisories carry no
	// dispatch and survive. (Node-health evacuation deliberately ignores
	// windows — that policy applies its own rules.)
	if !maintenanceOpen(cfg, now) {
		kept := out[:0]
		for _, c := range out {
			if !c.Executable {
				kept = append(kept, c)
			}
		}
		out = kept
	}

	rankCandidates(out)
	return out
}

// evaluateComponent classifies one component by deployment mode (the
// classification sets Executable, it does not drop the candidate) and
// evaluates its instances.
func evaluateComponent(ctx *evalCtx, w *snapshot.Workload, comp *snapshot.Component) []policy.Candidate {
	switch comp.DeploymentMode {
	case constants.MultiNode:
		// LWS-backed groups are unsafe to migrate automatically
		// (RecreateGroupOnPodRestart tears the group down with no surge
		// protection) but the opportunity must still surface.
		if !*ctx.cfg.LWSRecommendationsEnabled {
			return nil
		}
		if componentPrimaryPool(ctx.snap, comp) != ctx.pool {
			return nil
		}
		return []policy.Candidate{advisory(w, comp, policy.ComponentWideInstance,
			policy.AdvisoryLWSMigrationUnsupported, componentPrimaryNode(comp), largestInstanceGPUs(comp))}
	case constants.RawDeployment, constants.OMENative:
		// Executable surfaces, handled below.
	default:
		return nil
	}
	if componentPrimaryPool(ctx.snap, comp) != ctx.pool {
		return nil
	}

	// Component-wide downgrades, most fundamental first; one advisory, not
	// a stack.
	from := componentPrimaryNode(comp)
	footprint := largestInstanceGPUs(comp)
	if avail := modelOf(ctx.snap, w); avail != nil {
		if avail.ResolveError != "" {
			return []policy.Candidate{advisory(w, comp, policy.ComponentWideInstance,
				policy.AdvisoryModelUnresolved, from, footprint)}
		}
		if avail.VolumePinned {
			// An RWO/RWOP volume attaches to one node at a time and
			// the source still holds it while any replacement
			// starts; eviction would only recreate the pod on the
			// same node.
			return []policy.Candidate{advisory(w, comp, policy.ComponentWideInstance,
				policy.AdvisoryVolumePinned, from, footprint)}
		}
	}
	if comp.DeploymentMode == constants.OMENative {
		if !*ctx.cfg.OMENativeMigrationEnabled {
			return []policy.Candidate{advisory(w, comp, policy.ComponentWideInstance,
				policy.AdvisoryMigrationSurfaceDisabled, from, footprint)}
		}
		if !ctx.snap.OMENativeAvailable {
			return []policy.Candidate{advisory(w, comp, policy.ComponentWideInstance,
				policy.AdvisoryOMENativeUnavailable, from, footprint)}
		}
	} else if !*ctx.cfg.RawDeploymentMigrationEnabled {
		return []policy.Candidate{advisory(w, comp, policy.ComponentWideInstance,
			policy.AdvisoryMigrationSurfaceDisabled, from, footprint)}
	}

	ready := readyInstances(comp)
	instances := append([]*snapshot.Instance(nil), comp.Instances...)
	sort.Slice(instances, func(i, j int) bool { return instances[i].Index < instances[j].Index })

	var out []policy.Candidate
	for _, inst := range instances {
		// Terminating instances are the Arbiter's exclusion; skipping
		// here is the OEP-sanctioned optimization.
		if inst.TotalGPUs == 0 || hasTerminating(inst) || !instanceInPool(ctx.snap, inst, ctx.pool) {
			continue
		}
		if c, ok := evaluateInstance(ctx, w, comp, inst, ready); ok {
			out = append(out, c)
		}
	}
	return out
}

// evaluateInstance simulates one prospective move mode-aware (place-then-free
// for surge shapes, free-then-place only for multi-replica eviction), scores
// it benefit-minus-cost, and applies the emergency and spot-source boosts.
func evaluateInstance(ctx *evalCtx, w *snapshot.Workload, comp *snapshot.Component,
	inst *snapshot.Instance, ready int) (policy.Candidate, bool) {

	prints := instanceFootprints(inst)
	if len(prints) == 0 {
		return policy.Candidate{}, false
	}
	from := primaryNode(inst)
	// OMENative migrates an Instance by surging it regardless of replica
	// count; RawDeployment surges only when evicting the sole ready
	// replica would be an outage. The controller re-branches at execution
	// time against live state; the simulation predicts from the snapshot.
	surgeShaped := comp.DeploymentMode == constants.OMENative || ready <= 1

	exclude := make(map[string]bool, len(inst.NodesSet))
	for node := range inst.NodesSet {
		exclude[node] = true
	}
	// Rank targets that fit the SMALLEST member pod: a multi-pod
	// instance's pods land on different targets, and excluding nodes that
	// only fit the smaller pods would fabricate NoSurgeHeadroom.
	// placeThenFree re-checks each footprint's size per target; for
	// single-pod instances min and max coincide.
	minPod := prints[len(prints)-1].gpus
	ranked := rankTargets(ctx.snap, ctx.cfg, ctx.bins, w, minPod, exclude)

	var after []binState
	if surgeShaped {
		bins, _, ok := placeThenFree(ctx.bins, prints, ranked)
		if !ok {
			// Not dispatchable: without surge headroom the migration
			// would stall in SurgePending until timeout. Surface
			// "wants to move, no room" instead.
			c := advisory(w, comp, inst.Index, policy.AdvisoryNoSurgeHeadroom, from, inst.TotalGPUs)
			c.SurgeShaped = true
			return c, true
		}
		after = bins
	} else {
		bins, target := freeThenPlace(ctx.bins, prints[0], ranked)
		if target == "" {
			// Nowhere to land even once the source frees its GPUs
			// (a full pool, or a model/spot constraint that empties
			// the ranking): evicting would strand the replacement
			// in Pending for no gain. Surface, do not dispatch.
			return advisory(w, comp, inst.Index, policy.AdvisoryNoFeasibleTarget, from, inst.TotalGPUs), true
		}
		after = bins
	}

	benefit := ctx.before - weightedFrag(after, ctx.ladder, ctx.weights, ctx.totalFree)
	cost := modeCost(comp.DeploymentMode, len(comp.Instances), ready)
	score := benefit - ctx.costWeight*cost

	emergency := unblocksOverAgePending(ctx, w, after)
	if emergency && score > 0 {
		score *= emergencyBoostFactor
	}
	if score > 0 && spotPrefersSource(w, ctx.cfg) {
		if node := ctx.snap.Nodes[from]; node != nil && node.Preemptible {
			score *= spotSourceBoostFactor
		}
	}

	hints := ranked
	if len(hints) > maxHintTargets {
		hints = hints[:maxHintTargets]
	}
	return policy.Candidate{
		Policy:          PolicyName,
		Workload:        w.NamespacedName,
		Component:       comp.Type,
		Instance:        inst.Index,
		Mode:            comp.DeploymentMode,
		Reason:          policy.ReasonFragmentation,
		FromNode:        from,
		HintTargetNodes: append([]string(nil), hints...),
		Executable:      true,
		SurgeShaped:     surgeShaped,
		FootprintGPUs:   inst.TotalGPUs,
		Benefit:         benefit,
		Cost:            cost,
		Score:           score,
		Emergency:       emergency,
	}, true
}

// modeCost prices disruption per the OEP's cost table.
func modeCost(mode constants.DeploymentModeType, instanceCount, ready int) float64 {
	if mode == constants.OMENative {
		if instanceCount == 1 {
			return costOMENativeSurge
		}
		return costOMENativeRolling
	}
	if ready <= 1 {
		return costRollingRestart
	}
	return costTargetedEviction
}

// unblocksOverAgePending reports whether the simulated after-state seats a
// pending pod (real or virtual) that the observed state cannot, aged past
// emergencyPendingAgeMinutes, within the workload's tenant scope.
func unblocksOverAgePending(ctx *evalCtx, w *snapshot.Workload, after []binState) bool {
	minAge := ctx.cfg.EmergencyPendingAge()
	for i := range ctx.pendings {
		p := &ctx.pendings[i]
		if ctx.snap.Timestamp.Sub(p.PendingSince) <= minAge {
			continue
		}
		if !tenantCompatible(ctx.snap, ctx.cfg, w, p) {
			continue
		}
		if !canSeat(ctx.bins, p.GPUsNeeded) && canSeat(after, p.GPUsNeeded) {
			return true
		}
	}
	return false
}

// tenantCompatible enforces the tenant boundary on the emergency boost: a
// move is boosted only by pending demand it may legitimately serve — its own
// namespace, or (when the cluster admin has not vetoed cross-tenant moves) a
// workload sharing its explicit tenant group. Pure consolidation benefit is
// pool-wide and needs no tenant check.
func tenantCompatible(snap *snapshot.ClusterSnapshot, cfg *config.Config, w *snapshot.Workload, p *snapshot.PendingPod) bool {
	if p.Namespace == w.NamespacedName.Namespace {
		return true
	}
	if !*cfg.AllowCrossTenantOptimization || w.TenantGroup == "" {
		return false
	}
	if p.ISVC.Name == "" {
		return false
	}
	owner, ok := snap.Workloads[p.ISVC]
	return ok && owner.TenantGroup == w.TenantGroup
}

// spotPrefersSource resolves the per-workload spot-policy annotation against
// the cluster default for the source-side boost: "migrate" always boosts,
// "ignore" never does, anything else follows spotPolicy.preferAsSource.
func spotPrefersSource(w *snapshot.Workload, cfg *config.Config) bool {
	switch w.SpotPolicy {
	case "migrate":
		return true
	case "ignore":
		return false
	default:
		return *cfg.SpotPolicy.PreferAsSource
	}
}

// inCooldown applies the per-workload cooldown from the newest terminal
// migration, honoring the alfred.ome.io/cooldown-minutes override.
func inCooldown(w *snapshot.Workload, cfg *config.Config, now time.Time) bool {
	if w.LastMigration == nil {
		return false
	}
	cooldown := cfg.PerWorkloadCooldown()
	if w.CooldownOverride != nil {
		cooldown = *w.CooldownOverride
	}
	return now.Sub(*w.LastMigration) < cooldown
}

var weekdayNames = map[time.Weekday]string{
	time.Monday: "Mon", time.Tuesday: "Tue", time.Wednesday: "Wed",
	time.Thursday: "Thu", time.Friday: "Fri", time.Saturday: "Sat", time.Sunday: "Sun",
}

// maintenanceOpen reports whether defragmentation may dispatch now: no
// windows means always, otherwise inside at least one weekly UTC window.
// Window syntax is validated at config load.
func maintenanceOpen(cfg *config.Config, now time.Time) bool {
	if len(cfg.MaintenanceWindows) == 0 {
		return true
	}
	utc := now.UTC()
	day := weekdayNames[utc.Weekday()]
	minute := utc.Hour()*60 + utc.Minute()
	for _, w := range cfg.MaintenanceWindows {
		dayMatch := false
		for _, d := range w.Days {
			if d == day {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			continue
		}
		start, _ := time.Parse("15:04", w.Start)
		end, _ := time.Parse("15:04", w.End)
		if minute >= start.Hour()*60+start.Minute() && minute < end.Hour()*60+end.Minute() {
			return true
		}
	}
	return false
}

// rankCandidates orders by FinalScore descending, breaking ties toward the
// smaller footprint — smaller moves fit more often, disrupt less, and each
// completion frees GPUs that make larger moves feasible later. Remaining
// ties order deterministically by identity.
func rankCandidates(out []policy.Candidate) {
	sort.SliceStable(out, func(i, j int) bool {
		a, b := &out[i], &out[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.FootprintGPUs != b.FootprintGPUs {
			return a.FootprintGPUs < b.FootprintGPUs
		}
		if a.Workload.String() != b.Workload.String() {
			return a.Workload.String() < b.Workload.String()
		}
		if a.Component != b.Component {
			return a.Component < b.Component
		}
		return a.Instance < b.Instance
	})
}

func advisory(w *snapshot.Workload, comp *snapshot.Component, instance int32, reason, from string, footprint int64) policy.Candidate {
	return policy.Candidate{
		Policy:         PolicyName,
		Workload:       w.NamespacedName,
		Component:      comp.Type,
		Instance:       instance,
		Mode:           comp.DeploymentMode,
		Reason:         policy.ReasonFragmentation,
		FromNode:       from,
		Executable:     false,
		AdvisoryReason: reason,
		FootprintGPUs:  footprint,
	}
}

// --- snapshot traversal helpers (deterministic ordering throughout) ---

func sortedWorkloads(snap *snapshot.ClusterSnapshot) []*snapshot.Workload {
	out := make([]*snapshot.Workload, 0, len(snap.Workloads))
	for _, w := range snap.Workloads {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].NamespacedName.String() < out[j].NamespacedName.String()
	})
	return out
}

func sortedComponents(w *snapshot.Workload) []*snapshot.Component {
	out := make([]*snapshot.Component, 0, len(w.Components))
	for _, comp := range w.Components {
		out = append(out, comp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out
}

func modelOf(snap *snapshot.ClusterSnapshot, w *snapshot.Workload) *snapshot.ModelAvailability {
	if w.ModelKey.Zero() {
		return nil
	}
	return snap.Models[w.ModelKey]
}

func readyInstances(comp *snapshot.Component) int {
	count := 0
	for _, inst := range comp.Instances {
		if inst.ReadyPods > 0 {
			count++
		}
	}
	return count
}

func hasTerminating(inst *snapshot.Instance) bool {
	for _, pod := range inst.Pods {
		if pod.Terminating {
			return true
		}
	}
	return false
}

// instanceInPool reports whether every member pod sits on a node of the
// pool; instances straddling pools (or on unknown nodes) are skipped.
func instanceInPool(snap *snapshot.ClusterSnapshot, inst *snapshot.Instance, pool string) bool {
	for node := range inst.NodesSet {
		n, ok := snap.Nodes[node]
		if !ok || n.GPUPool != pool {
			return false
		}
	}
	return len(inst.NodesSet) > 0
}

// primaryNode is the node holding the largest share of the instance's GPUs
// (name tie-break) — the FromNode a migration request would carry.
func primaryNode(inst *snapshot.Instance) string {
	perNode := map[string]int64{}
	for _, pod := range inst.Pods {
		perNode[pod.Node] += pod.GPUs
	}
	best, bestGPUs := "", int64(-1)
	for node, gpus := range perNode {
		if gpus > bestGPUs || (gpus == bestGPUs && node < best) {
			best, bestGPUs = node, gpus
		}
	}
	return best
}

// componentPrimaryNode is primaryNode over all of a component's pods.
func componentPrimaryNode(comp *snapshot.Component) string {
	perNode := map[string]int64{}
	for _, inst := range comp.Instances {
		for _, pod := range inst.Pods {
			perNode[pod.Node] += pod.GPUs
		}
	}
	best, bestGPUs := "", int64(-1)
	for node, gpus := range perNode {
		if gpus > bestGPUs || (gpus == bestGPUs && node < best) {
			best, bestGPUs = node, gpus
		}
	}
	return best
}

// componentPrimaryPool attributes a component to the pool holding the
// largest share of its GPUs (lexicographic tie-break), so component-wide
// advisories are emitted exactly once.
func componentPrimaryPool(snap *snapshot.ClusterSnapshot, comp *snapshot.Component) string {
	perPool := map[string]int64{}
	for _, inst := range comp.Instances {
		for _, pod := range inst.Pods {
			node, ok := snap.Nodes[pod.Node]
			if !ok {
				continue
			}
			perPool[node.GPUPool] += pod.GPUs
		}
	}
	best, bestGPUs := "", int64(-1)
	for pool, gpus := range perPool {
		if gpus > bestGPUs || (gpus == bestGPUs && pool < best) {
			best, bestGPUs = pool, gpus
		}
	}
	return best
}

func largestInstanceGPUs(comp *snapshot.Component) int64 {
	var largest int64
	for _, inst := range comp.Instances {
		if inst.TotalGPUs > largest {
			largest = inst.TotalGPUs
		}
	}
	return largest
}

func poolPendings(snap *snapshot.ClusterSnapshot, pool string) []snapshot.PendingPod {
	var out []snapshot.PendingPod
	for _, p := range snap.PendingPods {
		if p.GPUPool == pool || p.GPUPool == "" {
			out = append(out, p)
		}
	}
	return out
}
