package coordination

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/irprojector"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/v1beta1convert"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
	"sigs.k8s.io/ome/pkg/utils"
)

// The batch step paces a rollout group that lists both the engine and the
// decoder: each member's starts are counted in batches of its step, and no
// member opens batch b+1 until every member's batch b serves.
//
// A drain-first member (RecreatePod, or in-place on a multi-pod gang) steps
// on drains by its maxUnavailable budget; a SurgeThenDrain member steps on
// surges by its maxSurge budget, both as the budget gates resolve them.
//
// With 12 prefill beside 8 decode, both surging under the default budget,
// batch 0 creates 3 prefill and 2 decode, and the prefill's fourth surge
// waits until the 2 new decode serve. A hand-over is never a start.
//
// A pairing member that patches single pods in place leaves the other gates
// in charge. A router joins when it steps the way a pairing member does.
// Governed members are not consulted against the serving band.
//
// One wait is waived: a wait on a pairing peer the pairing gate holds still
// (EvaluateUpdateGate), since that peer moves only once this member has: a
// peer whose open attempts are all hand-overs that gate holds, or one with
// no attempt open whose every start that gate holds and this member's
// start releases. A peer short of serving pods of its own is waited on.
//
// Two operator settings layer on this (GroupDefaults.PrefillDecode). Under
// keepRatio a pairing member that patches single pods in place steps on
// its maxUnavailable like a drain-first one, and the engine's and decoder's
// batches are planned together so the share of each side started toward
// the new version stays within the group's tolerance of the other side's
// (pairedBatchPlan). When firstSide names a side, the other side's starts
// of a batch wait until the named side's new Instances of that batch serve.
//
// A member's batches cover the Instances the roll may move: its count less
// the Instances its partition holds on their revision, read with the same
// rule the roll holds them by. A member whose movable Instances have all
// served is done, whatever its partition keeps back, and no peer waits on
// it; a partition lowered later starts the held Instances as a new batch.
//
// A peer's roll target is read off its own status, which a push reaches
// one status write after the spec. A peer whose status was not written
// for the spec it holds, or whose spec was projected from another service
// generation than the acting member's, may still record the previous
// target, with the Instances of the previous push promoted on it. Such a
// peer is read as not yet observed toward the push: nothing started, its
// count and step from its spec, so a wait on it is a wait on its first
// batch until its status records the push (peerStatusPredatesPush).

// batchMember is one member of the group as the batch step reads its
// replica: its desired count, the Instances its partition holds, its step,
// its planned batch marks when the ratio rule planned them, and its
// movable Instances by what the roll has done to them.
type batchMember struct {
	component v1beta1.ComponentType
	desired   int32
	held      int32 // Instances the partition keeps on their revision; the roll never moves them
	step      int32
	marks     []int32 // the count started after each planned batch; nil means batches of step
	promoted  int32   // Instances running the target revision
	open      int32   // Instances with an update attempt toward the target in flight
	ended     int32   // Instances whose attempt toward the target ended without serving
}

// movable is the number of Instances the roll may move: the desired count
// less the Instances the partition holds.
func (m batchMember) movable() int32 {
	if m.held >= m.desired {
		return 0
	}
	return m.desired - m.held
}

// started is the number of starts the member has opened, counted from zero:
// every Instance on the target, moving to it, or whose move ended. The
// starts of the current wake-up are among them, since each is stamped on
// its row, where the gate reads, before the next start is consulted.
func (m batchMember) started() int32 { return m.promoted + m.open + m.ended }

// settled is what a peer may stop waiting on: Instances serving on the
// target, plus attempts that ended, which the escalation reports and the
// retry ladder reopens as a fresh start that counts as open again.
func (m batchMember) settled() int32 { return m.promoted + m.ended }

