package ops

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// TestRecreateUpdate_WaitsForPodReadyAfterEnablingServing: with the old pods
// gone and the recreated pod ContainersReady + serving, promotion still waits
// for kubelet to fold the serving gate into PodReady. Promotion releases the
// Instance's unavailability budget slot, so promoting before the pod is
// routable would let the next Instance drain first.
func TestRecreateUpdate_WaitsForPodReadyAfterEnablingServing(t *testing.T) {
	tests := []struct {
		name      string
		podReady  bool
		wantDone  bool
		wantPhase v1beta1.OMENativeInstancePhase
	}{
		{name: "serving gate set but PodReady not observed", podReady: false, wantDone: false, wantPhase: v1beta1.OMENativeInstanceUpdating},
		{name: "PodReady observed", podReady: true, wantDone: true, wantPhase: v1beta1.OMENativeInstanceReady},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			legacyResetExpectations(t)
			isvc, ir := legacyISVCReadyAtIncarnation("llama-70b", "prod", 1)
			c := legacyNewFakeClient(t, isvc, ir)
			targetSpec := legacyTargetSpecImage("llama:v2")
			target := legacyEnsureTargetCR(t, c, isvc, targetSpec)

			// Mid-recreate state: Phase A drained and deleted the incarnation-1
			// pod, Phase B created the incarnation-2 pod; only Phase C is left.
			if err := legacyMutateInstance(c, isvc, workload.ComponentEngine)(context.Background(), 0, func(s *workload.InstanceStatus) bool {
				s.Incarnation = 2
				s.Phase = workload.InstancePhaseUpdating
				s.TargetRevision = target.Name
				s.Operation = &workload.InstanceOperation{Type: workload.InstanceOperationUpdate, Step: updateStepDrain, TargetRevision: target.Name}
				return true
			}); err != nil {
				t.Fatalf("seed in-flight recreate: %v", err)
			}

			pod := legacyPodAtIncarnation(isvc, 0, 2, true /* ContainersReady */, true /* serving */)
			pod.Spec = *targetSpec.DeepCopy()
			if tc.podReady {
				pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{Type: corev1.PodReady, Status: corev1.ConditionTrue})
			}
			if err := c.Create(context.Background(), pod); err != nil {
				t.Fatalf("create replacement pod: %v", err)
			}

			input := legacyTestInput(isvc, c, workload.ComponentEngine)
			plan := legacyComponentPlan(workload.UpdateStrategyRecreatePod, nil)
			done, err := recreateUpdate(context.Background(), legacyTestDeps(c), input, plan, plan.Instances[0], target, []*corev1.Pod{pod})
			if err != nil {
				t.Fatalf("recreateUpdate: %v", err)
			}
			if done != tc.wantDone {
				t.Fatalf("done: got %t, want %t", done, tc.wantDone)
			}
			statuses := legacyInstanceStatusesOnIR(c, isvc, workload.ComponentEngine)
			if len(statuses) != 1 {
				t.Fatalf("instance statuses: got %d, want 1", len(statuses))
			}
			if statuses[0].Phase != tc.wantPhase {
				t.Fatalf("phase: got %q, want %q", statuses[0].Phase, tc.wantPhase)
			}
		})
	}
}
