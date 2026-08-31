package workload

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// InstanceEvidence is the per-instance observed failure evidence for one
// reconcile: the first workload-caused/terminal stuck pod (if any) and
// whether the in-flight Operation's deadline has elapsed. It is EVIDENCE
// ONLY — building it never writes Phase; deciding the Failed transition
// from it belongs to the reconcile.
type InstanceEvidence struct {
	// StuckPod is the first live, non-deleting pod parked in a
	// workload-caused/terminal waiting state past the grace window
	// (FirstStuckPodForInstance). Nil when no pod is stuck.
	StuckPod *corev1.Pod
	// StuckReason is the kubelet waiting reason of StuckPod ("" when none).
	StuckReason string
	// DeadlinePassed reports whether the instance is in a transient phase
	// with an in-flight Operation whose Deadline lies in the past.
	DeadlinePassed bool
}

// ObservedSnapshot owns the decision-epoch observations for one reconcile.
// Cache and API-reader views remain independent, lazy, and memoized. Evidence
// uses the cache view; destructive planning uses the live-read view.
//
// Not safe for concurrent use — one snapshot per reconcile goroutine.
type ObservedSnapshot struct {
	deps      Deps
	input     ReconcileInput
	component ComponentType
	insts     []InstanceStatus

	live   memoObservation
	cached memoObservation
}

// memoObservation memoizes one Pod observation and its read result.
type memoObservation struct {
	done        bool
	observation ComponentObservation
	err         error
}

// NewObservedSnapshot builds the snapshot; pods are materialized lazily on
// first access.
func NewObservedSnapshot(deps Deps, input ReconcileInput, component ComponentType, insts []InstanceStatus) *ObservedSnapshot {
	return &ObservedSnapshot{deps: deps, input: input, component: component, insts: insts}
}

// LiveObservation returns the decision-epoch destructive-read view. Its source
// records the cached-client fallback when no API reader is configured.
func (s *ObservedSnapshot) LiveObservation(ctx context.Context) (ComponentObservation, error) {
	if !s.live.done {
		var pods PodObservation
		if s.input.AuthoritativePods != nil {
			scope := PodObservationScopeUnknown
			if s.input.AuthoritativePods.OwnerUID != "" && s.input.OwnerObject != nil &&
				s.input.AuthoritativePods.OwnerUID == s.input.OwnerObject.GetUID() {
				scope = PodObservationScopeOwnerUID
			}
			pods = newPodObservation(
				PodObservationSourceAPIReader,
				scope,
				s.input.AuthoritativePods.Pods,
				s.input.AuthoritativePods.ByInstance,
			)
		} else {
			pods, s.live.err = liveObservePods(ctx, s.deps, s.input, s.component)
		}
		if s.live.err == nil {
			s.live.observation, s.live.err = NewDecisionObservation(s.insts, pods)
		}
		s.live.done = true
	}
	return s.live.observation, s.live.err
}

// CachedObservation returns the decision-epoch cache observation.
func (s *ObservedSnapshot) CachedObservation(ctx context.Context) (ComponentObservation, error) {
	if !s.cached.done {
		pods, err := cachedObservePods(ctx, s.deps, s.input, s.component)
		s.cached.err = err
		if err == nil {
			s.cached.observation, s.cached.err = NewDecisionObservation(s.insts, pods)
		}
		s.cached.done = true
	}
	return s.cached.observation, s.cached.err
}

// LivePods returns the Component's pods bucketed by instance from the live-read
// role used by destructive planning.
// Memoized: at most one live List per reconcile.
func (s *ObservedSnapshot) LivePods(ctx context.Context) (map[int32][]*corev1.Pod, error) {
	observation, err := s.LiveObservation(ctx)
	if err != nil {
		return nil, err
	}
	return observation.pods.byInstance, nil
}

// CachedPods returns the Component's pods bucketed by instance from the
// CACHED (informer) source — the non-destructive read the update pass uses.
// Memoized: at most one cached List per reconcile.
func (s *ObservedSnapshot) CachedPods(ctx context.Context) (map[int32][]*corev1.Pod, error) {
	observation, err := s.CachedObservation(ctx)
	if err != nil {
		return nil, err
	}
	return observation.pods.byInstance, nil
}

// EvidenceFor returns the per-instance failure evidence for idx, derived
// from the cached-read pods + the instance's Operation deadline. Evidence
// only — no Phase write. Stuck detection attributes pods the same way the
// escalation pass blames them (podsForStuckCheck: a gang Update surge is
// inspected through its replacement gang, a Migrate pair through
// own-plus-sibling). A non-positive grace disables stuck-pod evidence
// entirely (fast escalation off; the deadline backstop still reports). A
// pod-read error yields deadline-only evidence, so a transient read
// failure never fabricates a stuck signal. now/grace are supplied by the
// caller (clock seam).
func (s *ObservedSnapshot) EvidenceFor(ctx context.Context, idx int32, now time.Time, grace time.Duration) InstanceEvidence {
	var ev InstanceEvidence
	var inst *InstanceStatus
	for i := range s.insts {
		if s.insts[i].Index == idx {
			inst = &s.insts[i]
			break
		}
	}
	if inst != nil {
		ev.DeadlinePassed = operationDeadlinePassed(inst, now)
	}
	if grace <= 0 {
		return ev
	}
	cached, err := s.CachedPods(ctx)
	if err != nil {
		return ev
	}
	pods := cached[idx]
	if inst != nil {
		pods = podsForStuckCheck(*inst, cached)
	}
	if pod, reason := FirstStuckPodForInstance(pods, now, grace); pod != nil {
		ev.StuckPod = pod
		ev.StuckReason = reason
	}
	return ev
}
