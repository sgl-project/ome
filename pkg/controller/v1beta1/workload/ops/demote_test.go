package ops_test

import (
	"context"
	"testing"

	ops "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/ops"
	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// TestDemoteUnbackedInstances pins the truth-pass mutation: Ready with no
// Operation demotes to Pending preserving everything else; a row an op
// claimed since selection is left untouched (the fresh-row guard).
func TestDemoteUnbackedInstances(t *testing.T) {
	rows := map[int32]*workload.InstanceStatus{
		0: {Index: 0, Incarnation: 3, Phase: workload.InstancePhaseReady, RunningRevision: "rev-a"},
		1: {Index: 1, Incarnation: 2, Phase: workload.InstancePhaseReady,
			Operation: &workload.InstanceOperation{ID: "u-1", Type: workload.InstanceOperationUpdate}},
	}
	input := workload.ReconcileInput{
		MutateInstance: func(_ context.Context, idx int32, mutate func(*workload.InstanceStatus) bool) error {
			mutate(rows[idx])
			return nil
		},
	}
	selections := []workload.DemotionSelection{
		{Index: 0, Reason: "no live pods back the Ready phase"},
		{Index: 1, Reason: "stale selection: an op claimed the row"},
	}
	if err := ops.DemoteUnbackedInstances(context.Background(), workload.Deps{}, input, workload.ComponentPlan{Component: workload.ComponentEngine}, selections); err != nil {
		t.Fatalf("DemoteUnbackedInstances: %v", err)
	}
	if rows[0].Phase != workload.InstancePhasePending {
		t.Errorf("row 0 phase = %s, want Pending", rows[0].Phase)
	}
	if rows[0].Incarnation != 3 || rows[0].RunningRevision != "rev-a" {
		t.Errorf("row 0 must keep Incarnation and RunningRevision: %+v", rows[0])
	}
	if rows[1].Phase != workload.InstancePhaseReady {
		t.Errorf("row 1 phase = %s, want Ready (fresh-row guard must no-op)", rows[1].Phase)
	}
}
