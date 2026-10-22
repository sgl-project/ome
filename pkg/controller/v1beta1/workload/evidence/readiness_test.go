package evidence

// The two readiness limbos, as the classifiers see them: running every
// container yet never ContainersReady, and containers ready yet a
// declared readiness gate that never folds into Ready. The shapes are
// disjoint by construction, so a pod is evidence for at most one.

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// TestPodRunningNotReady_ExcludesWaitingContainers pins the detector: only a Running pod whose
// containers have all started yet never report ContainersReady counts.
// A pod with a container parked in a waiting state is a kubelet wedge
// the stuck-pod evidence classifies, not this shape.
func TestPodRunningNotReady_ExcludesWaitingContainers(t *testing.T) {
	now := time.Now()
	if !PodRunningNotReady(runningNotReadyPod("engine-0-default-0", now)) {
		t.Errorf("running pod with ContainersReady=False: got false want true")
	}
	if PodRunningNotReady(servingPod("engine-0-default-0")) {
		t.Errorf("serving pod: got true want false")
	}

	backoff := runningNotReadyPod("engine-0-default-0", now)
	backoff.Status.ContainerStatuses[0].State = corev1.ContainerState{
		Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
	}
	if PodRunningNotReady(backoff) {
		t.Errorf("CrashLoopBackOff pod: got true want false (a kubelet wedge, not a readiness limbo)")
	}
	if PodRunningNotReady(nil) {
		t.Errorf("nil pod: got true want false")
	}
}

// TestPodGateNotFolded_IsDisjointFromRunningNotReady pins the detector and that it is disjoint from the
// containers-never-ready limbo: this shape requires ContainersReady True and
// Ready not True, so a pod is evidence for at most one of the two.
func TestPodGateNotFolded_IsDisjointFromRunningNotReady(t *testing.T) {
	now := time.Now()
	pod := gateNotFoldedPod("engine-0-default-0", now)
	if !podGateNotFolded(pod) {
		t.Errorf("containers ready with Ready=False: got false want true")
	}
	if PodRunningNotReady(pod) {
		t.Errorf("gate limbo also read as a containers-not-ready limbo: the two shapes must be disjoint")
	}
	if podGateNotFolded(runningNotReadyPod("engine-0-default-0", now)) {
		t.Errorf("containers-not-ready pod: got true want false")
	}
	if podGateNotFolded(servingPod("engine-0-default-0")) {
		t.Errorf("serving pod: got true want false")
	}

	backoff := gateNotFoldedPod("engine-0-default-0", now)
	backoff.Status.ContainerStatuses[0].State = corev1.ContainerState{
		Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
	}
	if podGateNotFolded(backoff) {
		t.Errorf("CrashLoopBackOff pod: got true want false (a kubelet wedge, not a gate limbo)")
	}
	if podGateNotFolded(nil) {
		t.Errorf("nil pod: got true want false")
	}
}
