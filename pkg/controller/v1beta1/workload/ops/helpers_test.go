package ops

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// This package's test names for the fixture package's builders.
const testRevisionHashLegacy = fixture.InstanceRevisionHash

var (
	legacyBoolPtr                       = ptr.To[bool]
	legacyComponentPlan                 = fixture.SinglePodPlan
	legacyEngineRevisionKey             = fixture.EngineRevisionKey
	legacyEnsureTargetCR                = fixture.EnsureTargetRevision
	legacyEnsureTargetCRWithMeta        = fixture.EnsureTargetRevisionWithMeta
	legacyFoldServingIntoPodReady       = fixture.MarkPodReadyAt
	legacyFromV1beta1Op                 = fixture.OperationFromAPI
	legacyInstanceIR                    = fixture.InferenceReplica
	legacyInstanceStatuses              = fixture.InstanceStatuses
	legacyInstanceStatusesOnIR          = fixture.StoredInstanceStatuses
	legacyIRName                        = fixture.InferenceReplicaName
	legacyISVCReadyAtIncarnation        = fixture.ServiceWithReadyInstance
	legacyMinimalISVC                   = fixture.MinimalInferenceService
	legacyMultiPodComponentPlan         = fixture.LeaderWorkerPlan
	legacyNewFakeClient                 = fixture.FakeClient
	legacyNewFakeClientWithInterceptors = fixture.FakeClientWithInterceptors
	legacyPodAtIncarnation              = fixture.PodAtRecreation
	legacyPodForInstance                = fixture.PodForInstance
	legacyRemoveInstance                = fixture.RemoveInstanceOnReplica
	legacyResetExpectations             = fixture.ResetExpectations
	legacyRunningPodAtRevision          = fixture.ServingPodWithImage
	legacySeedRunningRevision           = fixture.SeedRunningRevision
	legacySeedRunningRevisionWithMeta   = fixture.SeedRunningRevisionWithMeta
	legacySliceWithEndpoint             = fixture.EndpointSliceForPod
	legacyStampPodRevisionHash          = fixture.LabelPodWithRevision
	legacyStoredRevision                = fixture.StoredRevision
	legacyTargetSpecImage               = fixture.PodSpecWithImage
	legacyTestDeps                      = fixture.Deps
	legacyTestInput                     = fixture.ReconcileInput
	legacyTestPodLabels                 = fixture.PodLabels
	legacyToV1beta1Op                   = fixture.OperationToAPI
)

type terminalMutationStore struct {
	ownerUID   k8stypes.UID
	statuses   map[int32]workload.InstanceStatus
	retryBlock map[string]workload.RetryBlock
	readErr    error
	applyErr   error
	applyCall  int
	writes     int
	log        *[]string
}

func (s *terminalMutationStore) apply(
	_ context.Context,
	mutations []workload.InstanceMutation,
	targetRevision string,
	mutateRetryBlock func(*workload.RetryBlock) workload.RetryBlockDisposition,
) error {
	s.applyCall++
	if s.log != nil {
		*s.log = append(*s.log, "status")
	}
	if s.readErr != nil {
		return s.readErr
	}
	snapshot := workload.InstanceMutationSnapshot{
		OwnerUID:  s.ownerUID,
		Instances: cloneTerminalStatuses(s.statuses),
	}
	for _, mutation := range mutations {
		if mutation.BatchPrecondition != nil && !mutation.BatchPrecondition(snapshot) {
			return workload.ErrStatusMutationPrecondition
		}
	}

	next := cloneTerminalStatuses(s.statuses)
	type callback struct {
		fn      func(*workload.InstanceStatus, *workload.InstanceStatus)
		before  *workload.InstanceStatus
		current *workload.InstanceStatus
	}
	callbacks := make([]callback, 0, len(mutations))
	changed := false
	for _, mutation := range mutations {
		current, found := next[mutation.Index]
		if mutation.Remove {
			if !found {
				continue
			}
			before := cloneTerminalStatus(current)
			if mutation.Precondition != nil && !mutation.Precondition(&before) {
				continue
			}
			delete(next, mutation.Index)
			changed = true
			if mutation.OnCommit != nil {
				callbacks = append(callbacks, callback{fn: mutation.OnCommit, before: &before})
			}
			continue
		}
		if !found {
			current = workload.InstanceStatus{Index: mutation.Index}
		}
		before := cloneTerminalStatus(current)
		if mutation.Precondition != nil && !mutation.Precondition(&before) {
			continue
		}
		if !mutation.Mutate(&current) {
			continue
		}
		next[mutation.Index] = current
		changed = true
		if mutation.OnCommit != nil {
			committed := cloneTerminalStatus(current)
			callbacks = append(callbacks, callback{fn: mutation.OnCommit, before: &before, current: &committed})
		}
	}
	nextRetryBlocks := cloneTerminalRetryBlocks(s.retryBlock)
	retryBlockChanged := false
	if mutateRetryBlock != nil {
		block, found := nextRetryBlocks[targetRevision]
		if !found {
			block = workload.RetryBlock{TargetRevision: targetRevision}
		}
		switch mutateRetryBlock(&block) {
		case workload.RetryBlockPersist:
			block.TargetRevision = targetRevision
			nextRetryBlocks[targetRevision] = block
			retryBlockChanged = true
		case workload.RetryBlockRemove:
			if found {
				delete(nextRetryBlocks, targetRevision)
				retryBlockChanged = true
			}
		}
	}
	if !changed && !retryBlockChanged {
		return nil
	}
	if s.applyErr != nil {
		return s.applyErr
	}
	s.statuses = next
	s.retryBlock = nextRetryBlocks
	s.writes++
	for _, callback := range callbacks {
		callback.fn(callback.before, callback.current)
	}
	return nil
}

