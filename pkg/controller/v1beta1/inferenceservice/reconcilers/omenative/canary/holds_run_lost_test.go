package canary

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// A unit held with no run says why the run layer cannot take it over. The
// unit serves its first step under the manual pause when its run is lost;
// the run layer holds the plan because the inline body names a metric
// provider the cluster does not bind (RolloutPlanReady False,
// ProviderUnbound). The hold's event names that reason and the message, so
// the operator learns what to fix, and says that a rollback still applies
// under the hold while a promote waits for the plan. The unit itself holds
// as before: Serving at step 0, the split as written.
func TestReconcile_RunLostUnderAPlanThatDoesNotComposeNamesThePlanError(t *testing.T) {
	isvc, in := fixtureServingManual(t)
	in.RunActive = false
	isvc.Status.SetCondition(apis.ConditionType(v1beta1.RolloutPlanReadyCondition), &apis.Condition{
		Type:    apis.ConditionType(v1beta1.RolloutPlanReadyCondition),
		Status:  corev1.ConditionFalse,
		Reason:  v1beta1.RolloutPlanReasonProviderUnbound,
		Message: "groups[0]: metric provider \"unbound\" is not bound in this cluster",
	})
	rec := record.NewFakeRecorder(8)
	in.Recorder = rec

	res := mustReconcile(t, isvc, in)
	if !res.Active || isvc.Status.Components[v1beta1.EngineComponent].RolloutPhase != v1beta1.RolloutPhasePaused {
		t.Fatalf("the unit must hold where it is: %+v phase=%s", res, isvc.Status.Components[v1beta1.EngineComponent].RolloutPhase)
	}
	event := strings.Join(drainEvents(rec), "\n")
	for _, want := range []string{EventReasonCanaryRunLost, "ProviderUnbound", "not bound in this cluster", "promote", "rollback"} {
		if !strings.Contains(event, want) {
			t.Fatalf("the hold's event must name %q, got %q", want, event)
		}
	}
}

// A unit held with no run while the plan composes names no plan error: the
// run layer takes the record over on its next pass.
func TestReconcile_RunLostUnderAPlanThatComposesNamesNoPlanError(t *testing.T) {
	isvc, in := fixtureServingManual(t)
	in.RunActive = false
	rec := record.NewFakeRecorder(8)
	in.Recorder = rec
	mustReconcile(t, isvc, in)
	event := strings.Join(drainEvents(rec), "\n")
	if !strings.Contains(event, EventReasonCanaryRunLost) || strings.Contains(event, "does not compose") {
		t.Fatalf("the hold's event must not name a plan error while the plan composes, got %q", event)
	}
}
