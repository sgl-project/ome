package coordination

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
)

// stampedSeqIR is a member of a Sequential chain as the projector and the
// replica controller leave it: the parent generation and the template hash
// stamped beside the spec, the status repeating the generation and the stamp
// it was written for, and both revisions named.
func stampedSeqIR(isvc *v1beta1.InferenceService, c v1beta1.ComponentType, generation, observedGeneration int64, stamp, observedStamp, currentHash, updateHash string) *v1beta1.InferenceReplica {
	annotations := parentGenAnnotation(isvc.Generation)
	annotations[constants.InferenceReplicaTemplateHashAnnotationKey] = stamp
	ir := seqIR(isvc.Name, c, currentHash, updateHash, 1, nil)
	ir.Generation = generation
	ir.Annotations = annotations
	ir.Status.ObservedGeneration = observedGeneration
	ir.Status.ObservedTemplateHash = observedStamp
	ir.Status.Conditions = []metav1.Condition{{Type: conditionReady, Status: metav1.ConditionTrue, LastTransitionTime: metav1.Now()}}
	return ir
}

// TestCheckSequentialGate_StatusLagReadsTheTemplatesNotTheGeneration pins the
// gate's status-lag signal on a chain of the engine then the decoder. The
// engine is converged on its revision; its generation moved three times for
// writes outside its templates and its status is three generations behind,
// but it repeats the template stamp its spec carries: the chain is idle and
// the decoder may start. The same engine with a changed template (a new
// stamp its status does not repeat) and its generation observed is about to
// roll: the chain waits on it and the decoder is held.
func TestCheckSequentialGate_StatusLagReadsTheTemplatesNotTheGeneration(t *testing.T) {
	isvc := mkSequentialFixtureEngineFirst("seq-isvc")
	isvc.Generation = 2
	decoder := stampedSeqIR(isvc, v1beta1.DecoderComponent, 2, 2, "d-stamp", "d-stamp", "OLD", "OLD")

	rewritten := stampedSeqIR(isvc, v1beta1.EngineComponent, 5, 2, "e-stamp", "e-stamp", "OLD", "OLD")
	reads := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects([]client.Object{rewritten, decoder}...).Build()
	allowed, reason := CheckSequentialGate(context.Background(), reads, isvc, v1beta1.DecoderComponent)
	if !allowed {
		t.Fatalf("an engine whose generation moved with no template change leaves the chain idle; the decoder was held: %s", reason)
	}

	retemplated := stampedSeqIR(isvc, v1beta1.EngineComponent, 3, 3, "e-stamp-2", "e-stamp", "OLD", "OLD")
	reads = fake.NewClientBuilder().WithScheme(testScheme()).WithObjects([]client.Object{retemplated, decoder}...).Build()
	allowed, reason = CheckSequentialGate(context.Background(), reads, isvc, v1beta1.DecoderComponent)
	if allowed {
		t.Fatalf("an engine whose status predates its templates is about to roll; the decoder must wait, got %q", reason)
	}
	if reason != "Sequential waiting on engine" {
		t.Errorf("reason=%q, want the chain waiting on the engine", reason)
	}
}
