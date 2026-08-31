package workload_test

// Escalation-pass guards: the pass decides the Failed transition from
// snapshot evidence, and it must never stamp Failed over an Instance
// whose blamed pod set is actually serving (the stale-bookkeeping /
// wedged-Operation shape), while still firing identically for a
// genuinely wedged pod set.

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
)

// servingPod builds a pod that is ContainersReady AND carries the
// ome.io/serving readiness gate — a pod in the load-balancer rotation.
func servingPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Hour)),
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue},
				{Type: "ome.io/serving", Status: corev1.ConditionTrue},
			},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "main",
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
}

// escalationFixture wires a ReconcileInput whose MutateInstance applies
// the callback to the local store and whose WarnInstanceFailed /
// MutateRetryBlock record their calls.
type escalationRecorder struct {
	store  []workload.InstanceStatus
	warns  []string
	blocks []workload.RetryBlock
}

func escalationFixture(insts []workload.InstanceStatus) (workload.ReconcileInput, *escalationRecorder) {
	rec := &escalationRecorder{store: append([]workload.InstanceStatus(nil), insts...)}
	input := workload.ReconcileInput{
		MutateInstance: func(_ context.Context, idx int32, mutate func(*workload.InstanceStatus) bool) error {
			for i := range rec.store {
				if rec.store[i].Index == idx {
					mutate(&rec.store[i])
					return nil
				}
			}
			return nil
		},
		MutateRetryBlock: func(_ context.Context, rev string, mutate func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
			b := workload.RetryBlock{TargetRevision: rev}
			mutate(&b)
			rec.blocks = append(rec.blocks, b)
			return nil
		},
		WarnInstanceFailed: func(_ int32, _, reason string) {
			rec.warns = append(rec.warns, reason)
		},
	}
	input.ObservedState.InstanceStatuses = append([]workload.InstanceStatus(nil), insts...)
	return input, rec
}

// TestEscalation_DeadlineElapsedButServing_NoFailedStamp pins the
// Failed-while-serving guard: an Instance whose Operation.Deadline has
// elapsed but whose pods are all present, Ready AND serving must NOT be
// stamped Failed (no mutation, no WarnInstanceFailed, no RetryBlock).
// Stamping Failed over a serving workload is a status lie — the pods
// are fine, only the Operation bookkeeping is stale.
func TestEscalation_DeadlineElapsedButServing_NoFailedStamp(t *testing.T) {
	now := time.Now()
	insts := []workload.InstanceStatus{{
		Index:    0,
		Phase:    workload.InstancePhaseUpdating,
		PodCount: 1,
		Operation: &workload.InstanceOperation{
			Type:           workload.InstanceOperationUpdate,
			TargetRevision: "own-engine-newhash",
			Deadline:       metav1.NewTime(now.Add(-time.Hour)),
		},
	}}
	input, rec := escalationFixture(insts)

	if err := runEscalationPass(t, workload.Deps{}, input, singleInstancePlan(0, 1),
		map[int32][]*corev1.Pod{0: {servingPod("engine-0-default-0")}}); err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	if rec.store[0].Phase != workload.InstancePhaseUpdating {
		t.Errorf("Phase: got %q want Updating (serving Instance must not be stamped Failed)", rec.store[0].Phase)
	}
	if rec.store[0].Operation == nil {
		t.Errorf("Operation: got nil want untouched (guard skips the Instance entirely)")
	}
	if len(rec.warns) != 0 {
		t.Errorf("WarnInstanceFailed: got %v want none", rec.warns)
	}
	if len(rec.blocks) != 0 {
		t.Errorf("RetryBlock: got %v want none", rec.blocks)
	}
}

