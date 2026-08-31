package servingruntime

import (
	"errors"
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
	"sigs.k8s.io/ome/pkg/constants"
	"sigs.k8s.io/ome/pkg/runtimeinheritance"
)

func TestProjectInheritanceResult_SuccessSetsChainAndCondition(t *testing.T) {
	g := gomega.NewWithT(t)
	current := v1beta1.ServingRuntimeStatus{}
	chain := []string{"profile", "rt"}

	got := projectInheritanceResult(current, 7, chain, nil)

	g.Expect(got.InheritanceChain).To(gomega.Equal(chain))
	g.Expect(got.Conditions).To(gomega.HaveLen(1))
	cond := got.Conditions[0]
	g.Expect(cond.Type).To(gomega.Equal(constants.InheritanceReadyConditionType))
	g.Expect(cond.Status).To(gomega.Equal(metav1.ConditionTrue))
	g.Expect(cond.Reason).To(gomega.Equal(ReasonResolved))
	g.Expect(cond.ObservedGeneration).To(gomega.BeNumerically("==", 7))
}

func TestProjectInheritanceResult_ErrorPreservesLastChain(t *testing.T) {
	g := gomega.NewWithT(t)
	current := v1beta1.ServingRuntimeStatus{
		InheritanceChain: []string{"profile", "rt"},
	}

	pnf := &runtimeinheritance.ParentNotFoundError{Parent: "ghost", Chain: []string{"rt"}}
	got := projectInheritanceResult(current, 3, nil, pnf)

	// Previously-recorded chain is preserved so operators can see what
	// was last known-good while debugging the broken parent.
	g.Expect(got.InheritanceChain).To(gomega.Equal([]string{"profile", "rt"}))
	g.Expect(got.Conditions).To(gomega.HaveLen(1))
	g.Expect(got.Conditions[0].Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(got.Conditions[0].Reason).To(gomega.Equal(ReasonParentNotFound))
}

func TestClassifyResolveError(t *testing.T) {
	g := gomega.NewWithT(t)
	g.Expect(classifyResolveError(&runtimeinheritance.ParentNotFoundError{})).To(gomega.Equal(ReasonParentNotFound))
	g.Expect(classifyResolveError(&runtimeinheritance.CycleError{})).To(gomega.Equal(ReasonCycle))
	g.Expect(classifyResolveError(&runtimeinheritance.MaxDepthExceededError{})).To(gomega.Equal(ReasonMaxDepthExceeded))
	g.Expect(classifyResolveError(errors.New("apiserver unreachable"))).To(gomega.Equal(ReasonResolverInternal))
}

func TestSetCondition_PreservesLastTransitionTimeOnNoChange(t *testing.T) {
	g := gomega.NewWithT(t)
	earlier := metav1.NewTime(metav1.Now().Add(-1))
	conds := []metav1.Condition{
		{
			Type:               constants.InheritanceReadyConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             ReasonResolved,
			LastTransitionTime: earlier,
		},
	}

	setCondition(&conds, metav1.Condition{
		Type:   constants.InheritanceReadyConditionType,
		Status: metav1.ConditionTrue,
		Reason: ReasonResolved,
	})

	g.Expect(conds).To(gomega.HaveLen(1))
	g.Expect(conds[0].LastTransitionTime).To(gomega.Equal(earlier))
}

func TestSetCondition_BumpsLastTransitionTimeOnStatusFlip(t *testing.T) {
	g := gomega.NewWithT(t)
	earlier := metav1.NewTime(metav1.Now().Add(-1))
	conds := []metav1.Condition{
		{
			Type:               constants.InheritanceReadyConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             ReasonResolved,
			LastTransitionTime: earlier,
		},
	}

	setCondition(&conds, metav1.Condition{
		Type:   constants.InheritanceReadyConditionType,
		Status: metav1.ConditionFalse,
		Reason: ReasonCycle,
	})

	g.Expect(conds[0].Status).To(gomega.Equal(metav1.ConditionFalse))
	g.Expect(conds[0].LastTransitionTime).NotTo(gomega.Equal(earlier), "must bump when status flips")
}
