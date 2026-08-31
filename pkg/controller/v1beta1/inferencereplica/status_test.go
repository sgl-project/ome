package inferencereplica

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/obsmetrics"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/v1beta1convert"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

func materializeInlinePublicationStatuses(t *testing.T, insts []v1beta1.OMENativeInstanceStatus, byIndex map[int32][]*corev1.Pod, availableByPod map[string]struct{}) []v1beta1.OMENativeInstanceStatus {
	t.Helper()
	observation, err := workload.NewOwnedPublicationObservation(
		v1beta1convert.InstanceStatusSliceToWorkload(insts),
		workload.NewCachedSelectorPodObservation(nil, byIndex),
		availableByPod,
	)
	if err != nil {
		t.Fatalf("build publication observation: %v", err)
	}
	statuses, err := observation.TakeInlineV1Statuses()
	if err != nil {
		t.Fatalf("materialize publication status: %v", err)
	}
	return v1beta1convert.InstanceStatusSliceFromWorkload(statuses)
}

// TestPublicationObservation_StampsLivePodState pins the
// per-Instance projection: handed a (status slice, by-index pod
// map, available-pod set) tuple, it stamps each Instance's per-pod
// counters off the live pod state. This is the load-bearing step that
// lets the downstream publication classifier see the
// real pod state instead of a Phase-only proxy.
//
// Regression guard: an IR aggregator that never populates per-Instance
// ReadyPodCount makes downstream rollups read zero on every Instance
// even when the underlying pods are ContainersReady. This test pins
// the exact counter-write contract so a future refactor can't silently
// drop it.
func TestPublicationObservation_StampsLivePodState(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("llama-engine", "prod", 2)
	pod0 := podForIR(ir, 0, "default", 0, true, true)
	pod0.Spec.NodeName = "node-a"
	pod1 := podForIR(ir, 1, "default", 0, false, false)
	insts := []v1beta1.OMENativeInstanceStatus{
		// Pretend the legacy Phase signal lags reality: Instance 0
		// still Phase=Updating from a prior reconcile, but its pod is
		// already ContainersReady + serving. The fix should report
		// ReadyPodCount=1 / ServingPodCount=1 / AvailablePodCount=1
		// off the live pod state regardless of Phase.
		{Index: 0, Phase: v1beta1.OMENativeInstanceUpdating, PodCount: 0, ReadyPodCount: 0},
		// Pending Instance with a pod that hasn't flipped Ready —
		// counters should all be zero except PodCount.
		{Index: 1, Phase: v1beta1.OMENativeInstancePending, PodCount: 0, ReadyPodCount: 0},
	}
	byIndex := map[int32][]*corev1.Pod{
		0: {pod0},
		1: {pod1},
	}
	availableByPod := map[string]struct{}{pod0.Name: {}}

	insts = materializeInlinePublicationStatuses(t, insts, byIndex, availableByPod)

	g.Expect(insts[0].PodCount).To(gomega.Equal(int32(1)),
		"Instance 0 has 1 live pod")
	g.Expect(insts[0].ReadyPodCount).To(gomega.Equal(int32(1)),
		"Instance 0's pod is ContainersReady; ReadyPodCount must be 1 even though Phase=Updating")
	g.Expect(insts[0].ServingPodCount).To(gomega.Equal(int32(1)),
		"Instance 0's pod has serving=True; ServingPodCount must be 1")
	g.Expect(insts[0].AvailablePodCount).To(gomega.Equal(int32(1)),
		"Instance 0's pod is in the available EndpointSlice set")
	g.Expect(insts[0].NodesOccupied).To(gomega.Equal([]string{"node-a"}),
		"NodesOccupied must reflect the scheduling node")

	g.Expect(insts[1].PodCount).To(gomega.Equal(int32(1)),
		"Instance 1 has 1 live pod (not yet Ready)")
	g.Expect(insts[1].ReadyPodCount).To(gomega.Equal(int32(0)),
		"Instance 1's pod is not ContainersReady")
	g.Expect(insts[1].AvailablePodCount).To(gomega.Equal(int32(0)),
		"Instance 1's pod is not in the available set")
}

// TestPublicationObservation_EmptyPodSet pins the no-pods edge
// case: an Instance with zero matching pods (e.g., post-drain pre-
// recreate, scale-down victim mid-delete) must end up with all-zero
// counters. Catches a future bug where the loop accidentally retains
// stale counter values from a prior reconcile.
func TestPublicationObservation_EmptyPodSet(t *testing.T) {
	g := gomega.NewWithT(t)
	insts := []v1beta1.OMENativeInstanceStatus{
		// Stale stamps from a prior reconcile when this Instance had pods.
		{Index: 0, PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, AvailablePodCount: 1,
			NodesOccupied: []string{"node-x"}},
	}
	insts = materializeInlinePublicationStatuses(t, insts, map[int32][]*corev1.Pod{}, map[string]struct{}{})

	g.Expect(insts[0].PodCount).To(gomega.BeZero())
	g.Expect(insts[0].ReadyPodCount).To(gomega.BeZero())
	g.Expect(insts[0].ServingPodCount).To(gomega.BeZero())
	g.Expect(insts[0].AvailablePodCount).To(gomega.BeZero())
	g.Expect(insts[0].NodesOccupied).To(gomega.BeNil(),
		"NodesOccupied must round-trip nil ↔ nil rather than nil ↔ empty-slice")
}

// TestPublicationObservation_Admitted verifies the Admitted signal
// is computed correctly: true when the Instance has pods and none carry
// an admission scheduling gate (Kueue has granted quota), false while
// gated/queued or before pods exist. Used by the multi-cluster control
// plane to decide the fan-out race winner.
func TestPublicationObservation_Admitted(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("llama-engine", "prod", 3)

	// Index 0 → one normal (ungated) running pod
	pod0 := podForIR(ir, 0, "default", 0, true, true)

	// Index 1 → one pod with Kueue admission gate still present (queued)
	pod1 := podForIR(ir, 1, "default", 0, false, false)
	pod1.Spec.SchedulingGates = []corev1.PodSchedulingGate{{Name: "kueue.x-k8s.io/admission"}}

	// Index 2 → no pods (empty instance)

	insts := []v1beta1.OMENativeInstanceStatus{
		{Index: 0},
		{Index: 1},
		{Index: 2},
	}
	byIndex := map[int32][]*corev1.Pod{
		0: {pod0},
		1: {pod1},
		// 2: no entry (zero pods)
	}

	insts = materializeInlinePublicationStatuses(t, insts, byIndex, nil)

	g.Expect(insts[0].Admitted).To(gomega.BeTrue(),
		"Instance 0 has a pod and it is not admission-gated → Admitted=true")
	g.Expect(insts[1].Admitted).To(gomega.BeFalse(),
		"Instance 1's pod still carries the Kueue admission gate → Admitted=false")
	g.Expect(insts[2].Admitted).To(gomega.BeFalse(),
		"Instance 2 has no pods yet → Admitted=false")
}

func TestAggregateAndWriteStatusPersistsCompactRowsAndMirrorsCurrentObservations(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("model-engine", "default", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{{
		Index: 0, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady,
		ReadyPodCount: 9, ScheduledPodCount: 9, NodesOccupied: []string{"legacy-node"},
	}}
	pod := podForIR(ir, 0, "default", 0, true, true)
	pod.Spec.NodeName = "node-a"
	r, stored := newReconciler(t, ir, pod)
	statusWrites := 0
	r.Client = interceptor.NewClient(stored.(client.WithWatch), interceptor.Funcs{
		SubResourceUpdate: func(ctx context.Context, c client.Client, subresource string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			assertCompactInferenceReplicaWrite(t, obj)
			statusWrites++
			return c.SubResource(subresource).Update(ctx, obj, opts...)
		},
	})
	r.APIReader = r.Client
	plan := workload.ComponentPlan{
		Component: v1beta1convert.ComponentTypeToWorkload(ir.Spec.Component),
		Replicas:  1,
		Instances: []workload.InstancePlan{{
			Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}},
		}},
	}

	g.Expect(r.aggregateAndWriteStatus(context.Background(), ir, plan, nil)).To(gomega.Succeed())
	got := &v1beta1.InferenceReplica{}
	g.Expect(stored.Get(context.Background(), client.ObjectKeyFromObject(ir), got)).To(gomega.Succeed())
	g.Expect(got.Status.InstanceStatuses[0].Admitted).To(gomega.BeTrue())
	g.Expect(got.Status.InstanceStatuses[0].PodCount).To(gomega.Equal(int32(1)))
	g.Expect(got.Status.InstanceStatuses[0].ServingPodCount).To(gomega.Equal(int32(1)))
	g.Expect(got.Status.InstanceStatuses[0].ReadyPodCount).To(gomega.BeZero())
	g.Expect(got.Status.InstanceStatuses[0].ScheduledPodCount).To(gomega.BeZero())
	g.Expect(got.Status.InstanceStatuses[0].NodesOccupied).To(gomega.BeNil())
	g.Expect(got.Status.ReadyReplicas).To(gomega.Equal(int32(1)),
		"component readiness is computed from the current Pod observation")
	g.Expect(ir.Status.InstanceStatuses[0].Admitted).To(gomega.BeFalse(),
		"the caller snapshot retains its reconcile-entry admission value")
	g.Expect(ir.Status.InstanceStatuses[0].ReadyPodCount).To(gomega.Equal(int32(1)))
	g.Expect(ir.Status.InstanceStatuses[0].ScheduledPodCount).To(gomega.Equal(int32(1)))
	g.Expect(ir.Status.InstanceStatuses[0].NodesOccupied).To(gomega.Equal([]string{"node-a"}),
		"same-pass observations remain available without being persisted")

	rvAfterCompaction := got.ResourceVersion
	g.Expect(r.aggregateAndWriteStatus(context.Background(), got.DeepCopy(), plan, nil)).To(gomega.Succeed())
	steady := &v1beta1.InferenceReplica{}
	g.Expect(stored.Get(context.Background(), client.ObjectKeyFromObject(ir), steady)).To(gomega.Succeed())
	g.Expect(steady.ResourceVersion).To(gomega.Equal(rvAfterCompaction),
		"a steady compact publication must perform zero status writes")
	g.Expect(statusWrites).To(gomega.Equal(1),
		"legacy compaction writes once and the steady compact pass writes nothing")
}

