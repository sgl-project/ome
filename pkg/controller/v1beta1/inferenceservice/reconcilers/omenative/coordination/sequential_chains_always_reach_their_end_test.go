package coordination

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/v1beta1convert"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workloadstatus "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/status"
	workloadtypes "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
	"sigs.k8s.io/ome/pkg/utils"
)

// This file drives the production gate stack (EvaluateUpdateGate, with the
// Sequential gate last in it) over a model of one Sequential chain: two or
// three Components declared as single-Component blueGreen groups in list
// order, which ResolveGroups folds into one Sequential group. The model is
// each member's InferenceReplica as the replica controller publishes it:
// the rows, the current and update revisions, the generation the status was
// written for, the generation the projector recorded, the partition the
// replica holds Instances at, the retry record and the Ready condition.
//
// Between passes the world makes one move: the projector reaches a member,
// a started Instance serves, a pod is lost, an attempt fails, the retry
// record moves, the wait after readiness passes, or the operator acts (a
// partition set, raised, lowered or cleared on a member; a canary step's
// partition written on a member; a scale; a second push; a push back to
// version 1). After every move the dispatcher's admission runs for every
// member the way the replica controller runs it: the retry record, the
// per-Component budget and the gate, every admitted start written to its
// row before the next question, the Create pass rebuilding rows that lost
// their pods, and the status write that ends a pass. Every path is explored
// breadth-first to a bound with a hash of visited states, and seeded runs go
// deeper. At every state the model checks that the chain can still reach
// its end, that no later Component started while an earlier one was
// unfinished, and that the budgets hold. OME_SEQUENTIAL_HUNT=full widens
// the sample and the bounds.

// seqLastRevision is the highest version the operator pushes: version 1 is
// what the fleet stands on, version 2 the push, version 3 a later push.
const seqLastRevision int8 = 3

// seqRevisionName names a Component's revision the way the replica records
// it; the trailing segment is the hash the gates compare.
func seqRevisionName(comp v1beta1.ComponentType, rev int8) string {
	if rev <= 0 {
		return ""
	}
	return batchServiceName + "-" + string(comp) + "-v" + strconv.Itoa(int(rev))
}

// seqRevisionOf reads the version back off a revision name; 0 for none.
func seqRevisionOf(name string) int8 {
	i := strings.LastIndex(name, "-v")
	if i < 0 {
		return 0
	}
	n, err := strconv.Atoi(name[i+2:])
	if err != nil {
		return 0
	}
	return int8(n)
}

// seqRowKind is what the roll has done to one Instance, as its row reads.
type seqRowKind uint8

const (
	seqReady           seqRowKind = iota // every pod Ready and serving on rev
	seqLost                              // the pods are gone; the row reads Pending on rev
	seqRebuild                           // the Create pass rebuilds the pods on rev
	seqDrain                             // a recreate toward to is open; nothing serves
	seqSurge                             // a surge toward to is open; the source serves on rev
	seqFailedUnserving                   // a recreate toward to ended Failed; nothing serves
	seqFailedSurge                       // a surge toward to failed; published Ready over the serving source
	seqCreating                          // a new Instance builds toward to
)

var seqRowKindNames = [...]string{"ready", "lost", "rebuild", "drain", "surge", "failed", "failedsurge", "creating"}

// seqRow is one Instance: its kind, the revision it runs (0 before it has
// served) and the revision its attempt or its creation moves toward. scaled
// marks an Instance a scale-up created during the roll.
type seqRow struct {
	kind   seqRowKind
	rev    int8
	to     int8
	scaled bool
}

func (r seqRow) serving() bool {
	switch r.kind {
	case seqReady, seqSurge, seqFailedSurge:
		return true
	}
	return false
}

// open reports an update attempt in flight.
func (r seqRow) open() bool { return r.kind == seqDrain || r.kind == seqSurge }

func (r seqRow) String() string {
	s := seqRowKindNames[r.kind]
	if r.rev > 0 {
		s += "@" + strconv.Itoa(int(r.rev))
	}
	if r.to > 0 && r.kind != seqReady {
		s += ">" + strconv.Itoa(int(r.to))
	}
	if r.scaled {
		s += "+"
	}
	return s
}

// seqRetry is the state of a member's retry record for its target.
type seqRetry uint8

const (
	seqRetryNone     seqRetry = iota
	seqRetryBackoff           // a failure was recorded; no fresh start until the backoff ends
	seqRetryDue               // the backoff ended; the next pass admits a wave of starts
	seqRetryRetrying          // one wave at a time while an attempt is in flight
	seqRetryHeld              // the retries are spent; no fresh start until a new revision
)

var seqRetryNames = [...]string{"", "backoff", "due", "retrying", "held"}

// seqMemberConfig is one Component of the chain: its count, how it rolls,
// its own budget and the user partition in place before the push.
type seqMemberConfig struct {
	comp      v1beta1.ComponentType
	desired   int32
	strategy  v1beta1.UpdateStrategyType
	budget    string // the member's own maxSurge when it surges, else its own maxUnavailable
	partition int32  // the rollingUpdate.partition set before the push; 0 for none
}

func (m seqMemberConfig) surges() bool { return m.strategy == v1beta1.UpdateStrategySurgeThenDrain }

func (m seqMemberConfig) short() string {
	if m.surges() {
		return "surge"
	}
	return "drain"
}

// seqBudgets is how many times the world may make each costly move in one
// run.
type seqBudgets struct {
	fails, losses, scales, partitionEdits, steps, pushes, rollbacks int8
}

// seqConfig is one configuration of the chain under exploration.
type seqConfig struct {
	name        string
	members     []seqMemberConfig // in chain order
	afterReady  time.Duration     // the wait after a Component is Ready before the next starts (the group's Soak)
	maxAttempts int32
	budgets     seqBudgets
}

func (c seqConfig) String() string {
	parts := make([]string, 0, len(c.members))
	for _, m := range c.members {
		p := fmt.Sprintf("%s=%d/%s/%s", m.comp, m.desired, m.short(), m.budget)
		if m.partition > 0 {
			p += fmt.Sprintf("/partition=%d", m.partition)
		}
		parts = append(parts, p)
	}
	s := strings.Join(parts, " -> ")
	if c.afterReady > 0 {
		s += " wait=" + c.afterReady.String()
	}
	return s + fmt.Sprintf(" retries=%d", c.maxAttempts)
}

// seqMember is one member's replica as published, with the world's
// remaining moves on it.
type seqMember struct {
	rows     []seqRow
	retry    seqRetry
	attempts int32
	retryRev int8 // the revision the retry record names

	target       int8 // the revision the replica's spec asks for, as projected
	statusTarget int8 // UpdateRevision as published
	current      int8 // CurrentRevision as published; 0 while the engine keeps it empty
	clearedFrom  int8 // the revision an empty current revision was cleared from while an Instance still ran another

	irGen      int64 // the replica's metadata.generation
	statusGen  int64 // status.observedGeneration
	annotation int64 // the parent generation the projector recorded

	userPartition   int32 // the projected rollingUpdate.partition; 0 for none
	pacingPartition int32 // the projected spec.pacing.partition; -1 for none
	replicas        int32 // the projected spec.replicas

	readyTrue bool // the Ready condition reads True
	waited    bool // the wait after readiness has passed since Ready turned True
	flips     int8 // how often Ready turned False again after it had been waited out

	fails, losses, scales, partitionEdits, steps int8
}

func (m seqMember) count(kinds ...seqRowKind) int32 {
	var n int32
	for _, r := range m.rows {
		for _, k := range kinds {
			if r.kind == k {
				n++
			}
		}
	}
	return n
}

func (m seqMember) inFlight() bool {
	for _, r := range m.rows {
		if r.open() {
			return true
		}
	}
	return false
}

// partition is the partition the replica holds Instances at: the projected
// pacing value when set (an explicit 0 releases every Instance), else the
// user's rollingUpdate.partition.
func (m seqMember) partition() int32 {
	if m.pacingPartition >= 0 {
		return m.pacingPartition
	}
	return m.userPartition
}

func (m seqMember) String() string {
	parts := make([]string, 0, len(m.rows))
	for _, r := range m.rows {
		parts = append(parts, r.String())
	}
	out := strings.Join(parts, ",")
	out += fmt.Sprintf(" target=%d current=%d", m.target, m.current)
	if m.retry != seqRetryNone {
		out += fmt.Sprintf(" retry=%s/%d", seqRetryNames[m.retry], m.attempts)
	}
	if p := m.partition(); p > 0 || m.pacingPartition == 0 {
		out += fmt.Sprintf(" partition=%d", p)
	}
	if m.readyTrue {
		out += " ready"
		if m.waited {
			out += "+waited"
		}
	}
	return out
}

// seqHold is the first denial a member met in its last pass.
type seqHold struct {
	gate   v1beta1.RolloutHoldGate
	reason string
}

func (h *seqHold) String() string {
	if h == nil {
		return "none"
	}
	return fmt.Sprintf("%s: %s", h.gate, h.reason)
}

// seqMove is one thing the world does between two admissions.
type seqMove struct {
	kind   string
	member int
	idx    int
	value  int32
}

func (m seqMove) String() string {
	switch m.kind {
	case "push", "rollback":
		return m.kind
	case "project", "project-between":
		return m.kind
	case "backoff", "prune", "spent", "wait":
		return fmt.Sprintf("%s member %d", m.kind, m.member)
	case "scale", "partition", "step":
		return fmt.Sprintf("%s member %d to %d", m.kind, m.member, m.value)
	}
	return fmt.Sprintf("%s member %d row %d", m.kind, m.member, m.idx)
}

// seqState is the model at one moment: the service spec, every member's
// replica, the holds of the last admission, the order breaks the passes
// recorded, and the moves that led here.
type seqState struct {
	members    []seqMember
	gen        int64   // the service's generation
	rev        int8    // the revision the service spec names
	desired    []int32 // the count the spec asks of each member
	partitions []int32 // the user partition the spec sets on each member
	pushes     int8
	rollbacks  int8
	holds      []*seqHold
	breaks     []seqBreak
	path       []seqMove
}

// seqBreak is an order break a pass recorded: the class it falls in and
// what started ahead of what.
type seqBreak struct {
	class, message string
}