// TestEscalation_DeadlineElapsedBelowDesiredServing_StillEscalates pins
// the guard's bound: pods serving but FEWER than desired do not prove
// health — the deadline expiry still fires (a partially-serving gang
// that never converged is exactly what InstanceReadyTimeout bounds).
func TestEscalation_DeadlineElapsedBelowDesiredServing_StillEscalates(t *testing.T) {
	now := time.Now()
	insts := []workload.InstanceStatus{{
		Index:    0,
		Phase:    workload.InstancePhaseCreating,
		PodCount: 2,
		Operation: &workload.InstanceOperation{
			Type:     workload.InstanceOperationCreate,
			Deadline: metav1.NewTime(now.Add(-time.Hour)),
		},
	}}
	input, rec := escalationFixture(insts)

	if err := runEscalationPass(t, workload.Deps{}, input, singleInstancePlan(0, 2),
		map[int32][]*corev1.Pod{0: {servingPod("engine-0-leader-0")}}); err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	if rec.store[0].Phase != workload.InstancePhaseFailed {
		t.Errorf("Phase: got %q want Failed (1/2 serving is not converged)", rec.store[0].Phase)
	}
	if len(rec.warns) != 1 {
		t.Errorf("WarnInstanceFailed: got %v want one call", rec.warns)
	}
}

// TestEscalation_StuckPodNotServing_IdenticalDispositionOutcome pins the
// evidence-driven fast path: a genuinely stuck pod (ImagePullBackOff
// past grace, not serving) on a single-pod Update attempt still lands
// the full disposition outcome — RetryBlock for the attempt's target
// revision, Operation cleared, Phase=Failed, one operator warning.
func TestEscalation_StuckPodNotServing_IdenticalDispositionOutcome(t *testing.T) {
	now := time.Now()
	insts := []workload.InstanceStatus{{
		Index:    0,
		Phase:    workload.InstancePhaseUpdating,
		PodCount: 1,
		Operation: &workload.InstanceOperation{
			Type:           workload.InstanceOperationUpdate,
			TargetRevision: "own-engine-badhash",
			Deadline:       metav1.NewTime(now.Add(30 * time.Minute)),
		},
	}}
	input, rec := escalationFixture(insts)
	input.StuckPodGrace = time.Second

	stuck := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-0-default-1",
			CreationTimestamp: metav1.NewTime(now.Add(-time.Minute)),
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "main",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
			}},
		},
	}

	if err := runEscalationPass(t, workload.Deps{}, input, singleInstancePlan(0, 1),
		map[int32][]*corev1.Pod{0: {stuck}}); err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	if rec.store[0].Phase != workload.InstancePhaseFailed {
		t.Errorf("Phase: got %q want Failed", rec.store[0].Phase)
	}
	if rec.store[0].Operation != nil {
		t.Errorf("Operation: got %+v want nil (disposition clears the failed attempt)", rec.store[0].Operation)
	}
	if len(rec.blocks) != 1 || rec.blocks[0].TargetRevision != "own-engine-badhash" {
		t.Errorf("RetryBlock: got %+v want one block for own-engine-badhash", rec.blocks)
	}
	if len(rec.warns) != 1 {
		t.Errorf("WarnInstanceFailed: got %v want one call", rec.warns)
	}
}

// TestEscalation_MultiInstanceStamps_OneBatchedWrite pins the batched
// flush: when the adapter wires ApplyInstanceMutations, a
// multi-instance escalation of plain Failed stamps lands in ONE batched
// write — zero per-instance MutateInstance calls — with every warning
// fired after the write, one per instance.
func TestEscalation_MultiInstanceStamps_OneBatchedWrite(t *testing.T) {
	now := time.Now()
	var insts []workload.InstanceStatus
	for idx := int32(0); idx < 3; idx++ {
		insts = append(insts, workload.InstanceStatus{
			Index:    idx,
			Phase:    workload.InstancePhaseRestarting,
			PodCount: 1,
			Operation: &workload.InstanceOperation{
				Type:     workload.InstanceOperationRestart,
				Deadline: metav1.NewTime(now.Add(-time.Hour)),
			},
		})
	}
	store := append([]workload.InstanceStatus(nil), insts...)
	batchCalls, mutateCalls := 0, 0
	var warns []int32
	warnsBeforeWrite := -1
	input := workload.ReconcileInput{
		ApplyInstanceMutations: func(_ context.Context, muts []workload.InstanceMutation) error {
			batchCalls++
			warnsBeforeWrite = len(warns)
			for _, m := range muts {
				for i := range store {
					if store[i].Index == m.Index {
						m.Mutate(&store[i])
					}
				}
			}
			return nil
		},
		MutateInstance: func(_ context.Context, _ int32, _ func(*workload.InstanceStatus) bool) error {
			mutateCalls++
			return nil
		},
		WarnInstanceFailed: func(idx int32, _, _ string) {
			warns = append(warns, idx)
		},
	}
	input.ObservedState.InstanceStatuses = append([]workload.InstanceStatus(nil), insts...)

	if err := runEscalationPass(t, workload.Deps{}, input, workload.ComponentPlan{}, nil); err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	if batchCalls != 1 {
		t.Errorf("ApplyInstanceMutations calls: got %d want 1 (k stamps must coalesce into one batched write)", batchCalls)
	}
	if mutateCalls != 0 {
		t.Errorf("MutateInstance calls: got %d want 0 (plain stamps must route through the batch)", mutateCalls)
	}
	for i := range store {
		if store[i].Phase != workload.InstancePhaseFailed {
			t.Errorf("instance %d Phase: got %q want Failed", store[i].Index, store[i].Phase)
		}
		if store[i].Operation == nil {
			t.Errorf("instance %d Operation: got nil want preserved (deadline stamp keeps the Operation)", store[i].Index)
		}
	}
	if len(warns) != 3 {
		t.Errorf("WarnInstanceFailed: got %v want one call per instance", warns)
	}
	if warnsBeforeWrite != 0 {
		t.Errorf("warnings fired before the batched write: got %d want 0 (warn follows its write)", warnsBeforeWrite)
	}
}

