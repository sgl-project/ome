package ops

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clocktesting "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// Tests for the minReadySeconds pacing gate: each strategy holds its
// budget-releasing step until the new pods have been Ready for the window.

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

// TestSurgeUpdate_HoldsSourceDrainForMinReadySeconds: a PodReady surge pod
// that has not yet been Ready for minReadySeconds keeps the source in
// rotation and the Operation at Step=Surge (budget slot held); once the
// window elapses — inclusive at the exact boundary — the same pass drains
// and deletes the source.
func TestSurgeUpdate_HoldsSourceDrainForMinReadySeconds(t *testing.T) {
	legacyResetExpectations(t)
	isvc, ir := surgeISVCReady("llama-70b", "prod", 1)
	oldPod := surgePodAtOrdinal(isvc, 0, 1, 0, true, true)
	surgePod := surgePodAtOrdinal(isvc, 0, 1, 1, true, true)
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{
		Name: "llama-70b-engine-rev-v2hash", Namespace: isvc.Namespace,
	}}
	surgePod.Labels[query.LabelRevisionHash] = query.RevisionHashFromControllerRevisionName(target.Name)
	podReadyAt(surgePod, minReadyWindowStart.Add(-10*time.Second))
	c := legacyNewFakeClient(t, isvc, ir, oldPod, surgePod, target)

	clk := clocktesting.NewFakeClock(minReadyWindowStart)
	input := legacyTestInput(isvc, c, workload.ComponentEngine)
	input.Clock = clk
	plan := surgePlan()
	plan.MinReadySeconds = 20
	if err := patchInstanceStatusSurgingForUpdate(context.Background(), input, 0, target.Name, plan.InstanceReadyTimeout); err != nil {
		t.Fatalf("pre-stamp: %v", err)
	}

	done, err := surgeUpdate(context.Background(), workload.Deps{Client: c}, input, plan, plan.Instances[0], target, []*corev1.Pod{oldPod, surgePod})
	if err != nil {
		t.Fatalf("surgeUpdate inside window: %v", err)
	}
	if done {
		t.Fatalf("expected done=false while the replacement is inside the minReadySeconds window")
	}
	freshOld := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(oldPod), freshOld); err != nil {
		t.Fatalf("source missing inside the window: %v", err)
	}
	if !podreadiness.IsServing(freshOld) {
		t.Fatalf("source left rotation before the replacement was Available")
	}
	status := legacyInstanceStatusesOnIR(c, isvc, workload.ComponentEngine)[0]
	if status.Operation == nil || status.Operation.Step != updateStepSurge {
		t.Fatalf("operation advanced past Surge inside the window: %+v", status.Operation)
	}

	// Exactly readyAt + 20s: the boundary counts as Available.
	clk.SetTime(minReadyWindowStart.Add(10 * time.Second))
	freshSurge := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(surgePod), freshSurge); err != nil {
		t.Fatalf("get replacement: %v", err)
	}
	done, err = surgeUpdate(context.Background(), workload.Deps{Client: c}, input, plan, plan.Instances[0], target, []*corev1.Pod{freshOld, freshSurge})
	if err != nil {
		t.Fatalf("surgeUpdate after window: %v", err)
	}
	if done {
		t.Fatalf("expected done=false while deleting source")
	}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(oldPod), &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("source was not deleted once the replacement became Available: %v", err)
	}
}