func TestMirrorInstanceCountersUsesTransientPublicationIntersection(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
		{Index: 3, Phase: v1beta1.OMENativeInstanceDeleting, Admitted: true, PodCount: 9, NodesOccupied: []string{"old-node"}},
		{Index: 9, Phase: v1beta1.OMENativeInstanceReady, PodCount: 7, ReadyPodCount: 6},
		{Index: 1, Phase: v1beta1.OMENativeInstanceUpdating},
	}}
	publication := []workload.InstanceStatus{
		{Index: 1, PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, ScheduledPodCount: 1, AvailablePodCount: 1, NodesOccupied: []string{"node-a"}},
		{Index: 3, PodCount: 2, ReadyPodCount: 2, NodesOccupied: []string{"superseded"}},
		{Index: 3, PodCount: 4, ReadyPodCount: 3, ServingPodCount: 2, ScheduledPodCount: 4, AvailablePodCount: 1},
		{Index: 7, PodCount: 1, ReadyPodCount: 1},
	}

	mirrorInstanceCounters(status, publication)

	g.Expect(status.InstanceStatuses[0]).To(gomega.Equal(v1beta1.OMENativeInstanceStatus{
		Index: 3, Phase: v1beta1.OMENativeInstanceDeleting, Admitted: true,
		PodCount: 4, ReadyPodCount: 3, ServingPodCount: 2, ScheduledPodCount: 4, AvailablePodCount: 1,
	}), "the last duplicate publication row wins without changing lifecycle or admission")
	g.Expect(status.InstanceStatuses[1]).To(gomega.Equal(v1beta1.OMENativeInstanceStatus{
		Index: 9, Phase: v1beta1.OMENativeInstanceReady, PodCount: 7, ReadyPodCount: 6,
	}), "caller-only rows remain untouched")
	g.Expect(status.InstanceStatuses[2].Phase).To(gomega.Equal(v1beta1.OMENativeInstanceUpdating))
	g.Expect(status.InstanceStatuses[2].NodesOccupied).To(gomega.Equal([]string{"node-a"}))

	publication[0].NodesOccupied[0] = "mutated"
	g.Expect(status.InstanceStatuses[2].NodesOccupied).To(gomega.Equal([]string{"node-a"}),
		"mirrored node slices must not alias the transient publication")
	mirrorInstanceCounters(nil, publication)
}

// TestAggregateStatus_AvailableReplicas_MatchesReadyWithEndpointSlices
// pins the AvailableReplicas counter via the end-to-end Reconcile path.
// With pods ContainersReady AND published as Ready in the per-Component
// headless Service's EndpointSlice, AvailableReplicas should collapse
// onto ReadyReplicas. The aggregator post-fix reads availability off
// the slice exactly like the omenative direct path
// (TestAggregateAndWriteStatus_PerInstanceCountersFromObservedPods),
// so the two adapters produce byte-identical counters.
//
// Replaces the legacy assertion that AvailableReplicas always mirrors
// ReadyReplicas. The two CAN diverge — see
// TestAggregateStatus_AvailableReplicas_ZeroWhenSliceNotReady — so the
// surface tested here is "with EndpointSlices published Ready, the two
// match".
func TestAggregateStatus_AvailableReplicas_MatchesReadyWithEndpointSlices(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("llama-engine", "prod", 3)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady,
			PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, ActiveOrdinal: 0},
		{Index: 1, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady,
			PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, ActiveOrdinal: 0},
		{Index: 2, Incarnation: 1, Phase: v1beta1.OMENativeInstanceCreating,
			PodCount: 1, ReadyPodCount: 0, ServingPodCount: 0, ActiveOrdinal: 0},
	}
	pod0 := podForIR(ir, 0, "default", 0, true, true)
	pod1 := podForIR(ir, 1, "default", 0, true, true)
	pod2 := podForIR(ir, 2, "default", 0, false, false)
	slice0 := sliceForIRPod(ir, pod0, true)
	slice1 := sliceForIRPod(ir, pod1, true)
	r, c := newReconciler(t, ir, pod0, pod1, pod2, slice0, slice1)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())
	g.Expect(got.Status.ReadyReplicas).To(gomega.Equal(int32(2)),
		"ReadyReplicas should be 2 (two pods ContainersReady)")
	g.Expect(got.Status.AvailableReplicas).To(gomega.Equal(int32(2)),
		"AvailableReplicas should match ReadyReplicas when EndpointSlices publish both pods Ready")
}

// TestAggregateStatus_AvailableReplicas_ZeroWhenSliceNotReady pins the
// EndpointSlice-gated availability semantic: a pod ContainersReady but
// NOT yet in any EndpointSlice's Ready endpoints reads
// AvailablePodCount=0 → AvailableReplicas=0, while ReadyReplicas still
// reports 1. Mirrors the omenative direct path's
// TestAggregateAndWriteStatus_AvailableRequiresEndpointSliceReady.
//
// This is the "newly-Ready pod hasn't been picked up by kube-proxy
// yet" window — operators get a useful signal that the pod is alive
// but not in rotation, instead of the previous always-mirrors-ready
// behavior that hid the kube-proxy lag.
func TestAggregateStatus_AvailableReplicas_ZeroWhenSliceNotReady(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady,
			PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, ActiveOrdinal: 0},
	}
	pod0 := podForIR(ir, 0, "default", 0, true, true)
	// EndpointSlice publishes the endpoint as Ready=false (kube-proxy
	// hasn't picked it up yet).
	slice0 := sliceForIRPod(ir, pod0, false)
	r, c := newReconciler(t, ir, pod0, slice0)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())
	g.Expect(got.Status.ReadyReplicas).To(gomega.Equal(int32(1)),
		"ReadyReplicas counts the ContainersReady pod regardless of EndpointSlice state")
	g.Expect(got.Status.AvailableReplicas).To(gomega.Equal(int32(0)),
		"AvailableReplicas waits for EndpointSlice Ready=true (kube-proxy gate)")
}

// TestAggregateStatus_ReadyCondition_TrueWhenAllReady_RolloutDone pins
// the happy-path Ready=True branch: ReadyReplicas == Replicas AND
// CurrentRevision == UpdateRevision → Ready=True/AllInstancesReady.
//
// Tests computeReadyCondition directly because the full-Reconcile path
// is sensitive to the exact ControllerRevision-name shape the workload
// pipeline computes. The helper is the load-bearing logic; the
// integration test TestAggregateStatus_AvailableReplicas_MatchesReadyForSimpleCase
// covers the Reconcile-side wiring.
func TestAggregateStatus_ReadyCondition_TrueWhenAllReady_RolloutDone(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:        1,
		ReadyReplicas:   1,
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-a",
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, Phase: v1beta1.OMENativeInstanceReady},
		},
	}
	cond := computeReadyCondition(status, nil, nil, nil)
	g.Expect(cond.Type).To(gomega.Equal(InferenceReplicaConditionReady))
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionTrue),
		"Ready=True when ReadyReplicas == Replicas AND CurrentRevision == UpdateRevision")
	g.Expect(cond.Reason).To(gomega.Equal(ReasonAllInstancesReady))
}

