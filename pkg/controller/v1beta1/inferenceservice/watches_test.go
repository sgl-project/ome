package inferenceservice

import (
	"testing"

	"github.com/onsi/gomega"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	v1beta1 "sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// TestISVCReconcileTriggerPredicate_DropsStatusOnlyUpdate pins the For()
// watch filter that breaks the HPA-status reconcile loop: a status-only ISVC
// update (resourceVersion bumped, generation/labels/annotations unchanged)
// must be dropped, while a generation change AND an annotation change must
// pass. The annotation case is load-bearing — OME drives canary
// promote/rollback through annotations that do NOT bump generation, so a
// blanket GenerationChangedPredicate would silently break rollouts.
func TestISVCReconcileTriggerPredicate_DropsStatusOnlyUpdate(t *testing.T) {
	g := gomega.NewWithT(t)
	p := isvcReconcileTriggerPredicate()

	base := func() *v1beta1.InferenceService {
		return &v1beta1.InferenceService{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   "ns1",
				Name:        "isvc1",
				Generation:  3,
				Labels:      map[string]string{"app": "x"},
				Annotations: map[string]string{"ome.io/canary": "stable"},
			},
		}
	}

	cases := []struct {
		name     string
		mutate   func(newObj *v1beta1.InferenceService)
		wantPass bool
	}{
		{
			name: "status-only update (rv bump, no gen/meta change) → dropped",
			mutate: func(o *v1beta1.InferenceService) {
				o.ResourceVersion = "999"
				o.Status.URL = nil // status churn
			},
			wantPass: false,
		},
		{
			name: "generation change (spec) → passes",
			mutate: func(o *v1beta1.InferenceService) {
				o.Generation = 4
			},
			wantPass: true,
		},
		{
			name: "annotation change (no gen bump) → passes",
			mutate: func(o *v1beta1.InferenceService) {
				o.Annotations["ome.io/canary"] = "promote"
			},
			wantPass: true,
		},
		{
			name: "label change (no gen bump) → passes",
			mutate: func(o *v1beta1.InferenceService) {
				o.Labels["app"] = "y"
			},
			wantPass: true,
		},
		{
			// Deleting an ISVC that carries the controller finalizer is an
			// UPDATE (deletionTimestamp set), not a Delete event. Dropping
			// it stalls finalizer teardown until an unrelated event wakes
			// the reconciler.
			name: "deletionTimestamp set (finalizer-held delete) → passes",
			mutate: func(o *v1beta1.InferenceService) {
				now := metav1.Now()
				o.DeletionTimestamp = &now
				o.Finalizers = []string{"ome.io/finalizer"}
			},
			wantPass: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			oldObj := base()
			newObj := base()
			newObj.ResourceVersion = "2"
			tc.mutate(newObj)
			got := p.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
			g.Expect(got).To(gomega.Equal(tc.wantPass))
		})
	}

	// Create / Delete / Generic always pass — lifecycle transitions must
	// reach the reconciler.
	g.Expect(p.Create(event.CreateEvent{Object: base()})).To(gomega.BeTrue())
	g.Expect(p.Delete(event.DeleteEvent{Object: base()})).To(gomega.BeTrue())
	g.Expect(p.Generic(event.GenericEvent{Object: base()})).To(gomega.BeTrue())
}

// TestOwnedStatusIgnoringPredicate_DropsHPAStatusOnly pins the Owns(HPA)
// filter: an HPA status-only update (the metrics controller rewriting
// .status.conditions, no generation/metadata change) must be dropped so it
// does not re-reconcile the owning ISVC; an HPA spec change (generation bump)
// must pass.
func TestOwnedStatusIgnoringPredicate_DropsHPAStatusOnly(t *testing.T) {
	g := gomega.NewWithT(t)
	p := ownedStatusIgnoringPredicate()

	base := func() *autoscalingv2.HorizontalPodAutoscaler {
		return &autoscalingv2.HorizontalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns1", Name: "hpa1", Generation: 1},
		}
	}

	// Status-only update: generation unchanged, only .status churns → drop.
	oldHPA := base()
	newHPA := base()
	newHPA.ResourceVersion = "2"
	newHPA.Status.CurrentReplicas = 5
	newHPA.Status.Conditions = []autoscalingv2.HorizontalPodAutoscalerCondition{
		{Type: autoscalingv2.ScalingActive, Reason: "FailedGetResourceMetric"},
	}
	g.Expect(p.Update(event.UpdateEvent{ObjectOld: oldHPA, ObjectNew: newHPA})).To(gomega.BeFalse(),
		"HPA status-only churn must not re-reconcile the ISVC")

	// Spec change: generation bumped → pass.
	specOld := base()
	specNew := base()
	specNew.Generation = 2
	g.Expect(p.Update(event.UpdateEvent{ObjectOld: specOld, ObjectNew: specNew})).To(gomega.BeTrue(),
		"HPA spec change must re-reconcile the ISVC")

	// Metadata change with no generation bump → pass.
	metaOld := base()
	metaNew := base()
	metaNew.Annotations = map[string]string{"k": "v"}
	g.Expect(p.Update(event.UpdateEvent{ObjectOld: metaOld, ObjectNew: metaNew})).To(gomega.BeTrue())

	// Create / Delete / Generic always pass.
	g.Expect(p.Create(event.CreateEvent{Object: base()})).To(gomega.BeTrue())
	g.Expect(p.Delete(event.DeleteEvent{Object: base()})).To(gomega.BeTrue())
}