// TestEscalation_BatchedWriteFails_NoWarnings pins the flush error
// contract: a failed batched write emits no warnings (same as the
// immediate path — the stamp's warning follows its write) and surfaces
// the error.
func TestEscalation_BatchedWriteFails_NoWarnings(t *testing.T) {
	now := time.Now()
	insts := []workload.InstanceStatus{{
		Index:    0,
		Phase:    workload.InstancePhaseRestarting,
		PodCount: 1,
		Operation: &workload.InstanceOperation{
			Type:     workload.InstanceOperationRestart,
			Deadline: metav1.NewTime(now.Add(-time.Hour)),
		},
	}}
	warned := 0
	input := workload.ReconcileInput{
		ApplyInstanceMutations: func(_ context.Context, _ []workload.InstanceMutation) error {
			return context.DeadlineExceeded
		},
		WarnInstanceFailed: func(_ int32, _, _ string) { warned++ },
	}
	input.ObservedState.InstanceStatuses = insts

	if err := runEscalationPass(t, workload.Deps{}, input, workload.ComponentPlan{}, nil); err == nil {
		t.Fatalf("escalation pass: got nil error, want the flush failure surfaced")
	}
	if warned != 0 {
		t.Errorf("WarnInstanceFailed: got %d calls want 0 (no warning without a landed write)", warned)
	}
}