// TestAggregateStatus_ReadyCondition_UnknownDuringRollout pins the
// in-flight-rollout branch: CurrentRevision != UpdateRevision →
// Ready=Unknown/RolloutInProgress.
//
// The explicit Unknown status (not False) is load-bearing: operators
// running `kubectl wait --for=condition=Ready` see "still churning,
// check back" instead of a False that would short-circuit their wait.
func TestAggregateStatus_ReadyCondition_UnknownDuringRollout(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:             2,
		ReadyReplicas:        1,
		UpdatedReadyReplicas: 1,
		CurrentRevision:      "rev-a",
		UpdateRevision:       "rev-b",
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: "rev-b"},
			{Index: 1, Phase: v1beta1.OMENativeInstanceUpdating, RunningRevision: "rev-a",
				TargetRevision: "rev-b"},
		},
	}
	cond := computeReadyCondition(status, nil, nil, nil)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionUnknown),
		"Ready=Unknown while CurrentRevision != UpdateRevision (rollout in flight)")
	g.Expect(cond.Reason).To(gomega.Equal(ReasonRolloutInProgress))
}

// TestAggregateStatus_ReadyCondition_StagedAtPartition pins the
// staged branch: a static rollingUpdate.partition holds Instances on the
// prior revision by design (CurrentRevision != UpdateRevision forever), so
// Ready must be True/Staged — NOT Unknown/RolloutInProgress.
func TestAggregateStatus_ReadyCondition_StagedAtPartition(t *testing.T) {
	g := gomega.NewWithT(t)
	part := int32(1)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:             3,
		ReadyReplicas:        3,
		UpdatedReadyReplicas: 2,
		CurrentRevision:      "rev-a",
		UpdateRevision:       "rev-b",
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: "rev-b"},
			{Index: 1, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: "rev-b"},
			{Index: 2, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: "rev-a"}, // held by partition
		},
	}
	cond := computeReadyCondition(status, nil, nil, &v1beta1.InferenceReplicaPacing{Partition: &part})
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionTrue),
		"Ready=True when converged to a static partition (2 on target + 1 held, all Ready)")
	g.Expect(cond.Reason).To(gomega.Equal(ReasonStaged))

	// Not yet converged (held instance still updating) → back to RolloutInProgress.
	status.InstanceStatuses[2].Phase = v1beta1.OMENativeInstanceUpdating
	status.ReadyReplicas = 2
	cond = computeReadyCondition(status, nil, nil, &v1beta1.InferenceReplicaPacing{Partition: &part})
	g.Expect(cond.Reason).To(gomega.Equal(ReasonRolloutInProgress),
		"not staged until the partitioned shape is fully reached")
}

// TestAggregateStatus_ReadyCondition_FalseOnStuck pins the
// stuck-replica-count branch: ReadyReplicas < Replicas with
// CurrentRevision == UpdateRevision (no rollout in flight) →
// Ready=False/ReplicaCountMismatch.
//
// Operationally this catches: a steady-state Component lost an Instance
// (pod evicted, node drained, etc.) but the workload pipeline hasn't
// either recovered or escalated yet. The False signal pages operators
// instead of letting kubectl-wait hang on a missing condition.
func TestAggregateStatus_ReadyCondition_FalseOnStuck(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:        2,
		ReadyReplicas:   1,
		ServingReplicas: 1,
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-a",
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: "rev-a"},
			// Pending (not Failed) so the InstanceFailed override doesn't
			// fire; we want the ReplicaCountMismatch branch.
			{Index: 1, Phase: v1beta1.OMENativeInstancePending, RunningRevision: "rev-a"},
		},
	}
	// nil lifecycle keeps the strict floor, so serving 1/2 is below it.
	cond := computeReadyCondition(status, nil, nil, nil)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionFalse),
		"Ready=False when serving is below the availability floor with no rollout in flight")
	g.Expect(cond.Reason).To(gomega.Equal(ReasonReplicaCountMismatch))
}

// TestAggregateStatus_ReadyCondition_TrueWhenServingWithinBudget verifies that
// lifecycle rollingUpdate.maxUnavailable supplies the Instance readiness floor.
func TestAggregateStatus_ReadyCondition_TrueWhenServingWithinBudget(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:        10,
		ReadyReplicas:   9,
		ServingReplicas: 9,
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-a",
	}
	mu := intstr.FromString("25%") // floor = 10 - ceil(2.5) = 7
	lifecycle := &v1beta1.LifecycleSpec{
		UpdateStrategy: &v1beta1.UpdateStrategy{
			RollingUpdate: &v1beta1.RollingUpdate{MaxUnavailable: &mu},
		},
	}
	cond := computeReadyCondition(status, nil, lifecycle, nil)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionTrue),
		"Ready=True when serving (9) >= floor (7) even though ReadyReplicas < Replicas")
	g.Expect(cond.Reason).To(gomega.Equal(ReasonMinimumAvailable))
}

func TestAggregateStatus_ReadyCondition_SurgeUsesDesiredReplicaFloor(t *testing.T) {
	g := gomega.NewWithT(t)
	desired := int32(16)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:        32,
		ReadyReplicas:   16,
		ServingReplicas: 16,
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-a",
	}
	mu := intstr.FromString("25%")
	lifecycle := &v1beta1.LifecycleSpec{
		UpdateStrategy: &v1beta1.UpdateStrategy{
			RollingUpdate: &v1beta1.RollingUpdate{MaxUnavailable: &mu},
		},
	}

	cond := computeReadyCondition(status, &desired, lifecycle, nil)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(cond.Reason).To(gomega.Equal(ReasonMinimumAvailable))
	g.Expect(cond.Message).To(gomega.Equal("16/32 Instances serving (min 12)"))
}

func TestAggregateStatus_ReadyCondition_PacingBudgetDoesNotRelaxReadiness(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:        10,
		ReadyReplicas:   9,
		ServingReplicas: 9,
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-a",
	}
	mu := intstr.FromString("100%")
	cond := computeReadyCondition(status, nil, nil, &v1beta1.InferenceReplicaPacing{MaxUnavailable: &mu})
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(cond.Reason).To(gomega.Equal(ReasonReplicaCountMismatch))
}

// TestAggregateStatus_ReadyCondition_FalseOnFailed pins the
// InstanceFailed override: any Instance Phase=Failed → Ready=False/
// InstanceFailed, regardless of revision state.
//
// This is the highest-priority branch in computeReadyCondition's
// precedence chain — a failed Instance overrides RolloutInProgress
// (rollout is permanently wedged), AllInstancesReady (one bad apple),
// and ReplicaCountMismatch (Failed is the more specific reason).
func TestAggregateStatus_ReadyCondition_FalseOnFailed(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:      2,
		ReadyReplicas: 1,
		// CurrentRevision != UpdateRevision would normally flip to
		// Unknown; the Failed override must win regardless.
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-b",
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, Phase: v1beta1.OMENativeInstanceReady},
			{Index: 1, Phase: v1beta1.OMENativeInstanceFailed},
		},
	}
	cond := computeReadyCondition(status, nil, nil, nil)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionFalse),
		"Ready=False when any Instance has Phase=Failed, overriding rollout-in-flight")
	g.Expect(cond.Reason).To(gomega.Equal(ReasonInstanceFailed))
}

// TestAggregateStatus_ReadyCondition_ObservedGenerationStamped pins
// that the Ready condition carries ObservedGeneration off the
// IR.Status.ObservedGeneration field so consumers can correlate
// condition transitions to a specific spec generation. Matches the
// pattern apimeta.SetStatusCondition + the standard K8s convention.
func TestAggregateStatus_ReadyCondition_ObservedGenerationStamped(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		ObservedGeneration: 7,
		Replicas:           1,
		ReadyReplicas:      1,
		CurrentRevision:    "rev-a",
		UpdateRevision:     "rev-a",
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, Phase: v1beta1.OMENativeInstanceReady},
		},
	}
	cond := computeReadyCondition(status, nil, nil, nil)
	g.Expect(cond.ObservedGeneration).To(gomega.Equal(int64(7)),
		"Ready condition must stamp ObservedGeneration off status.ObservedGeneration")
}

