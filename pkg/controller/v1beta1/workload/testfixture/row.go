package testfixture

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// RowWithOperation is an Instance row in the given phase with an update
// operation open on it.
func RowWithOperation(index int32, phase types.InstancePhase) types.InstanceStatus {
	return types.InstanceStatus{
		Index: index, Phase: phase,
		Operation: &types.InstanceOperation{ID: "op-1", Type: types.InstanceOperationUpdate},
	}
}

// UpdatingRowAtSurge is an Updating row of a gang update at its surge step:
// the revisions, the counts and every field of the operation are set, so a
// test that copies, writes or decodes a row sees a field that went missing.
func UpdatingRowAtSurge(index int32) types.InstanceStatus {
	surge := int32(9)
	started := metav1.NewTime(time.Date(2026, time.August, 15, 10, 0, 0, 987654321, time.UTC))
	return types.InstanceStatus{
		Index:           index,
		Incarnation:     7,
		Phase:           types.InstancePhaseUpdating,
		RunningRevision: "rev-old",
		TargetRevision:  "rev-new",
		ActiveOrdinal:   1,
		PodCount:        8,
		ReadyPodCount:   7,
		Operation: &types.InstanceOperation{
			ID:              "gang-update-7",
			Type:            types.InstanceOperationUpdate,
			Step:            types.UpdateStepSurge,
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
