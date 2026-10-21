package escalation_test

// Supersede-prune coverage: the end-of-pass GC must remove exactly the
// RetryBlocks whose revision nothing targets anymore — keeping every
// block still reachable through CurrentRevision, UpdateRevision, the
// roll target, or a live per-Instance Operation — and must not run at
// all under Paused or Teardown.

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocktesting "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/escalation"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// recordRetryBlockRemovals wires a recording MutateRetryBlock that
// captures the revision of every Remove disposition.
func recordRetryBlockRemovals(input *types.ReconcileInput) *[]string {
	removed := &[]string{}
	input.MutateRetryBlock = func(_ context.Context, rev string, mutate func(*types.RetryBlock) types.RetryBlockDisposition) error {
		b := types.RetryBlock{TargetRevision: rev}
		if mutate(&b) == types.RetryBlockRemove {
			*removed = append(*removed, rev)
		}
		return nil
	}
	return removed
}

// TestPruneSupersededRetryBlocks_KeepSetMatrix pins the keep set: one
// block per keep-set member (CurrentRevision, UpdateRevision, roll
// target, a live Operation.TargetRevision) is retained while a block
// matching none of them is removed.
func TestPruneSupersededRetryBlocks_KeepSetMatrix(t *testing.T) {
	input := minimalInput(t)
	removed := recordRetryBlockRemovals(&input)
	input.ObservedState.CurrentRevision = "own-engine-current1"
	input.ObservedState.UpdateRevision = "own-engine-update01"
	input.ObservedState.InstanceStatuses = []types.InstanceStatus{
		{Index: 0, Phase: types.InstancePhaseUpdating, Operation: &types.InstanceOperation{
			Type:           types.InstanceOperationUpdate,
			TargetRevision: "own-engine-optarget",
		}},
	}
	input.ObservedState.RetryBlocks = []types.RetryBlock{
		{TargetRevision: "own-engine-current1", State: types.RetryBlockHeld},
		{TargetRevision: "own-engine-update01", State: types.RetryBlockHeld},
		{TargetRevision: "own-engine-rolltgt1", State: types.RetryBlockHeld},
		{TargetRevision: "own-engine-optarget", State: types.RetryBlockHeld},
		{TargetRevision: "own-engine-stale001", State: types.RetryBlockHeld},
	}
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: "own-engine-rolltgt1"}}

	if err := escalation.PruneSupersededRetryBlocks(context.Background(), input, target); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(*removed) != 1 || (*removed)[0] != "own-engine-stale001" {
		t.Fatalf("removed: got %v want [own-engine-stale001]", *removed)
	}
}

// TestPruneSupersededRetryBlocks_MultiStaleBatch removes every stale
// block in one pass, whatever its state — Backoff and Held orphans are
// the same class of dead entry once their revision is superseded.
func TestPruneSupersededRetryBlocks_MultiStaleBatch(t *testing.T) {
	input := minimalInput(t)
	removed := recordRetryBlockRemovals(&input)
	input.ObservedState.CurrentRevision = "own-engine-current1"
	input.ObservedState.RetryBlocks = []types.RetryBlock{
		{TargetRevision: "own-engine-stale001", State: types.RetryBlockHeld},
		{TargetRevision: "own-engine-current1", State: types.RetryBlockBackoff},
		{TargetRevision: "own-engine-stale002", State: types.RetryBlockBackoff},
		{TargetRevision: "own-engine-stale003", State: types.RetryBlockRetryInProgress},
	}

	if err := escalation.PruneSupersededRetryBlocks(context.Background(), input, nil); err != nil {
		t.Fatalf("prune: %v", err)
	}
	want := []string{"own-engine-stale001", "own-engine-stale002", "own-engine-stale003"}
	if len(*removed) != len(want) {
		t.Fatalf("removed: got %v want %v", *removed, want)
	}
	for i, rev := range want {
		if (*removed)[i] != rev {
			t.Fatalf("removed: got %v want %v", *removed, want)
		}
	}
}