// TestAggregateStatus_RecreatePath_ReadyReplicasReflectsLivePods:
// during a recreate or SurgeThenDrain rollout the per-Instance
// lifecycle Phase can lag the live pod state (Phase=Updating while the
// new pod has flipped ContainersReady but the workload pipeline hasn't
// yet stamped Phase=Ready). The IR rollup MUST count the Instance as
// Ready off the live pod state rather than the lifecycle bookkeeping —
// otherwise ISVC's engine.omenative.readyReplicas reads 0 even though
// the pod is already serving.
//
// Setup mirrors the post-recreate moment: Instance 0 carries
// Phase=Updating with a deep operation snapshot (drain step
// committed), but its replacement pod is live and ContainersReady.
// After Reconcile, IR.Status.ReadyReplicas must be 1.
func TestAggregateStatus_RecreatePath_ReadyReplicasReflectsLivePods(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("llama-engine", "prod", 1)
	// Pretend a recreate is mid-flight: the controller has stamped
	// Phase=Updating with an Update operation but the new pod just
	// flipped ContainersReady. The aggregator must observe the live
	// pod state and count the Instance Ready regardless of Phase.
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{
			Index:           0,
			Incarnation:     2,
			Phase:           v1beta1.OMENativeInstanceUpdating,
			RunningRevision: "rev-a",
			TargetRevision:  "rev-b",
		},
	}
	// Live pod is ContainersReady + serving (post-recreate, pre-Phase
	// bump). The aggregator computes readiness directly from this pod.
	pod0 := podForIR(ir, 0, "default", 0, true, true)
	// EndpointSlice published — kube-proxy has picked the pod up so
	// AvailableReplicas can also reach 1.
	slice0 := sliceForIRPod(ir, pod0, true)
	r, c := newReconciler(t, ir, pod0, slice0)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())
	g.Expect(got.Status.ReadyReplicas).To(gomega.Equal(int32(1)),
		"ReadyReplicas must reflect live pod state during recreate (Phase=Updating but pod ContainersReady)")
	g.Expect(got.Status.AvailableReplicas).To(gomega.Equal(int32(1)),
		"AvailableReplicas must match ReadyReplicas when EndpointSlice publishes the pod Ready")
	g.Expect(got.Status.InstanceStatuses[0].ReadyPodCount).To(gomega.BeZero(),
		"per-Instance readiness is derived rather than persisted")
}

// TestAggregateStatus_RecreatePath_OldPodCountsReadyDuringDrain pins
// the operationally-critical surge-tolerant counter behavior: during
// the drain phase of a recreate, the OLD pod is still serving (the
// drain hasn't yet completed). The IR rollup MUST count it as Ready
// throughout drain so SurgeThenDrain / RecreatePod observers see a
// continuous ServingReplicas signal, not a drop-to-zero glitch that
// would page operators or trip the RatioBalanced gate.
//
// Setup: Instance Phase=Updating, drain step in progress. The
// canonical pod still carries ContainersReady=True. The aggregator
// must observe the pod state and count it Ready off
// surge-tolerant readiness classifier.
func TestAggregateStatus_RecreatePath_OldPodCountsReadyDuringDrain(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{
			Index:           0,
			Incarnation:     1,
			Phase:           v1beta1.OMENativeInstanceUpdating,
			RunningRevision: "rev-a",
			TargetRevision:  "rev-b",
			// The drain marker is in flight; the pod is still alive
			// and ContainersReady but the workload pipeline has set
			// Phase=Updating.
			Operation: &v1beta1.InstanceOperation{
				Type: v1beta1.InstanceOperationUpdate,
				Step: "Drain",
			},
		},
	}
	pod0 := podForIR(ir, 0, "default", 0, true, true)
	slice0 := sliceForIRPod(ir, pod0, true)
	r, c := newReconciler(t, ir, pod0, slice0)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())
	// Critical: ReadyReplicas stays 1 throughout drain even though
	// Phase=Updating. Without the fix this read as 0, breaking
	// SurgeThenDrain's "serving >= 1" continuity promise.
	g.Expect(got.Status.ReadyReplicas).To(gomega.Equal(int32(1)),
		"the still-serving canonical pod must count Ready during drain (Phase=Updating)")
}

// TestAggregateStatus_ReadyCondition_PersistedToInformerCache pins the
// end-to-end wiring: a reconciled IR with a real status update lands
// the Ready condition on the apiserver-side IR object, observable via
// apimeta.FindStatusCondition. Catches a future regression where the
// computeReadyCondition helper is correct but the aggregator forgets
// to call SetStatusCondition (or writes to the wrong slice).
//
// Uses the same setup as TestReconcile_StatusAggregator_RollsUpCounters:
// real pods backing the pre-seeded Instances so the workload pipeline
// doesn't reset state. ReadyReplicas=2 and empty CurrentRevision (first
// reconcile) means the True branch can't fire — we just assert the
// condition is present and well-formed.
func TestAggregateStatus_ReadyCondition_PersistedToInformerCache(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("llama-engine", "prod", 2)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady,
			PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, ActiveOrdinal: 0},
		{Index: 1, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady,
			PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, ActiveOrdinal: 0},
	}
	pod0 := podForIR(ir, 0, "default", 0, true, true)
	pod1 := podForIR(ir, 1, "default", 0, true, true)
	r, c := newReconciler(t, ir, pod0, pod1)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())

	cond := apimeta.FindStatusCondition(got.Status.Conditions, InferenceReplicaConditionReady)
	g.Expect(cond).NotTo(gomega.BeNil(),
		"Ready condition must be persisted to IR.Status.Conditions after every reconcile pass")
	g.Expect(cond.Type).To(gomega.Equal(InferenceReplicaConditionReady))
	// LastTransitionTime is stamped by apimeta.SetStatusCondition when
	// the condition is first inserted.
	g.Expect(cond.LastTransitionTime.IsZero()).To(gomega.BeFalse(),
		"LastTransitionTime must be set by SetStatusCondition")
}

// TestAggregateAndWriteStatus_NoWriteWhenUnchanged pins the no-op
// short-circuit: once an IR's status has converged, a second
// aggregateAndWriteStatus pass that recomputes the identical status must
// perform ZERO Status().Update writes. Each IR status write re-triggers
// this controller AND the irprojector rollup onto the parent ISVC, so at
// scale the prior unconditional write amplified idle reconciles into
// a write storm.
//
// Verified via ResourceVersion: the fake client bumps RV on every
// Status().Update. The first pass changes status (RV bumps); the second
// pass over an already-converged IR must leave RV untouched. The knative
// SetCondition / apimeta.SetStatusCondition LastTransitionTime-preserving
// semantics are what make the second-pass status DeepEqual the first.
func TestAggregateAndWriteStatus_NoWriteWhenUnchanged(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady,
			PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, ActiveOrdinal: 0},
	}
	pod0 := podForIR(ir, 0, "default", 0, true, true)
	slice0 := sliceForIRPod(ir, pod0, true)
	r, c := newReconciler(t, ir, pod0, slice0)

	plan := workload.ComponentPlan{
		Component: v1beta1convert.ComponentTypeToWorkload(ir.Spec.Component),
		Replicas:  1,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
	}
	key := types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace}

	// First pass: status changes (per-Instance counters, conditions,
	// LabelSelector, ObservedGeneration all get stamped) → a write lands.
	g.Expect(r.aggregateAndWriteStatus(context.Background(), ir, plan, nil)).To(gomega.Succeed())
	afterFirst := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(), key, afterFirst)).To(gomega.Succeed())
	rvAfterFirst := afterFirst.ResourceVersion

	// Second pass over the now-converged IR: recomputed status equals
	// live status → the DeepEqual guard must skip Status().Update, so the
	// ResourceVersion is unchanged.
	g.Expect(r.aggregateAndWriteStatus(context.Background(), afterFirst.DeepCopy(), plan, nil)).To(gomega.Succeed())
	afterSecond := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(), key, afterSecond)).To(gomega.Succeed())
	g.Expect(afterSecond.ResourceVersion).To(gomega.Equal(rvAfterFirst),
		"a no-op aggregateAndWriteStatus pass must perform ZERO writes (ResourceVersion unchanged)")
}