func (st *seqState) clone() *seqState {
	out := &seqState{
		members: make([]seqMember, len(st.members)), gen: st.gen, rev: st.rev,
		desired: append([]int32(nil), st.desired...), partitions: append([]int32(nil), st.partitions...),
		pushes: st.pushes, rollbacks: st.rollbacks,
		holds: make([]*seqHold, len(st.holds)), breaks: append([]seqBreak(nil), st.breaks...), path: append([]seqMove(nil), st.path...),
	}
	for i, m := range st.members {
		m.rows = append([]seqRow(nil), m.rows...)
		out.members[i] = m
	}
	return out
}

// key hashes what decides the future: the rows, the published revisions,
// whether the projection and the status trail the spec, the partitions, the
// counts, the retry record, the readiness clock and the budgets left.
func (st *seqState) key() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d|%d%d|", st.rev, st.pushes, st.rollbacks)
	for i, m := range st.members {
		for _, r := range m.rows {
			b.WriteByte(byte('a' + r.kind))
			b.WriteByte(byte('0' + r.rev))
			b.WriteByte(byte('0' + r.to))
			if r.scaled {
				b.WriteByte('+')
			}
		}
		projected, written := m.annotation == st.gen, m.statusGen == m.irGen
		fmt.Fprintf(&b, "|%d%d%d|%d%d%d%d|%t%t|%d/%d/%d|%d/%d|%t%t|%d%d%d%d%d;",
			m.retry, m.attempts, m.retryRev, m.target, m.statusTarget, m.current, m.clearedFrom, projected, written,
			m.userPartition, m.pacingPartition, m.replicas, st.desired[i], st.partitions[i], m.readyTrue, m.waited,
			m.fails, m.losses, m.scales, m.partitionEdits, m.steps)
	}
	return b.String()
}

func (st *seqState) String() string {
	var parts []string
	for i, m := range st.members {
		parts = append(parts, fmt.Sprintf("member %d [%s] hold=%s", i, m, st.holds[i]))
	}
	return fmt.Sprintf("spec version %d: ", st.rev) + strings.Join(parts, " | ")
}

func seqPath(st *seqState) string {
	parts := make([]string, 0, len(st.path))
	for _, mv := range st.path {
		parts = append(parts, mv.String())
	}
	return strings.Join(parts, " -> ")
}

// seqWorld builds the objects the gates read from a state and runs the
// dispatcher's admission over them.
type seqWorld struct {
	t      *testing.T
	cfg    seqConfig
	ctx    context.Context
	scheme *runtime.Scheme
	isvc   *v1beta1.InferenceService
	// gateCalls counts the questions put to the production gate stack.
	gateCalls int
	// settleOrder, when set, is the order the members' passes run in after
	// every move; the explorer alternates the order by depth instead.
	settleOrder []int
}

func newSeqWorld(t *testing.T, cfg seqConfig) *seqWorld {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := v1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1beta1: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add appsv1: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	groups := make([]v1beta1.RolloutGroup, 0, len(cfg.members))
	for i, m := range cfg.members {
		g := v1beta1.RolloutGroup{Components: []v1beta1.ComponentType{m.comp}, BlueGreen: &v1beta1.GroupBlueGreen{}}
		// The wait is the one after a group, so the last group carries none.
		if cfg.afterReady > 0 && i < len(cfg.members)-1 {
			g.Soak = &metav1.Duration{Duration: cfg.afterReady}
		}
		groups = append(groups, g)
	}
	isvc := &v1beta1.InferenceService{
		ObjectMeta: metav1.ObjectMeta{Name: batchServiceName, Namespace: batchNamespace, Generation: 1},
		Spec:       v1beta1.InferenceServiceSpec{Rollout: &v1beta1.RolloutSpec{Groups: groups}},
	}
	return &seqWorld{t: t, cfg: cfg, ctx: context.Background(), scheme: scheme, isvc: pinActiveRun(isvc)}
}

// initial is the moment after the push of version 2: the service's
// generation moved, no replica has been projected yet, every Instance runs
// version 1 and serves.
func (w *seqWorld) initial() *seqState {
	st := &seqState{gen: 2, rev: 2, pushes: w.cfg.budgets.pushes, rollbacks: w.cfg.budgets.rollbacks}
	for _, mc := range w.cfg.members {
		m := seqMember{
			rows: make([]seqRow, mc.desired), target: 1, statusTarget: 1, current: 1,
			irGen: 1, statusGen: 1, annotation: 1,
			userPartition: mc.partition, pacingPartition: -1, replicas: mc.desired,
			readyTrue: true, waited: true,
			fails: w.cfg.budgets.fails, losses: w.cfg.budgets.losses, scales: w.cfg.budgets.scales,
			partitionEdits: w.cfg.budgets.partitionEdits, steps: w.cfg.budgets.steps,
		}
		for i := range m.rows {
			m.rows[i] = seqRow{kind: seqReady, rev: 1}
		}
		st.members = append(st.members, m)
		st.desired = append(st.desired, mc.desired)
		st.partitions = append(st.partitions, mc.partition)
		st.holds = append(st.holds, nil)
	}
	return st
}

// isvcAt is the service the gates read at this state: the pinned chain at
// the state's generation.
func (w *seqWorld) isvcAt(st *seqState) *v1beta1.InferenceService {
	isvc := w.isvc.DeepCopy()
	isvc.Generation = st.gen
	return isvc
}

// v1beta1Rows is the member's rows as the status publishes them.
func (w *seqWorld) v1beta1Rows(i int, m *seqMember) []v1beta1.OMENativeInstanceStatus {
	comp := w.cfg.members[i].comp
	attempt := func(step string, to int8) *v1beta1.InstanceOperation {
		return &v1beta1.InstanceOperation{ID: "attempt", Type: v1beta1.InstanceOperationUpdate, Step: step, TargetRevision: seqRevisionName(comp, to)}
	}
	create := func(to int8) *v1beta1.InstanceOperation {
		return &v1beta1.InstanceOperation{ID: "create", Type: v1beta1.InstanceOperationCreate, TargetRevision: seqRevisionName(comp, to)}
	}
	out := make([]v1beta1.OMENativeInstanceStatus, 0, len(m.rows))
	for idx, r := range m.rows {
		row := v1beta1.OMENativeInstanceStatus{Index: int32(idx), RunningRevision: seqRevisionName(comp, r.rev)}
		switch r.kind {
		case seqReady:
			row.Phase, row.PodCount, row.ReadyPodCount, row.ServingPodCount = v1beta1.OMENativeInstanceReady, 1, 1, 1
		case seqLost:
			row.Phase = v1beta1.OMENativeInstancePending
		case seqRebuild:
			row.Phase, row.Operation = v1beta1.OMENativeInstanceCreating, create(r.rev)
		case seqDrain:
			row.Phase, row.TargetRevision, row.Operation = v1beta1.OMENativeInstanceUpdating, seqRevisionName(comp, r.to), attempt(workloadtypes.UpdateStepDrain, r.to)
		case seqSurge:
			row.Phase, row.TargetRevision, row.Operation = v1beta1.OMENativeInstanceUpdating, seqRevisionName(comp, r.to), attempt(workloadtypes.UpdateStepSurge, r.to)
			row.PodCount, row.ReadyPodCount, row.ServingPodCount = 2, 1, 1
		case seqFailedUnserving:
			row.Phase, row.TargetRevision = v1beta1.OMENativeInstanceFailed, seqRevisionName(comp, r.to)
		case seqFailedSurge:
			row.Phase, row.TargetRevision = v1beta1.OMENativeInstanceReady, seqRevisionName(comp, r.to)
			row.PodCount, row.ReadyPodCount, row.ServingPodCount = 2, 1, 1
		case seqCreating:
			row.Phase, row.TargetRevision, row.Operation = v1beta1.OMENativeInstanceCreating, seqRevisionName(comp, r.to), create(r.to)
		}
		out = append(out, row)
	}
	return out
}

// retryBlocks is the member's retry record as the status publishes it.
func (w *seqWorld) retryBlocks(i int, m *seqMember) []v1beta1.RetryBlock {
	if m.retry == seqRetryNone {
		return nil
	}
	state := v1beta1.RetryBlockBackoff
	switch m.retry {
	case seqRetryRetrying:
		state = v1beta1.RetryBlockRetryInProgress
	case seqRetryHeld:
		state = v1beta1.RetryBlockHeld
	}
	return []v1beta1.RetryBlock{{TargetRevision: seqRevisionName(w.cfg.members[i].comp, m.retryRev), State: state, AttemptsStarted: m.attempts}}
}

// replica is the InferenceReplica a member publishes.
func (w *seqWorld) replica(i int, m *seqMember) *v1beta1.InferenceReplica {
	mc := w.cfg.members[i]
	ir := &v1beta1.InferenceReplica{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: batchNamespace, Name: batchServiceName + "-" + string(mc.comp),
			Generation: m.irGen, Annotations: parentGenAnnotation(m.annotation),
		},
		Spec: v1beta1.InferenceReplicaSpec{
			Component: mc.comp,
			Replicas:  ptr.To(m.replicas),
			Lifecycle: &v1beta1.LifecycleSpec{UpdateStrategy: &v1beta1.UpdateStrategy{Type: mc.strategy, RollingUpdate: &v1beta1.RollingUpdate{}}},
		},
		Status: v1beta1.InferenceReplicaStatus{
			ObservedGeneration: m.statusGen,
			CurrentRevision:    seqRevisionName(mc.comp, m.current),
			UpdateRevision:     seqRevisionName(mc.comp, m.statusTarget),
			InstanceStatuses:   w.v1beta1Rows(i, m),
			RetryBlocks:        w.retryBlocks(i, m),
		},
	}
	if mc.surges() {
		ir.Spec.Lifecycle.UpdateStrategy.RollingUpdate.MaxSurge = ptr.To(intstr.Parse(mc.budget))
	} else {
		ir.Spec.Lifecycle.UpdateStrategy.RollingUpdate.MaxUnavailable = ptr.To(intstr.Parse(mc.budget))
	}
	if m.userPartition > 0 {
		ir.Spec.Lifecycle.UpdateStrategy.RollingUpdate.Partition = ptr.To(m.userPartition)
	}
	if m.pacingPartition >= 0 {
		ir.Spec.Pacing = &v1beta1.InferenceReplicaPacing{Partition: ptr.To(m.pacingPartition)}
	}
	var serving, updated int32
	for _, r := range m.rows {
		if r.serving() {
			serving++
		}
		if r.kind == seqReady && r.rev == m.statusTarget {
			updated++
		}
	}
	ir.Status.Replicas = int32(len(m.rows))
	ir.Status.ServingReplicas, ir.Status.ReadyReplicas, ir.Status.UpdatedReplicas = serving, serving, updated
	// The Ready condition: True once the member is converged or staged, and
	// its transition time is what the next member's wait is measured from.
	cond := metav1.Condition{Type: conditionReady, Status: metav1.ConditionUnknown, Reason: "RolloutInProgress", LastTransitionTime: metav1.Now()}
	if m.readyTrue {
		cond.Status, cond.Reason = metav1.ConditionTrue, "AllInstancesReady"
		if m.waited {
			cond.LastTransitionTime = metav1.NewTime(time.Now().Add(-2 * time.Hour))
		}
	}
	ir.Status.Conditions = []metav1.Condition{cond}
	return ir
}

