package inferencereplica

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/v1beta1convert"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload"
)

// TestReadyCondition_NoReplicas_ScaledToZero pins the scaled-to-zero
// branch: Replicas == 0 with no rollout in flight must read
// Ready=False/NoReplicas. The zero-replica check sits BEFORE the
// ReadyReplicas == Replicas comparison in the precedence chain — without
// that ordering, 0 == 0 would satisfy the AllInstancesReady branch and a
// Component with zero desired Instances would read Ready=True.
func TestReadyCondition_NoReplicas_ScaledToZero(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:        0,
		ReadyReplicas:   0,
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-a",
	}
	cond := computeReadyCondition(status, nil, nil, nil)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionFalse),
		"zero desired Instances must NOT read Ready=True via the 0==0 AllInstancesReady comparison")
	g.Expect(cond.Reason).To(gomega.Equal(ReasonNoReplicas))
}

// TestReadyCondition_RolloutOutranksNoReplicas pins the documented
// precedence between the rollout-in-flight branch and the zero-replica
// branch: a Component mid-rollout with a momentary zero Replicas counter
// reads Unknown/RolloutInProgress ("churning, check back"), not the
// steady-state False/NoReplicas signal.
func TestReadyCondition_RolloutOutranksNoReplicas(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:        0,
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-b",
	}
	cond := computeReadyCondition(status, nil, nil, nil)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionUnknown))
	g.Expect(cond.Reason).To(gomega.Equal(ReasonRolloutInProgress),
		"an in-flight rollout outranks the zero-replica branch in the precedence chain")
}

// TestReadyCondition_FailedOutranksNoReplicas pins that the
// InstanceFailed override is the highest-priority branch even against a
// zero Replicas counter: a lingering Failed Instance row must surface as
// False/InstanceFailed, not the softer False/NoReplicas.
func TestReadyCondition_FailedOutranksNoReplicas(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:        0,
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-a",
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, Phase: v1beta1.OMENativeInstanceFailed},
		},
	}
	cond := computeReadyCondition(status, nil, nil, nil)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(cond.Reason).To(gomega.Equal(ReasonInstanceFailed),
		"a Failed Instance is the most specific signal and must outrank NoReplicas")
}

// TestReadyCondition_ZeroPartitionIsNotStaged pins that an explicit
// partition of zero means "full rollout", never Staged: a shape fully
// converged onto the target revision but not yet promoted
// (CurrentRevision != UpdateRevision) reads Unknown/RolloutInProgress.
// Staged is reserved for a non-zero partition intentionally holding
// Instances on the prior revision.
func TestReadyCondition_ZeroPartitionIsNotStaged(t *testing.T) {
	g := gomega.NewWithT(t)
	part := int32(0)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:             2,
		ReadyReplicas:        2,
		UpdatedReadyReplicas: 2,
		CurrentRevision:      "rev-a",
		UpdateRevision:       "rev-b",
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: "rev-b"},
			{Index: 1, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: "rev-b"},
		},
	}
	cond := computeReadyCondition(status, nil, nil, &v1beta1.InferenceReplicaPacing{Partition: &part})
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionUnknown))
	g.Expect(cond.Reason).To(gomega.Equal(ReasonRolloutInProgress),
		"partition=0 is a full rollout: convergence completes via promotion, not the Staged branch")
}

// TestRolloutStalledCondition_NoRolloutEver pins the empty-UpdateRevision
// branch: before any rollout has ever targeted a revision the condition
// is False/Progressing with the no-rollout message, even when an
// Instance carries a recorded failure.
func TestRolloutStalledCondition_NoRolloutEver(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas: 1,
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, LastFailure: &v1beta1.InstanceTermination{Reason: "CrashLoopBackOff"}},
		},
	}
	cond := computeRolloutStalledCondition(status)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(cond.Reason).To(gomega.Equal(ReasonRolloutProgressing))
	g.Expect(cond.Message).To(gomega.Equal("no rollout in flight"))
}