func TestAggregateAndWriteStatusReusesPublicationReadsAcrossConflictRetry(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("model-engine", "default", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{{
		Index: 0, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady,
		ReadyPodCount: 3, ScheduledPodCount: 3, NodesOccupied: []string{"legacy-node"},
	}}
	pod := podForIR(ir, 0, "default", 0, true, true)
	slice := sliceForIRPod(ir, pod, true)
	podLists := 0
	endpointSliceLists := 0
	statusWrites := 0
	funcs := interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			switch list.(type) {
			case *corev1.PodList:
				podLists++
			case *discoveryv1.EndpointSliceList:
				endpointSliceLists++
			}
			return c.List(ctx, list, opts...)
		},
		SubResourceUpdate: func(ctx context.Context, c client.Client, subresource string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
			assertCompactInferenceReplicaWrite(t, obj)
			statusWrites++
			if statusWrites == 1 {
				latest := &v1beta1.InferenceReplica{}
				if err := c.Get(ctx, client.ObjectKeyFromObject(obj), latest); err != nil {
					return err
				}
				latest.Status.InstanceStatuses[0].Phase = v1beta1.OMENativeInstanceDeleting
				latest.Status.InstanceStatuses[0].Operation = &v1beta1.InstanceOperation{
					ID: "delete-0", Type: v1beta1.InstanceOperationDelete, Step: "Drain",
				}
				latest.Status.InstanceStatuses[0].RunningRevision = "rev-target"
				latest.Status.InstanceStatuses[0].ReadyPodCount = 5
				latest.Status.InstanceStatuses[0].ScheduledPodCount = 5
				latest.Status.InstanceStatuses[0].NodesOccupied = []string{"concurrent-node"}
				latest.Status.InstanceStatuses = append(latest.Status.InstanceStatuses, v1beta1.OMENativeInstanceStatus{
					Index: 1, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: "rev-target",
					ReadyPodCount: 2, ScheduledPodCount: 2, NodesOccupied: []string{"concurrent-node"},
				})
				if err := c.SubResource(subresource).Update(ctx, latest); err != nil {
					return err
				}
				return apierrors.NewConflict(
					schema.GroupResource{Group: "ome.io", Resource: "inferencereplicas"},
					obj.GetName(), errors.New("conflict"))
			}
			return c.SubResource(subresource).Update(ctx, obj, opts...)
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(ir, pod, slice).
		WithStatusSubresource(&v1beta1.InferenceReplica{}).
		WithIndex(&corev1.Pod{}, query.OMENativePodIndexField, query.OMENativePodIndexExtractor).
		WithInterceptorFuncs(funcs).
		Build()
	r := &Reconciler{Client: c, APIReader: c, Log: logf.Log.WithName("test"), Expectations: workload.NewExpectations()}
	plan := workload.ComponentPlan{
		Component: v1beta1convert.ComponentTypeToWorkload(ir.Spec.Component),
		Replicas:  1,
		Instances: []workload.InstancePlan{{
			Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}},
		}},
	}

	target := &appsv1.ControllerRevision{ObjectMeta: metav1.ObjectMeta{Name: "rev-target"}}
	g.Expect(r.aggregateAndWriteStatus(context.Background(), ir, plan, target)).To(gomega.Succeed())
	g.Expect(statusWrites).To(gomega.Equal(2))
	g.Expect(podLists).To(gomega.Equal(1), "the publication Pod observation is captured outside conflict retry")
	g.Expect(endpointSliceLists).To(gomega.Equal(1), "the publication availability observation is captured outside conflict retry")
	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(), client.ObjectKeyFromObject(ir), got)).To(gomega.Succeed())
	g.Expect(got.Status.InstanceStatuses[0].Phase).To(gomega.Equal(v1beta1.OMENativeInstanceDeleting))
	g.Expect(got.Status.InstanceStatuses[0].Operation).NotTo(gomega.BeNil())
	g.Expect(got.Status.InstanceStatuses[0].Operation.ID).To(gomega.Equal("delete-0"))
	g.Expect(got.Status.InstanceStatuses).To(gomega.HaveLen(2))
	for _, status := range got.Status.InstanceStatuses {
		g.Expect(status.ReadyPodCount).To(gomega.BeZero())
		g.Expect(status.ScheduledPodCount).To(gomega.BeZero())
		g.Expect(status.NodesOccupied).To(gomega.BeNil())
	}
	g.Expect(got.Status.Replicas).To(gomega.Equal(int32(2)))
	g.Expect(got.Status.UpdatedReplicas).To(gomega.Equal(int32(2)))
	g.Expect(got.Status.UpdatedReadyReplicas).To(gomega.Equal(int32(1)))
}

func TestAggregateAndWriteStatus_RebasesAfterAdjacentLifecycleWrite(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{{
		Index: 0, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady,
	}}
	stale := ir.DeepCopy()
	apiClient := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(ir).
		WithStatusSubresource(&v1beta1.InferenceReplica{}).
		Build()
	key := client.ObjectKeyFromObject(ir)
	live := &v1beta1.InferenceReplica{}
	g.Expect(apiClient.Get(context.Background(), key, live)).To(gomega.Succeed())
	live.Status.InstanceStatuses[0].Phase = v1beta1.OMENativeInstanceDeleting
	live.Status.InstanceStatuses[0].Operation = &v1beta1.InstanceOperation{
		ID: "delete-0", Type: v1beta1.InstanceOperationDelete, Step: "Drain",
	}
	g.Expect(apiClient.Status().Update(context.Background(), live)).To(gomega.Succeed())

	staleCache := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(stale).
		WithStatusSubresource(&v1beta1.InferenceReplica{}).
		Build()
	r := &Reconciler{
		Client:       &staleReadingClient{Client: apiClient, reader: staleCache},
		APIReader:    apiClient,
		Log:          logf.Log.WithName("test"),
		Expectations: workload.NewExpectations(),
	}
	plan := workload.ComponentPlan{
		Component: v1beta1convert.ComponentTypeToWorkload(ir.Spec.Component),
		Replicas:  1,
		Instances: []workload.InstancePlan{{
			Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}},
		}},
	}

	g.Expect(r.aggregateAndWriteStatus(context.Background(), stale, plan, nil)).To(gomega.Succeed())
	got := &v1beta1.InferenceReplica{}
	g.Expect(apiClient.Get(context.Background(), key, got)).To(gomega.Succeed())
	g.Expect(got.Status.InstanceStatuses).To(gomega.HaveLen(1))
	g.Expect(got.Status.InstanceStatuses[0].Phase).To(gomega.Equal(v1beta1.OMENativeInstanceDeleting))
	g.Expect(got.Status.InstanceStatuses[0].Operation).NotTo(gomega.BeNil())
	g.Expect(got.Status.InstanceStatuses[0].Operation.ID).To(gomega.Equal("delete-0"),
		"aggregate status must preserve the adjacent lifecycle commit")
}

func TestAggregateAndWriteStatus_SameNameReplacementIsUntouched(t *testing.T) {
	g := gomega.NewWithT(t)
	stale := baselineIR("llama-engine", "prod", 1)
	replacement := stale.DeepCopy()
	replacement.UID = types.UID("replacement-uid")
	replacement.Status.Replicas = 7
	replacement.Status.ReadyReplicas = 6
	replacement.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{{
		Index: 0, Incarnation: 9, Phase: v1beta1.OMENativeInstanceDeleting,
	}}
	wantStatus := replacement.Status.DeepCopy()
	live, writes := newCountingStatusClient(t, 0, replacement)
	r := &Reconciler{
		Client:       live,
		APIReader:    live,
		Log:          logf.Log.WithName("test"),
		Expectations: workload.NewExpectations(),
	}
	plan := workload.ComponentPlan{
		Component: v1beta1convert.ComponentTypeToWorkload(stale.Spec.Component),
		Replicas:  1,
		Instances: []workload.InstancePlan{{
			Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}},
		}},
	}

	g.Expect(r.aggregateAndWriteStatus(context.Background(), stale, plan, nil)).To(gomega.Succeed())
	g.Expect(*writes).To(gomega.Equal(0))
	got := &v1beta1.InferenceReplica{}
	g.Expect(live.Get(context.Background(), client.ObjectKeyFromObject(replacement), got)).To(gomega.Succeed())
	g.Expect(got.UID).To(gomega.Equal(replacement.UID))
	g.Expect(got.Status).To(gomega.Equal(*wantStatus))
}

func TestAggregateAndWriteStatus_GenerationChangeAborts(t *testing.T) {
	g := gomega.NewWithT(t)
	stale := baselineIR("llama-engine", "prod", 1)
	liveObject := stale.DeepCopy()
	liveObject.Generation++
	liveObject.Status.Replicas = 7
	liveObject.Status.ReadyReplicas = 6
	wantStatus := liveObject.Status.DeepCopy()
	live, writes := newCountingStatusClient(t, 0, liveObject)
	r := &Reconciler{
		Client:       live,
		APIReader:    live,
		Log:          logf.Log.WithName("test"),
		Expectations: workload.NewExpectations(),
	}
	plan := workload.ComponentPlan{
		Component: v1beta1convert.ComponentTypeToWorkload(stale.Spec.Component),
		Replicas:  1,
		Instances: []workload.InstancePlan{{
			Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}},
		}},
	}
	metricBefore := make(map[string]float64)
	for _, result := range []string{obsmetrics.ResultSuccess, obsmetrics.ResultConflict, obsmetrics.ResultNotFound, obsmetrics.ResultError} {
		metricBefore[result] = irStatusUpdateMetric(t, result)
	}

	err := r.aggregateAndWriteStatus(context.Background(), stale, plan, nil)
	g.Expect(errors.Is(err, workload.ErrStatusMutationPrecondition)).To(gomega.BeTrue())
	g.Expect(*writes).To(gomega.Equal(0))
	for result, before := range metricBefore {
		g.Expect(irStatusUpdateMetric(t, result)).To(gomega.Equal(before),
			"a generation precondition must not report a terminal status-write outcome")
	}
	got := &v1beta1.InferenceReplica{}
	g.Expect(live.Get(context.Background(), client.ObjectKeyFromObject(liveObject), got)).To(gomega.Succeed())
	g.Expect(got.Generation).To(gomega.Equal(liveObject.Generation))
	g.Expect(got.Status).To(gomega.Equal(*wantStatus))
}

// liveReadsThroughPromotion is how many InferenceReplica reads reach the
// authoritative reader up to and including the CurrentRevision promotion:
// the migration-entry sync, the aggregate-condition write, then promotion.
// Every conflict-retry closure re-reads its base live, so promotion is not
// the first live read.
const liveReadsThroughPromotion = 3

