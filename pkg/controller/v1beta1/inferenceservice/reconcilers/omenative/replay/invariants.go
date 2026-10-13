package replay

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/podreadiness"
	"sigs.k8s.io/ome/pkg/controller/v1beta1/workload/query"
)

// TickView is the read-only picture of one tick an observer receives once
// the pass has run. The service and the replicas are copies of what the
// apiserver holds after the pass. The pods are the ones the pass read: a
// pass of the service creates and deletes no pod, so they are the same
// before and after it.
type TickView struct {
	// Tick is the timeline tick the pass ran for.
	Tick int
	// Now is the injected clock after the tick's events.
	Now time.Time
	// Service is the InferenceService after the pass: the status the
	// flush left, which is the previous publication when the flush failed.
	Service *v1beta1.InferenceService
	// Replicas is each Component's InferenceReplica as published before
	// the pass: the revision pair and the rows the run layer and the
	// executors read.
	Replicas map[v1beta1.ComponentType]*v1beta1.InferenceReplica
	// Pods lists each Component's pods in name order.
	Pods map[v1beta1.ComponentType][]*corev1.Pod
	// Run is what the run layer did in the pass.
	Run RunView
}

// RunView is the run layer's outcome for one pass.
type RunView struct {
	// Ran reports that the run layer ran: the service declares a rollout
	// spec or still carries a rollout status.
	Ran bool
	// Opened reports that the pass pinned a fresh active run. Adopted
	// marks the open that bound a canary already in flight to a run
	// instead of starting one.
	Opened  bool
	Adopted bool
	// Targets are the revisions the opened run pinned, per Component.
	Targets []v1beta1.RolloutRunTarget
}

// TickObserver receives the view of every tick a pass ran in.
type TickObserver func(view TickView)

// Finding is one invariant violation.
type Finding struct {
	// Invariant is the invariant's short id.
	Invariant string
	// Component is the Component the finding is about; empty when it is
	// about the service as a whole.
	Component v1beta1.ComponentType
	// Class is the stable key a tally over recorded scenarios groups by:
	// the invariant, the rule and the shape involved, with no number or
	// name in it.
	Class string
	// Detail is the one-line account with the numbers and names.
	Detail string
}

// The invariant ids, as the findings and the tallies spell them.
const (
	// InvariantTrafficBacked is I2: every published traffic target with a
	// share above zero names a revision with a serving pod behind it.
	InvariantTrafficBacked = "I2"
	// InvariantRunOpens is the run layer's own rule: a run opens only
	// when there is something to roll.
	InvariantRunOpens = "run"
)

// componentOrder is the order the controller walks the Components in, so
// findings list in one order whatever a map iterates.
var componentOrder = []v1beta1.ComponentType{v1beta1.EngineComponent, v1beta1.DecoderComponent, v1beta1.RouterComponent}

// CheckInvariants runs every invariant over one tick's view.
func CheckInvariants(view TickView) []Finding {
	var findings []Finding
	findings = append(findings, CheckTrafficBacked(view)...)
	findings = append(findings, CheckRunOpensOnChange(view)...)
	return findings
}

// CheckTrafficBacked is invariant I2, traffic backed: no published traffic
// target sends a share above zero to a revision that has no serving pod. A
// pod serves when it is not being deleted, its PodReady condition and the
// serving gate are both True, and its runner is one a Service publishes.
// The class says whether the target is the Component's latest revision
// and whether the revision has pods at all.
func CheckTrafficBacked(view TickView) []Finding {
	if view.Service == nil {
		return nil
	}
	var findings []Finding
	for _, c := range componentOrder {
		cs, ok := view.Service.Status.Components[c]
		if !ok {
			continue
		}
		for _, target := range cs.Traffic {
			if target.Percent <= 0 {
				continue
			}
			hash := revisionHashOfService(target.RevisionName)
			var pods, serving int
			for _, pod := range view.Pods[c] {
				if pod.Labels[query.LabelRevisionHash] != hash || !query.RoutedRunner(pod) {
					continue
				}
				pods++
				if pod.DeletionTimestamp == nil && podreadiness.ReadyAndServing(pod) {
					serving++
				}
			}
			if serving > 0 {
				continue
			}
			role := "previous"
			if target.LatestRevision {
				role = "latest"
			}
			why := "pods-not-serving"
			if pods == 0 {
				why = "no-pods"
			}
			findings = append(findings, Finding{
				Invariant: InvariantTrafficBacked,
				Component: c,
				Class:     fmt.Sprintf("%s/traffic-to-dark-revision[%s,%s]", InvariantTrafficBacked, role, why),
				Detail: fmt.Sprintf("%s sends %d%% to %s, which has %d pod(s) and none serving",
					c, target.Percent, target.RevisionName, pods),
			})
		}
	}
	return findings
}

// CheckRunOpensOnChange holds the run layer to opening on a change: a run
// that opens pins at least one Component whose target revision differs
// from its stable one, or finds a replica whose published pair is
// diverged or whose rows are not all on its update revision. A run that
// opens with none of these opened on an unchanged spec.
func CheckRunOpensOnChange(view TickView) []Finding {
	if !view.Run.Opened {
		return nil
	}
	for _, target := range view.Run.Targets {
		if target.Revision != target.StableRevision {
			return nil
		}
	}
	for _, c := range componentOrder {
		ir := view.Replicas[c]
		if ir == nil {
			continue
		}
		if ir.Status.CurrentRevision != ir.Status.UpdateRevision {
			return nil
		}
		for _, row := range ir.Status.InstanceStatuses {
			if row.RunningRevision != ir.Status.UpdateRevision || (row.TargetRevision != "" && row.TargetRevision != row.RunningRevision) {
				return nil
			}
		}
	}
	how := "opened"
	if view.Run.Adopted {
		how = "adopted"
	}
	return []Finding{{
		Invariant: InvariantRunOpens,
		Class:     fmt.Sprintf("%s/opens-on-unchanged-spec[%s]", InvariantRunOpens, how),
		Detail: fmt.Sprintf("a run %s with every target equal to its stable revision (%s) and every replica pair converged",
			how, targetsValue(view.Run.Targets)),
	}}
}