// objects are everything the gates read for a state: the replicas and every
// revision of every member.
func (w *seqWorld) objects(st *seqState) []client.Object {
	var objs []client.Object
	for i := range w.cfg.members {
		objs = append(objs, w.replica(i, &st.members[i]))
		for rev := int8(1); rev <= seqLastRevision; rev++ {
			objs = append(objs, pairingCR(batchNamespace, batchServiceName, w.cfg.members[i].comp, "v"+strconv.Itoa(int(rev)), ""))
		}
	}
	return objs
}

func (w *seqWorld) client(st *seqState) client.Client {
	return fake.NewClientBuilder().WithScheme(w.scheme).WithObjects(w.objects(st)...).Build()
}

// heldIndices is the set of Instances the partition holds, by the roll's own
// rule: the lowest-indexed Instances off the target that no update attempt
// is moving.
func (w *seqWorld) heldIndices(m *seqMember) map[int]bool {
	held := map[int]bool{}
	p := m.partition()
	if p <= 0 {
		return held
	}
	var n int32
	for idx, r := range m.rows {
		if r.open() || (r.rev == m.target && r.kind != seqCreating) {
			continue
		}
		if n < p {
			held[idx] = true
			n++
		}
	}
	return held
}

// finishedFor reports whether a member has nothing left to roll toward rev:
// no attempt open and every Instance on rev, except those a partition holds.
// An Instance a scale-up is still creating is new capacity, not the roll's
// work, whatever revision it builds toward.
func (w *seqWorld) finishedFor(m *seqMember, rev int8) bool {
	var off int32
	for _, r := range m.rows {
		if r.open() {
			return false
		}
		if r.kind == seqCreating {
			continue
		}
		if r.rev != rev {
			off++
		}
	}
	return off == 0 || (m.partition() > 0 && off <= m.partition())
}

// complete reports the end of the roll: every member projected, its status
// written, every Instance Ready on the spec's revision or held by the
// member's partition, between one and the partition of them.
func (w *seqWorld) complete(st *seqState) bool {
	for i := range st.members {
		m := &st.members[i]
		if m.annotation != st.gen || m.statusGen != m.irGen || m.target != st.rev {
			return false
		}
		var off int32
		for _, r := range m.rows {
			if r.kind != seqReady {
				return false
			}
			if r.rev != st.rev {
				off++
			}
		}
		if off != 0 && !(m.partition() > 0 && off <= m.partition()) {
			return false
		}
	}
	return true
}

// writeStatus is the status write that ends a member's pass: the generation
// observed, the update revision, the current revision by the engine's own
// rule, and the Ready condition with its clock.
func (w *seqWorld) writeStatus(i int, m *seqMember) {
	comp := w.cfg.members[i].comp
	m.statusGen = m.irGen
	m.statusTarget = m.target
	insts := v1beta1convert.InstanceStatusSliceToWorkload(w.v1beta1Rows(i, m))
	update := seqRevisionName(comp, m.target)
	var blocks []workloadtypes.RetryBlock
	if m.retry != seqRetryNone {
		blocks = []workloadtypes.RetryBlock{{TargetRevision: seqRevisionName(comp, m.retryRev)}}
	}
	pods := workloadstatus.PodRevisions{}
	for idx, r := range m.rows {
		switch r.kind {
		case seqReady:
			pods[int32(idx)] = []query.RevisionID{query.RevisionFromName(seqRevisionName(comp, r.rev))}
		case seqSurge, seqFailedSurge:
			pods[int32(idx)] = []query.RevisionID{query.RevisionFromName(seqRevisionName(comp, r.rev)), query.RevisionFromName(seqRevisionName(comp, r.to))}
		}
	}
	before, oldCurrent := seqRevisionName(comp, m.clearedFrom), seqRevisionName(comp, m.current)
	current := workloadstatus.CurrentRevisionFor(insts, oldCurrent, update, blocks, before, pods)
	m.clearedFrom = seqRevisionOf(workloadstatus.WithdrawnRevisionAfter(before, oldCurrent, current, update))
	m.current = seqRevisionOf(current)

	ready := w.readyCondition(i, m, insts)
	switch {
	case ready && !m.readyTrue:
		m.waited = false
	case !ready && m.readyTrue:
		if m.waited {
			m.flips++
		}
		m.waited = false
	}
	m.readyTrue = ready
}

// readyCondition is the replica's Ready condition as the status writer
// derives it: False on a Failed Instance, True when staged at a partition,
// Unknown while the revisions differ, True when every Instance is Ready or
// the serving count meets the member's own minimum.
func (w *seqWorld) readyCondition(i int, m *seqMember, insts []workloadtypes.InstanceStatus) bool {
	mc := w.cfg.members[i]
	for _, r := range m.rows {
		if r.kind == seqFailedUnserving {
			return false
		}
	}
	replicas := int32(len(m.rows))
	if p := m.partition(); p > 0 && workloadstatus.ReachedDesiredShape(insts, seqRevisionName(mc.comp, m.target), p, replicas) {
		return true
	}
	if m.current != m.target {
		return false
	}
	if replicas == 0 {
		return false
	}
	var serving int32
	for _, r := range m.rows {
		if r.serving() {
			serving++
		}
	}
	if serving == replicas {
		return true
	}
	var maxUnavailable *intstr.IntOrString
	if !mc.surges() {
		maxUnavailable = ptr.To(intstr.Parse(mc.budget))
	}
	return serving >= utils.AvailabilityFloor(replicas, nil, nil, maxUnavailable)
}