type firstIRGenerationReader struct {
	client.Reader
	generation int64
	// staleUntil is how many leading InferenceReplica reads report the
	// pre-change generation. The window has to cover everything up to and
	// including promotion so the generation change lands between promotion
	// and aggregation.
	staleUntil int
	reads      int
}

func (r *firstIRGenerationReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := r.Reader.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	ir, ok := obj.(*v1beta1.InferenceReplica)
	if !ok {
		return nil
	}
	r.reads++
	if r.reads <= r.staleUntil {
		ir.Generation = r.generation
	}
	return nil
}

func TestReconcile_GenerationChangeRequeuesBeforeDeferredEffects(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		changeBeforeAggregation bool
	}{
		{name: "promotion observes the new generation"},
		{name: "aggregation observes the new generation", changeBeforeAggregation: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			stale := baselineIR("llama-engine", "prod", 1)
			stale.Finalizers = []string{TeardownFinalizer}
			stale.Spec.Paused = true
			// Retention is configured (per-IR spec limit) so the sweep
			// WOULD trim the fence if the deferred pass wrongly ran it.
			stale.Spec.RevisionHistoryLimit = ptr.To(int32(testRevisionRetention))
			liveObject := stale.DeepCopy()
			liveObject.Generation++

			objects := []client.Object{liveObject}
			oldestRevision := ""
			for i := 0; i < testRevisionRetention+2; i++ {
				revision := seedControllerRevision(liveObject, fmt.Sprintf("fence%02d", i), int64(i+1))
				if i == 0 {
					oldestRevision = revision.Name
				}
				objects = append(objects, revision)
			}
			r, live := newReconciler(t, objects...)
			staleCache := fake.NewClientBuilder().
				WithScheme(testScheme(t)).
				WithObjects(stale).
				Build()
			r.Client = &staleReadingClient{Client: live, reader: staleCache}
			var generationReader *firstIRGenerationReader
			if tc.changeBeforeAggregation {
				generationReader = &firstIRGenerationReader{Reader: live, generation: stale.Generation, staleUntil: liveReadsThroughPromotion}
				r.APIReader = generationReader
			} else {
				r.APIReader = live
			}

			result, err := r.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(stale),
			})
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(result.Requeue).To(gomega.BeTrue())
			g.Expect(result.RequeueAfter).To(gomega.BeZero())
			if generationReader != nil {
				g.Expect(generationReader.reads).To(gomega.Equal(liveReadsThroughPromotion+1),
					"promotion must accept the starting generation before aggregation observes the change")
			}

			got := &v1beta1.InferenceReplica{}
			g.Expect(live.Get(context.Background(), client.ObjectKeyFromObject(liveObject), got)).To(gomega.Succeed())
			g.Expect(got.Status.ObservedGeneration).NotTo(gomega.Equal(got.Generation))
			survivors := listRevisionNames(t, live, liveObject.Namespace)
			g.Expect(survivors).To(gomega.HaveKey(oldestRevision),
				"the stale deferred pass must not run revision retention")
		})
	}
}

// TestAggregateAndWriteStatus_ConflictSurfacesAsConflict pins the
// terminal-conflict contract the deferred Reconcile handler keys on: when
// every status write loses the optimistic-lock race (RetryOnConflict
// exhausted), aggregateAndWriteStatus returns an error that
// apierrors.IsConflict recognizes — so the caller maps it to a benign
// requeue instead of an ERROR-level "Reconciler error".
func TestAggregateAndWriteStatus_ConflictSurfacesAsConflict(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, Incarnation: 1, Phase: v1beta1.OMENativeInstanceReady,
			PodCount: 1, ReadyPodCount: 1, ServingPodCount: 1, ActiveOrdinal: 0},
	}
	pod0 := podForIR(ir, 0, "default", 0, true, true)

	// Force every status write to lose the optimistic-lock race so the
	// aggregator returns the terminal conflict RetryOnConflict surfaces.
	conflict := interceptor.Funcs{
		SubResourceUpdate: func(_ context.Context, _ client.Client, _ string, obj client.Object, _ ...client.SubResourceUpdateOption) error {
			return apierrors.NewConflict(
				schema.GroupResource{Group: "ome.io", Resource: "inferencereplicas"},
				obj.GetName(), fmt.Errorf("the object has been modified"))
		},
	}
	c := fake.NewClientBuilder().
		WithScheme(testScheme(t)).
		WithObjects(ir, pod0).
		WithStatusSubresource(&v1beta1.InferenceReplica{}).
		WithInterceptorFuncs(conflict).
		Build()
	r := &Reconciler{Client: c, APIReader: c, Log: logf.Log.WithName("test"), Expectations: workload.NewExpectations()}

	plan := workload.ComponentPlan{
		Component: v1beta1convert.ComponentTypeToWorkload(ir.Spec.Component),
		Replicas:  1,
		Instances: []workload.InstancePlan{
			{Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}}},
		},
	}

	err := r.aggregateAndWriteStatus(context.Background(), ir, plan, nil)
	g.Expect(err).To(gomega.HaveOccurred())
	g.Expect(apierrors.IsConflict(err)).To(gomega.BeTrue(),
		"a terminal status conflict must remain recognizable so the caller can requeue instead of logging ERROR")
}

// TestAggregateStatus_BadImageSurge_EscalatesInstanceToFailed pins the
// end-to-end fast-escalation path: a SurgeThenDrain surge pod stuck in
// ImagePullBackOff past the config-driven stuck-pod grace
// (lifecycle stuckPodGracePeriod) must escalate the Instance Phase to
// Failed within the grace window — not after the full 30-minute
// InstanceReadyTimeout — so the parent ISVC's coordination layer sees
// Component.Failed promptly rather than the group sitting at Surging.
//
// Setup mirrors a realistic mid-surge shape:
//   - Instance idx=0 carries Phase=Updating with an in-flight
//     Operation{Type=Update, Step=Surge} (the surge stamp the workload
//     pipeline writes when starting SurgeThenDrain).
//   - Two pods at ordinal 0 / 1 (old serving + surge).
//   - The surge pod (ordinal 1) is past the grace window with
//     ImagePullBackOff in its ContainerStatuses.
//
// Expected post-Reconcile: Instance Phase flips to Failed via the
// workload escalation pass at the end of workload.Reconcile. Without it
// the test fails: Phase stays at Updating because nothing consumes the
// stuck-pod evidence.
func TestAggregateStatus_BadImageSurge_EscalatesInstanceToFailed(t *testing.T) {
	g := gomega.NewWithT(t)

	ir := baselineIR("llama-engine", "prod", 1)
	// Mid-surge state: Instance Phase=Updating with an in-flight Surge
	// step. Status pre-seeded so the workload dispatcher's
	// DetectUpdateTrigger sees the in-progress operation rather than
	// firing a fresh surge.
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{
			Index:           0,
			Incarnation:     1,
			Phase:           v1beta1.OMENativeInstanceUpdating,
			RunningRevision: "rev-a",
			TargetRevision:  "rev-b",
			ActiveOrdinal:   0,
			Operation: &v1beta1.InstanceOperation{
				Type: v1beta1.InstanceOperationUpdate,
				Step: "Surge",
				// Far-future deadline so the slow-path expiry
				// (production: 30 min) doesn't race the fast-path
				// escalator we're verifying.
				Deadline: metav1.NewTime(time.Now().Add(30 * time.Minute)),
			},
		},
	}
	// Old (canonical) pod at ordinal 0 — serving v1 traffic.
	oldPod := podForIR(ir, 0, "default", 0, true /*ready*/, true /*serving*/)
	// Surge pod at ordinal 1 — image pulled into ImagePullBackOff.
	// CreationTimestamp set well past the grace window so
	// PodStuckPullFailure's age check fires.
	surgePod := podForIR(ir, 0, "default", 1, false /*ready*/, false /*serving*/)
	surgePod.CreationTimestamp = metav1.NewTime(time.Now().Add(-5 * time.Second))
	surgePod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{
			Name: "ome-container",
			State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"},
			},
		},
	}

	r, c := newReconcilerWithGrace(t, 1*time.Millisecond, ir, oldPod, surgePod)
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())

	g.Expect(got.Status.InstanceStatuses).To(gomega.HaveLen(1))
	g.Expect(got.Status.InstanceStatuses[0].Phase).
		To(gomega.Equal(v1beta1.OMENativeInstanceFailed),
			"workload escalation pass must consume the stuck-pod evidence: surge pod stuck in ImagePullBackOff past grace must flip Instance Phase to Failed")
	// The deadline disposition owns single-pod update escalations:
	// ImagePullBackOff is a workload-caused failure, so the Operation is
	// CLEARED in the same transition (Failed-with-no-Operation is a fresh
	// start; the old Failed-with-Operation continuation re-armed the
	// stampers every pass — the churn loop) and the target revision is
	// recorded in a RetryBlock (Held here: no updateRetry policy is
	// configured, fail-safe).
	g.Expect(got.Status.InstanceStatuses[0].Operation).To(gomega.BeNil(),
		"disposition must clear the failed attempt's Operation in the same transition")
	g.Expect(got.Status.RetryBlocks).To(gomega.HaveLen(1),
		"workload-caused failure must record a RetryBlock for the attempt's target revision")
	g.Expect(got.Status.RetryBlocks[0].State).To(gomega.Equal(v1beta1.RetryBlockHeld),
		"nil retry policy fails safe: first failure Holds")
	g.Expect(got.Status.RetryBlocks[0].Reason).To(gomega.Equal("ImagePullBackOff"))
	g.Expect(got.Status.InstanceStatuses[0].LastFailure).NotTo(gomega.BeNil())
	g.Expect(got.Status.InstanceStatuses[0].LastFailure.Reason).To(gomega.Equal("ImagePullBackOff"))
}

