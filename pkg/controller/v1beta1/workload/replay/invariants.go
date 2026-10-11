package replay

import (
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/escalation"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	types "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// TickView is the read-only picture of one tick an observer receives once
// the pass and the deadline park/rearm step have run. Rows and retry blocks
// are copies; pods and endpoints are read live from the fake apiserver, as
// the next pass will list them.
type TickView struct {
	// Tick is the timeline tick the pass ran for.
	Tick int
	// Now is the injected clock after the tick's events.
	Now time.Time
	// Rows are the owner's rows after the pass, each carrying the PodCount
	// the status publication wrote for the next pass to read: the count of
	// the pod set the pass started from, before its own creates and
	// deletes.
	Rows []types.InstanceStatus
	// RetryBlocks are the owner's per-revision retry blocks after the pass.
	RetryBlocks []types.RetryBlock
	// Pods buckets the Component's pods by Instance index as the apiserver
	// lists them after the pass, Terminating pods included.
	Pods map[int32][]*corev1.Pod
	// Routed maps a pod name to the per-revision routed Service whose
	// EndpointSlice publishes it as an available endpoint. A pod absent
	// from the map is in no routed rotation.
	Routed map[string]string
	// Holds are the rollout holds the pass recorded, in order. A cleared
	// hold is not a hold and is not listed.
	Holds []types.RolloutHold
	// Commits are the row writes the store resolved during the tick, in
	// commit order, rejected and no-change mutations included.
	Commits []RowCommit
	// Spec is the desired spec the pass ran against.
	Spec TickSpec
	// StuckPodGrace is the configured grace a pod may fail its readiness
	// for before the engine reads it as out of service.
	StuckPodGrace time.Duration
}

// TickSpec is the part of the desired spec the invariants and the reading
// comparison read.
type TickSpec struct {
	Replicas int32
	// PodsPerInstance is the pod count one Instance is expected to run:
	// one, or the leader plus the workers of a gang.
	PodsPerInstance int32
	// MinReadySeconds is how long a pod holds PodReady before it counts
	// as available.
	MinReadySeconds int32
	// MaxUnavailable, MaxSurge and Partition are the rolling-update knobs
	// as the engine receives them; nil is unset.
	MaxUnavailable *intstr.IntOrString
	MaxSurge       *intstr.IntOrString
	Partition      *int32
	Paused         bool
	// Teardown marks the owner deleted: the plan carries no Instance and
	// every pod is on its way out.
	Teardown bool
}

// TickObserver receives the view of every tick a pass ran in.
type TickObserver func(view TickView)

// Finding is one invariant violation.
type Finding struct {
	// Invariant is the invariant's short id: I1, I3, I4 or I5.
	Invariant string
	// Index is the Instance the finding is about, or ComponentWide when
	// it is about the Component as a whole.
	Index int32
	// Class is the stable key a tally over recorded scenarios groups by:
	// the invariant, the rule and the row states involved, with no number
	// or name in it.
	Class string
	// Detail is the one-line account with the numbers and names.
	Detail string
}

// ComponentWide is the Finding.Index of a finding about the Component as
// a whole rather than one Instance.
const ComponentWide int32 = -1

// The invariant ids, as the findings and the tallies spell them.
const (
	InvariantCapacity     = "I1"
	InvariantBoundedWaits = "I3"
	InvariantStatusTruth  = "I4"
	InvariantSingleOwner  = "I5"
)

// CheckInvariants runs every invariant over one tick's view.
func CheckInvariants(view TickView) []Finding {
	var findings []Finding
	findings = append(findings, CheckCapacity(view)...)
	findings = append(findings, CheckBoundedWaits(view)...)
	findings = append(findings, CheckStatusTruth(view)...)
	findings = append(findings, CheckSingleOwner(view)...)
	return findings
}

// CheckCapacity is invariant I1, capacity: the pods that are Ready,
// serving and in a routed Service's rotation, over every Instance, number
// at least replicas minus the resolved maxUnavailable. The budget comes
// from the engine's own PerComponentMaxUnavailableBudget, so the rounding
// is the engine's; an unset budget caps nothing and the check is skipped,
// as it is for a deleted owner. The replay runs one Component with no
// pairing protocol, so only the protocol-empty branch of the capacity rule
// is checked here.
func CheckCapacity(view TickView) []Finding {
	if view.Spec.Teardown {
		return nil
	}
	rolling := &types.RollingUpdate{MaxUnavailable: view.Spec.MaxUnavailable}
	budget := escalation.PerComponentMaxUnavailableBudget(rolling, view.Spec.Replicas)
	if budget == escalation.BudgetNoLimit {
		return nil
	}
	floor := view.Spec.Replicas - budget
	if floor <= 0 {
		return nil
	}
	var serving int32
	servingByIndex := map[int32]int32{}
	for index, pods := range view.Pods {
		for _, pod := range pods {
			if servesInRotation(view, pod) {
				serving++
				servingByIndex[index]++
			}
		}
	}
	if serving >= floor {
		return nil
	}
	return []Finding{{
		Invariant: InvariantCapacity,
		Index:     ComponentWide,
		Class:     fmt.Sprintf("%s/below-floor[%s]", InvariantCapacity, darkInstanceLabels(view, servingByIndex)),
		Detail: fmt.Sprintf("%d pod(s) serve in a routed rotation; the floor is %d (replicas %d minus maxUnavailable %d)",
			serving, floor, view.Spec.Replicas, budget),
	}}
}

// servesInRotation reports whether a pod carries traffic: Ready and
// serving by the engine's own reading, and published as an available
// endpoint of a routed Service.
func servesInRotation(view TickView, pod *corev1.Pod) bool {
	return podreadiness.ReadyAndServing(pod) && view.Routed[pod.Name] != ""
}

// darkInstanceLabels names the states of the Instances that serve nothing,
// sorted and deduplicated, with a marker when fewer rows exist than
// replicas. It is the part of a capacity class that says why the floor was
// missed.
func darkInstanceLabels(view TickView, servingByIndex map[int32]int32) string {
	labels := map[string]struct{}{}
	for i := range view.Rows {
		row := &view.Rows[i]
		if servingByIndex[row.Index] == 0 {
			labels[stateLabel(row)] = struct{}{}
		}
	}
	if int32(len(view.Rows)) < view.Spec.Replicas {
		labels["no-row"] = struct{}{}
	}
	return joinSorted(labels)
}

// CheckBoundedWaits is invariant I3, bounded waits: an operation still in
// flight past its deadline names what it waits on, or the pass recorded a
// rollout hold for the Component. A settled row runs no clock, and a
// parked (zero) deadline is a wait the adapter has already explained.
func CheckBoundedWaits(view TickView) []Finding {
	if len(view.Holds) > 0 {
		return nil
	}
	var findings []Finding
	for i := range view.Rows {
		row := &view.Rows[i]
		if !types.InFlight(row) {
			continue
		}
		op := row.Operation
		if op == nil || op.Deadline.IsZero() || !view.Now.After(op.Deadline.Time) || op.Waiting != "" {
			continue
		}
		findings = append(findings, Finding{
			Invariant: InvariantBoundedWaits,
			Index:     row.Index,
			Class:     fmt.Sprintf("%s/overdue-unexplained[%s]", InvariantBoundedWaits, stateLabel(row)),
			Detail: fmt.Sprintf("index %d: %s/%s passed its deadline %s ago with no waiting reason and no hold recorded",
				row.Index, op.Type, op.Step, view.Now.Sub(op.Deadline.Time)),
		})
	}
	return findings
}

// CheckStatusTruth is invariant I4, status truth: a Ready row has a pod
// that is Ready and serving; a Failed row has none, unless its LastFailure
// names a replacement attempt beside a serving source; and a row's
// PodCount is the number of pods the apiserver lists for its Instance.
// ServingPodCount is not published by the driver and is not compared.
func CheckStatusTruth(view TickView) []Finding {
	var findings []Finding
	for i := range view.Rows {
		row := &view.Rows[i]
		pods := view.Pods[row.Index]
		var serving []string
		for _, pod := range pods {
			if podreadiness.ReadyAndServing(pod) {
				serving = append(serving, pod.Name)
			}
		}
		switch row.Phase {
		case types.InstancePhaseReady:
			if len(serving) == 0 {
				findings = append(findings, Finding{
					Invariant: InvariantStatusTruth,
					Index:     row.Index,
					Class:     fmt.Sprintf("%s/ready-row-dark[%s]", InvariantStatusTruth, darkReason(pods)),
					Detail:    fmt.Sprintf("index %d is Ready with %d pod(s) and none Ready and serving", row.Index, len(pods)),
				})
			}
		case types.InstancePhaseFailed:
			if len(serving) > 0 && !failureNamesReplacement(row, serving) {
				findings = append(findings, Finding{
					Invariant: InvariantStatusTruth,
					Index:     row.Index,
					Class:     fmt.Sprintf("%s/failed-row-serving[%s]", InvariantStatusTruth, operationLabel(row.Operation)),
					Detail: fmt.Sprintf("index %d is Failed while %s serve(s); lastFailure names %q",
						row.Index, strings.Join(serving, ","), failedPodName(row)),
				})
			}
		}
		if row.PodCount != int32(len(pods)) {
			findings = append(findings, Finding{
				Invariant: InvariantStatusTruth,
				Index:     row.Index,
				Class:     fmt.Sprintf("%s/podcount-differs[%s]", InvariantStatusTruth, stateLabel(row)),
				Detail:    fmt.Sprintf("index %d publishes podCount=%d but the apiserver lists %d pod(s)", row.Index, row.PodCount, len(pods)),
			})
		}
	}
	return findings
}

// darkReason says why a Ready row has no pod that is Ready and serving:
// it has no pods, a pod is serving on its containers with the kubelet yet
// to fold the gate into PodReady, or its pods have lost readiness.
func darkReason(pods []*corev1.Pod) string {
	if len(pods) == 0 {
		return "no-pods"
	}
	for _, pod := range pods {
		if podreadiness.IsContainersReady(pod) && podreadiness.IsServing(pod) {
			return "pod-ready-pending"
		}
	}
	return "pods-not-serving"
}

// operationLabel names an operation for a class string.
func operationLabel(op *types.InstanceOperation) string {
	if op == nil {
		return "no-operation"
	}
	return string(op.Type) + "/" + op.Step
}

// failureNamesReplacement reports whether a Failed row's LastFailure names
// a pod other than the ones serving: the record of a replacement attempt
// that died beside a source still carrying traffic.
func failureNamesReplacement(row *types.InstanceStatus, serving []string) bool {
	name := failedPodName(row)
	if name == "" {
		return false
	}
	for _, pod := range serving {
		if pod == name {
			return false
		}
	}
	return true
}

func failedPodName(row *types.InstanceStatus) string {
	if row.LastFailure == nil {
		return ""
	}
	return row.LastFailure.PodName
}

// CheckSingleOwner is invariant I5, single owner: within one tick at most
// one committed write per index changes a row's phase or its operation's
// identity (presence, id, type or step), and no index is written under two
// operation types. A write belongs to the operation type it leaves on the
// row, or to the type it cleared. A write that only refreshes timestamps,
// counters or the waiting token is a stamp and does not count; a mutation
// that did not commit wrote nothing.
func CheckSingleOwner(view TickView) []Finding {
	type track struct {
		changes int
		path    []string
		opTypes map[string]struct{}
	}
	byIndex := map[int32]*track{}
	var order []int32
	for _, commit := range view.Commits {
		if commit.Outcome != PreconditionOK {
			continue
		}
		t := byIndex[commit.Index]
		if t == nil {
			t = &track{opTypes: map[string]struct{}{}, path: []string{stateLabel(commit.Before)}}
			byIndex[commit.Index] = t
			order = append(order, commit.Index)
		}
		if !identityChanged(commit.Before, commit.After) {
			continue
		}
		t.changes++
		t.path = append(t.path, afterLabel(commit))
		if verb := writtenUnder(commit); verb != "" {
			t.opTypes[verb] = struct{}{}
		}
	}
	var findings []Finding
	for _, index := range order {
		t := byIndex[index]
		if t.changes > 1 {
			findings = append(findings, Finding{
				Invariant: InvariantSingleOwner,
				Index:     index,
				Class:     fmt.Sprintf("%s/identity-writes[%s]", InvariantSingleOwner, strings.Join(t.path, ">")),
				Detail: fmt.Sprintf("index %d took %d committed writes changing its phase or operation in one tick: %s",
					index, t.changes, strings.Join(t.path, " > ")),
			})
		}
		if len(t.opTypes) > 1 {
			findings = append(findings, Finding{
				Invariant: InvariantSingleOwner,
				Index:     index,
				Class:     fmt.Sprintf("%s/two-operation-types[%s]", InvariantSingleOwner, joinSorted(t.opTypes)),
				Detail:    fmt.Sprintf("index %d was written under operation types %s in one tick", index, joinSorted(t.opTypes)),
			})
		}
	}
	return findings
}

// identityChanged reports whether a committed write moved the row's phase
// or its operation's identity. A row appearing or leaving counts; a write
// that touched only timestamps, counters or the waiting token does not.
func identityChanged(before, after *types.InstanceStatus) bool {
	if before == nil || after == nil {
		return before != after
	}
	if before.Phase != after.Phase {
		return true
	}
	if (before.Operation == nil) != (after.Operation == nil) {
		return true
	}
	if before.Operation == nil {
		return false
	}
	return before.Operation.ID != after.Operation.ID ||
		before.Operation.Type != after.Operation.Type ||
		before.Operation.Step != after.Operation.Step
}

// writtenUnder names the operation type a committed write belongs to: the
// one it leaves on the row, else the one it cleared. A write that moves
// only the phase of a row with no operation belongs to no verb.
func writtenUnder(commit RowCommit) string {
	if commit.After != nil && commit.After.Operation != nil {
		return string(commit.After.Operation.Type)
	}
	if commit.Before != nil && commit.Before.Operation != nil {
		return string(commit.Before.Operation.Type)
	}
	return ""
}

// afterLabel names the state a committed write left behind.
func afterLabel(commit RowCommit) string {
	if commit.Remove || commit.After == nil {
		return "removed"
	}
	return stateLabel(commit.After)
}

// stateLabel names a row's state for a class string: the ownership table's
// state id, Empty for no row, and Unknown for a shape the table does not
// name.
func stateLabel(row *types.InstanceStatus) string {
	state := types.StateOf(row)
	if state == types.StateUnknown {
		return "Unknown"
	}
	return string(state)
}

func joinSorted(set map[string]struct{}) string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}
