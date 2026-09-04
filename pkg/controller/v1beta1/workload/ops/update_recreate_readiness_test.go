package ops

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestRecreateUpdateWaitsForPodReadyAfterEnablingServing(t *testing.T) {
	tests := []struct {
		name      string
		podReady  bool
		wantDone  bool
		wantPhase workload.InstancePhase
	}{
		{
			name:      "serving gate set but pod ready not observed",
			podReady:  false,
			wantDone:  false,
			wantPhase: workload.InstancePhaseUpdating,
		},
		{
			name:      "pod ready observed",
			podReady:  true,
			wantDone:  true,
			wantPhase: workload.InstancePhaseReady,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			legacyResetExpectations(t)
			isvc, ir := legacyISVCReadyAtIncarnation("llama-70b", "prod", 1)
			client := legacyNewFakeClient(t, isvc, ir)
			targetSpec := legacyTargetSpecImage("llama:v2")
			target := legacyEnsureTargetCR(t, client, isvc, targetSpec)

			if err := legacyMutateInstance(client, isvc, workload.ComponentEngine)(context.Background(), 0, func(status *workload.InstanceStatus) bool {
				status.Incarnation = 2
				status.Phase = workload.InstancePhaseUpdating
				status.TargetRevision = target.Name
				status.Operation = &workload.InstanceOperation{
					Type:           workload.InstanceOperationUpdate,
					Step:           updateStepDrain,
					TargetRevision: target.Name,
				}
				return true
			}); err != nil {
				t.Fatalf("seed in-flight recreate: %v", err)
			}

			pod := legacyPodAtIncarnation(isvc, 0, 2, true, true)
			pod.Spec = *targetSpec.DeepCopy()
			if test.podReady {
				pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				})
			}
			if err := client.Create(context.Background(), pod); err != nil {
				t.Fatalf("create replacement pod: %v", err)
			}

			input := legacyTestInput(isvc, client, workload.ComponentEngine)
			plan := legacyComponentPlan(workload.UpdateStrategyRecreatePod, nil)
			done, err := recreateUpdate(
				context.Background(),
				legacyTestDeps(client),
				input,
				plan,
				plan.Instances[0],
				target,
				[]*corev1.Pod{pod},
			)
			if err != nil {
				t.Fatalf("recreateUpdate: %v", err)
			}
			if done != test.wantDone {
				t.Fatalf("done: got %t, want %t", done, test.wantDone)
			}

			statuses := legacyInstanceStatusesOnIR(client, isvc, workload.ComponentEngine)
			if len(statuses) != 1 {
				t.Fatalf("instance statuses: got %d, want 1", len(statuses))
			}
			if got := workload.InstancePhase(statuses[0].Phase); got != test.wantPhase {
				t.Fatalf("phase: got %q, want %q", got, test.wantPhase)
			}
		})
	}
}
