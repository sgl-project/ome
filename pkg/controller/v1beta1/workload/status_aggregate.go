package workload

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/drain"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

// DefaultRevisionRetention caps non-live ControllerRevisions per
// workload (StatefulSet default; rollback headroom without unbounded
// etcd growth).
const DefaultRevisionRetention = 10

// terminalPullFailureReasons enumerates kubelet container
// state.Waiting reasons that cannot self-recover. Every entry below
// has been observed in the wild as a wedge that requires either a
// spec change (fix the image tag) or a node-level fix (registry
// creds).
//
// The set covers both pull-side failures (image cannot be obtained)
// and post-pull failures (image obtained but the container cannot
// run or stay running). Naming retains the historical "pull" prefix
// for backwards compatibility, but the classifier is the generic
// terminal-waiting-reason matcher the escalator delegates to —
// every reason that lands a container in Waiting forever without
// hope of self-recovery belongs here.
var terminalPullFailureReasons = map[string]struct{}{
	"ErrImagePull":               {},
	"ImagePullBackOff":           {},
	"InvalidImageName":           {},
	"CreateContainerConfigError": {},
	"CreateContainerError":       {},
	// CrashLoopBackOff lands when the container repeatedly exits
	// non-zero and kubelet's restart backoff has converged on its
	// per-pod cap (~5 minutes between attempts). Common operator-
	// reported triggers: bad command-line args, missing config, image
	// with broken entrypoint. Permanent until the spec changes — even
	// if the pod briefly transitions through Running between attempts,
	// the kubelet's stable steady-state for a wedged container is
	// Waiting{Reason=CrashLoopBackOff}. Without terminal
	// classification here, a bumped image that pulls cleanly but
	// crashes immediately would sit at Phase=Updating until
	// InstanceReadyTimeout expires.
	"CrashLoopBackOff": {},
	// RunContainerError signals the container runtime rejected the
	// start (corrupt manifest, missing required shared libraries,
	// exec-format mismatch — e.g., amd64 image on arm64 node).
	// Permanent until the image is fixed; the controller cannot
	// retry past a broken binary.
	"RunContainerError": {},
}

// PodStuckPullFailure returns (true, reason) when pod has at least
// one container in a terminal pull-failure waiting state that has
// been stuck for ≥ grace from `now`, otherwise (false, "").
//
// Stuck duration is measured from CreationTimestamp (kubelet doesn't
// expose per-state transition time). For ImagePullBackOff kubelet
// retries at least once before flipping to BackOff (~30s on a 404),
// so a 60s grace catches stuck pulls while ignoring the brief
// PodInitializing / ContainerCreating / ImagePulling window.
func PodStuckPullFailure(pod *corev1.Pod, now time.Time, grace time.Duration) (bool, string) {
	if pod == nil {
		return false, ""
	}
	if pod.CreationTimestamp.IsZero() {
		return false, ""
	}
	age := now.Sub(pod.CreationTimestamp.Time)
	if age < grace {
		return false, ""
	}
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting == nil {
			continue
		}
		if _, ok := terminalPullFailureReasons[cs.State.Waiting.Reason]; ok {
			return true, cs.State.Waiting.Reason
		}
	}
	for _, cs := range pod.Status.InitContainerStatuses {
		if cs.State.Waiting == nil {
			continue
		}
		if _, ok := terminalPullFailureReasons[cs.State.Waiting.Reason]; ok {
			return true, cs.State.Waiting.Reason
		}
	}
	return false, ""
}

// AvailablePodSet returns pod names that are Ready and non-terminating
// in any EndpointSlice for serviceName. drain.EndpointAvailable
// excludes terminating-but-Ready endpoints — they're not getting new
// traffic. Single slice list avoids the N+1 reads
// drain.IsPodInRotation per pod would do.
//
// Adapter-agnostic: takes the namespace and headless-service name
// directly, no v1beta1 ISVC handle needed.
func AvailablePodSet(ctx context.Context, reads client.Reader, namespace, serviceName string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	slices, err := drain.EndpointSlicesForService(ctx, reads, namespace, serviceName)
	if err != nil {
		return nil, err
	}
	for _, slice := range slices {
		for _, ep := range slice.Endpoints {
			if ep.TargetRef == nil || ep.TargetRef.Kind != "" && ep.TargetRef.Kind != "Pod" {
				continue
			}
			if !drain.EndpointAvailable(ep) {
				continue
			}
			out[ep.TargetRef.Name] = struct{}{}
		}
	}
	return out, nil
}

