package testfixture_test

import (
	"testing"

	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestRowWithOperation_HasAnUpdateOpenInTheGivenPhase(t *testing.T) {
	row := fixture.RowWithOperation(3, types.InstancePhaseUpdating)
	if row.Index != 3 || row.Phase != types.InstancePhaseUpdating {
		t.Fatalf("row = %+v, want index 3 Updating", row)
	}
	if row.Operation == nil || row.Operation.Type != types.InstanceOperationUpdate || row.Operation.ID == "" {
		t.Fatalf("operation = %+v, want an update with an id", row.Operation)
	}
}

func TestUpdatingRowAtSurge_SetsTheRevisionsCountsAndEveryOperationField(t *testing.T) {
	row := fixture.UpdatingRowAtSurge(2)
	if row.Index != 2 || row.Phase != types.InstancePhaseUpdating || row.RunningRevision == "" || row.TargetRevision == "" ||
		row.Incarnation == 0 || row.PodCount == 0 || row.ReadyPodCount == 0 || row.ActiveOrdinal == 0 {
		t.Fatalf("row = %+v, want index 2 Updating with both revisions and the counts set", row)
	}
	op := row.Operation
	if op == nil || op.Step != types.UpdateStepSurge || op.SurgeIndex == nil || len(op.HintTargetNodes) == 0 ||
		op.StartedAt.IsZero() || op.LastProgressAt.IsZero() || op.Deadline.IsZero() ||
		op.ID == "" || op.RequestUUID == "" || op.TargetRevision == "" || op.Reason == "" || op.FromNode == "" || op.RetryCount == 0 {
		t.Fatalf("operation = %+v, want every field set at the surge step", op)
	}
	if other := fixture.UpdatingRowAtSurge(2); other.Operation.SurgeIndex == row.Operation.SurgeIndex {
		t.Errorf("two rows share one SurgeIndex pointer; each call must build its own")
	}
}