// TestPruneRetryBlocks_HeldBlockOfTheCurrentRevisionSurvivesACrashLoop
// pins that neither end-of-pass prune removes a Held block whose revision
// the status now names as current while its rows remember a crash of
// their promoted set: the supersede-prune keeps every revision still in
// play and the outlived-prune never touches a Held ladder, which only a
// corrective revision or an operator release ends.
func TestPruneRetryBlocks_HeldBlockOfTheCurrentRevisionSurvivesACrashLoop(t *testing.T) {
	const rev = "own-engine-current1"
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	readySince := metav1.NewTime(now.Add(-10 * time.Minute))
	crashed := metav1.NewTime(now.Add(-time.Minute))
	input := minimalInput(t)
	input.Clock = clocktesting.NewFakeClock(now)
	removed := recordRetryBlockRemovals(&input)
	input.ObservedState.CurrentRevision = rev
	input.ObservedState.UpdateRevision = rev
	input.ObservedState.InstanceStatuses = []types.InstanceStatus{
		{Index: 0, Incarnation: 1, Phase: types.InstancePhaseReady, RunningRevision: rev, PodCount: 1, ServingPodCount: 1, ReadySince: &readySince,
			LastFailure: &types.InstanceTermination{PodName: "llama-70b-engine-0-default-0", Reason: "CrashLoopBackOff", Time: crashed}},
	}
	input.ObservedState.RetryBlocks = []types.RetryBlock{{TargetRevision: rev, State: types.RetryBlockHeld, AttemptsStarted: 3}}
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: rev}}
	pods := func(context.Context) (map[int32][]*corev1.Pod, error) {
		return map[int32][]*corev1.Pod{0: {enginePod("llama-70b", "prod", 0)}}, nil
	}

	if err := escalation.PruneSupersededRetryBlocks(context.Background(), input, target); err != nil {
		t.Fatalf("supersede prune: %v", err)
	}
	if err := escalation.PruneOutlivedRetryBlocks(context.Background(), input, minimalPlan(), target, pods); err != nil {
		t.Fatalf("outlived prune: %v", err)
	}
	if len(*removed) != 0 {
		t.Fatalf("the Held block of the current revision must survive while its rows remember a crash, removed %v", *removed)
	}
}

// TestPruneSupersededRetryBlocks_NoSeamNoOp pins the unwired-adapter
// guard: a nil MutateRetryBlock is a clean no-op even with stale blocks
// present.
func TestPruneSupersededRetryBlocks_NoSeamNoOp(t *testing.T) {
	input := minimalInput(t)
	input.MutateRetryBlock = nil
	input.ObservedState.RetryBlocks = []types.RetryBlock{
		{TargetRevision: "own-engine-stale001", State: types.RetryBlockHeld},
	}
	if err := escalation.PruneSupersededRetryBlocks(context.Background(), input, nil); err != nil {
		t.Fatalf("prune with nil seam: %v", err)
	}
}

// TestReconcile_PrunesSupersededRetryBlock pins the dispatcher wiring:
// a normal (non-paused, non-teardown) reconcile removes a stale block
// and keeps the CurrentRevision one.
func TestReconcile_PrunesSupersededRetryBlock(t *testing.T) {
	scheme := makeScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	deps := types.Deps{Client: c}

	in := minimalInput(t)
	removed := recordRetryBlockRemovals(&in)
	in.ObservedState.CurrentRevision = "own-engine-current1"
	in.ObservedState.RetryBlocks = []types.RetryBlock{
		{TargetRevision: "own-engine-current1", State: types.RetryBlockHeld},
		{TargetRevision: "own-engine-stale001", State: types.RetryBlockHeld},
	}
	plan := minimalPlan()

	if _, err := workload.Reconcile(context.Background(), deps, in, plan, nil); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(*removed) != 1 || (*removed)[0] != "own-engine-stale001" {
		t.Fatalf("removed: got %v want [own-engine-stale001]", *removed)
	}
}

// TestReconcile_Paused_NoSupersededPrune: Paused suspends the lifecycle
// machinery, the prune included — a paused pass must leave every block
// untouched.
func TestReconcile_Paused_NoSupersededPrune(t *testing.T) {
	scheme := makeScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	deps := types.Deps{Client: c}

	in := minimalInput(t)
	removed := recordRetryBlockRemovals(&in)
	in.ObservedState.CurrentRevision = "own-engine-current1"
	in.ObservedState.RetryBlocks = []types.RetryBlock{
		{TargetRevision: "own-engine-stale001", State: types.RetryBlockHeld},
	}
	plan := minimalPlan()
	plan.Paused = true

	if _, err := workload.Reconcile(context.Background(), deps, in, plan, nil); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(*removed) != 0 {
		t.Fatalf("paused reconcile pruned %v, want none", *removed)
	}
}

