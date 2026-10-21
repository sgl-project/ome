package fakeclient_test

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

func TestCountingVerb_RefusedCall_IsCountedWithItsKey(t *testing.T) {
	creates := fakeclient.CountingVerb(fakeclient.Create)
	c := fakeclient.New(t, fakeclient.FailingVerb(fakeclient.Create, fakeclient.EveryCall, errors.New("refused")), creates)
	cm := configMap("a", "v")
	if err := c.Create(context.Background(), cm); err == nil {
		t.Fatal("create was not refused")
	}
	if creates.Count() != 1 {
		t.Fatalf("creates = %d, want the refused call counted", creates.Count())
	}
	if keys := creates.Keys(); keys[0] != client.ObjectKeyFromObject(cm) {
		t.Errorf("key = %v, want %v", keys[0], client.ObjectKeyFromObject(cm))
	}
}

func TestCountingVerb_DeleteOptions_AreRecordedResolved(t *testing.T) {
	deletes := fakeclient.CountingVerb(fakeclient.Delete)
	cm := configMap("a", "v")
	c := fakeclient.New(t, fakeclient.WithObjects(cm), deletes)
	uid := types.UID("u1")
	if err := c.Delete(context.Background(), cm, client.Preconditions{UID: &uid}, client.GracePeriodSeconds(5)); err != nil {
		t.Fatalf("delete: %v", err)
	}
	opts := fakeclient.OptionsOf[client.DeleteOptions](deletes)
	if len(opts) != 1 {
		t.Fatalf("recorded %d delete options, want 1", len(opts))
	}
	if opts[0].Preconditions == nil || opts[0].Preconditions.UID == nil || *opts[0].Preconditions.UID != uid {
		t.Errorf("preconditions = %+v, want uid %s", opts[0].Preconditions, uid)
	}
	if opts[0].GracePeriodSeconds == nil || *opts[0].GracePeriodSeconds != 5 {
		t.Errorf("grace = %v, want 5", opts[0].GracePeriodSeconds)
	}
}

func TestCountingVerb_ListOptions_CarryNamespaceAndLabels(t *testing.T) {
	lists := fakeclient.CountingVerb(fakeclient.List)
	c := fakeclient.New(t, lists)
	if err := c.List(context.Background(), &corev1.PodList{}, client.InNamespace("ns"), client.MatchingLabels{"app": "a"}); err != nil {
		t.Fatalf("list: %v", err)
	}
	opts := fakeclient.OptionsOf[client.ListOptions](lists)
	if len(opts) != 1 || opts[0].Namespace != "ns" || opts[0].LabelSelector.String() != "app=a" {
		t.Errorf("list options = %+v, want namespace ns and app=a", opts)
	}
	if keys := lists.Keys(); len(keys) != 1 || keys[0] != (client.ObjectKey{}) {
		t.Errorf("keys = %v, want one empty key for a List", keys)
	}
}

func TestCountingVerb_OnKind_IgnoresOtherKinds(t *testing.T) {
	gets := fakeclient.CountingVerb(fakeclient.Get).OnKind(&corev1.Pod{})
	p, cm := pod("a", ""), configMap("a", "v")
	c := fakeclient.New(t, fakeclient.WithObjects(p, cm), gets)
	ctx := context.Background()
	if err := c.Get(ctx, client.ObjectKeyFromObject(p), &corev1.Pod{}); err != nil {
		t.Fatalf("get pod: %v", err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); err != nil {
		t.Fatalf("get configmap: %v", err)
	}
	if gets.Count() != 1 {
		t.Errorf("gets = %d, want only the pod's", gets.Count())
	}
}

func TestCountingVerb_StatusUpdate_IsNotCountedAsUpdate(t *testing.T) {
	updates := fakeclient.CountingVerb(fakeclient.Update)
	statusUpdates := fakeclient.CountingVerb(fakeclient.StatusUpdate)
	p := pod("a", "")
	c := fakeclient.New(t, fakeclient.WithObjects(p), fakeclient.WithStatusSubresource(&corev1.Pod{}), updates, statusUpdates)
	ctx := context.Background()
	if err := c.Status().Update(ctx, p); err != nil {
		t.Fatalf("status update: %v", err)
	}
	if updates.Count() != 0 || statusUpdates.Count() != 1 {
		t.Fatalf("after Status().Update: updates = %d, status updates = %d, want 0 and 1", updates.Count(), statusUpdates.Count())
	}
	if err := c.Update(ctx, p); err != nil {
		t.Fatalf("update: %v", err)
	}
	if updates.Count() != 1 || statusUpdates.Count() != 1 {
		t.Errorf("after Update: updates = %d, status updates = %d, want 1 and 1", updates.Count(), statusUpdates.Count())
	}
}

func TestCountingVerb_All_ListsCallsInOrder(t *testing.T) {
	gets := fakeclient.CountingVerb(fakeclient.Get)
	c := fakeclient.New(t, gets)
	ctx := context.Background()
	for _, name := range []string{"first", "second"} {
		_ = c.Get(ctx, client.ObjectKey{Namespace: "ns", Name: name}, &corev1.ConfigMap{})
	}
	all := gets.All()
	if len(all) != 2 || all[0].Key.Name != "first" || all[1].Key.Name != "second" {
		t.Errorf("all = %+v, want first then second", all)
	}
	if _, ok := all[0].Options.(client.GetOptions); !ok {
		t.Errorf("options are %T, want client.GetOptions", all[0].Options)
	}
}

func TestCountingWrites_CountsEveryWriteVerbAndNoRead(t *testing.T) {
	cm := configMap("a", "v")
	writes := fakeclient.CountingWrites()
	c := fakeclient.New(t, fakeclient.WithObjects(cm), fakeclient.WithStatusSubresource(&corev1.Pod{}), writes)
	ctx := context.Background()
	if err := c.Get(ctx, client.ObjectKeyFromObject(cm), &corev1.ConfigMap{}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if err := c.List(ctx, &corev1.ConfigMapList{}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if writes.Count() != 0 {
		t.Fatalf("writes after reads = %d, want 0", writes.Count())
	}
	p := pod("a", "")
	if err := c.Create(ctx, p); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := c.Update(ctx, p); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := c.Status().Update(ctx, p); err != nil {
		t.Fatalf("status update: %v", err)
	}
	if err := c.Patch(ctx, cm, client.MergeFrom(cm.DeepCopy())); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if err := c.Delete(ctx, cm); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := c.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace("ns")); err != nil {
		t.Fatalf("delete all of: %v", err)
	}
	if writes.Count() != 6 {
		t.Errorf("writes = %d, want the create, update, status update, patch, delete and delete-all-of", writes.Count())
	}
}
