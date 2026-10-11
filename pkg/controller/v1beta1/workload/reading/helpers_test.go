package reading_test

// Pod fixtures shared by the tests in this package. servingPod is the one
// healthy shape; every other shape is a modifier applied to it, so a case
// reads as "this shape, that level" rather than as a page of pod status.

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

const (
	hashA = "0a1b2c3d"
	hashB = "4e5f6a7b"
)

// now is the pass's clock in every test.
var now = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

type mod func(*corev1.Pod)

// shape builds a serving pod and applies the modifiers in order.
func shape(name string, mods ...mod) *corev1.Pod {
	pod := servingPod(name)
	for _, m := range mods {
		m(pod)
	}
	return pod
}

// servingPod is the healthy shape: Running, every container ready, PodReady
// and the serving gate True for an hour, on revision hashA as the default
// runner of a single-pod Instance.
func servingPod(name string) *corev1.Pod {
	t := metav1.NewTime(now.Add(-time.Hour))
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			CreationTimestamp: metav1.NewTime(now.Add(-2 * time.Hour)),
			Labels: map[string]string{
				query.LabelRevisionHash: hashA,
				query.LabelRunner:       types.RunnerDefault,
			},
		},
		Spec: corev1.PodSpec{
			ReadinessGates: []corev1.PodReadinessGate{{ConditionType: podreadiness.ConditionType}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{Type: corev1.ContainersReady, Status: corev1.ConditionTrue, LastTransitionTime: t},
				{Type: podreadiness.ConditionType, Status: corev1.ConditionTrue, LastTransitionTime: t},
				{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: t},
			},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "main",
				Ready: true,
				State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
			}},
		},
	}
}

func setCondition(pod *corev1.Pod, t corev1.PodConditionType, status corev1.ConditionStatus, at time.Time) {
	for i := range pod.Status.Conditions {
		if pod.Status.Conditions[i].Type == t {
			pod.Status.Conditions[i].Status = status
			pod.Status.Conditions[i].LastTransitionTime = metav1.NewTime(at)
			return
		}
	}
	pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{Type: t, Status: status, LastTransitionTime: metav1.NewTime(at)})
}

func dropCondition(pod *corev1.Pod, t corev1.PodConditionType) {
	kept := pod.Status.Conditions[:0]
	for _, c := range pod.Status.Conditions {
		if c.Type != t {
			kept = append(kept, c)
		}
	}
	pod.Status.Conditions = kept
}

// Runner and label shapes.

func asWorker(pod *corev1.Pod)   { pod.Labels[query.LabelRunner] = types.RunnerWorker }
func asLeader(pod *corev1.Pod)   { pod.Labels[query.LabelRunner] = types.RunnerLeader }
func noRevision(pod *corev1.Pod) { delete(pod.Labels, query.LabelRevisionHash) }

func onRevision(hash string) mod {
	return func(pod *corev1.Pod) { pod.Labels[query.LabelRevisionHash] = hash }
}

func protocol(p string) mod {
	return func(pod *corev1.Pod) { pod.Labels[query.LabelPairingProtocol] = p }
}

// Gate shapes.

// gateAbsent is a pod whose serving condition was never written while the
// kubelet reports PodReady True: the shape coordination counts as serving
// and podreadiness does not.
func gateAbsent(pod *corev1.Pod) { dropCondition(pod, podreadiness.ConditionType) }

// gateNotFolded is the pass that wrote the gate: the gate reads True but
// the kubelet has not folded it into PodReady yet.
func gateNotFolded(pod *corev1.Pod) { setCondition(pod, corev1.PodReady, corev1.ConditionFalse, now) }

// gateHeldBy is a pod a writer holds out of rotation, with PodReady already
// following the gate.
func gateHeldBy(writer, key string) mod {
	return func(pod *corev1.Pod) {
		setCondition(pod, podreadiness.ConditionType, corev1.ConditionFalse, now)
		setCondition(pod, corev1.PodReady, corev1.ConditionFalse, now)
		for i := range pod.Status.Conditions {
			if pod.Status.Conditions[i].Type == podreadiness.ConditionType {
				pod.Status.Conditions[i].Message = fmt.Sprintf(`[{"userAgent":%q,"key":%q}]`, writer, key)
			}
		}
	}
}

// nodeLost is a serving pod whose kubelet stopped reporting: the node
// lifecycle controller withdrew PodReady and left everything else.
func nodeLost(pod *corev1.Pod) { setCondition(pod, corev1.PodReady, corev1.ConditionFalse, now) }

// Readiness shapes.

// unreadySince is a promoted pod whose readiness probe stopped passing at
// since: the gate stays True, ContainersReady and PodReady are False.
func unreadySince(since time.Time) mod {
	return func(pod *corev1.Pod) {
		setCondition(pod, corev1.ContainersReady, corev1.ConditionFalse, since)
		setCondition(pod, corev1.PodReady, corev1.ConditionFalse, since)
		pod.Status.ContainerStatuses[0].Ready = false
	}
}

// neverReady is a pod that is still coming up: Running, its container not
// ready since its creation, no gate written yet.
func neverReady(pod *corev1.Pod) {
	unreadySince(pod.CreationTimestamp.Time)(pod)
	gateAbsent(pod)
}

// parkedIn is a pod with its container parked in a terminal kubelet
// waiting reason.
func parkedIn(reason string) mod {
	return func(pod *corev1.Pod) {
		unreadySince(now.Add(-10 * time.Minute))(pod)
		pod.Status.ContainerStatuses[0].State = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason}}
	}
}

func readySince(at time.Time) mod {
	return func(pod *corev1.Pod) { setCondition(pod, corev1.PodReady, corev1.ConditionTrue, at) }
}

func readyWithoutTransitionTime(pod *corev1.Pod) {
	for i := range pod.Status.Conditions {
		if pod.Status.Conditions[i].Type == corev1.PodReady {
			pod.Status.Conditions[i].LastTransitionTime = metav1.Time{}
		}
	}
}

// Lifecycle shapes.

// failed is a terminal pod whose conditions were never updated after the
// phase moved, so they still read healthy.
func failed(pod *corev1.Pod)    { pod.Status.Phase = corev1.PodFailed }
func succeeded(pod *corev1.Pod) { pod.Status.Phase = corev1.PodSucceeded }
func terminating(pod *corev1.Pod) {
	ts := metav1.NewTime(now)
	pod.DeletionTimestamp = &ts
}
func pending(pod *corev1.Pod) {
	pod.Status = corev1.PodStatus{Phase: corev1.PodPending}
}

func pods(ps ...*corev1.Pod) []*corev1.Pod { return ps }
