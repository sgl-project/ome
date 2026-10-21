package evidence_test

// Pod fixtures for the tests written as a consumer of the package come
// from the fixture package; the names below are the ones crashloop_test.go
// calls.

import (
	"time"

	corev1 "k8s.io/api/core/v1"

	fixture "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/testfixture"
)

// tNow is the fixed clock the classifier tests read against.
var tNow = time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)

var runningNotReadyPod = fixture.RunningNotReadyPod

// servingPod is the healthy shape with no revision label, so a crash-loop
// reading counts the pod as the row's own whatever revision the row names.
func servingPod(name string) *corev1.Pod {
	return fixture.Pod(name, fixture.NoRevision)
}

// unreadyAfterServingPod is servingPod after its readiness probe stopped
// passing at since: the serving gate stays True, ContainersReady and
// PodReady are False since then, and the pod was created an hour before.
func unreadyAfterServingPod(name string, since time.Time) *corev1.Pod {
	return fixture.Pod(name, fixture.NoRevision, fixture.CreatedAt(since.Add(-time.Hour)), fixture.UnreadySince(since))
}