// pass is one update pass of one member: the rows follow the count the
// replica asks for, the retry record, then every fresh start candidate in
// selection order (Instances out of rotation first, then index order) not
// held by the partition, each against the per-Component budget and the
// gate, every admitted start written before the next question; a row that
// lost its pods and was not started is rebuilt by the Create pass; the
// status write ends the pass. The first denial is the member's hold.
func (w *seqWorld) pass(st *seqState, i int) (admitted int, hold *seqHold) {
	mc := w.cfg.members[i]
	m := &st.members[i]
	defer w.writeStatus(i, m)
	// The Create pass rebuilds, on the revision it ran, every row that lost
	// its pods and was not started, whatever held the start.
	defer func() {
		for idx, r := range m.rows {
			if r.kind == seqLost {
				m.rows[idx].kind = seqRebuild
			}
		}
	}()
	// A retry record of a revision the spec no longer asks for is removed.
	if m.retry != seqRetryNone && m.retryRev != m.target {
		m.retry, m.attempts = seqRetryNone, 0
	}
	// The Create and scale-down passes: new Instances build toward the
	// target; a scale-down retires the highest indices.
	for int32(len(m.rows)) < m.replicas {
		m.rows = append(m.rows, seqRow{kind: seqCreating, to: m.target, scaled: true})
	}
	if int32(len(m.rows)) > m.replicas {
		m.rows = m.rows[:m.replicas]
	}
	switch {
	case m.retry == seqRetryHeld:
		return 0, &seqHold{gate: v1beta1.RolloutHoldGateHeld, reason: fmt.Sprintf("update held after %d failed attempt(s)", m.attempts)}
	case m.retry == seqRetryBackoff:
		return 0, &seqHold{gate: v1beta1.RolloutHoldGateRetryBlock, reason: fmt.Sprintf("update in backoff after %d failed attempt(s)", m.attempts)}
	case m.retry == seqRetryRetrying && m.inFlight():
		return 0, &seqHold{gate: v1beta1.RolloutHoldGateRetryBlock, reason: "retrying one attempt at a time"}
	}
	// A failed attempt toward a revision the spec has left settles on the
	// revision the Instance runs when that is the target: a Failed Instance
	// with no pods is the Create pass's to rebuild on the target it already
	// records, and a failed replacement beside a serving source on the
	// target is a leftover of a replaced revision.
	for idx, r := range m.rows {
		if r.rev != m.target {
			continue
		}
		switch r.kind {
		case seqFailedUnserving:
			m.rows[idx] = seqRow{kind: seqLost, rev: r.rev}
		case seqFailedSurge:
			m.rows[idx] = seqRow{kind: seqReady, rev: r.rev}
		}
	}
	held := w.heldIndices(m)
	// A Failed Instance with no pods that the partition holds is not
	// retried; the Create pass rebuilds it on the revision it ran.
	for idx, r := range m.rows {
		if r.kind == seqFailedUnserving && held[idx] {
			m.rows[idx] = seqRow{kind: seqRebuild, rev: r.rev}
		}
	}
	// The roll's candidates: Instances off the target the partition does
	// not hold, those out of rotation first. An Instance that lost its pods
	// on the target revision is the Create pass's to rebuild, not the roll's.
	var order []int
	for idx, r := range m.rows {
		if (r.kind == seqLost || r.kind == seqFailedUnserving) && r.rev != m.target && !held[idx] {
			order = append(order, idx)
		}
	}
	for idx, r := range m.rows {
		if ((r.kind == seqReady && r.rev != m.target) || r.kind == seqFailedSurge) && !held[idx] {
			order = append(order, idx)
		}
	}
	type answer struct {
		kind seqRowKind
		rev  int8
	}
	denied := map[answer]bool{}
	var inFlightSurge, inFlightUnavail, gateUnavail int32
	var c client.Client
	own := utils.ScaledCountFromIntOrString(ptr.To(intstr.Parse(mc.budget)), int32(len(m.rows)), true)
	target := pairingCR(batchNamespace, batchServiceName, mc.comp, "v"+strconv.Itoa(int(m.target)), "")
	for _, idx := range order {
		r := m.rows[idx]
		if denied[answer{r.kind, r.rev}] {
			continue
		}
		// A surge needs a serving source: an Instance that lost its pods is
		// recreated whatever the strategy says.
		mechanism := workloadtypes.UpdateStrategyRecreatePod
		if mc.surges() && (r.kind == seqReady || r.kind == seqFailedSurge) {
			mechanism = workloadtypes.UpdateStrategySurgeThenDrain
		}
		// A recreate of a Failed Instance that serves nothing skips the
		// gate: its outage is already in the gate's serving count.
		exempt := r.kind == seqFailedUnserving
		var h *seqHold
		if mechanism == workloadtypes.UpdateStrategySurgeThenDrain {
			prior := m.count(seqSurge, seqFailedSurge)
			if r.kind == seqFailedSurge {
				prior-- // the retry takes its own failed pod's slot
			}
			if prior+inFlightSurge+1 > own {
				h = &seqHold{gate: v1beta1.RolloutHoldGateBudget, reason: fmt.Sprintf("per-Component surge budget %d exhausted (would become %d)", own, prior+inFlightSurge+1)}
			}
		} else if prior := m.count(seqDrain); prior+inFlightUnavail+1 > own {
			h = &seqHold{gate: v1beta1.RolloutHoldGateBudget, reason: fmt.Sprintf("per-Component unavailability budget %d exhausted (would become %d)", own, prior+inFlightUnavail+1)}
		}
		if h == nil && !exempt {
			if c == nil {
				c = w.client(st)
			}
			w.gateCalls++
			allowed, gate, reason := EvaluateUpdateGate(w.ctx, c, w.isvcAt(st), mc.comp, seqRevisionName(mc.comp, r.rev), target, nil, GroupDefaults{}, mechanism, inFlightSurge, gateUnavail)
			if !allowed {
				h = &seqHold{gate: gate, reason: reason}
			}
		}
		if h != nil {
			denied[answer{r.kind, r.rev}] = true
			if hold == nil {
				hold = h
			}
			continue
		}
		if !exempt && i > 0 && w.cfg.afterReady > 0 {
			// The wait after readiness: the member before this one must have
			// been Ready for the configured time before a start here.
			if prev := &st.members[i-1]; !(prev.readyTrue && prev.waited) {
				class := "known-wait-skipped-in-the-promotion-window"
				switch {
				case !prev.readyTrue:
					class = "known-wait-skipped-while-the-earlier-member-is-not-ready"
				case prev.partition() > 0 && prev.current != prev.statusTarget:
					// A member staged at a partition is the completed previous
					// Component, and the wait is measured from its Ready transition.
					class = "wait-skipped-after-a-staged-member"
				}
				st.breaks = append(st.breaks, seqBreak{class: class, message: fmt.Sprintf("%s started Instance %d toward version %d without the wait after %s (%s)",
					mc.comp, idx, m.target, w.cfg.members[i-1].comp, *prev)})
			}
		}
		if !exempt {
			for j := 0; j < i; j++ {
				earlier := &st.members[j]
				if w.finishedFor(earlier, m.target) {
					continue
				}
				class := "order"
				switch {
				case m.target != st.rev:
					// The member's own projection trails the spec: it starts a
					// revision the operator has since replaced.
					class = "known-stale-projection-starts-a-retracted-revision"
				case earlier.statusTarget != m.target && earlier.current != earlier.statusTarget && w.stagedOrDone(earlier, earlier.statusTarget):
					// A member whose current revision trails its update revision
					// (staged at a partition, or a retry record still naming the
					// revision) while its Instances sit at the shape that
					// revision asks for reads as done against the revision its
					// status still records, until the status write records the
					// new one.
					class = "known-member-at-its-old-shape-read-as-done-after-a-push"
				}
				st.breaks = append(st.breaks, seqBreak{class: class, message: fmt.Sprintf("%s started Instance %d toward version %d while %s was not finished with it (%s)",
					mc.comp, idx, m.target, w.cfg.members[j].comp, *earlier)})
			}
		}
		if mechanism == workloadtypes.UpdateStrategySurgeThenDrain {
			m.rows[idx] = seqRow{kind: seqSurge, rev: r.rev, to: m.target}
			inFlightSurge++
		} else {
			m.rows[idx] = seqRow{kind: seqDrain, rev: r.rev, to: m.target}
			inFlightUnavail++
			if !exempt {
				gateUnavail++
			}
		}
		if m.retry == seqRetryDue {
			m.retry = seqRetryRetrying
		}
		admitted++
		c = nil
	}
	return admitted, hold
}

// settle runs passes over the members in the given order until a whole
// round admits nothing and writes nothing new.
func (w *seqWorld) settle(st *seqState, order []int) {
	for round := 0; round < 32; round++ {
		before := st.key()
		total := 0
		for _, i := range order {
			a, h := w.pass(st, i)
			st.holds[i] = h
			total += a
		}
		if total == 0 && st.key() == before {
			return
		}
	}
	w.t.Fatalf("admission did not settle in 32 rounds: %s", st)
}

func (w *seqWorld) order(depth int) []int {
	if w.settleOrder != nil {
		return append([]int(nil), w.settleOrder...)
	}
	order := make([]int, len(w.cfg.members))
	for i := range order {
		order[i] = i
	}
	if depth%2 == 1 {
		// the later members first on odd depths
		for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}
	}
	return order
}

// projectorOrder is the order the service controller projects the
// Components in, whatever order the chain declares: the engine, then the
// decoder, then the router.
func (w *seqWorld) projectorOrder() []int {
	rank := func(c v1beta1.ComponentType) int {
		switch c {
		case v1beta1.EngineComponent:
			return 0
		case v1beta1.DecoderComponent:
			return 1
		}
		return 2
	}
	order := make([]int, len(w.cfg.members))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return rank(w.cfg.members[order[a]].comp) < rank(w.cfg.members[order[b]].comp) })
	return order
}

// project is the projector reaching one member: the parent generation is
// recorded, and the spec's revision, count and user partition land on the
// replica, which moves its generation when any of them changed.
func (w *seqWorld) project(st *seqState, i int) {
	m := &st.members[i]
	m.annotation = st.gen
	changed := m.target != st.rev || m.replicas != st.desired[i] || m.userPartition != st.partitions[i]
	if m.target != st.rev {
		// Rows that run the new target but recorded a failed attempt toward
		// another revision settle on it: a failed replacement beside a
		// serving source is removed, a Failed Instance with no pods is
		// rebuilt on the revision it ran.
		for idx, r := range m.rows {
			if r.rev != st.rev {
				continue
			}
			switch r.kind {
			case seqFailedSurge:
				m.rows[idx] = seqRow{kind: seqReady, rev: r.rev}
			case seqFailedUnserving:
				m.rows[idx] = seqRow{kind: seqLost, rev: r.rev}
			}
		}
	}
	m.target, m.replicas, m.userPartition = st.rev, st.desired[i], st.partitions[i]
	if changed {
		m.irGen++
	}
}

// progressMoves are the moves that advance the roll by themselves: the
// projector reaches a member, a started or rebuilt Instance serves, a
// backoff ends, a retry record is removed, the wait after readiness passes.
func (w *seqWorld) progressMoves(st *seqState) []seqMove {
	var out []seqMove
	// The projector's pass writes every member in its order within one
	// tick; the replicas may reconcile between two of its writes or only
	// after all of them.
	for i := range st.members {
		if st.members[i].annotation != st.gen {
			out = append(out, seqMove{kind: "project"}, seqMove{kind: "project-between"})
			break
		}
	}
	for i := range st.members {
		m := &st.members[i]
		first, last := map[seqRowKind]int{}, map[seqRowKind]int{}
		for idx, r := range m.rows {
			if _, ok := first[r.kind]; !ok {
				first[r.kind] = idx
			}
			last[r.kind] = idx
		}
		for _, kind := range []seqRowKind{seqDrain, seqSurge, seqCreating, seqRebuild} {
			idx, ok := first[kind]
			if !ok {
				continue
			}
			out = append(out, seqMove{kind: "serve", member: i, idx: idx})
			if last[kind] != idx {
				out = append(out, seqMove{kind: "serve", member: i, idx: last[kind]})
			}
		}
		switch {
		case m.retry == seqRetryBackoff:
			out = append(out, seqMove{kind: "backoff", member: i})
		case m.retry == seqRetryRetrying && !m.inFlight():
			out = append(out, seqMove{kind: "prune", member: i})
		}
		if w.cfg.afterReady > 0 && m.readyTrue && !m.waited {
			out = append(out, seqMove{kind: "wait", member: i})
		}
	}
	return out
}

// adverseMoves are the moves that cost the roll something: an attempt
// fails, a pod is lost, the retries are spent, or the operator acts.
func (w *seqWorld) adverseMoves(st *seqState) []seqMove {
	var out []seqMove
	for i := range st.members {
		m := &st.members[i]
		seenFail, seenLoss := map[seqRowKind]bool{}, map[int8]bool{}
		for idx, r := range m.rows {
			switch r.kind {
			case seqDrain, seqSurge:
				if m.fails > 0 && !seenFail[r.kind] {
					seenFail[r.kind] = true
					out = append(out, seqMove{kind: "fail", member: i, idx: idx})
				}
			case seqReady:
				// One loss per revision the member runs: a pod of the old
				// version and a pod of the new one are different pictures.
				if m.losses > 0 && !seenLoss[r.rev] {
					seenLoss[r.rev] = true
					out = append(out, seqMove{kind: "lose", member: i, idx: idx})
				}
			}
		}
		if m.retry != seqRetryHeld && m.retry != seqRetryNone && m.count(seqFailedUnserving, seqFailedSurge) > 0 {
			out = append(out, seqMove{kind: "spent", member: i})
		}
		n := st.desired[i]
		if m.scales > 0 {
			for _, delta := range []int32{1, -1, n / 2, -(n / 2)} {
				if delta != 0 && n+delta >= 1 {
					out = append(out, seqMove{kind: "scale", member: i, value: n + delta})
				}
			}
		}
		if m.partitionEdits > 0 {
			values := []int32{n / 2, n - 1, n}
			if st.partitions[i] > 0 {
				values = append(values, 0, st.partitions[i]+1)
			}
			seen := map[int32]bool{st.partitions[i]: true}
			for _, v := range values {
				if v < 0 || seen[v] {
					continue
				}
				seen[v] = true
				out = append(out, seqMove{kind: "partition", member: i, value: v})
			}
		}
		if m.steps > 0 {
			// A canary step's partition: a step that holds half, the final
			// step that releases every Instance, and the step removed.
			for _, v := range []int32{n / 2, 0, -1} {
				if v != m.pacingPartition && (v >= 0 || m.pacingPartition >= 0) {
					out = append(out, seqMove{kind: "step", member: i, value: v})
				}
			}
		}
	}
	if st.pushes > 0 && st.rev < seqLastRevision {
		out = append(out, seqMove{kind: "push"})
	}
	if st.rollbacks > 0 && st.rev >= 2 {
		out = append(out, seqMove{kind: "rollback"})
	}
	return out
}