// TestGangSurgeUpdate_HoldsSourceDrainForMinReadySeconds is the gang
// counterpart: a PodReady replacement gang inside the window leaves every
// source pod serving; after the window the source gang is drained.
func TestGangSurgeUpdate_HoldsSourceDrainForMinReadySeconds(t *testing.T) {
	legacyResetExpectations(t)
	isvc, _ := surgeISVCReady("llama-70b", "prod", 1)
	plan := gangSurgePlan()
	plan.MinReadySeconds = 20

	v1Name := "llama-70b-engine-rev-v1hash"
	v2Name := "llama-70b-engine-rev-v2hash"
	v1Hash := query.RevisionHashFromControllerRevisionName(v1Name)
	v2Hash := query.RevisionHashFromControllerRevisionName(v2Name)
	ir := gangSurgeInFlightIR(isvc, v1Name, v2Name)
	c := legacyNewFakeClient(t, isvc, ir)
	makeCR(t, c, isvc, v2Name)

	for _, runner := range []string{"leader", "worker"} {
		if err := c.Create(context.Background(), gangPodAt(isvc, 0, runner, v1Hash, true, true)); err != nil {
			t.Fatalf("seed source gang pod (%s): %v", runner, err)
		}
	}
	for _, runner := range []string{"leader", "worker"} {
		p := gangPodAt(isvc, 1, runner, v2Hash, true, true)
		podReadyAt(p, minReadyWindowStart.Add(-10*time.Second))
		if err := c.Create(context.Background(), p); err != nil {
			t.Fatalf("seed surge gang pod (%s): %v", runner, err)
		}
	}

	clk := clocktesting.NewFakeClock(minReadyWindowStart)
	input := gangInputWithRemove(isvc, c)
	input.Clock = clk
	v2 := &appsv1.ControllerRevision{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "prod", Name: v2Name}, v2); err != nil {
		t.Fatalf("get v2 CR: %v", err)
	}
	// Hold the pass at the post-drain Satisfied() gate so drained source
	// pods stay observable instead of being deleted in the same pass.
	workload.DefaultExpectations.ExpectDeletes("prod", "llama-70b", workload.ComponentEngine, 0, 1)

	sourcePods := func() []*corev1.Pod {
		pods, err := query.LiveListPodsForInstance(context.Background(), c, "prod", "llama-70b", workload.ComponentEngine, 0)
		if err != nil {
			t.Fatalf("list source gang pods: %v", err)
		}
		if len(pods) != 2 {
			t.Fatalf("source gang pods: got %d want 2", len(pods))
		}
		return pods
	}

	if _, err := surgeUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], v2, nil); err != nil {
		t.Fatalf("gang surge pass inside window: %v", err)
	}
	for _, pod := range sourcePods() {
		if !podreadiness.IsServing(pod) {
			t.Fatalf("source gang pod %s left rotation before the replacement gang was Available", pod.Name)
		}
	}
	src := legacyInstanceStatusesOnIR(c, isvc, workload.ComponentEngine)[0]
	if src.Operation == nil || src.Operation.Step != updateStepSurge {
		t.Fatalf("source operation advanced past Surge inside the window: %+v", src.Operation)
	}

	clk.SetTime(minReadyWindowStart.Add(10 * time.Second))
	if _, err := surgeUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], v2, nil); err != nil {
		t.Fatalf("gang surge pass after window: %v", err)
	}
	for _, pod := range sourcePods() {
		if podreadiness.IsServing(pod) {
			t.Fatalf("source gang pod %s still serving after the replacement gang became Available", pod.Name)
		}
	}
}