// mark is the cumulative count batch b should have started: the planned
// mark when the ratio rule planned the batches, else b steps, either way
// capped at the movable count.
func (m batchMember) mark(b int32) int32 {
	if b <= 0 {
		return 0
	}
	movable := m.movable()
	if m.marks != nil {
		if int(b) > len(m.marks) {
			return movable
		}
		return min32(m.marks[b-1], movable)
	}
	if t := b * m.step; t < movable {
		return t
	}
	return movable
}

// nextBatch is the batch the member's next start belongs to, counted from
// one: the first batch whose mark its starts have not reached. A member
// with every batch started is past its last.
func (m batchMember) nextBatch() int32 {
	started, movable := m.started(), m.movable()
	for b := int32(1); ; b++ {
		mark := m.mark(b)
		if mark > started {
			return b
		}
		if mark >= movable {
			return b + 1
		}
	}
}

// batchStep is one group's reading for one start: the members in group
// order, the acting member's position among them, and the pairing side
// that moves first inside each batch, empty when both start together.
// governs is false when the ratio and budget gates decide on their own.
type batchStep struct {
	governs   bool
	acting    int
	members   []batchMember
	firstSide v1beta1.ComponentType
}

// stepKind is what a member counts its starts in: drains for a drain-first
// member, surges for a SurgeThenDrain member, nothing for one that patches
// single pods in place or has no replica.
type stepKind int

const (
	stepNone stepKind = iota
	stepDrain
	stepSurge
)

// stepKindOf reads a replica's step kind off its declared update strategy;
// an unset strategy is the SurgeThenDrain default.
func stepKindOf(ir *v1beta1.InferenceReplica) stepKind {
	switch {
	case ir == nil:
		return stepNone
	case drainsFirst(ir):
		return stepDrain
	case surgeStrategy(ir):
		return stepSurge
	}
	return stepNone
}

// readBatchStep reads the group's replicas and decides whether the batch
// step governs this start: the group rolls its members together (not
// Sequential), lists both engine and decoder, both of them step, and the
// acting member is a batched member with a step to count in. A member of
// another kind is batched when it steps the way some pairing member does and
// is left out otherwise. Under keepRatio a pairing member that patches
// single pods in place steps on drains too, and the pairing members' marks
// are planned to the group's tolerance. A read error is returned for the
// caller to fail closed on.
func (ctx GateContext) readBatchStep() (batchStep, error) {
	if ctx.ShortCircuit || ctx.Group.Policy == v1beta1.CoordinationPolicySequential || !groupPairsEngineAndDecoder(ctx.Group) {
		return batchStep{}, nil
	}
	rules := ctx.Defaults.PrefillDecode
	type read struct {
		comp v1beta1.ComponentType
		ir   *v1beta1.InferenceReplica
		kind stepKind
	}
	reads := make([]read, 0, len(ctx.Group.Components))
	pairingKinds := map[stepKind]bool{}
	for _, comp := range ctx.Group.Components {
		ir, _, err := irprojector.DecodedComponentIRFor(ctx.Ctx, ctx.Reads, ctx.ISVC, comp)
		if err != nil {
			return batchStep{}, fmt.Errorf("cannot read %s IR status, failing closed: %w", comp, err)
		}
		kind := stepKindOf(ir)
		if comp == v1beta1.EngineComponent || comp == v1beta1.DecoderComponent {
			if kind == stepNone && ir != nil && rules.KeepRatio {
				kind = stepDrain
			}
			if kind == stepNone {
				return batchStep{}, nil
			}
			pairingKinds[kind] = true
		}
		reads = append(reads, read{comp: comp, ir: ir, kind: kind})
	}
	step := batchStep{acting: -1, firstSide: rules.namedFirstSide()}
	var acting *v1beta1.InferenceReplica
	for _, r := range reads {
		if r.comp == ctx.Component {
			acting = r.ir
		}
	}
	pair := map[v1beta1.ComponentType]int{}
	for _, r := range reads {
		if !pairingKinds[r.kind] {
			continue
		}
		target := rollTarget(r.ir)
		if r.comp == ctx.Component {
			step.acting = len(step.members)
			if ctx.TargetRevision != nil {
				target = ctx.TargetRevision.Name
			}
		}
		if r.comp == v1beta1.EngineComponent || r.comp == v1beta1.DecoderComponent {
			pair[r.comp] = len(step.members)
		}
		member := readBatchMember(r.ir, r.comp, target, ctx.Group.Pacing, r.kind)
		if r.comp != ctx.Component && peerStatusPredatesPush(r.ir, acting) {
			member = member.notYetObserved()
		}
		step.members = append(step.members, member)
	}
	if step.acting < 0 || step.members[step.acting].step <= 0 {
		return batchStep{}, nil
	}
	if rules.KeepRatio {
		if tolerance := ctx.batchTolerance(); tolerance != nil {
			engine, decoder := &step.members[pair[v1beta1.EngineComponent]], &step.members[pair[v1beta1.DecoderComponent]]
			engine.marks, decoder.marks = pairedBatchPlan(engine.movable(), decoder.movable(), engine.step, decoder.step, *tolerance)
		}
	}
	step.governs = true
	return step, nil
}