// TestReconcile_Teardown_NoSupersededPrune: teardown runs only the
// Delete pipeline — no RetryBlock bookkeeping of any kind.
func TestReconcile_Teardown_NoSupersededPrune(t *testing.T) {
	scheme := makeScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	deps := types.Deps{Client: c}

	in := minimalInput(t)
	removed := recordRetryBlockRemovals(&in)
	in.Teardown = true
	in.ObservedState.RetryBlocks = []types.RetryBlock{
		{TargetRevision: "own-engine-stale001", State: types.RetryBlockHeld},
	}

	if _, err := workload.Reconcile(context.Background(), deps, in, minimalPlan(), nil); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(*removed) != 0 {
		t.Fatalf("teardown reconcile pruned %v, want none", *removed)
	}
}

// TestPruneOutlivedRetryBlocks_HealedInPlaceSetIsOutlivedAtTheWindow pins
// the prune of a block an idle crash loop recorded against the running
// revision once the kubelet brings the same pod back serving, under a
// restart policy that rebuilds nothing. The row's ReadySince stays where
// it was and the pod's status carries its restart, so the set is read as
// a comeback: unproven until it has held Ready for the proven window,
// the stuck-pod grace here, since it re-entered Ready. The block is kept
// inside that window and pruned once the window has run.
func TestPruneOutlivedRetryBlocks_HealedInPlaceSetIsOutlivedAtTheWindow(t *testing.T) {
	const rev = "own-engine-current1"
	readyAgain := time.Date(2026, 1, 1, 0, 10, 0, 0, time.UTC)
	readySince := metav1.NewTime(readyAgain.Add(-10 * time.Minute))
	crashed := metav1.NewTime(readyAgain.Add(-20 * time.Second))
	due := metav1.NewTime(readyAgain.Add(40 * time.Second))
	healedPod := func() *corev1.Pod {
		pod := enginePod("llama-70b", "prod", 0)
		pod.CreationTimestamp = metav1.NewTime(readySince.Add(-time.Minute))
		pod.Labels[query.LabelRevisionHash] = query.RevisionHashFromControllerRevisionName(rev)
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{
			{Type: corev1.ContainersReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(readyAgain)},
			{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(readyAgain)},
			{Type: query.ServingConditionType, Status: corev1.ConditionTrue, LastTransitionTime: readySince},
		}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name:         constants.MainContainerName,
			Image:        "test:v1",
			Ready:        true,
			RestartCount: 2,
			LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode: 1, Reason: "Error",
				StartedAt:  metav1.NewTime(crashed.Add(-5 * time.Second)),
				FinishedAt: crashed,
			}},
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(readyAgain.Add(-15 * time.Second))}},
		}}
		return pod
	}
	for _, tc := range []struct {
		name   string
		at     time.Duration
		grace  time.Duration
		pruned bool
	}{
		{name: "inside the window the block is kept", at: 45 * time.Second, grace: time.Minute},
		{name: "at the window the block is pruned", at: 60 * time.Second, grace: time.Minute, pruned: true},
		// Neither readiness knob is configured: no span proves the set, so
		// the block is kept however long the set serves, and the next
		// death counts on it.
		{name: "with no window configured the block is kept", at: time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := minimalInput(t)
			input.Clock = clocktesting.NewFakeClock(readyAgain.Add(tc.at))
			input.StuckPodGrace = tc.grace
			removed := recordRetryBlockRemovals(&input)
			input.ObservedState.CurrentRevision = rev
			input.ObservedState.UpdateRevision = rev
			input.ObservedState.InstanceStatuses = []types.InstanceStatus{
				{Index: 0, Incarnation: 1, Phase: types.InstancePhaseReady, RunningRevision: rev, PodCount: 1, ServingPodCount: 1, ReadySince: &readySince,
					LastFailure: &types.InstanceTermination{PodName: "llama-70b-engine-0-default-0", ContainerName: constants.MainContainerName, Reason: "CrashLoopBackOff", Time: crashed}},
			}
			input.ObservedState.RetryBlocks = []types.RetryBlock{{TargetRevision: rev, State: types.RetryBlockBackoff, AttemptsStarted: 1, NextRetryAt: &due, Reason: "CrashLoopBackOff"}}
			target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: rev, CreationTimestamp: metav1.NewTime(readySince.Add(-time.Hour))}}
			pods := func(context.Context) (map[int32][]*corev1.Pod, error) {
				return map[int32][]*corev1.Pod{0: {healedPod()}}, nil
			}

			if err := escalation.PruneOutlivedRetryBlocks(context.Background(), input, minimalPlan(), target, pods); err != nil {
				t.Fatalf("outlived prune: %v", err)
			}
			if got := len(*removed) == 1 && (*removed)[0] == rev; got != tc.pruned {
				t.Fatalf("pruned=%v want %v (removed %v)", got, tc.pruned, *removed)
			}
		})
	}
}