// TestRecreateUpdate_PromotionWaitsForMinReadySeconds: with the old pods
// gone and the recreated pod serving and PodReady, promotion (which clears
// the Operation and releases the unavailability budget slot) waits until
// the pod has been Ready for the window.
func TestRecreateUpdate_PromotionWaitsForMinReadySeconds(t *testing.T) {
	legacyResetExpectations(t)
	isvc, ir := legacyISVCReadyAtIncarnation("llama-70b", "prod", 1)
	isvc.Spec.Engine.ComponentExtensionSpec.Lifecycle = &v1beta1.LifecycleSpec{
		UpdateStrategy: &v1beta1.UpdateStrategy{Type: v1beta1.UpdateStrategyRecreatePod},
	}
	newPod := legacyPodAtIncarnation(isvc, 0, 2, true, true)
	newPod.Spec.Containers = []corev1.Container{{Name: "main", Image: "llama:v2"}}
	podReadyAt(newPod, minReadyWindowStart.Add(-5*time.Second))
	target := legacyTargetSpecImage("llama:v2")
	c := legacyNewFakeClient(t, isvc, ir, newPod)
	tcr := legacyEnsureTargetCR(t, c, isvc, target)

	// Mid-recreate state: Phase A drained and deleted the incarnation-1 pod,
	// Phase B created the incarnation-2 pod; only Phase C (promotion) is left.
	fresh := &v1beta1.InferenceReplica{}
	key := client.ObjectKey{Namespace: isvc.Namespace, Name: legacyIRName(isvc, workload.ComponentEngine)}
	if err := c.Get(context.Background(), key, fresh); err != nil {
		t.Fatalf("get IR: %v", err)
	}
	fresh.Status.InstanceStatuses[0] = v1beta1.OMENativeInstanceStatus{
		Index:          0,
		Incarnation:    2,
		Phase:          v1beta1.OMENativeInstanceUpdating,
		TargetRevision: tcr.Name,
		Operation: &v1beta1.InstanceOperation{
			Type: v1beta1.InstanceOperationUpdate, Step: updateStepDrain, TargetRevision: tcr.Name,
		},
	}
	if err := c.Status().Update(context.Background(), fresh); err != nil {
		t.Fatalf("seed mid-recreate status: %v", err)
	}

	clk := clocktesting.NewFakeClock(minReadyWindowStart)
	input := legacyTestInput(isvc, c, workload.ComponentEngine)
	input.Clock = clk
	plan := legacyComponentPlan(workload.UpdateStrategyRecreatePod, nil)
	plan.MinReadySeconds = 20

	done, err := recreateUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], tcr, []*corev1.Pod{newPod})
	if err != nil {
		t.Fatalf("recreateUpdate inside window: %v", err)
	}
	if done {
		t.Fatalf("expected done=false while the recreated pod is inside the minReadySeconds window")
	}
	s := legacyInstanceStatusesOnIR(c, isvc, workload.ComponentEngine)[0]
	if s.Phase != v1beta1.OMENativeInstanceUpdating || s.Operation == nil {
		t.Fatalf("promoted inside the window: phase=%q operation=%+v", s.Phase, s.Operation)
	}

	clk.SetTime(minReadyWindowStart.Add(15 * time.Second))
	done, err = recreateUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], tcr, []*corev1.Pod{newPod})
	if err != nil {
		t.Fatalf("recreateUpdate after window: %v", err)
	}
	if !done {
		t.Fatalf("expected done=true once the recreated pod became Available")
	}
	s = legacyInstanceStatusesOnIR(c, isvc, workload.ComponentEngine)[0]
	if s.Phase != v1beta1.OMENativeInstanceReady || s.RunningRevision != tcr.Name || s.Operation != nil {
		t.Fatalf("promotion after window: phase=%q runningRevision=%q operation=%+v", s.Phase, s.RunningRevision, s.Operation)
	}
}

// convergedInPlaceFixture seeds an in-place update whose only remaining step
// is promotion: the pod is serving, already relabeled to the target revision,
// runs the target image, and the target revision's routed Service still
// lists it as ready, so any re-drain would park the pass at the drain check.
type convergedInPlaceFixture struct {
	isvc   *v1beta1.InferenceService
	client client.Client
	pod    *corev1.Pod
	target *corev1.PodSpec
	tcr    *appsv1.ControllerRevision
	plan   workload.ComponentPlan
}

