package irprojector

import (
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// TestIRStatusToComponentStatus_FieldContract pins the 1:1 projection of
// every IR.Status summary field onto the ISVC-side LifecycleStatus. The
// ISVC subtree is the only status surface kubectl/dashboard consumers
// read for an OMENative Component, so a field silently dropped here
// disappears from the user-visible API even though the IR carries it.
func TestIRStatusToComponentStatus_FieldContract(t *testing.T) {
	g := gomega.NewWithT(t)
	cc := int32(3)
	ir := &v1beta1.InferenceReplica{
		Status: v1beta1.InferenceReplicaStatus{
			ObservedGeneration:   7,
			Replicas:             5,
			ReadyReplicas:        4,
			ServingReplicas:      3,
			AvailableReplicas:    2,
			UpdatedReplicas:      1,
			UpdatedReadyReplicas: 1,
			CurrentRevision:      "engine-aaaa1111",
			UpdateRevision:       "engine-bbbb2222",
			CollisionCount:       &cc,
			LabelSelector:        "app=example,component=engine",
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionUnknown, Reason: "RolloutInProgress"},
				{Type: "RolloutStalled", Status: metav1.ConditionFalse, Reason: "Progressing"},
			},
		},
	}

	out := IRStatusToComponentStatus(ir)

	g.Expect(out.ObservedGeneration).To(gomega.Equal(int64(7)))
	g.Expect(out.Replicas).To(gomega.Equal(int32(5)))
	g.Expect(out.ReadyReplicas).To(gomega.Equal(int32(4)))
	g.Expect(out.ServingReplicas).To(gomega.Equal(int32(3)))
	g.Expect(out.AvailableReplicas).To(gomega.Equal(int32(2)))
	g.Expect(out.UpdatedReplicas).To(gomega.Equal(int32(1)))
	g.Expect(out.UpdatedReadyReplicas).To(gomega.Equal(int32(1)))
	g.Expect(out.CurrentRevision).To(gomega.Equal("engine-aaaa1111"))
	g.Expect(out.UpdateRevision).To(gomega.Equal("engine-bbbb2222"))
	g.Expect(out.LabelSelector).To(gomega.Equal("app=example,component=engine"))
	g.Expect(out.CollisionCount).NotTo(gomega.BeNil())
	g.Expect(*out.CollisionCount).To(gomega.Equal(int32(3)))
	g.Expect(out.Conditions).To(gomega.HaveLen(2))
	g.Expect(out.Conditions[0].Type).To(gomega.Equal("Ready"))
	g.Expect(out.Conditions[1].Type).To(gomega.Equal("RolloutStalled"))
}

// TestIRStatusToComponentStatus_ConditionsDoNotAlias pins the deep-copy
// contract on the projected Conditions slice: the ISVC-side copy must not
// share a backing array with the IR's status, or a later in-place
// condition update on the IR (SetStatusCondition mutates entries) would
// silently rewrite the already-projected ISVC subtree.
func TestIRStatusToComponentStatus_ConditionsDoNotAlias(t *testing.T) {
	g := gomega.NewWithT(t)
	ir := &v1beta1.InferenceReplica{
		Status: v1beta1.InferenceReplicaStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue, Reason: "AllInstancesReady"},
			},
		},
	}

	out := IRStatusToComponentStatus(ir)
	ir.Status.Conditions[0].Status = metav1.ConditionFalse
	ir.Status.Conditions[0].Reason = "InstanceFailed"

	g.Expect(out.Conditions[0].Status).To(gomega.Equal(metav1.ConditionTrue),
		"the projected condition must not alias the IR's backing array")
	g.Expect(out.Conditions[0].Reason).To(gomega.Equal("AllInstancesReady"))
}

// TestIRStatusToComponentStatus_EmptyConditionsProjectNil pins the
// nil-vs-empty contract: an IR with no conditions projects a nil slice
// (omitted in serialization), not an empty non-nil one, and the returned
// LifecycleStatus pointer itself is always non-nil — its presence marks
// the Component as IR-managed for downstream preservation logic.
func TestIRStatusToComponentStatus_EmptyConditionsProjectNil(t *testing.T) {
	g := gomega.NewWithT(t)
	out := IRStatusToComponentStatus(&v1beta1.InferenceReplica{})
	g.Expect(out).NotTo(gomega.BeNil())
	g.Expect(out.Conditions).To(gomega.BeNil())
	g.Expect(out.CollisionCount).To(gomega.BeNil())
}