// A gang surge onto the roll target failed to start, its abandon recorded
// the attempt as a Backoff block and reset the source Ready on the revision
// it still runs. A sibling that has served the target all along does not
// outlive that failure on the source's behalf: the block stays while the
// source still has to roll onto the target, and the rows are read the old
// way once the target moves on.
func TestPruneOutlivedRetryBlocks_RollTargetKeptWhileARowStillHasToRollOntoIt(t *testing.T) {
	const (
		target  = "own-engine-current1"
		running = "own-engine-pushed01"
		newer   = "own-engine-newer001"
	)
	now := time.Date(2026, 1, 1, 0, 10, 0, 0, time.UTC)
	readySince := metav1.NewTime(now.Add(-time.Hour))
	due := metav1.NewTime(now.Add(2 * time.Minute))
	servingPod := func(idx int32, rev string) *corev1.Pod {
		pod := enginePod("llama-70b", "prod", idx)
		pod.CreationTimestamp = metav1.NewTime(readySince.Add(-time.Minute))
		pod.Labels[query.LabelRevisionHash] = query.RevisionHashFromControllerRevisionName(rev)
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{
			{Type: corev1.ContainersReady, Status: corev1.ConditionTrue, LastTransitionTime: readySince},
			{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: readySince},
			{Type: query.ServingConditionType, Status: corev1.ConditionTrue, LastTransitionTime: readySince},
		}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name: constants.MainContainerName, Image: "test:v1", Ready: true,
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: readySince}},
		}}
		return pod
	}
	plan := minimalPlan()
	plan.Replicas = 2
	plan.Instances = append(plan.Instances, types.InstancePlan{Index: 1, Incarnation: 2, Runners: []types.RunnerPlan{{Name: "default", Size: 1}}})
	for _, tc := range []struct {
		name       string
		rollTarget string
		pruned     bool
	}{
		{name: "the target's block stays while the second Instance is off it", rollTarget: target},
		{name: "a newer target leaves the old revision's rows to the old reading", rollTarget: newer, pruned: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := minimalInput(t)
			input.Clock = clocktesting.NewFakeClock(now)
			input.StuckPodGrace = time.Minute
			removed := recordRetryBlockRemovals(&input)
			input.ObservedState.CurrentRevision = target
			input.ObservedState.UpdateRevision = tc.rollTarget
			input.ObservedState.InstanceStatuses = []types.InstanceStatus{
				{Index: 0, Incarnation: 1, Phase: types.InstancePhaseReady, RunningRevision: target, PodCount: 1, ServingPodCount: 1, ReadySince: &readySince},
				{Index: 1, Incarnation: 2, Phase: types.InstancePhaseReady, RunningRevision: running, PodCount: 1, ReadySince: &readySince,
					LastFailure: &types.InstanceTermination{PodName: "llama-70b-engine-2-default-0", ContainerName: constants.MainContainerName, Reason: "CreateContainerConfigError", Time: metav1.NewTime(now.Add(-time.Minute))}},
			}
			input.ObservedState.RetryBlocks = []types.RetryBlock{{TargetRevision: target, State: types.RetryBlockBackoff, AttemptsStarted: 1, NextRetryAt: &due, Reason: "CreateContainerConfigError"}}
			rollTarget := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: tc.rollTarget, CreationTimestamp: metav1.NewTime(readySince.Add(-time.Hour))}}
			pods := func(context.Context) (map[int32][]*corev1.Pod, error) {
				return map[int32][]*corev1.Pod{0: {servingPod(0, target)}, 1: {servingPod(1, running)}}, nil
			}

			if err := escalation.PruneOutlivedRetryBlocks(context.Background(), input, plan, rollTarget, pods); err != nil {
				t.Fatalf("outlived prune: %v", err)
			}
			if got := len(*removed) == 1 && (*removed)[0] == target; got != tc.pruned {
				t.Fatalf("pruned=%v want %v (removed %v)", got, tc.pruned, *removed)
			}
		})
	}
}
