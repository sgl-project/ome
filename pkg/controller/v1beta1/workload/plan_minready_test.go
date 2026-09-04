package workload

import "testing"

func TestBuildPlan_CarriesMinReadySeconds(t *testing.T) {
	desired := singlePodDesired(1, Lifecycle{})
	desired.MinReadySeconds = 20
	plan, err := BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.MinReadySeconds != 20 {
		t.Fatalf("MinReadySeconds: got %d want 20", plan.MinReadySeconds)
	}

	// Unset projects to 0 (Available as soon as Ready); a negative value
	// from a hand-edited source cannot widen the window below zero.
	desired.MinReadySeconds = -7
	plan, err = BuildPlan(ComponentEngine, desired, WorkloadObservedState{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.MinReadySeconds != 0 {
		t.Fatalf("MinReadySeconds: got %d want 0", plan.MinReadySeconds)
	}
}