// apply makes one move and then lets the dispatcher admit what the gates
// allow.
func (w *seqWorld) apply(st *seqState, mv seqMove) *seqState {
	next := st.clone()
	next.path = append(next.path, mv)
	next.breaks = nil
	var m *seqMember
	switch mv.kind {
	case "push", "rollback", "project", "project-between":
	default:
		m = &next.members[mv.member]
	}
	switch mv.kind {
	case "project":
		for _, i := range w.projectorOrder() {
			w.project(next, i)
		}
	case "project-between":
		for _, i := range w.projectorOrder() {
			w.project(next, i)
			w.settle(next, w.order(len(next.path)))
		}
	case "serve":
		r := m.rows[mv.idx]
		switch r.kind {
		case seqRebuild:
			m.rows[mv.idx] = seqRow{kind: seqReady, rev: r.rev}
		default:
			m.rows[mv.idx] = seqRow{kind: seqReady, rev: r.to, scaled: r.scaled}
		}
	case "fail":
		r := m.rows[mv.idx]
		if r.kind == seqSurge {
			m.rows[mv.idx].kind = seqFailedSurge
		} else {
			m.rows[mv.idx].kind = seqFailedUnserving
		}
		m.fails--
		// A failure counts once per wave: a record in backoff or Held is not
		// moved by a second failure of the same wave.
		if m.retry != seqRetryBackoff && m.retry != seqRetryHeld {
			m.attempts++
			m.retryRev = r.to
			if m.attempts >= w.cfg.maxAttempts {
				m.retry = seqRetryHeld
			} else {
				m.retry = seqRetryBackoff
			}
		}
	case "lose":
		m.rows[mv.idx] = seqRow{kind: seqLost, rev: m.rows[mv.idx].rev}
		m.losses--
	case "backoff":
		m.retry = seqRetryDue
	case "prune":
		m.retry, m.attempts = seqRetryNone, 0
	case "spent":
		m.retry = seqRetryHeld
	case "wait":
		m.waited = true
	case "scale":
		next.desired[mv.member] = mv.value
		next.gen++
		m.scales--
	case "partition":
		next.partitions[mv.member] = mv.value
		next.gen++
		m.partitionEdits--
	case "step":
		m.pacingPartition = mv.value
		m.irGen++
		m.steps--
	case "push":
		next.rev++
		next.gen++
		next.pushes--
	case "rollback":
		next.rev = 1
		next.gen++
		next.rollbacks--
	}
	w.settle(next, w.order(len(next.path)))
	return next
}

// seqFinding is one invariant broken, or one known reading met, at one state.
type seqFinding struct {
	class   string
	message string
	state   *seqState
}

// seqKnownClasses are the readings the explorer reports without failing,
// each in words: the engine's designed behaviour, the behaviour not decided
// yet, and the readings the tests below record as the current behaviour.
var seqKnownClasses = map[string]string{
	"known-spent-retries-hold-the-chain":                            "an earlier Component whose retries are spent with Instances left on the old version keeps every later Component waiting under \"Sequential waiting on <c>\"; the hold names the Component, not its spent retries (the Sequential failure hold the KIND spec pins)",
	"known-retries-spent":                                           "the only unfinished Component's retries are spent and nothing waits on it; a new revision or a release of the held revision ends it",
	"known-member-at-its-old-shape-read-as-done-after-a-push":       "a member whose current revision trails its update revision (staged at a partition, or a retry record still naming the revision) while its Instances sit at the shape that revision asks for reads as done against the revision its status still records; after the next push or a push back the later member starts before it moved, until its status records the new revision (not decided yet)",
	"known-stale-projection-starts-a-retracted-revision":            "a member still projected for a push the operator has since replaced starts that revision in the pass before the projector rewrites its spec, when the earlier member has nothing to roll for the new spec; the gate reads the stale projection as the member's own turn (not decided yet)",
	"known-failed-replacement-under-a-held-instance-keeps-its-slot": "a surge replacement that failed beside its serving source keeps its surge slot until a start on that Instance replaces it; once a partition holds that Instance no start comes, and under a surge budget of one the member's movable Instances never start (not decided yet; the budget side, not the Sequential gate)",
	"known-wait-skipped-in-the-promotion-window":                    "in the pass between the previous Component's last Instance serving and its status write recording the new current revision, the gate reads it as in flight and skips the wait after readiness; the next Component may start at once (not decided yet; the same reading)",
	"known-wait-skipped-while-the-earlier-member-is-not-ready":      "the previous Component finished its roll but its Ready condition reads False (a pod lost under its serving minimum), so the wait after readiness has no clock and the next Component starts at once (which clock the wait reads is not decided yet)",
	"known-scale-up-ahead":                                          "a scale-up on a waiting Component creates its new Instances on the new version before the earlier Component finished (which revision an Instance created during a staged roll runs is not decided yet)",
	"known-wait-restarts-after-a-pod-loss":                          "a pod lost on the finished Component takes it under its serving minimum, its Ready condition turns False and then True again, and the next Component's wait after readiness starts over from the rebuild (which clock the wait reads is not decided yet)",
	"known-lost-pod-on-a-waiting-member-is-rebuilt":                 "an Instance of a waiting Component that lost its pods is offered to the roll first, which the Sequential gate holds, and the Create pass rebuilds it on its old version in the same pass; the hold names the earlier Component (by design)",
	"known-budget-after-a-scale":                                    "an attempt that started before a scale-down counts against the smaller budget until it ends (by design)",
}

// check reads every invariant at one settled state.
func (w *seqWorld) check(st *seqState) []seqFinding {
	var out []seqFinding
	add := func(class, format string, args ...any) {
		out = append(out, seqFinding{class: class, message: fmt.Sprintf(format, args...), state: st})
	}
	for _, b := range st.breaks {
		add(b.class, "%s", b.message)
	}
	pacing := pacingWithDefaults(nil, GroupDefaults{})
	for i, mc := range w.cfg.members {
		m := st.members[i]
		n := int32(len(m.rows))
		own := utils.ScaledCountFromIntOrString(ptr.To(intstr.Parse(mc.budget)), n, true)
		// An attempt that started before a scale-down counts against the
		// smaller budget until it ends; that excess is reported on its own.
		budgetClass := "budget"
		if m.scales < w.cfg.budgets.scales {
			budgetClass = "known-budget-after-a-scale"
		}
		if mc.surges() {
			open := m.count(seqSurge)
			if open+m.count(seqFailedSurge) > own {
				add(budgetClass, "%s has %d surges in flight beside %d failed surge pods against its own budget of %d", mc.comp, open, m.count(seqFailedSurge), own)
			}
			if group := MaxSurgeBudget(pacing, n); open > group {
				add(budgetClass, "%s has %d surges in flight against the group budget of %d", mc.comp, open, group)
			}
		} else {
			open := m.count(seqDrain)
			if open > own {
				add(budgetClass, "%s has %d Instances out of rotation against its own budget of %d", mc.comp, open, own)
			}
			if group := MaxUnavailableBudget(pacing, n); open+m.count(seqFailedUnserving) > group {
				add(budgetClass, "%s has %d Instances out of rotation beside %d failed against the group unavailability budget of %d", mc.comp, open, m.count(seqFailedUnserving), group)
			}
		}
		// New Instances of a later member on a version an earlier member has
		// not finished.
		for j := 0; j < i; j++ {
			for _, r := range m.rows {
				rev := r.rev
				if r.kind == seqCreating {
					rev = r.to
				}
				if r.scaled && (r.kind == seqCreating || r.kind == seqReady) && !w.finishedFor(&st.members[j], rev) {
					add("known-scale-up-ahead", "%s runs a scale-up Instance on version %d while %s is not finished with it (%s)", mc.comp, rev, w.cfg.members[j].comp, st.members[j])
					break
				}
			}
		}
		if h := st.holds[i]; h != nil && h.gate == v1beta1.RolloutHoldGateSequential {
			if strings.Contains(h.reason, "Sequential.Soak") {
				for j := 0; j < i; j++ {
					if st.members[j].flips > 0 {
						add("known-wait-restarts-after-a-pod-loss", "%s waits again (%s) after %s turned Ready a second time", mc.comp, h.reason, w.cfg.members[j].comp)
					}
				}
			}
			if m.count(seqRebuild) > 0 {
				add("known-lost-pod-on-a-waiting-member-is-rebuilt", "%s rebuilds an Instance that lost its pods under %s", mc.comp, h)
			}
		}
	}
	if !w.complete(st) && len(w.progressMoves(st)) == 0 {
		class, message := w.classifyStopped(st)
		add(class, "%s", message)
	}
	return out
}

// classifyStopped names why a state that cannot move is what it is. The
// first unfinished member in chain order decides: its retries are spent
// ("known-spent-retries-hold-the-chain" when a later member waits on it,
// "known-retries-spent" when none does); a failed surge replacement
// under an Instance its partition holds keeps the surge slot its movable
// Instances need ("known-failed-replacement-under-a-held-instance-keeps-its-slot");
// or nothing explains it ("no-way-out").
func (w *seqWorld) classifyStopped(st *seqState) (class, message string) {
	var lines []string
	for i, mc := range w.cfg.members {
		m := &st.members[i]
		if w.finishedFor(m, st.rev) && m.count(seqLost, seqRebuild, seqCreating) == 0 && w.stagedOrDone(m, st.rev) {
			continue
		}
		var line string
		switch {
		case m.retry == seqRetryHeld:
			line = fmt.Sprintf("%s is Held with %s", mc.comp, m)
			if class == "" {
				class = "known-retries-spent"
			}
		case w.heldFailedReplacementStopsIt(st, i):
			line = fmt.Sprintf("%s (%s) holds on %s with a failed replacement under a held Instance", mc.comp, m, st.holds[i])
			if class == "" {
				class = "known-failed-replacement-under-a-held-instance-keeps-its-slot"
			}
		default:
			line = fmt.Sprintf("%s (%s) holds on %s", mc.comp, m, st.holds[i])
			switch class {
			case "":
				class = "no-way-out"
			case "known-retries-spent":
				class = "known-spent-retries-hold-the-chain"
			}
		}
		lines = append(lines, line)
	}
	if class == "" {
		class = "no-way-out"
	}
	return class, strings.Join(lines, "; ")
}