// CountReadyPods returns the number of pods whose ContainersReady
// condition is True.
func CountReadyPods(pods []*corev1.Pod) int32 {
	var n int32
	for _, p := range pods {
		if podreadiness.IsContainersReady(p) {
			n++
		}
	}
	return n
}

// CountServingPods counts pods that are BOTH ContainersReady AND have
// the controller's serving gate set to True — i.e., pods actually in
// the load-balancer rotation. This is the count MaxUnavailable budgets
// in coordination/ratio.go gate against; ContainersReady alone misses
// the case where the controller has flipped serving=False (in-place
// update drain, recreate Phase A) while containers technically remain
// Ready.
func CountServingPods(pods []*corev1.Pod) int32 {
	var n int32
	for _, p := range pods {
		if podreadiness.IsContainersReady(p) && podreadiness.IsServing(p) {
			n++
		}
	}
	return n
}

// CountScheduledPods returns the number of pods with Spec.NodeName set.
func CountScheduledPods(pods []*corev1.Pod) int32 {
	var n int32
	for _, p := range pods {
		if p.Spec.NodeName != "" {
			n++
		}
	}
	return n
}

// CountAvailablePods returns the number of pods named in
// availableByName. Used in conjunction with AvailablePodSet to compute
// per-Instance availability.
func CountAvailablePods(pods []*corev1.Pod, availableByName map[string]struct{}) int32 {
	var n int32
	for _, p := range pods {
		if _, ok := availableByName[p.Name]; ok {
			n++
		}
	}
	return n
}

