package placement

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

// deleteMemberCopy removes the derived InferenceService on one member together
// with the InferenceReplicas and Pods it owned, as a delete on the member and
// the member's own garbage collection would.
func deleteMemberCopy(t *testing.T, f *backendFixture, name string) {
	t.Helper()
	worker := f.workers[name]
	member := &v1beta1.InferenceService{}
	if err := worker.Get(t.Context(), client.ObjectKeyFromObject(f.source), member); err != nil {
		t.Fatal(err)
	}
	if err := worker.Delete(t.Context(), member); err != nil {
		t.Fatal(err)
	}
	if err := worker.DeleteAllOf(t.Context(), &v1beta1.InferenceReplica{}, client.InNamespace(member.Namespace)); err != nil {
		t.Fatal(err)
	}
	if err := worker.DeleteAllOf(t.Context(), &corev1.Pod{}, client.InNamespace(member.Namespace)); err != nil {
		t.Fatal(err)
	}
}

// A Single source converged on member-a loses its derived copy to a delete on
// that member. The next pass stops reporting a converged placement, and once
// the loss grace has run out a pass places the service again.
func TestSinglePlacementDeletedOnMemberIsRecreated(t *testing.T) {
	f := singleFixture(t, false)
	f.reconcile(t)
	seedSingleAdmission(t, f, "member-a", v1beta1.EngineComponent)
	f.reconcile(t)
	f.reconcile(t)
	acknowledgeSingleFloor(t, f, "member-a", 3)
	f.reconcile(t)
	// The converged pass still asks for a re-read, so a copy that disappears
	// without any member event is noticed by the poll at the latest.
	result, err := f.reconciler.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.source)})
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter <= 0 {
		t.Fatalf("converged pass does not re-read the member: %+v", result)
	}
	settled := &v1beta1.InferenceService{}
	if err := f.reconciler.Get(t.Context(), client.ObjectKeyFromObject(f.source), settled); err != nil {
		t.Fatal(err)
	}
	if condition := settled.Status.GetCondition(v1beta1.PlacementConverged); condition == nil || condition.Status != corev1.ConditionTrue {
		t.Fatalf("placement did not converge: %+v", condition)
	}
	if diff := cmp.Diff(v1beta1.PlacementPhasePlaced, settled.Status.Placement.Phase); diff != "" {
		t.Fatalf("settled phase (-want +got):\n%s", diff)
	}
	f.reconciler.WinnerLostGracePeriod = time.Minute

	deleteMemberCopy(t, f, "member-a")

	lost := f.reconcile(t)
	condition := lost.Status.GetCondition(v1beta1.PlacementConverged)
	if condition == nil || condition.Status == corev1.ConditionTrue {
		t.Fatalf("source still reports a converged placement after its member copy was deleted: %+v", condition)
	}
	if diff := cmp.Diff("AwaitingWinnerRecovery", condition.Reason); diff != "" {
		t.Fatalf("hold reason (-want +got):\n%s", diff)
	}
	if lost.Status.Placement.Phase == v1beta1.PlacementPhasePlaced {
		t.Fatalf("source still reports phase Placed against a deleted member copy: %+v", lost.Status.Placement)
	}
	_, graceRunning := f.reconciler.winnerLostSince.Load(f.source.UID)
	if !graceRunning {
		t.Fatal("the loss grace did not start")
	}
	for _, name := range []string{"member-a", "member-b"} {
		if err := f.workers[name].Get(t.Context(), client.ObjectKeyFromObject(f.source), &v1beta1.InferenceService{}); !apierrors.IsNotFound(err) {
			t.Fatalf("member %s holds a copy before the grace ran out: %v", name, err)
		}
	}

	f.reconciler.winnerLostSince.Store(f.source.UID, time.Now().Add(-2*f.reconciler.WinnerLostGracePeriod))
	var placedOn []string
	for pass := 0; pass < 6 && len(placedOn) == 0; pass++ {
		f.reconcile(t)
		for _, name := range []string{"member-a", "member-b"} {
			err := f.workers[name].Get(t.Context(), client.ObjectKeyFromObject(f.source), &v1beta1.InferenceService{})
			if err == nil {
				placedOn = append(placedOn, name)
			} else if !apierrors.IsNotFound(err) {
				t.Fatal(err)
			}
		}
	}
	// The fresh race offers the service to every eligible member, as the first
	// placement did; the first member to admit it wins and the other copy goes.
	if diff := cmp.Diff([]string{"member-a", "member-b"}, placedOn); diff != "" {
		got := &v1beta1.InferenceService{}
		if err := f.reconciler.Get(t.Context(), client.ObjectKeyFromObject(f.source), got); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("members offered the service after the grace (-want +got):\n%s\nplacement status: %+v", diff, got.Status.Placement)
	}
	seedSingleAdmission(t, f, "member-a", v1beta1.EngineComponent)
	f.reconcile(t)
	f.reconcile(t)
	acknowledgeSingleFloor(t, f, "member-a", 3)
	var recovered *v1beta1.InferenceService
	for pass := 0; pass < 6; pass++ {
		recovered = f.reconcile(t)
		if condition := recovered.Status.GetCondition(v1beta1.PlacementConverged); condition != nil && condition.Status == corev1.ConditionTrue {
			break
		}
	}
	if condition := recovered.Status.GetCondition(v1beta1.PlacementConverged); condition == nil || condition.Status != corev1.ConditionTrue {
		t.Fatalf("placement did not converge again after the member admitted the new copy: %+v", condition)
	}
	if diff := cmp.Diff("member-a", recovered.Status.Placement.Plan.Winner); diff != "" {
		t.Fatalf("placed member after recovery (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(v1beta1.PlacementPhasePlaced, recovered.Status.Placement.Phase); diff != "" {
		t.Fatalf("phase after recovery (-want +got):\n%s", diff)
	}
	if err := f.workers["member-b"].Get(t.Context(), client.ObjectKeyFromObject(f.source), &v1beta1.InferenceService{}); !apierrors.IsNotFound(err) {
		t.Fatalf("the losing member kept its copy after recovery: %v", err)
	}
}
