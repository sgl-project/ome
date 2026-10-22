package testfixture

import (
	"context"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// InstanceRevisionHash is the revision hash PodForInstance puts on its pod.
const InstanceRevisionHash = "testrev1"

// PodLabels is the label set the renderer puts on every pod of an Instance:
// the owner, the component, the Instance index, the Instance's recreation
// count (its `.Incarnation`), the runner, the ordinal and the manager.
func PodLabels(isvc string, component types.ComponentType, index int32, runner string, recreation int64, ordinal int32) map[string]string {
	return map[string]string{
		constants.InferenceServicePodLabelKey: isvc,
		constants.OMEComponentLabel:           string(component),
		query.LabelInstanceIdx:                fmt.Sprintf("%d", index),
		query.LabelInstanceIncarnation:        fmt.Sprintf("%d", recreation),
		query.LabelRunner:                     runner,
		query.LabelManagedBy:                  query.ManagedByOMENative,
		query.LabelPodOrdinal:                 fmt.Sprintf("%d", ordinal),
	}
}

// PodForInstance is the pod the renderer would build for the engine
// Instance of the given index at its first recreation count: one runner
// container, the Instance's labels and InstanceRevisionHash. ready adds
// ContainersReady True and serving adds the serving gate True, both at the
// wall clock.
func PodForInstance(isvc *v1beta1.InferenceService, index int32, ready, serving bool) *corev1.Pod {
	return PodAtRecreation(isvc, index, 1, ready, serving)
}

// PodAtRecreation is PodForInstance for the given recreation count.
func PodAtRecreation(isvc *v1beta1.InferenceService, index int32, recreation int64, ready, serving bool) *corev1.Pod {
	labels := PodLabels(isvc.Name, types.ComponentEngine, index, types.RunnerDefault, recreation, 0)
	labels[query.LabelRevisionHash] = InstanceRevisionHash
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      query.PodName(isvc.Name, types.ComponentEngine, index, types.RunnerDefault, 0),
			Namespace: isvc.Namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: ContainerName, Image: "test:v1"}},
		},
	}
	now := metav1.NewTime(time.Now())
	if ready {
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:               corev1.ContainersReady,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: now,
		})
	}
	if serving {
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type:               query.ServingConditionType,
			Status:             corev1.ConditionTrue,
			LastTransitionTime: now,
		})
	}
	return pod
}

// ServingPodWithImage is a pod the engine reads as serving at the given
// recreation count: ready, serving, PodReady folded in by the kubelet at
// the wall clock, running the given image.
func ServingPodWithImage(isvc *v1beta1.InferenceService, index int32, recreation int64, image string) *corev1.Pod {
	pod := PodAtRecreation(isvc, index, recreation, true, true)
	pod.Spec.Containers = []corev1.Container{{Name: ContainerName, Image: image}}
	pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
		Type:               corev1.PodReady,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	})
	return pod
}

// PodSpecWithImage is the one-container pod spec a roll targets.
func PodSpecWithImage(image string) *corev1.PodSpec {
	return &corev1.PodSpec{
		Containers: []corev1.Container{{Name: ContainerName, Image: image}},
	}
}

// MarkPodReadyAt is the kubelet step a unit fixture lacks: once the serving
// gate is satisfied, the kubelet folds it into PodReady. It appends PodReady
// True at readyAt to the stored pod, the instant the minReadySeconds window
// measures from.
func MarkPodReadyAt(t *testing.T, c client.Client, pod *corev1.Pod, readyAt time.Time) {
	t.Helper()
	live := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), live); err != nil {
		t.Fatalf("get pod before writing PodReady: %v", err)
	}
	live.Status.Conditions = append(live.Status.Conditions, corev1.PodCondition{
		Type:               corev1.PodReady,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.NewTime(readyAt),
	})
	if err := c.Status().Update(context.Background(), live); err != nil {
		t.Fatalf("write PodReady: %v", err)
	}
}

// LabelPodWithRevision relabels the stored pod with the hash of the given
// ControllerRevision, the shape of a pod whose in-place roll the engine
// has fully observed.
func LabelPodWithRevision(t *testing.T, c client.Client, pod *corev1.Pod, revisionName string) {
	t.Helper()
	fresh := &corev1.Pod{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(pod), fresh); err != nil {
		t.Fatalf("get pod for revision hash: %v", err)
	}
	fresh.Labels[query.LabelRevisionHash] = query.RevisionHashFromControllerRevisionName(revisionName)
	if err := c.Update(context.Background(), fresh); err != nil {
		t.Fatalf("label pod with revision hash: %v", err)
	}
}

// EndpointSliceForPod is an EndpointSlice of the given Service with one
// endpoint that targets the pod and reports the given Ready condition.
func EndpointSliceForPod(namespace, sliceName, serviceName string, pod *corev1.Pod, ready bool) *discoveryv1.EndpointSlice {
	readyPtr := ready
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sliceName,
			Namespace: namespace,
			Labels:    map[string]string{discoveryv1.LabelServiceName: serviceName},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{
			{
				Addresses: []string{"10.0.0.1"},
				Conditions: discoveryv1.EndpointConditions{
					Ready: &readyPtr,
				},
				TargetRef: &corev1.ObjectReference{
					Kind:      "Pod",
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			},
		},
	}
}
