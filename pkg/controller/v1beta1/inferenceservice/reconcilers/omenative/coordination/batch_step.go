package coordination

import (
	"fmt"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/inferenceservice/reconcilers/irprojector"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// The batch step paces a group whose members all roll drain-first
// (RecreatePod, or an in-place strategy on a multi-pod Instance, which
// rebuilds the gang). Each member's drains are counted in batches of its
// step, the maxUnavailable budget the budget gate resolves for it, and no
// member opens batch b+1 until every member's batch b serves. With 6 prefill
// at 3 beside 2 decode at 1 the first batch takes 3 prefill and 1 decode
// together, and the decoder's second drain waits until the 3 new prefill
// serve. While it governs a group it is the members' lockstep, and the
// serving band is not consulted for them.

// batchMember is one member of the group as the batch step reads its
// replica: its desired count, its step, and its Instances by what the roll
// has done to them.
type batchMember struct {
	component v1beta1.ComponentType
	desired   int32
	step      int32
	promoted  int32 // Instances running the target revision
	open      int32 // Instances with an update attempt toward the target in flight
	ended     int32 // Instances whose attempt toward the target ended without serving
}

// started is the number of drains the member has opened, counted from zero:
// every Instance on the target, moving to it, or whose move ended. The
// starts of the current wake-up are among them, since each is stamped on
// its row, where the gate reads, before the next start is consulted.
func (m batchMember) started() int32 { return m.promoted + m.open + m.ended }

// settled is what a peer may stop waiting on: Instances serving on the
// target, plus attempts that ended, which the escalation reports and the
// retry ladder reopens as a fresh start that counts as open again.
func (m batchMember) settled() int32 { return m.promoted + m.ended }

// mark is the cumulative count batch b should have settled: b steps, capped
// at the desired count for a last partial batch.
func (m batchMember) mark(b int32) int32 {
	if t := b * m.step; t < m.desired {
		return t
	}
	return m.desired
}

// batchStep is one group's reading for one start: the members in group
// order and the acting member's position among them. governs is false when
// the ratio and budget gates decide on their own.
type batchStep struct {
	governs bool
	acting  int
	members []batchMember
}

// readBatchStep reads the group's replicas and decides whether the batch
// step governs this start: the group rolls its members together (not
// Sequential), has two or more members, every member's replica rolls
// drain-first, and the acting member has a step to count in. A missing
// replica, a SurgeThenDrain or unset strategy, or a single-pod in-place
// member leaves the other gates in charge. A read error is returned for the
// caller to fail closed on.
func (ctx GateContext) readBatchStep() (batchStep, error) {
	if ctx.ShortCircuit || ctx.Group.Policy == v1beta1.CoordinationPolicySequential || len(ctx.Group.Components) < 2 {
		return batchStep{}, nil
	}
	step := batchStep{acting: -1}
	for _, comp := range ctx.Group.Components {
		ir, _, err := irprojector.DecodedComponentIRFor(ctx.Ctx, ctx.Reads, ctx.ISVC, comp)
		if err != nil {
			return batchStep{}, fmt.Errorf("cannot read %s IR status, failing closed: %w", comp, err)
		}
		if ir == nil || !drainsFirst(ir) {
			return batchStep{}, nil
		}
		target := rollTarget(ir)
		if comp == ctx.Component {
			step.acting = len(step.members)
			if ctx.TargetRevision != nil {
				target = ctx.TargetRevision.Name
			}
		}
		step.members = append(step.members, readBatchMember(ir, comp, target, ctx.Group.Pacing))
	}
	if step.acting < 0 || step.members[step.acting].step <= 0 {
		return batchStep{}, nil
	}
	step.governs = true
	return step, nil
}

// hold answers the acting member's next drain. The drain opens batch b, its
// drains so far divided by its step, and every member's batch b must have
// settled first. The first member short of its mark names the hold, peers
// before the acting member itself.
func (s batchStep) hold() (held bool, reason string) {
	me := s.members[s.acting]
	b := me.started() / me.step
	if b == 0 {
		return false, ""
	}
	order := make([]int, 0, len(s.members))
	for i := range s.members {
		if i != s.acting {
			order = append(order, i)
		}
	}
	order = append(order, s.acting)
	for _, i := range order {
		x := s.members[i]
		mark := x.mark(b)
		if x.settled() >= mark {
			continue
		}
		failed := ""
		if x.ended > 0 {
			failed = fmt.Sprintf(", %d failed", x.ended)
		}
		return true, fmt.Sprintf("%s batch %d waits for %s batch %d to serve (%d of %d serving%s)",
			me.component, b+1, x.component, b, x.promoted, mark, failed)
	}
	return false, ""
}

// readBatchMember classifies a replica's Instances against its roll target.
// The step is the member's own maxUnavailable capped by the group budget,
// as the budget gates resolve it.
func readBatchMember(ir *v1beta1.InferenceReplica, comp v1beta1.ComponentType, target string, pacing v1beta1.CoordinationPacing) batchMember {
	floor := resolvePairFloor(ir, pacing)
	m := batchMember{component: comp, desired: floor.desired, step: floor.maxUnavailable}
	for i := range ir.Status.InstanceStatuses {
		switch classifyBatchRow(&ir.Status.InstanceStatuses[i], target) {
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
// attempt ended without serving, any other is open. A Create or Migrate on
// the target is a new Instance, not a drain of this roll.
func classifyBatchRow(row *v1beta1.OMENativeInstanceStatus, target string) batchRowClass {
	if target == "" {
		return batchRowUntouched
	}
	op := row.Operation
	update := op != nil && op.Type == v1beta1.InstanceOperationUpdate
	pinnedElsewhere := update && op.TargetRevision != "" && op.TargetRevision != target
	if row.RunningRevision == target && !pinnedElsewhere {
		return batchRowPromoted
	}
	toward := row.TargetRevision == target || (update && op.TargetRevision == target)
	if !toward || (op != nil && !update) {
		return batchRowUntouched
	}
	if row.Phase == v1beta1.OMENativeInstanceFailed || (update && op.Step == workloadtypes.UpdateStepParked) {
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