// heldFailedReplacementStopsIt reports a member whose next start the
// per-Component surge budget denies while a failed surge replacement sits
// under an Instance the partition holds, so no start ever frees its slot.
func (w *seqWorld) heldFailedReplacementStopsIt(st *seqState, i int) bool {
	m := &st.members[i]
	h := st.holds[i]
	if h == nil || h.gate != v1beta1.RolloutHoldGateBudget || !strings.Contains(h.reason, "surge budget") || m.inFlight() {
		return false
	}
	held := w.heldIndices(m)
	for idx, r := range m.rows {
		if r.kind == seqFailedSurge && held[idx] {
			return true
		}
	}
	return false
}

// stagedOrDone reports a member at its end: every Instance Ready on rev, or
// between one and its partition held Ready on another revision, since a
// partition holds what still runs the old revision, at most what it asks
// for.
func (w *seqWorld) stagedOrDone(m *seqMember, rev int8) bool {
	var off int32
	for _, r := range m.rows {
		if r.kind != seqReady {
			return false
		}
		if r.rev != rev {
			off++
		}
	}
	return off == 0 || (m.partition() > 0 && off <= m.partition())
}

// seqReport is what one configuration's exploration found.
type seqReport struct {
	states, depth, runs int
	findings            map[string]seqFinding
	counts              map[string]int
}

func (w *seqWorld) record(rep *seqReport, st *seqState) {
	for _, f := range w.check(st) {
		rep.counts[f.class]++
		if _, ok := rep.findings[f.class]; !ok {
			rep.findings[f.class] = f
		}
	}
}

func (w *seqWorld) explore(maxStates, maxDepth int) seqReport {
	rep := seqReport{findings: map[string]seqFinding{}, counts: map[string]int{}}
	start := w.initial()
	w.settle(start, w.order(0))
	visited := map[string]bool{start.key(): true}
	queue := []*seqState{start}
	for len(queue) > 0 && rep.states < maxStates {
		st := queue[0]
		queue = queue[1:]
		rep.states++
		if len(st.path) > rep.depth {
			rep.depth = len(st.path)
		}
		w.record(&rep, st)
		if w.complete(st) || len(st.path) >= maxDepth {
			if len(st.path) >= maxDepth && !w.complete(st) {
				rep.counts["depth"]++
				if _, ok := rep.findings["depth"]; !ok {
					rep.findings["depth"] = seqFinding{class: "depth", message: "the run did not end within the depth bound", state: st}
				}
			}
			continue
		}
		for _, mv := range append(w.progressMoves(st), w.adverseMoves(st)...) {
			child := w.apply(st, mv)
			k := child.key()
			if visited[k] {
				continue
			}
			visited[k] = true
			queue = append(queue, child)
		}
	}
	return rep
}

// randomRuns plays whole runs with a fixed seed, favouring the moves that
// finish a roll, to reach the depths the breadth-first walk does not.
func (w *seqWorld) randomRuns(rep *seqReport, seed int64, runs, maxDepth int) {
	rng := rand.New(rand.NewSource(seed))
	for r := 0; r < runs; r++ {
		st := w.initial()
		w.settle(st, w.order(0))
		rep.runs++
		for depth := 0; ; depth++ {
			w.record(rep, st)
			if w.complete(st) {
				break
			}
			progress, adverse := w.progressMoves(st), w.adverseMoves(st)
			if len(progress) == 0 && len(adverse) == 0 {
				break
			}
			if depth >= maxDepth {
				rep.counts["depth"]++
				if _, ok := rep.findings["depth"]; !ok {
					rep.findings["depth"] = seqFinding{class: "depth", message: "the run did not end within the depth bound", state: st}
				}
				break
			}
			var mv seqMove
			if len(progress) == 0 || (len(adverse) > 0 && rng.Intn(4) == 0) {
				mv = adverse[rng.Intn(len(adverse))]
			} else {
				mv = progress[rng.Intn(len(progress))]
			}
			st = w.apply(st, mv)
			if len(st.path) > rep.depth {
				rep.depth = len(st.path)
			}
		}
	}
}

// seqChain builds a configuration: the members in chain order, each with
// the given strategy and budget, a router of two behind them when asked.
func seqChain(shape []int32, comps []v1beta1.ComponentType, strategy v1beta1.UpdateStrategyType, budget string, partitions []int32, wait time.Duration, retries int32, budgets seqBudgets) seqConfig {
	cfg := seqConfig{afterReady: wait, maxAttempts: retries, budgets: budgets}
	for i, comp := range comps {
		var p int32
		if i < len(partitions) {
			p = partitions[i]
		}
		cfg.members = append(cfg.members, seqMemberConfig{comp: comp, desired: shape[i], strategy: strategy, budget: budget, partition: p})
	}
	cfg.name = cfg.String()
	return cfg
}

func seqDefaultBudgets(full bool) seqBudgets {
	b := seqBudgets{fails: 1, losses: 1, scales: 1, partitionEdits: 1, steps: 1, pushes: 1, rollbacks: 1}
	if full {
		b.fails, b.losses = 2, 2
	}
	return b
}

// seqConfigs lists the configurations to explore: the reference chains
// under both strategies, both budgets, with and without a router, the
// partition on the first member, the second, both or none, with and without
// a wait after readiness; then a seeded sample over the whole space.
func seqConfigs(full bool) []seqConfig {
	eng, dec, rtr := v1beta1.EngineComponent, v1beta1.DecoderComponent, v1beta1.RouterComponent
	recreate, surge := v1beta1.UpdateStrategyRecreatePod, v1beta1.UpdateStrategySurgeThenDrain
	budgets := seqDefaultBudgets(full)
	var out []seqConfig
	pairs := [][2]int32{{12, 8}, {6, 2}}
	if full {
		pairs = append(pairs, [2]int32{16, 8})
	}
	partitionSets := [][]int32{nil, {2, 0}, {0, 2}, {2, 2}}
	for _, shape := range pairs {
		for _, strategy := range []v1beta1.UpdateStrategyType{recreate, surge} {
			for _, partitions := range partitionSets {
				for _, wait := range []time.Duration{0, 10 * time.Minute} {
					out = append(out, seqChain(shape[:], []v1beta1.ComponentType{eng, dec}, strategy, "10%", partitions, wait, 3, budgets))
				}
			}
			out = append(out, seqChain([]int32{shape[0], shape[1], 2}, []v1beta1.ComponentType{eng, dec, rtr}, strategy, "10%", []int32{2, 0, 0}, 10*time.Minute, 3, budgets))
			if full {
				out = append(out, seqChain(shape[:], []v1beta1.ComponentType{eng, dec}, strategy, "1", []int32{0, 2}, 10*time.Minute, 3, budgets))
				out = append(out, seqChain([]int32{shape[0], shape[1], 2}, []v1beta1.ComponentType{eng, dec, rtr}, strategy, "1", nil, 0, 3, budgets))
			}
		}
	}
	for _, engines := range []int32{4, 8} {
		for _, strategy := range []v1beta1.UpdateStrategyType{recreate, surge} {
			for _, partitions := range [][]int32{nil, {1, 0}} {
				out = append(out, seqChain([]int32{engines, 2}, []v1beta1.ComponentType{eng, rtr}, strategy, "1", partitions, 10*time.Minute, 3, budgets))
			}
			if full {
				out = append(out, seqChain([]int32{engines, 2}, []v1beta1.ComponentType{eng, rtr}, strategy, "10%", nil, 0, 1, budgets))
			}
		}
	}
	rng := rand.New(rand.NewSource(20261019))
	sample := 40
	if full {
		sample = 120
	}
	shapes := [][]int32{{12, 8}, {16, 8}, {6, 2}, {4, 2}, {8, 2}, {2, 6}, {3, 3}, {1, 1}}
	for n := 0; n < sample; n++ {
		shape := append([]int32(nil), shapes[rng.Intn(len(shapes))]...)
		comps := []v1beta1.ComponentType{eng, dec}
		if rng.Intn(3) == 0 {
			comps = []v1beta1.ComponentType{dec, eng}
		}
		if rng.Intn(3) == 0 {
			shape = append(shape, 2)
			comps = append(comps, rtr)
		}
		strategy := []v1beta1.UpdateStrategyType{recreate, surge}[rng.Intn(2)]
		budget := []string{"1", "10%"}[rng.Intn(2)]
		partitions := make([]int32, len(comps))
		for i := range partitions {
			if rng.Intn(3) == 0 {
				partitions[i] = []int32{1, 2, shape[i] / 2}[rng.Intn(3)]
			}
		}
		var wait time.Duration
		if rng.Intn(2) == 0 {
			wait = 10 * time.Minute
		}
		retries := int32(3)
		if rng.Intn(4) == 0 {
			retries = 1
		}
		cfg := seqChain(shape, comps, strategy, budget, partitions, wait, retries, budgets)
		// A mixed chain: the second member rolls the other way.
		if rng.Intn(3) == 0 {
			other := surge
			if strategy == surge {
				other = recreate
			}
			cfg.members[1].strategy = other
			cfg.name = cfg.String()
		}
		out = append(out, cfg)
	}
	return out
}

