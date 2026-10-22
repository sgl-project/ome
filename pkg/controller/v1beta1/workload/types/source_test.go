package types

import "testing"

// The observation answers "the row at this index" once, for every
// reader: a hit aliases the observed row, a miss is nil rather than a
// zero row, and nothing about the slice order is assumed.
func TestWorkloadObservedState_InstanceAliasesTheObservedRow(t *testing.T) {
	observed := WorkloadObservedState{InstanceStatuses: []InstanceStatus{
		{Index: 3, Phase: InstancePhaseReady},
		{Index: 0, Phase: InstancePhaseCreating},
	}}
	row := observed.Instance(0)
	if row == nil || row.Phase != InstancePhaseCreating {
		t.Fatalf("Instance(0) = %+v, want the Creating row", row)
	}
	if row != &observed.InstanceStatuses[1] {
		t.Fatal("the lookup must alias the observation, not copy it")
	}
	if got := observed.Instance(7); got != nil {
		t.Fatalf("Instance(7) = %+v, want nil for an index the observation lacks", got)
	}
	var empty WorkloadObservedState
	if got := empty.Instance(0); got != nil {
		t.Fatalf("an empty observation has no rows: got %+v", got)
	}
}

// A pass indexes the rows once. A copy of the state made after that reads
// through the same index, and a lookup on a current index neither rebuilds
// it nor invents a row.
func TestWorkloadObservedState_CopiesShareTheRowIndex(t *testing.T) {
	observed := WorkloadObservedState{InstanceStatuses: []InstanceStatus{{Index: 2}, {Index: 5}}}
	observed.IndexRows()
	if observed.rowIndex == nil {
		t.Fatal("IndexRows must build the index")
	}
	built := observed.rowIndex
	observed.IndexRows()
	if observed.rowIndex != built {
		t.Fatal("IndexRows on an indexed slice must keep the index")
	}
	copied := observed
	if copied.rowIndex != built {
		t.Fatal("a copy made after IndexRows must carry the same index")
	}
	if row := copied.Instance(5); row != &observed.InstanceStatuses[1] {
		t.Fatalf("Instance(5) through the copy = %p, want the observed row %p", row, &observed.InstanceStatuses[1])
	}
	if copied.Instance(7) != nil {
		t.Fatal("Instance(7) must be nil for an index the observation lacks")
	}
}

// An index covers the slice it was built from and no other. Once the slice
// is replaced, a lookup scans the current rows until IndexRows runs again,
// and a copy that still holds the earlier slice keeps reading that slice.
func TestWorkloadObservedState_ReplacedRowsAreReadFromTheCurrentSlice(t *testing.T) {
	observed := WorkloadObservedState{InstanceStatuses: []InstanceStatus{{Index: 0}, {Index: 1}}}
	observed.IndexRows()
	earlier := observed
	observed.InstanceStatuses = append(observed.InstanceStatuses, InstanceStatus{Index: 9})
	if row := observed.Instance(9); row != &observed.InstanceStatuses[2] {
		t.Fatalf("Instance(9) after the append = %v, want the appended row", row)
	}
	observed.InstanceStatuses = observed.InstanceStatuses[1:]
	if observed.Instance(0) != nil {
		t.Fatal("Instance(0) must be nil once the row is dropped")
	}
	if row := observed.Instance(9); row != &observed.InstanceStatuses[1] {
		t.Fatalf("Instance(9) after the drop = %v, want the row at its new position", row)
	}
	if observed.rowIndex != earlier.rowIndex {
		t.Fatal("a lookup must not build an index; only IndexRows does")
	}
	observed.IndexRows()
	if observed.rowIndex == earlier.rowIndex || !observed.rowIndex.covers(observed.InstanceStatuses) {
		t.Fatal("IndexRows on a replaced slice must build an index for the current slice")
	}
	if row := observed.Instance(9); row != &observed.InstanceStatuses[1] {
		t.Fatalf("Instance(9) through the new index = %v, want the row at its position", row)
	}
	if earlier.Instance(9) != nil || earlier.Instance(0) != &earlier.InstanceStatuses[0] {
		t.Fatal("a copy holding the earlier slice reads that slice")
	}
}

// Rows swapped in place under an index are still found: the row at the
// recorded position no longer carries the index, so the lookup scans.
func TestWorkloadObservedState_ReorderedRowsAreFoundAgain(t *testing.T) {
	observed := WorkloadObservedState{InstanceStatuses: []InstanceStatus{
		{Index: 0, Phase: InstancePhaseReady},
		{Index: 1, Phase: InstancePhaseCreating},
	}}
	observed.IndexRows()
	observed.InstanceStatuses[0], observed.InstanceStatuses[1] = observed.InstanceStatuses[1], observed.InstanceStatuses[0]
	if row := observed.Instance(1); row != &observed.InstanceStatuses[0] || row.Phase != InstancePhaseCreating {
		t.Fatalf("Instance(1) after the swap = %+v, want the Creating row at position 0", row)
	}
	if row := observed.Instance(0); row != &observed.InstanceStatuses[1] || row.Phase != InstancePhaseReady {
		t.Fatalf("Instance(0) after the swap = %+v, want the Ready row at position 1", row)
	}
}