// batchTolerance is the tolerance the ratio rule sizes batches to: the
// group's maintainRatio tolerance, else the operator's configured default.
// Nil leaves each side on its own step.
func (ctx GateContext) batchTolerance() *int32 {
	if t := ctx.Group.Pacing.RatioTolerancePercent; t != nil {
		return t
	}
	return ctx.Defaults.RatioTolerancePercent
}

// hold answers the acting member's next start, which belongs to batch b:
// every member's batch b-1 but waived's must have settled first, and the
// first short, peers before self, names the hold. When a first side is
// named, a start of the other pairing side also waits until the named
// side's batch b has settled.
func (s batchStep) hold(waived v1beta1.ComponentType) (held bool, reason string) {
	me := s.members[s.acting]
	b := me.nextBatch()
	if b > 1 {
		order := make([]int, 0, len(s.members))
		for i := range s.members {
			if i != s.acting {
				order = append(order, i)
			}
		}
		order = append(order, s.acting)
		for _, i := range order {
			x := s.members[i]
			if x.component == waived {
				continue
			}
			if mark := x.mark(b - 1); x.settled() < mark {
				return true, fmt.Sprintf("%s batch %d waits for %s batch %d to serve (%d of %d serving%s%s)",
					me.component, b, x.component, b-1, x.promoted, mark, failedNote(x), movableNote(x))
			}
		}
	}
	if s.firstSide != "" && me.component == pairingPeer(s.firstSide) {
		for _, x := range s.members {
			if x.component != s.firstSide || x.component == waived {
				continue
			}
			if mark := x.mark(b); x.settled() < mark {
				return true, fmt.Sprintf("%s batch %d waits for %s batch %d to serve first (%d of %d serving%s%s)",
					me.component, b, x.component, b, x.promoted, mark, failedNote(x), movableNote(x))
			}
		}
	}
	return false, ""
}

// failedNote names a member's ended attempts in a hold reason.
func failedNote(m batchMember) string {
	if m.ended > 0 {
		return fmt.Sprintf(", %d failed", m.ended)
	}
	return ""
}

// movableNote names the Instances a member's partition lets the roll move
// in a hold reason, when it holds any.
func movableNote(m batchMember) string {
	if m.held > 0 {
		return fmt.Sprintf(", %d of %d movable", m.movable(), m.desired)
	}
	return ""
}