// TestEscalation_DispositionFlushesPendingStamps pins the write-ahead
// preservation: when a buffered plain stamp is followed by a
// disposition-routed instance, the buffer flushes BEFORE the
// disposition's own writes (RetryBlock upsert, then the immediate
// op-clear MutateInstance) so the overall write order matches the
// unbatched pass.
func TestEscalation_DispositionFlushesPendingStamps(t *testing.T) {
	now := time.Now()
	insts := []workload.InstanceStatus{
		{
			// Buffered: Restart deadline expiry takes the plain stamp.
			Index:    0,
			Phase:    workload.InstancePhaseRestarting,
			PodCount: 1,
			Operation: &workload.InstanceOperation{
				Type:     workload.InstanceOperationRestart,
				Deadline: metav1.NewTime(now.Add(-time.Hour)),
			},
		},
		{
			// Disposition: single-pod Update attempt with a stuck pod.
			Index:    1,
			Phase:    workload.InstancePhaseUpdating,
			PodCount: 1,
			Operation: &workload.InstanceOperation{
				Type:           workload.InstanceOperationUpdate,
				TargetRevision: "own-engine-badhash",
				Deadline:       metav1.NewTime(now.Add(30 * time.Minute)),
			},
		},
	}
	store := append([]workload.InstanceStatus(nil), insts...)
	var sequence []string
	input := workload.ReconcileInput{
		ApplyInstanceMutations: func(_ context.Context, muts []workload.InstanceMutation) error {
			sequence = append(sequence, "batch")
			for _, m := range muts {
				for i := range store {
					if store[i].Index == m.Index {
						m.Mutate(&store[i])
					}
				}
			}
			return nil
		},
		MutateInstance: func(_ context.Context, idx int32, mutate func(*workload.InstanceStatus) bool) error {
			sequence = append(sequence, "mutate")
			for i := range store {
				if store[i].Index == idx {
					mutate(&store[i])
				}
			}
			return nil
		},
		MutateRetryBlock: func(_ context.Context, _ string, mutate func(*workload.RetryBlock) workload.RetryBlockDisposition) error {
			sequence = append(sequence, "retryblock")
			b := workload.RetryBlock{}
			mutate(&b)
			return nil
		},
		WarnInstanceFailed: func(_ int32, _, _ string) {},
	}
	input.ObservedState.InstanceStatuses = append([]workload.InstanceStatus(nil), insts...)
	input.StuckPodGrace = time.Second

	stuck := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-1-default-0",
			CreationTimestamp: metav1.NewTime(now.Add(-time.Minute)),
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "main",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}},
			}},
		},
	}

	if err := runEscalationPass(t, workload.Deps{}, input,
		workload.ComponentPlan{Instances: []workload.InstancePlan{
			{Index: 0, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		}},
		map[int32][]*corev1.Pod{1: {stuck}}); err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	want := []string{"batch", "retryblock", "mutate"}
	if len(sequence) != len(want) {
		t.Fatalf("write sequence: got %v want %v", sequence, want)
	}
	for i := range want {
		if sequence[i] != want[i] {
			t.Fatalf("write sequence: got %v want %v (pending stamps must flush before the disposition's write-ahead-ordered writes)", sequence, want)
		}
	}
	if store[0].Phase != workload.InstancePhaseFailed {
		t.Errorf("instance 0 Phase: got %q want Failed", store[0].Phase)
	}
	if store[1].Phase != workload.InstancePhaseFailed || store[1].Operation != nil {
		t.Errorf("instance 1: got Phase=%q Operation=%v want Failed with Operation cleared (disposition path)", store[1].Phase, store[1].Operation)
	}
}

