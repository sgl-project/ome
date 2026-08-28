package types

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodTermination extracts the most operator-relevant container failure
// diagnostics from pod into an *InstanceTermination, stamping `now` as the
// record time. Returns nil when pod is nil and when pod carries no
// failure signal at all — a healthy pod has nothing to report.
//
// Selection precedence (the failure operators most want to see first):
//
//  1. A container terminated with a non-zero exit code — the canonical
//     crash signal (OOMKilled exit 137, generic Error exit 1, ...). Both
//     the live State and the LastTerminationState are consulted, since a
//     CrashLoopBackOff pod shows the crash in LastTerminationState while
//     its live State is Waiting.
//  2. A container stuck in a terminal waiting state (CrashLoopBackOff,
//     ImagePullBackOff, CreateContainerError, ...) — the wedge signal the
//     stuck-pod escalator fires on. ExitCode is left nil (no process ran).
//  3. Any terminated container at all, once the pod phase is Failed
//     (exit 0 but the pod is Failed — rare, e.g. a restartPolicy=Never
//     sidecar exiting). The phase guard matters: an exit-0 container is
//     the normal state of a completed init container, so an unguarded
//     scan reports a termination for every healthy pod that has one.
//  4. Pod-level fallback: Reason="PodFailed" with the pod's status message,
//     when the pod phase is Failed but no per-container detail survived.
//
// Init containers are consulted with the same precedence after regular
// containers so an init-stage crash is still captured.
func PodTermination(pod *corev1.Pod, now metav1.Time) *InstanceTermination {
	if pod == nil {
		return nil
	}

	allStatuses := func() []corev1.ContainerStatus {
		out := make([]corev1.ContainerStatus, 0, len(pod.Status.ContainerStatuses)+len(pod.Status.InitContainerStatuses))
		out = append(out, pod.Status.ContainerStatuses...)
		out = append(out, pod.Status.InitContainerStatuses...)
		return out
	}()

	// 1. Non-zero terminated exit code (live or last-termination).
	for _, cs := range allStatuses {
		if t := nonZeroTerminated(cs); t != nil {
			return terminationFromTerminated(pod, cs.Name, t, now)
		}
	}
	// 2. Terminal waiting-state wedge.
	for _, cs := range allStatuses {
		if cs.State.Waiting != nil && isTerminalWaitingReason(cs.State.Waiting.Reason) {
			return &InstanceTermination{
				PodName:       pod.Name,
				ContainerName: cs.Name,
				Reason:        cs.State.Waiting.Reason,
				Message:       cs.State.Waiting.Message,
				Time:          now,
			}
		}
	}
	// 3. Any terminated container, only once the pod itself has Failed.
	// A clean exit is ordinary in a healthy pod — every completed init
	// container is one — so without the phase guard a Running pod would
	// report a termination it never suffered.
	if pod.Status.Phase == corev1.PodFailed {
		for _, cs := range allStatuses {
			if cs.State.Terminated != nil {
				return terminationFromTerminated(pod, cs.Name, cs.State.Terminated, now)
			}
			if cs.LastTerminationState.Terminated != nil {
				return terminationFromTerminated(pod, cs.Name, cs.LastTerminationState.Terminated, now)
			}
		}
	}
	// 4. Pod-level fallback.
	if pod.Status.Phase == corev1.PodFailed {
		return &InstanceTermination{
			PodName: pod.Name,
			Reason:  "PodFailed",
			Message: pod.Status.Message,
			Time:    now,
		}
	}
	return nil
}

// PodTerminationWithReason is PodTermination with an explicit reason
// override used by the stuck-pod escalator, which has already classified
// the wedge reason from the live waiting state. When PodTermination can't
// extract any per-container detail (e.g. the status snapshot raced the
// kubelet write), the override still gives operators the classified reason
// + pod name rather than an empty record. When PodTermination DID extract a
// record but with an empty Reason, the override fills it in.
func PodTerminationWithReason(pod *corev1.Pod, reason string, now metav1.Time) *InstanceTermination {
	t := PodTermination(pod, now)
	if t == nil {
		name := ""
		if pod != nil {
			name = pod.Name
		}
		return &InstanceTermination{PodName: name, Reason: reason, Time: now}
	}
	if t.Reason == "" {
		t.Reason = reason
	}
	return t
}

// nonZeroTerminated returns the terminated state (live or last) carrying a
// non-zero exit code, or nil. Live State wins over LastTerminationState so
// the freshest crash is reported.
func nonZeroTerminated(cs corev1.ContainerStatus) *corev1.ContainerStateTerminated {
	if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
		return cs.State.Terminated
	}
	if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.ExitCode != 0 {
		return cs.LastTerminationState.Terminated
	}
	return nil
}

// terminationFromTerminated builds an InstanceTermination from a terminated
// container state. Reason falls back to "Error" when kubelet left it blank
// (it occasionally does for OOM races) so the record is never reason-less.
func terminationFromTerminated(pod *corev1.Pod, container string, t *corev1.ContainerStateTerminated, now metav1.Time) *InstanceTermination {
	reason := t.Reason
	if reason == "" {
		reason = "Error"
	}
	exit := t.ExitCode
	return &InstanceTermination{
		PodName:       pod.Name,
		ContainerName: container,
		Reason:        reason,
		ExitCode:      &exit,
		Message:       t.Message,
		Time:          now,
	}
}

// isTerminalWaitingReason mirrors the workload escalator's
// terminalPullFailureReasons set. Duplicated here (rather than imported
// from the root workload package) to keep workload/types free of an import
// edge back to its parent — the set is small and changes rarely.
func isTerminalWaitingReason(reason string) bool {
	switch reason {
	case "ErrImagePull",
		"ImagePullBackOff",
		"InvalidImageName",
		"CreateContainerConfigError",
		"CreateContainerError",
		"CrashLoopBackOff",
		"RunContainerError":
		return true
	}
	return false
}

// ShortString renders an InstanceTermination as a compact, grep-friendly
// fragment for K8s event messages, e.g. `pod foo-0 container main failed
// (OOMKilled, exit 137)` or `pod foo-0 container main stuck
// (ImagePullBackOff)`. Returns "" for a nil receiver so callers can embed
// it unconditionally.
func (t *InstanceTermination) ShortString() string {
	if t == nil {
		return ""
	}
	var b string
	if t.PodName != "" {
		b = "pod " + t.PodName
	} else {
		b = "pod <unknown>"
	}
	if t.ContainerName != "" {
		b += " container " + t.ContainerName
	}
	switch {
	case t.ExitCode != nil:
		b += fmt.Sprintf(" failed (%s, exit %d)", reasonOrUnknown(t.Reason), *t.ExitCode)
	case t.Reason == "PodFailed":
		b += " failed (PodFailed)"
	default:
		b += fmt.Sprintf(" stuck (%s)", reasonOrUnknown(t.Reason))
	}
	return b
}

func reasonOrUnknown(reason string) string {
	if reason == "" {
		return "unknown"
	}
	return reason
}