func cloneTerminalRetryBlocks(in map[string]workload.RetryBlock) map[string]workload.RetryBlock {
	out := make(map[string]workload.RetryBlock, len(in))
	for revision, block := range in {
		copy := block
		if block.NextRetryAt != nil {
			next := *block.NextRetryAt
			copy.NextRetryAt = &next
		}
		if block.FirstFailureAt != nil {
			first := *block.FirstFailureAt
			copy.FirstFailureAt = &first
		}
		if block.LastFailureAt != nil {
			last := *block.LastFailureAt
			copy.LastFailureAt = &last
		}
		out[revision] = copy
	}
	return out
}

func cloneTerminalStatuses(in map[int32]workload.InstanceStatus) map[int32]workload.InstanceStatus {
	out := make(map[int32]workload.InstanceStatus, len(in))
	for index, status := range in {
		out[index] = cloneTerminalStatus(status)
	}
	return out
}

func cloneTerminalStatus(status workload.InstanceStatus) workload.InstanceStatus {
	out := status
	if status.Operation != nil {
		operation := *status.Operation
		operation.HintTargetNodes = append([]string(nil), status.Operation.HintTargetNodes...)
		if status.Operation.SurgeIndex != nil {
			surge := *status.Operation.SurgeIndex
			operation.SurgeIndex = &surge
		}
		out.Operation = &operation
	}
	out.NodesOccupied = append([]string(nil), status.NodesOccupied...)
	out.Conditions = append([]metav1.Condition(nil), status.Conditions...)
	return out
}

func terminalStatusFixture(index int32) workload.InstanceStatus {
	surge := int32(9)
	started := metav1.NewTime(time.Date(2026, time.August, 15, 10, 0, 0, 987654321, time.UTC))
	return workload.InstanceStatus{
		Index:           index,
		Incarnation:     7,
		Phase:           workload.InstancePhaseUpdating,
		RunningRevision: "rev-old",
		TargetRevision:  "rev-new",
		ActiveOrdinal:   1,
		PodCount:        8,
		ReadyPodCount:   7,
		Operation: &workload.InstanceOperation{
			ID:              "gang-update-7",
			Type:            workload.InstanceOperationUpdate,
			Step:            workload.UpdateStepSurge,
			RequestUUID:     "request-a",
			TargetRevision:  "rev-new",
			RetryCount:      2,
			Reason:          "rollout",
			FromNode:        "node-a",
			HintTargetNodes: []string{"node-b", "node-c"},
			SurgeIndex:      &surge,
			StartedAt:       started,
			LastProgressAt:  started,
			Deadline:        metav1.NewTime(started.Add(time.Hour)),
		},
	}
}