// readBatchMember classifies a replica's Instances against its roll target.
// The step is the member's own budget of its kind capped by the group's, as
// the budget gates resolve it. An Instance the partition holds is counted
// as held and nowhere else. A member with no roll target has nothing to
// move and reads as settled.
func readBatchMember(ir *v1beta1.InferenceReplica, comp v1beta1.ComponentType, target string, pacing v1beta1.CoordinationPacing, kind stepKind) batchMember {
	m := batchMember{component: comp}
	if kind == stepSurge {
		m.desired, m.step = resolveSurgeStep(ir, pacing)
	} else {
		floor := resolvePairFloor(ir, pacing)
		m.desired, m.step = floor.desired, floor.maxUnavailable
	}
	if target == "" {
		m.promoted = m.desired
		return m
	}
	rows := ir.Status.InstanceStatuses
	held := partitionHeldRows(ir, rows, target)
	for i := range rows {
		if held[rows[i].Index] {
			m.held++
			continue
		}
		switch classifyBatchRow(&rows[i], target) {
		case batchRowPromoted:
			m.promoted++
		case batchRowOpen:
			m.open++
		case batchRowEnded:
			m.ended++
		}
	}
	return m
}

// partitionHeldRows is the set of a replica's Instances its partition holds
// on their revision, read as the roll reads it: the effective partition off
// the replica's spec (irprojector.IRPartition) and the lowest-indexed
// Instances still off the target that no attempt is moving
// (query.PartitionHeldIndices), each row decoded as the engine holds it.
func partitionHeldRows(ir *v1beta1.InferenceReplica, rows []v1beta1.OMENativeInstanceStatus, target string) map[int32]bool {
	partition := irprojector.IRPartition(ir)
	if partition <= 0 {
		return nil
	}
	observed := make([]*workloadtypes.InstanceStatus, 0, len(rows))
	for i := range rows {
		s := v1beta1convert.InstanceStatusToWorkload(rows[i])
		observed = append(observed, &s)
	}
	return query.PartitionHeldIndices(&partition, observed, target)
}

// resolveSurgeStep is a surging member's step: its own rollingUpdate.maxSurge
// against its desired count, capped by the group's surge budget, the way the
// surge gate resolves it. Zero when the replica records no count.
func resolveSurgeStep(ir *v1beta1.InferenceReplica, pacing v1beta1.CoordinationPacing) (desired, step int32) {
	desired = effectiveDesiredReplicas(ir)
	if desired <= 0 {
		return 0, 0
	}
	step = MaxSurgeBudget(pacing, desired)
	if lc := ir.Spec.Lifecycle; lc != nil && lc.UpdateStrategy != nil && lc.UpdateStrategy.RollingUpdate != nil && lc.UpdateStrategy.RollingUpdate.MaxSurge != nil {
		if own := utils.ScaledCountFromIntOrString(lc.UpdateStrategy.RollingUpdate.MaxSurge, desired, true); own < step {
			step = own
		}
	}
	return desired, step
}

// effectiveDesiredReplicas is the count the replica's workload runs toward:
// spec.replicas, held at the placement replica limit while a placement surge
// pause defers growth, the cap desiredFromIR applies. An autoscaler can raise
// spec.replicas above the limit, and no Instance is created for the excess,
// so a batch counted against it could never settle. Zero when unresolved.
func effectiveDesiredReplicas(ir *v1beta1.InferenceReplica) int32 {
	if ir == nil || ir.Spec.Replicas == nil || *ir.Spec.Replicas <= 0 {
		return 0
	}
	desired := *ir.Spec.Replicas
	if policy := ir.Spec.PlacementExecution; policy != nil && policy.PauseSurge && ir.Spec.PlacementReplicaLimit != nil {
		desired = min(desired, *ir.Spec.PlacementReplicaLimit)
	}
	return max(desired, 0)
}

type batchRowClass int

const (
	batchRowUntouched batchRowClass = iota
	batchRowPromoted
	batchRowOpen
	batchRowEnded
)

