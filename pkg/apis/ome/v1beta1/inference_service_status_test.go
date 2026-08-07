package v1beta1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

// TestRuntimeReadyIsAdvisory_DoesNotClobberReady pins the core property of the
// RuntimeReady advisory condition: it is NOT a dependent of the aggregate
// Ready (conditionSet). A currently-serving ISVC (IngressReady + EngineReady
// True) whose spec.runtime is edited to a missing runtime gets
// RuntimeReady=False but MUST stay Ready=True, so its running pods aren't
// marked unhealthy over a bad spec edit that hasn't been applied.
func TestRuntimeReadyIsAdvisory_DoesNotClobberReady(t *testing.T) {
	ss := &InferenceServiceStatus{}
	// Make the ISVC healthy: both aggregate-Ready dependents True.
	ss.SetCondition(IngressReady, &apis.Condition{Type: IngressReady, Status: corev1.ConditionTrue})
	ss.SetCondition(EngineReady, &apis.Condition{Type: EngineReady, Status: corev1.ConditionTrue})
	if !ss.IsReady() {
		t.Fatalf("precondition: ISVC should be Ready with IngressReady+EngineReady True")
	}

	// Edit spec.runtime to a missing runtime → advisory False.
	ss.SetCondition(RuntimeReady, &apis.Condition{
		Type: RuntimeReady, Status: corev1.ConditionFalse,
		Reason: "RuntimeNotFound", Message: "runtime foo not found",
	})
	if !ss.IsReady() {
		t.Errorf("RuntimeReady=False must NOT flip the aggregate Ready (advisory only)")
	}
	rc := ss.GetCondition(RuntimeReady)
	if rc == nil || rc.Status != corev1.ConditionFalse || rc.Reason != "RuntimeNotFound" {
		t.Errorf("RuntimeReady advisory not recorded correctly: %+v", rc)
	}

	// Runtime later resolves → advisory cleared to True, Ready still True.
	ss.SetCondition(RuntimeReady, &apis.Condition{
		Type: RuntimeReady, Status: corev1.ConditionTrue, Reason: "RuntimeResolved",
	})
	if !ss.IsReady() {
		t.Errorf("clearing RuntimeReady must keep the aggregate Ready True")
	}
	if c := ss.GetCondition(RuntimeReady); c == nil || c.Status != corev1.ConditionTrue {
		t.Errorf("RuntimeReady should be cleared to True, got %+v", c)
	}
}

// TestRuntimeReadyDoesNotMakeUnreadyServiceReady is the guard for the inverse:
// the advisory must not accidentally *satisfy* readiness. An ISVC missing a
// dependent (EngineReady unset) is not Ready, and setting RuntimeReady=True
// must not change that.
func TestRuntimeReadyDoesNotMakeUnreadyServiceReady(t *testing.T) {
	ss := &InferenceServiceStatus{}
	ss.SetCondition(IngressReady, &apis.Condition{Type: IngressReady, Status: corev1.ConditionTrue})
	ss.SetCondition(RuntimeReady, &apis.Condition{Type: RuntimeReady, Status: corev1.ConditionTrue, Reason: "RuntimeResolved"})
	if ss.IsReady() {
		t.Errorf("RuntimeReady=True must not make a service Ready while EngineReady is unset")
	}
}