// TestAggregateStatus_StuckPodWithinGrace_NoEscalation pins the
// IR-path debounce: a stuck pod still inside the config-driven
// stuck-pod grace (lifecycle stuckPodGracePeriod) must NOT escalate.
// Without this guard, a brief transient pull failure
// on a flaky registry would mis-classify a pod that's about to recover.
// Mirrors the equivalent omenative-direct-path test
// TestSurgeUpdate_ImagePullBackOff_WithinGrace_NoEscalation.
func TestAggregateStatus_StuckPodWithinGrace_NoEscalation(t *testing.T) {
	g := gomega.NewWithT(t)

	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{
			Index:           0,
			Incarnation:     1,
			Phase:           v1beta1.OMENativeInstanceUpdating,
			RunningRevision: "rev-a",
			TargetRevision:  "rev-b",
			ActiveOrdinal:   0,
			Operation: &v1beta1.InstanceOperation{
				Type:     v1beta1.InstanceOperationUpdate,
				Step:     "Surge",
				Deadline: metav1.NewTime(time.Now().Add(30 * time.Minute)),
			},
		},
	}
	oldPod := podForIR(ir, 0, "default", 0, true, true)
	surgePod := podForIR(ir, 0, "default", 1, false, false)
	surgePod.CreationTimestamp = metav1.NewTime(time.Now().Add(-5 * time.Second))
	surgePod.Status.ContainerStatuses = []corev1.ContainerStatus{
		{Name: "ome-container", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}},
	}
	// Generous explicit grace so the 5-second-old pod stays inside the
	// window with fast escalation ACTIVE (a zero grace would disable the
	// fast path entirely and make this test vacuous).
	r, c := newReconcilerWithGrace(t, 60*time.Second, ir, oldPod, surgePod)
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())
	g.Expect(got.Status.InstanceStatuses[0].Phase).
		To(gomega.Equal(v1beta1.OMENativeInstanceUpdating),
			"stuck pod still inside grace window must NOT escalate (debounce holds)")
}

// TestAggregateStatus_DeadlineExpired_EscalatesInstanceToFailed pins the
// broad backstop end-to-end: an Instance stuck in a transient phase whose
// in-flight Operation.Deadline (InstanceReadyTimeout) has elapsed must
// flip to Phase=Failed via the workload escalation pass — even when no
// pod is in a terminal kubelet waiting state, so the fast-path stuck-pod
// evidence CANNOT be what fails it.
//
// This is the regression guard for the InstanceReadyTimeout consumer:
// without the deadline branch of the escalation pass the timeout is
// inert and a never-Ready gang sits at Phase=Updating indefinitely.
func TestAggregateStatus_DeadlineExpired_EscalatesInstanceToFailed(t *testing.T) {
	g := gomega.NewWithT(t)

	ir := baselineIR("llama-engine", "prod", 1)
	// Create-stuck Instance: Phase=Creating with an in-flight Create
	// operation whose Deadline already elapsed. patchInstanceStatusCreating
	// is idempotent (Phase=Creating + Op.Type=Create + Incarnation>0 → no
	// re-stamp), so the dispatcher's Create pass leaves the expired
	// Deadline intact for the backstop to read. A pod exists at the
	// canonical ordinal but never went Ready — the never-converging gang
	// the InstanceReadyTimeout is meant to bound.
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{
			Index:         0,
			Incarnation:   1,
			Phase:         v1beta1.OMENativeInstanceCreating,
			ActiveOrdinal: 0,
			Operation: &v1beta1.InstanceOperation{
				Type: v1beta1.InstanceOperationCreate,
				Step: "CreatePods",
				// Deadline already in the past → the InstanceReadyTimeout
				// backstop must escalate this Instance.
				Deadline: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
			},
		},
	}
	// Pod exists at the canonical ordinal but is not Ready (Pending) and
	// has NO terminal kubelet waiting reason, so the stuck-pod escalator
	// stays out of it.
	pendingPod := podForIR(ir, 0, "default", 0, false /*ready*/, false /*serving*/)

	// Generous stuck-pod grace so the fast path can't fire — the ONLY
	// thing that can fail this Instance is the deadline backstop under
	// test.
	r, c := newReconcilerWithGrace(t, 60*time.Second, ir, pendingPod)
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())

	g.Expect(got.Status.InstanceStatuses).To(gomega.HaveLen(1))
	g.Expect(got.Status.InstanceStatuses[0].Phase).
		To(gomega.Equal(v1beta1.OMENativeInstanceFailed),
			"workload escalation pass must run the deadline backstop: an Operation.Deadline in the past must flip the Instance Phase to Failed")
	// The deadline disposition owns Create-attempt expiries: no
	// workload-caused pod evidence and no relocation budget configured →
	// terminal branch clears the Operation (fresh start next pass) and
	// records no RetryBlock (failure not proven revision-scoped).
	g.Expect(got.Status.InstanceStatuses[0].Operation).To(gomega.BeNil(),
		"disposition must clear the expired attempt's Operation")
	g.Expect(got.Status.RetryBlocks).To(gomega.BeEmpty(),
		"terminal disposition must not record a RetryBlock")
	// LastFailure records the DeadlineExceeded cause.
	g.Expect(got.Status.InstanceStatuses[0].LastFailure).NotTo(gomega.BeNil(),
		"deadline escalation must record a LastFailure diagnostic")
	g.Expect(got.Status.InstanceStatuses[0].LastFailure.Reason).
		To(gomega.Equal(workload.DeadlineExceededReason))
}

// TestAggregateStatus_GatedInstance_DeadlineParked_NotEscalated pins the
// admission-gating exemption: an Instance whose Operation.Deadline is in
// the past but whose pod is held by a scheduling gate (queued for
// admission, e.g. by Kueue) is NOT stuck — it is queued. The IR aggregator
// must park its deadline (zero it) so the backstop never fails it. Without
// this, every Kueue-gated OME workload would die at InstanceReadyTimeout.
func TestAggregateStatus_GatedInstance_DeadlineParked_NotEscalated(t *testing.T) {
	g := gomega.NewWithT(t)

	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{
			Index:         0,
			Incarnation:   1,
			Phase:         v1beta1.OMENativeInstanceCreating,
			ActiveOrdinal: 0,
			Operation: &v1beta1.InstanceOperation{
				Type: v1beta1.InstanceOperationCreate,
				Step: "CreatePods",
				// Past deadline: would be escalated WERE the pod not gated.
				Deadline: metav1.NewTime(time.Now().Add(-1 * time.Minute)),
			},
		},
	}
	// Pod exists at the canonical ordinal but is held by a scheduling gate
	// (queued for admission), not Ready and not stuck.
	gatedPod := podForIR(ir, 0, "default", 0, false /*ready*/, false /*serving*/)
	gatedPod.Spec.SchedulingGates = []corev1.PodSchedulingGate{{Name: "kueue.x-k8s.io/admission"}}

	// Explicit generous grace: fast escalation is active but the gated
	// pod carries no stuck evidence, so only the deadline backstop could
	// fail it — and the gate exemption must stop that.
	r, c := newReconcilerWithGrace(t, 60*time.Second, ir, gatedPod)
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())

	g.Expect(got.Status.InstanceStatuses).To(gomega.HaveLen(1))
	g.Expect(got.Status.InstanceStatuses[0].Phase).
		NotTo(gomega.Equal(v1beta1.OMENativeInstanceFailed),
			"a Kueue-gated Instance is queued, not stuck: the deadline backstop must NOT fail it")
	g.Expect(got.Status.InstanceStatuses[0].Operation).NotTo(gomega.BeNil())
	g.Expect(got.Status.InstanceStatuses[0].Operation.Deadline.IsZero()).
		To(gomega.BeTrue(),
			"a gated Instance's deadline must be parked (zeroed) so it never expires while queued")
}