// minReadyWindowStart is the fake "now" every test below anchors on.
var minReadyWindowStart = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// podReadyAt appends a PodReady=True condition whose lastTransitionTime is
// readyAt — the timestamp the availability rule measures from.
func podReadyAt(pod *corev1.Pod, readyAt time.Time) {
	pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
		Type:               corev1.PodReady,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(readyAt),
	})
}

// nonSurgeFixture is one Instance with a roll in flight and one pod. The
// caller decorates the pod, edits the input or the plan, then runs a pass
// and reads the row back.
type nonSurgeFixture struct {
	isvc       *v1beta1.InferenceService
	c          client.Client
	targetCR   *appsv1.ControllerRevision
	targetSpec *corev1.PodSpec
	pod        *corev1.Pod
	strategy   workload.UpdateStrategyType
	// blocks records every RetryBlock the pass asked to write.
	blocks *[]string
}

// inPlaceInFlightFixture seeds an in-place patch already stamped on the
// row: Phase=Updating, Op{Update, InPlace} at the target, and the pod it
// is patching still on the running revision's image.
func inPlaceInFlightFixture(t *testing.T) *nonSurgeFixture {
	t.Helper()
	legacyResetExpectations(t)
	isvc, ir, _ := inPlaceRollWithoutPods(t)
	ir.Status.InstanceStatuses[0].Operation.Strategy = string(v1beta1.UpdateStrategyInPlaceIfPossible)
	targetSpec := legacyTargetSpecImage("llama:v2")
	pod := legacyPodAtIncarnation(isvc, 0, 1, false /* not ready */, false /* not serving */)
	pod.Spec.Containers = []corev1.Container{{Name: "main", Image: "llama:v1"}}
	c := legacyNewFakeClient(t, isvc, ir, pod)
	legacySeedRunningRevision(t, c, isvc, workload.ComponentEngine, 0, legacyTargetSpecImage("llama:v1"))
	tcr := legacyEnsureTargetCR(t, c, isvc, targetSpec)
	return &nonSurgeFixture{
		isvc: isvc, c: c, targetCR: tcr, targetSpec: targetSpec, pod: pod,
		strategy: workload.UpdateStrategyInPlaceIfPossible,
	}
}

// recreateDrainInFlightFixture seeds a recreate already stamped on the
// row: Phase=Updating, Op{Update, Drain} at the bumped Incarnation. The
// pod the caller supplies decides which phase of the step the pass lands
// in — an old-incarnation pod keeps it in Phase A, a bumped one in C.
func recreateDrainInFlightFixture(t *testing.T) *nonSurgeFixture {
	t.Helper()
	legacyResetExpectations(t)
	isvc, ir := legacyISVCReadyAtIncarnation("llama-70b", "prod", 2)
	isvc.Spec.Engine.ComponentExtensionSpec.Lifecycle = &v1beta1.LifecycleSpec{
		UpdateStrategy: &v1beta1.UpdateStrategy{Type: v1beta1.UpdateStrategyRecreatePod},
	}
	targetSpec := legacyTargetSpecImage("llama:v2")
	c := legacyNewFakeClient(t, isvc, ir)
	legacySeedRunningRevision(t, c, isvc, workload.ComponentEngine, 0, legacyTargetSpecImage("llama:v1"))
	tcr := legacyEnsureTargetCR(t, c, isvc, targetSpec)

	live := &v1beta1.InferenceReplica{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(ir), live); err != nil {
		t.Fatalf("re-read IR: %v", err)
	}
	live.Status.InstanceStatuses[0].Phase = v1beta1.OMENativeInstanceUpdating
	live.Status.InstanceStatuses[0].Incarnation = 2
	live.Status.InstanceStatuses[0].TargetRevision = tcr.Name
	live.Status.InstanceStatuses[0].Operation = &v1beta1.InstanceOperation{
		ID:             "update-0-1",
		Type:           v1beta1.InstanceOperationUpdate,
		Step:           workload.UpdateStepDrain,
		TargetRevision: tcr.Name,
		Strategy:       string(v1beta1.UpdateStrategyRecreatePod),
		StartedAt:      metav1.NewTime(time.Now().Add(-time.Minute)),
		LastProgressAt: metav1.NewTime(time.Now().Add(-time.Minute)),
		Deadline:       metav1.NewTime(time.Now().Add(29 * time.Minute)),
	}
	if err := c.Status().Update(context.Background(), live); err != nil {
		t.Fatalf("seed the in-flight recreate: %v", err)
	}
	return &nonSurgeFixture{
		isvc: isvc, c: c, targetCR: tcr, targetSpec: targetSpec,
		strategy: workload.UpdateStrategyRecreatePod,
	}
}

