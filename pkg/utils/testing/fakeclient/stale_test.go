package fakeclient_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/ome/pkg/utils/testing/fakeclient"
)

func TestStaleRead_Get_AnswersTheCopyNotTheStore(t *testing.T) {
	c := fakeclient.New(t, fakeclient.WithObjects(configMap("a", "new")), fakeclient.StaleRead(configMap("a", "old")))
	got := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: "a"}, got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Data["k"] != "old" {
		t.Errorf("data = %v, want the stale copy's k=old", got.Data)
	}
}

func TestStaleRead_Get_AnswersAfterTheStoreDeletedIt(t *testing.T) {
	c := fakeclient.New(t, fakeclient.StaleRead(configMap("a", "old")))
	got := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "ns", Name: "a"}, got); err != nil {
		t.Fatalf("get = %v, want the stale copy", err)
	}
	if got.Data["k"] != "old" {
		t.Errorf("data = %v, want k=old", got.Data)
	}
}

func TestStaleRead_List_ReplacesMatchingItemsAndAddsNone(t *testing.T) {
	c := fakeclient.New(t,
		fakeclient.WithObjects(configMap("a", "new"), configMap("b", "new")),
		fakeclient.StaleRead(configMap("a", "old"), configMap("gone", "old")))
	list := &corev1.ConfigMapList{}
	if err := c.List(context.Background(), list, client.InNamespace("ns")); err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[string]string{}
	for _, item := range list.Items {
		got[item.Name] = item.Data["k"]
	}
	want := map[string]string{"a": "old", "b": "new"}
	if len(got) != len(want) || got["a"] != want["a"] || got["b"] != want["b"] {
		t.Errorf("list = %v, want %v", got, want)
	}
}

func TestStaleRead_OtherKind_IsUntouched(t *testing.T) {
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns"}, StringData: map[string]string{"k": "new"}}
	c := fakeclient.New(t, fakeclient.WithObjects(secret), fakeclient.StaleRead(configMap("a", "old")))
	got := &corev1.Secret{}
	if err := c.Get(context.Background(), client.ObjectKeyFromObject(secret), got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.StringData["k"] != "new" {
		t.Errorf("secret data = %v, want the store's k=new", got.StringData)
	}
}

func TestStaleRead_Copy_IsNotSharedBetweenReads(t *testing.T) {
	c := fakeclient.New(t, fakeclient.StaleRead(configMap("a", "old")))
	ctx := context.Background()
	first := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "ns", Name: "a"}, first); err != nil {
		t.Fatalf("get: %v", err)
	}
	first.Data["k"] = "changed by the caller"
	second := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "ns", Name: "a"}, second); err != nil {
		t.Fatalf("get: %v", err)
	}
	if second.Data["k"] != "old" {
		t.Errorf("second read = %q, want the copy untouched by the first reader", second.Data["k"])
	}
}
