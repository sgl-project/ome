package rolloutrun

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/rollout"
)

// Tests for a rejected revision that stays rejected: no run is pinned and a
// canary unit's record holds a rejected revision a member still targets.
// The divergence toward that revision is a hold, so the passes below open
// nothing and park nothing whatever else moved around the hold.

// A standing hold is left alone by everything that is not a different
// target, a cleared hold or a straggler outside the rejection: a target
// back on the stable revision ends the hold with nothing to roll, a plan or
// policy edit, a reference that stops or starts resolving, a missing or
// converged replica, a coordination phase, the rollback verb and the
// executor parking the unit for want of its stable revision all leave the
// service with no run and RolloutPlanReady True NoActiveRun.
func TestAStandingHoldOpensNothing(t *testing.T) {
	held := func() *v1beta1.InferenceReplica { return divergedIR(oldRev, newRev) }
	refHeld := func(t *testing.T) *v1beta1.InferenceService {
		isvc := isvcFixture(refCanary(policyName))
		setEngineUnit(isvc, v1beta1.RolloutPhaseRolledBack, rejectedRecord(t))
		return isvc
	}
	cases := []struct {
		name  string
		setup func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object)
		check func(t *testing.T, isvc *v1beta1.InferenceService)
	}{
		{
			name: "spec.revision[revert]",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				return rejectedISVC(t, v1beta1.RolloutPhaseRolledBack), []runtime.Object{settledIR(oldRev)}
			},
		},
		{
			name: "spec.planEdit",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				isvc := rejectedISVC(t, v1beta1.RolloutPhaseRolledBack)
				requireNoRunAfter(t, isvc, held())
				isvc.Spec.Rollout.Groups[0].Canary = canaryBody(50, 100)
				return isvc, []runtime.Object{held()}
			},
		},
		{
			name: "policy.changed",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				isvc := refHeld(t)
				requireNoRunAfter(t, isvc, held(), canaryPolicy(policyName))
				return isvc, []runtime.Object{held(), canaryPolicy(policyName, 50, 100)}
			},
		},
		{
			name: "policy.unresolved",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				isvc := refHeld(t)
				requireNoRunAfter(t, isvc, held(), canaryPolicy(policyName))
				return isvc, []runtime.Object{held()}
			},
		},
		{
			name: "policy.resolved",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				isvc := refHeld(t)
				requireNoRunAfter(t, isvc, held())
				return isvc, []runtime.Object{held(), canaryPolicy(policyName)}
			},
		},
		{
			name: "ir.missing",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				return rejectedISVC(t, v1beta1.RolloutPhaseRolledBack), nil
			},
		},
		{
			// The engine converged on its own revision while the router's hold stands.
			name: "ir.converged",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				engine := namedIR("llm-a-engine", engineStableRev, engineStableRev)
				engine.Status.Replicas, engine.Status.UpdatedReplicas = 2, 2
				return concurrentCanaryISVC(t), []runtime.Object{namedIR("llm-a-router", routerStableRev, routerRejectedRev), engine}
			},
		},
		{
			name: "coord.staged",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				isvc := rejectedISVC(t, v1beta1.RolloutPhaseRolledBack)
				setCoordinationPhase(isvc, v1beta1.CoordinationPhaseStaged)
				return isvc, []runtime.Object{held()}
			},
		},
		{
			name: "coord.settled",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				isvc := rejectedISVC(t, v1beta1.RolloutPhaseRolledBack)
				setCoordinationPhase(isvc, v1beta1.CoordinationPhaseIdle)
				return isvc, []runtime.Object{held()}
			},
		},
		{
			name: "verb.rollback",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				isvc := rejectedISVC(t, v1beta1.RolloutPhaseRolledBack)
				annotate(isvc, constants.RolloutRollbackAnnotation, "true")
				return isvc, []runtime.Object{held()}
			},
			check: func(t *testing.T, isvc *v1beta1.InferenceService) {
				if isvc.Annotations[constants.RolloutRollbackAnnotation] != "true" {
					t.Fatal("the run layer consumed the rollback verb")
				}
			},
		},
		{
			// The settled hold's stable revision left the replica's history and
			// the executor parked the unit Failed for want of it; the member
			// still targets the rejected revision.
			name: "canary.parked",
			setup: func(t *testing.T) (*v1beta1.InferenceService, []runtime.Object) {
				isvc := rejectedISVC(t, v1beta1.RolloutPhaseFailed)
				isvc.Status.Rollout = rolledBackRunOnFile(t)
				rollout.CanaryStatusFor(&isvc.Status, v1beta1.EngineComponent).Failed = &v1beta1.CanaryFailure{Reason: v1beta1.CanaryFailureStableRevisionMissing}
				return isvc, []runtime.Object{held()}
			},
			check: func(t *testing.T, isvc *v1beta1.InferenceService) {
				if cs := rollout.CanaryStatusFor(&isvc.Status, v1beta1.EngineComponent); cs.Failed == nil || cs.RolledBackRevisionHash != hashOf(t, newRev) {
					t.Fatalf("the run layer touched the parked unit's record: %+v", cs)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isvc, objects := tc.setup(t)
			for i := 0; i < 2; i++ {
				out := pass(t, isvc, objects...)
				if out.Parked || out.Opened || out.StateChanged {
					t.Fatalf("pass %d: outcome = %+v, want the hold left alone", i, out)
				}
				requireNoRun(t, isvc, v1beta1.RolloutPlanReasonNoRun)
			}
			if tc.check != nil {
				tc.check(t, isvc)
			}
		})
	}
}