func newConvergedInPlaceFixture(t *testing.T) convergedInPlaceFixture {
	t.Helper()
	legacyResetExpectations(t)
	isvc, ir := legacyISVCReadyAtIncarnation("llama-70b", "prod", 1)
	ir.Status.InstanceStatuses[0] = v1beta1.OMENativeInstanceStatus{
		Index:       0,
		Incarnation: 1,
		Phase:       v1beta1.OMENativeInstanceUpdating,
		Operation: &v1beta1.InstanceOperation{
			Type: v1beta1.InstanceOperationUpdate, Step: updateStepInPlace,
		},
	}
	isvc.Spec.Engine.ComponentExtensionSpec.Lifecycle = &v1beta1.LifecycleSpec{
		UpdateStrategy: &v1beta1.UpdateStrategy{
			Type: v1beta1.UpdateStrategyInPlaceIfPossible,
			InPlaceUpdateStrategy: &v1beta1.InPlaceUpdateStrategy{
				MarkNotReadyDuringLifecycle: legacyBoolPtr(true),
			},
		},
	}
	target := legacyTargetSpecImage("llama:v2")
	pod := legacyPodAtIncarnation(isvc, 0, 1, true /* ready */, false /* drained */)
	pod.Spec.Containers = []corev1.Container{{Name: "main", Image: "llama:v2"}}
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "main", Image: "llama:v2"}}
	podReadyAt(pod, minReadyWindowStart.Add(-5*time.Second))
	c := legacyNewFakeClient(t, isvc, ir, pod)
	legacySeedRunningRevision(t, c, isvc, workload.ComponentEngine, 0, legacyTargetSpecImage("llama:v1"))
	tcr := legacyEnsureTargetCR(t, c, isvc, target)
	legacyStampPodRevisionHash(t, c, pod, tcr.Name)
	targetService := query.PerRevisionServiceName(isvc.Name, workload.ComponentEngine, query.RevisionHashFromControllerRevisionName(tcr.Name))
	if err := c.Create(context.Background(), legacySliceWithEndpoint(isvc.Namespace, "engine-target", targetService, pod, true)); err != nil {
		t.Fatalf("seed target EndpointSlice: %v", err)
	}
	plan := legacyComponentPlan(workload.UpdateStrategyInPlaceIfPossible,
		&workload.InPlaceUpdateStrategy{MarkNotReadyDuringLifecycle: legacyBoolPtr(true)})
	return convergedInPlaceFixture{isvc: isvc, client: c, pod: pod, target: target, tcr: tcr, plan: plan}
}

func (f convergedInPlaceFixture) update(t *testing.T, clk *clocktesting.FakeClock) bool {
	t.Helper()
	input := legacyTestInput(f.isvc, f.client, workload.ComponentEngine)
	input.Clock = clk
	done, err := Update(context.Background(), legacyTestDeps(f.client), input, f.plan, f.plan.Instances[0], f.tcr, f.target)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	return done
}

func (f convergedInPlaceFixture) podServing(t *testing.T) bool {
	t.Helper()
	got := &corev1.Pod{}
	if err := f.client.Get(context.Background(), client.ObjectKeyFromObject(f.pod), got); err != nil {
		t.Fatalf("get pod: %v", err)
	}
	return podreadiness.IsServing(got)
}

// TestInPlaceUpdate_PromotionWaitsForMinReadySeconds: a converged in-place
// pod is returned to rotation and then held at Phase=Updating until it has
// been Ready for the window. The waiting passes must not drain the pod
// again — an EndpointSlice that still lists it as ready would then park the
// rollout at the drain check forever — so the pod stays serving throughout.
func TestInPlaceUpdate_PromotionWaitsForMinReadySeconds(t *testing.T) {
	f := newConvergedInPlaceFixture(t)
	f.plan.MinReadySeconds = 20
	clk := clocktesting.NewFakeClock(minReadyWindowStart)

	if f.update(t, clk) {
		t.Fatalf("expected done=false while the patched pod is inside the minReadySeconds window")
	}
	if !f.podServing(t) {
		t.Fatalf("converged pod must return to rotation before the window starts counting")
	}
	s := legacyInstanceStatusesOnIR(f.client, f.isvc, workload.ComponentEngine)[0]
	if s.Phase != v1beta1.OMENativeInstanceUpdating {
		t.Fatalf("promoted inside the window: phase=%q", s.Phase)
	}

	clk.SetTime(minReadyWindowStart.Add(15 * time.Second))
	if !f.update(t, clk) {
		t.Fatalf("expected done=true once the patched pod became Available (a re-drain would park at the drain check)")
	}
	if !f.podServing(t) {
		t.Fatalf("pod must stay in rotation across the wait")
	}
	s = legacyInstanceStatusesOnIR(f.client, f.isvc, workload.ComponentEngine)[0]
	if s.Phase != v1beta1.OMENativeInstanceReady || s.RunningRevision != f.tcr.Name || s.Operation != nil {
		t.Fatalf("promotion after window: phase=%q runningRevision=%q operation=%+v", s.Phase, s.RunningRevision, s.Operation)
	}
}