// TestRolloutStalledCondition_ObservedGenerationStamped pins that the
// advisory condition carries ObservedGeneration off
// status.ObservedGeneration, matching the Ready condition's contract so
// consumers can correlate both conditions to a spec generation.
func TestRolloutStalledCondition_ObservedGenerationStamped(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		ObservedGeneration: 9,
		Replicas:           1,
		CurrentRevision:    "rev-a",
		UpdateRevision:     "rev-b",
	}
	cond := computeRolloutStalledCondition(status)
	g.Expect(cond.ObservedGeneration).To(gomega.Equal(int64(9)))
}

// TestRolloutStalledCondition_MultiReasonSummarySorted pins the message
// contract for a stall with heterogeneous failures: reasons are counted,
// rendered as "Reason xN", and sorted by reason so the message is
// byte-stable across reconciles (an unstable message would defeat the
// no-op write short-circuit and churn status every pass).
func TestRolloutStalledCondition_MultiReasonSummarySorted(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:        4,
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-b",
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, RunningRevision: "rev-a", LastFailure: &v1beta1.InstanceTermination{Reason: "ErrImagePull"}},
			{Index: 1, RunningRevision: "rev-a", LastFailure: &v1beta1.InstanceTermination{Reason: "CrashLoopBackOff"}},
			{Index: 2, RunningRevision: "rev-a", LastFailure: &v1beta1.InstanceTermination{Reason: "CrashLoopBackOff"}},
			// Already on target: its stale failure must not count.
			{Index: 3, RunningRevision: "rev-b", LastFailure: &v1beta1.InstanceTermination{Reason: "ErrImagePull"}},
		},
	}
	cond := computeRolloutStalledCondition(status)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(cond.Reason).To(gomega.Equal(ReasonInstancesFailing))
	g.Expect(cond.Message).To(gomega.Equal(
		"3/4 Instance(s) failing rollout to rev-b (CrashLoopBackOff x2, ErrImagePull x1)"))
	g.Expect(computeRolloutStalledCondition(status).Message).To(gomega.Equal(cond.Message),
		"the failure summary must be byte-stable across recomputations")
}

// TestRolloutStalledCondition_EmptyReasonFallback pins the summary
// fallback: a terminal failure recorded without a reason string still
// renders a meaningful message instead of an empty parenthetical.
func TestRolloutStalledCondition_EmptyReasonFallback(t *testing.T) {
	g := gomega.NewWithT(t)
	status := &v1beta1.InferenceReplicaStatus{
		Replicas:        1,
		CurrentRevision: "rev-a",
		UpdateRevision:  "rev-b",
		InstanceStatuses: []v1beta1.OMENativeInstanceStatus{
			{Index: 0, RunningRevision: "rev-a", LastFailure: &v1beta1.InstanceTermination{}},
		},
	}
	cond := computeRolloutStalledCondition(status)
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(cond.Message).To(gomega.ContainSubstring("terminal failure"))
}

