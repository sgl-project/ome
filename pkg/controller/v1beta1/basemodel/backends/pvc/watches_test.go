package pvc

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/ome/pkg/apis/ome/v1beta1"
)

func TestMapToBaseModels(t *testing.T) {
	scheme := newPVCTestScheme(t)
	ctx := context.Background()
	log := logf.Log.WithName("test")

	matching := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "matching", Namespace: "models"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("pvc://my-pvc/sub"),
		}},
	}
	otherPVC := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "other-pvc", Namespace: "models"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("pvc://different-pvc/sub"),
		}},
	}
	notPVC := &v1beta1.BaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "not-pvc", Namespace: "models"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("oci://n/ns/b/bucket/o/path"),
		}},
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(matching, otherPVC, notPVC).Build()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pvc", Namespace: "models"},
	}
	got := MapToBaseModels(ctx, c, log, pvc)
	if len(got) != 1 {
		t.Fatalf("got %d requests, want 1", len(got))
	}
	if got[0].Name != "matching" || got[0].Namespace != "models" {
		t.Errorf("got %+v, want models/matching", got[0])
	}
}

func TestMapToClusterBaseModels(t *testing.T) {
	scheme := newPVCTestScheme(t)
	ctx := context.Background()
	log := logf.Log.WithName("test")

	matching := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "matching"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("pvc://shared:my-pvc/sub"),
		}},
	}
	wrongNs := &v1beta1.ClusterBaseModel{
		ObjectMeta: metav1.ObjectMeta{Name: "wrong-ns"},
		Spec: v1beta1.BaseModelSpec{Storage: &v1beta1.StorageSpec{
			StorageUri: ptr.To("pvc://other-ns:my-pvc/sub"),
		}},
	}
	c := ctrlclientfake.NewClientBuilder().WithScheme(scheme).WithObjects(matching, wrongNs).Build()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "my-pvc", Namespace: "shared"},
	}
	got := MapToClusterBaseModels(ctx, c, log, pvc)
	if len(got) != 1 {
		t.Fatalf("got %d requests, want 1", len(got))
	}
	if got[0].Name != "matching" {
		t.Errorf("got %+v, want matching", got[0])
	}
}

func TestCreatePhasePredicate(t *testing.T) {
	p := CreatePhasePredicate()
	pending := &corev1.PersistentVolumeClaim{Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending}}
	bound := &corev1.PersistentVolumeClaim{Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}}

	if !p.Create(event.CreateEvent{Object: pending}) {
		t.Errorf("Create event must fire")
	}
	if p.Update(event.UpdateEvent{ObjectOld: pending, ObjectNew: pending}) {
		t.Errorf("phase-unchanged update must NOT fire")
	}
	if !p.Update(event.UpdateEvent{ObjectOld: pending, ObjectNew: bound}) {
		t.Errorf("Pending→Bound update must fire")
	}
	if !p.Delete(event.DeleteEvent{Object: bound}) {
		t.Errorf("Delete event must fire")
	}
}