// seqExplore explores every configuration keep admits and reports each
// broken invariant with the shortest path to it. A finding of class
// no-way-out (a state that cannot move with nothing known explaining it),
// order (a later Component started before an earlier one finished), budget,
// depth or unsettled fails the test; the known classes are reported with
// their reading.
func seqExplore(t *testing.T, keep func(seqConfig) bool) {
	t.Helper()
	full := os.Getenv("OME_SEQUENTIAL_HUNT") == "full"
	maxStates, maxDepth, runs := 80, 40, 6
	if full {
		maxStates, maxDepth, runs = 240, 60, 8
	}
	var configs []seqConfig
	for _, cfg := range seqConfigs(full) {
		if keep(cfg) {
			configs = append(configs, cfg)
		}
	}
	started := time.Now()
	totals := map[string]int{}
	configsWith := map[string]int{}
	var states, gateCalls, allRuns int
	type example struct {
		cfg     seqConfig
		finding seqFinding
	}
	examples := map[string][]example{}
	for n, cfg := range configs {
		w := newSeqWorld(t, cfg)
		rep := w.explore(maxStates, maxDepth)
		w.randomRuns(&rep, 1, runs, 4*maxDepth)
		states += rep.states
		gateCalls += w.gateCalls
		allRuns += rep.runs
		if full {
			var parts []string
			for class, c := range rep.counts {
				parts = append(parts, fmt.Sprintf("%s=%d", class, c))
			}
			sort.Strings(parts)
			t.Logf("config %d/%d [%s]: %d states, depth %d: %s", n+1, len(configs), cfg.name, rep.states, rep.depth, strings.Join(parts, " "))
		}
		for class, c := range rep.counts {
			totals[class] += c
		}
		for class, f := range rep.findings {
			configsWith[class]++
			if len(examples[class]) < 8 {
				examples[class] = append(examples[class], example{cfg: cfg, finding: f})
			}
		}
	}
	classes := make([]string, 0, len(totals))
	for class := range totals {
		classes = append(classes, class)
	}
	sort.Strings(classes)
	t.Logf("%d configurations, %d states, %d random runs, %d gate questions, %s", len(configs), states, allRuns, gateCalls, time.Since(started).Round(time.Second))
	for _, class := range classes {
		if known, ok := seqKnownClasses[class]; ok {
			t.Logf("%s: %d states across %d configurations: %s", class, totals[class], configsWith[class], known)
		} else {
			t.Logf("%s: %d states across %d configurations", class, totals[class], configsWith[class])
		}
		for _, ex := range examples[class] {
			t.Logf("  [%s]\n    %s\n    path: %s\n    state: %s", ex.cfg.name, ex.finding.message, seqPath(ex.finding.state), ex.finding.state)
		}
		if _, ok := seqKnownClasses[class]; !ok {
			t.Errorf("%s: %d states (first: %s: %s)", class, totals[class], examples[class][0].cfg.name, examples[class][0].finding.message)
		}
	}
}

// TestSequentialChains_TwoComponentChainsReachTheirEnd explores every chain
// of two Components: the engine then the decoder on the reference shapes,
// an engine then a router, under both strategies and budgets, with a
// partition on either member or both, with and without a wait after
// readiness; failures, lost pods, spent retries, scales, partition edits,
// canary steps, a second push and a push back to version 1 thrown in.
func TestSequentialChains_TwoComponentChainsReachTheirEnd(t *testing.T) {
	seqExplore(t, func(c seqConfig) bool { return len(c.members) == 2 })
}

// TestSequentialChains_ThreeComponentChainsReachTheirEnd explores the same
// space with a router of two as the chain's last member.
func TestSequentialChains_ThreeComponentChainsReachTheirEnd(t *testing.T) {
	seqExplore(t, func(c seqConfig) bool { return len(c.members) == 3 })
}

// serveAll brings every started, created or rebuilt Instance up, the
// projector reaching every member first, until nothing is in flight.
func (w *seqWorld) serveAll(st *seqState) *seqState {
	for round := 0; round < 128; round++ {
		var mv *seqMove
		for _, candidate := range w.progressMoves(st) {
			if candidate.kind == "project" || candidate.kind == "serve" {
				c := candidate
				mv = &c
				break
			}
		}
		if mv == nil {
			return st
		}
		st = w.apply(st, *mv)
	}
	w.t.Fatalf("the fleet did not come up in 128 rounds: %s", st)
	return st
}

// serveOn brings n started Instances of one member up, lowest index first.
func (w *seqWorld) serveOn(st *seqState, member int, n int) *seqState {
	for k := 0; k < n; k++ {
		served := false
		for idx, r := range st.members[member].rows {
			if r.open() {
				st = w.apply(st, seqMove{kind: "serve", member: member, idx: idx})
				served = true
				break
			}
		}
		if !served {
			w.t.Fatalf("member %d has no started Instance to serve: %s", member, st)
		}
	}
	return st
}

// projectAll lets the projector's pass reach every member.
func (w *seqWorld) projectAll(st *seqState) *seqState {
	for i := range st.members {
		if st.members[i].annotation != st.gen {
			return w.apply(st, seqMove{kind: "project"})
		}
	}
	return st
}

// TestSequentialChains_APartitionRaisedAboveTheMovedCountRestsStagedAndReleasesTheChain
// records a partition that asks to hold more Instances than still run the old
// version. 12 engines then 8 decoders, both RecreatePod at 10 percent. The
// operator pushes version 2; six engines serve it and two more are moving.
// The operator raises the engine's partition to 8, asking to keep 8 on
// version 1. The two started engines finish, so 8 run version 2 and 4 run
// version 1: the partition holds the 4, the engine rests staged and Ready
// at that shape, and the decoders roll to the end. The second picture is
// the same shape reached by a scale: with a partition of 6 set before the
// push the engine rolls 6, rests staged and the decoders start; the engine
// is then scaled to 4, which retires the 6 moved Instances and leaves 4 on
// version 1 under a partition of 6; the engine rests staged at the 4 and the
// decoders finish. Lowering the partition to 4 afterwards changes nothing.
func TestSequentialChains_APartitionRaisedAboveTheMovedCountRestsStagedAndReleasesTheChain(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	budgets := seqBudgets{partitionEdits: 2, scales: 1}
	for _, tc := range []struct {
		name          string
		before        []int32
		edit, release seqMove
	}{
		{
			name: "the partition raised to 8 after 6 engines moved",
			edit: seqMove{kind: "partition", member: 0, value: 8}, release: seqMove{kind: "partition", member: 0, value: 4},
		},
		{
			name: "the engine scaled from 12 to 4 under a partition of 6", before: []int32{6, 0},
			edit: seqMove{kind: "scale", member: 0, value: 4}, release: seqMove{kind: "partition", member: 0, value: 4},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newSeqWorld(t, seqChain([]int32{12, 8}, []v1beta1.ComponentType{eng, dec}, v1beta1.UpdateStrategyRecreatePod, "10%", tc.before, 0, 3, budgets))
			st := w.initial()
			w.settle(st, w.order(0))
			st = w.projectAll(st)
			st = w.serveOn(st, 0, 6)
			st = w.apply(st, tc.edit)
			st = w.projectAll(st)
			st = w.serveAll(st)
			engine := st.members[0]
			var held int32
			for _, r := range engine.rows {
				if r.rev != st.rev {
					held++
				}
			}
			if !w.complete(st) || !engine.readyTrue || held != 4 || st.holds[0] != nil || st.holds[1] != nil {
				t.Fatalf("the engine must rest staged and Ready with the 4 Instances still on version 1 held and the decoders done, got %s", st)
			}

			// Lowering the partition to the count still on version 1 changes
			// nothing: the shape is already the one it asks for.
			st = w.apply(st, tc.release)
			st = w.projectAll(st)
			if !w.complete(st) || st.members[0].inFlight() || st.members[1].inFlight() {
				t.Fatalf("the chain must stay at its end once the partition matches what still runs version 1, got %s", st)
			}
		})
	}
}

// TestSequentialChains_AStagedMemberReadsAsDoneUntilItsStatusRecordsTheNextPush
// records the current behaviour the exploration reports as
// known-member-at-its-old-shape-read-as-done-after-a-push. 12 engines, then 8 decoders of which 2 stay on version 1, then 2
// routers. Version 2 rolls in order and rests with the decoder staged: 6
// decoders on version 2, 2 held on version 1, its current revision still
// version 1. The operator pushes version 3 and the engines finish it. The
// projector writes the decoder's and the router's new spec; the router's
// replica reconciles before the decoder's status records version 3. The
// gate reads the decoder's status: update revision version 2, current
// revision version 1, and 6 of 8 on version 2 with 2 held, the staged
// shape, so it reads the decoder as done and admits the router's first
// start toward version 3 while the 6 decoders still have to move. Once the
// decoder's status records version 3 the same question holds the router.
// This reading is not decided yet; the test records the current behaviour
// and is updated when it changes.
func TestSequentialChains_AStagedMemberReadsAsDoneUntilItsStatusRecordsTheNextPush(t *testing.T) {
	eng, dec, rtr := v1beta1.EngineComponent, v1beta1.DecoderComponent, v1beta1.RouterComponent
	w := newSeqWorld(t, seqChain([]int32{12, 8, 2}, []v1beta1.ComponentType{eng, dec, rtr}, v1beta1.UpdateStrategyRecreatePod, "10%", []int32{0, 2, 0}, 0, 3, seqBudgets{pushes: 1}))
	st := w.initial()
	w.settle(st, w.order(0))
	st = w.serveAll(st)
	if !w.complete(st) || st.members[1].current != 1 || st.members[1].statusTarget != 2 {
		t.Fatalf("the picture is wrong: version 2 must have rolled with the decoder staged at 2 on version 1, got %s", st)
	}
	st = w.apply(st, seqMove{kind: "push"})
	// The projector reaches the engine; the engines finish version 3.
	next := st.clone()
	w.project(next, 0)
	w.settle(next, w.order(0))
	st = next
	for !w.finishedFor(&st.members[0], 3) {
		st = w.serveOn(st, 0, 1)
	}
	// The projector reaches the decoder and the router; the router's
	// replica runs its pass before the decoder's status write.
	next = st.clone()
	w.project(next, 1)
	w.project(next, 2)
	admitted, hold := w.pass(next, 2)
	decoder := next.members[1]
	if decoder.statusTarget != 2 || decoder.current != 1 {
		t.Fatalf("the picture is wrong: the decoder's status must still record version 2 over version 1, got %s", decoder)
	}
	if admitted == 0 {
		t.Fatalf("the router was held (%s) while the decoder's status still records version 2; the current behaviour admits the router's start, and this test is updated when it changes", hold)
	}
	t.Logf("known-member-at-its-old-shape-read-as-done-after-a-push: %s (router: %s; decoder: %s)", seqKnownClasses["known-member-at-its-old-shape-read-as-done-after-a-push"], next.members[2], decoder)

	// Once the decoder's status records version 3, the router is held.
	later := st.clone()
	w.project(later, 1)
	w.project(later, 2)
	w.pass(later, 1)
	admitted, hold = w.pass(later, 2)
	if admitted != 0 || hold == nil || hold.gate != v1beta1.RolloutHoldGateSequential || hold.reason != "Sequential waiting on decoder" {
		t.Fatalf("after the decoder's status write the router must wait on the decoder, got admitted=%d hold=%s", admitted, hold)
	}
}