// TestEscalation_MigratePair_StuckSurgePod_NotStamped pins the
// migration-authority contract on the FAST path: a migration surge pod
// stuck in a terminal waiting state past the stuck-pod grace must NOT
// stamp either pair side Failed. podsForStuckCheck blames a Migrate
// pair through own-plus-sibling pods, so without the exclusion the
// stuck surge pod fails the healthy still-serving source — and a
// Failed source drops out of migrationSourceIndices, letting the plan
// allocate a phantom index. The migration record's expiry pass owns
// migration failure.
func TestEscalation_MigratePair_StuckSurgePod_NotStamped(t *testing.T) {
	now := time.Now()
	surgeIdx := int32(1)
	sourceIdx := int32(0)
	future := metav1.NewTime(now.Add(30 * time.Minute))
	insts := []workload.InstanceStatus{
		{
			Index:    0,
			Phase:    workload.InstancePhaseMigrating,
			PodCount: 1,
			Operation: &workload.InstanceOperation{
				Type:        workload.InstanceOperationMigrate,
				Step:        "CreateSurge",
				RequestUUID: "mig-1",
				SurgeIndex:  &surgeIdx,
				Deadline:    future,
			},
		},
		{
			Index: 1,
			Phase: workload.InstancePhaseCreating,
			Operation: &workload.InstanceOperation{
				Type:        workload.InstanceOperationMigrate,
				Step:        "CreateSurge",
				RequestUUID: "mig-1",
				SurgeIndex:  &sourceIdx,
				Deadline:    future,
			},
		},
	}
	input, rec := escalationFixture(insts)
	input.StuckPodGrace = time.Second

	stuck := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-1-default-0",
			CreationTimestamp: metav1.NewTime(now.Add(-time.Minute)),
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "main",
				State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}},
		},
	}

	if err := runEscalationPass(t, workload.Deps{}, input,
		workload.ComponentPlan{Instances: []workload.InstancePlan{
			{Index: 0, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
			{Index: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		}},
		map[int32][]*corev1.Pod{
			0: {servingPod("engine-0-default-0")},
			1: {stuck},
		}); err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	if got := rec.store[0]; got.Phase != workload.InstancePhaseMigrating || got.Operation == nil {
		t.Errorf("migration source must be untouched by the stuck-pod fast path; got phase=%q op=%+v", got.Phase, got.Operation)
	}
	if got := rec.store[1]; got.Phase != workload.InstancePhaseCreating || got.Operation == nil {
		t.Errorf("migration surge must be untouched by the stuck-pod fast path; got phase=%q op=%+v", got.Phase, got.Operation)
	}
	if len(rec.warns) != 0 {
		t.Errorf("WarnInstanceFailed: got %v want none (Migrate pairs are record-owned)", rec.warns)
	}
}

// TestEscalation_GangSurgeSource_GatedAttemptPods_DeadlineSkipped pins
// the gate exemption across the surge pair: a gang-surge SOURCE's
// attempt pods live in its Operation.SurgeIndex bucket, so while they
// queue for admission the source's elapsed deadline must not expire —
// failing it would tear down the queued gang (losing queue position)
// and RetryBlock a healthy target revision.
func TestEscalation_GangSurgeSource_GatedAttemptPods_DeadlineSkipped(t *testing.T) {
	now := time.Now()
	surgeIdx := int32(2)
	insts := []workload.InstanceStatus{{
		Index:    0,
		Phase:    workload.InstancePhaseUpdating,
		PodCount: 1,
		Operation: &workload.InstanceOperation{
			Type:           workload.InstanceOperationUpdate,
			Step:           "Surge",
			TargetRevision: "own-engine-newhash",
			SurgeIndex:     &surgeIdx,
			Deadline:       metav1.NewTime(now.Add(-time.Hour)),
		},
	}}
	input, rec := escalationFixture(insts)

	gated := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-2-default-0",
			CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
		},
		Spec: corev1.PodSpec{
			SchedulingGates: []corev1.PodSchedulingGate{{Name: "kueue.x-k8s.io/admission"}},
		},
	}

	if err := runEscalationPass(t, workload.Deps{}, input, singleInstancePlan(0, 1),
		map[int32][]*corev1.Pod{
			0: {servingPod("engine-0-default-0")},
			2: {gated},
		}); err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	if rec.store[0].Phase != workload.InstancePhaseUpdating {
		t.Errorf("Phase: got %q want Updating (queued attempt pods must park the source's deadline, not expire it)", rec.store[0].Phase)
	}
	if len(rec.warns) != 0 {
		t.Errorf("WarnInstanceFailed: got %v want none", rec.warns)
	}
	if len(rec.blocks) != 0 {
		t.Errorf("RetryBlock: got %v want none", rec.blocks)
	}
}

// TestEscalation_AdmissionGatedInstance_DeadlineSkipped pins the gated
// skip: an Instance whose pod is still held by an admission scheduling
// gate is queued, not stuck — an elapsed deadline observed before the
// parking step zeroes it must not expire the Instance.
func TestEscalation_AdmissionGatedInstance_DeadlineSkipped(t *testing.T) {
	now := time.Now()
	insts := []workload.InstanceStatus{{
		Index:    0,
		Phase:    workload.InstancePhaseCreating,
		PodCount: 1,
		Operation: &workload.InstanceOperation{
			Type:     workload.InstanceOperationCreate,
			Deadline: metav1.NewTime(now.Add(-time.Hour)),
		},
	}}
	input, rec := escalationFixture(insts)

	gated := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "engine-0-default-0",
			CreationTimestamp: metav1.NewTime(now.Add(-time.Hour)),
		},
		Spec: corev1.PodSpec{
			SchedulingGates: []corev1.PodSchedulingGate{{Name: "kueue.x-k8s.io/admission"}},
		},
	}

	if err := runEscalationPass(t, workload.Deps{}, input, singleInstancePlan(0, 1),
		map[int32][]*corev1.Pod{0: {gated}}); err != nil {
		t.Fatalf("escalation pass: %v", err)
	}

	if rec.store[0].Phase != workload.InstancePhaseCreating {
		t.Errorf("Phase: got %q want Creating (gated Instance is queued, not stuck)", rec.store[0].Phase)
	}
	if len(rec.warns) != 0 {
		t.Errorf("WarnInstanceFailed: got %v want none", rec.warns)
	}
}
