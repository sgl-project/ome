package ops

import (
	"context"
	"fmt"

	workload "sigs.k8s.io/ome/pkg/controller/v1beta1/workload/types"
)

// DemoteUnbackedInstances applies the truth pass: a status-only
// Ready→Pending transition for Instances the decision layer proved
// unbacked (no live pods, no in-flight Operation). The mutation
// re-checks its preconditions against the fresh row, so an op that won
// the race keeps ownership and the demotion no-ops. Counters, revisions,
// and Incarnation are preserved: recovery stays with the ordinary
// passes — Create re-materializes a Pending Instance's pods exactly as
// it would a Ready one's.
func DemoteUnbackedInstances(ctx context.Context, deps workload.Deps, input workload.ReconcileInput, plan workload.ComponentPlan, selections []workload.DemotionSelection) error {
	for _, sel := range selections {
		demoted := false
		err := input.MutateInstance(ctx, sel.Index, func(s *workload.InstanceStatus) bool {
			if s.Phase != workload.InstancePhaseReady || s.Operation != nil {
				return false
			}
			s.Phase = workload.InstancePhasePending
			demoted = true
			return true
		})
		if err != nil {
			return fmt.Errorf("workload.Reconcile: demote instance %d (component=%s): %w", sel.Index, plan.Component, err)
		}
		if demoted {
			recordWarning(deps.Recorder, eventTarget(input), workload.EventReasonInstanceDemoted,
				"Instance %d (component=%s) demoted Ready→Pending: %s", sel.Index, plan.Component, sel.Reason)
		}
	}
	return nil
}