// input builds a fresh ReconcileInput and wires the RetryBlock recorder.
func (f *nonSurgeFixture) input() workload.ReconcileInput {
	in := legacyTestInput(f.isvc, f.c, workload.ComponentEngine)
	in.ObservedState.UpdateRevision = f.targetCR.Name
	blocks := &[]string{}
	f.blocks = blocks
	in.MutateRetryBlock = func(_ context.Context, rev string, mutate func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
		b := workload.RetryBlock{TargetRevision: rev}
		if d := mutate(&b); d != workload.RetryBlockUnchanged {
			*blocks = append(*blocks, rev)
		}
		return nil
	}
	return in
}

// nonSurgeOutcome is everything a pass over an in-flight roll may move.
type nonSurgeOutcome struct {
	done        bool
	phase       v1beta1.OMENativeInstancePhase
	step        string
	incarnation int64
	failed      bool
	blocks      int
	podSurvives bool
	// podImage is the image on the observed pod after the pass, so an
	// edit that silently changed what was written to it is visible.
	podImage string
}

// run drives one pass with pods and reports what moved. tweak may edit
// the input and the plan the way an operator config change would.
func (f *nonSurgeFixture) run(t *testing.T, pods []*corev1.Pod, tweak func(*workload.ReconcileInput, *workload.ComponentPlan)) nonSurgeOutcome {
	t.Helper()
	in := f.input()
	plan := legacyComponentPlan(f.strategy, nil)
	if tweak != nil {
		tweak(&in, &plan)
	}
	deps := workload.Deps{Client: f.c, Recorder: record.NewFakeRecorder(32)}
	done, err := UpdateWithPods(context.Background(), deps, in, plan, plan.Instances[0], f.targetCR, f.targetSpec, pods)
	if err != nil {
		t.Fatalf("UpdateWithPods: %v", err)
	}
	s := legacyInstanceStatusesOnIR(f.c, f.isvc, workload.ComponentEngine)[0]
	out := nonSurgeOutcome{
		done:        done,
		phase:       s.Phase,
		incarnation: s.Incarnation,
		failed:      s.LastFailure != nil,
		blocks:      len(*f.blocks),
		podSurvives: true,
	}
	if s.Operation != nil {
		out.step = s.Operation.Step
	}
	if len(pods) > 0 {
		fresh := &corev1.Pod{}
		err := f.c.Get(context.Background(), client.ObjectKeyFromObject(pods[0]), fresh)
		out.podSurvives = err == nil
		if err == nil && len(fresh.Spec.Containers) > 0 {
			out.podImage = fresh.Spec.Containers[0].Image
		}
	}
	return out
}

// terminatingPod deletes pod through a finalizer so the fake client keeps
// the object with a deletionTimestamp — what the live List hands a pass
// over a pod the apiserver has not collected yet.
func terminatingPod(t *testing.T, c client.Client, pod *corev1.Pod) *corev1.Pod {
	t.Helper()
	ctx := context.Background()
	live := &corev1.Pod{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(pod), live); err != nil {
		t.Fatalf("re-read pod %s: %v", pod.Name, err)
	}
	live.Finalizers = append(live.Finalizers, "ome.io/test-hold")
	if err := c.Update(ctx, live); err != nil {
		t.Fatalf("pin pod %s with a finalizer: %v", pod.Name, err)
	}
	if err := c.Delete(ctx, live); err != nil {
		t.Fatalf("delete pod %s: %v", pod.Name, err)
	}
	out := &corev1.Pod{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(pod), out); err != nil {
		t.Fatalf("re-read terminating pod %s: %v", pod.Name, err)
	}
	if out.DeletionTimestamp == nil {
		t.Fatalf("pod %s did not become Terminating", pod.Name)
	}
	return out
}

// configChange is one operator-config edit under a short name.
type configChange struct {
	name  string
	tweak func(*workload.ReconcileInput, *workload.ComponentPlan)
}

