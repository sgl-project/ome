package testfixture_test

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestMinimalInferenceService_HasAUIDAndTheEngineReplicaCount(t *testing.T) {
	isvc := fixture.MinimalInferenceService("svc", "ns", 3)
	if isvc.Name != "svc" || isvc.Namespace != "ns" || isvc.UID == "" {
		t.Fatalf("service = %+v, want svc/ns with a UID", isvc.ObjectMeta)
	}
	if isvc.Spec.Engine == nil || isvc.Spec.Engine.MinReplicas == nil || *isvc.Spec.Engine.MinReplicas != 3 {
		t.Fatalf("engine = %+v, want minReplicas 3", isvc.Spec.Engine)
	}
}

func TestInferenceReplica_IsNamedForTheComponentAndCarriesTheRows(t *testing.T) {
	isvc := fixture.MinimalInferenceService("svc", "ns", 1)
	ir := fixture.InferenceReplica(isvc, types.ComponentDecoder, v1beta1.OMENativeInstanceStatus{Index: 1})
	if ir.Name != "svc-decoder" || ir.Name != fixture.InferenceReplicaName(isvc, types.ComponentDecoder) || ir.Namespace != "ns" {
		t.Fatalf("replica %s/%s, want ns/svc-decoder", ir.Namespace, ir.Name)
	}
	if len(ir.Status.InstanceStatuses) != 1 || ir.Status.InstanceStatuses[0].Index != 1 {
		t.Fatalf("rows = %+v, want the one row given", ir.Status.InstanceStatuses)
	}
}

func TestServiceWithReadyInstance_IsReadyAtTheGivenRecreationCount(t *testing.T) {
	isvc, ir := fixture.ServiceWithReadyInstance("svc", "ns", 2)
	c := fixture.FakeClient(t, isvc, ir)
	rows := fixture.InstanceStatuses(c, isvc, types.ComponentEngine)
	if len(rows) != 1 || rows[0].Index != 0 || rows[0].Phase != types.InstancePhaseReady || rows[0].Incarnation != 2 {
		t.Fatalf("rows = %+v, want Instance 0 Ready at recreation count 2", rows)
	}
	if fixture.StoredInstanceStatuses(c, isvc, types.ComponentDecoder) != nil {
		t.Errorf("a component with no replica must read as nil rows")
	}
}

func TestMutateInstanceOnReplica_WritesTheRowAndCreatesTheReplicaOnFirstWrite(t *testing.T) {
	isvc := fixture.MinimalInferenceService("svc", "ns", 1)
	c := fixture.FakeClient(t, isvc)
	mutate := fixture.MutateInstanceOnReplica(c, isvc, types.ComponentEngine)
	ctx := context.Background()
	err := mutate(ctx, 0, func(row *types.InstanceStatus) bool {
		row.Phase = types.InstancePhaseCreating
		row.Operation = &types.InstanceOperation{ID: "create-0", Type: types.InstanceOperationCreate, SurgeIndex: new(int32)}
		return true
	})
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	rows := fixture.InstanceStatuses(c, isvc, types.ComponentEngine)
	if len(rows) != 1 || rows[0].Phase != types.InstancePhaseCreating || rows[0].Operation == nil || rows[0].Operation.SurgeIndex == nil {
		t.Fatalf("rows after the first write = %+v, want one Creating row with its operation", rows)
	}
	if err := mutate(ctx, 0, func(*types.InstanceStatus) bool { return false }); err != nil {
		t.Fatalf("a write that changes nothing: %v", err)
	}
	remove := fixture.RemoveInstanceOnReplica(c, isvc, types.ComponentEngine)
	if removed, err := remove(ctx, 0); err != nil || !removed {
		t.Fatalf("remove = %v, %v; want true", removed, err)
	}
	if removed, err := remove(ctx, 0); err != nil || removed {
		t.Fatalf("second remove = %v, %v; want false", removed, err)
	}
	if rows := fixture.InstanceStatuses(c, isvc, types.ComponentEngine); len(rows) != 0 {
		t.Fatalf("rows after the remove = %+v, want none", rows)
	}
}

func TestOperationAndTermination_SurviveTheRoundTripToTheAPI(t *testing.T) {
	surge := int32(3)
	at := metav1.Now()
	op := &types.InstanceOperation{
		ID: "op", Type: types.InstanceOperationUpdate, Step: types.UpdateStepSurge, StartedAt: at, LastProgressAt: at, Deadline: at,
		TargetRevision: "rev", Strategy: types.UpdateStrategySurgeThenDrain, Reason: "why", Waiting: "on", CapacityRefusedAt: &at,
		RequestUUID: "req", RetryCount: 2, SurgeIndex: &surge,
	}
	if got := fixture.OperationFromAPI(fixture.OperationToAPI(op)); !reflect.DeepEqual(got, op) {
		t.Errorf("operation round trip = %+v, want %+v", got, op)
	}
	code := int32(137)
	term := &types.InstanceTermination{PodName: "pod", ContainerName: "main", Reason: "OOMKilled", Message: "m", Time: at, ExitCode: &code}
	if got := fixture.TerminationFromAPI(fixture.TerminationToAPI(term)); !reflect.DeepEqual(got, term) {
		t.Errorf("termination round trip = %+v, want %+v", got, term)
	}
	if fixture.OperationFromAPI(nil) != nil || fixture.OperationToAPI(nil) != nil || fixture.TerminationFromAPI(nil) != nil || fixture.TerminationToAPI(nil) != nil {
		t.Errorf("nil must round-trip as nil")
	}
}