// TestInPlaceUpdate_ZeroWindowPromotionRetryDoesNotRedrain: with no window,
// a pass that finds the pod already relabeled to the target revision and
// serving (promotion did not persist on an earlier pass) promotes it as-is.
// Re-draining a converged pod would take capacity out of rotation for
// nothing and, with the routed Service still listing it, never observe it
// drained.
func TestInPlaceUpdate_ZeroWindowPromotionRetryDoesNotRedrain(t *testing.T) {
	f := newConvergedInPlaceFixture(t)
	clk := clocktesting.NewFakeClock(minReadyWindowStart)

	if !f.update(t, clk) {
		t.Fatalf("expected done=true: a converged, serving pod promotes without a window")
	}
	if !f.podServing(t) {
		t.Fatalf("converged pod must not be drained on the promotion pass")
	}
	s := legacyInstanceStatusesOnIR(f.client, f.isvc, workload.ComponentEngine)[0]
	if s.Phase != v1beta1.OMENativeInstanceReady || s.RunningRevision != f.tcr.Name || s.Operation != nil {
		t.Fatalf("promotion: phase=%q runningRevision=%q operation=%+v", s.Phase, s.RunningRevision, s.Operation)
	}
}

// TestSurgeUpdate_WindowGatesOnlyStepSurge: the window decides when the
// source may leave rotation, so it applies at Step=Surge only. Once the
// source is marked not-serving (Step=SurgeDrain), a replacement that is
// Ready but inside the window — as after a readiness flap — must not hold
// the drained source out of service; the pass proceeds to delete it.
func TestSurgeUpdate_WindowGatesOnlyStepSurge(t *testing.T) {
	legacyResetExpectations(t)
	isvc, ir := surgeISVCReady("llama-70b", "prod", 1)
	oldPod := surgePodAtOrdinal(isvc, 0, 1, 0, true, false /* already drained */)
	surgePod := surgePodAtOrdinal(isvc, 0, 1, 1, true, true)
	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{
		Name: "llama-70b-engine-rev-v2hash", Namespace: isvc.Namespace,
	}}
	surgePod.Labels[query.LabelRevisionHash] = query.RevisionHashFromControllerRevisionName(target.Name)
	podReadyAt(surgePod, minReadyWindowStart.Add(-10*time.Second))
	c := legacyNewFakeClient(t, isvc, ir, oldPod, surgePod, target)

	clk := clocktesting.NewFakeClock(minReadyWindowStart)
	plan := surgePlan()
	plan.MinReadySeconds = 20
	stamp := legacyTestInput(isvc, c, workload.ComponentEngine)
	stamp.Clock = clk
	if err := patchInstanceStatusSurgingForUpdate(context.Background(), stamp, 0, target.Name, plan.InstanceReadyTimeout); err != nil {
		t.Fatalf("pre-stamp surge: %v", err)
	}
	if err := patchInstanceStatusSurgeStepDrain(context.Background(), stamp, 0); err != nil {
		t.Fatalf("pre-stamp drain: %v", err)
	}
	// Observe the persisted Step=SurgeDrain, as a later pass would.
	input := legacyTestInput(isvc, c, workload.ComponentEngine)
	input.Clock = clk

	done, err := surgeUpdate(context.Background(), workload.Deps{Client: c}, input, plan, plan.Instances[0], target, []*corev1.Pod{oldPod, surgePod})
	if err != nil {
		t.Fatalf("surgeUpdate past Step=Surge: %v", err)
	}
	if done {
		t.Fatalf("expected done=false while deleting source")
	}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(oldPod), &corev1.Pod{}); !apierrors.IsNotFound(err) {
		t.Fatalf("drained source must be deleted without waiting out the window again: %v", err)
	}
}