// configChangeCases is the set of knobs an in-flight roll must not read.
// retryBlockHistoryLimit has no field here on purpose:
// it is consumed by the status writer's end-of-pass prune, so what an
// update pass owes it is to write no RetryBlock at all — which every
// case here asserts through the outcome's block count.
func configChangeCases() []configChange {
	return []configChange{
		{
			name: "update retry policy",
			tweak: func(in *workload.ReconcileInput, _ *workload.ComponentPlan) {
				in.UpdateRetryPolicy = &workload.RetryPolicy{
					MaxAttempts: 7, InitialDelay: time.Minute, MaxDelay: time.Hour, Multiplier: 2,
				}
			},
		},
		{
			name: "force-delete policy",
			tweak: func(in *workload.ReconcileInput, _ *workload.ComponentPlan) {
				in.ForceDelete = fdPolicy()
			},
		},
		{
			name: "scale-down cadence",
			tweak: func(in *workload.ReconcileInput, _ *workload.ComponentPlan) {
				in.ScaleDownRequeueInterval = 90 * time.Second
			},
		},
		{
			name: "requeue cadence",
			tweak: func(in *workload.ReconcileInput, _ *workload.ComponentPlan) {
				in.Requeue = workload.RequeueIntervals{Operation: 11 * time.Minute, Gate: 7 * time.Minute}
			},
		},
		{
			name: "gang schedule clamp",
			tweak: func(_ *workload.ReconcileInput, plan *workload.ComponentPlan) {
				plan.GangScheduleTimeout = &workload.GangScheduleTimeoutClamp{Min: time.Minute, Max: 10 * time.Minute}
			},
		},
		{
			name: "migration audit caps",
			tweak: func(in *workload.ReconcileInput, _ *workload.ComponentPlan) {
				in.MigrationAudit = &workload.MigrationAuditPolicy{MaxInFlight: 1, MaxPerWindow: 2, Window: time.Hour}
			},
		},
		{
			name: "superseded RetryBlock history",
			tweak: func(in *workload.ReconcileInput, _ *workload.ComponentPlan) {
				in.ObservedState.RetryBlocks = []workload.RetryBlock{
					{TargetRevision: "llama-70b-engine-stale001", State: workload.RetryBlockHeld},
					{TargetRevision: "llama-70b-engine-stale002", State: workload.RetryBlockBackoff},
				}
			},
		},
	}
}

// gangSurgeTargetMarkerAt reports the live replacement marker as the row
// shows it: Creating at the GangSurgeTarget step, pinned to revision.
func gangSurgeTargetMarkerAt(row *workload.InstanceStatus, revision string) bool {
	return row != nil && row.Phase == workload.InstancePhaseCreating &&
		row.Operation != nil && row.Operation.Type == workload.InstanceOperationUpdate &&
		row.Operation.Step == workload.UpdateStepGangSurgeTarget &&
		row.Operation.TargetRevision == revision
}

// releaseTerminatingPod drops the finalizer terminatingPod pinned, so the
// fake client collects the object the way the apiserver does once the
// kubelet has acknowledged the deletion.
func releaseTerminatingPod(t *testing.T, c client.Client, pod *corev1.Pod) {
	t.Helper()
	ctx := context.Background()
	live := &corev1.Pod{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(pod), live); err != nil {
		t.Fatalf("re-read terminating pod %s: %v", pod.Name, err)
	}
	live.Finalizers = nil
	if err := c.Update(ctx, live); err != nil {
		t.Fatalf("release pod %s: %v", pod.Name, err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(pod), &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("pod %s still exists after its finalizer was released: %v", pod.Name, err)
	}
}

// gangPodNames names the leader and worker pods of a gang Instance the
// gang surge fixtures seed, as the drain gate is asked about them.
func gangPodNames(isvc *v1beta1.InferenceService, instanceIdx int32) []string {
	return []string{
		query.PodName(isvc.Name, workload.ComponentEngine, instanceIdx, "leader", 0),
		query.PodName(isvc.Name, workload.ComponentEngine, instanceIdx, "worker", 0),
	}
}

// sameNames reports whether two name lists hold the same names in any order.
func sameNames(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, name := range got {
		seen[name]++
	}
	for _, name := range want {
		if seen[name] == 0 {
			return false
		}
		seen[name]--
	}
	return true
}
