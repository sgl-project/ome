package fakeclient_test

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

func TestEditedRead_Get_EditsTheAnswerOfItsTypeOnly(t *testing.T) {
	p, cm := pod("a", "node-1"), configMap("a", "v")
	c := fakeclient.New(t, fakeclient.WithObjects(p, cm),
		fakeclient.EditedRead(func(cm *corev1.ConfigMap) { cm.Data["k"] = "edited" }))
	ctx := context.Background()
	gotCM := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), gotCM); err != nil {
		t.Fatalf("get configmap: %v", err)
	}
	if gotCM.Data["k"] != "edited" {
		t.Errorf("data = %v, want the edited k=edited", gotCM.Data)
	}
	gotPod := &corev1.Pod{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(p), gotPod); err != nil {
		t.Fatalf("get pod: %v", err)
	}
	if gotPod.Spec.NodeName != "node-1" {
		t.Errorf("node = %q, want the pod left alone", gotPod.Spec.NodeName)
	}
	stored := &corev1.ConfigMap{}
	if err := fakeclient.New(t, fakeclient.WithObjects(cm)).Get(ctx, client.ObjectKeyFromObject(cm), stored); err != nil {
		t.Fatalf("get stored: %v", err)
	}
	if stored.Data["k"] != "v" {
		t.Errorf("stored data = %v, want the store untouched", stored.Data)
	}
}

func TestEditedRead_List_EditsTheListOfItsType(t *testing.T) {
	c := fakeclient.New(t, fakeclient.WithObjects(configMap("a", "v"), configMap("b", "v")),
		fakeclient.EditedRead(func(l *corev1.ConfigMapList) { l.Items = l.Items[:1]; l.SetContinue("next-page") }))
	ctx := context.Background()
	cms := &corev1.ConfigMapList{}
	if err := c.List(ctx, cms); err != nil {
		t.Fatalf("list configmaps: %v", err)
	}
	if len(cms.Items) != 1 || cms.GetContinue() != "next-page" {
		t.Errorf("list = %d items, continue %q; want 1 item and next-page", len(cms.Items), cms.GetContinue())
	}
	pods := &corev1.PodList{}
	if err := c.List(ctx, pods); err != nil {
		t.Fatalf("list pods: %v", err)
	}
	if pods.GetContinue() != "" {
		t.Errorf("pod list continue = %q, want other kinds left alone", pods.GetContinue())
	}
}

func TestEditedRead_RefusedRead_IsNotEdited(t *testing.T) {
	cm := configMap("a", "v")
	edits := 0
	c := fakeclient.New(t, fakeclient.WithObjects(cm),
		fakeclient.FailingVerb(fakeclient.Get, 1, refused),
		fakeclient.EditedRead(func(*corev1.ConfigMap) { edits++ }))
	ctx := context.Background()
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); !errors.Is(err, refused) {
		t.Fatalf("get 1 = %v, want refused", err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); err != nil {
		t.Fatalf("get 2: %v", err)
	}
	if edits != 1 {
		t.Errorf("edits = %d, want only the answered read edited", edits)
	}
}
