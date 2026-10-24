package reading_test

// The rotation helpers: the rotation set built from EndpointSlices, and the
// routed Services an Instance's pods belong to.

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/reading"
	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestRotationFromEndpointSlices(t *testing.T) {
	yes, no := true, false
	slices := []discoveryv1.EndpointSlice{
		{Endpoints: []discoveryv1.Endpoint{
			{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "ready"}, Conditions: discoveryv1.EndpointConditions{Ready: &yes}},
			{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "ready-nil"}},
			{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "not-ready"}, Conditions: discoveryv1.EndpointConditions{Ready: &no}},
			{TargetRef: &corev1.ObjectReference{Kind: "Pod", Name: "terminating"}, Conditions: discoveryv1.EndpointConditions{Ready: &yes, Terminating: &yes}},
			{TargetRef: &corev1.ObjectReference{Kind: "Node", Name: "node"}, Conditions: discoveryv1.EndpointConditions{Ready: &yes}},
			{Conditions: discoveryv1.EndpointConditions{Ready: &yes}},
		}},
		{Endpoints: []discoveryv1.Endpoint{
			{TargetRef: &corev1.ObjectReference{Name: "second-slice"}, Conditions: discoveryv1.EndpointConditions{Ready: &yes}},
		}},
	}
	got := reading.RotationFromEndpointSlices(slices)
	want := reading.Rotation{"ready": {}, "ready-nil": {}, "second-slice": {}}
	if len(got) != len(want) {
		t.Fatalf("Rotation = %v, want %v", got, want)
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			t.Fatalf("Rotation = %v, want %v", got, want)
		}
	}
	if len(reading.RotationFromEndpointSlices(nil)) != 0 {
		t.Fatalf("no slices should give an empty rotation")
	}
}

func TestRoutedServices(t *testing.T) {
	got := reading.RoutedServices("svc", types.ComponentType("engine"), fixture.Pods(
		fixture.Pod("a", fixture.OnRevision(fixture.RevisionHashB)),
		fixture.Pod("b", fixture.OnRevision(fixture.RevisionHashA)),
		fixture.Pod("c", fixture.OnRevision(fixture.RevisionHashA)),
		fixture.Pod("w", fixture.AsWorker, fixture.OnRevision(fixture.RevisionHashA)),
		fixture.Pod("n", fixture.NoRevision),
		nil,
	))
	want := []string{
		query.PerRevisionServiceName("svc", "engine", fixture.RevisionHashA),
		query.PerRevisionServiceName("svc", "engine", fixture.RevisionHashB),
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("RoutedServices = %v, want %v", got, want)
	}
}