// TestReconcileHeldDeadlines_GangSurgeSourceParked pins the surge-pair
// parking rule: while a gang-surge attempt's pods (living in the
// source's Operation.SurgeIndex bucket) queue for admission, BOTH the
// target marker's deadline (own bucket gated) and the SOURCE's deadline
// must park. Without the source-side park, only the target waits — the
// source's clock keeps running and expires during a legitimate queue
// wait, tearing down the queued gang.
func TestReconcileHeldDeadlines_GangSurgeSourceParked(t *testing.T) {
	g := gomega.NewWithT(t)

	surgeIdx := int32(1)
	future := metav1.NewTime(time.Now().Add(30 * time.Minute))
	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{
			Index:       0,
			Incarnation: 1,
			Phase:       v1beta1.OMENativeInstanceUpdating,
			Operation: &v1beta1.InstanceOperation{
				Type:       v1beta1.InstanceOperationUpdate,
				Step:       "Surge",
				SurgeIndex: &surgeIdx,
				Deadline:   future,
			},
		},
		{
			Index:       1,
			Incarnation: 1,
			Phase:       v1beta1.OMENativeInstanceCreating,
			Operation: &v1beta1.InstanceOperation{
				Type:     v1beta1.InstanceOperationUpdate,
				Step:     workload.UpdateStepGangSurgeTarget,
				Deadline: future,
			},
		},
	}
	sourcePod := podForIR(ir, 0, "default", 0, true, true)
	gatedPod := podForIR(ir, 1, "default", 0, false, false)
	gatedPod.Spec.SchedulingGates = []corev1.PodSchedulingGate{{Name: "kueue.x-k8s.io/admission"}}

	r, c := newReconciler(t, ir, sourcePod, gatedPod)
	byIndex := map[int32][]*corev1.Pod{0: {sourcePod}, 1: {gatedPod}}
	g.Expect(r.reconcileHeldDeadlines(context.Background(), ir, byIndex, false, 30*time.Minute)).To(gomega.Succeed())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())
	g.Expect(got.Status.InstanceStatuses).To(gomega.HaveLen(2))
	g.Expect(got.Status.InstanceStatuses[0].Operation.Deadline.IsZero()).
		To(gomega.BeTrue(),
			"gang-surge SOURCE deadline must park while its SurgeIndex bucket is admission-gated")
	g.Expect(got.Status.InstanceStatuses[1].Operation.Deadline.IsZero()).
		To(gomega.BeTrue(),
			"gang-surge target deadline must park while its own pods are admission-gated")
}

// TestAggregateStatus_PausedInstance_DeadlineParkedAndRearmed verifies that
// pausing an in-flight operation is a real circuit breaker: neither terminal
// pod state nor an elapsed deadline may fail it while paused, and unpausing
// restarts the timeout window instead of expiring the old deadline.
func TestAggregateStatus_PausedInstance_DeadlineParkedAndRearmed(t *testing.T) {
	g := gomega.NewWithT(t)

	ir := baselineIR("llama-engine", "prod", 1)
	ir.Spec.Paused = true
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{{
		Index:         0,
		Incarnation:   1,
		Phase:         v1beta1.OMENativeInstanceCreating,
		ActiveOrdinal: 0,
		Operation: &v1beta1.InstanceOperation{
			Type:     v1beta1.InstanceOperationCreate,
			Step:     "CreatePods",
			Deadline: metav1.NewTime(time.Now().Add(-time.Minute)),
		},
	}}
	stuckPod := podForIR(ir, 0, "default", 0, false, false)
	stuckPod.CreationTimestamp = metav1.NewTime(time.Now().Add(-time.Minute))
	stuckPod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "ome-container",
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
			Reason: "CrashLoopBackOff",
		}},
	}}

	r, c := newReconcilerWithGrace(t, time.Millisecond, ir, stuckPod)
	key := types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	paused := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(), key, paused)).To(gomega.Succeed())
	g.Expect(paused.Status.InstanceStatuses[0].Phase).To(gomega.Equal(v1beta1.OMENativeInstanceCreating))
	g.Expect(paused.Status.InstanceStatuses[0].LastFailure).To(gomega.BeNil())
	g.Expect(paused.Status.InstanceStatuses[0].Operation.Deadline.IsZero()).To(gomega.BeTrue())

	// Simulate the container recovering while the operator investigates. The
	// unpause assertion below isolates deadline rearming from the independent
	// stuck-pod fast-failure policy.
	recoveredPod := &corev1.Pod{}
	g.Expect(c.Get(context.Background(), client.ObjectKeyFromObject(stuckPod), recoveredPod)).To(gomega.Succeed())
	recoveredPod.Status.ContainerStatuses = nil
	g.Expect(c.Status().Update(context.Background(), recoveredPod)).To(gomega.Succeed())

	paused.Spec.Paused = false
	g.Expect(c.Update(context.Background(), paused)).To(gomega.Succeed())
	beforeUnpause := time.Now()
	_, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	unpaused := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(), key, unpaused)).To(gomega.Succeed())
	g.Expect(unpaused.Status.InstanceStatuses[0].Phase).To(gomega.Equal(v1beta1.OMENativeInstanceCreating))
	g.Expect(unpaused.Status.InstanceStatuses[0].Operation.Deadline.Time).To(gomega.BeTemporally(">", beforeUnpause))
}

// TestAggregateStatus_DeadlineFuture_NoEscalation pins the negative
// case: a transient-phase Instance whose Operation.Deadline is still in
// the future must NOT be escalated by the deadline backstop.
func TestAggregateStatus_DeadlineFuture_NoEscalation(t *testing.T) {
	g := gomega.NewWithT(t)

	ir := baselineIR("llama-engine", "prod", 1)
	ir.Status.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{
			Index:         0,
			Incarnation:   1,
			Phase:         v1beta1.OMENativeInstanceCreating,
			ActiveOrdinal: 0,
			Operation: &v1beta1.InstanceOperation{
				Type:     v1beta1.InstanceOperationCreate,
				Step:     "CreatePods",
				Deadline: metav1.NewTime(time.Now().Add(30 * time.Minute)),
			},
		},
	}
	pendingPod := podForIR(ir, 0, "default", 0, false, false)

	// Explicit generous grace: fast escalation active, but the pending
	// pod has no stuck evidence — only the (future) deadline could fire.
	r, c := newReconcilerWithGrace(t, 60*time.Second, ir, pendingPod)
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
	})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace},
		got)).To(gomega.Succeed())
	g.Expect(got.Status.InstanceStatuses[0].Phase).
		To(gomega.Equal(v1beta1.OMENativeInstanceCreating),
			"Operation.Deadline in the future must NOT escalate (backstop respects the timeout window)")
}

// TestComputeRolloutStalledCondition pins the advisory RolloutStalled
// condition: True only when a rollout is in flight AND an Instance carries a
// terminal failure while not yet on the target; and it is a DISTINCT condition
// type from Ready (advisory, not a Ready dependent).
func TestComputeRolloutStalledCondition(t *testing.T) {
	const target = "isvc-engine-5af56222"
	const cur = "isvc-engine-0f41b090"
	mk := func() *v1beta1.InferenceReplicaStatus {
		return &v1beta1.InferenceReplicaStatus{Replicas: 3, CurrentRevision: cur, UpdateRevision: target}
	}

	// in-flight rollout + an Instance failing while NOT on target → stalled.
	s := mk()
	s.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, RunningRevision: target},
		{Index: 1, RunningRevision: cur, LastFailure: &v1beta1.InstanceTermination{Reason: "CrashLoopBackOff"}},
	}
	if c := computeRolloutStalledCondition(s); c.Status != metav1.ConditionTrue || c.Reason != ReasonInstancesFailing {
		t.Errorf("failing Instance mid-rollout: got %s/%s want True/%s", c.Status, c.Reason, ReasonInstancesFailing)
	}

	// stale failure on an Instance ALREADY on target → not stalled.
	s2 := mk()
	s2.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, RunningRevision: target, LastFailure: &v1beta1.InstanceTermination{Reason: "CrashLoopBackOff"}},
	}
	if c := computeRolloutStalledCondition(s2); c.Status != metav1.ConditionFalse {
		t.Errorf("stale failure on updated Instance must not stall: got %s", c.Status)
	}

	// no rollout in flight (converged) → not stalled even with a failure.
	s3 := mk()
	s3.CurrentRevision = target
	s3.InstanceStatuses = []v1beta1.OMENativeInstanceStatus{
		{Index: 0, RunningRevision: cur, LastFailure: &v1beta1.InstanceTermination{Reason: "CrashLoopBackOff"}},
	}
	if c := computeRolloutStalledCondition(s3); c.Status != metav1.ConditionFalse {
		t.Errorf("converged rollout must not stall: got %s", c.Status)
	}

	// Advisory: distinct condition type from Ready (never folds into it).
	if computeRolloutStalledCondition(mk()).Type == InferenceReplicaConditionReady {
		t.Errorf("RolloutStalled must be a distinct condition type from Ready")
	}
}
