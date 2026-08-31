package ops

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestCreateTransitionRollbackPreservesPublicationOnlyFields(t *testing.T) {
	previous := workload.InstanceStatus{
		Index: 4, Incarnation: 3, Phase: workload.InstancePhaseFailed,
		RunningRevision: "revision-a", PodCount: 2, ServingPodCount: 1,
		AvailablePodCount: 1, Admitted: true, ActiveOrdinal: 1,
	}
	committed := previous
	committed.Phase = workload.InstancePhaseCreating
	committed.Operation = &workload.InstanceOperation{ID: "create-4", Type: workload.InstanceOperationCreate}
	transition := statusTransition{index: 4, previous: &previous, current: &committed}
	mutation, ok := transition.rollbackMutation()
	if !ok || mutation.Mutate == nil || mutation.Remove {
		t.Fatalf("rollback mutation = %+v, want restoring mutation", mutation)
	}

	current := committed
	current.ReadyPodCount = 2
	current.ScheduledPodCount = 2
	observedNodes := []string{"node-a", "node-b"}
	current.NodesOccupied = observedNodes
	if !mutation.Mutate(&current) {
		t.Fatal("publication-only changes blocked lifecycle rollback")
	}
	want := previous
	want.ReadyPodCount = 2
	want.ScheduledPodCount = 2
	want.NodesOccupied = []string{"node-a", "node-b"}
	if !reflect.DeepEqual(current, want) {
		t.Fatalf("restored status:\n got: %+v\nwant: %+v", current, want)
	}
	observedNodes[0] = "mutated"
	if current.NodesOccupied[0] != "node-a" {
		t.Fatal("restored node observation aliases the committed status")
	}
}

func TestCreateTransitionRollbackIgnoresExactlyRemovableFields(t *testing.T) {
	committed := workload.InstanceStatus{
		Index: 2, Incarnation: 7, Phase: workload.InstancePhaseCreating,
		RunningRevision: "revision-a", TargetRevision: "revision-b",
		PodCount: 2, ServingPodCount: 1, AvailablePodCount: 1, Admitted: true,
		Operation:     &workload.InstanceOperation{ID: "create-2", Type: workload.InstanceOperationCreate},
		ActiveOrdinal: 1,
	}
	transition := statusTransition{index: 2, current: &committed}
	mutation, ok := transition.rollbackMutation()
	if !ok || !mutation.Remove || mutation.Precondition == nil {
		t.Fatalf("rollback mutation = %+v, want conditional removal", mutation)
	}

	removable := []struct {
		name   string
		mutate func(*workload.InstanceStatus)
	}{
		{name: "ready pods", mutate: func(status *workload.InstanceStatus) { status.ReadyPodCount++ }},
		{name: "scheduled pods", mutate: func(status *workload.InstanceStatus) { status.ScheduledPodCount++ }},
		{name: "occupied nodes", mutate: func(status *workload.InstanceStatus) { status.NodesOccupied = []string{"node-a"} }},
	}
	for _, test := range removable {
		t.Run("allows "+test.name, func(t *testing.T) {
			current := committed
			test.mutate(&current)
			if !mutation.Precondition(&current) {
				t.Fatalf("%s changed rollback ownership", test.name)
			}
		})
	}

	retained := []struct {
		name   string
		mutate func(*workload.InstanceStatus)
	}{
		{name: "index", mutate: func(status *workload.InstanceStatus) { status.Index++ }},
		{name: "incarnation", mutate: func(status *workload.InstanceStatus) { status.Incarnation++ }},
		{name: "phase", mutate: func(status *workload.InstanceStatus) { status.Phase = workload.InstancePhaseReady }},
		{name: "running revision", mutate: func(status *workload.InstanceStatus) { status.RunningRevision = "revision-c" }},
		{name: "target revision", mutate: func(status *workload.InstanceStatus) { status.TargetRevision = "revision-c" }},
		{name: "pod count", mutate: func(status *workload.InstanceStatus) { status.PodCount++ }},
		{name: "serving pods", mutate: func(status *workload.InstanceStatus) { status.ServingPodCount++ }},
		{name: "available pods", mutate: func(status *workload.InstanceStatus) { status.AvailablePodCount++ }},
		{name: "admission", mutate: func(status *workload.InstanceStatus) { status.Admitted = false }},
		{name: "conditions", mutate: func(status *workload.InstanceStatus) {
			status.Conditions = []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}
		}},
		{name: "operation", mutate: func(status *workload.InstanceStatus) { status.Operation = nil }},
		{name: "active ordinal", mutate: func(status *workload.InstanceStatus) { status.ActiveOrdinal++ }},
		{name: "last failure", mutate: func(status *workload.InstanceStatus) {
			status.LastFailure = &workload.InstanceTermination{Reason: "Error"}
		}},
	}
	for _, test := range retained {
		t.Run("rejects "+test.name, func(t *testing.T) {
			current := committed
			test.mutate(&current)
			if mutation.Precondition(&current) {
				t.Fatalf("%s did not fence lifecycle rollback", test.name)
			}
		})
	}
}