// rolledBackRunOnFile is the run record a settled rejection of newRev leaves
// on file: the run closed RolledBack with the engine recorded at newRev over
// its stable oldRev.
func rolledBackRunOnFile(t *testing.T) *v1beta1.RolloutStatus {
	t.Helper()
	return &v1beta1.RolloutStatus{LastRun: &v1beta1.RolloutRunRecord{
		Outcome: v1beta1.RolloutRunRolledBack,
		TargetRevisions: []v1beta1.RolloutRunTarget{
			{Component: v1beta1.EngineComponent, Revision: hashOf(t, newRev), StableRevision: hashOf(t, oldRev)},
		},
	}}
}

// The hold ended and the rejected revision comes back. The unit rolled the
// new revision back, the operator set the spec back to the stable revision,
// and the hold ended with nothing to roll: the service reads Closed with the
// unit's record still naming the rejection and the rolled-back run on file.
// A push back to the rejected revision meets the standing rejection again:
// no run opens, no plan is held back, and the service reads RejectedSticky
// on every later pass. The member set of the rejection is read from the rolled-back
// run record; with no run record on file (a status written by a controller
// without the run model) it is rebuilt from the observed targets, with the
// same reading.
func TestTheRejectedRevisionPushedAgainAfterTheHoldEndedIsHeldAgain(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record *v1beta1.RolloutStatus
	}{
		{name: "the member set comes from the rolled-back run record", record: rolledBackRunOnFile(t)},
		{name: "the member set comes from the observed targets", record: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isvc := rejectedISVC(t, v1beta1.RolloutPhaseRolledBack)
			isvc.Status.Rollout = tc.record
			// The spec is back on the stable revision: the hold ends with
			// nothing to roll and the record keeps the rejection.
			requireNoRunAfter(t, isvc, settledIR(oldRev))
			if cs := rollout.CanaryStatusFor(&isvc.Status, v1beta1.EngineComponent); cs.RolledBackRevisionHash != hashOf(t, newRev) {
				t.Fatalf("the ended hold dropped the rejection from the record: %+v", cs)
			}
			// The rejected revision is pushed again.
			for i := 0; i < 2; i++ {
				out := pass(t, isvc, divergedIR(oldRev, newRev))
				if out.Parked || out.Opened || out.StateChanged || out.RequeueAfter != 0 {
					t.Fatalf("pass %d: outcome = %+v, want the push read as a hold", i, out)
				}
				requireNoRun(t, isvc, v1beta1.RolloutPlanReasonNoRun)
			}
			if tc.record != nil && (isvc.Status.Rollout.LastRun == nil || isvc.Status.Rollout.LastRun.Outcome != v1beta1.RolloutRunRolledBack) {
				t.Fatalf("the hold touched the run record: %+v", isvc.Status.Rollout.LastRun)
			}
			// The hold names the member: a target off the rejected revision would open a run.
			if out := pass(t, isvc, divergedIR(oldRev, thirdRev)); !out.Opened {
				t.Fatalf("a different target must open a run, got %+v", out)
			}
		})
	}
}

// A resume written after the hold ended. The unit rolled the new revision
// back, the operator set the spec back to the stable revision, and then
// wrote the resume verb naming the rejected revision. No member targets that
// revision any more, so the executor refuses the verb and writes nothing:
// the record keeps its rejection, the phase stays RolledBack, only the
// annotation is removed. The run layer then reads a settled hold with
// nothing to roll, on this pass and the next: no run opens, nothing is
// adopted.
func TestARefusedResumeAfterTheHoldEndedOpensNothing(t *testing.T) {
	isvc := rejectedISVC(t, v1beta1.RolloutPhaseRolledBack)
	isvc.Status.Rollout = rolledBackRunOnFile(t)
	requireNoRunAfter(t, isvc, settledIR(oldRev))
	for i := 1; i <= 2; i++ {
		out := pass(t, isvc, settledIR(oldRev))
		if out.Opened || out.Adopted || out.StateChanged || v1beta1.RolloutRunActive(isvc) {
			t.Fatalf("pass %d after the refused resume: outcome = %+v, want nothing opened or adopted", i, out)
		}
	}
	cs := rollout.CanaryStatusFor(&isvc.Status, v1beta1.EngineComponent)
	if cs == nil || cs.RolledBackRevisionHash != hashOf(t, newRev) || isvc.Status.Components[v1beta1.EngineComponent].RolloutPhase != v1beta1.RolloutPhaseRolledBack {
		t.Fatalf("the run layer must leave the refused unit's hold as it found it, got %+v", cs)
	}
}