// TestAggregateAndWriteStatus_ObservedGenerationFollowsGeneration pins
// the end-to-end ObservedGeneration contract through the aggregator: the
// persisted status stamps ObservedGeneration off metadata.generation, and
// BOTH persisted conditions (Ready + RolloutStalled) carry the same
// value, so `kubectl wait` style consumers can tell whether a condition
// reflects the spec they just wrote.
func TestAggregateAndWriteStatus_ObservedGenerationFollowsGeneration(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("status-c-engine", "status-c", 1)
	ir.Generation = 4
	r, c := newReconciler(t, ir)
	plan := workload.ComponentPlan{
		Component: v1beta1convert.ComponentTypeToWorkload(ir.Spec.Component),
		Replicas:  1,
		Instances: []workload.InstancePlan{{
			Index: 0, Incarnation: 1, Runners: []workload.RunnerPlan{{Name: "default", Size: 1}},
		}},
	}

	g.Expect(r.aggregateAndWriteStatus(context.Background(), ir, plan, nil, false, nil)).To(gomega.Succeed())

	got := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: ir.Name, Namespace: ir.Namespace}, got)).To(gomega.Succeed())
	g.Expect(got.Status.ObservedGeneration).To(gomega.Equal(int64(4)),
		"status.observedGeneration must track metadata.generation after aggregation")

	ready := apimeta.FindStatusCondition(got.Status.Conditions, InferenceReplicaConditionReady)
	g.Expect(ready).NotTo(gomega.BeNil())
	g.Expect(ready.ObservedGeneration).To(gomega.Equal(int64(4)))
	// No Instance rows and no live pods: the aggregator publishes zero
	// counters, so the scaled-to-zero contract applies end to end.
	g.Expect(ready.Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(ready.Reason).To(gomega.Equal(ReasonNoReplicas))

	stalled := apimeta.FindStatusCondition(got.Status.Conditions, InferenceReplicaConditionRolloutStalled)
	g.Expect(stalled).NotTo(gomega.BeNil())
	g.Expect(stalled.ObservedGeneration).To(gomega.Equal(int64(4)))

	g.Expect(ir.Status.ObservedGeneration).To(gomega.Equal(int64(4)),
		"the caller's in-memory IR must mirror the committed ObservedGeneration")
}

// TestPublicationCounters_DeriveFromPodsNotPhases pins the Component
// counter contract: Replicas/Ready/Serving/Available derive from the
// live pod observation, never from the persisted per-Instance lifecycle
// Phase, while Updated/UpdatedReady additionally key off the durable
// RunningRevision row matching the target revision. A Phase that lags or
// contradicts the pod state (Failed row with a healthy pod, Ready row
// with an unready pod) must not skew any counter.
func TestPublicationCounters_DeriveFromPodsNotPhases(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := baselineIR("status-c-engine", "status-c", 3)
	// Phase deliberately contradicts the live pod state on every row.
	insts := []v1beta1.OMENativeInstanceStatus{
		// Row says Failed; pod is ready+serving+available, on target.
		{Index: 0, Phase: v1beta1.OMENativeInstanceFailed, RunningRevision: "rev-b"},
		// Row says Ready; pod exists but is not ContainersReady. On target.
		{Index: 1, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: "rev-b"},
		// Row says Ready; pod ready+serving but held on the prior revision.
		{Index: 2, Phase: v1beta1.OMENativeInstanceReady, RunningRevision: "rev-a"},
	}
	pod0 := podForIR(ir, 0, "default", 0, true, true)
	pod1 := podForIR(ir, 1, "default", 0, false, false)
	pod2 := podForIR(ir, 2, "default", 0, true, true)

	observation, err := workload.NewOwnedPublicationObservation(
		v1beta1convert.InstanceStatusSliceToWorkload(insts),
		workload.NewCachedSelectorPodObservation(nil, map[int32][]*corev1.Pod{
			0: {pod0},
			1: {pod1},
			2: {pod2},
		}),
		map[string]struct{}{pod0.Name: {}},
	)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	desired := map[int32]int32{0: 1, 1: 1, 2: 1}
	_, counters, err := observation.TakeInlineV1Publication(desired, "rev-b")
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Expect(counters.Replicas).To(gomega.Equal(int32(3)),
		"Replicas counts every published row regardless of Phase")
	g.Expect(counters.ReadyReplicas).To(gomega.Equal(int32(2)),
		"ReadyReplicas follows ContainersReady pods, ignoring the Failed lifecycle row")
	g.Expect(counters.ServingReplicas).To(gomega.Equal(int32(2)),
		"ServingReplicas follows the serving gate on live pods")
	g.Expect(counters.AvailableReplicas).To(gomega.Equal(int32(1)),
		"AvailableReplicas follows EndpointSlice membership only")
	g.Expect(counters.UpdatedReplicas).To(gomega.Equal(int32(2)),
		"UpdatedReplicas counts rows whose RunningRevision matches the target")
	g.Expect(counters.UpdatedReadyReplicas).To(gomega.Equal(int32(1)),
		"UpdatedReadyReplicas requires BOTH the target revision and a Ready pod")
}
