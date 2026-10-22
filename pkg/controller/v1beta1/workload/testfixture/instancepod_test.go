package testfixture_test

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

func TestPodForInstance_CarriesTheRenderedNameAndLabels(t *testing.T) {
	isvc := fixture.MinimalInferenceService("svc", "ns", 1)
	pod := fixture.PodForInstance(isvc, 2, false, false)
	if want := query.PodName("svc", types.ComponentEngine, 2, types.RunnerDefault, 0); pod.Name != want || pod.Namespace != "ns" {
		t.Fatalf("pod %s/%s, want ns/%s", pod.Namespace, pod.Name, want)
	}
	want := fixture.PodLabels("svc", types.ComponentEngine, 2, types.RunnerDefault, 1, 0)
	want[query.LabelRevisionHash] = fixture.InstanceRevisionHash
	for k, v := range want {
		if pod.Labels[k] != v {
			t.Errorf("label %s = %q, want %q", k, pod.Labels[k], v)
		}
	}
	if len(pod.Labels) != len(want) {
		t.Errorf("labels = %v, want exactly %v", pod.Labels, want)
	}
	if pod.Labels[constants.OMEComponentLabel] != string(types.ComponentEngine) || pod.Labels[query.LabelManagedBy] != query.ManagedByOMENative {
		t.Errorf("labels = %v, want the engine component managed by OMENative", pod.Labels)
	}
	if len(pod.Status.Conditions) != 0 {
		t.Errorf("conditions = %+v, want none for a pod neither ready nor serving", pod.Status.Conditions)
	}
}

func TestPodForInstance_ReadyAndServingAddTheirConditions(t *testing.T) {
	isvc := fixture.MinimalInferenceService("svc", "ns", 1)
	pod := fixture.PodForInstance(isvc, 0, true, true)
	if c := condition(pod, corev1.ContainersReady); c == nil || c.Status != corev1.ConditionTrue {
		t.Errorf("ContainersReady = %+v, want True", c)
	}
	if c := condition(pod, query.ServingConditionType); c == nil || c.Status != corev1.ConditionTrue {
		t.Errorf("serving gate = %+v, want True", c)
	}
	if c := condition(pod, corev1.PodReady); c != nil {
		t.Errorf("PodReady = %+v, want none until the kubelet folds the gate in", c)
	}
	labels := fixture.PodAtRecreation(isvc, 0, 4, false, false).Labels
	for k, v := range fixture.PodLabels("svc", types.ComponentEngine, 0, types.RunnerDefault, 4, 0) {
		if labels[k] != v {
			t.Errorf("label %s = %q at recreation count 4, want %q", k, labels[k], v)
		}
	}
}

func TestServingPodWithImage_IsReadyServingAndPodReadyOnTheImage(t *testing.T) {
	isvc := fixture.MinimalInferenceService("svc", "ns", 1)
	pod := fixture.ServingPodWithImage(isvc, 0, 2, "model:v2")
	if pod.Spec.Containers[0].Image != "model:v2" || pod.Spec.Containers[0].Name != fixture.ContainerName {
		t.Fatalf("container = %+v, want %s on model:v2", pod.Spec.Containers[0], fixture.ContainerName)
	}
	for _, typ := range []corev1.PodConditionType{corev1.ContainersReady, query.ServingConditionType, corev1.PodReady} {
		if c := condition(pod, typ); c == nil || c.Status != corev1.ConditionTrue {
			t.Errorf("condition %s = %+v, want True", typ, c)
		}
	}
	if spec := fixture.PodSpecWithImage("model:v3"); len(spec.Containers) != 1 || spec.Containers[0].Image != "model:v3" {
		t.Errorf("PodSpecWithImage = %+v, want one container on model:v3", spec)
	}
}

func TestMarkPodReadyAt_WritesPodReadyOnTheStoredPod(t *testing.T) {
	isvc := fixture.MinimalInferenceService("svc", "ns", 1)
	pod := fixture.PodForInstance(isvc, 0, true, true)
	c := fixture.FakeClient(t, pod)
	at := fixture.Epoch.Add(-time.Minute)
	fixture.MarkPodReadyAt(t, c, pod, at)
	stored := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), stored); err != nil {
		t.Fatalf("get: %v", err)
	}
	if ready := condition(stored, corev1.PodReady); ready == nil || ready.Status != corev1.ConditionTrue || !ready.LastTransitionTime.Time.Equal(at) {
		t.Fatalf("PodReady = %+v, want True since %s", ready, at)
	}
}

func TestLabelPodWithRevision_PutsTheRevisionsHashOnTheStoredPod(t *testing.T) {
	isvc := fixture.MinimalInferenceService("svc", "ns", 1)
	pod := fixture.PodForInstance(isvc, 0, true, true)
	c := fixture.FakeClient(t, isvc, pod)
	cr := fixture.EnsureTargetRevision(t, c, isvc, fixture.PodSpecWithImage("model:v2"))
	fixture.LabelPodWithRevision(t, c, pod, cr.Name)
	stored := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), stored); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got, want := stored.Labels[query.LabelRevisionHash], query.RevisionHashFromControllerRevisionName(cr.Name); got != want {
		t.Fatalf("revision label = %q, want %q", got, want)
	}
}

func TestEndpointSliceForPod_TargetsThePodWithTheGivenReadiness(t *testing.T) {
	pod := fixture.ServingPod("a")
	pod.Namespace = "ns"
	slice := fixture.EndpointSliceForPod("ns", "svc-abc", "svc", pod, false)
	if slice.Labels["kubernetes.io/service-name"] != "svc" || len(slice.Endpoints) != 1 {
		t.Fatalf("slice = %+v, want one endpoint of Service svc", slice)
	}
	ep := slice.Endpoints[0]
	if ep.TargetRef == nil || ep.TargetRef.Name != "a" || ep.TargetRef.Namespace != "ns" || ep.Conditions.Ready == nil || *ep.Conditions.Ready {
		t.Fatalf("endpoint = %+v, want pod ns/a not ready", ep)
	}
}
