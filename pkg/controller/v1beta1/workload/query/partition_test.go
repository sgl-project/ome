package query

import (
	"reflect"
	"testing"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// TestPartitionHeldIndices pins the one reading of which Instances a
// partition holds: the lowest-indexed Instances off the target that no
// update attempt is moving, counted to the partition, whatever order the
// rows arrive in; an unobserved Instance, an Instance on the target, an
// attempt in flight or its Failed continuation and a transient migration
// surge are never held, and a parked attempt is held like the Failed row
// it stands for.
func TestPartitionHeldIndices(t *testing.T) {
	const target, old = "owner-engine-target", "owner-engine-old"
	partition := func(v int32) *int32 { return &v }
	ready := func(index int32, revision string) *workload.InstanceStatus {
		return &workload.InstanceStatus{Index: index, Phase: workload.InstancePhaseReady, RunningRevision: revision}
	}
	update := func(index int32, phase workload.InstancePhase, step string) *workload.InstanceStatus {
		return &workload.InstanceStatus{Index: index, Phase: phase, RunningRevision: old,
			Operation: &workload.InstanceOperation{Type: workload.InstanceOperationUpdate, Step: step, TargetRevision: target}}
	}
	cases := []struct {
		name      string
		partition *int32
		observed  []*workload.InstanceStatus
		want      map[int32]bool
	}{
		{name: "no partition holds nothing", partition: nil, observed: []*workload.InstanceStatus{ready(0, old), ready(1, old)}, want: map[int32]bool{}},
		{name: "a partition of zero holds nothing", partition: partition(0), observed: []*workload.InstanceStatus{ready(0, old)}, want: map[int32]bool{}},
		{name: "the lowest-indexed old Instances are held, whatever the row order", partition: partition(2),
			observed: []*workload.InstanceStatus{ready(7, old), ready(2, old), ready(5, old)}, want: map[int32]bool{2: true, 5: true}},
		{name: "an Instance on the target is past holding and an unobserved one has nothing to hold", partition: partition(2),
			observed: []*workload.InstanceStatus{ready(0, target), nil, ready(2, old), ready(3, old)}, want: map[int32]bool{2: true, 3: true}},
		{name: "an attempt in flight must finish, so the hold falls to the next old Instance", partition: partition(1),
			observed: []*workload.InstanceStatus{update(0, workload.InstancePhaseUpdating, workload.UpdateStepSurge), ready(1, old)}, want: map[int32]bool{1: true}},
		{name: "a Failed continuation of an attempt is never held; a parked attempt is", partition: partition(2),
			observed: []*workload.InstanceStatus{update(0, workload.InstancePhaseFailed, workload.UpdateStepDrain), update(1, workload.InstancePhaseFailed, workload.UpdateStepParked), ready(2, old), ready(3, old)},
			want:     map[int32]bool{1: true, 2: true}},
		{name: "a transient migration surge is never held", partition: partition(1),
			observed: []*workload.InstanceStatus{
				{Index: 0, Phase: workload.InstancePhaseCreating, Operation: &workload.InstanceOperation{Type: workload.InstanceOperationMigrate}},
				ready(1, old),
			}, want: map[int32]bool{1: true}},
		{name: "a partition past the old count holds every old Instance", partition: partition(5),
			observed: []*workload.InstanceStatus{ready(0, target), ready(1, old), ready(2, old)}, want: map[int32]bool{1: true, 2: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PartitionHeldIndices(tc.partition, tc.observed, target); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("held %v, want %v", got, tc.want)
			}
		})
	}
}

// TestHeldByPartition pins the count predicate: a candidate is held when
// its rank in the ascending candidate order is below a positive partition.
func TestHeldByPartition(t *testing.T) {
	two := int32(2)
	zero := int32(0)
	for _, tc := range []struct {
		partition *int32
		rank      int32
		held      bool
	}{{nil, 0, false}, {&zero, 0, false}, {&two, 0, true}, {&two, 1, true}, {&two, 2, false}} {
		if got := HeldByPartition(tc.partition, tc.rank); got != tc.held {
			t.Errorf("HeldByPartition(%v, %d) = %v, want %v", tc.partition, tc.rank, got, tc.held)
		}
	}
}
