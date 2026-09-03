package irprojector

import (
	"context"
	"testing"

	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// TestEnsureInferenceReplica_PolicyHold_PreservesStoredBlockAndReplicas pins
// the projector's policy-hold contract on a live IR: with a stored KEDA
// block driving /scale, a hold pass (ResolvedAutoscaler=nil +
// PreserveAutoscaler=true) must leave BOTH the stored block and the
// scaler-written replica count untouched. Replica ownership is decided
// from the PRESERVED block, not the nil pass parameter — otherwise every
// hold pass would stamp MinReplicas and fight the frozen scaler,
// scaling a loaded fleet to min during a policy outage.
func TestEnsureInferenceReplica_PolicyHold_PreservesStoredBlockAndReplicas(t *testing.T) {
	g := gomega.NewWithT(t)
	isvc := baselineISVC("llama", "prod") // MinReplicas = 2
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()

	// First projection: the rendered KEDA block lands as the IR's stored
	// last-known-good.
	p := minimalParams(t, isvc, c)
	p.ResolvedAutoscaler = autoscalerBlockA()
	_, err := EnsureInferenceReplica(context.Background(), p)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	// The scaler writes /scale directly on the live object.
	const scaledReplicas int32 = 8
	live := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: "llama-engine", Namespace: "prod"}, live)).To(gomega.Succeed())
	live.Spec.Replicas = ptr.To(scaledReplicas)
	g.Expect(c.Update(context.Background(), live)).To(gomega.Succeed())

	// Hold pass: the policy could not render, so the dispatch site passes
	// nil + PreserveAutoscaler=true.
	hold := minimalParams(t, isvc, c)
	hold.ResolvedAutoscaler = nil
	hold.PreserveAutoscaler = true
	_, err = EnsureInferenceReplica(context.Background(), hold)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	after := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: "llama-engine", Namespace: "prod"}, after)).To(gomega.Succeed())
	g.Expect(after.Spec.Autoscaler).To(gomega.Equal(autoscalerBlockA()),
		"a hold must leave the stored last-known-good block untouched — "+
			"the freeze exists to keep exactly this state")
	g.Expect(after.Spec.Replicas).NotTo(gomega.BeNil())
	g.Expect(*after.Spec.Replicas).To(gomega.Equal(scaledReplicas),
		"replica ownership on a hold follows the preserved (scaling) block — "+
			"re-stamping MinReplicas would fight the frozen scaler at HPA cadence")

	// The same nil pass WITHOUT the hold resumes whole-block replace: the
	// stored block is cleared and the controller re-owns the count.
	resume := minimalParams(t, isvc, c)
	resume.ResolvedAutoscaler = nil
	resume.PreserveAutoscaler = false
	_, err = EnsureInferenceReplica(context.Background(), resume)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	replaced := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: "llama-engine", Namespace: "prod"}, replaced)).To(gomega.Succeed())
	g.Expect(replaced.Spec.Autoscaler).To(gomega.BeNil(),
		"PreserveAutoscaler=false must restore whole-block replace semantics — "+
			"nil ResolvedAutoscaler clears the stored block")
	g.Expect(replaced.Spec.Replicas).NotTo(gomega.BeNil())
	g.Expect(*replaced.Spec.Replicas).To(gomega.Equal(int32(2)),
		"with the block cleared the ISVC controller owns the count again "+
			"and re-applies MinReplicas")
}

// TestEnsureInferenceReplica_PolicyHold_CreateProjectsNoBlock pins the
// held-first-reconcile shape: on Create there is no stored block to
// preserve, so a hold projects nil — no scaler block, never a default HPA
// — while Replicas still initializes from MinReplicas.
func TestEnsureInferenceReplica_PolicyHold_CreateProjectsNoBlock(t *testing.T) {
	g := gomega.NewWithT(t)
	isvc := baselineISVC("llama", "prod") // MinReplicas = 2
	c := fake.NewClientBuilder().WithScheme(testScheme(t)).Build()

	p := minimalParams(t, isvc, c)
	p.ResolvedAutoscaler = nil
	p.PreserveAutoscaler = true
	_, err := EnsureInferenceReplica(context.Background(), p)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	created := &v1beta1.InferenceReplica{}
	g.Expect(c.Get(context.Background(),
		types.NamespacedName{Name: "llama-engine", Namespace: "prod"}, created)).To(gomega.Succeed())
	g.Expect(created.Spec.Autoscaler).To(gomega.BeNil(),
		"a held first reconcile has no last-known-good — it must project nil, "+
			"not a spurious empty block")
	g.Expect(created.Spec.Replicas).NotTo(gomega.BeNil())
	g.Expect(*created.Spec.Replicas).To(gomega.Equal(int32(2)),
		"create still stamps MinReplicas as the initial desired count")
}