// classifyBatchRow reads what the roll has done to one Instance. It is
// promoted when it runs the target with no attempt pinned elsewhere, the
// reading of the UpdatedReplicas counter. An attempt toward the target stays
// on the row from its first stamp to the promote: a Failed row or a parked
// attempt ended without serving, any other is open. The phase is read as
// the engine holds it (v1beta1convert.InstanceStatusToWorkload): the status
// publication writes Ready over a failed replacement while the old pod it
// was to replace still serves, and that attempt ended all the same. A
// Create or Migrate on the target is a new Instance, not a start of this
// roll, and the slot a gang surge builds its replacement in belongs to the
// source row's start.
func classifyBatchRow(row *v1beta1.OMENativeInstanceStatus, target string) batchRowClass {
	if target == "" {
		return batchRowUntouched
	}
	op := row.Operation
	update := op != nil && op.Type == v1beta1.InstanceOperationUpdate
	if update && (op.Step == workloadtypes.UpdateStepGangSurgeTarget || op.Step == workloadtypes.UpdateStepGangSurgeTargetCleanup) {
		return batchRowUntouched
	}
	pinnedElsewhere := update && op.TargetRevision != "" && op.TargetRevision != target
	if row.RunningRevision == target && !pinnedElsewhere {
		return batchRowPromoted
	}
	toward := row.TargetRevision == target || (update && op.TargetRevision == target)
	if !toward || (op != nil && !update) {
		return batchRowUntouched
	}
	phase := v1beta1convert.InstanceStatusToWorkload(*row).Phase
	if phase == workloadtypes.InstancePhaseFailed || (update && op.Step == workloadtypes.UpdateStepParked) {
		return batchRowEnded
	}
	return batchRowOpen
}

// drainsFirst reports whether a replica's update takes an Instance out of
// rotation before anything returns: RecreatePod, or an in-place strategy on
// a multi-pod Instance, which rebuilds the whole gang. An unset strategy is
// the SurgeThenDrain default; a shape with no stored runners reads as
// single-pod.
func drainsFirst(ir *v1beta1.InferenceReplica) bool {
	if ir.Spec.Lifecycle == nil || ir.Spec.Lifecycle.UpdateStrategy == nil {
		return false
	}
	switch ir.Spec.Lifecycle.UpdateStrategy.Type {
	case v1beta1.UpdateStrategyRecreatePod:
		return true
	case v1beta1.UpdateStrategyInPlaceIfPossible, v1beta1.UpdateStrategyInPlaceOnly:
		var pods int32
		for _, r := range ir.Spec.Runners {
			pods += r.Size
		}
		return pods > 1
	}
	return false
}

// rollTarget is the revision a replica moves its Instances to: the rollback
// revision while one is pinned, else its update revision.
func rollTarget(ir *v1beta1.InferenceReplica) string {
	if p := ir.Spec.Pacing; p != nil && p.RollbackToRevision != nil && *p.RollbackToRevision != "" {
		return *p.RollbackToRevision
	}
	return ir.Status.UpdateRevision
}

// peerStatusPredatesPush reports a peer replica whose recorded roll target
// may still be the previous push's: its status was not written for the
// pod templates its spec holds (StatusObservedTemplates), or its spec was
// projected from another service generation than the acting replica's
// (the parent-generation record the projector writes on both). A spec
// write that changes no template leaves the peer current. Two replicas
// with no record are compared on their status alone. A missing peer
// predates nothing.
func peerStatusPredatesPush(peer, acting *v1beta1.InferenceReplica) bool {
	if peer == nil {
		return false
	}
	if !StatusObservedTemplates(peer) {
		return true
	}
	if acting == nil {
		return false
	}
	peerGeneration, peerRecorded := peer.Annotations[constants.InferenceReplicaParentGenerationAnnotationKey]
	actingGeneration, actingRecorded := acting.Annotations[constants.InferenceReplicaParentGenerationAnnotationKey]
	return peerRecorded && actingRecorded && peerGeneration != actingGeneration
}

// notYetObserved is the member as the batch step reads it while its status
// predates the push: its count and step stand, nothing is started toward
// the target, and no Instance reads held, so a peer waits on its first
// batch until the status records the push.
func (m batchMember) notYetObserved() batchMember {
	return batchMember{component: m.component, desired: m.desired, step: m.step}
}