// UniqueNodes returns the deterministically-sorted set of node names
// hosting at least one pod. Returns nil for empty / all-unscheduled
// so NodesOccupied round-trips through nil ↔ nil rather than nil ↔
// empty-slice.
func UniqueNodes(pods []*corev1.Pod) []string {
	if len(pods) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	for _, p := range pods {
		if p.Spec.NodeName == "" {
			continue
		}
		seen[p.Spec.NodeName] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// DesiredPodCountByInstance maps each planned Instance index to its
// desired canonical pod count (sum of Runner sizes). Returns nil for
// an empty plan so callers fall back to the observed PodCount
// comparison (during scale-down with stale status entries).
func DesiredPodCountByInstance(plan ComponentPlan) map[int32]int32 {
	if len(plan.Instances) == 0 {
		return nil
	}
	out := make(map[int32]int32, len(plan.Instances))
	for _, inst := range plan.Instances {
		out[inst.Index] = inst.TotalPods()
	}
	return out
}

// DesiredFor returns the desired pod count for idx — the plan entry's
// value when present, otherwise the observed PodCount (no plan info ⇒
// use the observed count, for stale-status scale-down Instances).
func DesiredFor(desiredByIdx map[int32]int32, idx int32, observedPodCount int32) int32 {
	if desiredByIdx == nil {
		return observedPodCount
	}
	if d, ok := desiredByIdx[idx]; ok {
		return d
	}
	return observedPodCount
}

// InstanceMeetsThreshold classifies an Instance when at least its desired pod
// count satisfies the relevant Ready, Serving, or Available predicate.
//
// Surge-tolerant: a naive `observed == PodCount` would mis-classify
// mid-surge Instances as not-Ready the whole time the +1 surge pod is
// starting up — the visible serving dip we explicitly want to avoid.
//
// desired falls back to observedPodCount when the plan has no entry
// for this index (scale-down with stale status).
func InstanceMeetsThreshold(observedPodCount, observedSatisfying, desired int32) bool {
	if observedPodCount == 0 {
		return false
	}
	if desired <= 0 {
		return observedSatisfying == observedPodCount
	}
	return observedSatisfying >= desired
}

// CountServingInstances counts Instances whose desired-many pods are
// BOTH ContainersReady AND serving=True. This is the count the
// coordination unavailability gate (GateContext.CheckUnavailability)
// works from — "instances actually in the load balancer rotation
// right now."
func CountServingInstances(insts []InstanceStatus, desiredByIdx map[int32]int32) int32 {
	var n int32
	for _, s := range insts {
		desired := DesiredFor(desiredByIdx, s.Index, s.PodCount)
		if InstanceMeetsThreshold(s.PodCount, s.ServingPodCount, desired) {
			n++
		}
	}
	return n
}

// CountAvailableInstances counts Instances whose desired-many pods
// are in EndpointSlice rotation. Strict sub-condition of Ready —
// kube-proxy only publishes Ready pods.
func CountAvailableInstances(insts []InstanceStatus, desiredByIdx map[int32]int32) int32 {
	var n int32
	for _, s := range insts {
		desired := DesiredFor(desiredByIdx, s.Index, s.PodCount)
		if InstanceMeetsThreshold(s.PodCount, s.AvailablePodCount, desired) {
			n++
		}
	}
	return n
}

// ReachedDesiredShape reports whether insts have converged to the desired
// staged shape: exactly (replicas-partition) instances Ready on targetRev,
// the remaining `partition` instances Ready on a prior revision, and no
// missing/extra instances. partition=0 is full rollout, so RolloutComplete
// is ReachedDesiredShape(insts, target, 0, len(insts)). Held instances must
// be Ready too — a staged component is only "at rest" when every instance
// (new and held) is healthy.
func ReachedDesiredShape(insts []InstanceStatus, targetRevName string, partition, replicas int32) bool {
	if replicas <= 0 || int32(len(insts)) != replicas || partition < 0 || partition > replicas {
		return false
	}
	tgt := query.RevisionFromName(targetRevName)
	var onTarget, held int32
	for _, s := range insts {
		if s.Phase != InstancePhaseReady {
			return false
		}
		if query.RevisionFromName(s.RunningRevision).Same(tgt) {
			onTarget++
		} else {
			held++
		}
	}
	return onTarget == replicas-partition && held == partition
}

// RolloutComplete is the CurrentRevision promotion gate: every Instance
// on targetRevName AND Phase=Ready.
//
// Operates on workload.InstanceStatus so both adapters share the same
// gate without converting back to a v1beta1 shape.
func RolloutComplete(insts []InstanceStatus, targetRevName string) bool {
	return ReachedDesiredShape(insts, targetRevName, 0, int32(len(insts)))
}

// FirstStuckPodForInstance returns the first pod in pods that is stuck
// in a terminal kubelet waiting state past the grace window, plus the
// kubelet reason string. Returns (nil, "") when no pod is stuck —
// caller skips escalation.
//
// "First" is order-deterministic by the input slice; callers that need
// a stable index should sort pods before calling. For event emission
// the order doesn't matter (the message names the specific pod).
//
// Adapter-agnostic counterpart of the omenative-side helper; takes
// nothing v1beta1-shaped.
func FirstStuckPodForInstance(pods []*corev1.Pod, now time.Time, grace time.Duration) (*corev1.Pod, string) {
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		// Skip pods marked for deletion — kubelet may be tearing them
		// down, and "ImagePullBackOff on a deleting pod" is not an
		// escalation signal (the pod is about to be gone anyway).
		if pod.DeletionTimestamp != nil {
			continue
		}
		if stuck, reason := PodStuckPullFailure(pod, now, grace); stuck {
			return pod, reason
		}
	}
	return nil, ""
}

// HasWedgedPodAgainstCurrent reports whether any pod carries a
// revision-hash label disagreeing with currentHash — i.e., a pod
// created for a revision the controller doesn't yet (or no longer)
// recognize as current.
//
// Pods missing the label are skipped (no signal). Empty currentHash
// means no revision has been promoted yet — treat every pod hash as
// matching so the wedged-recovery branch doesn't over-fire on the
// initial Create.
//
// Used by the wedged-pod recovery branch of the escalator: catches
// the post-surge shape where the state machine has been reset out of
// the transient phase set but a real wedged surge pod still exists.
func HasWedgedPodAgainstCurrent(pods []*corev1.Pod, currentHash, revisionHashLabel string) bool {
	if currentHash == "" || revisionHashLabel == "" {
		return false
	}
	current := query.RevisionFromHash(currentHash)
	for _, pod := range pods {
		if pod == nil {
			continue
		}
		hash := pod.Labels[revisionHashLabel]
		if hash == "" {
			continue
		}
		if !query.RevisionFromHash(hash).Same(current) {
			return true
		}
	}
	return false
}

// CountersForInstance computes the per-Instance counter set from the
// live pod bucket. availableByPod is the map returned by
// AvailablePodSet.
func CountersForInstance(pods []*corev1.Pod, availableByPod map[string]struct{}) InstanceCounters {
	return InstanceCounters{
		PodCount:          int32(len(pods)),
		ReadyPodCount:     CountReadyPods(pods),
		ServingPodCount:   CountServingPods(pods),
		ScheduledPodCount: CountScheduledPods(pods),
		AvailablePodCount: CountAvailablePods(pods, availableByPod),
		Admitted:          instancePodsAdmitted(pods),
		NodesOccupied:     UniqueNodes(pods),
	}
}

func instancePodsAdmitted(pods []*corev1.Pod) bool {
	if len(pods) == 0 {
		return false
	}
	for _, pod := range pods {
		if PodAdmissionGated(pod) {
			return false
		}
	}
	return true
}

// InstanceCounters is the field-level result of CountersForInstance.
// The caller maps fields onto the per-Instance status by assignment.
type InstanceCounters struct {
	PodCount          int32
	ReadyPodCount     int32
	ServingPodCount   int32
	ScheduledPodCount int32
	AvailablePodCount int32
	Admitted          bool
	NodesOccupied     []string
}

// ComponentCounters is the field-level Component publication result.
type ComponentCounters struct {
	Replicas             int32
	ReadyReplicas        int32
	ServingReplicas      int32
	AvailableReplicas    int32
	UpdatedReplicas      int32
	UpdatedReadyReplicas int32
}

// TakeInlineV1Publication consumes the owned rows, overlays the publication
// observation, and computes Component counters from that same observation.
// Only durable identity and revision fields participate from the stored rows.
func (o *ComponentObservation) TakeInlineV1Publication(desiredByIdx map[int32]int32, targetRevName string) ([]InstanceStatus, ComponentCounters, error) {
	var counters ComponentCounters
	target := query.RevisionFromName(targetRevName)
	statuses, err := o.takeInlineV1Statuses(func(index int32, runningRevision string, current InstanceCounters) {
		desired := DesiredFor(desiredByIdx, index, current.PodCount)
		if InstanceMeetsThreshold(current.PodCount, current.ReadyPodCount, desired) {
			counters.ReadyReplicas++
		}
		if InstanceMeetsThreshold(current.PodCount, current.ServingPodCount, desired) {
			counters.ServingReplicas++
		}
		if InstanceMeetsThreshold(current.PodCount, current.AvailablePodCount, desired) {
			counters.AvailableReplicas++
		}
		if targetRevName != "" && query.RevisionFromName(runningRevision).Same(target) {
			counters.UpdatedReplicas++
			if InstanceMeetsThreshold(current.PodCount, current.ReadyPodCount, desired) {
				counters.UpdatedReadyReplicas++
			}
		}
	})
	if err != nil {
		return nil, ComponentCounters{}, err
	}
	counters.Replicas = int32(len(statuses))
	return statuses, counters, nil
}

// String formats InstanceCounters for debug output.
func (c InstanceCounters) String() string {
	return fmt.Sprintf("pods=%d ready=%d serving=%d scheduled=%d available=%d nodes=%v",
		c.PodCount, c.ReadyPodCount, c.ServingPodCount, c.ScheduledPodCount, c.AvailablePodCount, c.NodesOccupied)
}

// disposableAttempt reports whether s's in-flight attempt is owned by
// the deadline disposition (DisposeExpiredAttempt) rather than the
// plain Failed-preserving-Operation stamp.
//
//   - Create ops (any pod count): YES. Creates have no abandon /
//     teardown continuation — a Failed-with-Operation create would
//     re-arm through the Create stamper every pass (the churn loop).
//     The disposition's Operation-clear is correct for gang creates
//     too: a Failed-no-Operation gang create rebuilds via the ordinary
//     trigger + RetryBlock gate.
//   - Single-pod Update ops (no gang surge markers): YES — same
//     re-arming stampers, same fix.
//   - Gang Update ops (SurgeIndex set / gang surge target step /
//     multi-pod plan): NO. Their Failed-with-Operation continuation is
//     what routes the dispatcher into abandonFailedGangSurge (surge
//     teardown → RetryBlock → source reset); clearing the Operation
//     here would strand the wedged surge gang.
//   - Restart / Migrate ops: NO — their expiry semantics are unchanged.
func disposableAttempt(s *InstanceStatus, desiredPods int32) bool {
	if s == nil || s.Operation == nil {
		return false
	}
	switch s.Operation.Type {
	case InstanceOperationCreate:
		return true
	case InstanceOperationUpdate:
		if s.Operation.SurgeIndex != nil || s.Operation.Step == UpdateStepGangSurgeTarget || s.Operation.Step == UpdateStepGangSurgeTargetCleanup {
			return false
		}
		// Deliberate divergence: multi-pod RECREATE/in-place updates
		// (desiredPods > 1, no surge markers) also keep the legacy
		// Failed-with-Operation stamp — conservative; an abandonless
		// teardown/disposition for those is out of scope here.
		return desiredPods <= 1
	default:
		return false
	}
}

// podsForStuckCheck returns the pods whose stuck state should escalate s.
// During a gang Update surge, inspect the replacement gang exclusively: the
// source stays on the prior revision by design and may be the broken workload
// this corrective rollout is replacing. Treating that old failure as a surge
// failure makes the recovery path delete each replacement immediately. An
// empty replacement bucket is also intentional while its pods are created.
//
// Migrate also uses SurgeIndex, including as a reverse sibling pointer on its
// target status. Preserve its existing own-plus-sibling behavior here; its
// failure semantics are separate from the gang Update state machine.
func podsForStuckCheck(s InstanceStatus, byIdx map[int32][]*corev1.Pod) []*corev1.Pod {
	own := byIdx[s.Index]
	if s.Operation == nil || s.Operation.SurgeIndex == nil {
		return own
	}
	sibling := byIdx[*s.Operation.SurgeIndex]
	if s.Operation.Type == InstanceOperationUpdate {
		return sibling
	}
	if len(sibling) == 0 {
		return own
	}
	out := make([]*corev1.Pod, 0, len(own)+len(sibling))
	out = append(out, own...)
	out = append(out, sibling...)
	return out
}

// ShouldCheckForStuckPods reports whether s qualifies for the
// fast-failure check. Two paths qualify:
//
//   - Transient-phase path: an Instance with an in-flight Operation in
//     the {Creating, Updating, Restarting, Migrating} set.
//
//   - Wedged-pod recovery path: an Instance with at least one pod
//     whose revision-hash label disagrees with currentHash, regardless
//     of Phase / Operation. Catches the wedged-state recovery case
//     where the controller's per-Instance bookkeeping has been reset
//     while a real stuck pod still exists.
//
// Skips Failed (already done) and Deleting (the scale-down pass owns its own
// deadline). Empty currentHash suppresses the wedged branch (no
// rollout has ever promoted → over-firing risk during initial Create).
func ShouldCheckForStuckPods(s *InstanceStatus, pods []*corev1.Pod, currentHash, revisionHashLabel string) bool {
	if s == nil {
		return false
	}
	if s.Phase == InstancePhaseFailed || s.Phase == InstancePhaseDeleting {
		return false
	}
	if s.Operation != nil {
		switch s.Phase {
		case InstancePhaseCreating,
			InstancePhaseUpdating,
			InstancePhaseRestarting,
			InstancePhaseMigrating:
			return true
		}
	}
	return HasWedgedPodAgainstCurrent(pods, currentHash, revisionHashLabel)
}

// stuckPodFailedMutation builds the Phase=Failed stamp for a stuck pod.
// Guard logic: a fresh-empty slot (Phase=="") is treated as a sentinel
// for "resurrected slot, don't touch", a Phase=Failed slot is a no-op,
// and any other Phase flips to Failed.
//
// The Operation field is preserved (when present) — operators want to
// see WHAT was in flight when the kubelet wedge surfaced. termination,
// when non-nil, is recorded on LastFailure in the same write so the
// wedged pod's diagnostics survive the recreate / teardown that follows
// the escalation; nil leaves LastFailure untouched.
func stuckPodFailedMutation(termination *InstanceTermination) func(*InstanceStatus) bool {
	return func(s *InstanceStatus) bool {
		if s.Phase == "" {
			// Fresh-empty slot from the writer's append path — the real
			// InstanceStatus was deleted out from under us
			// (migrate-abandon cleanup, etc.). Don't resurrect.
			return false
		}
		if s.Phase == InstancePhaseFailed {
			return false
		}
		s.Phase = InstancePhaseFailed
		if termination != nil {
			captured := *termination
			s.LastFailure = &captured
		}
		return true
	}
}