// TestSequentialChains_SpentRetriesOnTheFirstComponentHoldTheSecondUnderItsName
// records the engine's known failure hold: 12 engines then 8 decoders under
// RecreatePod at 10 percent; the engine's first start fails three times and
// its retries are spent. The engine reports "update held after 3 failed
// attempt(s)" and the decoders wait under "Sequential waiting on engine",
// which names the Component and not its spent retries. The KIND spec of the
// Sequential failure hold pins the same behaviour on a cluster.
func TestSequentialChains_SpentRetriesOnTheFirstComponentHoldTheSecondUnderItsName(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	w := newSeqWorld(t, seqChain([]int32{12, 8}, []v1beta1.ComponentType{eng, dec}, v1beta1.UpdateStrategyRecreatePod, "10%", nil, 0, 1, seqBudgets{fails: 1}))
	st := w.initial()
	w.settle(st, w.order(0))
	st = w.projectAll(st)
	st = w.apply(st, seqMove{kind: "fail", member: 0, idx: 0})
	st = w.serveAll(st)
	if st.members[0].retry != seqRetryHeld {
		t.Fatalf("the picture is wrong: the engine's retries must be spent, got %s", st)
	}
	if h := st.holds[0]; h == nil || h.gate != v1beta1.RolloutHoldGateHeld {
		t.Fatalf("the engine's hold = %s, want Held", h)
	}
	if h := st.holds[1]; h == nil || h.gate != v1beta1.RolloutHoldGateSequential || h.reason != "Sequential waiting on engine" {
		t.Fatalf("the decoder's hold = %s, want Sequential \"Sequential waiting on engine\"", h)
	}
	if len(w.progressMoves(st)) != 0 || w.complete(st) {
		t.Fatalf("the chain must stand still, got moves %v (%s)", w.progressMoves(st), st)
	}
	t.Logf("known-spent-retries-hold-the-chain: %s (engine: %s; decoder holds on %s)", seqKnownClasses["known-spent-retries-hold-the-chain"], st.members[0], st.holds[1])
}

// TestSequentialChains_AScaleUpOnTheWaitingComponentBringsUpTheNewVersionEarly
// records behaviour not decided yet, as the engine reads it now: 12 engines then 8
// decoders; the engine rolls, and the decoder is scaled from 8 to 9. The
// ninth decoder is created on version 2 and serves it while the engines are
// still moving. The gate holds starts, not creations.
func TestSequentialChains_AScaleUpOnTheWaitingComponentBringsUpTheNewVersionEarly(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	w := newSeqWorld(t, seqChain([]int32{12, 8}, []v1beta1.ComponentType{eng, dec}, v1beta1.UpdateStrategyRecreatePod, "10%", nil, 0, 3, seqBudgets{scales: 1}))
	st := w.initial()
	w.settle(st, w.order(0))
	st = w.projectAll(st)
	st = w.serveOn(st, 0, 2)
	st = w.apply(st, seqMove{kind: "scale", member: 1, value: 9})
	st = w.projectAll(st)
	decoder := st.members[1]
	if n := len(decoder.rows); n != 9 || decoder.rows[8].kind != seqCreating {
		t.Fatalf("the picture is wrong: the ninth decoder must be building, got %s", decoder)
	}
	if decoder.rows[8].to != 2 {
		t.Fatalf("the ninth decoder builds toward version %d; the current behaviour builds it toward version 2, and this test is updated when it changes", decoder.rows[8].to)
	}
	st = w.apply(st, seqMove{kind: "serve", member: 1, idx: 8})
	if r := st.members[1].rows[8]; r.kind != seqReady || r.rev != 2 || w.finishedFor(&st.members[0], 2) {
		t.Fatalf("the picture is wrong: the ninth decoder must serve version 2 while the engine is unfinished, got %s", st)
	}
	t.Logf("known-scale-up-ahead: %s (decoder: %s; engine: %s)", seqKnownClasses["known-scale-up-ahead"], st.members[1], st.members[0])
}

// TestSequentialChains_APodLostOnTheFinishedComponentRestartsTheWait records
// which clock the wait reads, not decided yet, as the engine reads it now:
// 12 engines under SurgeThenDrain then 8 decoders, with a ten minute
// wait between them. The engines finish, the wait passes and the decoders
// start. One engine pod is lost: a surging member declares no
// maxUnavailable, so one missing pod takes the engine under its serving
// minimum and its Ready condition turns False, then True again once the pod
// is rebuilt. The decoder's next start waits out the ten minutes again under
// "Sequential.Soak: decoder waiting out the 10m0s soak after engine".
func TestSequentialChains_APodLostOnTheFinishedComponentRestartsTheWait(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	w := newSeqWorld(t, seqChain([]int32{12, 8}, []v1beta1.ComponentType{eng, dec}, v1beta1.UpdateStrategySurgeThenDrain, "10%", nil, 10*time.Minute, 3, seqBudgets{losses: 1}))
	w.settleOrder = []int{0, 1} // the engine's status write lands before the decoder asks
	st := w.initial()
	w.settle(st, w.order(0))
	st = w.projectAll(st)
	for !w.finishedFor(&st.members[0], 2) {
		st = w.serveOn(st, 0, 1)
	}
	if h := st.holds[1]; h == nil || !strings.Contains(h.reason, "Sequential.Soak") {
		t.Fatalf("the picture is wrong: the decoder must wait out the ten minutes, got hold %s (%s)", h, st)
	}
	st = w.apply(st, seqMove{kind: "wait", member: 0})
	if !st.members[1].inFlight() {
		t.Fatalf("the picture is wrong: the decoders must start once the wait passed, got %s", st)
	}
	st = w.serveOn(st, 1, 1)
	st = w.apply(st, seqMove{kind: "lose", member: 0, idx: 0})
	if st.members[0].readyTrue {
		t.Fatalf("the picture is wrong: one lost pod must take a surging engine under its serving minimum, got %s", st.members[0])
	}
	st = w.serveAll(st)
	if h := st.holds[1]; h == nil || h.gate != v1beta1.RolloutHoldGateSequential || h.reason != "Sequential.Soak: decoder waiting out the 10m0s soak after engine" {
		t.Fatalf("the decoder's hold = %s; the current behaviour starts the wait over, and this test is updated when it changes", h)
	}
	t.Logf("known-wait-restarts-after-a-pod-loss: %s (decoder: %s holds on %s)", seqKnownClasses["known-wait-restarts-after-a-pod-loss"], st.members[1], st.holds[1])
}

// TestSequentialChains_TheWaitAfterReadinessIsMeasuredAfterAStagedComponent
// records the wait after readiness behind a staged Component. 12 engines of
// which 2 stay on version 1, then 8 decoders, with a ten minute wait between
// them. The ten movable engines serve version 2 and the engine rests staged,
// Ready, with its current revision at version 1 by design. The decoders wait
// out the ten minutes from the engine's Ready transition and start once the
// wait has passed. The same chain with no partition waits the same way.
func TestSequentialChains_TheWaitAfterReadinessIsMeasuredAfterAStagedComponent(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	w := newSeqWorld(t, seqChain([]int32{12, 8}, []v1beta1.ComponentType{eng, dec}, v1beta1.UpdateStrategyRecreatePod, "10%", []int32{2, 0}, 10*time.Minute, 3, seqBudgets{}))
	w.settleOrder = []int{0, 1}
	st := w.initial()
	w.settle(st, w.order(0))
	st = w.projectAll(st)
	for !w.finishedFor(&st.members[0], 2) {
		st = w.serveOn(st, 0, 1)
	}
	engine := st.members[0]
	if !engine.readyTrue || engine.waited || engine.current != 1 || engine.statusTarget != 2 {
		t.Fatalf("the picture is wrong: the engine must rest staged and Ready with its current revision at version 1, got %s", engine)
	}
	if h := st.holds[1]; st.members[1].inFlight() || h == nil || h.reason != "Sequential.Soak: decoder waiting out the 10m0s soak after engine" {
		t.Fatalf("the decoder must wait out the ten minutes after the staged engine, got %s (hold %s)", st.members[1], h)
	}
	st = w.apply(st, seqMove{kind: "wait", member: 0})
	if !st.members[1].inFlight() {
		t.Fatalf("the decoders must start once the wait passed, got %s", st)
	}

	// Without the partition the same chain waits.
	w = newSeqWorld(t, seqChain([]int32{12, 8}, []v1beta1.ComponentType{eng, dec}, v1beta1.UpdateStrategyRecreatePod, "10%", nil, 10*time.Minute, 3, seqBudgets{}))
	w.settleOrder = []int{0, 1}
	st = w.initial()
	w.settle(st, w.order(0))
	st = w.projectAll(st)
	for !w.finishedFor(&st.members[0], 2) {
		st = w.serveOn(st, 0, 1)
	}
	if h := st.holds[1]; st.members[1].inFlight() || h == nil || h.reason != "Sequential.Soak: decoder waiting out the 10m0s soak after engine" {
		t.Fatalf("the picture is wrong: without a partition the decoder must wait, got %s (hold %s)", st.members[1], h)
	}
}

// TestSequentialChains_ThePartitionOnTheFirstComponentHandsOverOnceStaged
// pins the designed reading beside the finding above: with a partition of 2
// set before the push on 12 engines, the engine rolls 10, rests staged, and
// the decoders start; the chain ends with 2 engines on version 1 and the
// decoder's hold cleared.
func TestSequentialChains_ThePartitionOnTheFirstComponentHandsOverOnceStaged(t *testing.T) {
	eng, dec := v1beta1.EngineComponent, v1beta1.DecoderComponent
	w := newSeqWorld(t, seqChain([]int32{12, 8}, []v1beta1.ComponentType{eng, dec}, v1beta1.UpdateStrategyRecreatePod, "10%", []int32{2, 0}, 0, 3, seqBudgets{}))
	st := w.initial()
	w.settle(st, w.order(0))
	st = w.serveAll(st)
	if !w.complete(st) {
		t.Fatalf("the chain did not end: %s", st)
	}
	engine := st.members[0]
	var old int32
	for _, r := range engine.rows {
		if r.rev == 1 {
			old++
		}
	}
	if old != 2 || !engine.readyTrue || st.holds[1] != nil {
		t.Fatalf("want 2 engines on version 1, the engine Ready and the decoder's hold cleared, got %s", st)
	}
}
